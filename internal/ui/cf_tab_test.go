package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/cfutil"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesCFTunnel(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabCFTunnel {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabCFTunnel missing")
	}
	if TabCFTunnel.String() != "CF Tunnel" {
		t.Fatalf("String=%q", TabCFTunnel.String())
	}
}

func TestCFLandingAndOpen(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{3000}}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterCFTab(&p)
	landing := stripANSI(a.renderCFLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "CLOUDFLARE") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.cfOpen {
		t.Fatal("enter should open client")
	}
	view := stripANSI(a.renderCFTab(&p))
	for _, want := range []string{"devscope", "cloudflare", "TUNNELS", "DETALHES", "LOGS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "NAV") || strings.Contains(view, "QUICK STATS") {
		t.Fatalf("old layout leftovers:\n%s", view)
	}
}

func TestCFSidebarKey(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabCFTunnel}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "CF Tunnel") {
		t.Fatalf("sidebar missing CF Tunnel: %q", got)
	}
}

func TestCFWizardEditsURLAsText(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{4321}}
	a := &App{width: 100, height: 30, cfOpen: true}
	a.beginCFWizard(&p)
	if a.cfNewURL != "http://127.0.0.1:3000" {
		t.Fatalf("default url should be 127.0.0.1:3000, got %q", a.cfNewURL)
	}

	a.cfWizardFocusField(cfWizURL)
	a.cfNewURL = "http://127.0.0.1:432"
	a.cfWizardCursor = len(a.cfNewURL)
	_, _ = a.updateCFWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, &p)
	if a.cfNewURL != "http://127.0.0.1:4321" {
		t.Fatalf("type rune: got %q", a.cfNewURL)
	}
	_, _ = a.updateCFWizard(tea.KeyMsg{Type: tea.KeyBackspace}, &p)
	if a.cfNewURL != "http://127.0.0.1:432" {
		t.Fatalf("backspace: got %q", a.cfNewURL)
	}

	a.cfWizardFocusField(cfWizMode)
	a.cfNewMode = "quick"
	_, _ = a.updateCFWizard(tea.KeyMsg{Type: tea.KeySpace}, &p)
	if a.cfNewMode != "named" {
		t.Fatalf("space should cycle mode to named, got %q", a.cfNewMode)
	}

	view := stripANSI(a.renderCFWizard(&p, 60, 14))
	if !strings.Contains(view, "fixo") || !strings.Contains(view, "digiliza") {
		t.Fatalf("project should be fixed in wizard: %q", view)
	}
}

func TestCFSetupScreen(t *testing.T) {
	a := &App{
		width: 100, height: 30, cfOpen: true, cfSubTab: cfTabSetup,
		cfAuth: cfutil.AuthInfo{CLI: true, Version: "2026.7.3", CertPath: "/home/u/.cloudflared/cert.pem"},
	}
	view := stripANSI(a.renderCFSetup(80, 20))
	for _, want := range []string{"INSTALL", "LOGIN", "NAMED", "QUICK", "I", "L"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in setup:\n%s", want, view)
		}
	}
}

func TestCFHintsIncludeInstallLogin(t *testing.T) {
	a := &App{cfOpen: true}
	h := a.cfHints()
	if !strings.Contains(h, "install") || !strings.Contains(h, "login") {
		t.Fatalf("hints: %q", h)
	}
}

func TestCFTunnelsViewLayout(t *testing.T) {
	a := &App{
		width: 120, height: 30, cfOpen: true, cfSubTab: cfTabTunnels,
		cfTunnels: []cfutil.Tunnel{
			{Name: "site", Project: "digiliza-site", Port: 4321, Mode: "quick", Hostname: "messaging-cal-ghz-kid.trycloudflare.com", Status: "online"},
		},
	}
	got := stripANSI(a.renderCFTunnelsView(&core.Project{Name: "digiliza-site"}, 120, 20))
	for _, want := range []string{"DETALHES", "LOGS", "TUNNELS", "site", "messaging-cal-ghz-kid"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "QUICK STATS") || strings.Contains(got, "NAV") {
		t.Fatalf("old layout leftovers:\n%s", got)
	}
}

func TestCFWizardIsModal(t *testing.T) {
	a := &App{
		width: 110, height: 32, cfOpen: true, cfWizard: true, cfSubTab: cfTabTunnels,
		cfNewName: "api", cfNewURL: "http://127.0.0.1:3000", cfNewMode: "quick",
	}
	view := stripANSI(a.renderCFTab(&core.Project{Name: "digiliza"}))
	for _, want := range []string{"CLOUDFLARE", "Novo túnel", "fixo", "digiliza", "preview", "TUNNELS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in wizard modal:\n%s", want, view)
		}
	}
}

func TestCFDeleteConfirmModal(t *testing.T) {
	a := &App{
		width: 100, height: 30, cfOpen: true, cfSubTab: cfTabTunnels,
		cfConfirmDelete: true,
		cfTunnels: []cfutil.Tunnel{
			{Name: "site", Mode: "quick", Hostname: "x.trycloudflare.com", Status: "online"},
		},
	}
	view := stripANSI(a.renderCFTab(&core.Project{Name: "demo"}))
	for _, want := range []string{"Excluir túnel", "site", "y confirma"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in delete modal:\n%s", want, view)
		}
	}
}
