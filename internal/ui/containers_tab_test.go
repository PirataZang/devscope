package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/mattn/go-runewidth"
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
	// A política de reinício mora no nome; a coluna ESTADO passou a mostrar o
	// estado do container, que é outra coisa.
	if !strings.Contains(got, "∞ web") {
		t.Fatalf("missing always marker on web name:\n%s", truncate(got, 400))
	}
	if strings.Contains(got, "∞ db") {
		t.Fatal("db should not show always marker")
	}
	// A barra de comandos larga precisa continuar oferecendo a troca de política.
	if !strings.Contains(got, "S-R") || !strings.Contains(strings.ToLower(got), "reinício") {
		t.Fatalf("barra de comandos deve listar S-R reinício:\n%s", truncate(got, 600))
	}
}

func TestShiftUStartsComposeUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := core.Project{
		Path: dir, Name: "alpha",
		HasDockerCompose: true,
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "exited", ProjectPath: dir},
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
	// Terminals send Shift+U as "U".
	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
	if cmd == nil {
		t.Fatal("U on containers should start compose up")
	}
	if a.statusMsg != "compose up…" && a.containerStatusMsg != "compose up…" {
		t.Fatalf("expected compose up feedback, status=%q container=%q", a.statusMsg, a.containerStatusMsg)
	}
}

func TestShiftDStartsComposeDownOnContainers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := core.Project{
		Path: dir, Name: "alpha",
		HasDockerCompose: true,
		DeployScript:     "make deploy",
		Containers: []core.Container{
			{ID: "c1", Name: "web", Status: "running", ProjectPath: dir},
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
	// Terminals send Shift+D as "D" — must not fall through to deploy confirm.
	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if cmd == nil {
		t.Fatal("D on containers should start compose down")
	}
	if a.deployConfirm {
		t.Fatal("D on containers must not open deploy confirm when compose exists")
	}
	if a.statusMsg != "compose down…" && a.containerStatusMsg != "compose down…" {
		t.Fatalf("expected compose down feedback, status=%q container=%q", a.statusMsg, a.containerStatusMsg)
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
		selectedProject:    &p,
		snapshot:           core.Snapshot{Projects: []core.Project{p}},
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
	// AÇÕES saiu do rodapé: virou barra larga de comandos no fim da tela.
	if !strings.Contains(plain, "LOGS") || !strings.Contains(plain, "PORTAS") {
		t.Fatalf("bottom panels missing:\n%s", truncate(plain, 300))
	}
	if strings.Contains(plain, "AÇÕES") {
		t.Fatalf("AÇÕES não deveria mais ocupar coluna no rodapé:\n%s", truncate(plain, 300))
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
	for _, want := range []string{"todos os projetos", "PROJETO", "alpha-app", "beta-app", "one-web", "two-db"} {
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

// O docker devolve o tempo em meia dúzia de formatos; a coluna tem 9 colunas.
func TestContainerUptimeLabelParsesDockerStatuses(t *testing.T) {
	cases := map[string]string{
		"Up 2 days":                     "2d",
		"Up 2 days (healthy)":           "2d",
		"Up 34 minutes (unhealthy)":     "34min",
		"Exited (0) 3 hours ago":        "3h",
		"Restarting (1) 12 seconds ago": "12s",
		"Up About a minute":             "1min",
		"":                              emDash,
	}
	for status, want := range cases {
		if got := containerUptimeLabel(core.Container{Status: status}); got != want {
			t.Fatalf("%q → %q, queria %q", status, got, want)
		}
	}
}

// A contagem comparava o texto do Status ("Up 2 days") com "running" e por
// isso dizia sempre "0 no ar".
func TestContainerCountsUseState(t *testing.T) {
	list := []core.Container{
		{Name: "a", State: "running", Status: "Up 2 days"},
		{Name: "b", State: "running", Status: "Up 2 days", Health: "unhealthy"},
		{Name: "c", State: "restarting", Status: "Restarting (1) 3 seconds ago"},
		{Name: "d", State: "exited", Status: "Exited (0) 1 hour ago"},
		{Name: "e", State: "paused", Status: "Paused"},
	}
	running, restarting, unhealthy, stopped, paused := containerCounts(list)
	if running != 1 || restarting != 1 || unhealthy != 1 || stopped != 1 || paused != 1 {
		t.Fatalf("run=%d rest=%d unh=%d stop=%d paus=%d", running, restarting, unhealthy, stopped, paused)
	}
}

// Cada estado precisa de palavra E cor próprias: só o pulso não distingue
// exited de paused, nem created de exited.
func TestContainerStateVisualIsDistinctPerState(t *testing.T) {
	a := &App{animFrame: 3}
	cases := []struct {
		c     core.Container
		label string
	}{
		{core.Container{State: "running", Status: "Up 2 days"}, "running"},
		{core.Container{State: "running", Status: "Up 2 days", Health: "unhealthy"}, "unhealthy"},
		{core.Container{State: "restarting", Status: "Restarting (1) 3 seconds ago"}, "restarting"},
		{core.Container{State: "paused", Status: "Paused"}, "paused"},
		{core.Container{State: "created", Status: "Created"}, "created"},
		{core.Container{State: "exited", Status: "Exited (0) 1 hour ago"}, "exited"},
	}
	seenLabel := map[string]bool{}
	seenLook := map[string]bool{}
	for _, tc := range cases {
		wave, waveStyle, label, style := a.containerStateVisual(tc.c)
		_ = waveStyle
		if label != tc.label {
			t.Fatalf("%q → %q, queria %q", tc.c.State, label, tc.label)
		}
		if seenLabel[label] {
			t.Fatalf("rótulo repetido: %q", label)
		}
		seenLabel[label] = true
		// glifo + cor juntos têm que ser únicos, senão dois estados se confundem
		look := fmt.Sprintf("%s/%v", wave, style.GetForeground())
		if seenLook[look] {
			t.Fatalf("estado %q não se distingue visualmente dos outros: %q", label, look)
		}
		seenLook[look] = true
		// 10 colunas de ponto = 5 caracteres Braille no terminal.
		if runewidth.StringWidth(wave) != containerWaveWidth {
			t.Fatalf("faixa de %q ocupa %d caracteres, esperado %d",
				label, runewidth.StringWidth(wave), containerWaveWidth)
		}
	}
}

// A ação pendente é o retorno do comando que acabou de ser dado — precisa
// ganhar do estado atual.
func TestContainerPendingActionWinsOverState(t *testing.T) {
	a := &App{animFrame: 1, containerActions: map[string]string{"web": "restart"}}
	_, _, label, _ := a.containerStateVisual(core.Container{Name: "web", State: "running", Status: "Up 2 days"})
	if label != "restarting" {
		t.Fatalf("ação pendente deveria vencer: %q", label)
	}
	_, _, label, _ = a.containerStateVisual(core.Container{Name: "outro", State: "running", Status: "Up 2 days"})
	if label != "running" {
		t.Fatalf("container sem ação pendente: %q", label)
	}
}

// Com Shift+A a lista mistura tudo que roda na máquina. O container que não é
// do projeto aberto precisa de cor diferente — sem isso dá para parar o
// container errado sem perceber.
func TestShowAllPaintsForeignProjectDifferently(t *testing.T) {
	mine := core.Project{Name: "digiliza", Path: "/apps/digiliza"}
	other := core.Project{Name: "portfolio", Path: "/apps/portfolio"}
	a := &App{
		width: 130, height: 34, containerShowAll: true,
		selectedProject: &mine,
		snapshot:        core.Snapshot{Projects: []core.Project{mine, other}},
	}

	own := a.containerProjectStyle(core.Container{Name: "a", ProjectPath: mine.Path})
	foreign := a.containerProjectStyle(core.Container{Name: "b", ProjectPath: other.Path})
	orphan := a.containerProjectStyle(core.Container{Name: "c"})

	if own.GetForeground() == foreign.GetForeground() {
		t.Fatal("container de outro projeto precisa de cor diferente do aberto")
	}
	if foreign.GetForeground() != StyleWarning.GetForeground() {
		t.Fatalf("o de fora deve ser amarelo, got %v", foreign.GetForeground())
	}
	if orphan.GetForeground() != StyleWarning.GetForeground() {
		t.Fatalf("órfão do docker também não é do projeto aberto: %v", orphan.GetForeground())
	}

	// E o rótulo tem que dizer de quem é, não repetir o projeto aberto.
	if got := a.containerProjectLabel(core.Container{Name: "b", ProjectPath: other.Path}); got != "portfolio" {
		t.Fatalf("rótulo do dono: %q", got)
	}
	if got := a.containerProjectLabel(core.Container{Name: "c"}); got != "órfão" {
		t.Fatalf("sem projeto: %q", got)
	}
}
