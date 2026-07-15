package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

type gitSubview int

const (
	gitSubviewMain gitSubview = iota
	gitSubviewCommit
)

type gitFocus int

const (
	gitFocusBranches gitFocus = iota
	gitFocusCommits
	gitFocusFiles
)

type gitCommitDetailFocus int

const (
	gitCommitFocusMessage gitCommitDetailFocus = iota
	gitCommitFocusFiles
)

const (
	gitBranchColWidthMin = 34
	gitCommitColWidthMin = 72
)

func (a *App) gitListViewport() int {
	// Viewport = total panel content height minus internal git-panel chrome:
	// title(1) + status-bar(1) + scroll-up(1) + scroll-down(1) + remote(1) + 2 hint lines = 7
	v := a.contentPanelHeight() - 7
	if v < 8 {
		return 8
	}
	return v
}

func (a *App) gitPanelInnerLines() int {
	return a.gitListViewport() + 3 // title + scroll up + scroll down
}

func (a *App) gitBranchColWidth() int {
	if a.width <= 0 {
		return gitBranchColWidthMin
	}
	w := a.width / 4
	if w < gitBranchColWidthMin {
		return gitBranchColWidthMin
	}
	if w > 50 {
		return 50
	}
	return w
}

func (a *App) gitCommitColWidth() int {
	if a.width <= 0 {
		return 90
	}
	w := a.width - a.gitBranchColWidth() - 20
	if w < gitCommitColWidthMin {
		w = gitCommitColWidthMin
	}
	return w
}

func (a *App) renderGitTab(p *core.Project) string {
	if a.gitSubview == gitSubviewCommit {
		return a.renderGitCommitDetail(p)
	}

	title := StyleSection.Render("Git") + "  " + StyleMuted.Render(shortenPath(p.Path))
	g := a.projectGitInfo(p)
	if g == nil || !g.IsRepo {
		return StylePanel.Render(title + "\n\n" + StyleMuted.Render("Este diretório não é um repositório git."))
	}
	viewBranch := a.gitViewBranch
	if viewBranch == "" {
		viewBranch = g.Branch
	}

	// Status bar — inclui remote compacto ao final se disponível
	statusBar := a.renderGitStatusBar(g, viewBranch)
	if g.Remote != "" {
		remote := g.Remote
		remote = strings.TrimPrefix(remote, "https://")
		remote = strings.TrimPrefix(remote, "http://")
		remote = strings.TrimPrefix(remote, "git@")
		remote = strings.TrimSuffix(remote, ".git")
		statusBar += "   " + StyleMuted.Render("↗ "+truncate(remote, 45))
	}

	// Linha de notificação — sempre 1 linha fixa para evitar saltos de altura
	var notifLine string
	switch {
	case a.gitStatusMsg != "":
		style := StyleMuted
		if strings.Contains(a.gitStatusMsg, "✓") {
			style = StyleHealthy
		} else if strings.Contains(a.gitStatusMsg, "erro") || strings.Contains(a.gitStatusMsg, ":") {
			style = StyleWarning
		}
		notifLine = style.Render(a.gitStatusMsg)
	case a.gitCherryPickActive:
		src := a.gitCherryPickSourceBranch
		if src == "" {
			src = "?"
		}
		notifLine = StyleGitCherry.Render(
			"🍒 Cherry-pick de " + src + ": " + a.gitCherryPickSummary() + " — shift+v na branch destino",
		)
	case a.gitSelectedCommitCount() > 0:
		notifLine = StyleGitSelected.Render(
			fmt.Sprintf("✓ %d commit(s) selecionado(s) — shift+c para copiar cherry-pick", a.gitSelectedCommitCount()),
		)
	case a.gitActionLoading:
		notifLine = StyleMuted.Render("executando...")
	}

	sections := []string{
		title,
		statusBar,
		notifLine, // 1 linha fixa (vazia ou com mensagem)
		"",
		a.renderGitMainColumns(g, viewBranch),
	}

	if a.gitShowWorkingTree() {
		sections = append(sections, "", a.renderGitFiles(g, viewBranch))
	}

	return StylePanel.Render(strings.Join(sections, "\n"))
}

func (a *App) renderGitStatusBar(g *core.GitInfo, viewBranch string) string {
	head := StyleGitBranchHead.Render("◈ " + g.Branch)
	if g.Ahead > 0 || g.Behind > 0 {
		head += "  " + StyleAccent.Render(fmt.Sprintf("↑%d ↓%d", g.Ahead, g.Behind))
	}

	var wt string
	switch {
	case g.Modified == 0 && g.Untracked == 0:
		wt = StyleHealthy.Render("✓ clean")
	case g.Untracked > 0:
		wt = StyleWarning.Render(fmt.Sprintf("● %d modified · %d untracked", g.Modified, g.Untracked))
	default:
		wt = StyleWarning.Render(fmt.Sprintf("● %d modified", g.Modified))
	}

	parts := []string{head, wt}
	if viewBranch != g.Branch {
		parts = append(parts, StyleAccent.Render("👁 "+viewBranch))
	}
	if n := a.gitSelectedCommitCount(); n > 0 && !a.gitCherryPickActive {
		parts = append(parts, StyleGitSelected.Render(fmt.Sprintf("%d selected", n)))
	}
	if a.gitCherryPickActive {
		parts = append(parts, StyleGitCherry.Render(fmt.Sprintf("🍒 %d to paste", len(a.gitCherryPickBuffer))))
	}
	if a.gitMarkedBranch != "" {
		parts = append(parts, StyleGitMarked.Render("↑ "+a.gitMarkedBranch))
	}
	if g.StashCount > 0 {
		parts = append(parts, StyleMuted.Render(fmt.Sprintf("stash %d", g.StashCount)))
	}
	return strings.Join(parts, "   ")
}

func (a *App) renderGitMainColumns(g *core.GitInfo, viewBranch string) string {
	inner := a.gitPanelInnerLines()
	branchCol := fitGitPanelLines(a.renderGitBranches(g, viewBranch), inner)
	commitCol := fitGitPanelLines(a.renderGitCommits(viewBranch), inner)

	branchBox := StyleGitColumn.Width(a.gitBranchColWidth()).Height(inner).Render(branchCol)
	commitBox := StyleGitColumn.Width(a.gitCommitColWidth()).Height(inner).Render(commitCol)
	return lipgloss.JoinHorizontal(lipgloss.Top, branchBox, " ", commitBox)
}

func fitGitPanelLines(content string, lines int) string {
	parts := strings.Split(content, "\n")
	if len(parts) > lines {
		parts = parts[:lines]
	}
	for len(parts) < lines {
		parts = append(parts, "")
	}
	return strings.Join(parts, "\n")
}

func gitScrollUpLine(n int) string {
	if n > 0 {
		return StyleMuted.Render(fmt.Sprintf("  ↑ %d", n))
	}
	return " "
}

func gitScrollDownLine(n int) string {
	if n > 0 {
		return StyleMuted.Render(fmt.Sprintf("  ↓ %d", n))
	}
	return " "
}

func (a *App) renderGitCommitDetail(p *core.Project) string {
	c := a.gitSelectedCommit
	title := StyleSection.Render("Commit") + "  " + StyleMuted.Render("(somente leitura)")

	lines := []string{
		title,
		"",
		StyleTabActive.Render(c.Hash),
		StyleMuted.Render(fmt.Sprintf("  %s  •  %s", c.Author, c.Date)),
		"",
	}

	if a.gitCommitFilesLoading {
		lines = append(lines, StyleMuted.Render("Carregando commit..."))
		return StylePanel.Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, a.renderGitCommitMessageBox())
	lines = append(lines, "", a.renderGitCommitFilesList())

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderGitCommitMessageBox() string {
	msgLabel := StyleMuted.Render("Mensagem")
	if a.gitCommitDetailFocus == gitCommitFocusMessage {
		msgLabel = StyleTabActive.Render("Mensagem")
	}

	text := a.gitCommitFullMsg
	if text == "" {
		text = a.gitSelectedCommit.Message
	}
	msgWidth := maxInt(a.width-28, 40)
	msgLines := wrapText(text, msgWidth)
	viewport := a.gitCommitMessageViewport()

	if a.gitCommitMsgCursor >= len(msgLines) {
		a.gitCommitMsgCursor = maxInt(len(msgLines)-1, 0)
	}
	a.gitCommitMsgScroll = ensureVisible(a.gitCommitMsgCursor, a.gitCommitMsgScroll, viewport, len(msgLines))

	start := a.gitCommitMsgScroll
	end := minInt(start+viewport, len(msgLines))

	var boxContent []string
	if start > 0 {
		boxContent = append(boxContent, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
	}
	for i := start; i < end; i++ {
		line := msgLines[i]
		if line == "" {
			line = " "
		}
		rendered := "  " + line
		if a.gitCommitDetailFocus == gitCommitFocusMessage && i == a.gitCommitMsgCursor {
			boxContent = append(boxContent, StyleSelected.Render(rendered))
		} else {
			boxContent = append(boxContent, StyleNormal.Render(rendered))
		}
	}
	if end < len(msgLines) {
		boxContent = append(boxContent, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", len(msgLines)-end)))
	}

	box := StyleInnerPanel.Render(strings.Join(boxContent, "\n"))
	return msgLabel + "\n" + box
}

func (a *App) renderGitCommitFilesList() string {
	filesLabel := StyleMuted.Render("Arquivos alterados")
	if a.gitCommitDetailFocus == gitCommitFocusFiles {
		filesLabel = StyleTabActive.Render(fmt.Sprintf("Arquivos alterados (%d)", len(a.gitCommitFiles)))
	} else {
		filesLabel = StyleMuted.Render(fmt.Sprintf("Arquivos alterados (%d)", len(a.gitCommitFiles)))
	}

	lines := []string{filesLabel}
	files := a.gitCommitFiles
	if len(files) == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum arquivo)"))
		return strings.Join(lines, "\n")
	}

	viewport := a.gitCommitFilesViewport()
	start := a.gitCommitFileScroll
	end := minInt(start+viewport, len(files))
	if start > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
	}
	for i := start; i < end; i++ {
		f := files[i]
		line := fmt.Sprintf("  %s  %s", commitChangeStyle(f.Status), f.Path)
		if a.gitCommitDetailFocus == gitCommitFocusFiles && a.gitCommitFileCursor == i {
			lines = append(lines, StyleSelected.Render(line))
		} else {
			lines = append(lines, StyleNormal.Render(line))
		}
	}
	remaining := len(files) - end
	if remaining > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", remaining)))
	}
	return strings.Join(lines, "\n")
}

func (a *App) gitCommitMessageViewport() int {
	return 6
}

// gitPanelHeight is kept for compatibility but delegates to contentPanelHeight.
func (a *App) gitPanelHeight() int {
	return a.contentPanelHeight()
}

func (a *App) renderGitBranches(g *core.GitInfo, viewBranch string) string {
	branches := a.filteredGitBranches(a.gitBranchesForUI())
	title := "Branches"
	if a.gitFocus == gitFocusBranches {
		title = StyleTabActive.Render(title)
	}
	if a.gitBranchFilter != "" {
		title += StyleMuted.Render(fmt.Sprintf(" · %s", a.gitBranchFilter))
	}
	lines := []string{StyleSection.Render(title)}
	if len(branches) == 0 {
		lines = append(lines, gitScrollUpLine(0))
		for i := 0; i < a.gitListViewport(); i++ {
			if i == 0 {
				lines = append(lines, StyleMuted.Render("  (sem branches)"))
			} else {
				lines = append(lines, "")
			}
		}
		lines = append(lines, gitScrollDownLine(0))
		return strings.Join(lines, "\n")
	}

	viewport := a.gitListViewport()
	a.gitBranchScroll = ensureVisible(a.gitBranchCursor, a.gitBranchScroll, viewport, len(branches))
	start := a.gitBranchScroll
	end := minInt(start+viewport, len(branches))

	lines = append(lines, gitScrollUpLine(start))
	for i := start; i < end; i++ {
		lines = append(lines, a.renderGitBranchLine(branches[i], viewBranch, g.Branch, i))
	}
	for i := end - start; i < viewport; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, gitScrollDownLine(len(branches)-end))
	return strings.Join(lines, "\n")
}

func (a *App) renderGitBranchLine(b core.GitBranch, viewBranch, headBranch string, idx int) string {
	marker := "  "
	if b.Current {
		marker = "● "
	} else if b.Name == viewBranch {
		marker = "◎ "
	}

	viewing := b.Name == viewBranch && viewBranch != headBranch
	viewTag := ""
	if viewing {
		viewTag = " (view)"
	}

	nameMax := a.gitBranchColWidth() - 4 - len(viewTag)
	if nameMax < 8 {
		nameMax = 8
	}
	name := truncate(b.Name, nameMax)

	style := StyleNormal
	switch {
	case b.Name == a.gitMarkedBranch && a.gitFocus == gitFocusBranches && a.gitBranchCursor == idx:
		style = StyleGitMarkedCursor
	case b.Name == a.gitMarkedBranch:
		style = StyleGitMarked
	case a.gitFocus == gitFocusBranches && a.gitBranchCursor == idx:
		style = StyleSelected
	case b.Current:
		style = StyleGitBranchHead
	case b.Name == viewBranch:
		style = StyleTabActive
	}

	line := style.Render(marker + name)
	if viewing {
		line += StyleAccent.Render(viewTag)
	}
	return line
}

func (a *App) renderGitCommits(viewBranch string) string {
	title := "Commits"
	if a.gitFocus == gitFocusCommits {
		title = StyleTabActive.Render(title)
	}
	title += StyleMuted.Render(" · " + viewBranch)
	if a.gitBranchLoading {
		return fitGitPanelLines(
			StyleSection.Render(title)+"\n"+StyleMuted.Render("  carregando..."),
			a.gitPanelInnerLines(),
		)
	}

	commits := a.gitDisplayedCommits()
	if len(commits) == 0 {
		return fitGitPanelLines(
			StyleSection.Render(title)+"\n"+StyleMuted.Render("  (sem commits)"),
			a.gitPanelInnerLines(),
		)
	}

	viewport := a.gitListViewport()
	a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, viewport, len(commits))
	start := a.gitCommitScroll
	end := minInt(start+viewport, len(commits))

	lines := []string{StyleSection.Render(title)}
	lines = append(lines, gitScrollUpLine(start))
	for i := start; i < end; i++ {
		lines = append(lines, a.renderGitCommitLine(commits[i], i))
	}
	for i := end - start; i < viewport; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, gitScrollDownLine(len(commits)-end))
	return strings.Join(lines, "\n")
}

func (a *App) renderGitCommitLine(c core.GitCommit, idx int) string {
	cherry := a.isGitCommitInCherryBuffer(c.Hash)
	selected := a.isGitCommitSelected(c.Hash)
	cursor := a.gitFocus == gitFocusCommits && a.gitCommitCursor == idx

	marker := ""
	if cherry {
		marker = " 🍒"
	} else if selected {
		marker = " ✓"
	}

	colW := a.gitCommitColWidth() - 6
	msgW := maxInt(colW-28, 24)
	line := fmt.Sprintf("  %-9s  %-*s  %s%s",
		c.Hash, msgW, truncate(c.Message, msgW), truncate(c.Author, 16), marker)

	switch {
	case cherry && cursor:
		return StyleGitCherryCursor.Render(line)
	case cherry:
		return StyleGitCherry.Render(line)
	case selected && cursor:
		return StyleGitCherryCursor.Render(line)
	case selected:
		return StyleGitSelected.Render(line)
	case cursor:
		return StyleSelected.Render(line)
	default:
		return StyleNormal.Render(line)
	}
}

func (a *App) renderGitFiles(g *core.GitInfo, viewBranch string) string {
	if viewBranch != g.Branch {
		return StyleMuted.Render(fmt.Sprintf("Working Tree — checkout %s (space) para ver alterações locais", g.Branch))
	}

	title := "Working Tree"
	if a.gitFocus == gitFocusFiles {
		title = StyleTabActive.Render(title)
	}
	lines := []string{StyleSection.Render(title + StyleMuted.Render(fmt.Sprintf(" (%d)", len(g.Files))))}
	if len(g.Files) == 0 {
		lines = append(lines, StyleHealthy.Render("  ✓ working tree limpo"))
		return strings.Join(lines, "\n")
	}

	viewport := a.gitFilesViewport()
	a.gitFileScroll = ensureVisible(a.gitFileCursor, a.gitFileScroll, viewport, len(g.Files))
	start := a.gitFileScroll
	end := minInt(start+viewport, len(g.Files))

	if start > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d", start)))
	}
	for i := start; i < end; i++ {
		f := g.Files[i]
		code := gitStatusLabel(f.Staging, f.Worktree)
		line := fmt.Sprintf("  %s  %s", gitStatusStyle(code), f.Path)
		if a.gitFocus == gitFocusFiles && a.gitFileCursor == i {
			lines = append(lines, StyleSelected.Render(line))
		} else {
			lines = append(lines, StyleNormal.Render(line))
		}
	}
	remaining := len(g.Files) - end
	if remaining > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d", remaining)))
	}
	return strings.Join(lines, "\n")
}

func (a *App) gitBranchViewport() int {
	return a.gitListViewport()
}

func (a *App) gitCommitsViewport() int {
	return a.gitListViewport()
}

func (a *App) gitFilesViewport() int {
	return 3
}

func (a *App) gitCommitFilesViewport() int {
	return 8
}

func (a *App) gitShowWorkingTree() bool {
	return false
}

func commitChangeStyle(status string) string {
	if status == "" {
		return StyleMuted.Render("?")
	}
	switch status[0] {
	case 'A':
		return StyleHealthy.Render(padRight(status, 2))
	case 'D':
		return StyleUnhealthy.Render(padRight(status, 2))
	case 'M':
		return StyleWarning.Render(padRight(status, 2))
	case 'R':
		return StyleAccent.Render(padRight(status, 3))
	default:
		return StyleMuted.Render(padRight(status, 2))
	}
}

func (a *App) initGitTab(p *core.Project) {
	a.gitSubview = gitSubviewMain
	a.gitFocus = gitFocusBranches
	a.gitBranchCursor = 0
	a.gitBranchScroll = 0
	a.gitCommitCursor = 0
	a.gitCommitScroll = 0
	a.gitFileCursor = 0
	a.gitFileScroll = 0
	a.gitCommitFileCursor = 0
	a.gitCommitFileScroll = 0
	a.gitCommitMsgScroll = 0
	a.gitCommitMsgCursor = 0
	a.gitCommitDetailFocus = gitCommitFocusMessage
	a.gitCommitFiles = nil
	a.gitCommitFilesLoading = false
	a.gitSelectedCommit = core.GitCommit{}
	a.gitBranchLoading = false
	a.gitCommitFullMsg = ""
	a.gitBranchFilterOn = false
	a.gitBranchFilterInput = ""
	a.gitBranchFilter = ""
	a.clearGitCommitSelection()
	a.gitCherryPickBuffer = nil
	a.gitCherryPickMarked = nil
	a.gitCherryPickActive = false
	a.gitCherryPickSourceBranch = ""
	a.gitStatusMsg = ""
	a.gitActionLoading = false
	a.gitPromptOn = false
	a.gitPromptInput = ""
	a.gitPromptBranch = ""
	a.gitConfirmOn = false
	a.gitConfirmAction = ""
	a.gitConfirmBranch = ""
	a.gitBranchLoadGen = 0
	a.gitMarkedBranch = ""
	a.gitBranches = nil
	a.gitBranchDenylist = nil
	a.gitCommitSelectAnchor = -1

	if p.Git == nil || !p.Git.IsRepo {
		a.gitViewBranch = ""
		a.gitBranchCommits = nil
		a.gitRenderCache = nil
		return
	}

	a.gitViewBranch = p.Git.Branch
	a.gitBranchCommits = p.Git.Commits
	a.syncGitBranchesFrom(p)
	a.syncGitBranchCursor(a.gitBranchesForUI())
}

func (a *App) filteredGitBranches(branches []core.GitBranch) []core.GitBranch {
	if a.gitBranchFilter == "" {
		return branches
	}
	f := strings.ToLower(a.gitBranchFilter)
	var out []core.GitBranch
	for _, b := range branches {
		if strings.Contains(strings.ToLower(b.Name), f) {
			out = append(out, b)
		}
	}
	return out
}

func (a *App) syncGitBranchCursor(branches []core.GitBranch) {
	filtered := a.filteredGitBranches(branches)
	for i, b := range filtered {
		if b.Name == a.gitViewBranch {
			a.gitBranchCursor = i
			a.gitBranchScroll = ensureVisible(i, a.gitBranchScroll, a.gitBranchViewport(), len(filtered))
			return
		}
	}
	a.gitBranchCursor = 0
	a.gitBranchScroll = 0
}

func (a *App) gitFocusNext() {
	if a.gitSubview == gitSubviewCommit {
		return
	}
	switch a.gitFocus {
	case gitFocusBranches:
		a.gitFocus = gitFocusCommits
	case gitFocusCommits:
		if a.gitShowWorkingTree() {
			a.gitFocus = gitFocusFiles
		} else {
			a.gitFocus = gitFocusBranches
		}
	default:
		a.gitFocus = gitFocusBranches
	}
}

func (a *App) gitFocusPrev() {
	if a.gitSubview == gitSubviewCommit {
		return
	}
	switch a.gitFocus {
	case gitFocusFiles:
		a.gitFocus = gitFocusCommits
	case gitFocusCommits:
		a.gitFocus = gitFocusBranches
	default:
		if a.gitShowWorkingTree() {
			a.gitFocus = gitFocusFiles
		} else {
			a.gitFocus = gitFocusCommits
		}
	}
}

func (a *App) updateGitCursor(delta int, p *core.Project, shift bool) tea.Cmd {
	if p.Git == nil {
		return nil
	}

	if a.gitSubview == gitSubviewCommit {
		text := a.gitCommitFullMsg
		if text == "" {
			text = a.gitSelectedCommit.Message
		}
		msgLines := wrapText(text, maxInt(a.width-28, 40))

		if a.gitCommitDetailFocus == gitCommitFocusMessage {
			if len(msgLines) == 0 {
				return nil
			}
			viewport := a.gitCommitMessageViewport()
			a.gitCommitMsgCursor = clampCursor(a.gitCommitMsgCursor+delta, len(msgLines))
			a.gitCommitMsgScroll = ensureVisible(a.gitCommitMsgCursor, a.gitCommitMsgScroll, viewport, len(msgLines))
			return nil
		}

		if len(a.gitCommitFiles) == 0 {
			return nil
		}
		viewport := a.gitCommitFilesViewport()
		a.gitCommitFileCursor = clampCursor(a.gitCommitFileCursor+delta, len(a.gitCommitFiles))
		a.gitCommitFileScroll = ensureVisible(a.gitCommitFileCursor, a.gitCommitFileScroll, viewport, len(a.gitCommitFiles))
		return nil
	}

	switch a.gitFocus {
	case gitFocusBranches:
		branches := a.filteredGitBranches(a.gitBranchesForUI())
		if len(branches) == 0 {
			return nil
		}
		viewport := a.gitBranchViewport()
		a.gitBranchCursor = clampCursor(a.gitBranchCursor+delta, len(branches))
		a.gitBranchScroll = ensureVisible(a.gitBranchCursor, a.gitBranchScroll, viewport, len(branches))
		branch := branches[a.gitBranchCursor].Name
		return a.selectGitBranch(p, branch)

	case gitFocusCommits:
		commits := a.gitDisplayedCommits()
		if len(commits) == 0 {
			return nil
		}
		prev := a.gitCommitCursor
		viewport := a.gitCommitsViewport()
		a.gitCommitCursor = clampCursor(a.gitCommitCursor+delta, len(commits))
		a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, viewport, len(commits))

		if shift {
			if a.gitCommitSelectAnchor < 0 {
				a.gitCommitSelectAnchor = prev
			}
			a.gitSelectCommitRange(commits, a.gitCommitSelectAnchor, a.gitCommitCursor)
		} else {
			a.gitCommitSelectAnchor = a.gitCommitCursor
		}

	case gitFocusFiles:
		if a.gitViewBranch != "" && a.gitViewBranch != p.Git.Branch {
			return nil
		}
		if len(p.Git.Files) == 0 {
			return nil
		}
		viewport := a.gitFilesViewport()
		a.gitFileCursor = clampCursor(a.gitFileCursor+delta, len(p.Git.Files))
		a.gitFileScroll = ensureVisible(a.gitFileCursor, a.gitFileScroll, viewport, len(p.Git.Files))
	}
	return nil
}

func (a *App) gitSelectCommitRange(commits []core.GitCommit, anchor, cursor int) {
	if anchor < 0 || cursor < 0 {
		return
	}
	lo := minInt(anchor, cursor)
	hi := maxInt(anchor, cursor)
	if a.gitSelectedCommits == nil {
		a.gitSelectedCommits = make(map[string]bool)
	}
	for i := lo; i <= hi && i < len(commits); i++ {
		a.gitSelectedCommits[commits[i].Hash] = true
	}
}

func (a *App) gitDisplayedCommits() []core.GitCommit {
	if len(a.gitBranchCommits) > 0 {
		return a.gitBranchCommits
	}
	return nil
}

func (a *App) selectGitBranch(p *core.Project, branch string) tea.Cmd {
	if branch == "" || branch == a.gitViewBranch {
		return nil
	}
	a.gitViewBranch = branch
	a.gitCommitCursor = 0
	a.gitCommitScroll = 0
	a.clearGitCommitSelection()
	return a.requestGitBranchCommits(p.Path, branch)
}

func (a *App) openGitCommitDetail(p *core.Project, commit core.GitCommit) tea.Cmd {
	a.gitSubview = gitSubviewCommit
	a.gitSelectedCommit = commit
	a.gitCommitFiles = nil
	a.gitCommitFullMsg = ""
	a.gitCommitFilesLoading = true
	a.gitCommitFileCursor = 0
	a.gitCommitFileScroll = 0
	a.gitCommitMsgScroll = 0
	a.gitCommitMsgCursor = 0
	a.gitCommitDetailFocus = gitCommitFocusMessage
	return loadGitCommitDetail(p.Path, commit.Hash)
}

func (a *App) gitSpaceAction(p *core.Project) tea.Cmd {
	if p.Git == nil || !p.Git.IsRepo {
		return nil
	}
	switch a.gitFocus {
	case gitFocusBranches:
		branches := a.filteredGitBranches(a.gitBranchesForUI())
		if a.gitBranchCursor >= len(branches) {
			return nil
		}
		branch := branches[a.gitBranchCursor].Name
		if branch == p.Git.Branch {
			return a.selectGitBranch(p, branch)
		}
		return a.gitCheckoutBranch(p, branch)
	case gitFocusCommits:
		a.toggleGitCommitSelection(p)
	}
	return nil
}
