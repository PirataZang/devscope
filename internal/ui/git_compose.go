package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type gitComposeFocus int

const (
	gitComposeFocusEditor gitComposeFocus = iota
	gitComposeFocusCommit
	gitComposeFocusCancel
)

func (a *App) startGitCompose(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	a.gitComposeOn = true
	a.gitComposeMsg = ""
	a.gitComposeEdit = editorState{Cursor: 0, Anchor: -1}
	a.gitComposeFocus = gitComposeFocusEditor
	a.gitStatusMsg = "mensagem · tab botões · enter no Commitar"
}

func (a *App) closeGitCompose() {
	a.gitComposeOn = false
	a.gitComposeMsg = ""
	a.gitComposeEdit = editorState{Anchor: -1}
	a.gitComposeFocus = gitComposeFocusEditor
}

func (a *App) updateGitCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.closeGitCompose()
		a.gitStatusMsg = "commit cancelado"
		return a, nil
	case "tab":
		a.gitComposeFocus = (a.gitComposeFocus + 1) % 3
		return a, nil
	case "shift+tab":
		a.gitComposeFocus = (a.gitComposeFocus + 2) % 3
		return a, nil
	case "enter":
		switch a.gitComposeFocus {
		case gitComposeFocusCommit:
			return a, a.submitGitCompose()
		case gitComposeFocusCancel:
			a.closeGitCompose()
			a.gitStatusMsg = "commit cancelado"
			return a, nil
		}
		// editor: fall through to multiline newline
	}

	if a.gitComposeFocus != gitComposeFocusEditor {
		return a, nil
	}

	newText, handled := editorApplyKey(msg, a.gitComposeMsg, &a.gitComposeEdit, true)
	if handled {
		a.gitComposeMsg = newText
	}
	return a, nil
}

func (a *App) submitGitCompose() tea.Cmd {
	msg := strings.TrimSpace(a.gitComposeMsg)
	if msg == "" {
		a.gitStatusMsg = "mensagem vazia"
		a.gitComposeFocus = gitComposeFocusEditor
		return nil
	}
	p := a.currentProject()
	if p == nil {
		a.closeGitCompose()
		return nil
	}
	a.closeGitCompose()
	return a.gitCommit(p, msg)
}

func (a *App) gitCommit(p *core.Project, message string) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "commitando…"
	path := p.Path
	return func() tea.Msg {
		err := collectors.GitCommit(path, message)
		return gitActionDoneMsg{path: path, action: "commit", branch: collectors.GitCurrentBranch(path), err: err}
	}
}

func (a *App) renderGitCompose() string {
	background := a.renderProject()
	p := a.currentProject()
	g := a.projectGitInfo(p)

	boxW := minInt(a.width-4, maxInt(56, a.width*90/100))
	boxH := minInt(a.height-2, maxInt(18, a.height*80/100))
	editorH := maxInt(8, boxH-12)

	branch := "—"
	staged, modified, untracked := 0, 0, 0
	if g != nil {
		branch = g.Branch
		staged, modified, untracked = g.Staged, g.Modified, g.Untracked
	}

	editing := a.gitComposeFocus == gitComposeFocusEditor
	ed := a.gitComposeEdit
	bodyLines := renderEditorLines(a.gitComposeMsg, &ed, boxW-4, editorH, editing, false)
	a.gitComposeEdit = ed

	commitBtn := "  Commitar  "
	cancelBtn := "  Cancelar  "
	switch a.gitComposeFocus {
	case gitComposeFocusCommit:
		commitBtn = StyleSelected.Render("▸ Commitar ◂")
		cancelBtn = StyleMuted.Render(cancelBtn)
	case gitComposeFocusCancel:
		commitBtn = StyleMuted.Render(commitBtn)
		cancelBtn = StyleSelected.Render("▸ Cancelar ◂")
	default:
		commitBtn = StyleMuted.Render(commitBtn)
		cancelBtn = StyleMuted.Render(cancelBtn)
	}

	msgLabel := "mensagem"
	if editing {
		msgLabel = "mensagem  (enter = nova linha · tab = botões)"
	}

	lines := []string{
		StyleSection.Render("Novo commit"),
		StyleMuted.Render("branch  ") + StyleWarning.Render(branch),
		StyleMuted.Render(fmt.Sprintf("staged %d  ·  modified %d  ·  untracked %d", staged, modified, untracked)),
		"",
		StyleMuted.Render(msgLabel),
	}
	lines = append(lines, bodyLines...)
	lines = append(lines, "",
		commitBtn+"    "+cancelBtn,
		StyleMuted.Render("tab troca foco  ·  enter no botão confirma  ·  esc sai"),
	)
	if staged == 0 {
		lines = append(lines, StyleMuted.Render("sem stage: tracked modificados entram no commit (git add -u)"))
	}

	box := StylePanel.
		Width(boxW).
		Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
	return overlayCentered(background, box, a.width, a.height)
}
