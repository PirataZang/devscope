package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesDatabase(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabDatabase {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabDatabase missing from AllTabs")
	}
	if TabDatabase.String() != "Database" {
		t.Fatalf("String=%q", TabDatabase.String())
	}
}

func TestDbLandingEnterAndEsc(t *testing.T) {
	p := core.Project{
		Path: "/p",
		Name: "p",
		Containers: []core.Container{
			{Name: "db", Image: "postgres:16", State: "running"},
		},
	}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterDbTab(&p)
	if a.tab != TabDatabase || a.dbOpen {
		t.Fatalf("8 should open landing, tab=%v open=%v", a.tab, a.dbOpen)
	}
	landing := stripANSI(a.renderDbLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "postgres") {
		t.Fatalf("landing missing prompt/target: %q", landing)
	}

	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.dbOpen || a.tab != TabDatabase {
		t.Fatalf("enter should open client, open=%v tab=%v", a.dbOpen, a.tab)
	}
	_ = cmd // may refresh tables async

	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.dbOpen || a.tab != TabDatabase || a.view != ViewProject {
		t.Fatalf("esc should return to tab 8 landing, open=%v tab=%v view=%v", a.dbOpen, a.tab, a.view)
	}
}

func TestSidebarShowsDatabaseTool(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabDatabase}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "TOOLS") || !strings.Contains(got, "Database") {
		t.Fatalf("sidebar missing Database tool: %q", got)
	}
	if !strings.Contains(got, "tab · shift+tab") {
		t.Fatalf("footer should mention tab · shift+tab: %q", got)
	}
}

func TestDbClientRenderAndFilter(t *testing.T) {
	p := &core.Project{Name: "demo", Path: "/p"}
	a := &App{
		width:      120,
		height:     40,
		dbOpen:     true,
		tab:        TabDatabase,
		dbTargets:  []collectors.DBTarget{{Label: "db", Engine: collectors.DBEnginePostgres, User: "u", Database: "app"}},
		dbTables:   []string{"users", "posts", "sessions"},
		dbSchema: collectors.DBTableInfo{
			Table: "users",
			Rows:  120,
			Columns: []collectors.DBColumn{
				{Name: "id", Type: "bigint", Nullable: "NO", Key: "PK"},
				{Name: "email", Type: "text", Nullable: "NO"},
			},
		},
		dbSQL:    "SELECT 1;",
		dbResult: " ?column? \n----------\n 1\n(1 row)",
	}
	a.dbResultRows = parseDBResultRows(a.dbResult)
	view := stripANSI(a.renderDbTab(p))
	for _, want := range []string{"TABELAS", "SCHEMA", "SQL", "RESULT", "users", "email", "postgres"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	a.dbFilter = "user"
	if got := a.filteredDbTables(); len(got) != 1 || got[0] != "users" {
		t.Fatalf("filter: %#v", got)
	}
	if a.dbResultRows != 1 {
		t.Fatalf("rows=%d", a.dbResultRows)
	}
}
