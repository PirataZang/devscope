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

	boxW := minInt(a.width-4, maxInt(58, a.width*72/100))
	boxH := minInt(a.height-2, maxInt(22, a.height*72/100))
	innerW := maxInt(32, boxW-6)
	accent := tabAccentColor(TabGit)

	branch := "—"
	staged, modified, untracked := 0, 0, 0
	if g != nil {
		branch = g.Branch
		staged, modified, untracked = g.Staged, g.Modified, g.Untracked
	}
	proj := ""
	if p != nil {
		proj = p.Name
	}

	editing := a.gitComposeFocus == gitComposeFocusEditor
	editorH := maxInt(6, boxH-20)
	ed := a.gitComposeEdit
	bodyLines := renderEditorLines(a.gitComposeMsg, &ed, innerW-2, editorH, editing, false)
	a.gitComposeEdit = ed

	lines := tunnelModalChrome("GIT", accent, "Novo commit", "escrever mensagem e confirmar", proj, innerW)
	lines = append(lines, "")

	branchBox := renderApiTitledBox("branch",
		[]string{StyleWarning.Bold(true).Render(truncate(branch, innerW-2))},
		innerW, 3, false,
	)
	lines = append(lines, strings.Split(branchBox, "\n")...)
	lines = append(lines, "")

	metrics := tunnelMetricRow([][2]string{
		{"STAGED", fmt.Sprintf("%d", staged)},
		{"MODIFIED", fmt.Sprintf("%d", modified)},
		{"UNTRACKED", fmt.Sprintf("%d", untracked)},
	}, innerW)
	if metrics != "" {
		lines = append(lines, strings.Split(metrics, "\n")...)
		lines = append(lines, "")
	}

	msgTitle := "mensagem"
	if editing {
		msgTitle = "mensagem · enter nova linha"
	}
	msgBox := renderApiTitledBox(msgTitle, bodyLines, innerW, editorH+2, editing)
	lines = append(lines, strings.Split(msgBox, "\n")...)
	lines = append(lines, "")

	preview := StyleMuted.Render("preview  ")
	first := strings.TrimSpace(strings.SplitN(a.gitComposeMsg, "\n", 2)[0])
	if first == "" {
		preview += StyleMuted.Render("(digite a mensagem)")
	} else {
		preview += StyleHealthy.Render(truncate(first, maxInt(20, innerW-12)))
	}
	lines = append(lines, preview)

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
	lines = append(lines, "",
		commitBtn+StyleMuted.Render("    ")+cancelBtn,
	)
	if staged == 0 {
		lines = append(lines, StyleMuted.Render("sem stage: tracked modificados entram (git add -u)"))
	}
	lines = append(lines, "",
		StyleMuted.Render("tab troca foco  ·  enter no botão confirma  ·  esc cancela"),
	)

	if boxH < len(lines) {
		boxH = len(lines)
	}
	box := StylePanel.
		Width(boxW).
		BorderForeground(accent).
		Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
	return overlayCentered(background, box, a.width, a.height)
}
