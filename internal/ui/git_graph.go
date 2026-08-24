package ui

import (
	"fmt"
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
func graphCommitIcon(subject string) string {
	s := strings.ToLower(subject)
	switch {
	case strings.HasPrefix(s, "merge"):
		return StyleAccent.Render("🔀")
	case strings.Contains(s, "security"):
		return StyleUnhealthy.Render("🔒")
	case strings.HasPrefix(s, "fix") || strings.Contains(s, "bug"):
		return StyleWarning.Render("⚠")
	case strings.HasPrefix(s, "feat") || strings.HasPrefix(s, "add"):
		return StyleHealthy.Render("✚")
	case strings.HasPrefix(s, "docs") || strings.HasPrefix(s, "doc"):
		return StyleMuted.Render("📄")
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

func (a *App) openGitGraph(p *core.Project) tea.Cmd {
	a.gitSubview = gitSubviewGraph
	a.gitGraphCursor = 0
	a.gitGraphScroll = 0
	commits := collectors.GitLogDAG(p.Path, 300)
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

func (a *App) handleGitGraphKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.gitSubview = gitSubviewMain
		return a, nil
	case "up", "k":
		if i := a.prevGitGraphCommitRow(a.gitGraphCursor); i >= 0 {
			a.gitGraphCursor = i
			return a, a.requestGitGraphDetail(p)
		}
	case "down", "j":
		if i := a.nextGitGraphCommitRow(a.gitGraphCursor); i >= 0 {
			a.gitGraphCursor = i
			return a, a.requestGitGraphDetail(p)
		}
	case "pgup":
		a.gitGraphScroll = maxInt(0, a.gitGraphScroll-a.gitGraphViewport())
	case "pgdown":
		a.gitGraphScroll = minInt(maxInt(0, len(a.gitGraphNodes())-1), a.gitGraphScroll+a.gitGraphViewport())
	case "r":
		return a, a.openGitGraph(p)
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

func (a *App) gitGraphViewport() int {
	v := a.projectPanelHeight()*62/100 - 4
	if v < 4 {
		return 4
	}
	return v
}

func (a *App) renderGitGraph(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())

	if len(a.gitGraphNodes()) == 0 {
		return renderApiTitledBox("GIT GRAPH", fitExactLines([]string{StyleMuted.Render("Sem commits para desenhar o grafo.")}, h-2), w, h, true)
	}

	topH := maxInt(8, h*62/100)
	bottomH := maxInt(8, h-topH-1)

	list := a.renderGitGraphList(w, topH)
	halfW := w / 2
	detail := a.renderGitGraphDetailPane(halfW, bottomH)
	files := a.renderGitGraphFilesPane(w-halfW, bottomH)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, detail, files)
	return lipgloss.JoinVertical(lipgloss.Left, list, bottom)
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
	return renderApiTitledBox(fmt.Sprintf("COMMITS (%d)", len(nodes)), fitExactLines(lines, inner), width, height, true)
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
	rightW := hashW + dateW + authorW + 2
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

func (a *App) renderGitGraphDetailPane(width, height int) string {
	inner := maxInt(3, height-2)
	node, ok := a.selectedGitGraphNode()
	if !ok {
		return renderApiTitledBox("COMMIT DETAIL", fitExactLines([]string{StyleMuted.Render("selecione um commit")}, inner), width, height, false)
	}
	c := node.commit
	lines := []string{
		StyleMuted.Render("Commit  ") + StyleAccent.Render(c.Hash),
		StyleMuted.Render("Author  ") + StyleWarning.Render(c.Author) + StyleMuted.Render(" <"+c.AuthorEmail+">"),
		StyleMuted.Render("Date    ") + StyleNormal.Render(c.Date),
	}
	if len(c.Parents) > 0 {
		parent := c.Parents[0]
		if len(parent) > 8 {
			parent = parent[:8]
		}
		lines = append(lines, StyleMuted.Render("Parent  ")+StyleAccent.Render(parent))
	}
	lines = append(lines, "")
	if a.gitGraphDetailHash == c.Hash && a.gitGraphDetailMsg != "" {
		lines = append(lines, wrapText(a.gitGraphDetailMsg, maxInt(20, width-4))...)
	} else {
		lines = append(lines, StyleMuted.Render(c.Subject))
	}
	return renderApiTitledBox("COMMIT DETAIL", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderGitGraphFilesPane(width, height int) string {
	inner := maxInt(3, height-2)
	node, ok := a.selectedGitGraphNode()
	if !ok {
		return renderApiTitledBox("CHANGED FILES", fitExactLines([]string{StyleMuted.Render("selecione um commit")}, inner), width, height, false)
	}
	if a.gitGraphDetailHash != node.commit.Hash {
		return renderApiTitledBox("CHANGED FILES", fitExactLines([]string{a.loadingText("carregando…")}, inner), width, height, false)
	}
	if len(a.gitGraphDetailFiles) == 0 {
		return renderApiTitledBox("CHANGED FILES", fitExactLines([]string{StyleMuted.Render("(nenhum arquivo)")}, inner), width, height, false)
	}
	insTotal, delTotal := 0, 0
	fileLines := make([]string, 0, len(a.gitGraphDetailFiles))
	for _, f := range a.gitGraphDetailFiles {
		insTotal += f.Insertions
		delTotal += f.Deletions
		fileLines = append(fileLines, StyleNormal.Render(truncate(f.Path, maxInt(10, width-16)))+
			" "+StyleHealthy.Render(fmt.Sprintf("+%d", f.Insertions))+
			" "+StyleUnhealthy.Render(fmt.Sprintf("-%d", f.Deletions)))
	}
	header := StyleMuted.Render(fmt.Sprintf("%d arquivo(s)  ", len(a.gitGraphDetailFiles))) +
		StyleHealthy.Render(fmt.Sprintf("+%d ", insTotal)) + StyleUnhealthy.Render(fmt.Sprintf("-%d", delTotal))
	lines := append([]string{header, ""}, fileLines...)
	return renderApiTitledBox("CHANGED FILES", fitExactLines(lines, inner), width, height, false)
}
