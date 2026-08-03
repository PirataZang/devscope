package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/sshutil"
)

func TestAllTabsIncludesSSH(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabSSH {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabSSH missing")
	}
	if TabSSH.String() != "SSH Tunnel" {
		t.Fatalf("String=%q", TabSSH.String())
	}
}

func TestSSHLandingAndOpen(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{3000}}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterSSHTab(&p)
	landing := stripANSI(a.renderSSHLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "SSH") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.sshOpen {
		t.Fatal("enter should open client")
	}
	view := stripANSI(a.renderSSHTab(&p))
	for _, want := range []string{"devscope", "ssh", "TUNNELS", "DETALHES", "LOGS", "AÇÕES", "novo túnel", "start"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestSSHSidebarKey(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabSSH}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "SSH Tunnel") {
		t.Fatalf("sidebar missing SSH Tunnel: %q", got)
	}
}

func TestSSHWizardCyclesMode(t *testing.T) {
	p := core.Project{Path: "/p", Name: "digiliza", Ports: []int{3000}}
	a := &App{width: 100, height: 36, sshOpen: true}
	a.beginSSHWizard(&p)
	if !a.sshWizard || a.sshNewLocalPortStr == "" {
		t.Fatalf("wizard not ready: portStr=%q", a.sshNewLocalPortStr)
	}
	if a.sshNewMode != sshutil.ModeRemote {
		t.Fatalf("default mode=%q want remote", a.sshNewMode)
	}
	if a.sshNewLocalPortStr != "3000" || a.sshNewBind != "127.0.0.1:3000" {
		t.Fatalf("project port seed: port=%q bind=%q", a.sshNewLocalPortStr, a.sshNewBind)
	}
	a.sshWizardFocusField(sshWizMode)
	_, _ = a.updateSSHWizard(tea.KeyMsg{Type: tea.KeySpace}, &p)
	if a.sshNewMode != sshutil.ModeLocal {
		t.Fatalf("space → local, got %q", a.sshNewMode)
	}
	_, _ = a.updateSSHWizard(tea.KeyMsg{Type: tea.KeySpace}, &p)
	if a.sshNewMode != sshutil.ModeDynamic {
		t.Fatalf("space → dynamic, got %q", a.sshNewMode)
	}

	a.sshWizardFocusField(sshWizLocalPort)
	a.sshNewLocalPortStr = "80"
	a.sshWizardCursor = 2
	_, _ = a.updateSSHWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'8'}}, &p)
	if a.sshNewLocalPortStr != "808" {
		t.Fatalf("type digit: got %q", a.sshNewLocalPortStr)
	}

	view := stripANSI(a.renderSSHWizard(&p, 70, 28))
	if !strings.Contains(view, "target") || !strings.Contains(view, "digiliza") {
		t.Fatalf("wizard: %q", view)
	}
}

func TestSSHTunnelsRender(t *testing.T) {
	a := &App{
		width: 120, height: 40, sshOpen: true, sshSubTab: sshTabTunnels,
		sshTunnels: []sshutil.Tunnel{
			{Name: "db", Project: "digiliza", LocalPort: 5433, Mode: "local", Target: "u@h", Status: "online", Forward: "L :5433 → 127.0.0.1:5432"},
			{Name: "api", Project: "digiliza", LocalPort: 3000, Mode: "local", Target: "u@h", Status: "offline"},
		},
		sshCfg: sshutil.ProjectConfig{Project: "digiliza"},
	}
	view := stripANSI(a.renderSSHTab(&core.Project{Name: "digiliza"}))
	if !strings.Contains(view, "db") || !strings.Contains(view, "api") {
		t.Fatalf("tunnels missing:\n%s", view)
	}
	if !strings.Contains(view, "5433") {
		t.Fatalf("port missing:\n%s", view)
	}
}

func TestSSHTunnelFromWizard(t *testing.T) {
	a := &App{
		sshNewName: "db", sshNewMode: sshutil.ModeLocal,
		sshNewLocalPortStr: "5433", sshNewBind: "127.0.0.1:5432",
		sshNewTarget: "deploy@vps", sshNewIdentity: "~/.ssh/id_ed25519",
	}
	cfg, err := a.sshTunnelFromWizard()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "db" || cfg.LocalPort != 5433 || cfg.RemotePort != 5432 || cfg.Target != "deploy@vps" {
		t.Fatalf("%+v", cfg)
	}
}
