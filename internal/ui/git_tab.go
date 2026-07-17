package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

type gitSubview int

const (
	gitSubviewMain gitSubview = iota
	gitSubviewBranch
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
	gitCommitFocusFiles gitCommitDetailFocus = iota
	gitCommitFocusDiff
)

type gitDiffLine struct {
	kind   string
	oldNum int
	newNum int
	text   string
}

const (
	gitBranchColWidthMin = 14
	gitCommitColWidthMin = 20
)

func (a *App) gitListViewport() int {
	// Total panel height = gitListViewport() + 3 (internal column chrome) + 6 (external git panel chrome)
	v := a.contentPanelHeight() - 9
	if v < 6 {
		return 6
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
	w := (a.width - 15) * 32 / 100
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
	switch a.gitSubview {
	case gitSubviewBranch:
		return a.renderGitBranchHistory(p)
	case gitSubviewCommit:
		return a.renderGitCommitDetail(p)
	}

	current := a.currentProject()
	if current == nil {
		current = p
	}
	title := StyleSection.Render("Git") + "  " + StyleMuted.Render(shortenPath(current.Path))
	g := current.Git
	if a.projectGitLoading && (g == nil || !g.IsRepo || len(g.Branches) == 0) {
		return StylePanel.Render(title + "\n\n" + StyleMuted.Render("Carregando informações do Git..."))
	}
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

	sections = append(sections, "",
		StyleMuted.Render("Atalhos: space checkout │ x/shift+c/shift+v cherry-pick │ n/d/R/M branch │ p/P pull/push"),
	)

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

func (a *App) renderGitBranchHistory(p *core.Project) string {
	height := maxInt(12, a.height-2)
	viewport := maxInt(1, height-7)
	branch := a.gitViewBranch
	if branch == "" && p.Git != nil {
		branch = p.Git.Branch
	}

	title := StyleSection.Render(truncate("Commits  "+branch, maxInt(10, a.width-6)))
	var body []string
	position := ""

	if a.gitBranchLoading {
		body = append(body, StyleMuted.Render("  Carregando commits..."))
	} else {
		commits := a.gitDisplayedCommits()
		if len(commits) == 0 {
			body = append(body, StyleMuted.Render("  (sem commits)"))
		} else {
			a.gitCommitCursor = clampCursor(a.gitCommitCursor, len(commits))
			a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, viewport, len(commits))
			start := a.gitCommitScroll
			end := minInt(start+viewport, len(commits))
			for i := start; i < end; i++ {
				body = append(body, a.renderGitBranchCommitLine(commits[i], i, maxInt(10, a.width-8)))
			}
			position = fmt.Sprintf("%d/%d", a.gitCommitCursor+1, len(commits))
		}
	}
	for len(body) < viewport {
		body = append(body, "")
	}

	footer := StyleMuted.Render(truncate(
		"↑↓ navegar  enter abrir commit  "+position+"  esc voltar",
		maxInt(10, a.width-6),
	))
	panel := StylePanel.
		Width(maxInt(10, a.width-6)).
		Render(strings.Join([]string{title, strings.Join(body, "\n"), footer}, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left,
		fitProjectPanel(panel, a.width, height),
		a.renderStatusBar("histórico da branch"),
	)
}

func (a *App) renderGitBranchCommitLine(c core.GitCommit, idx, width int) string {
	marker := "  "
	if a.gitCommitCursor == idx {
		marker = "▶ "
	}
	msgW := maxInt(8, width-28)
	line := fmt.Sprintf("%s%-9s  %-*s  %s",
		marker,
		truncate(c.Hash, 9),
		msgW,
		truncate(c.Message, msgW),
		truncate(c.Author, 14),
	)
	line = truncate(line, width)
	if a.gitCommitCursor == idx {
		return StyleSelected.Render(line)
	}
	return StyleNormal.Render(line)
}

func (a *App) renderGitCommitDetail(p *core.Project) string {
	height := maxInt(12, a.height-2)
	panelW := maxInt(20, a.width)
	innerW := maxInt(16, panelW-4) // border only; avoid panel padding wrap
	c := a.gitSelectedCommit
	headerLines := a.renderGitCommitHeaderLines(innerW)
	bodyHeight := maxInt(6, height-len(headerLines)-3) // header + footer + status
	filesW := a.gitCommitFilesPanelWidth()
	if filesW+21 > innerW {
		filesW = maxInt(16, innerW/3)
	}
	diffW := maxInt(20, innerW-filesW-1)

	filesInner := maxInt(8, filesW-2)
	diffInner := maxInt(12, diffW-2)
	filesCol := renderGitFixedBox(a.renderGitCommitFilesSidebarLines(bodyHeight, filesInner), filesW, bodyHeight)
	diffCol := renderGitFixedBox(a.renderGitCommitDiffPanelLines(bodyHeight, diffInner), diffW, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, filesCol, diffCol)

	position := a.gitCommitDiffPosition(maxInt(1, bodyHeight-1))
	hHint := ""
	textW := maxInt(8, diffInner-11)
	if maxH := a.gitCommitDiffMaxHScroll(textW); maxH > 0 || a.gitCommitDiffHScroll > 0 {
		hHint = fmt.Sprintf("  ↔ col %d", a.gitCommitDiffHScroll)
	}
	searchHint := ""
	if a.gitDiffSearchQuery != "" {
		matches := a.gitDiffSearchMatches()
		if len(matches) == 0 {
			searchHint = "  /0"
		} else {
			searchHint = fmt.Sprintf("  /%d/%d", a.gitDiffSearchIdx+1, len(matches))
		}
	}
	footer := StyleMuted.Render(truncate(
		"tab foco  ←→ lateral  n/p arq  ↑↓"+searchHint+hHint+"  "+position+"  esc",
		innerW,
	))

	content := append(append([]string{}, headerLines...), body, footer)
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(panelW).
		Render(strings.Join(content, "\n"))
	panel = clampRenderedHeight(panel, height)
	return lipgloss.JoinVertical(lipgloss.Left,
		panel,
		a.renderStatusBar("diff do commit · "+c.Hash),
	)
}

func renderGitFixedBox(lines []string, width, height int) string {
	inner := maxInt(4, width-2)
	body := make([]string, 0, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		// Force exact visual width so JoinHorizontal never wraps.
		if lipgloss.Width(line) > inner {
			line = truncate(line, inner)
		}
		body = append(body, padRightVisible(line, inner))
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Width(width)
	return border.Render(strings.Join(body, "\n"))
}

func padRightVisible(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func clampRenderedHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (a *App) renderGitCommitHeaderLines(width int) []string {
	c := a.gitSelectedCommit
	msg := a.gitCommitFullMsg
	if msg == "" {
		msg = c.Message
	}
	if width < 20 {
		width = 20
	}
	lines := []string{
		StyleSection.Render(truncate("Commit  "+c.Hash, width)),
		StyleMuted.Render(truncate(fmt.Sprintf("%s  •  %s", c.Author, c.Date), width)),
	}

	msgLines := wrapText(msg, width)
	if a.gitCommitMsgExpanded {
		limit := minInt(len(msgLines), 8)
		for i := 0; i < limit; i++ {
			lines = append(lines, StyleNormal.Render(truncate(msgLines[i], width)))
		}
		if len(msgLines) > 1 {
			lines = append(lines, StyleMuted.Render(truncate("m recolher", width)))
		}
	} else {
		summary := c.Message
		if summary == "" && len(msgLines) > 0 {
			summary = msgLines[0]
		}
		if len(msgLines) > 1 {
			lines = append(lines, StyleNormal.Render(truncate(summary, maxInt(8, width-12)))+StyleMuted.Render("  m+"))
		} else {
			lines = append(lines, StyleNormal.Render(truncate(summary, width)))
		}
	}
	return lines
}

func (a *App) gitCommitFilesPanelWidth() int {
	w := a.width * 28 / 100
	if w < 20 {
		w = 20
	}
	if w > 36 {
		w = 36
	}
	return w
}

func (a *App) renderGitCommitFilesSidebarLines(height, width int) []string {
	title := StyleSection.Render(truncate("Arquivos", width))
	if a.gitCommitDetailFocus == gitCommitFocusFiles {
		title = StyleTabActive.Render(truncate("Arquivos", width))
	}
	lines := []string{title}

	if a.gitCommitFilesLoading {
		lines = append(lines, StyleMuted.Render(truncate("carregando...", width)))
		return fitExactLines(lines, height)
	}
	files := a.gitCommitFiles
	if len(files) == 0 {
		lines = append(lines, StyleMuted.Render(truncate("(nenhum)", width)))
		return fitExactLines(lines, height)
	}

	viewport := maxInt(1, height-1)
	a.gitCommitFileCursor = clampCursor(a.gitCommitFileCursor, len(files))
	a.gitCommitFileScroll = ensureVisible(a.gitCommitFileCursor, a.gitCommitFileScroll, viewport, len(files))
	start := a.gitCommitFileScroll
	end := minInt(start+viewport, len(files))
	for i := start; i < end; i++ {
		f := files[i]
		status := commitChangeStylePlain(f.Status)
		name := filepathBase(f.Path)
		text := truncate(status+" "+name, width)
		switch {
		case a.gitCommitDetailFocus == gitCommitFocusFiles && a.gitCommitFileCursor == i:
			lines = append(lines, StyleSelected.Render(truncate("▶ "+status+" "+name, width)))
		case a.gitCommitFileCursor == i:
			lines = append(lines, StyleTabActive.Render(truncate("• "+status+" "+name, width)))
		default:
			lines = append(lines, StyleNormal.Render(text))
		}
	}
	return fitExactLines(lines, height)
}

func fitExactLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func filepathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 && i+1 < len(path) {
		return path[i+1:]
	}
	return path
}

func commitChangeStylePlain(status string) string {
	if status == "" {
		return "?"
	}
	return status
}

func (a *App) renderGitCommitDiffPanelLines(height, width int) []string {
	titleText := "Diff"
	if a.gitCommitFileCursor < len(a.gitCommitFiles) {
		titleText = "Diff · " + a.gitCommitFiles[a.gitCommitFileCursor].Path
	}
	title := StyleSection.Render(truncate(titleText, width))
	if a.gitCommitDetailFocus == gitCommitFocusDiff {
		title = StyleTabActive.Render(truncate(titleText, width))
	}
	lines := []string{title}
	viewport := maxInt(1, height-1)

	if a.gitCommitFilesLoading || a.gitCommitDiffLoading {
		lines = append(lines, StyleMuted.Render(truncate("Carregando diff...", width)))
		return fitExactLines(lines, height)
	}

	all := a.parseGitDiffLines()
	textW := maxInt(8, width-11) // line numbers gutter
	a.gitCommitDiffHScroll = clampScroll(a.gitCommitDiffHScroll, textW, a.gitCommitDiffMaxLineWidth())
	a.gitCommitDiffScroll = clampScroll(a.gitCommitDiffScroll, viewport, len(all))
	start := a.gitCommitDiffScroll
	end := minInt(start+viewport, len(all))
	matchSet := a.gitDiffMatchLineSet()
	currentMatch := -1
	if matches := a.gitDiffSearchMatches(); len(matches) > 0 && a.gitDiffSearchIdx < len(matches) {
		currentMatch = matches[a.gitDiffSearchIdx]
	}
	for i := start; i < end; i++ {
		lines = append(lines, renderGitDiffLine(all[i], width, a.gitCommitDiffHScroll, matchSet[i], i == currentMatch))
	}
	return fitExactLines(lines, height)
}

func (a *App) renderGitCommitDiffBody(viewport int) string {
	diffInner := maxInt(12, a.width-a.gitCommitFilesPanelWidth()-8)
	return strings.Join(a.renderGitCommitDiffPanelLines(viewport+1, diffInner), "\n")
}

func (a *App) gitCommitDiffMaxLineWidth() int {
	maxW := 0
	for _, line := range a.parseGitDiffLines() {
		if w := lipgloss.Width(line.text); w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (a *App) gitCommitDiffMaxHScroll(viewWidth int) int {
	return maxInt(0, a.gitCommitDiffMaxLineWidth()-viewWidth)
}

func (a *App) gitCommitDiffHScrollBy(delta int) {
	textW := maxInt(8, a.width-a.gitCommitFilesPanelWidth()-8-11)
	maxH := a.gitCommitDiffMaxHScroll(textW)
	a.gitCommitDiffHScroll += delta
	if a.gitCommitDiffHScroll < 0 {
		a.gitCommitDiffHScroll = 0
	}
	if a.gitCommitDiffHScroll > maxH {
		a.gitCommitDiffHScroll = maxH
	}
}

func (a *App) gitCommitDiffLines() []string {
	parsed := a.parseGitDiffLines()
	out := make([]string, len(parsed))
	for i, line := range parsed {
		out[i] = line.text
	}
	return out
}

func (a *App) parseGitDiffLines() []gitDiffLine {
	content := a.gitCommitDiff
	if content == "" {
		return []gitDiffLine{{kind: "meta", text: "(vazio)"}}
	}
	raw := strings.Split(content, "\n")
	if len(raw) == 1 && raw[0] == "" {
		return []gitDiffLine{{kind: "meta", text: "(vazio)"}}
	}

	var out []gitDiffLine
	oldN, newN := 0, 0
	for _, line := range raw {
		line = sanitizeTerminalLine(line)
		switch {
		case strings.HasPrefix(line, "@@"):
			o, n := parseDiffHunkHeader(line)
			if o > 0 {
				oldN = o
			}
			if n > 0 {
				newN = n
			}
			out = append(out, gitDiffLine{kind: "hunk", text: line})
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			out = append(out, gitDiffLine{kind: "meta", text: line})
		case strings.HasPrefix(line, "+"):
			out = append(out, gitDiffLine{kind: "add", newNum: newN, text: line})
			newN++
		case strings.HasPrefix(line, "-"):
			out = append(out, gitDiffLine{kind: "remove", oldNum: oldN, text: line})
			oldN++
		default:
			out = append(out, gitDiffLine{kind: "context", oldNum: oldN, newNum: newN, text: line})
			if oldN > 0 {
				oldN++
			}
			if newN > 0 {
				newN++
			}
		}
	}
	return out
}

func parseDiffHunkHeader(line string) (oldStart, newStart int) {
	// @@ -12,3 +14,4 @@
	_, rest, ok := strings.Cut(line, "@@")
	if !ok {
		return 0, 0
	}
	rest = strings.TrimSpace(rest)
	if i := strings.Index(rest, "@@"); i >= 0 {
		rest = strings.TrimSpace(rest[:i])
	}
	parts := strings.Fields(rest)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			oldStart = atoiPrefix(strings.TrimPrefix(part, "-"))
		}
		if strings.HasPrefix(part, "+") {
			newStart = atoiPrefix(strings.TrimPrefix(part, "+"))
		}
	}
	return oldStart, newStart
}

func atoiPrefix(s string) int {
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = s[:i]
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func (a *App) gitCommitDiffPosition(viewport int) string {
	if a.gitCommitDiffLoading || a.gitCommitFilesLoading {
		return ""
	}
	all := a.parseGitDiffLines()
	if len(all) == 0 {
		return "0/0"
	}
	start := a.gitCommitDiffScroll
	end := minInt(start+viewport, len(all))
	return fmt.Sprintf("%d-%d/%d", start+1, end, len(all))
}

func renderGitDiffLine(line gitDiffLine, width, hScroll int, matched, current bool) string {
	numW := 4
	oldS, newS := "    ", "    "
	if line.oldNum > 0 {
		oldS = fmt.Sprintf("%*d", numW, line.oldNum)
	}
	if line.newNum > 0 {
		newS = fmt.Sprintf("%*d", numW, line.newNum)
	}
	gutter := oldS + " " + newS + "│"
	textW := maxInt(4, width-len(gutter))
	display := sliceColumns(line.text, hScroll, textW)

	style := StyleNormal
	switch line.kind {
	case "add":
		style = StyleDiffAdd
	case "remove":
		style = StyleDiffRemove
	case "hunk":
		style = StyleDiffHunk
	case "meta":
		style = StyleDiffMeta
	}
	if current {
		style = StyleDiffMatch
	} else if matched {
		style = StyleWarning
	}
	// Inline styles avoid lipgloss Width wrapping that broke the columns.
	return StyleDiffNum.Render(gutter) + style.Render(display)
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
	title += StyleMuted.Render(" · " + truncate(viewBranch, maxInt(8, a.gitCommitColWidth()-12)))
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

	colW := maxInt(12, a.gitCommitColWidth()-4)
	var line string
	if colW < 55 {
		msgW := maxInt(8, colW-11-len(marker))
		line = fmt.Sprintf(" %-7s %s%s", truncate(c.Hash, 7), truncate(c.Message, msgW), marker)
	} else {
		msgW := colW - 29 - len(marker)
		line = fmt.Sprintf("  %-9s  %-*s  %s%s",
			c.Hash, msgW, truncate(c.Message, msgW), truncate(c.Author, 14), marker)
	}
	line = truncate(line, colW)

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

func (a *App) gitShowWorkingTree() bool {
	return false
}

func (a *App) gitBranchHistoryViewport() int {
	return maxInt(1, maxInt(12, a.height-2)-7)
}

func (a *App) gitCommitDiffViewport() int {
	height := maxInt(12, a.height-2)
	header := len(a.renderGitCommitHeaderLines(maxInt(20, a.width-4)))
	return maxInt(1, height-header-3-1) // footer + title
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
	a.gitCommitDetailFocus = gitCommitFocusDiff
	a.gitCommitFiles = nil
	a.gitCommitFilesLoading = false
	a.gitSelectedCommit = core.GitCommit{}
	a.gitBranchLoading = false
	a.gitCommitFullMsg = ""
	a.gitCommitDiff = ""
	a.gitCommitDiffLoading = false
	a.gitCommitDiffScroll = 0
	a.gitCommitDiffHScroll = 0
	a.gitCommitDiffCache = nil
	a.gitCommitDiffGen = 0
	a.gitCommitMsgExpanded = false
	a.gitDiffSearchOn = false
	a.gitDiffSearchInput = ""
	a.gitDiffSearchQuery = ""
	a.gitDiffSearchIdx = 0
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
	a.gitPromptCursor = 0
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
	if a.gitSubview != gitSubviewMain {
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
	if a.gitSubview != gitSubviewMain {
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

	if a.gitSubview == gitSubviewBranch {
		commits := a.gitDisplayedCommits()
		if len(commits) == 0 {
			return nil
		}
		viewport := a.gitBranchHistoryViewport()
		a.gitCommitCursor = clampCursor(a.gitCommitCursor+delta, len(commits))
		a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, viewport, len(commits))
		return nil
	}

	if a.gitSubview == gitSubviewCommit {
		if a.gitCommitDetailFocus == gitCommitFocusFiles {
			if len(a.gitCommitFiles) == 0 {
				return nil
			}
			prev := a.gitCommitFileCursor
			viewport := maxInt(1, a.gitCommitDiffViewport())
			a.gitCommitFileCursor = clampCursor(a.gitCommitFileCursor+delta, len(a.gitCommitFiles))
			a.gitCommitFileScroll = ensureVisible(a.gitCommitFileCursor, a.gitCommitFileScroll, viewport, len(a.gitCommitFiles))
			if a.gitCommitFileCursor != prev && a.selectedProject != nil {
				file := a.gitCommitFiles[a.gitCommitFileCursor].Path
				return a.requestGitCommitFileDiff(a.selectedProject.Path, a.gitSelectedCommit.Hash, file)
			}
			return nil
		}
		a.gitCommitDiffScrollBy(delta)
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

func (a *App) openGitBranchHistory(p *core.Project, branch string) tea.Cmd {
	a.gitSubview = gitSubviewBranch
	a.gitFocus = gitFocusCommits
	a.gitCommitCursor = 0
	a.gitCommitScroll = 0
	if branch == "" {
		return nil
	}
	if branch == a.gitViewBranch && (a.gitBranchLoading || len(a.gitBranchCommits) > 0) {
		return nil
	}
	a.gitViewBranch = branch
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
	a.gitCommitDetailFocus = gitCommitFocusDiff
	a.gitCommitDiff = ""
	a.gitCommitDiffLoading = false
	a.gitCommitDiffScroll = 0
	a.gitCommitDiffHScroll = 0
	a.gitCommitDiffCache = nil
	a.gitCommitDiffGen = 0
	a.gitCommitMsgExpanded = false
	a.gitDiffSearchOn = false
	a.gitDiffSearchInput = ""
	a.gitDiffSearchQuery = ""
	a.gitDiffSearchIdx = 0
	return loadGitCommitDetail(p.Path, commit.Hash)
}

func (a *App) switchGitCommitFile(delta int) tea.Cmd {
	if len(a.gitCommitFiles) == 0 || a.selectedProject == nil {
		return nil
	}
	a.gitCommitFileCursor = clampCursor(a.gitCommitFileCursor+delta, len(a.gitCommitFiles))
	a.gitCommitFileScroll = ensureVisible(a.gitCommitFileCursor, a.gitCommitFileScroll, a.gitCommitDiffViewport(), len(a.gitCommitFiles))
	a.gitDiffSearchQuery = ""
	a.gitDiffSearchIdx = 0
	a.gitCommitDiffHScroll = 0
	file := a.gitCommitFiles[a.gitCommitFileCursor].Path
	return a.requestGitCommitFileDiff(a.selectedProject.Path, a.gitSelectedCommit.Hash, file)
}

func (a *App) gitCommitDiffScrollBy(delta int) {
	lines := a.parseGitDiffLines()
	viewport := a.gitCommitDiffViewport()
	a.gitCommitDiffScroll = clampScroll(a.gitCommitDiffScroll+delta, viewport, len(lines))
}

func (a *App) toggleGitCommitDetailFocus() {
	if a.gitCommitDetailFocus == gitCommitFocusFiles {
		a.gitCommitDetailFocus = gitCommitFocusDiff
	} else {
		a.gitCommitDetailFocus = gitCommitFocusFiles
	}
}

func (a *App) gitDiffSearchMatches() []int {
	q := strings.ToLower(strings.TrimSpace(a.gitDiffSearchQuery))
	if q == "" {
		return nil
	}
	var matches []int
	for i, line := range a.parseGitDiffLines() {
		if strings.Contains(strings.ToLower(line.text), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (a *App) gitDiffMatchLineSet() map[int]bool {
	matches := a.gitDiffSearchMatches()
	if len(matches) == 0 {
		return nil
	}
	set := make(map[int]bool, len(matches))
	for _, i := range matches {
		set[i] = true
	}
	return set
}

func (a *App) jumpGitDiffSearch(delta int) {
	matches := a.gitDiffSearchMatches()
	if len(matches) == 0 {
		return
	}
	a.gitDiffSearchIdx = (a.gitDiffSearchIdx + delta) % len(matches)
	if a.gitDiffSearchIdx < 0 {
		a.gitDiffSearchIdx += len(matches)
	}
	a.gitCommitDetailFocus = gitCommitFocusDiff
	a.gitCommitDiffScroll = ensureVisible(matches[a.gitDiffSearchIdx], a.gitCommitDiffScroll, a.gitCommitDiffViewport(), len(a.parseGitDiffLines()))
}

func (a *App) applyGitDiffSearch() {
	a.gitDiffSearchQuery = strings.TrimSpace(a.gitDiffSearchInput)
	a.gitDiffSearchIdx = 0
	a.gitDiffSearchOn = false
	if a.gitDiffSearchQuery == "" {
		return
	}
	a.jumpGitDiffSearch(0)
}

func (a *App) updateGitDiffSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.gitDiffSearchOn = false
		a.gitDiffSearchInput = a.gitDiffSearchQuery
		return a, nil
	case tea.KeyEnter:
		a.applyGitDiffSearch()
		return a, nil
	case tea.KeyBackspace:
		if a.gitDiffSearchInput != "" {
			r := []rune(a.gitDiffSearchInput)
			a.gitDiffSearchInput = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		a.gitDiffSearchInput += string(msg.Runes)
	}
	return a, nil
}

func (a *App) renderGitDiffSearchPrompt() string {
	content := a.renderGitCommitDetail(a.currentProject())
	prompt := StylePanel.Render("Buscar no diff: " + a.gitDiffSearchInput + "█")
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		prompt,
		a.renderStatusBar("digite o termo | enter buscar | esc cancelar"),
	)
}

func (a *App) handleGitDedicatedKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if a.gitSubview == gitSubviewCommit {
			if a.gitDiffSearchQuery != "" {
				a.gitDiffSearchQuery = ""
				a.gitDiffSearchIdx = 0
				return a, nil
			}
			a.gitSubview = gitSubviewBranch
			a.gitCommitDiffCache = nil
			return a, nil
		}
		a.gitSubview = gitSubviewMain
		a.gitFocus = gitFocusBranches
		return a, nil
	case "enter":
		if a.gitSubview == gitSubviewBranch {
			commits := a.gitDisplayedCommits()
			if a.gitCommitCursor < len(commits) {
				return a, a.openGitCommitDetail(p, commits[a.gitCommitCursor])
			}
		}
	case "tab":
		if a.gitSubview == gitSubviewCommit {
			a.toggleGitCommitDetailFocus()
			return a, nil
		}
	case "m":
		if a.gitSubview == gitSubviewCommit {
			a.gitCommitMsgExpanded = !a.gitCommitMsgExpanded
			return a, nil
		}
	case "/":
		if a.gitSubview == gitSubviewCommit {
			a.gitDiffSearchOn = true
			a.gitDiffSearchInput = a.gitDiffSearchQuery
			return a, nil
		}
	case "n":
		if a.gitSubview == gitSubviewCommit {
			return a, a.switchGitCommitFile(1)
		}
	case "p":
		if a.gitSubview == gitSubviewCommit {
			return a, a.switchGitCommitFile(-1)
		}
	case "N":
		if a.gitSubview == gitSubviewCommit && a.gitDiffSearchQuery != "" {
			a.jumpGitDiffSearch(1)
			return a, nil
		}
	case "P":
		if a.gitSubview == gitSubviewCommit && a.gitDiffSearchQuery != "" {
			a.jumpGitDiffSearch(-1)
			return a, nil
		}
	case "left", "h":
		if a.gitSubview == gitSubviewCommit {
			if a.gitCommitDetailFocus == gitCommitFocusDiff {
				a.gitCommitDiffHScrollBy(-4)
				return a, nil
			}
			a.gitCommitDetailFocus = gitCommitFocusFiles
			return a, nil
		}
	case "right", "l":
		if a.gitSubview == gitSubviewCommit {
			if a.gitCommitDetailFocus == gitCommitFocusDiff {
				a.gitCommitDiffHScrollBy(4)
				return a, nil
			}
			a.gitCommitDetailFocus = gitCommitFocusDiff
			return a, nil
		}
	case "up", "k":
		return a, a.updateGitCursor(-1, p, false)
	case "down", "j":
		return a, a.updateGitCursor(1, p, false)
	case "pgup", "shift+up", "shift+k":
		if a.gitSubview == gitSubviewCommit {
			if a.gitCommitDetailFocus == gitCommitFocusFiles {
				return a, a.updateGitCursor(-a.gitCommitDiffViewport(), p, false)
			}
			a.gitCommitDiffScrollBy(-a.gitCommitDiffViewport())
		} else {
			return a, a.updateGitCursor(-a.gitBranchHistoryViewport(), p, false)
		}
	case "pgdown", "shift+down", "shift+j":
		if a.gitSubview == gitSubviewCommit {
			if a.gitCommitDetailFocus == gitCommitFocusFiles {
				return a, a.updateGitCursor(a.gitCommitDiffViewport(), p, false)
			}
			a.gitCommitDiffScrollBy(a.gitCommitDiffViewport())
		} else {
			return a, a.updateGitCursor(a.gitBranchHistoryViewport(), p, false)
		}
	case "home", "g":
		if a.gitSubview == gitSubviewCommit {
			a.gitCommitDiffScroll = 0
		} else {
			a.gitCommitCursor = 0
			a.gitCommitScroll = 0
		}
	case "end", "G":
		if a.gitSubview == gitSubviewCommit {
			a.gitCommitDiffScrollBy(len(a.parseGitDiffLines()))
		} else {
			commits := a.gitDisplayedCommits()
			if len(commits) > 0 {
				a.gitCommitCursor = len(commits) - 1
				a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, a.gitBranchHistoryViewport(), len(commits))
			}
		}
	}
	return a, nil
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
