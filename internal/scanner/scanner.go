package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/detectors"
)

var scanPathCache = struct {
	mu sync.Mutex
	m  map[string]scanCacheEntry
}{m: make(map[string]scanCacheEntry)}

type scanCacheEntry struct {
	fingerprint int64
	projects    []core.Project
}

type Scanner struct {
	paths    []string
	maxDepth int
	ignore   []string
}

func New(paths []string, maxDepth int, ignore []string) *Scanner {
	return &Scanner{
		paths:    paths,
		maxDepth: maxDepth,
		ignore:   ignore,
	}
}

// FastScan quickly finds project roots using generic markers (.git, .env, package.json, etc.).
func (s *Scanner) FastScan(ctx context.Context) ([]core.Project, error) {
	return s.discover(ctx, false)
}

// Scan performs a deeper scan with framework detection and module grouping.
func (s *Scanner) Scan(ctx context.Context) ([]core.Project, error) {
	return s.discover(ctx, true)
}

func (s *Scanner) discover(ctx context.Context, deep bool) ([]core.Project, error) {
	seen := make(map[string]bool)
	var projects []core.Project

	for _, root := range s.paths {
		select {
		case <-ctx.Done():
			return projects, ctx.Err()
		default:
		}

		if _, err := os.Stat(root); err != nil {
			continue
		}

		if cached, ok := s.cachedProjects(root, deep); ok {
			projects = append(projects, cached...)
			continue
		}

		var rootProjects []core.Project
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if !d.IsDir() {
				return nil
			}

			if isIgnored(d.Name(), s.ignore) {
				return filepath.SkipDir
			}

			if depthRelative(root, path) > s.maxDepth {
				return filepath.SkipDir
			}

			markers, ok := s.readMarkers(path, deep)
			if !ok || seen[path] || isServiceSubfolder(path, markers) {
				return nil
			}

			seen[path] = true
			if deep {
				rootProjects = append(rootProjects, s.BuildProject(path))
			} else {
				rootProjects = append(rootProjects, s.BuildProjectStub(path, markers))
			}
			return filepath.SkipDir
		})
		if err != nil {
			return projects, err
		}
		s.storeCache(root, deep, rootProjects)
		projects = append(projects, rootProjects...)
	}

	return projects, nil
}

// MergeDiscovered adds projects found via Docker/PM2/compose that are not in the filesystem scan.
func (s *Scanner) MergeDiscovered(ctx context.Context, projects []core.Project) []core.Project {
	seen := make(map[string]bool, len(projects))
	for _, p := range projects {
		seen[filepath.Clean(p.Path)] = true
	}

	roots := DiscoverRunningRoots(ctx)
	for root := range roots {
		root = filepath.Clean(root)
		if seen[root] {
			continue
		}
		markers, ok := readFastMarkers(root)
		if !ok || isServiceSubfolder(root, markers) {
			continue
		}
		projects = append(projects, s.BuildProjectStub(root, markers))
		seen[root] = true
	}
	return projects
}

// EnrichProjects runs a focused deep scan on known project paths in parallel.
func (s *Scanner) EnrichProjects(ctx context.Context, stubs []core.Project) []core.Project {
	if len(stubs) == 0 {
		return nil
	}

	results := make([]core.Project, len(stubs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, stub := range stubs {
		wg.Add(1)
		go func(i int, stub core.Project) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				results[i] = stub
				return
			default:
			}

			enriched := s.BuildProject(stub.Path)
			enriched.ID = stub.ID
			if stub.Git != nil {
				enriched.Git = stub.Git
			}
			results[i] = enriched
		}(i, stub)
	}

	wg.Wait()
	return results
}

func (s *Scanner) readMarkers(path string, deep bool) (dirMarkers, bool) {
	if deep {
		return readDirMarkers(path)
	}
	return readFastMarkers(path)
}

// FocusedProjectPath returns the project root for the current working directory.
func FocusedProjectPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return ResolveProjectRoot(cwd)
}

// ProjectStubAt builds a lightweight project stub for a known root path.
func (s *Scanner) ProjectStubAt(path string) core.Project {
	path = filepath.Clean(path)
	markers, ok := readFastMarkers(path)
	if !ok {
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			markers = dirMarkers{Git: true}
		}
	}
	return s.BuildProjectStub(path, markers)
}

func (s *Scanner) BuildProjectStub(path string, m dirMarkers) core.Project {
	fw, lang := guessFromMarkers(m)
	status := core.StatusUnknown
	if m.DockerCompose || m.Dockerfile {
		status = core.StatusStopped
	}

	return core.Project{
		ID:   projectID(path),
		Name: filepath.Base(path),
		Path: path,
		Framework: core.FrameworkInfo{
			Name:     fw,
			Language: lang,
		},
		Status:           status,
		Health:           core.HealthUnknown,
		HasDockerCompose: m.DockerCompose,
		HasDockerfile:    m.Dockerfile,
	}
}

func depthRelative(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

func (s *Scanner) BuildProject(path string) core.Project {
	name := filepath.Base(path)
	matches := detectors.DetectMatches(path)
	fw := core.FrameworkInfo{Name: "Unknown", Language: "Unknown"}
	var frameworks []core.FrameworkInfo
	for _, m := range matches {
		cf := core.FrameworkInfo{Name: m.Name, Version: m.Version, Language: m.Language}
		frameworks = append(frameworks, cf)
	}
	if len(frameworks) > 0 {
		fw = frameworks[0]
	}

	modules := detectModules(path)

	var projectModules []core.ProjectModule
	for _, m := range modules {
		projectModules = append(projectModules, core.ProjectModule{
			Name: m.Name,
			Path: m.Path,
			Role: m.Role,
		})
	}

	markers, _ := readDirMarkers(path)
	status := core.StatusUnknown
	if markers.DockerCompose || markers.Dockerfile {
		status = core.StatusStopped
	}

	return core.Project{
		ID:   projectID(path),
		Name: name,
		Path: path,
		Framework: core.FrameworkInfo{
			Name:     fw.Name,
			Version:  fw.Version,
			Language: fw.Language,
		},
		Frameworks:       frameworks,
		Status:           status,
		Health:           core.HealthUnknown,
		Modules:          projectModules,
		HasDockerCompose: markers.DockerCompose,
		HasDockerfile:    markers.Dockerfile,
	}
}

func projectID(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

func (s *Scanner) rootFingerprint(root string) int64 {
	info, err := os.Stat(root)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

func (s *Scanner) cachedProjects(root string, deep bool) ([]core.Project, bool) {
	scanPathCache.mu.Lock()
	defer scanPathCache.mu.Unlock()
	key := root + fmtDeep(deep)
	fp := s.rootFingerprint(root)
	if e, ok := scanPathCache.m[key]; ok && e.fingerprint == fp {
		return e.projects, true
	}
	return nil, false
}

func (s *Scanner) storeCache(root string, deep bool, projects []core.Project) {
	scanPathCache.mu.Lock()
	defer scanPathCache.mu.Unlock()
	key := root + fmtDeep(deep)
	copied := make([]core.Project, len(projects))
	copy(copied, projects)
	scanPathCache.m[key] = scanCacheEntry{
		fingerprint: s.rootFingerprint(root),
		projects:    copied,
	}
}

func fmtDeep(deep bool) string {
	if deep {
		return ":deep"
	}
	return ":fast"
}
