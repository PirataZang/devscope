package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// cellGlyph is the keifu-style rounded-curve mapping — ╭╮╰╯ for branch/merge
// points instead of the diagonal /\ git's own --graph output uses.
func cellGlyph(k cellType, isHead bool) rune {
	switch k {
	case cellPipe:
		return '│'
	case cellCommit:
		if isHead {
			return '◉'
		}
		return '●'
	case cellBranchRight:
		return '╭'
	case cellBranchLeft:
		return '╮'
	case cellMergeRight:
		return '╰'
	case cellMergeLeft:
		return '╯'
	case cellHorizontal:
		return '─'
	case cellHorizontalPipe:
		return '┼'
	case cellTeeRight:
		return '├'
	case cellTeeLeft:
		return '┤'
	case cellTeeUp:
		return '┴'
	default:
		return ' '
	}
}

func renderGraphCells(cells []graphCell, isHead bool) string {
	var b strings.Builder
	for _, c := range cells {
		if c.kind == cellEmpty {
			b.WriteByte(' ')
			continue
		}
		color := graphLaneGlyphColor(c.color)
		if c.kind == cellHorizontalPipe {
			color = graphLaneGlyphColor(c.pipeColor)
		}
		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		b.WriteString(style.Render(string(cellGlyph(c.kind, isHead))))
	}
	return b.String()
}

// graphCommitIcon picks a small marker from common conventional-commit
// keywords — purely cosmetic, falls back to a plain bullet.
// graphCommitIcon: só glifos de 1 coluna. Emoji mede 2 colunas nas libs e 1 na
// maioria dos terminais, e desalinhava a grade inteira do grafo.
func graphCommitIcon(subject string) string {
	s := strings.ToLower(subject)
	switch {
	case strings.HasPrefix(s, "merge"):
		return StyleAccent.Render("⑂")
	case strings.Contains(s, "security"):
		return StyleUnhealthy.Render("⚿")
	case strings.HasPrefix(s, "fix") || strings.Contains(s, "bug"):
		return StyleWarning.Render("⚠")
	case strings.HasPrefix(s, "feat") || strings.HasPrefix(s, "add"):
		return StyleHealthy.Render("✚")
	case strings.HasPrefix(s, "docs") || strings.HasPrefix(s, "doc"):
		return StyleMuted.Render("≡")
	case strings.HasPrefix(s, "chore") || strings.HasPrefix(s, "refactor"):
		return StyleMuted.Render("⚙")
	case strings.HasPrefix(s, "test"):
		return StyleMuted.Render("✓")
	default:
		return StyleMuted.Render("•")
	}
}

func renderGitGraphRefBadges(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	first := refs[0]
	return lipgloss.NewStyle().Foreground(ColorPink).Bold(true).Render("[" + first + "]")
}

type gitGraphDetailMsg struct {
	gen   int
	hash  string
	msg   string
	files []collectors.GitCommitFileStat
}

// gitGraphFocus is which of the three graph panes takes the arrow keys.
type gitGraphFocus int

const (
	gitGraphFocusCommits gitGraphFocus = iota
	gitGraphFocusDetail
	gitGraphFocusFiles
)

const gitGraphPaneCount = 3

func (a *App) openGitGraph(p *core.Project) tea.Cmd {
	a.gitSubview = gitSubviewGraph
	a.gitGraphFocus = gitGraphFocusCommits
	a.gitGraphBranchRef = ""
	a.gitGraphBranchPicker = false
	return a.reloadGitGraph(p)
}

// reloadGitGraph re-walks the DAG honouring the active branch filter, so both
// the initial open and `r` share one path.
func (a *App) reloadGitGraph(p *core.Project) tea.Cmd {
	a.gitGraphCursor = 0
	a.gitGraphScroll = 0
	var refs []string
	if a.gitGraphBranchRef != "" {
		refs = []string{a.gitGraphBranchRef}
	}
	commits := collectors.GitLogDAG(p.Path, 300, refs...)
	a.gitGraphLayout = buildGraphLayout(commits)
	if i := a.nextGitGraphCommitRow(-1); i >= 0 {
		a.gitGraphCursor = i
	}
	a.gitGraphDetailHash = ""
	a.gitGraphDetailMsg = ""
	a.gitGraphDetailFiles = nil
	return a.requestGitGraphDetail(p)
}

func (a *App) gitGraphNodes() []graphNode {
	return a.gitGraphLayout.nodes
}

func (a *App) selectedGitGraphNode() (graphNode, bool) {
	nodes := a.gitGraphNodes()
	if a.gitGraphCursor < 0 || a.gitGraphCursor >= len(nodes) {
		return graphNode{}, false
	}
	n := nodes[a.gitGraphCursor]
	if n.commit == nil {
		return graphNode{}, false
	}
	return n, true
}

func (a *App) nextGitGraphCommitRow(from int) int {
	nodes := a.gitGraphNodes()
	for i := from + 1; i < len(nodes); i++ {
		if nodes[i].commit != nil {
			return i
		}
	}
	return -1
}

func (a *App) prevGitGraphCommitRow(from int) int {
	nodes := a.gitGraphNodes()
	for i := from - 1; i >= 0; i-- {
		if nodes[i].commit != nil {
			return i
		}
	}
	return -1
}

func (a *App) requestGitGraphDetail(p *core.Project) tea.Cmd {
	a.gitGraphDetailScroll = 0
	a.gitGraphDetailHScroll = 0
	a.gitGraphFilesScroll = 0
	a.gitGraphFilesHScroll = 0
	node, ok := a.selectedGitGraphNode()
	if !ok {
		a.gitGraphDetailHash = ""
		return nil
	}
	a.gitGraphDetailGen++
	gen := a.gitGraphDetailGen
	hash := node.commit.Hash
	path := p.Path
	return func() tea.Msg {
		msg := collectors.CollectCommitFullMessage(path, hash)
		files := collectors.CollectCommitFileStats(path, hash)
		return gitGraphDetailMsg{gen: gen, hash: hash, msg: msg, files: files}
	}
}

func (a *App) handleGitGraphDetail(msg gitGraphDetailMsg) {
	if msg.gen != a.gitGraphDetailGen {
		return
	}
	a.gitGraphDetailHash = msg.hash
	a.gitGraphDetailMsg = msg.msg
	a.gitGraphDetailFiles = msg.files
}

// gitGraphScrollBy moves the focused pane's vertical offset; the render pass
// clamps the upper bound once it knows the pane's height.
func (a *App) gitGraphScrollBy(delta int) {
	switch a.gitGraphFocus {
	case gitGraphFocusDetail:
		a.gitGraphDetailScroll = maxInt(0, a.gitGraphDetailScroll+delta)
	case gitGraphFocusFiles:
		a.gitGraphFilesScroll = maxInt(0, a.gitGraphFilesScroll+delta)
	}
}

func (a *App) gitGraphHScrollBy(delta int) {
	switch a.gitGraphFocus {
	case gitGraphFocusDetail:
		a.gitGraphDetailHScroll = maxInt(0, a.gitGraphDetailHScroll+delta)
	case gitGraphFocusFiles:
		a.gitGraphFilesHScroll = maxInt(0, a.gitGraphFilesHScroll+delta)
	}
}

func (a *App) handleGitGraphKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.gitGraphBranchPicker {
		return a.handleGitGraphBranchPickerKeys(msg, p)
	}
	switch msg.String() {
	case "esc", "q":
		a.gitSubview = gitSubviewMain
		return a, nil
	case "tab":
		a.gitGraphFocus = (a.gitGraphFocus + 1) % gitGraphPaneCount
	case "shift+tab":
		a.gitGraphFocus = (a.gitGraphFocus + gitGraphPaneCount - 1) % gitGraphPaneCount
	case "B":
		a.openGitGraphBranchPicker()
	case "up", "k":
		if a.gitGraphFocus != gitGraphFocusCommits {
			a.gitGraphScrollBy(-1)
			return a, nil
		}
		if i := a.prevGitGraphCommitRow(a.gitGraphCursor); i >= 0 {
			a.gitGraphCursor = i
			return a, a.requestGitGraphDetail(p)
		}
	case "down", "j":
		if a.gitGraphFocus != gitGraphFocusCommits {
			a.gitGraphScrollBy(1)
			return a, nil
		}
		if i := a.nextGitGraphCommitRow(a.gitGraphCursor); i >= 0 {
			a.gitGraphCursor = i
			return a, a.requestGitGraphDetail(p)
		}
	case "left", "h":
		a.gitGraphHScrollBy(-4)
	case "right", "l":
		a.gitGraphHScrollBy(4)
	case "shift+left", "H":
		a.gitGraphHScrollBy(-40)
	case "shift+right", "L":
		a.gitGraphHScrollBy(40)
	case "home":
		a.gitGraphHScrollBy(-1 << 20)
	case "pgup":
		if a.gitGraphFocus != gitGraphFocusCommits {
			a.gitGraphScrollBy(-a.gitGraphPaneViewport())
			return a, nil
		}
		a.gitGraphScroll = maxInt(0, a.gitGraphScroll-a.gitGraphViewport())
	case "pgdown":
		if a.gitGraphFocus != gitGraphFocusCommits {
			a.gitGraphScrollBy(a.gitGraphPaneViewport())
			return a, nil
		}
		a.gitGraphScroll = minInt(maxInt(0, len(a.gitGraphNodes())-1), a.gitGraphScroll+a.gitGraphViewport())
	case "r":
		return a, a.reloadGitGraph(p)
	case "enter":
		if node, ok := a.selectedGitGraphNode(); ok {
			return a, a.openGitCommitDetail(p, core.GitCommit{
				Hash:    node.commit.Hash,
				Message: node.commit.Subject,
				Author:  node.commit.Author,
				Date:    node.commit.Date,
			})
		}
	}
	return a, nil
}

// --- branch filter -------------------------------------------------------

const gitGraphAllBranches = "— todas as branches —"

// gitGraphBranchNames unions the cached branch list with any ref the graph
// itself draws, so a branch missing from the cache is still selectable.
func (a *App) gitGraphBranchNames() []string {
	seen := map[string]bool{}
	var names []string
	add := func(n string) {
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	for _, b := range a.gitBranchesForUI() {
		add(b.Name)
	}
	for _, n := range a.gitGraphNodes() {
		if n.commit == nil {
			continue
		}
		for _, r := range n.commit.Refs {
			if strings.HasPrefix(r, "tag: ") {
				continue
			}
			add(r)
		}
	}
	sort.Strings(names)
	return names
}

func (a *App) openGitGraphBranchPicker() {
	a.gitGraphBranchOpts = append([]string{gitGraphAllBranches}, a.gitGraphBranchNames()...)
	a.gitGraphBranchCursor = 0
	for i, name := range a.gitGraphBranchOpts {
		if i > 0 && name == a.gitGraphBranchRef {
			a.gitGraphBranchCursor = i
			break
		}
	}
	a.gitGraphBranchScroll = 0
	a.gitGraphBranchFilter = ""
	a.gitGraphBranchPicker = true
}

// gitGraphBranchList is what the picker shows: the "all branches" entry plus
// the names matching the typed filter, same substring match the git screen's
// `b` filter uses.
func (a *App) gitGraphBranchList() []string {
	if len(a.gitGraphBranchOpts) == 0 {
		return nil
	}
	if a.gitGraphBranchFilter == "" {
		return a.gitGraphBranchOpts
	}
	f := strings.ToLower(a.gitGraphBranchFilter)
	out := []string{a.gitGraphBranchOpts[0]}
	for _, n := range a.gitGraphBranchOpts[1:] {
		if strings.Contains(strings.ToLower(n), f) {
			out = append(out, n)
		}
	}
	return out
}

func (a *App) handleGitGraphBranchPickerKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	opts := a.gitGraphBranchList()
	switch msg.String() {
	case "esc":
		a.gitGraphBranchPicker = false
	case "up":
		if a.gitGraphBranchCursor > 0 {
			a.gitGraphBranchCursor--
		}
	case "down":
		if a.gitGraphBranchCursor < len(opts)-1 {
			a.gitGraphBranchCursor++
		}
	case "backspace":
		if r := []rune(a.gitGraphBranchFilter); len(r) > 0 {
			a.gitGraphBranchFilter = string(r[:len(r)-1])
			a.gitGraphBranchCursor = 0
		}
	case "enter":
		a.gitGraphBranchPicker = false
		ref := ""
		if a.gitGraphBranchCursor > 0 && a.gitGraphBranchCursor < len(opts) {
			ref = opts[a.gitGraphBranchCursor]
		}
		a.gitGraphBranchRef = ref
		return a, a.reloadGitGraph(p)
	default:
		if len(msg.String()) == 1 {
			a.gitGraphBranchFilter += msg.String()
			a.gitGraphBranchCursor = 0
		}
	}
	return a, nil
}

func (a *App) renderGitGraphBranchPicker(width, height int) string {
	boxW := minInt(width-4, maxInt(44, width*45/100))
	innerW := maxInt(24, boxW-6)
	opts := a.gitGraphBranchList()
	// Fit the list to the branch count so a two-branch repo doesn't get a
	// half-empty modal; the cap keeps a hundred-branch one scrolling instead.
	listH := minInt(maxInt(3, len(opts)), maxInt(3, minInt(14, height-12)))

	lines := tunnelModalChrome("GIT", tabAccentColor(TabGit), "Filtrar por branch",
		"o grafo passa a percorrer só esta branch", "", innerW)
	lines = append(lines, "", StyleMuted.Render("filtro  ")+StyleNormal.Render(truncate(a.gitGraphBranchFilter, innerW-9)+"█"), "")

	a.gitGraphBranchCursor = minInt(a.gitGraphBranchCursor, maxInt(0, len(opts)-1))
	a.gitGraphBranchScroll = ensureVisible(a.gitGraphBranchCursor, a.gitGraphBranchScroll, listH, len(opts))
	start := a.gitGraphBranchScroll
	end := minInt(start+listH, len(opts))
	for i := start; i < end; i++ {
		style, prefix := StyleNormal, "  "
		if i == 0 {
			style = StyleMuted
		}
		if i > 0 && opts[i] == a.gitGraphBranchRef {
			style = StyleAccent
		}
		if i == a.gitGraphBranchCursor {
			style, prefix = StyleSelected, "▸ "
		}
		lines = append(lines, style.Render(truncate(prefix+opts[i], innerW)))
	}
	for i := end - start; i < listH; i++ {
		lines = append(lines, "")
	}
	if rem := len(opts) - end; rem > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("↓ %d abaixo", rem)))
	}
	lines = append(lines, "", StyleMuted.Render("digite filtra  ·  ↑↓ escolhe  ·  enter aplica  ·  esc cancela"))
	return tunnelModalBox(lines, boxW, len(lines), tabAccentColor(TabGit))
}

// --- rendering -----------------------------------------------------------

func (a *App) gitGraphViewport() int {
	v := a.projectPanelHeight()*62/100 - 4
	if v < 4 {
		return 4
	}
	return v
}

// gitGraphPaneViewport is the visible height of the two bottom panes, used
// for page-sized scroll steps.
func (a *App) gitGraphPaneViewport() int {
	h := maxInt(8, a.projectPanelHeight())
	return maxInt(2, maxInt(6, h-maxInt(8, h*62/100)-2)-2)
}

// graphPaneLine is one row of a scrollable pane: an optional pre-styled
// gutter that stays pinned, plus plain text that scrolls horizontally.
type graphPaneLine struct {
	prefix string
	text   string
	style  lipgloss.Style
}

// renderGraphScrollPane draws a titled box with both scroll axes over plain
// text, clamping the offsets in place now that the pane size is known.
func renderGraphScrollPane(title string, lines []graphPaneLine, vScroll, hScroll *int, width, height int, focused bool) string {
	inner := maxInt(1, height-2)
	prefixW, maxLine := 0, 0
	for _, l := range lines {
		if w := lipgloss.Width(l.prefix); w > prefixW {
			prefixW = w
		}
		if w := lipgloss.Width(l.text); w > maxLine {
			maxLine = w
		}
	}
	textW := maxInt(4, width-2-prefixW)

	*vScroll = clampScroll(*vScroll, inner, len(lines))
	*hScroll = clampScroll(*hScroll, textW, maxLine)

	start := *vScroll
	end := minInt(start+inner, len(lines))
	out := make([]string, 0, inner)
	for i := start; i < end; i++ {
		l := lines[i]
		out = append(out, l.prefix+strings.Repeat(" ", prefixW-lipgloss.Width(l.prefix))+
			l.style.Render(sliceColumns(l.text, *hScroll, textW)))
	}
	if len(lines) > inner {
		title += fmt.Sprintf("  ↕ %d/%d", end, len(lines))
	}
	if maxLine > textW || *hScroll > 0 {
		title += fmt.Sprintf("  ↔ %d", *hScroll)
	}
	return panelBox(title, fitExactLines(out, inner), width, height, focused)
}

func (a *App) renderGitGraph(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())

	if len(a.gitGraphNodes()) == 0 {
		empty := "Sem commits para desenhar o grafo."
		if a.gitGraphBranchRef != "" {
			empty = "Sem commits em " + a.gitGraphBranchRef + " — B para trocar de branch."
		}
		view := panelBox("GIT GRAPH", fitExactLines([]string{StyleMuted.Render(empty)}, h-2), w, h, true)
		if a.gitGraphBranchPicker {
			view = overlayCentered(view, a.renderGitGraphBranchPicker(w, h), w, h)
		}
		return view
	}

	topH := maxInt(8, h*62/100)
	bottomH := maxInt(6, h-topH-2)

	list := a.renderGitGraphList(w, topH)
	halfW := w / 2
	detail := renderGraphScrollPane("COMMIT DETAIL", a.gitGraphDetailLines(),
		&a.gitGraphDetailScroll, &a.gitGraphDetailHScroll, halfW, bottomH, a.gitGraphFocus == gitGraphFocusDetail)
	files := renderGraphScrollPane(a.gitGraphFilesTitle(), a.gitGraphFilesLines(),
		&a.gitGraphFilesScroll, &a.gitGraphFilesHScroll, w-halfW, bottomH, a.gitGraphFocus == gitGraphFocusFiles)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, detail, files)

	hint := StyleMuted.Render(truncate("tab painel · ↑↓ move/scroll · ←→ scroll lateral (shift = 10x) · B branch · enter detalhe · r recarregar · esc voltar", w))
	view := lipgloss.JoinVertical(lipgloss.Left, list, bottom, hint)
	if a.gitGraphBranchPicker {
		view = overlayCentered(view, a.renderGitGraphBranchPicker(w, h), w, h)
	}
	return view
}

func (a *App) renderGitGraphList(width, height int) string {
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner)
	nodes := a.gitGraphNodes()
	a.gitGraphScroll = ensureVisible(a.gitGraphCursor, a.gitGraphScroll, viewport, len(nodes))
	start := a.gitGraphScroll
	end := minInt(start+viewport, len(nodes))

	// Every node's cells slice is already padded to (maxLane+1)*2 by the
	// layout algorithm, so the graph column lines up across every row —
	// commit or bare connector — with no extra padding needed here.
	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		lines = append(lines, a.renderGitGraphListRow(nodes[i], i == a.gitGraphCursor, width-2))
	}
	title := panelTitle("COMMITS", fmt.Sprint(len(nodes)))
	if a.gitGraphBranchRef != "" {
		title += " · " + a.gitGraphBranchRef
	} else {
		title += " · todas"
	}
	return panelBox(title, fitExactLines(lines, inner), width, height, a.gitGraphFocus == gitGraphFocusCommits)
}

func (a *App) renderGitGraphListRow(node graphNode, selected bool, width int) string {
	cursor := "  "
	if selected {
		cursor = StyleSelected.Render("▸ ")
	}

	isHead := node.commit != nil && node.commit.IsHead
	graph := renderGraphCells(node.cells, isHead)
	if node.commit == nil {
		return cursor + graph
	}
	c := node.commit

	subjStyle := StyleNormal
	if selected {
		subjStyle = StyleSelected
	}

	left := graph + " " + graphCommitIcon(c.Subject)
	if badge := renderGitGraphRefBadges(c.Refs); badge != "" {
		left += " " + badge
	}
	left += " " + subjStyle.Render(c.Subject)

	const hashW, dateW, authorW = 8, 11, 14
	// +3: os 2 do prefixo de cursor e 1 do separador antes do autor. Sem isso
	// a linha estourava a caixa em 1 coluna e o hash saía com reticências.
	rightW := hashW + dateW + authorW + 3
	leftW := maxInt(10, width-rightW)
	if lipgloss.Width(left) > leftW {
		left = ansi.Truncate(left, leftW, "…")
	} else {
		left += strings.Repeat(" ", leftW-lipgloss.Width(left))
	}

	author := StyleWarning.Render(padRight(truncate(c.Author, authorW-1), authorW))
	date := StyleAccent.Render(padRight(c.Date, dateW))
	hash := StyleHealthy.Render(padRight(c.Short, hashW))

	return cursor + left + " " + author + date + hash
}

func (a *App) gitGraphDetailLines() []graphPaneLine {
	node, ok := a.selectedGitGraphNode()
	if !ok {
		return []graphPaneLine{{text: "selecione um commit", style: StyleMuted}}
	}
	c := node.commit
	lines := []graphPaneLine{
		{prefix: StyleMuted.Render("Commit  "), text: c.Hash, style: StyleAccent},
		{prefix: StyleMuted.Render("Author  "), text: c.Author + " <" + c.AuthorEmail + ">", style: StyleWarning},
		{prefix: StyleMuted.Render("Date    "), text: c.Date, style: StyleNormal},
	}
	if len(c.Refs) > 0 {
		lines = append(lines, graphPaneLine{
			prefix: StyleMuted.Render("Refs    "),
			text:   strings.Join(c.Refs, ", "),
			style:  lipgloss.NewStyle().Foreground(ColorPink).Bold(true),
		})
	}
	for i, parent := range c.Parents {
		label := "Parent  "
		if i > 0 {
			label = "        "
		}
		lines = append(lines, graphPaneLine{prefix: StyleMuted.Render(label), text: shortGitHash(parent), style: StyleAccent})
	}
	lines = append(lines, graphPaneLine{})

	body := c.Subject
	if a.gitGraphDetailHash == c.Hash && a.gitGraphDetailMsg != "" {
		body = a.gitGraphDetailMsg
	}
	for _, l := range strings.Split(strings.ReplaceAll(strings.TrimRight(body, "\n"), "\t", "    "), "\n") {
		lines = append(lines, graphPaneLine{text: sanitizeTerminalLine(l), style: StyleNormal})
	}
	return lines
}

func (a *App) gitGraphFilesTitle() string {
	title := "CHANGED FILES"
	node, ok := a.selectedGitGraphNode()
	if !ok || a.gitGraphDetailHash != node.commit.Hash {
		return title
	}
	ins, del := 0, 0
	for _, f := range a.gitGraphDetailFiles {
		ins += f.Insertions
		del += f.Deletions
	}
	return fmt.Sprintf("%s (%d)  +%d -%d", title, len(a.gitGraphDetailFiles), ins, del)
}

func (a *App) gitGraphFilesLines() []graphPaneLine {
	node, ok := a.selectedGitGraphNode()
	if !ok {
		return []graphPaneLine{{text: "selecione um commit", style: StyleMuted}}
	}
	if a.gitGraphDetailHash != node.commit.Hash {
		return []graphPaneLine{{text: "carregando…", style: StyleMuted}}
	}
	if len(a.gitGraphDetailFiles) == 0 {
		return []graphPaneLine{{text: "(nenhum arquivo)", style: StyleMuted}}
	}
	lines := make([]graphPaneLine, 0, len(a.gitGraphDetailFiles))
	for _, f := range a.gitGraphDetailFiles {
		prefix := StyleHealthy.Render(padRight(fmt.Sprintf("+%d", f.Insertions), 6)) +
			StyleUnhealthy.Render(padRight(fmt.Sprintf("-%d", f.Deletions), 6)) +
			StyleMuted.Render("│ ")
		lines = append(lines, graphPaneLine{prefix: prefix, text: f.Path, style: StyleNormal})
	}
	return lines
}
