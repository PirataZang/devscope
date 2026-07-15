package collectors

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

// ApplyProjectStatus sets Running/Stopped/Degraded and aggregates PM2 metrics.
func ApplyProjectStatus(projects []core.Project, pm2Roots map[string]bool) {
	for i := range projects {
		p := &projects[i]
		running := ProjectRunning(p.Containers) || PM2ProjectRunning(p.Workers) || pm2Roots[p.Path]

		// Add PM2 CPU/RAM to project metrics
		for _, w := range p.Workers {
			if strings.EqualFold(w.Status, "online") {
				p.Metrics.CPUPercent += w.CPU
				p.Metrics.MemoryMB += w.Memory / (1024 * 1024)
			}
		}

		degraded := false
		if running {
			if p.Health == core.HealthUnhealthy {
				degraded = true
			}
			for _, c := range p.Containers {
				if c.Status == "running" && c.Health != "" &&
					!strings.EqualFold(c.Health, "healthy") && !strings.EqualFold(c.Health, "none") {
					degraded = true
				}
			}
			for _, ssl := range p.SSL {
				if ssl.DaysLeft >= 0 && ssl.DaysLeft < 7 {
					degraded = true
				}
			}
		}

		switch {
		case running && degraded:
			p.Status = core.StatusDegraded
		case running:
			p.Status = core.StatusRunning
		case p.HasDockerCompose || p.HasDockerfile || p.WorkerCount > 0:
			p.Status = core.StatusStopped
		default:
			p.Status = core.StatusUnknown
		}
	}
}

// DetectDeployScript finds a deploy command for the project.
func DetectDeployScript(projectPath string) string {
	if script := filepath.Join(projectPath, "deploy.sh"); fileExists(script) {
		return script
	}
	if fileExists(filepath.Join(projectPath, "Makefile")) {
		data, err := os.ReadFile(filepath.Join(projectPath, "Makefile"))
		if err == nil && strings.Contains(string(data), "deploy:") {
			return "make deploy"
		}
	}
	pkg := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(pkg); err == nil {
		s := string(data)
		if strings.Contains(s, `"deploy"`) {
			return "npm run deploy"
		}
	}
	return ""
}

func AssignDeployScripts(projects []core.Project) {
	for i := range projects {
		projects[i].DeployScript = DetectDeployScript(projects[i].Path)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SortPinnedFirst returns projects with pinned paths first.
func SortPinnedFirst(projects []core.Project, pinned []string) []core.Project {
	if len(pinned) == 0 || len(projects) < 2 {
		return projects
	}
	pinSet := make(map[string]bool)
	for _, p := range pinned {
		pinSet[filepath.Clean(p)] = true
	}
	var pinnedList, rest []core.Project
	for _, p := range projects {
		if pinSet[filepath.Clean(p.Path)] {
			pinnedList = append(pinnedList, p)
		} else {
			rest = append(rest, p)
		}
	}
	return append(pinnedList, rest...)
}
