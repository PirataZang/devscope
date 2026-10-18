package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestDashboardShowsAndFiltersPending(t *testing.T) {
	a := responsiveApp(140, 40)
	// O fixture tem contagem no GitInfo; a medida do ciclo de pendências, quando
	// existe, é a que vale — limpo é limpo, mesmo com GitInfo velho dizendo ±7.
	for i := range a.snapshot.Projects {
		a.snapshot.Projects[i].Pending = &core.GitPending{}
	}
	a.snapshot.Projects[0].Pending = &core.GitPending{Changes: 3, Ahead: 2}
	a.snapshot.Projects[2].Pending = &core.GitPending{Behind: 1}
	got := stripANSI(a.renderDashboard())
	for _, want := range []string{"±3 ↑2", "↓1", "±2 pendentes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}

	a.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if n := len(a.filteredProjects()); n != 2 {
		t.Fatalf("p deixa só os 2 pendentes, ficaram %d", n)
	}
	if got := stripANSI(a.renderDashboard()); !strings.Contains(got, "só pendentes") {
		t.Fatalf("o filtro ligado tem de aparecer:\n%s", got)
	}
	a.updateDashboard(tea.KeyMsg{Type: tea.KeyEsc})
	if a.pendingOnly {
		t.Fatal("esc desliga o filtro")
	}
}
