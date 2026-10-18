package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
)

func TestOverviewDiskBoxAndConfirm(t *testing.T) {
	a := responsiveApp(140, 44)
	a.view, a.tab = ViewProject, TabOverview
	p := a.selectedProject
	a.diskPath = p.Path
	a.diskTargets = []collectors.CleanTarget{
		{Name: "node_modules", Path: p.Path + "/node_modules", Size: 1300 << 20, Hint: "npm install"},
		{Name: ".next", Path: p.Path + "/.next", Size: 40 << 20, Hint: "next build"},
	}
	got := stripANSI(a.View())
	for _, want := range []string{"DISCO · 1.3 GB", "node_modules", "1.3 GB", "40 MB", "x limpar tudo"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}

	if _, ok := a.handleOverviewKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, p); !ok || !a.gitConfirmOn {
		t.Fatal("x pede confirmação")
	}
	box := stripANSI(a.View())
	for _, want := range []string{"Limpar 1.3 GB", "node_modules · .next", "npm install · next build"} {
		if !strings.Contains(box, want) {
			t.Fatalf("a confirmação diz o que apaga e como volta — faltou %q:\n%s", want, box)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{512: "512 B", 2048: "2 KB", 5 << 20: "5 MB", 3 << 29: "1.5 GB"} {
		if got := formatBytes(n); got != want {
			t.Errorf("%d: %q, queria %q", n, got, want)
		}
	}
}
