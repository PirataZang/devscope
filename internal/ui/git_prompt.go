package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	a.gitStatusMsg = "modal delete  y confirma  n/esc cancela"
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
	a.gitStatusMsg = "modal merge  y confirma  n/esc cancela"
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

func (a *App) openPullStrategyModal(source string) {
	a.gitConfirmOn = true
	a.gitConfirmAction = "pull-strategy"
	a.gitConfirmBranch = source
	a.gitStatusMsg = "fast-forward impossível — m merge  r rebase  esc cancela"
}

func (a *App) updateGitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := a.currentProject()
	action := a.gitConfirmAction
	if action == "pull-strategy" {
		switch msg.String() {
		case "m", "M":
			a.gitConfirmOn = false
			source := a.gitConfirmBranch
			a.gitConfirmBranch = ""
			a.gitConfirmAction = ""
			if p == nil {
				return a, nil
			}
			return a, a.gitPullMerge(p, source)
		case "r", "R":
			a.gitConfirmOn = false
			source := a.gitConfirmBranch
			a.gitConfirmBranch = ""
			a.gitConfirmAction = ""
			if p == nil {
				return a, nil
			}
			return a, a.gitPullRebase(p, source)
		case "esc", "n", "N":
			a.gitConfirmOn = false
			a.gitConfirmBranch = ""
			a.gitConfirmAction = ""
			a.gitStatusMsg = "pull cancelado"
		}
		return a, nil
	}
	switch msg.String() {
	case "y", "Y":
		a.gitConfirmOn = false
		if p == nil {
			return a, nil
		}
		branch := a.gitConfirmBranch
		a.gitConfirmBranch = ""
		a.gitConfirmAction = ""
		switch action {
		case "delete":
			return a, a.gitDeleteBranch(p, branch)
		case "merge":
			return a, a.gitMergeBranch(p, branch)
		}
	case "esc", "n", "N":
		a.gitConfirmOn = false
		a.gitConfirmBranch = ""
		a.gitConfirmAction = ""
		a.gitStatusMsg = "cancelado"
	}
	return a, nil
}

func (a *App) renderGitConfirm() string {
	background := a.renderProject()
	w, h := maxInt(40, a.width), maxInt(12, a.height)
	branch := firstNonEmpty(a.gitConfirmBranch, "—")
	if a.gitConfirmAction == "pull-strategy" {
		return overlayCentered(background, a.renderPullStrategyBox(branch, w, h), w, h)
	}
	opts := deleteConfirmOpts{
		Brand:  "GIT",
		Color:  tabAccentColor(TabGit),
		Target: branch,
		Label:  "branch",
	}
	switch a.gitConfirmAction {
	case "merge":
		into := "HEAD"
		if p := a.currentProject(); p != nil {
			if g := a.projectGitInfo(p); g != nil && g.Branch != "" {
				into = g.Branch
			}
		}
		opts.Title = "Mesclar branch"
		opts.Subtitle = "git merge na branch atual"
		opts.Detail = truncate(branch+"  →  "+into, 48)
	default:
		opts.Title = "Excluir branch"
		opts.Subtitle = "git branch -D — remove a branch local"
		opts.Detail = "branch local"
	}
	box := renderDeleteConfirmBox(opts, w, h)
	return overlayCentered(background, box, w, h)
}

func (a *App) renderPullStrategyBox(source string, width, height int) string {
	color := tabAccentColor(TabGit)
	boxW := minInt(width-4, maxInt(44, width*50/100))
	boxH := minInt(height-2, maxInt(12, 14))
	innerW := maxInt(28, boxW-6)
	lines := tunnelModalChrome("GIT", color, "Pull divergente", "fast-forward impossível — escolha a estratégia", "", innerW)
	lines = append(lines, "")
	nameBox := renderApiTitledBox("origem",
		[]string{StyleWarning.Bold(true).Render(truncate("origin/"+firstNonEmpty(source, "—"), innerW-2))},
		innerW, 3, true,
	)
	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "",
		StyleMuted.Render("m merge (--no-ff)  ·  r rebase  ·  esc cancela"),
	)
	return tunnelModalBox(lines, boxW, boxH, color)
}

func (a *App) renderGitPrompt() string {
	background := a.renderProject()
	runes := []rune(a.gitPromptInput)
	a.gitPromptCursor = minInt(a.gitPromptCursor, len(runes))
	typed := string(runes[:a.gitPromptCursor]) + "█" + string(runes[a.gitPromptCursor:])
	name := strings.TrimSpace(a.gitPromptInput)

	boxW := minInt(a.width-4, maxInt(52, a.width*62/100))
	boxH := minInt(a.height-2, maxInt(20, a.height*60/100))
	innerW := maxInt(28, boxW-6)

	isRename := a.gitPromptKind == gitPromptRenameBranch
	brand := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render("GIT")
	title := "Nova branch"
	subtitle := "criar a partir de outra branch"
	baseLabel := "a partir de"
	actionHint := "enter cria"
	if isRename {
		title = "Renomear branch"
		subtitle = "definir um novo nome"
		baseLabel = "branch atual"
		actionHint = "enter renomeia"
	}

	baseBranch := strings.TrimSpace(a.gitPromptBranch)
	if baseBranch == "" {
		baseBranch = "—"
	}

	proj := ""
	if p := a.currentProject(); p != nil {
		proj = p.Name
	}

	lines := []string{
		brand + StyleMuted.Render("  ·  ") + StyleNormal.Render(title),
		StyleMuted.Render(subtitle),
		StyleMuted.Render(strings.Repeat("─", minInt(innerW, 48))),
	}
	if proj != "" {
		lines = append(lines, StyleMuted.Render("projeto  ")+StyleNormal.Render(truncate(proj, maxInt(12, innerW-10))))
	}

	baseBox := renderApiTitledBox(baseLabel,
		[]string{StyleWarning.Bold(true).Render(truncate(baseBranch, innerW-2))},
		innerW, 3, false,
	)
	nameBox := renderApiTitledBox("nome",
		[]string{StyleSelected.Render(truncate(typed, innerW-2))},
		innerW, 3, true,
	)

	preview := StyleMuted.Render("preview  ")
	if name == "" {
		preview += StyleMuted.Render("(digite um nome)")
	} else {
		preview += StyleWarning.Render(truncate(baseBranch, 18)) +
			StyleMuted.Render("  →  ") +
			StyleHealthy.Render(truncate(name, 22))
	}

	lines = append(lines, "")
	lines = append(lines, strings.Split(baseBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "",
		preview,
		StyleMuted.Render("letras, números, / e -  ·  sem espaços"),
		"",
		StyleMuted.Render(fmt.Sprintf("%s  ·  ←→ move cursor  ·  esc cancela", actionHint)),
	)

	box := StylePanel.
		Width(boxW).
		BorderForeground(ColorAccent).
		Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
	return overlayCentered(background, box, a.width, a.height)
}
