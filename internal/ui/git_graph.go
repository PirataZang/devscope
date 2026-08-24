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

// graphLanePalette cycles by column so a lane keeps a consistent color as it
// runs down through history — the same convention gitk/tig/lazygit use.
var graphLanePalette = []lipgloss.Color{ColorAccent, ColorSuccess, ColorWarning, ColorPink, ColorHighlight, ColorPrimary, ColorDanger}

func graphLaneColor(col int) lipgloss.Color {
	return graphLanePalette[col%len(graphLanePalette)]
}

// graphGlyph swaps git's blocky ASCII graph characters for rounder Unicode
// look-alikes. A terminal is a fixed character grid — it can't draw the
// smooth bezier curves a GUI graph does — so this is the closest practical
// upgrade: solid commit dots and cleaner line-drawing glyphs instead of
// bare `*`, `|`, `/`, `\`.
func graphGlyph(r rune) rune {
	switch r {
	case '*':
		return '●'
	case '|':
		return '│'
	case '/':
		return '╱'
	case '\\':
		return '╲'
	default:
		return r
	}
}

// graphRowLaneColor is the color of a commit row's own marker column — used
// to badge its hash the same color as its line in the graph.
func graphRowLaneColor(prefix string) lipgloss.Color {
	if idx := strings.IndexRune(prefix, '*'); idx >= 0 {
		return graphLaneColor(idx)
	}
	return ColorAccent
}

func colorizeGraphPrefix(prefix string) string {
	var b strings.Builder
	col := 0
	for _, r := range prefix {
		if r == ' ' {
			b.WriteRune(' ')
			col++
			continue
		}
		style := lipgloss.NewStyle().Foreground(graphLaneColor(col))
		b.WriteString(style.Render(string(graphGlyph(r))))
		col++
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

func renderGitGraphRefBadge(refs string) string {
	first := strings.TrimSpace(strings.SplitN(refs, ",", 2)[0])
	first = strings.TrimPrefix(first, "HEAD -> ")
	if first == "" {
		return ""
	}
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
	a.gitGraphRows = collectors.GitLogGraph(p.Path, 300)
	// Land on the first real commit row, not a connector-only line.
	if a.gitGraphRows != nil && a.gitGraphRows[0].Hash == "" {
		if i := a.nextGitGraphCommitRow(-1); i >= 0 {
			a.gitGraphCursor = i
		}
	}
	a.gitGraphDetailHash = ""
	a.gitGraphDetailMsg = ""
	a.gitGraphDetailFiles = nil
	return a.requestGitGraphDetail(p)
}

func (a *App) selectedGitGraphRow() (collectors.GitGraphRow, bool) {
	if a.gitGraphCursor < 0 || a.gitGraphCursor >= len(a.gitGraphRows) {
		return collectors.GitGraphRow{}, false
	}
	row := a.gitGraphRows[a.gitGraphCursor]
	if row.Hash == "" {
		return collectors.GitGraphRow{}, false
	}
	return row, true
}

func (a *App) nextGitGraphCommitRow(from int) int {
	for i := from + 1; i < len(a.gitGraphRows); i++ {
		if a.gitGraphRows[i].Hash != "" {
			return i
		}
	}
	return -1
}

func (a *App) prevGitGraphCommitRow(from int) int {
	for i := from - 1; i >= 0; i-- {
		if a.gitGraphRows[i].Hash != "" {
			return i
		}
	}
	return -1
}

func (a *App) requestGitGraphDetail(p *core.Project) tea.Cmd {
	row, ok := a.selectedGitGraphRow()
	if !ok {
		a.gitGraphDetailHash = ""
		return nil
	}
	a.gitGraphDetailGen++
	gen := a.gitGraphDetailGen
	hash := row.Hash
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
		a.gitGraphScroll = minInt(maxInt(0, len(a.gitGraphRows)-1), a.gitGraphScroll+a.gitGraphViewport())
	case "r":
		return a, a.openGitGraph(p)
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

	if len(a.gitGraphRows) == 0 {
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
	rows := a.gitGraphRows
	a.gitGraphScroll = ensureVisible(a.gitGraphCursor, a.gitGraphScroll, viewport, len(rows))
	start := a.gitGraphScroll
	end := minInt(start+viewport, len(rows))

	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		lines = append(lines, a.renderGitGraphListRow(rows[i], i == a.gitGraphCursor, width-2))
	}
	return renderApiTitledBox(fmt.Sprintf("COMMITS (%d)", len(rows)), fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderGitGraphListRow(row collectors.GitGraphRow, selected bool, width int) string {
	prefix := colorizeGraphPrefix(row.Prefix)
	if row.Hash == "" {
		return prefix
	}

	subjStyle := StyleNormal
	if selected {
		subjStyle = StyleSelected
	}

	left := prefix + " " + graphCommitIcon(row.Subject)
	if badge := renderGitGraphRefBadge(row.Refs); badge != "" {
		left += " " + badge
	}
	left += " " + subjStyle.Render(row.Subject)

	const hashW, dateW, authorW = 8, 11, 14
	rightW := hashW + dateW + authorW + 2
	leftW := maxInt(10, width-rightW)
	if lipgloss.Width(left) > leftW {
		left = ansi.Truncate(left, leftW, "…")
	} else {
		left += strings.Repeat(" ", leftW-lipgloss.Width(left))
	}

	author := StyleWarning.Render(padRight(truncate(row.Author, authorW-1), authorW))
	date := StyleAccent.Render(padRight(row.Date, dateW))
	hash := StyleHealthy.Render(padRight(row.Short, hashW))

	cursor := "  "
	if selected {
		cursor = StyleSelected.Render("▸ ")
	}
	return cursor + left + " " + author + date + hash
}

func (a *App) renderGitGraphDetailPane(width, height int) string {
	inner := maxInt(3, height-2)
	row, ok := a.selectedGitGraphRow()
	if !ok {
		return renderApiTitledBox("COMMIT DETAIL", fitExactLines([]string{StyleMuted.Render("selecione um commit")}, inner), width, height, false)
	}
	lines := []string{
		StyleMuted.Render("Commit  ") + StyleAccent.Render(row.Hash),
		StyleMuted.Render("Author  ") + StyleWarning.Render(row.Author) + StyleMuted.Render(" <"+row.AuthorEmail+">"),
		StyleMuted.Render("Date    ") + StyleNormal.Render(row.Date),
	}
	if row.Parent != "" {
		lines = append(lines, StyleMuted.Render("Parent  ")+StyleAccent.Render(row.Parent))
	}
	lines = append(lines, "")
	if a.gitGraphDetailHash == row.Hash && a.gitGraphDetailMsg != "" {
		lines = append(lines, wrapText(a.gitGraphDetailMsg, maxInt(20, width-4))...)
	} else {
		lines = append(lines, StyleMuted.Render(row.Subject))
	}
	return renderApiTitledBox("COMMIT DETAIL", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderGitGraphFilesPane(width, height int) string {
	inner := maxInt(3, height-2)
	row, ok := a.selectedGitGraphRow()
	if !ok {
		return renderApiTitledBox("CHANGED FILES", fitExactLines([]string{StyleMuted.Render("selecione um commit")}, inner), width, height, false)
	}
	if a.gitGraphDetailHash != row.Hash {
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
