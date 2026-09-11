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
	if !strings.Contains(got, "shift+R") || !strings.Contains(strings.ToLower(got), "reinício") {
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
	// AÇÕES saiu do rodapé (virou barra larga) e as três caixas lado a lado
	// viraram fatos rotulados + UMA caixa de logs.
	for _, want := range []string{"LOGS", "portas", "recursos"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("detalhe do container sem %q:\n%s", want, truncate(plain, 300))
		}
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

// A coluna vertical AÇÕES virou barra de comandos larga: é ela que precisa
// listar os atalhos agora. Antes este teste guardava uma caixa que só era
// desenhada dentro de um `if false`.
func TestContainersCommandBarListsAllShortcuts(t *testing.T) {
	a := &App{width: 160, height: 40, containerOnlyDocker: true}
	plain := stripANSI(a.renderContainersCommandBar(160))
	for _, key := range []string{"enter", "m", "e", "r", "s", "p", "d", "i", "n", "shift+R", "shift+U", "shift+D", "A", "v"} {
		if !strings.Contains(plain, key) {
			t.Fatalf("atalho %q sumiu da barra de comandos:\n%s", key, plain)
		}
	}
}

// As portas eram um painel com moldura; agora são uma linha de fatos no detalhe
// do container. A tela dedicada (enter) continua inteira.
func TestContainerPortFacts(t *testing.T) {
	a := &App{width: 120, height: 40}
	facts := a.containerPortFacts(core.Container{Ports: "0.0.0.0:8080->80/tcp, 0.0.0.0:5432->5432/tcp"})
	joined := stripANSI(strings.Join(facts, " "))
	for _, want := range []string{":8080", "80/tcp", ":5432"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("porta %q perdida: %q", want, joined)
		}
	}
	if got := a.containerPortFacts(core.Container{}); len(got) != 0 {
		t.Fatalf("sem portas a linha vira %q via factLine, não %v", emDash, got)
	}
	// Muitas portas: corta com "+N" em vez de estourar a linha.
	many := a.containerPortFacts(core.Container{Ports: "0.0.0.0:1->1/tcp, 0.0.0.0:2->2/tcp, 0.0.0.0:3->3/tcp, 0.0.0.0:4->4/tcp, 0.0.0.0:5->5/tcp, 0.0.0.0:6->6/tcp"})
	if !strings.Contains(stripANSI(strings.Join(many, " ")), "+2") {
		t.Fatalf("o excedente devia virar +N: %q", stripANSI(strings.Join(many, " ")))
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

// ─── sessão 4: lista dominante, detalhe sem quatro molduras ─────────────────

func containersFixture(n int) []core.Container {
	out := make([]core.Container, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, core.Container{
			ID: fmt.Sprintf("c%d", i), Name: fmt.Sprintf("svc-%d", i),
			Image: "img:latest", State: "running", Status: "Up 2 hours",
			CPU: 3.5, Memory: 120 << 20,
		})
	}
	return out
}

// A lista PEDE o que precisa. Antes a divisão era fixa em 66/34 e sete
// containers deixavam dez linhas em branco dentro da caixa LISTA.
func TestContainerSplitGivesListWhatItNeeds(t *testing.T) {
	const bodyH = 30
	small, smallDetail := containerSplitHeight(bodyH, 5)
	if small > 5+4 {
		t.Fatalf("com 5 containers a lista não deveria passar de %d linhas, pegou %d", 5+4, small)
	}
	if small+smallDetail != bodyH {
		t.Fatalf("a soma tem de fechar a altura: %d+%d != %d", small, smallDetail, bodyH)
	}
	// Muitos containers: a lista cresce, o detalhe cede até o piso.
	big, bigDetail := containerSplitHeight(bodyH, 40)
	if big <= small {
		t.Fatalf("com 40 containers a lista deveria crescer (%d → %d)", small, big)
	}
	if bigDetail < 9 {
		t.Fatalf("o detalhe não pode cair abaixo do piso legível: %d", bigDetail)
	}
	if big+bigDetail != bodyH {
		t.Fatalf("a soma tem de fechar a altura: %d+%d != %d", big, bigDetail, bodyH)
	}
	// Terminal curto: os dois ainda existem.
	for _, h := range []int{12, 14, 16, 20, 24, 40, 60} {
		tb, dt := containerSplitHeight(h, 30)
		if tb < 6 || dt < 5 || tb+dt != h {
			t.Fatalf("altura %d: lista=%d detalhe=%d", h, tb, dt)
		}
	}
}

// O aviso de "há mais containers" ficava numa linha DEPOIS da janela, e o
// fitExactLines cortava exatamente ela — nunca chegou à tela.
func TestContainerListAnnouncesOffscreenRows(t *testing.T) {
	p := &core.Project{Path: "/p", Name: "app", Containers: containersFixture(20)}
	a := &App{width: 120, height: 40, selectedProject: p, tab: TabContainers,
		snapshot: core.Snapshot{Projects: []core.Project{*p}}}
	plain := stripANSI(a.renderContainersTable(p.Containers, 100, 10))
	if !strings.Contains(plain, "↓") {
		t.Fatalf("a lista não avisa que há mais containers abaixo:\n%s", plain)
	}
	a.containerScroll, a.tabCursor = 10, 10
	plain = stripANSI(a.renderContainersTable(p.Containers, 100, 10))
	if !strings.Contains(plain, "↑") {
		t.Fatalf("rolada, a lista não avisa que há containers acima:\n%s", plain)
	}
}

// Comparar antes de descrever: CPU/MEM/TEMPO sobrevivem em telas estreitas,
// IMAGEM e PORTAS cedem primeiro — elas estão no detalhe do selecionado.
func TestContainerColumnsPrioritizeComparison(t *testing.T) {
	for _, tw := range []int{52, 60, 73, 90, 117, 160, 200} {
		a := &App{width: 200, containerTableWidth: tw}
		c := a.containerColumns()
		if c.cpu == 0 || c.mem == 0 || c.uptime == 0 {
			t.Fatalf("largura %d: CPU/MEM/TEMPO deviam sobreviver: %+v", tw, c)
		}
		if c.name < 12 {
			t.Fatalf("largura %d: NOME ilegível (%d)", tw, c.name)
		}
		if c.ports > 0 && c.image == 0 {
			t.Fatalf("largura %d: PORTAS não pode entrar antes de IMAGEM: %+v", tw, c)
		}
		// A linha inteira tem de caber na tabela.
		used := 1 + c.dot + c.name + c.cpu + c.mem + c.uptime + 4
		for _, opt := range []int{c.state, c.project, c.image, c.ports} {
			if opt > 0 {
				used += opt + 1
			}
		}
		if used > tw {
			t.Fatalf("largura %d: colunas somam %d: %+v", tw, used, c)
		}
	}
}

// `g` continua ciclando a métrica, mas agora compra RESOLUÇÃO em vez de só
// apagar duas linhas.
func TestStatsModeFocusBuysResolution(t *testing.T) {
	p := &core.Project{Path: "/p", Name: "app", Containers: containersFixture(2)}
	a := &App{width: 140, height: 44, selectedProject: p, tab: TabContainers,
		snapshot:            core.Snapshot{Projects: []core.Project{*p}},
		containerCPUHistory: []float64{5, 9, 14, 11, 18, 12},
		containerMemHistory: []float64{20, 22, 21, 25, 24, 23},
		containerNetHistory: []float64{100, 120, 90, 140, 110, 130},
	}
	all := stripANSI(strings.Join(a.containerResourceFacts(p.Containers[0], 130), " "))
	for _, want := range []string{"CPU", "MEM", "NET"} {
		if !strings.Contains(all, want) {
			t.Fatalf("modo 0 mostra as três: falta %q em %q", want, all)
		}
	}
	for mode, unique := range map[int]string{1: "CPU", 2: "MEM", 3: "NET"} {
		a.containerStatsMode = mode
		got := stripANSI(strings.Join(a.containerResourceFacts(p.Containers[0], 130), " "))
		if !strings.Contains(got, "média") || !strings.Contains(got, "pico") {
			t.Fatalf("modo %d (%s) devia trazer média e pico: %q", mode, unique, got)
		}
		if lbl := a.containerResourceLabel(); !strings.Contains(lbl, "g") {
			t.Fatalf("modo %d: o rótulo deve anunciar a tecla g, veio %q", mode, lbl)
		}
	}
	a.containerStatsMode = 0
	if lbl := a.containerResourceLabel(); lbl != "recursos" {
		t.Fatalf("modo 0 devia ser %q, veio %q", "recursos", lbl)
	}
}

// Um log é uma cauda: a última linha encosta na borda de baixo, como num tail.
func TestPadLinesTopAnchorsTheTail(t *testing.T) {
	got := padLinesTop([]string{"a", "b"}, 5)
	if len(got) != 5 || got[4] != "b" || got[3] != "a" || got[0] != "" {
		t.Fatalf("a cauda devia encostar embaixo: %q", got)
	}
	// Mais linhas que espaço: fica com as últimas.
	got = padLinesTop([]string{"a", "b", "c", "d"}, 2)
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Fatalf("devia manter as duas últimas: %q", got)
	}
}

// O detalhe tem altura fixa e nunca estoura a largura, com ou sem seleção.
func TestContainerDetailHeadIsStable(t *testing.T) {
	p := &core.Project{Path: "/p", Name: "app", Containers: containersFixture(3)}
	for _, sel := range []bool{true, false} {
		a := &App{width: 160, height: 44, tab: TabContainers,
			snapshot: core.Snapshot{Projects: []core.Project{*p}}}
		if sel {
			a.selectedProject = p
		}
		for _, w := range []int{40, 60, 80, 100, 120, 160, 200} {
			head := a.containerDetailHeadLines(w)
			if len(head) != 4 {
				t.Fatalf("sel=%v w=%d: detalhe com %d linhas, esperado 4", sel, w, len(head))
			}
			for i, l := range head {
				if lipgloss.Width(l) > w {
					t.Fatalf("sel=%v w=%d: linha %d mede %d", sel, w, i, lipgloss.Width(l))
				}
			}
		}
	}
}
