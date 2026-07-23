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
	gitSubviewFileDiff
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
	if a.gitListViewportOverride > 0 {
		return a.gitListViewportOverride
	}
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
	case gitSubviewFileDiff:
		return a.renderGitFileDiff(p)
	}

	current := a.currentProject()
	if current == nil {
		current = p
	}
	g := current.Git
	w := maxInt(60, a.width)
	h := maxInt(18, a.projectPanelHeight())

	if a.projectGitLoading && (g == nil || !g.IsRepo || len(g.Branches) == 0) {
		return renderApiTitledBox("GIT", fitExactLines([]string{StyleMuted.Render("Carregando informações do Git...")}, h-2), w, h, true)
	}
	if g == nil || !g.IsRepo {
		return renderApiTitledBox("GIT", fitExactLines([]string{StyleMuted.Render("Este diretório não é um repositório git.")}, h-2), w, h, true)
	}
	viewBranch := a.gitViewBranch
	if viewBranch == "" {
		viewBranch = g.Branch
	}

	header := a.renderGitHeader(current, g, w)
	stats := a.renderGitStatsRow(g, w)
	notif := a.renderGitNotifLine()
	chromeH := lipgloss.Height(header) + lipgloss.Height(stats) + lipgloss.Height(notif) + 1
	bodyH := maxInt(12, h-chromeH-1)

	bottomH := maxInt(5, bodyH*22/100)
	midH := maxInt(5, (bodyH-bottomH)/2)
	topH := maxInt(5, bodyH-bottomH-midH)

	top := a.renderGitMainColumnsSized(g, viewBranch, w, topH)
	mid := a.renderGitWorkingRow(g, viewBranch, w, midH)
	bottom := a.renderGitBottomBoxes(g, w, bottomH)
	actions := StyleMuted.Render("c commit  a/A stage toggle  space checkout  enter detail/diff  x cherry  p/P pull/push  n/d/R/M branch")

	return lipgloss.JoinVertical(lipgloss.Left, header, stats, notif, top, mid, bottom, actions)
}

func (a *App) renderGitHeader(p *core.Project, g *core.GitInfo, width int) string {
	path := shortenPath(p.Path)
	clean := StyleHealthy.Render("✓ clean")
	if g.Modified > 0 || g.Staged > 0 || g.Untracked > 0 {
		clean = StyleWarning.Render("● dirty")
	}
	remote := compactGitRemote(g.Remote)
	left := StyleSection.Render("GIT") + StyleMuted.Render("  "+path) + "  " + clean
	if g.StashCount > 0 {
		left += StyleMuted.Render(fmt.Sprintf("  stash:%d", g.StashCount))
	}
	if remote != "" {
		left += StyleMuted.Render("  ↗ "+truncate(remote, 36))
	}
	right := StyleMuted.Render("HEAD ") + StyleWarning.Render(g.Branch)
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func compactGitRemote(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "git@")
	u = strings.TrimSuffix(u, ".git")
	u = strings.ReplaceAll(u, ":", "/")
	return u
}

func (a *App) renderGitStatsRow(g *core.GitInfo, width int) string {
	boxW := maxInt(10, width/6)
	cards := []struct{ title, value string }{
		{"BRANCH", g.Branch},
		{"AHEAD/BEHIND", fmt.Sprintf("↑ %d / ↓ %d", g.Ahead, g.Behind)},
		{"MODIFIED", fmt.Sprintf("%d", g.Modified)},
		{"STAGED", fmt.Sprintf("%d", g.Staged)},
		{"UNTRACKED", fmt.Sprintf("%d", g.Untracked)},
		{"STASHES", fmt.Sprintf("%d", g.StashCount)},
	}
	var parts []string
	for _, c := range cards {
		val := StyleNormal.Render(truncate(c.value, boxW-4))
		if c.title == "BRANCH" {
			val = StyleWarning.Render(truncate(c.value, boxW-4))
		}
		if c.title == "AHEAD/BEHIND" && (g.Ahead > 0 || g.Behind > 0) {
			val = StyleAccent.Render(truncate(c.value, boxW-4))
		}
		body := []string{val}
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines(body, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderGitNotifLine() string {
	switch {
	case a.gitStatusMsg != "":
		style := StyleMuted
		if strings.Contains(a.gitStatusMsg, "✓") {
			style = StyleHealthy
		} else if strings.Contains(a.gitStatusMsg, "erro") || strings.Contains(a.gitStatusMsg, ":") {
			style = StyleWarning
		}
		return style.Render(truncate(a.gitStatusMsg, maxInt(40, a.width-4)))
	case a.gitCherryPickActive:
		src := a.gitCherryPickSourceBranch
		if src == "" {
			src = "?"
		}
		return StyleGitCherry.Render("🍒 " + a.gitCherryPickSummary() + " de " + src + " — shift+v cola")
	case a.gitSelectedCommitCount() > 0:
		return StyleGitSelected.Render(fmt.Sprintf("✓ %d selected — shift+c copia", a.gitSelectedCommitCount()))
	case a.gitActionLoading:
		return StyleMuted.Render("executando...")
	default:
		return StyleMuted.Render(" ")
	}
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
	return a.renderGitMainColumnsSized(g, viewBranch, maxInt(60, a.width), a.gitPanelInnerLines()+2)
}

func (a *App) renderGitMainColumnsSized(g *core.GitInfo, viewBranch string, width, height int) string {
	branchW := a.gitBranchColWidth()
	if branchW > width/2 {
		branchW = maxInt(14, width/3)
	}
	commitW := maxInt(24, width-branchW)
	// Temporarily size list viewport from height for this render.
	prevH := a.height
	// viewport ≈ height - title/scroll chrome (3)
	a.gitListViewportOverride = maxInt(3, height-3)
	branchLines := strings.Split(a.renderGitBranches(g, viewBranch), "\n")
	commitLines := strings.Split(a.renderGitCommits(viewBranch), "\n")
	a.gitListViewportOverride = 0
	_ = prevH
	bfocus := a.gitFocus == gitFocusBranches
	cfocus := a.gitFocus == gitFocusCommits
	branchBody := branchLines
	if len(branchBody) > 0 {
		branchBody = branchBody[1:] // drop internal section title; box has its own
	}
	commitBody := commitLines
	if len(commitBody) > 0 {
		commitBody = commitBody[1:]
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderApiTitledBox("BRANCHES", fitExactLines(branchBody, height-2), branchW, height, bfocus),
		renderApiTitledBox("COMMITS · "+truncate(viewBranch, 16), fitExactLines(commitBody, height-2), commitW, height, cfocus),
	)
}

func (a *App) renderGitWorkingRow(g *core.GitInfo, viewBranch string, width, height int) string {
	filesFocus := a.gitFocus == gitFocusFiles
	fileLines := a.gitFileLines(g, viewBranch, height-2)
	filesTitle := "MODIFIED FILES"
	if g.Staged > 0 {
		filesTitle += fmt.Sprintf(" · %d staged", g.Staged)
	}
	if filesFocus {
		filesTitle = "> " + filesTitle
	}
	return renderApiTitledBox(filesTitle, fitExactLines(fileLines, height-2), width, height, filesFocus)
}

func (a *App) gitFileLines(g *core.GitInfo, viewBranch string, maxLines int) []string {
	if viewBranch != g.Branch {
		return []string{StyleMuted.Render("checkout da branch para ver WT")}
	}
	if len(g.Files) == 0 {
		return []string{StyleHealthy.Render("✓ working tree limpo")}
	}
	viewport := maxInt(1, maxLines)
	a.gitFileScroll = ensureVisible(a.gitFileCursor, a.gitFileScroll, viewport, len(g.Files))
	start := a.gitFileScroll
	end := minInt(start+viewport, len(g.Files))
	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		f := g.Files[i]
		code := gitStatusLabel(f.Staging, f.Worktree)
		mark := "  "
		style := StyleMuted
		if a.gitFocus == gitFocusFiles && a.gitFileCursor == i {
			mark = "▸ "
			style = StyleSelected
		}
		line := style.Render(mark + gitStatusStyle(code) + " " + f.Path)
		if gitFileStaged(f) {
			line += "  " + StyleHealthy.Render("● staged")
		}
		lines = append(lines, line)
	}
	return lines
}

func (a *App) renderGitBottomBoxes(g *core.GitInfo, width, height int) string {
	w1 := width / 3
	w2 := width / 3
	w3 := width - w1 - w2

	act := make([]string, 0, height-2)
	if len(a.gitActivity) == 0 {
		if g.LastCommit != "" {
			act = append(act, StyleMuted.Render(timeNowHHMM())+" "+StyleNormal.Render("Commit "+g.LastCommit))
		}
		act = append(act, StyleMuted.Render("(ações recentes aparecem aqui)"))
	} else {
		for i, e := range a.gitActivity {
			if i >= height-2 {
				break
			}
			act = append(act, StyleNormal.Render(truncate(e, w1-4)))
		}
	}

	stashes := make([]string, 0, height-2)
	if len(g.Stashes) == 0 {
		stashes = append(stashes, StyleMuted.Render("(nenhum stash)"))
	} else {
		for i, s := range g.Stashes {
			if i >= height-2 {
				break
			}
			stashes = append(stashes, StyleMuted.Render(s.Ref)+" "+StyleNormal.Render(truncate(s.Message, w2-14)))
		}
	}

	remotes := make([]string, 0, height-2)
	if len(g.Remotes) == 0 {
		remotes = append(remotes, StyleMuted.Render("(sem remotes)"))
	} else {
		for _, r := range g.Remotes {
			remotes = append(remotes, StyleWarning.Render(r.Name))
			remotes = append(remotes, StyleMuted.Render("  "+truncate(compactGitRemote(r.URL), w3-6)))
			if r.Name == "origin" || r.Name == g.Remotes[0].Name {
				sync := StyleHealthy.Render("✓ up to date")
				if g.Ahead > 0 || g.Behind > 0 {
					sync = StyleAccent.Render(fmt.Sprintf("↑ %d ↓ %d", g.Ahead, g.Behind))
				}
				remotes = append(remotes, StyleMuted.Render("  "+g.Branch+"  ")+sync)
			}
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderApiTitledBox("RECENT ACTIVITY", fitExactLines(act, height-2), w1, height, false),
		renderApiTitledBox("STASHES", fitExactLines(stashes, height-2), w2, height, false),
		renderApiTitledBox("REMOTES", fitExactLines(remotes, height-2), w3, height, false),
	)
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
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
	branch := a.gitViewBranch
	if branch == "" && p != nil && p.Git != nil {
		branch = p.Git.Branch
	}
	commits := a.gitDisplayedCommits()
	if !a.gitBranchLoading && len(commits) > 0 {
		a.gitCommitCursor = clampCursor(a.gitCommitCursor, len(commits))
	}

	header := a.renderGitBranchHistoryHeader(p, branch, w)
	cards := a.renderGitBranchHistoryCards(p, branch, commits, w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(cards) + 2
	bodyH := maxInt(8, h-chromeH-2)

	rightW := maxInt(24, w*28/100)
	if rightW > 38 {
		rightW = 38
	}
	leftW := maxInt(36, w-rightW-1)
	list := a.renderGitBranchHistoryTable(commits, leftW, bodyH)
	detail := a.renderGitBranchHistoryInspector(p, branch, commits, rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)

	pos := ""
	if n := len(commits); n > 0 {
		pos = fmt.Sprintf("%d/%d", a.gitCommitCursor+1, n)
	}
	footer := StyleMuted.Render(truncate(
		"↑↓ navegar  enter abrir commit  pgup/pgdn  "+pos+"  esc voltar",
		maxInt(10, w-2),
	))
	return lipgloss.JoinVertical(lipgloss.Left,
		header, cards, body, footer,
		a.renderStatusBar("histórico · "+truncate(branch, 24)),
	)
}

func (a *App) renderGitBranchHistoryHeader(p *core.Project, branch string, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabGit)).Bold(true)
	left := accent.Render("devscope") + StyleMuted.Render(" › git › branch") +
		StyleMuted.Render("  ") + StyleWarning.Render(truncate(branch, 28))
	badge := StyleMuted.Render("○")
	if p != nil && p.Git != nil && p.Git.Branch == branch {
		badge = StyleHealthy.Render("● HEAD")
	}
	n := len(a.gitDisplayedCommits())
	right := badge + StyleMuted.Render(fmt.Sprintf("  commits:%d", n))
	if a.gitBranchLoading {
		right += StyleMuted.Render("  · atualizando…")
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderGitBranchHistoryCards(p *core.Project, branch string, commits []core.GitCommit, width int) string {
	authors := gitUniqueAuthors(commits)
	tip := "—"
	if len(commits) > 0 {
		tip = commits[0].Date
		if tip == "" {
			tip = "—"
		}
	}
	head := "outra"
	aheadBehind := "—"
	if p != nil && p.Git != nil {
		if p.Git.Branch == branch {
			head = "sim"
			aheadBehind = fmt.Sprintf("↑%d ↓%d", p.Git.Ahead, p.Git.Behind)
		}
	}
	boxW := maxInt(12, width/5)
	cards := []struct{ title, value string }{
		{"BRANCH", branch},
		{"COMMITS", fmt.Sprintf("%d", len(commits))},
		{"AUTHORS", fmt.Sprintf("%d", len(authors))},
		{"TIP", tip},
		{"HEAD", head + "  " + aheadBehind},
	}
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		val := StyleNormal.Render(truncate(c.value, boxW-4))
		switch c.title {
		case "BRANCH":
			val = StyleWarning.Render(truncate(c.value, boxW-4))
		case "HEAD":
			if head == "sim" {
				val = StyleHealthy.Render(truncate(c.value, boxW-4))
			}
		}
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines([]string{val}, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderGitBranchHistoryTable(commits []core.GitCommit, width, height int) string {
	viewport := maxInt(1, height-2)
	lines := make([]string, 0, viewport)
	if len(commits) == 0 {
		if a.gitBranchLoading {
			lines = append(lines, StyleMuted.Render("  Carregando commits…"))
		} else {
			lines = append(lines, StyleMuted.Render("  (sem commits nesta branch)"))
		}
	} else {
		a.gitCommitScroll = ensureVisible(a.gitCommitCursor, a.gitCommitScroll, viewport-1, len(commits))
		header := StyleMuted.Render(truncate(
			fmt.Sprintf("  %-8s %-12s %-14s %s", "HASH", "WHEN", "AUTHOR", "MESSAGE"),
			width-2,
		))
		lines = append(lines, header)
		start := a.gitCommitScroll
		end := minInt(start+viewport-1, len(commits))
		for i := start; i < end; i++ {
			lines = append(lines, a.renderGitBranchCommitLine(commits[i], i, width-2))
		}
	}
	return renderApiTitledBox("COMMITS", fitExactLines(lines, viewport), width, height, true)
}

func (a *App) renderGitBranchCommitLine(c core.GitCommit, idx, width int) string {
	marker := "  "
	if a.gitCommitCursor == idx {
		marker = "▶ "
	}
	msgW := maxInt(8, width-40)
	when := c.Date
	if when == "" {
		when = "—"
	}
	line := fmt.Sprintf("%s%-8s %-12s %-14s %s",
		marker,
		truncate(c.Hash, 8),
		truncate(when, 12),
		truncate(c.Author, 14),
		truncate(c.Message, msgW),
	)
	line = truncate(line, width)
	if a.gitCommitCursor == idx {
		return StyleSelected.Render(line)
	}
	hash := StyleAccent.Render(truncate(c.Hash, 8))
	rest := StyleMuted.Render(fmt.Sprintf(" %-12s %-14s ", truncate(when, 12), truncate(c.Author, 14))) +
		StyleNormal.Render(truncate(c.Message, msgW))
	return marker + hash + rest
}

func (a *App) renderGitBranchHistoryInspector(p *core.Project, branch string, commits []core.GitCommit, width, height int) string {
	detH := maxInt(8, height*55/100)
	actH := maxInt(5, height*22/100)
	authH := maxInt(4, height-detH-actH)

	details := []string{StyleMuted.Render("(nenhum commit)")}
	if len(commits) == 0 && a.gitBranchLoading {
		details = []string{StyleMuted.Render("carregando…")}
	} else if len(commits) > 0 && a.gitCommitCursor < len(commits) {
		c := commits[a.gitCommitCursor]
		details = []string{
			StyleMuted.Render("Hash    ") + StyleAccent.Render(c.Hash),
			StyleMuted.Render("Author  ") + StyleNormal.Render(truncate(c.Author, width-12)),
			StyleMuted.Render("When    ") + StyleMuted.Render(firstNonEmpty(c.Date, "—")),
			StyleMuted.Render("Branch  ") + StyleWarning.Render(truncate(branch, width-12)),
			"",
			StyleMuted.Render("Message"),
		}
		for _, part := range wrapGitMessage(c.Message, width-4) {
			details = append(details, StyleNormal.Render(part))
		}
		if p != nil && p.Git != nil && p.Git.Branch == branch && a.gitCommitCursor == 0 {
			details = append(details, "", StyleHealthy.Render("● tip da HEAD"))
		}
	}

	actions := moduleActionLines(
		[2]string{"enter", "abrir commit"},
		[2]string{"esc", "voltar"},
		[2]string{"↑↓", "navegar"},
		[2]string{"pg", "página"},
	)

	authors := gitAuthorCounts(commits)
	authLines := []string{StyleMuted.Render("(sem autores)")}
	if len(authors) > 0 {
		authLines = authLines[:0]
		maxN := authors[0].n
		show := minInt(6, len(authors))
		barW := maxInt(6, width-16)
		for i := 0; i < show; i++ {
			au := authors[i]
			pct := 100.0 * float64(au.n) / float64(maxN)
			authLines = append(authLines,
				StyleMuted.Render(fmt.Sprintf("%-10s ", truncate(au.name, 10)))+
					meterBar(pct, barW)+
					StyleMuted.Render(fmt.Sprintf(" %d", au.n)),
			)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DETALHES", fitExactLines(details, detH-2), width, detH, false),
		renderApiTitledBox("AÇÕES", fitExactLines(actions, actH-2), width, actH, false),
		renderApiTitledBox("AUTHORS", fitExactLines(authLines, authH-2), width, authH, false),
	)
}

func wrapGitMessage(msg string, width int) []string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return []string{StyleMuted.Render("(sem mensagem)")}
	}
	if width < 8 {
		width = 8
	}
	var out []string
	for len(msg) > 0 {
		if len(msg) <= width {
			out = append(out, msg)
			break
		}
		out = append(out, msg[:width])
		msg = msg[width:]
		if len(out) >= 6 {
			if msg != "" {
				out[len(out)-1] = truncate(out[len(out)-1]+"…", width)
			}
			break
		}
	}
	return out
}

type gitAuthorCount struct {
	name string
	n    int
}

func gitUniqueAuthors(commits []core.GitCommit) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range commits {
		if c.Author == "" || seen[c.Author] {
			continue
		}
		seen[c.Author] = true
		out = append(out, c.Author)
	}
	return out
}

func gitAuthorCounts(commits []core.GitCommit) []gitAuthorCount {
	m := map[string]int{}
	for _, c := range commits {
		if c.Author == "" {
			continue
		}
		m[c.Author]++
	}
	out := make([]gitAuthorCount, 0, len(m))
	for name, n := range m {
		out = append(out, gitAuthorCount{name: name, n: n})
	}
	// simple desc sort
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].n > out[i].n {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
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
	return parseDiffContent(a.gitCommitDiff)
}

func (a *App) parseWTDiffLines() []gitDiffLine {
	return parseDiffContent(a.gitWTDiff)
}

func parseDiffContent(content string) []gitDiffLine {
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
	// Mid row ≈ half of the main body after branches/commits.
	return maxInt(3, a.gitListViewport()/2)
}

func (a *App) gitShowWorkingTree() bool {
	return true
}

func (a *App) gitBranchHistoryViewport() int {
	h := maxInt(18, a.height-2)
	// header(~1) + cards(~3) + footer/status(~3) + box chrome(~2)
	return maxInt(5, h-10)
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
	a.gitWTDiffScroll = 0
	a.gitWTDiffHScroll = 0
	a.gitWTDiff = ""
	a.gitWTDiffFile = ""
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

func (a *App) gitFocusNext() tea.Cmd {
	if a.gitSubview != gitSubviewMain {
		return nil
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
	return nil
}

func (a *App) gitFocusPrev() tea.Cmd {
	if a.gitSubview != gitSubviewMain {
		return nil
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
	return nil
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

func (a *App) openGitFileDiff(p *core.Project) tea.Cmd {
	if p == nil || p.Git == nil {
		return nil
	}
	files := p.Git.Files
	if a.gitViewBranch != "" && a.gitViewBranch != p.Git.Branch {
		a.gitStatusMsg = "checkout da branch para ver o diff"
		return nil
	}
	if len(files) == 0 || a.gitFileCursor >= len(files) {
		a.gitStatusMsg = "nenhum arquivo modificado"
		return nil
	}
	f := files[a.gitFileCursor]
	a.gitSubview = gitSubviewFileDiff
	a.gitFocus = gitFocusFiles
	a.gitWTDiffScroll = 0
	a.gitWTDiffHScroll = 0
	if a.gitWTDiffFile == f.Path && strings.TrimSpace(a.gitWTDiff) != "" {
		return nil
	}
	return a.requestGitWorkingTreeDiff(p.Path, f.Path)
}

func (a *App) renderGitFileDiff(p *core.Project) string {
	height := maxInt(12, a.height-2)
	panelW := maxInt(20, a.width)
	innerW := maxInt(16, panelW-4)
	file := a.gitWTDiffFile
	if file == "" {
		if p != nil && p.Git != nil && a.gitFileCursor < len(p.Git.Files) {
			file = p.Git.Files[a.gitFileCursor].Path
		}
	}
	code := ""
	if p != nil && p.Git != nil {
		for _, f := range p.Git.Files {
			if f.Path == file {
				code = gitStatusLabel(f.Staging, f.Worktree)
				break
			}
		}
	}
	status := ""
	if code != "" {
		status = gitStatusStyle(code) + " "
	}
	title := StyleSection.Render("DIFF") + "  " + status + StyleNormal.Render(truncate(file, maxInt(20, innerW-12)))
	footerH := 1
	bodyH := maxInt(4, height-3-footerH) // title + blank + footer
	textW := maxInt(8, innerW-11)
	all := a.parseWTDiffLines()
	maxLine := 0
	for _, line := range all {
		if w := lipgloss.Width(line.text); w > maxLine {
			maxLine = w
		}
	}
	a.gitWTDiffHScroll = clampScroll(a.gitWTDiffHScroll, textW, maxLine)
	a.gitWTDiffScroll = clampScroll(a.gitWTDiffScroll, bodyH, len(all))
	start := a.gitWTDiffScroll
	end := minInt(start+bodyH, len(all))
	lines := make([]string, 0, bodyH+2)
	lines = append(lines, title, "")
	if strings.TrimSpace(a.gitWTDiff) == "" {
		lines = append(lines, StyleMuted.Render("Carregando diff..."))
	} else {
		for i := start; i < end; i++ {
			lines = append(lines, renderGitDiffLine(all[i], innerW, a.gitWTDiffHScroll, false, false))
		}
	}
	for len(lines) < bodyH+2 {
		lines = append(lines, "")
	}
	pos := "0/0"
	if len(all) > 0 {
		pos = fmt.Sprintf("%d-%d/%d", start+1, end, len(all))
	}
	hHint := ""
	if maxH := maxInt(0, maxLine-textW); maxH > 0 || a.gitWTDiffHScroll > 0 {
		hHint = fmt.Sprintf("  ↔ col %d", a.gitWTDiffHScroll)
	}
	footer := StyleMuted.Render(truncate("↑↓ scroll  ←→ lateral  pgup/pgdown  esc voltar  "+pos+hHint, innerW))
	content := append(lines[:bodyH+2], footer)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Width(panelW).
		Render(strings.Join(content, "\n"))
}

func (a *App) gitWTDiffScrollBy(delta int) {
	all := a.parseWTDiffLines()
	viewport := maxInt(4, a.height-6)
	a.gitWTDiffScroll = clampScroll(a.gitWTDiffScroll+delta, viewport, len(all))
}

func (a *App) gitWTDiffHScrollBy(delta int) {
	textW := maxInt(8, a.width-4-11)
	maxLine := 0
	for _, line := range a.parseWTDiffLines() {
		if w := lipgloss.Width(line.text); w > maxLine {
			maxLine = w
		}
	}
	maxH := maxInt(0, maxLine-textW)
	a.gitWTDiffHScroll += delta
	if a.gitWTDiffHScroll < 0 {
		a.gitWTDiffHScroll = 0
	}
	if a.gitWTDiffHScroll > maxH {
		a.gitWTDiffHScroll = maxH
	}
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
	if a.gitSubview == gitSubviewFileDiff {
		switch msg.String() {
		case "esc":
			a.gitSubview = gitSubviewMain
			a.gitFocus = gitFocusFiles
			return a, nil
		case "up", "k":
			a.gitWTDiffScrollBy(-1)
			return a, nil
		case "down", "j":
			a.gitWTDiffScrollBy(1)
			return a, nil
		case "left", "h":
			a.gitWTDiffHScrollBy(-4)
			return a, nil
		case "right", "l":
			a.gitWTDiffHScrollBy(4)
			return a, nil
		case "pgup", "shift+up", "shift+k":
			a.gitWTDiffScrollBy(-maxInt(4, a.height-8))
			return a, nil
		case "pgdown", "shift+down", "shift+j":
			a.gitWTDiffScrollBy(maxInt(4, a.height-8))
			return a, nil
		case "home":
			a.gitWTDiffScroll = 0
			return a, nil
		case "end":
			a.gitWTDiffScrollBy(len(a.parseWTDiffLines()))
			return a, nil
		}
		return a, nil
	}

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
	if p == nil || p.Git == nil || !p.Git.IsRepo {
		return nil
	}
	if a.gitActionLoading {
		return nil
	}
	// space = checkout (toggle de commit fica no `x`)
	branch := ""
	if a.gitFocus == gitFocusBranches {
		if name, ok := a.selectedGitBranch(p); ok {
			branch = name
		}
	}
	if branch == "" {
		branch = a.gitViewBranch
	}
	if branch == "" {
		return nil
	}
	if branch == p.Git.Branch {
		a.gitFocus = gitFocusBranches
		return a.selectGitBranch(p, branch)
	}
	a.gitFocus = gitFocusBranches
	return a.gitCheckoutBranch(p, branch)
}
