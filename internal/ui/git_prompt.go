package ui

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type gitPromptKind int

const (
	gitPromptNewBranch gitPromptKind = iota
	gitPromptRenameBranch
)

func (a *App) gitTabReady(p *core.Project) bool {
	return a.tab == TabGit && a.gitSubview == gitSubviewMain && p != nil && a.projectGitInfo(p) != nil && !a.gitActionLoading
}

func (a *App) targetGitBranch(p *core.Project) (string, bool) {
	g := a.projectGitInfo(p)
	if g == nil {
		return "", false
	}
	if a.gitFocus == gitFocusBranches {
		if branch, ok := a.selectedGitBranch(p); ok {
			return branch, true
		}
	}
	if a.gitViewBranch != "" {
		return a.gitViewBranch, true
	}
	return g.Branch, true
}

func (a *App) selectedGitBranch(p *core.Project) (string, bool) {
	g := a.projectGitInfo(p)
	if g == nil {
		return "", false
	}
	branches := a.filteredGitBranches(a.gitBranchesForUI())
	if a.gitBranchCursor >= len(branches) {
		return "", false
	}
	return branches[a.gitBranchCursor].Name, true
}

func (a *App) startGitNewBranch(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	from, ok := a.targetGitBranch(p)
	if !ok {
		from = g.Branch
	}
	a.gitFocus = gitFocusBranches
	a.gitPromptOn = true
	a.gitPromptKind = gitPromptNewBranch
	a.gitPromptInput = ""
	a.gitPromptCursor = 0
	a.gitPromptBranch = from
	a.gitStatusMsg = "nova branch a partir de " + from
}

func (a *App) startGitRenameBranch(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	branch, ok := a.targetGitBranch(p)
	if !ok {
		a.gitStatusMsg = "selecione uma branch"
		return
	}
	a.gitFocus = gitFocusBranches
	a.syncGitBranchCursor(a.gitBranchesForUI())
	a.gitPromptOn = true
	a.gitPromptKind = gitPromptRenameBranch
	a.gitPromptInput = ""
	a.gitPromptCursor = 0
	a.gitPromptBranch = branch
}

func (a *App) startGitDeleteBranch(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	branch, ok := a.targetGitBranch(p)
	if !ok {
		a.gitStatusMsg = "selecione uma branch"
		return
	}
	if branch == g.Branch {
		a.gitStatusMsg = "não é possível apagar a branch atual"
		return
	}
	a.gitConfirmOn = true
	a.gitConfirmAction = "delete"
	a.gitConfirmBranch = branch
	a.gitStatusMsg = "apagar branch " + branch + "? y/esc"
}

func (a *App) startGitMerge(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	branch, ok := a.targetGitBranch(p)
	if !ok {
		a.gitStatusMsg = "selecione uma branch"
		return
	}
	if branch == g.Branch {
		a.gitStatusMsg = "selecione outra branch para mesclar em " + g.Branch
		return
	}
	a.gitConfirmOn = true
	a.gitConfirmAction = "merge"
	a.gitConfirmBranch = branch
	a.gitStatusMsg = "mesclar " + branch + " em " + g.Branch + "? y/esc"
}

func (a *App) gitToggleMarkedBranch(p *core.Project) {
	branch, ok := a.targetGitBranch(p)
	if !ok {
		a.gitStatusMsg = "selecione uma branch"
		return
	}
	if a.gitMarkedBranch == branch {
		a.gitMarkedBranch = ""
		a.gitStatusMsg = "marca de origem removida"
		return
	}
	a.gitMarkedBranch = branch
	a.gitStatusMsg = "origem " + branch + " — pull (p) usa origin " + branch
}

func (a *App) gitPullSourceBranch(p *core.Project) string {
	if a.gitMarkedBranch != "" {
		return a.gitMarkedBranch
	}
	g := a.projectGitInfo(p)
	if g == nil {
		return ""
	}
	head := g.Branch
	if head == "" {
		return ""
	}
	return collectors.GitBranchOrigin(p.Path, head)
}

func (a *App) gitOpenPullRequest(p *core.Project) {
	g := a.projectGitInfo(p)
	if g == nil {
		return
	}
	head := g.Branch
	if branch, ok := a.targetGitBranch(p); ok {
		head = branch
	}
	base := collectors.GitDefaultPRBase(p.Path, head)
	url := collectors.GitHubCompareURL(g.Remote, base, head)
	if url == "" {
		a.gitStatusMsg = "remote GitHub não detectado"
		return
	}
	_ = exec.Command("xdg-open", url).Start()
	a.gitStatusMsg = "abrindo PR: " + base + "..." + head
}

func (a *App) updateGitPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.gitPromptCursor = minInt(a.gitPromptCursor, len([]rune(a.gitPromptInput)))
	switch msg.String() {
	case "esc":
		a.gitPromptOn = false
		a.gitPromptInput = ""
		a.gitPromptCursor = 0
		a.gitPromptBranch = ""
		a.gitStatusMsg = ""
	case "enter":
		name := strings.TrimSpace(a.gitPromptInput)
		if name == "" {
			a.gitStatusMsg = "nome vazio"
			return a, nil
		}
		p := a.currentProject()
		if p == nil {
			a.gitPromptOn = false
			return a, nil
		}
		a.gitPromptOn = false
		switch a.gitPromptKind {
		case gitPromptNewBranch:
			from := a.gitPromptBranch
			a.gitPromptBranch = ""
			a.gitPromptInput = ""
			a.gitPromptCursor = 0
			return a, a.gitCreateBranch(p, name, from)
		case gitPromptRenameBranch:
			oldName := a.gitPromptBranch
			a.gitPromptBranch = ""
			a.gitPromptInput = ""
			a.gitPromptCursor = 0
			return a, a.gitRenameBranch(p, oldName, name)
		}
	case "left":
		if a.gitPromptCursor > 0 {
			a.gitPromptCursor--
		}
	case "right":
		if a.gitPromptCursor < len([]rune(a.gitPromptInput)) {
			a.gitPromptCursor++
		}
	case "home":
		a.gitPromptCursor = 0
	case "end":
		a.gitPromptCursor = len([]rune(a.gitPromptInput))
	case "backspace":
		runes := []rune(a.gitPromptInput)
		if a.gitPromptCursor > 0 {
			runes = append(runes[:a.gitPromptCursor-1], runes[a.gitPromptCursor:]...)
			a.gitPromptCursor--
			a.gitPromptInput = string(runes)
		}
	case "delete":
		runes := []rune(a.gitPromptInput)
		if a.gitPromptCursor < len(runes) {
			runes = append(runes[:a.gitPromptCursor], runes[a.gitPromptCursor+1:]...)
			a.gitPromptInput = string(runes)
		}
	default:
		if len(msg.Runes) > 0 {
			runes := []rune(a.gitPromptInput)
			inserted := append([]rune(nil), msg.Runes...)
			runes = append(runes[:a.gitPromptCursor], append(inserted, runes[a.gitPromptCursor:]...)...)
			a.gitPromptCursor += len(inserted)
			a.gitPromptInput = string(runes)
		} else if len(msg.String()) == 1 {
			runes := []rune(a.gitPromptInput)
			inserted := []rune(msg.String())
			runes = append(runes[:a.gitPromptCursor], append(inserted, runes[a.gitPromptCursor:]...)...)
			a.gitPromptCursor += len(inserted)
			a.gitPromptInput = string(runes)
		}
	}
	return a, nil
}

func (a *App) updateGitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := a.currentProject()
	switch msg.String() {
	case "y", "Y":
		a.gitConfirmOn = false
		if p == nil {
			return a, nil
		}
		branch := a.gitConfirmBranch
		action := a.gitConfirmAction
		a.gitConfirmBranch = ""
		a.gitConfirmAction = ""
		switch action {
		case "delete":
			return a, a.gitDeleteBranch(p, branch)
		case "merge":
			return a, a.gitMergeBranch(p, branch)
		}
	case "esc":
		a.gitConfirmOn = false
		a.gitConfirmBranch = ""
		a.gitConfirmAction = ""
		a.gitStatusMsg = "cancelado"
	}
	return a, nil
}

func (a *App) renderGitPrompt() string {
	background := a.renderProject()
	runes := []rune(a.gitPromptInput)
	a.gitPromptCursor = minInt(a.gitPromptCursor, len(runes))
	input := string(runes[:a.gitPromptCursor]) + "█" + string(runes[a.gitPromptCursor:])

	title := "Nova branch"
	context := ""
	footer := "enter cria  ·  esc cancela"
	if a.gitPromptKind == gitPromptRenameBranch {
		title = "Renomear branch"
		if a.gitPromptBranch != "" {
			context = StyleMuted.Render("atual  ") + StyleWarning.Render(a.gitPromptBranch)
		}
		footer = "enter renomeia  ·  esc cancela"
	} else if a.gitPromptBranch != "" {
		context = StyleMuted.Render("a partir de  ") + StyleWarning.Render(a.gitPromptBranch)
	}

	lines := []string{
		StyleSection.Render(title),
		StyleMuted.Render("digite o nome da branch"),
		"",
	}
	if context != "" {
		lines = append(lines, context, "")
	}
	lines = append(lines,
		StyleMuted.Render("nome"),
		StyleSelected.Render("▸ "+input),
		"",
		StyleMuted.Render(footer),
	)

	boxW := minInt(56, maxInt(36, a.width-10))
	box := StylePanel.
		Width(boxW).
		Background(ColorBgPanel).
		Render(strings.Join(lines, "\n"))
	return overlayCentered(background, box, a.width, a.height)
}
