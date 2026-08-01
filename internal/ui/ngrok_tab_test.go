package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/ngrokutil"
)

func TestAllTabsIncludesNgrok(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabNgrok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabNgrok missing")
	}
	if TabNgrok.String() != "Ngrok" {
		t.Fatalf("String=%q", TabNgrok.String())
	}
}

func TestNgrokLandingAndOpen(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{3000}}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterNgrokTab(&p)
	landing := stripANSI(a.renderNgrokLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "NGROK") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.ngrokOpen {
		t.Fatal("enter should open client")
	}
	view := stripANSI(a.renderNgrokTab(&p))
	for _, want := range []string{"devscope", "ngrok", "TUNNELS", "DETALHES", "LOGS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "NAV") || strings.Contains(view, "QUICK STATS") {
		t.Fatalf("old layout leftovers:\n%s", view)
	}
}

func TestNgrokSidebarKey(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabNgrok}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "Ngrok") {
		t.Fatalf("sidebar missing Ngrok: %q", got)
	}
	if strings.Contains(got, "1-9") || strings.Contains(got, " *") {
		t.Fatalf("sidebar should not show numeric shortcuts: %q", got)
	}
}

func TestNgrokWizardEditsPortAsText(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{3000}}
	a := &App{width: 100, height: 30, ngrokOpen: true}
	a.beginNgrokWizard(&p)
	if !a.ngrokWizard || a.ngrokNewPortStr == "" {
		t.Fatalf("wizard not ready: portStr=%q", a.ngrokNewPortStr)
	}

	a.ngrokWizardFocusField(ngrokWizPort)
	a.ngrokNewPortStr = "80"
	a.ngrokWizardCursor = 2
	_, _ = a.updateNgrokWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'8'}}, &p)
	if a.ngrokNewPortStr != "808" {
		t.Fatalf("type digit: got %q", a.ngrokNewPortStr)
	}
	_, _ = a.updateNgrokWizard(tea.KeyMsg{Type: tea.KeyBackspace}, &p)
	if a.ngrokNewPortStr != "80" {
		t.Fatalf("backspace: got %q", a.ngrokNewPortStr)
	}
	_, _ = a.updateNgrokWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, &p)
	if a.ngrokNewPortStr != "80" {
		t.Fatalf("letters must not enter port: got %q", a.ngrokNewPortStr)
	}

	a.ngrokWizardFocusField(ngrokWizProto)
	a.ngrokNewProto = "http"
	_, _ = a.updateNgrokWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}, &p)
	if a.ngrokNewProto != "http" {
		t.Fatalf("proto must not accept text: got %q", a.ngrokNewProto)
	}
	_, _ = a.updateNgrokWizard(tea.KeyMsg{Type: tea.KeySpace}, &p)
	if a.ngrokNewProto != "tcp" {
		t.Fatalf("space should cycle proto to tcp, got %q", a.ngrokNewProto)
	}

	view := stripANSI(a.renderNgrokWizard(&p, 60, 12))
	if !strings.Contains(view, "fixo") || !strings.Contains(view, "digiliza") {
		t.Fatalf("project should be fixed in wizard: %q", view)
	}
	if strings.Contains(view, "+") && strings.Contains(view, "porta") {
		t.Fatal("old +/- port UI should be gone")
	}
}

func TestNgrokTunnelsRender(t *testing.T) {
	a := &App{
		width: 120, height: 40, ngrokOpen: true, ngrokSubTab: ngrokTabTunnels,
		ngrokTunnels: []ngrokutil.Tunnel{
			{Name: "api", Project: "digiliza", Port: 3000, Proto: "http", Domain: "x.ngrok-free.app", Status: "online", PublicURL: "https://x.ngrok-free.app"},
			{Name: "admin", Project: "digiliza", Port: 8081, Proto: "http", Status: "offline"},
		},
		ngrokAgent: ngrokutil.AgentInfo{Connected: true, Version: "3.5.0"},
		ngrokCfg:   ngrokutil.ProjectConfig{Project: "digiliza", Region: "us"},
	}
	view := stripANSI(a.renderNgrokTab(&core.Project{Name: "digiliza"}))
	if !strings.Contains(view, "api") || !strings.Contains(view, "admin") {
		t.Fatalf("tunnels missing: %s", view)
	}
}

func TestNgrokTunnelsViewLayout(t *testing.T) {
	a := &App{
		width: 100, height: 28, ngrokOpen: true, ngrokSubTab: ngrokTabTunnels,
		ngrokTunnels: []ngrokutil.Tunnel{
			{Name: "api", Project: "digiliza-site", Port: 3000, Proto: "http", Domain: "x.ngrok-free.app", Status: "offline"},
		},
		ngrokCfg: ngrokutil.ProjectConfig{Project: "demo", Region: "us"},
	}
	got := stripANSI(a.renderNgrokTunnelsView(&core.Project{Name: "demo"}, 100, 18))
	for _, want := range []string{"DETALHES", "LOGS", "TUNNELS", "api", "x.ngrok-free.app"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "QUICK STATS") || strings.Contains(got, "NAV") {
		t.Fatalf("old layout leftovers:\n%s", got)
	}
}

func TestNgrokWizardIsModal(t *testing.T) {
	a := &App{
		width: 100, height: 30, ngrokOpen: true, ngrokWizard: true, ngrokSubTab: ngrokTabTunnels,
		ngrokNewName: "api", ngrokNewPortStr: "3000", ngrokNewProto: "http",
	}
	view := stripANSI(a.renderNgrokTab(&core.Project{Name: "digiliza"}))
	for _, want := range []string{"NGROK", "Novo túnel", "fixo", "digiliza", "preview", "TUNNELS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in wizard modal:\n%s", want, view)
		}
	}
}

func TestNgrokDeleteConfirmModal(t *testing.T) {
	a := &App{
		width: 100, height: 30, ngrokOpen: true, ngrokSubTab: ngrokTabTunnels,
		ngrokConfirmDelete: true,
		ngrokTunnels: []ngrokutil.Tunnel{
			{Name: "api", Port: 3000, Proto: "http", Domain: "x.ngrok.app", Status: "online"},
		},
	}
	view := stripANSI(a.renderNgrokTab(&core.Project{Name: "demo"}))
	for _, want := range []string{"Excluir túnel", "api", "y confirma"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in delete modal:\n%s", want, view)
		}
	}
}
