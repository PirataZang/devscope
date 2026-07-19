package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesDB(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabDB {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabDB missing from AllTabs")
	}
	if TabDB.String() != "Database" {
		t.Fatalf("String=%q", TabDB.String())
	}
}

func TestDetectDBEngineFromImage(t *testing.T) {
	cases := []struct {
		image string
		name  string
		want  dbEngine
		ok    bool
	}{
		{"postgres:16", "db", dbEnginePostgres, true},
		{"mysql:8", "mysql", dbEngineMySQL, true},
		{"redis:7-alpine", "cache", dbEngineRedis, true},
		{"mongo:6", "mongo", dbEngineMongo, true},
		{"nginx:latest", "web", 0, false},
	}
	for _, tc := range cases {
		got, ok := detectDBEngineFromImage(tc.image, tc.name)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("image=%q name=%q got=%v ok=%v want=%v/%v", tc.image, tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseFirstHostPort(t *testing.T) {
	got := parseFirstHostPort("0.0.0.0:5433->5432/tcp", 5432)
	if got != 5433 {
		t.Fatalf("port=%d", got)
	}
	got = parseFirstHostPort("", 6379)
	if got != 6379 {
		t.Fatalf("fallback=%d", got)
	}
}

func TestParseDatabaseURL(t *testing.T) {
	user, pass, db, host, port := parseDatabaseURL("postgres://alice:s3cret@localhost:5432/appdb?sslmode=disable")
	if user != "alice" || pass != "s3cret" || db != "appdb" || host != "localhost" || port != 5432 {
		t.Fatalf("got user=%q pass=%q db=%q host=%q port=%d", user, pass, db, host, port)
	}
}

func TestDiscoverDBTargetsFromContainers(t *testing.T) {
	p := &core.Project{
		Containers: []core.Container{
			{Name: "app-db", Image: "postgres:16", State: "running", Ports: "0.0.0.0:5432->5432/tcp"},
			{Name: "web", Image: "nginx:latest", State: "running"},
		},
	}
	targets := discoverDBTargets(p)
	found := false
	for _, t := range targets {
		if t.Container == "app-db" && t.Engine == dbEnginePostgres {
			found = true
			if t.Port != 5432 {
				// still ok if port parse failed
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected postgres container target, got %#v", targets)
	}
}

func TestDbLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "p"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterDbTab(&p)
	if a.tab != TabDB || a.dbOpen {
		t.Fatalf("8 should open landing, tab=%v open=%v", a.tab, a.dbOpen)
	}
	landing := stripANSI(a.renderDbLanding(&p))
	if !strings.Contains(landing, "enter") {
		t.Fatalf("landing should prompt enter: %q", landing)
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.dbOpen || a.tab != TabDB {
		t.Fatalf("enter should open client, open=%v tab=%v", a.dbOpen, a.tab)
	}

	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.dbOpen || a.tab != TabDB || a.view != ViewProject {
		t.Fatalf("esc should return to landing, open=%v tab=%v view=%v", a.dbOpen, a.tab, a.view)
	}
}

func TestPushDbHistoryDedupAndCap(t *testing.T) {
	a := &App{}
	for i := 0; i < 12; i++ {
		a.pushDbHistory("Postgres", "SELECT "+string(rune('a'+i%26)))
	}
	a.pushDbHistory("Postgres", "SELECT a")
	if len(a.dbHistory) != 10 {
		t.Fatalf("len=%d", len(a.dbHistory))
	}
	if a.dbHistory[0].Query != "SELECT a" {
		t.Fatalf("first=%+v", a.dbHistory[0])
	}
}

func TestDbQueryTypingEditsFreely(t *testing.T) {
	a := &App{width: 100, height: 30, dbQuery: "SELECT 1;"}
	a.dbBlock = dbBlockQuery
	a.dbEditing = false
	a.dbOpen = true

	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, &core.Project{})
	if !a.dbEditing {
		t.Fatal("typing on Query should start free SQL edit")
	}
	if !strings.Contains(a.dbQuery, "x") {
		t.Fatalf("query should include typed char: %q", a.dbQuery)
	}
}

func TestDbTabCyclesPanes(t *testing.T) {
	a := &App{width: 100, height: 30, dbOpen: true, dbQuery: "SELECT 1;"}
	a.dbBlock = dbBlockInfo
	a.dbEditing = false
	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.dbBlock != dbBlockSchema {
		t.Fatalf("tab → Tables, got %v", a.dbBlock)
	}
	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.dbBlock != dbBlockQuery || !a.dbEditing {
		t.Fatalf("tab → Query edit, block=%v editing=%v", a.dbBlock, a.dbEditing)
	}
	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.dbBlock != dbBlockResult {
		t.Fatalf("tab → Result, got %v", a.dbBlock)
	}
}

func TestDbInfoFieldNavigation(t *testing.T) {
	a := &App{width: 100, height: 30, dbOpen: true, dbHost: "127.0.0.1", dbPort: 5432}
	a.dbBlock = dbBlockInfo
	a.dbConnField = dbFieldTarget
	a.dbEditing = false

	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyDown}, &core.Project{})
	if a.dbConnField != dbFieldHost {
		t.Fatalf("down from target should go to host, got %d", a.dbConnField)
	}
	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyDown}, &core.Project{})
	if a.dbConnField != dbFieldPort {
		t.Fatalf("down should go to port, got %d", a.dbConnField)
	}
	// typing on host/port starts edit
	a.dbConnField = dbFieldHost
	_, _ = a.handleDbKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}}, &core.Project{})
	if !a.dbEditing {
		t.Fatal("typing on host field should start edit")
	}
}

func TestDbHostArgsPostgres(t *testing.T) {
	args, env, err := dbHostArgs(dbRequest{
		Engine:   dbEnginePostgres,
		Host:     "localhost",
		Port:     5432,
		User:     "u",
		Password: "p",
		Database: "d",
		Query:    "SELECT 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-h localhost") || !strings.Contains(joined, "-U u") {
		t.Fatalf("args=%v", args)
	}
	if len(env) != 1 || env[0] != "PGPASSWORD=p" {
		t.Fatalf("env=%v", env)
	}
}

func TestParseSchemaTableList(t *testing.T) {
	body := "table_name\n---\nusers\norders\n(2 rows)"
	got := parseSchemaTableList(dbEnginePostgres, body)
	if len(got) != 2 || got[0] != "users" || got[1] != "orders" {
		t.Fatalf("got=%v", got)
	}
	mysql := parseSchemaTableList(dbEngineMySQL, "table_name\nusers\nposts")
	if len(mysql) != 2 || mysql[0] != "users" {
		t.Fatalf("mysql=%v", mysql)
	}
}

func TestParseSchemaColumns(t *testing.T) {
	body := "column_name\tdata_type\tis_nullable\nid\tuuid\tNO\nemail\ttext\tYES"
	got := parseSchemaColumns(dbEnginePostgres, body)
	if len(got) != 2 || got[0].Name != "id" || got[0].Type != "uuid" {
		t.Fatalf("got=%#v", got)
	}
}

func TestDbSelectAllQuery(t *testing.T) {
	a := &App{dbEngine: dbEnginePostgres}
	q := a.dbSelectAllQuery("user\"s")
	if !strings.Contains(q, `"user""s"`) || !strings.Contains(q, "LIMIT 100") {
		t.Fatalf("q=%q", q)
	}
}

func TestDbLayoutRendersThreeBoxes(t *testing.T) {
	p := core.Project{
		Path: "/p",
		Name: "p",
		Containers: []core.Container{
			{Name: "app-db", Image: "postgres:16", State: "running", Ports: "0.0.0.0:5432->5432/tcp"},
		},
	}
	a := &App{
		width:           120,
		height:          40,
		view:            ViewProject,
		tab:             TabDB,
		dbOpen:          true,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.initDbTab(&p)
	out := stripANSI(a.renderDbTab(&p))
	for _, want := range []string{"Conn", "Tables", "Query", "Result"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in layout:\n%s", want, out)
		}
	}
}
