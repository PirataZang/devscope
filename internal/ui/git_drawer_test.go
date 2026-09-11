package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

func drawerApp(w, h int) (*App, *core.Project) {
	g := &core.GitInfo{
		IsRepo: true, Branch: "main", Modified: 2, StashCount: 2,
		Branches: []core.GitBranch{{Name: "main", Current: true}, {Name: "develop"}},
		Commits:  []core.GitCommit{{Hash: "abc1234", Message: "fix", Date: "2h"}},
		Files:    []core.GitFileStatus{{Path: "a.go", Staging: " ", Worktree: "M"}},
		Stashes: []core.GitStash{
			{Ref: "stash@{0}", Message: "WIP on main: 111 experimento"},
			{Ref: "stash@{1}", Message: "On main: outro"},
		},
	}
	p := &core.Project{Path: "/p", Name: "repo", Git: g}
	a := &App{
		width: w, height: h, view: ViewProject, tab: TabGit,
		gitSubview: gitSubviewMain, gitFocus: gitFocusBranches,
		gitViewBranch: "main", gitBranches: g.Branches, gitBranchCommits: g.Commits,
		selectedProject: p, snapshot: core.Snapshot{Projects: []core.Project{*p}},
	}
	return a, p
}

// A tela padrão é a prioridade da sessão: BRANCHES + COMMITS + WORKTREE. Duas
// caixas, não cinco.
func TestGitDefaultScreenHasNoDrawerBoxes(t *testing.T) {
	a, p := drawerApp(140, 44)
	got := stripANSI(a.renderGitTab(p))
	if strings.Count(got, "┌─") > 3 {
		t.Fatalf("caixas demais na tela padrão (%d):\n%s", strings.Count(got, "┌─"), got)
	}
	for _, gone := range []string{"LOG DE COMANDOS", "STASHES"} {
		if strings.Contains(got, gone) {
			t.Fatalf("%q não é ambiente, é gaveta:\n%s", gone, got)
		}
	}
	if a.gitDrawerHeight(30) != 0 {
		t.Fatal("gaveta fechada não pode tomar altura")
	}
}

// `s` e `^l` abrem e fecham. A mesma tecla faz os dois caminhos.
func TestGitDrawerKeysToggle(t *testing.T) {
	a, p := drawerApp(140, 44)
	a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if a.gitDrawer != gitDrawerStash {
		t.Fatalf("s devia abrir o stash, veio %v", a.gitDrawer)
	}
	if !strings.Contains(stripANSI(a.renderGitTab(p)), "stash@{0}") {
		t.Fatal("a gaveta aberta devia mostrar os stashes")
	}
	a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if a.gitDrawer != gitDrawerNone {
		t.Fatalf("s de novo devia fechar, veio %v", a.gitDrawer)
	}
	a.updateProject(tea.KeyMsg{Type: tea.KeyCtrlL})
	if a.gitDrawer != gitDrawerLog || a.gitFocus != gitFocusCmdLog {
		t.Fatalf("^l devia abrir o log e levar o foco: drawer=%v focus=%v", a.gitDrawer, a.gitFocus)
	}
	// esc fecha a gaveta antes de sair do módulo.
	a.updateProject(tea.KeyMsg{Type: tea.KeyEsc})
	if a.gitDrawer != gitDrawerNone {
		t.Fatal("esc devia fechar a gaveta")
	}
	if a.view != ViewProject {
		t.Fatal("esc que fecha a gaveta não pode sair do projeto junto")
	}
}

// Sem stash, `s` explica em vez de abrir uma caixa vazia.
func TestGitStashKeyWithoutStashes(t *testing.T) {
	a, _ := drawerApp(140, 44)
	a.selectedProject.Git.StashCount = 0
	a.selectedProject.Git.Stashes = nil
	a.snapshot.Projects[0] = *a.selectedProject
	a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if a.gitDrawer != gitDrawerNone {
		t.Fatal("sem stash a gaveta não abre")
	}
	if !strings.Contains(a.gitStatusMsg, "nenhum stash") {
		t.Fatalf("devia explicar por que nada aconteceu: %q", a.gitStatusMsg)
	}
}

// Rodou um comando git: a saída aparece na hora em que interessa. Era o único
// valor da caixa permanente — e ela cobrava a tela inteira por ele.
func TestGitCommandOpensLogDrawer(t *testing.T) {
	a, _ := drawerApp(140, 44)
	if a.gitDrawer != gitDrawerNone {
		t.Fatal("começa fechada")
	}
	a.noteGitCommandRan()
	if a.gitDrawer != gitDrawerLog {
		t.Fatal("um comando git devia abrir o log sozinho")
	}
	// Fora da tela principal do Git, não interrompe.
	a.gitDrawer = gitDrawerNone
	a.gitSubview = gitSubviewGraph
	a.noteGitCommandRan()
	if a.gitDrawer != gitDrawerNone {
		t.Fatal("no grafo o log não deve abrir por cima")
	}
}

// O ciclo ←→ não pode parar num painel que não está na tela.
func TestGitFocusCycleSkipsClosedDrawer(t *testing.T) {
	a, _ := drawerApp(140, 44)
	seen := map[gitFocus]bool{}
	for i := 0; i < 8; i++ {
		a.gitFocusNext()
		seen[a.gitFocus] = true
	}
	if seen[gitFocusCmdLog] {
		t.Fatal("com a gaveta fechada o foco não pode cair no log")
	}
	for _, want := range []gitFocus{gitFocusBranches, gitFocusCommits, gitFocusFiles} {
		if !seen[want] {
			t.Fatalf("o ciclo não passou por %v", want)
		}
	}
	// Aberta, o log entra no ciclo.
	a.openGitDrawer(gitDrawerLog)
	a.gitFocus = gitFocusFiles
	a.gitFocusNext()
	if a.gitFocus != gitFocusCmdLog {
		t.Fatalf("com a gaveta aberta o log entra no ciclo, veio %v", a.gitFocus)
	}
	// E fechar devolve o foco para um painel visível.
	a.closeGitDrawer()
	if a.gitFocus == gitFocusCmdLog {
		t.Fatal("fechar a gaveta deve tirar o foco do log")
	}
	// shift+tab (prev) também respeita.
	a.gitFocus = gitFocusBranches
	a.gitFocusPrev()
	if a.gitFocus == gitFocusCmdLog {
		t.Fatal("←  com gaveta fechada não pode cair no log")
	}
}

// A gaveta é a única porta para stash e log: as teclas não podem ser as
// primeiras a cair quando a barra encolhe.
func TestGitCommandBarKeepsDrawerKeys(t *testing.T) {
	a, _ := drawerApp(120, 40)
	g := a.selectedProject.Git
	for _, w := range []int{70, 90, 110, 140, 200} {
		bar := stripANSI(a.renderGitCommandBar(g, w))
		if !strings.Contains(bar, "ctrl+l") {
			t.Fatalf("largura %d: a barra perdeu o log:\n%s", w, bar)
		}
	}
	// A tecla do stash vai colada ao contador: em 80 colunas a barra só tem
	// duas linhas e `s` era a primeira a cair.
	for _, w := range []int{60, 80, 100, 140, 200} {
		row := stripANSI(a.renderGitStatsRow(g, w))
		if !strings.Contains(row, "2 stash") || !strings.Contains(row, "s") {
			t.Fatalf("largura %d: o contador de stash não anuncia a tecla:\n%s", w, row)
		}
	}
}

// A gaveta empresta espaço, não toma a tela — e some quando não há o que
// emprestar.
func TestGitDrawerHeightStaysModest(t *testing.T) {
	a, _ := drawerApp(140, 44)
	a.gitDrawer = gitDrawerLog
	for _, bodyH := range []int{10, 14, 16, 20, 30, 44, 60} {
		h := a.gitDrawerHeight(bodyH)
		if h > bodyH/2 {
			t.Fatalf("corpo %d: gaveta com %d linhas é mais que metade", bodyH, h)
		}
		if h > 0 && h < 4 {
			t.Fatalf("corpo %d: gaveta de %d linhas é ilegível — devia ser 0", bodyH, h)
		}
	}
	if a.gitDrawerHeight(12) != 0 {
		t.Fatal("corpo curto: a gaveta cede em vez de espremer branches/commits")
	}
}

// Nada estoura em nenhum tamanho, com a gaveta aberta ou fechada.
func TestGitScreenFitsEveryTerminal(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}} {
		for _, d := range []gitDrawer{gitDrawerNone, gitDrawerLog, gitDrawerStash} {
			a, p := drawerApp(sz[0], sz[1])
			a.gitDrawer = d
			for i, line := range strings.Split(a.renderGitTab(p), "\n") {
				if lipgloss.Width(line) > sz[0] {
					t.Fatalf("%dx%d d=%d: linha %d mede %d", sz[0], sz[1], d, i, lipgloss.Width(line))
				}
			}
		}
	}
}
