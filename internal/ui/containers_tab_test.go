package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

func TestRestoreContainerCursorKeepsShowAllSelection(t *testing.T) {
	p1 := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "a1", Name: "alpha-1", Status: "running", ProjectPath: "/apps/one"},
			{ID: "a2", Name: "alpha-2", Status: "running", ProjectPath: "/apps/one"},
		},
	}
	p2 := core.Project{
		Path: "/apps/two", Name: "beta",
		Containers: []core.Container{
			{ID: "b1", Name: "beta-1", Status: "running", ProjectPath: "/apps/two"},
		},
	}
	a := &App{
		containerShowAll:   true,
		selectedProject:    &p1,
		snapshot:           core.Snapshot{Projects: []core.Project{p1, p2}},
		containerPreviewID: "b1",
		tabCursor:          0, // wrong index after a bad clamp
	}
	a.restoreContainerCursor("b1")
	if a.tabCursor != 2 {
		t.Fatalf("expected cursor on beta-1 (index 2), got %d", a.tabCursor)
	}
	// Simulates old bug: clamp against current project only.
	a.tabCursor = clampCursor(99, len(p1.Containers))
	if a.tabCursor != 1 {
		t.Fatalf("precondition: clamp to current project last=%d", a.tabCursor)
	}
	a.restoreContainerCursor("b1")
	if a.tabCursor != 2 {
		t.Fatalf("restore should recover show-all selection, got %d", a.tabCursor)
	}
}

func TestContainerRestartAlwaysIndicator(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", Restart: "always", ProjectPath: "/apps/one"},
			{ID: "c2", Name: "db", Status: "running", Restart: "no", ProjectPath: "/apps/one"},
		},
	}
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	if !containerRestartAlways(p.Containers[0]) || containerRestartAlways(p.Containers[1]) {
		t.Fatal("always detector")
	}
	got := stripANSI(a.renderContainerList(&p))
	if !strings.Contains(got, "∞always") {
		t.Fatalf("STATE should show ∞always:\n%s", truncate(got, 400))
	}
	if !strings.Contains(got, "∞ web") {
		t.Fatalf("missing always marker on web name:\n%s", truncate(got, 400))
	}
	if strings.Contains(got, "∞ db") {
		t.Fatal("db should not show always marker")
	}
	if !strings.Contains(got, "S-R") || !strings.Contains(strings.ToLower(got), "always") {
		t.Fatalf("AÇÕES should list S-R always:\n%s", truncate(got, 400))
	}
}

func TestShiftRSetsRestartAlwaysOnContainers(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", ProjectPath: "/apps/one"},
		},
		HasDockerCompose: true,
	}
	store := core.NewStateStore(nil)
	store.SetProjects([]core.Project{p})
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p, snapshot: store.Get(), store: store,
		cfg: &config.Config{},
	}
	// Terminals send Shift+R as "R" — must not fall through to compose restart.
	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("R on containers should start restart=always action")
	}
	if a.containerActionKind("web") != "always" {
		t.Fatalf("expected always action pending, got %q", a.containerActionKind("web"))
	}
}

func TestShiftRTogglesRestartAlwaysOff(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", Restart: "always", ProjectPath: "/apps/one"},
		},
	}
	store := core.NewStateStore(nil)
	store.SetProjects([]core.Project{p})
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p, snapshot: store.Get(), store: store,
		cfg: &config.Config{},
	}
	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("R on always container should clear policy")
	}
	if a.containerActionKind("web") != "no-always" {
		t.Fatalf("expected no-always action, got %q", a.containerActionKind("web"))
	}
}

func TestContainersBottomSurvivesDirtyDockerLogs(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", ProjectPath: "/apps/one"},
		},
	}
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
		containerPreviewID: "c1",
		// \r + ANSI + tab are what smash JoinHorizontal in real docker logs.
		containerPreviewLogs: "boot\r\x1b[31mERR\x1b[0m\tCould not find 'bundler'\nReady to run Vite...",
		containerPreviewVolumes: []string{
			"/home/igor/Área de trabalho/projetos/digiliza-chat-v2/packs",
			"/home/igor/Área de trabalho/projetos/digiliza-chat-v2/cache",
		},
	}
	bottom := a.renderContainersBottom(100, 10)
	if w := lipgloss.Width(bottom); w > 102 {
		t.Fatalf("bottom width %d exceeds pane (dirty docker output leaked)", w)
	}
	plain := stripANSI(bottom)
	if strings.Contains(plain, "\r") || strings.Contains(plain, "\t") {
		t.Fatal("control chars must be sanitized before render")
	}
	if !strings.Contains(plain, "LOGS") || !strings.Contains(plain, "PORTAS") || !strings.Contains(plain, "AÇÕES") {
		t.Fatalf("bottom panels missing:\n%s", truncate(plain, 300))
	}
}

func TestContainerPortsSubviewListsAndOpensPreview(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", Ports: "0.0.0.0:3000->3000/tcp, :::5173->5173/tcp", ProjectPath: "/apps/one"},
		},
	}
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if a.containerSubview != containerSubviewPorts {
		t.Fatalf("enter should open ports view, got %v", a.containerSubview)
	}
	got := stripANSI(a.renderContainerPorts(&p))
	if !strings.Contains(got, ":3000") || !strings.Contains(got, ":5173") {
		t.Fatalf("ports missing:\n%s", truncate(got, 400))
	}
	if cmd == nil {
		// two ports → no auto preview; enter on selected loads it
		_, cmd = a.handleContainerPortsKeys(tea.KeyMsg{Type: tea.KeyEnter}, &p)
	}
	if cmd == nil {
		t.Fatal("enter on port should load preview")
	}
}

func TestContainerOnlyDockerHidesMissing(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", ProjectPath: "/apps/one"},
			{ID: "c2", Name: "worker", Status: "exited", ProjectPath: "/apps/one"},
			{Name: "db", Status: "missing", Image: "compose", ProjectPath: "/apps/one"},
		},
	}
	a := &App{
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		containerOnlyDocker: true,
	}
	got := a.filteredContainers(&p)
	if len(got) != 2 {
		t.Fatalf("expected 2 docker instances, got %+v", got)
	}
	for _, c := range got {
		if c.Status == "missing" || c.ID == "" {
			t.Fatalf("missing leaked: %+v", c)
		}
	}
}

func TestContainersActionsBoxListsAllShortcuts(t *testing.T) {
	a := &App{containerOnlyDocker: true, containerShowAll: false}
	items := a.containerActionItems()
	if len(items) < 12 {
		t.Fatalf("too few actions: %d", len(items))
	}
	box := renderContainersActionsBox(30, 8, items...)
	plain := stripANSI(box)
	for _, key := range []string{"enter", "m", "v", "S-U", "S-D", "g", "/"} {
		if !strings.Contains(plain, key) {
			t.Fatalf("missing action %q in:\n%s", key, truncate(plain, 500))
		}
	}
}

func TestContainerPreviewPortLines(t *testing.T) {
	p := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", Ports: "127.0.0.1:8080->80/tcp", ProjectPath: "/apps/one"},
		},
	}
	a := &App{selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}}}
	lines := a.containerPreviewPortLines(5, 40)
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, ":8080") {
		t.Fatalf("expected host port in bottom panel:\n%s", plain)
	}
}

func TestContainerShowAllIncludesProjectColumn(t *testing.T) {
	p1 := core.Project{
		Path: "/apps/one", Name: "alpha-app",
		Containers: []core.Container{
			// ProjectPath is compose cwd (subdir), not the project root — common in docker ps.
			{ID: "1", Name: "one-web", Image: "nginx", Status: "running", ProjectPath: "/apps/one/docker"},
		},
	}
	p2 := core.Project{
		Path: "/apps/two", Name: "beta-app",
		Containers: []core.Container{
			{ID: "2", Name: "two-db", Image: "postgres", Status: "running", ProjectPath: "/apps/two/compose"},
		},
	}
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		containerShowAll: true,
		selectedProject:  &p1,
		snapshot:         core.Snapshot{Projects: []core.Project{p1, p2}},
	}
	got := stripANSI(a.renderContainerList(&p1))
	for _, want := range []string{"TODOS + ÓRFÃOS", "PROJECT", "alpha-app", "beta-app", "one-web", "two-db"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, truncate(got, 400))
		}
	}
	if strings.Contains(got, "/apps/one") || strings.Contains(got, "docker") && strings.Contains(got, "PROJECT") {
		// path must not appear as the PROJECT cell value
		if strings.Contains(got, "/apps/") {
			t.Fatalf("PROJECT column should show names, not paths:\n%s", truncate(got, 400))
		}
	}
	a.containerShowAll = false
	only := stripANSI(a.renderContainerList(&p1))
	if strings.Contains(only, "two-db") {
		t.Fatal("project filter should hide other project containers")
	}
	if strings.Contains(only, "PROJECT") {
		t.Fatal("project column only in show-all mode")
	}
}
