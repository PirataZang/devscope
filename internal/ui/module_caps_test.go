package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func capsApp(on map[Tab]bool) *App {
	p := &core.Project{Name: "app", Path: "/p", Status: core.StatusRunning}
	return &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: p,
		snapshot:        core.Snapshot{Projects: []core.Project{*p}},
		moduleCaps:      moduleCaps{path: "/p", ready: true, on: on},
	}
}

// Antes da sondagem responder, TUDO aparece. Esconder módulo por causa de uma
// medida que ainda não chegou é perder funcionalidade na primeira tela.
func TestModuleCapsShowsEverythingUntilProbed(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabOverview}
	if n := a.hiddenTabCount(); n != 0 {
		t.Fatalf("sem sondagem nada deveria estar oculto, %d ocultos", n)
	}
	if len(a.visibleTabs()) != len(AllTabs) {
		t.Fatalf("sem sondagem os %d módulos deveriam estar visíveis, vieram %d", len(AllTabs), len(a.visibleTabs()))
	}
	plain := stripANSI(a.renderProjectSidebar())
	for _, want := range []string{"Kubernetes", "Jenkins", "CF Tunnel", "Ngrok"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("%q sumiu antes da sondagem:\n%s", want, plain)
		}
	}
}

// Projeto Go sem Docker, sem Jenkins e sem túnel: a sidebar mostra o que ele
// tem, e diz quantos ficaram de fora.
func TestModuleCapsHidesIrrelevantAndAnnouncesThem(t *testing.T) {
	a := capsApp(map[Tab]bool{
		TabOverview: true, TabGit: true, TabAPI: true, TabDatabase: true, TabWebSocket: true,
	})
	plain := stripANSI(a.renderProjectSidebar())
	for _, want := range []string{"Visão Geral", "Git", "API", "Database"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("módulo relevante %q não aparece:\n%s", want, plain)
		}
	}
	for _, gone := range []string{"Kubernetes", "Jenkins", "CF Tunnel", "Ngrok", "Swarm"} {
		if strings.Contains(plain, gone) {
			t.Fatalf("módulo irrelevante %q continua na sidebar:\n%s", gone, plain)
		}
	}
	// Nada some em silêncio: a contagem e a tecla ficam na tela.
	if !strings.Contains(plain, "ocultos") || !strings.Contains(plain, "⋯") {
		t.Fatalf("a sidebar não anuncia os módulos ocultos:\n%s", plain)
	}
	if n := a.hiddenTabCount(); n != len(AllTabs)-5 {
		t.Fatalf("%d ocultos, esperado %d", n, len(AllTabs)-5)
	}
}

// Grupo que ficou sem nenhum módulo some junto — rótulo órfão é pior que a
// linha que ele deveria titular.
func TestModuleCapsDropsEmptyGroups(t *testing.T) {
	a := capsApp(map[Tab]bool{TabOverview: true, TabGit: true})
	plain := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(plain, "PROJETO") || !strings.Contains(plain, "CÓDIGO") {
		t.Fatalf("grupos com módulo deveriam aparecer:\n%s", plain)
	}
	for _, gone := range []string{"EXECUÇÃO", "REDE", "DADOS"} {
		if strings.Contains(plain, gone) {
			t.Fatalf("grupo vazio %q continua na sidebar:\n%s", gone, plain)
		}
	}
}

// A tecla `t` é o escape: traz tudo de volta e devolve.
func TestShowAllModulesToggle(t *testing.T) {
	a := capsApp(map[Tab]bool{TabOverview: true, TabGit: true})
	if strings.Contains(stripANSI(a.renderProjectSidebar()), "Kubernetes") {
		t.Fatal("Kubernetes não deveria aparecer antes do toggle")
	}
	a.showAllModules = true
	plain := stripANSI(a.renderProjectSidebar())
	for _, want := range []string{"Kubernetes", "Jenkins", "CF Tunnel", "Swarm", "Ngrok", "Nginx"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("com `t` ligado %q deveria voltar:\n%s", want, plain)
		}
	}
	if a.hiddenTabCount() != 0 {
		t.Fatal("com `t` ligado nada está oculto")
	}
	if len(a.visibleTabs()) != len(AllTabs) {
		t.Fatal("com `t` ligado o ciclo do tab passa por todos")
	}
}

// A aba ATIVA nunca some, mesmo irrelevante: você pode ter chegado nela pelo
// modo "todos", e a sidebar não pode apagar a linha de onde você está.
func TestActiveTabStaysVisibleEvenWhenIrrelevant(t *testing.T) {
	a := capsApp(map[Tab]bool{TabOverview: true, TabGit: true})
	a.tab = TabKubernetes
	plain := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(plain, "Kubernetes") {
		t.Fatalf("a aba ativa sumiu da sidebar:\n%s", plain)
	}
	if !strings.Contains(plain, "EXECUÇÃO") {
		t.Fatalf("o grupo da aba ativa sumiu:\n%s", plain)
	}
}

// tab/shift+tab andam só pelo que está na tela — ciclar por módulo invisível
// faria a seleção desaparecer da sidebar.
func TestTabCycleWalksOnlyVisibleModules(t *testing.T) {
	a := capsApp(map[Tab]bool{TabOverview: true, TabGit: true, TabContainers: true})
	p := a.currentProject()
	seen := map[Tab]bool{}
	for i := 0; i < 6; i++ {
		a.cycleProjectTab(1, p)
		seen[a.tab] = true
	}
	for tab := range seen {
		if !a.tabVisible(tab) {
			t.Fatalf("o ciclo parou em %v, que não está na sidebar", tab)
		}
	}
	for _, want := range []Tab{TabOverview, TabGit, TabContainers} {
		if !seen[want] {
			t.Fatalf("o ciclo não passou por %v", want)
		}
	}
	// E volta para trás pelo mesmo caminho.
	a.cycleProjectTab(-1, p)
	if !a.tabVisible(a.tab) {
		t.Fatalf("shift+tab parou em %v, invisível", a.tab)
	}
}

// A sondagem lê o disco, não adivinha: pasta de workflow presente liga o
// módulo, ausente desliga.
func TestProbeModuleCapsReadsTheProject(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "app", Path: dir, Git: &core.GitInfo{IsRepo: true}}
	a := &App{width: 120, height: 40, selectedProject: p}

	msg, _ := a.probeModuleCaps(p)().(moduleCapsMsg)
	if msg.on[TabActions] {
		t.Fatal("sem .github/workflows, GH Actions não é relevante")
	}
	if !msg.on[TabGit] {
		t.Fatal("com repositório git, Git é relevante")
	}
	if !msg.on[TabOverview] || !msg.on[TabAPI] {
		t.Fatal("Visão Geral e os scratchpads são sempre relevantes")
	}

	if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if msg, _ = a.probeModuleCaps(p)().(moduleCapsMsg); !msg.on[TabActions] {
		t.Fatal("com .github/workflows, GH Actions passa a ser relevante")
	}

	// Sem repositório, Git sai.
	p.Git = nil
	if msg, _ = a.probeModuleCaps(p)().(moduleCapsMsg); msg.on[TabGit] {
		t.Fatal("sem repositório, Git não é relevante")
	}
}

// A medida vale para UM projeto: trocar de projeto não pode herdar a anterior.
func TestModuleCapsIgnoresStaleProbe(t *testing.T) {
	a := capsApp(map[Tab]bool{TabOverview: true})
	a.handleModuleCapsMsg(moduleCapsMsg{path: "/outro", on: map[Tab]bool{TabGit: true}})
	if a.moduleCaps.path != "/p" {
		t.Fatal("sondagem de outro projeto não pode sobrescrever a atual")
	}
}

// A ordem do `tab` tem de ser a MESMA da sidebar, senão a navegação pula para
// uma linha que está acima na tela.
func TestAllTabsFollowsSidebarOrder(t *testing.T) {
	var want []Tab
	for _, g := range sidebarGroups() {
		want = append(want, g.tabs...)
	}
	if len(want) != len(AllTabs) {
		t.Fatalf("sidebar tem %d módulos, AllTabs tem %d", len(want), len(AllTabs))
	}
	for i := range want {
		if want[i] != AllTabs[i] {
			t.Fatalf("posição %d: sidebar %v, AllTabs %v", i, want[i], AllTabs[i])
		}
	}
}

// ─── toda porta de entrada precisa medir ────────────────────────────────────

// `devscope` dentro de um projeto entra direto no módulo, sem passar pelo
// openProject. Sem sondagem ali, moduleCaps.ready fica false para sempre e o
// modo "na dúvida, mostre" vira o modo permanente: os quinze módulos, em todo
// projeto, sempre. Foi o que aconteceu — a sidebar nunca filtrava na prática.
func TestCwdStartupProbesCapabilities(t *testing.T) {
	dir := t.TempDir()
	p := core.Project{Name: "app", Path: dir, Status: core.StatusUnknown}
	a := &App{
		width: 120, height: 40, tab: TabGit, view: ViewProject,
		selectedProject: &p, store: core.NewStateStore(nil),
		snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	// A sondagem tem de ser um dos comandos que o Init dispara.
	if !cmdBatchProbes(a.Init()) {
		t.Fatal("Init não sonda as capacidades do projeto aberto pelo diretório")
	}
	// E abrir pelo diretório invalida a medida anterior. O projeto tem de ser
	// o do cwd de verdade — é assim que findProjectForCwd o encontra.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	here := core.Project{Name: "aqui", Path: cwd, Status: core.StatusUnknown}
	a.moduleCaps = moduleCaps{path: "/outro", ready: true, on: map[Tab]bool{TabGit: true}}
	a.showAllModules = true
	a.snapshot.Projects = []core.Project{here}
	a.openProjectFromCwd()
	if a.selectedProject == nil || a.selectedProject.Path != cwd {
		t.Fatalf("o projeto do cwd não foi aberto: %+v", a.selectedProject)
	}
	if a.moduleCaps.ready {
		t.Error("abrir outro projeto tem de descartar a medida do anterior")
	}
	if a.showAllModules {
		t.Error("o modo 'mostrar todos' não pode vazar de um projeto para outro")
	}
}

// cmdBatchProbes roda o tea.Cmd e diz se alguma das mensagens é uma sondagem
// de capacidades. tea.Batch devolve BatchMsg com os comandos filhos.
func cmdBatchProbes(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case moduleCapsMsg:
		return true
	case tea.BatchMsg:
		for _, c := range msg {
			if cmdBatchProbes(c) {
				return true
			}
		}
	}
	return false
}

// A sondagem lê p.Git e p.Containers, que chegam DEPOIS da varredura. Medir
// antes classificaria um repositório git como "sem git" — e a medida errada
// não se corrigia sozinha, escondendo o Git de um projeto que tem Git.
func TestCapsAreRemeasuredWhenProjectDataLands(t *testing.T) {
	dir := t.TempDir()
	// Antes do carregamento: sem Git no projeto.
	thin := core.Project{Name: "app", Path: dir, Status: core.StatusUnknown}
	msg, _ := (&App{}).probeModuleCaps(&thin)().(moduleCapsMsg)
	if msg.on[TabGit] {
		t.Fatal("sem Git no projeto a sondagem não deve marcar Git relevante")
	}
	// Depois: o mesmo projeto, agora com o repositório carregado.
	fat := thin
	fat.Git = &core.GitInfo{IsRepo: true, Branch: "main"}
	msg, _ = (&App{}).probeModuleCaps(&fat)().(moduleCapsMsg)
	if !msg.on[TabGit] {
		t.Fatal("com o repositório carregado o Git tem de voltar a ser relevante")
	}
}

// A aba ativa sobrevive a uma medida que a considera irrelevante — senão abrir
// o devscope numa pasta sem git deixava a tela sem a aba em que se está.
func TestActiveTabSurvivesFirstProbe(t *testing.T) {
	p := &core.Project{Name: "app", Path: "/p", Status: core.StatusUnknown}
	a := &App{width: 120, height: 40, tab: TabGit, view: ViewProject, selectedProject: p,
		snapshot:   core.Snapshot{Projects: []core.Project{*p}},
		moduleCaps: moduleCaps{path: "/p", ready: true, on: map[Tab]bool{TabOverview: true}}}
	plain := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(plain, "Git") {
		t.Fatalf("a aba ativa sumiu da sidebar:\n%s", plain)
	}
}
