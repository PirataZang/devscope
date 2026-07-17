package collectors

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/metrics"
	"github.com/devscope/devscope/internal/scanner"
)

type Manager struct {
	store         *core.StateStore
	cfg           *config.Config
	hostCollector *metrics.HostCollector
	nginxDomains  []core.Domain
	sslCerts      []core.SSLCert
}

func NewManager(store *core.StateStore, cfg *config.Config) *Manager {
	return &Manager{
		store:         store,
		cfg:           cfg,
		hostCollector: metrics.NewHostCollector(),
		nginxDomains:  CollectNginxDomains(),
		sslCerts:      CollectSSLCerts(),
	}
}

// QuickScan finds projects via fast filesystem markers before the UI opens.
func (m *Manager) QuickScan(ctx context.Context) {
	s := scanner.New(m.cfg.Scan.Paths, m.cfg.Scan.MaxDepth, m.cfg.Scan.Ignore)
	projects, err := s.FastScan(ctx)
	if err != nil {
		log.Printf("fast scan error: %v", err)
		return
	}
	projects = filterNestedProjectList(projects)
	projects = SortPinnedFirst(projects, m.cfg.Pinned)
	populateGitSummaries(projects)
	if containers, meta, err := CollectDockerPS(ctx); err == nil {
		AssignContainersToProjects(projects, containers, meta)
		ApplyProjectStatus(projects, nil)
	}
	m.store.SetProjects(projects)
}

func (m *Manager) Start(ctx context.Context) {
	go m.safeRunCtx(ctx, "scanner", m.runScanner)
	go m.safeRunCtx(ctx, "docker", m.runDocker)
	go m.safeRunCtx(ctx, "metrics", m.runMetrics)
	go m.safeRunCtx(ctx, "health", m.runHealth)
}

// StartWithContext is an alias for Start.
func (m *Manager) StartWithContext(ctx context.Context) {
	m.Start(ctx)
}

func (m *Manager) runScanner(ctx context.Context) {
	s := scanner.New(m.cfg.Scan.Paths, m.cfg.Scan.MaxDepth, m.cfg.Scan.Ignore)
	m.deepScan(ctx, s)

	ticker := time.NewTicker(m.cfg.Refresh.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.deepScan(ctx, s)
		}
	}
}

func (m *Manager) deepScan(ctx context.Context, s *scanner.Scanner) {
	snap := m.store.Get()
	stubs := snap.Projects
	if len(stubs) == 0 {
		var err error
		stubs, err = s.FastScan(ctx)
		if err != nil {
			log.Printf("scanner error: %v", err)
			return
		}
	}
	stubs = s.MergeDiscovered(ctx, stubs)
	stubs = filterNestedProjectList(stubs)

	projects := s.EnrichProjects(ctx, stubs)
	m.refreshProjects(ctx, projects)
}

func (m *Manager) runDocker(ctx context.Context) {
	m.refreshDocker(ctx)

	ticker := time.NewTicker(m.cfg.Refresh.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshDocker(ctx)
		}
	}
}

func (m *Manager) refreshDocker(ctx context.Context) {
	snap := m.store.Get()
	if len(snap.Projects) == 0 {
		return
	}
	projects := cloneProjects(snap.Projects)
	m.refreshProjects(ctx, projects)
}

// refreshProjects runs a fast docker ps + lightweight metadata for the dashboard.
func (m *Manager) refreshProjects(ctx context.Context, projects []core.Project) {
	populateGitSummaries(projects)
	if containers, meta, err := CollectDockerPS(ctx); err != nil {
		log.Printf("docker ps error: %v", err)
	} else {
		AssignContainersToProjects(projects, containers, meta)
	}

	pm2Apps := CollectPM2(ctx)
	AssignWorkersToProjects(projects, pm2Apps)
	AssignPortsToProjects(projects, ReadListeningPorts())
	AssignDomainsToProjects(projects, m.nginxDomains)
	AssignSSLToProjects(projects, m.sslCerts)
	AssignDeployScripts(projects)
	ApplyProjectStatus(projects, nil)
	projects = SortPinnedFirst(projects, m.cfg.Pinned)
	m.store.SetProjects(projects)
}

func populateGitSummaries(projects []core.Project) {
	for i := range projects {
		if projects[i].Git == nil {
			projects[i].Git = CollectGitSummary(projects[i].Path)
		}
	}
}

func (m *Manager) runHealth(ctx context.Context) {
	m.refreshHealth(ctx)

	ticker := time.NewTicker(m.cfg.Refresh.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshHealth(ctx)
		}
	}
}

func (m *Manager) refreshHealth(ctx context.Context) {
	snap := m.store.Get()
	if len(snap.Projects) == 0 {
		return
	}
	projects := cloneProjects(snap.Projects)
	CollectHealth(ctx, projects, m.cfg.Health)
	ApplyProjectStatus(projects, nil)
	m.store.Update(func(s *core.Snapshot) {
		byPath := make(map[string]core.Project, len(projects))
		for _, p := range projects {
			byPath[p.Path] = p
		}
		for i := range s.Projects {
			if updated, ok := byPath[s.Projects[i].Path]; ok {
				s.Projects[i].Health = updated.Health
				s.Projects[i].HealthChecks = updated.HealthChecks
				s.Projects[i].Status = updated.Status
			}
		}
	})
}

func cloneProjects(in []core.Project) []core.Project {
	out := make([]core.Project, len(in))
	copy(out, in)
	return out
}

func (m *Manager) runMetrics(ctx context.Context) {
	m.store.SetHostMetrics(m.hostCollector.Collect())

	ticker := time.NewTicker(m.cfg.Refresh.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.store.SetHostMetrics(m.hostCollector.Collect())
		}
	}
}

func (m *Manager) safeRunCtx(ctx context.Context, name string, fn func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("collector %s panic: %v\n%s", name, r, debug.Stack())
		}
	}()
	fn(ctx)
}

// EnrichSnapshot runs a full enrichment pass (for scan/watch CLI).
func EnrichSnapshot(ctx context.Context, cfg *config.Config, projects []core.Project) []core.Project {
	m := NewManager(core.NewStateStore(cfg.Scan.Paths), cfg)
	enrichProjectsFull(ctx, projects, m)
	return SortPinnedFirst(projects, cfg.Pinned)
}
