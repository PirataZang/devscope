package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

func TestActionsBoxAlignsKeysAndFitsWidth(t *testing.T) {
	box := renderActionsBox(22, 40,
		[2]string{"↑↓", "navegar"},
		[2]string{"enter", "abrir"},
		[2]string{"space", "checkout"},
		[2]string{"←→", "painéis"},
	)
	plain := stripANSI(box)
	if !strings.Contains(plain, "AÇÕES") {
		t.Fatalf("missing title: %q", plain)
	}
	for _, want := range []string{"navegar", "abrir", "checkout", "painéis"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, plain)
		}
	}
	// Não deve virar torre vazia do tamanho pedido (40).
	if lipgloss.Height(box) > 12 {
		t.Fatalf("box too tall: %d\n%s", lipgloss.Height(box), plain)
	}
	if lipgloss.Width(box) > 22 {
		t.Fatalf("box wider than requested: %d", lipgloss.Width(box))
	}
	// Truncate ANSI-safe: descrição longa não deixa lixo tipo ":16".
	narrow := stripANSI(renderActionsBox(16, 10,
		[2]string{"space", "checkout-branch-longa"},
	))
	if strings.Contains(narrow, ":16") {
		t.Fatalf("truncated mangled line: %q", narrow)
	}
}

func TestActionsCmdWidthWiderOnLargeScreens(t *testing.T) {
	if actionsCmdWidth(50) != 0 {
		t.Fatal("narrow pane should omit AÇÕES")
	}
	if w := actionsCmdWidth(100); w < 20 {
		t.Fatalf("expected wider actions column, got %d", w)
	}
}

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
		for _, want := range []string{"Projeto", "DETALHES", "AÇÕES"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%s missing %q in:\n%s", name, want, view)
			}
		}
	}
}
