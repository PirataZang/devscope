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
		"VISÃO GERAL", "/digiliza", // barra do módulo identifica módulo + projeto
		"STACK & RUNTIME", "CONTAINERS", "SAÚDE", "ATIVIDADE", "GIT", "AÇÕES",
		"Laravel", "Vue", "DES-2834", ":3001",
		"1 problema", // faixa de alerta: probe API falhando
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in:\n%s", want, view)
		}
	}
	// O mesmo dado não pode voltar a aparecer em três caixas, nem placeholder.
	for _, gone := range []string{"DETALHES", "NOTAS", "notas locais", "Ambiente", "Servidor"} {
		if strings.Contains(view, gone) {
			t.Fatalf("overview deveria ter perdido %q:\n%s", gone, view)
		}
	}
}

// Sem problema nenhum a faixa de alerta não ocupa linha alguma.
func TestOverviewAlertOnlyWhenBroken(t *testing.T) {
	healthy := core.Project{
		Name: "ok", Path: "/p/ok", Status: core.StatusRunning, Health: core.HealthHealthy,
		HealthChecks: []core.HealthCheckResult{{URL: "https://ok/health", Status: core.HealthHealthy}},
	}
	a := &App{width: 120, height: 42, tab: TabOverview, selectedProject: &healthy,
		snapshot: core.Snapshot{Projects: []core.Project{healthy}}}
	if got := a.renderOverviewAlert(&healthy, 120); got != "" {
		t.Fatalf("sem problema a faixa deve sumir: %q", stripANSI(got))
	}
	if strings.Contains(stripANSI(a.renderOverviewTab(&healthy)), "⚠") {
		t.Fatal("faixa de alerta não deveria aparecer")
	}

	broken := healthy
	broken.Containers = []core.Container{{Name: "api", State: "restarting", Status: "Restarting (1)"}}
	got := stripANSI(a.renderOverviewAlert(&broken, 120))
	if !strings.Contains(got, "api reiniciando") || !strings.Contains(got, "1 problema") {
		t.Fatalf("faixa deve nomear o problema: %q", got)
	}
}

// Caixa de 2 linhas não pode virar torre de 13 — a sobra vai para CONTAINERS.
func TestOverviewBoxesFitTheirContent(t *testing.T) {
	p := core.Project{
		Name: "app", Path: "/p/app", Status: core.StatusRunning,
		Framework: core.FrameworkInfo{Name: "Go"},
		Containers: []core.Container{
			{Name: "api", Image: "app:1", State: "running", CPU: 1},
			{Name: "db", Image: "postgres:16", State: "running", CPU: 2},
		},
	}
	a := &App{width: 140, height: 50, tab: TabOverview, selectedProject: &p,
		snapshot: core.Snapshot{Projects: []core.Project{p}}}
	lines := strings.Split(stripANSI(a.renderOverviewTab(&p)), "\n")

	// A caixa STACK & RUNTIME tem 4 linhas de conteúdo (stack, docker, cpu, ram).
	start := -1
	for i, l := range lines {
		if strings.Contains(l, "STACK & RUNTIME") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("caixa STACK & RUNTIME não encontrada")
	}
	end := start
	for end < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[end+1]), "└") {
		end++
	}
	if h := end - start; h > 6 {
		t.Fatalf("STACK & RUNTIME esticou para %d linhas de conteúdo", h)
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
