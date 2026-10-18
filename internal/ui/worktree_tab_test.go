package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func sampleWorktrees(main string) []collectors.GitWorktree {
	return []collectors.GitWorktree{
		{Path: main, Head: "a1b2c3d4e5", Branch: "main", Main: true, Subject: "merge da release"},
		{Path: main + "-feat-login", Head: "b2c3d4e5f6", Branch: "feat/login", Changes: 3, Subject: "tela de login"},
		{Path: main + "-hotfix", Head: "c3d4e5f6a7", Branch: "hotfix", Locked: true, LockReason: "disco usb"},
		{Path: main + "-velha", Head: "d4e5f6a7b8", Detached: true, Prunable: true, PruneReason: "gitdir aponta para lugar que não existe"},
	}
}

func worktreeTestApp() (*App, *core.Project) {
	p := &core.Project{Name: "app", Path: "/p/app", Git: &core.GitInfo{IsRepo: true, Branch: "main"}}
	a := &App{
		width: 140, height: 40, view: ViewProject, tab: TabWorktrees, selectedProject: p,
		snapshot: core.Snapshot{Projects: []core.Project{*p}},
		wtList:   sampleWorktrees(p.Path),
	}
	return a, p
}

func TestWorktreesListShowsStateWithoutColor(t *testing.T) {
	a, p := worktreeTestApp()
	got := stripANSI(a.renderWorktreesTab(p))
	for _, want := range []string{"WORKTREES · 4", "feat/login", "3 alterações", "travada", "pasta sumiu", "(detached)", "◆", "⚿", "⚠", "enter abrir no DevScope"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
}

func TestWorktreesOnlyMainSaysWhatToDo(t *testing.T) {
	a, p := worktreeTestApp()
	a.wtList = a.wtList[:1]
	got := stripANSI(a.renderWorktreesTab(p))
	if !strings.Contains(got, "n abre outra branch em") || !strings.Contains(got, "app-<branch>") {
		t.Fatalf("estado vazio tem de dizer a saída e onde a pasta nasce:\n%s", got)
	}
}

func TestWorktreesKeys(t *testing.T) {
	a, p := worktreeTestApp()
	press := func(k string) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "down" {
			msg = tea.KeyMsg{Type: tea.KeyDown}
		}
		if _, ok := a.handleWorktreeKeys(msg, p); !ok {
			t.Fatalf("%q devia ser tratada pelo módulo", k)
		}
	}

	press("d")
	if a.gitConfirmOn || !strings.Contains(a.wtStatus, "principal") {
		t.Fatalf("a principal não se remove: confirm=%v status=%q", a.gitConfirmOn, a.wtStatus)
	}

	press("down")
	press("d")
	if !a.gitConfirmOn || a.gitConfirmAction != "wt-remove" || a.gitConfirmBranch != "/p/app-feat-login" {
		t.Fatalf("d pede confirmação da worktree selecionada: %+v", a.gitConfirmBranch)
	}
	box := stripANSI(a.renderGitConfirm())
	if !strings.Contains(box, "Remover worktree") || !strings.Contains(box, "3 alterações não commitadas serão perdidas") {
		t.Fatalf("a confirmação avisa o que se perde:\n%s", box)
	}
	a.gitConfirmOn = false

	press("down")
	press("d")
	if a.gitConfirmOn || !strings.Contains(a.wtStatus, "travada") {
		t.Fatalf("travada não se remove daqui: %q", a.wtStatus)
	}

	press("down")
	if cmd, _ := a.handleWorktreeKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); cmd != nil || !strings.Contains(a.wtStatus, "não existe") {
		t.Fatalf("enter na podável explica, não abre: %q", a.wtStatus)
	}

	press("n")
	if !a.gitPromptOn || a.gitPromptKind != gitPromptNewWorktree {
		t.Fatal("n abre o prompt de nova worktree")
	}
	a.gitPromptInput = "feat/x"
	if prompt := stripANSI(a.renderGitPrompt()); !strings.Contains(prompt, "Nova worktree") || !strings.Contains(prompt, "/p/app-feat-x") {
		t.Fatalf("o prompt mostra onde a pasta nasce:\n%s", prompt)
	}

	// Tecla que o módulo não usa segue para a camada de cima.
	if _, ok := a.handleWorktreeKeys(tea.KeyMsg{Type: tea.KeyTab}, p); ok {
		t.Fatal("tab não é do módulo — troca de módulo")
	}
}
