package collectors

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

type pm2App struct {
	Name   string `json:"name"`
	Monit  struct {
		CPU  float64 `json:"cpu"`
		Memory int64 `json:"memory"`
	} `json:"monit"`
	PM2Env struct {
		Status string `json:"status"`
		Cwd    string `json:"pm_cwd"`
	} `json:"pm2_env"`
	PMID      int `json:"pm_id"`
	RestartTime int `json:"restart_time"`
}

// CollectPM2 returns all PM2 workers.
func CollectPM2(ctx context.Context) []pm2App {
	if _, err := exec.LookPath("pm2"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return nil
	}
	var apps []pm2App
	if json.Unmarshal(out, &apps) != nil {
		return nil
	}
	return apps
}

// AssignWorkersToProjects links PM2 apps to projects by pm_cwd.
func AssignWorkersToProjects(projects []core.Project, apps []pm2App) {
	for i := range projects {
		projects[i].Workers = nil
		projects[i].WorkerCount = 0
	}

	for _, app := range apps {
		cwd := filepath.Clean(app.PM2Env.Cwd)
		if cwd == "" {
			continue
		}
		bestIdx := -1
		bestLen := 0
		for i, p := range projects {
			pp := filepath.Clean(p.Path)
			if cwd == pp || strings.HasPrefix(cwd, pp+string(filepath.Separator)) {
				if len(pp) > bestLen {
					bestLen = len(pp)
					bestIdx = i
				}
			}
		}
		if bestIdx < 0 {
			continue
		}
		w := core.Worker{
			Name:     app.Name,
			Status:   app.PM2Env.Status,
			CPU:      app.Monit.CPU,
			Memory:   app.Monit.Memory,
			Restarts: app.RestartTime,
		}
		projects[bestIdx].Workers = append(projects[bestIdx].Workers, w)
		projects[bestIdx].WorkerCount = len(projects[bestIdx].Workers)
	}
}

func PM2ProjectRunning(workers []core.Worker) bool {
	for _, w := range workers {
		if strings.EqualFold(w.Status, "online") || strings.EqualFold(w.Status, "launching") {
			return true
		}
	}
	return false
}
