package scanner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiscoverRunningRoots finds project roots for services currently running (Docker, PM2).
func DiscoverRunningRoots(ctx context.Context) map[string]bool {
	roots := make(map[string]bool)
	for _, p := range discoverDockerPaths(ctx) {
		if root := ResolveProjectRoot(p); root != "" {
			roots[root] = true
		}
	}
	for _, p := range discoverPM2Paths(ctx) {
		if root := ResolveProjectRoot(p); root != "" {
			roots[root] = true
		}
	}
	for _, p := range DiscoverComposeRoots(ctx) {
		roots[p] = true
	}
	return roots
}

// DiscoverComposeRoots returns project roots from docker compose labels on all containers.
func DiscoverComposeRoots(ctx context.Context) []string {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "ps", "-aq").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}
	ids := strings.Fields(string(out))
	args := append([]string{"inspect", "-f",
		"{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}\t{{index .Config.Labels \"com.docker.compose.project.config_files\"}}",
	}, ids...)
	out, err = exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var roots []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		for _, raw := range parts {
			for _, piece := range strings.Split(raw, ",") {
				piece = strings.TrimSpace(piece)
				if piece == "" {
					continue
				}
				root := filepath.Clean(piece)
				if strings.HasSuffix(strings.ToLower(root), ".yml") || strings.HasSuffix(strings.ToLower(root), ".yaml") {
					root = filepath.Dir(root)
				}
				if root := resolveExistingRoot(root); root != "" && !seen[root] {
					seen[root] = true
					roots = append(roots, root)
				}
			}
		}
	}
	return roots
}

func discoverDockerPaths(ctx context.Context) []string {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "ps", "-q").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}
	ids := strings.Fields(string(out))
	args := append([]string{"inspect", "-f", "{{.Config.WorkingDir}}\n{{range .Mounts}}{{.Source}}\n{{end}}"}, ids...)
	out, err = exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil
	}
	return existingPaths(strings.Split(strings.TrimSpace(string(out)), "\n"))
}

func discoverPM2Paths(ctx context.Context) []string {
	if _, err := exec.LookPath("pm2"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return nil
	}
	var apps []struct {
		PM2Env struct {
			Cwd string `json:"pm_cwd"`
		} `json:"pm2_env"`
	}
	if json.Unmarshal(out, &apps) != nil {
		return nil
	}
	var paths []string
	for _, app := range apps {
		if app.PM2Env.Cwd != "" {
			paths = append(paths, app.PM2Env.Cwd)
		}
	}
	return existingPaths(paths)
}

func existingPaths(paths []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || p == "/" || seen[p] {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		seen[p] = true
		result = append(result, p)
	}
	return result
}

func resolveExistingRoot(path string) string {
	path = filepath.Clean(path)
	if path == "" || path == "/" {
		return ""
	}
	if root := ResolveProjectRoot(path); root != "" {
		return root
	}
	if _, err := os.Stat(path); err == nil {
		if markers, ok := readFastMarkers(path); ok && !isServiceSubfolder(path, markers) {
			return path
		}
	}
	return ""
}

// ResolveProjectRoot walks up from path until a project marker is found.
func ResolveProjectRoot(path string) string {
	path = filepath.Clean(path)
	for {
		if markers, ok := readFastMarkers(path); ok && !isServiceSubfolder(path, markers) {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}
