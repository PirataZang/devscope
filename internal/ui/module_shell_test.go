package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestWatchTabsFollowModuleShell(t *testing.T) {
	p := core.Project{
		Name:   "demo",
		Path:   "/p",
		Status: core.StatusRunning,
		Health: core.HealthHealthy,
		Ports:  []int{8080},
		Containers: []core.Container{
			{Name: "api", Status: "running", State: "running", CPU: 2, Memory: 64 * 1024 * 1024},
		},
	}
	a := &App{
		width: 120, height: 40, tab: TabHealth,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	for name, view := range map[string]string{
		"health":  stripANSI(a.renderHealthTab(&p)),
		"logs":    stripANSI(a.renderLogsTab(&p)),
		"metrics": stripANSI(a.renderMetricsTab(&p)),
		"api":     stripANSI(a.renderApiLanding(&p)),
		"db":      stripANSI(a.renderDbLanding(&p)),
		"json":    stripANSI(a.renderJsonLanding(&p)),
		"jwt":     stripANSI(a.renderJwtLanding(&p)),
		"routes":  stripANSI(a.renderRoutesLanding(&p)),
	} {
		for _, want := range []string{"Projeto", "DETALHES", "AÇÕES RÁPIDAS"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%s missing %q in:\n%s", name, want, view)
			}
		}
	}
}
