package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/routeutil"
)

func TestAllTabsIncludesRoutes(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabRoutes {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabRoutes missing from AllTabs")
	}
	if TabRoutes.String() != "Rotas" {
		t.Fatalf("String=%q", TabRoutes.String())
	}
	if int(TabRoutes) != 11 {
		t.Fatalf("TabRoutes index=%d want 11", TabRoutes)
	}
}

func TestRoutesLandingEnterAndEsc(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "openapi.json"), []byte(`{"paths":{"/ping":{"get":{}}}}`), 0o644)
	p := core.Project{Path: dir, Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterRoutesTab(&p)
	if a.tab != TabRoutes || a.routesOpen {
		t.Fatalf("landing: tab=%v open=%v", a.tab, a.routesOpen)
	}
	landing := stripANSI(a.renderRoutesLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "Rotas") {
		t.Fatalf("landing: %q", landing)
	}

	cmd := a.openRoutesClient(&p)
	if !a.routesOpen || cmd == nil {
		t.Fatalf("open client open=%v cmd=%v", a.routesOpen, cmd)
	}
	msg := cmd()
	loaded, ok := msg.(routesLoadedMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	a.handleRoutesLoaded(loaded)
	if len(a.routes) == 0 {
		t.Fatal("expected routes from openapi")
	}

	_, _ = a.handleRoutesKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.routesOpen || a.tab != TabRoutes {
		t.Fatalf("esc: open=%v tab=%v", a.routesOpen, a.tab)
	}
}

func TestOpenApiWithPreset(t *testing.T) {
	p := &core.Project{Path: "/p", Ports: []int{3000}}
	a := &App{apiAuthType: apiAuthBearer, apiAuthToken: "tok"}
	_ = a.openApiWithPreset(p, "POST", "api/users")
	if a.tab != TabAPI || !a.apiOpen {
		t.Fatalf("tab=%v open=%v", a.tab, a.apiOpen)
	}
	if a.apiMethod != "POST" {
		t.Fatalf("method=%q", a.apiMethod)
	}
	if a.apiURL != "http://localhost:3000/api/users" {
		t.Fatalf("url=%q", a.apiURL)
	}
	if a.apiAuthToken != "tok" {
		t.Fatal("auth should be preserved")
	}
	if a.apiBlock != apiBlockURL {
		t.Fatalf("block=%v", a.apiBlock)
	}
}

func TestRoutesEnterOpensAPI(t *testing.T) {
	p := &core.Project{Path: "/p", Ports: []int{8080}}
	a := &App{
		routesOpen:   true,
		tab:          TabRoutes,
		routes:       []routeutil.Route{{Method: "DELETE", Path: "/items/{id}", Source: "fastapi"}},
		routesCursor: 0,
	}
	_, cmd := a.handleRoutesKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if cmd != nil {
		_ = cmd()
	}
	if a.tab != TabAPI || !a.apiOpen || a.routesOpen {
		t.Fatalf("tab=%v apiOpen=%v routesOpen=%v", a.tab, a.apiOpen, a.routesOpen)
	}
	if a.apiMethod != "DELETE" || a.apiURL != "http://localhost:8080/items/{id}" {
		t.Fatalf("method=%q url=%q", a.apiMethod, a.apiURL)
	}
}

func TestRoutesFilterByPath(t *testing.T) {
	a := &App{
		routesOpen: true,
		tab:        TabRoutes,
		routes: []routeutil.Route{
			{Method: "GET", Path: "/api/users"},
			{Method: "POST", Path: "/api/users"},
			{Method: "GET", Path: "/api/posts"},
			{Method: "DELETE", Path: "/health"},
		},
	}
	a.routesFilterOn = true
	a.routesFilterInput = ""
	for _, ch := range "users" {
		_, _ = a.updateRoutesFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	vis := a.filteredRoutes()
	if len(vis) != 2 {
		t.Fatalf("filter users: got %d %#v", len(vis), vis)
	}
	_, _ = a.updateRoutesFilter(tea.KeyMsg{Type: tea.KeyEnter})
	if a.routesFilterOn || a.routesFilter != "users" {
		t.Fatalf("after enter: on=%v filter=%q", a.routesFilterOn, a.routesFilter)
	}

	_, _ = a.handleRoutesKeys(tea.KeyMsg{Type: tea.KeyEsc}, &core.Project{})
	if a.routesFilter != "" || !a.routesOpen {
		t.Fatalf("esc should clear filter and stay open: filter=%q open=%v", a.routesFilter, a.routesOpen)
	}
}

func TestSidebarShowsUtilsRoutes(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabRoutes}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "UTILS") || !strings.Contains(got, "Rotas") {
		t.Fatalf("sidebar missing UTILS/Rotas: %q", got)
	}
	if !strings.Contains(got, "tab · shift+tab") {
		t.Fatalf("footer should mention tab · shift+tab: %q", got)
	}
}
