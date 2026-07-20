package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestOverviewDashboardLayout(t *testing.T) {
	p := core.Project{
		Name:             "digiliza",
		Path:             "/home/igor/digiliza",
		Status:           core.StatusDegraded,
		Health:           core.HealthUnhealthy,
		HasDockerCompose: true,
		ContainerCount:   9,
		Ports:            []int{3001, 8080},
		Frameworks: []core.FrameworkInfo{
			{Name: "Laravel", Language: "PHP"},
			{Name: "Vue", Language: "TypeScript"},
		},
		Git: &core.GitInfo{
			IsRepo:        true,
			Branch:        "DES-2834",
			LastCommit:    "a1b2c3d",
			LastCommitMsg: "fix auth",
			Ahead:         2,
			Behind:        1,
		},
		HealthChecks: []core.HealthCheckResult{
			{URL: "API", Status: core.HealthUnhealthy},
			{URL: "Database", Status: core.HealthHealthy},
		},
	}
	a := &App{
		width:           120,
		height:          42,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot: core.Snapshot{
			Projects:    []core.Project{p},
			HostMetrics: core.HostMetrics{CPUPercent: 12, MemoryPercent: 40, MemoryUsedMB: 1800, MemoryTotalMB: 8192, DiskPercent: 42},
		},
	}
	view := stripANSI(a.renderOverviewTab(&p))
	for _, want := range []string{
		"Projeto", "Ambiente", "Servidor", "Uptime",
		"PROJETO", "Atenção", "STACK", "RUNTIME", "MÓDULOS", "GIT",
		"ATIVIDADE", "HEALTH", "DETALHES", "AÇÕES RÁPIDAS", "NOTAS",
		"Laravel", "DES-2834",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in:\n%s", want, view)
		}
	}
}

func TestProjectEnvLabel(t *testing.T) {
	if got := projectEnvLabel(&core.Project{Git: &core.GitInfo{Branch: "DES-2834"}}); got != "Dev" {
		t.Fatalf("got %q", got)
	}
	if got := projectEnvLabel(&core.Project{Git: &core.GitInfo{Branch: "main"}}); got != "Prod" {
		t.Fatalf("got %q", got)
	}
}
