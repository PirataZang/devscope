package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesNginx(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabNginx {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabNginx missing")
	}
	if TabNginx.String() != "Nginx" {
		t.Fatalf("String=%q", TabNginx.String())
	}
}

func newNginxProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte("include sites/*.inc;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sitesDir := filepath.Join(dir, "sites")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	site := `server {
    listen 80;
    server_name api.example.com;
    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
`
	if err := os.WriteFile(filepath.Join(sitesDir, "api.inc"), []byte(site), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNginxLandingAndOpen(t *testing.T) {
	dir := newNginxProjectDir(t)
	p := core.Project{Path: dir, Name: "vps-nginx"}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterNginxTab(&p)
	landing := stripANSI(a.renderNginxLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "NGINX") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.nginxOpen {
		t.Fatal("enter should open client")
	}
	if cmd := a.refreshNginx(&p); cmd != nil {
		msg := cmd()
		a.handleNginxMsg(msg)
	}
	if len(a.nginxSites) != 1 || a.nginxSites[0].Name != "api" {
		t.Fatalf("sites: %+v", a.nginxSites)
	}
	view := stripANSI(a.renderNginxTab(&p))
	for _, want := range []string{"NGINX", "1 ROTAS", "2 ARQUIVO", "api", "127.0.0.1:3000"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestNginxCreateConfSingleViaWizard(t *testing.T) {
	dir := newNginxProjectDir(t)
	p := core.Project{Path: dir, Name: "vps-nginx"}
	a := &App{width: 120, height: 40, view: ViewProject, tab: TabNginx, selectedProject: &p}
	a.openNginxClient(&p)
	a.beginNginxConfWizard()
	a.nginxNewName = "static"
	a.nginxNewServerName = "static.example.com"
	a.nginxNewRoot = "/var/www/static"

	cmd := a.nginxCreateConf(&p)
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd()
	if am, ok := msg.(nginxActionMsg); !ok || am.err != "" {
		t.Fatalf("create failed: %+v", msg)
	}
	if _, err := os.Stat(filepath.Join(dir, "sites", "static.inc")); err != nil {
		t.Fatalf("file not written: %v", err)
	}
}

// TestNginxHubDrillDownCreateAndDeleteInc cobre o fluxo pedido: criar um
// .conf hub, apertar enter pra entrar nele, cadastrar uma .inc lá dentro,
// deletar, e voltar com esc sem sair do tab.
func TestNginxHubDrillDownCreateAndDeleteInc(t *testing.T) {
	dir := newNginxProjectDir(t)
	p := core.Project{Path: dir, Name: "vps-nginx"}
	a := &App{width: 120, height: 40, view: ViewProject, tab: TabNginx, selectedProject: &p}
	a.openNginxClient(&p)

	a.beginNginxConfWizard()
	a.nginxNewName = "apihub"
	a.nginxNewKind = "hub"
	a.nginxNewServerName = "178.104.78.64"
	a.nginxNewHubDirName = "apihub"
	msg := a.nginxCreateConf(&p)()
	if am, ok := msg.(nginxActionMsg); !ok || am.err != "" {
		t.Fatalf("create hub failed: %+v", msg)
	}
	_, cmd0 := a.handleNginxMsg(msg)
	if cmd0 == nil {
		t.Fatal("expected refreshNginx after creating hub")
	}
	a.handleNginxMsg(cmd0())

	// acha o hub recém-criado na lista e entra nele com enter.
	hubIdx := -1
	for i, s := range a.nginxSites {
		if s.Name == "apihub" {
			hubIdx = i
		}
	}
	if hubIdx < 0 {
		t.Fatalf("hub not discovered: %+v", a.nginxSites)
	}
	a.nginxCursor = hubIdx
	_, cmd := a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyEnter}, &p)
	if cmd == nil {
		t.Fatal("expected refreshNginxIncs command after drilling in")
	}
	a.handleNginxMsg(cmd())
	if a.nginxHub == nil || a.nginxHub.Name != "apihub" {
		t.Fatalf("expected drilled into apihub: %+v", a.nginxHub)
	}

	a.beginNginxIncWizard()
	a.nginxNewPath = "/api/users"
	a.nginxNewTarget = "7001"
	msg = a.nginxCreateInc()()
	if am, ok := msg.(nginxActionMsg); !ok || am.err != "" {
		t.Fatalf("create inc failed: %+v", msg)
	}
	_, cmd1 := a.handleNginxMsg(msg)
	if cmd1 == nil {
		t.Fatal("expected refreshNginxIncs after creating inc")
	}
	a.handleNginxMsg(cmd1())
	if len(a.nginxIncs) != 1 || a.nginxIncs[0].Location != "/api/users/" || a.nginxIncs[0].ProxyPass != "http://127.0.0.1:7001" {
		t.Fatalf("incs=%+v", a.nginxIncs)
	}

	a.nginxCursor = 0
	msg = a.nginxDeleteSelected(&p)()
	if am, ok := msg.(nginxActionMsg); !ok || am.err != "" {
		t.Fatalf("delete inc failed: %+v", msg)
	}
	_, cmd2 := a.handleNginxMsg(msg)
	if cmd2 == nil {
		t.Fatal("expected refreshNginxIncs after deleting inc")
	}
	a.handleNginxMsg(cmd2())
	if len(a.nginxIncs) != 0 {
		t.Fatalf("expected inc deleted: %+v", a.nginxIncs)
	}

	// esc volta pro nível 1 sem fechar o tab.
	_, _ = a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.nginxHub != nil || !a.nginxOpen {
		t.Fatalf("esc should pop back to top level, not close the tab: hub=%v open=%v", a.nginxHub, a.nginxOpen)
	}
}

func TestNginxShowAllScansOtherProjects(t *testing.T) {
	dirA := newNginxProjectDir(t) // has sites/api.inc
	dirB := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirB, "main.conf"), []byte("include sites/*.inc;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sitesB := filepath.Join(dirB, "sites")
	if err := os.MkdirAll(sitesB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sitesB, "shop.inc"), []byte("server {\n    listen 80;\n    server_name shop.example.com;\n    root /var/www/shop;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pA := core.Project{Path: dirA, Name: "vps-nginx"}
	pB := core.Project{Path: dirB, Name: "shop-nginx"}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabNginx,
		selectedProject: &pA,
		snapshot:        core.Snapshot{Projects: []core.Project{pA, pB}},
	}
	a.openNginxClient(&pA)
	msg := a.refreshNginx(&pA)()
	a.handleNginxMsg(msg)
	if len(a.nginxSites) != 1 {
		t.Fatalf("default scope should stay project-only: %+v", a.nginxSites)
	}
	if a.nginxForeign != 1 {
		t.Fatalf("foreign count=%d want 1", a.nginxForeign)
	}

	a.nginxShowAll = true
	msg = a.refreshNginx(&pA)()
	a.handleNginxMsg(msg)
	if len(a.nginxSites) != 2 {
		t.Fatalf("show-all should include shop-nginx site: %+v", a.nginxSites)
	}
	foundForeign := false
	for _, s := range a.nginxSites {
		if s.Name == "shop" && s.Project == "shop-nginx" {
			foundForeign = true
		}
	}
	if !foundForeign {
		t.Fatalf("expected shop site tagged with its project: %+v", a.nginxSites)
	}
}

func TestNginxSidebarKey(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabNginx}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "Nginx") {
		t.Fatalf("sidebar missing Nginx: %q", got)
	}
}
