package ui

import (
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

func (a *App) openGitGraph(p *core.Project) tea.Cmd {
	a.gitSubview = gitSubviewGraph
	a.gitGraphBranchCursor = 0
	a.gitGraphScroll = 0
	a.gitGraphFocusGraph = false
	a.gitGraphReachable = nil
	a.gitGraphRows = collectors.GitLogGraph(p.Path, 300)
	return nil
}

// gitGraphBranches prepends a synthetic "Todas" entry (index 0, clears the
// highlight filter) ahead of the project's real branches.
func (a *App) gitGraphBranches(p *core.Project) []core.GitBranch {
	branches := []core.GitBranch{{Name: "★ Todas"}}
	if p != nil && p.Git != nil {
		branches = append(branches, p.Git.Branches...)
	}
	return branches
}

func (a *App) applyGitGraphSelection(p *core.Project) {
	branches := a.gitGraphBranches(p)
	if a.gitGraphBranchCursor <= 0 || a.gitGraphBranchCursor >= len(branches) {
		a.gitGraphReachable = nil
		return
	}
	a.gitGraphReachable = collectors.CommitsReachableFrom(p.Path, branches[a.gitGraphBranchCursor].Name)
}

func (a *App) handleGitGraphKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	branches := a.gitGraphBranches(p)
	switch msg.String() {
	case "esc", "q":
		a.gitSubview = gitSubviewMain
		return a, nil
	case "left", "h":
		a.gitGraphFocusGraph = false
	case "right", "l":
		a.gitGraphFocusGraph = true
	case "up", "k":
		if a.gitGraphFocusGraph {
			if a.gitGraphScroll > 0 {
				a.gitGraphScroll--
			}
		} else if a.gitGraphBranchCursor > 0 {
			a.gitGraphBranchCursor--
			a.applyGitGraphSelection(p)
		}
	case "down", "j":
		if a.gitGraphFocusGraph {
			if a.gitGraphScroll < len(a.gitGraphRows)-1 {
				a.gitGraphScroll++
			}
		} else if a.gitGraphBranchCursor < len(branches)-1 {
			a.gitGraphBranchCursor++
			a.applyGitGraphSelection(p)
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
	v := a.projectPanelHeight() - 4
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

	leftW := maxInt(18, minInt(34, w*26/100))
	rightW := maxInt(30, w-leftW-1)

	left := a.renderGitGraphBranches(p, leftW, h)
	right := a.renderGitGraphCommits(rightW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (a *App) renderGitGraphBranches(p *core.Project, width, height int) string {
	branches := a.gitGraphBranches(p)
	inner := maxInt(3, height-2)
	lines := make([]string, 0, len(branches))
	for i, b := range branches {
		style := StyleNormal
		if b.Current {
			style = StyleGitBranchHead
		}
		if i == a.gitGraphBranchCursor {
			style = StyleGitSelected
			if !a.gitGraphFocusGraph {
				style = StyleSelected
			}
		}
		marker := "  "
		if b.Current {
			marker = "◈ "
		} else if b.Remote {
			marker = "☁ "
		}
		lines = append(lines, style.Render(truncate(marker+b.Name, width-4)))
	}
	return renderApiTitledBox("BRANCHES", fitExactLines(lines, inner), width, height, !a.gitGraphFocusGraph)
}

func (a *App) renderGitGraphCommits(width, height int) string {
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner)
	rows := a.gitGraphRows
	a.gitGraphScroll = minInt(maxInt(0, a.gitGraphScroll), maxInt(0, len(rows)-1))
	start := a.gitGraphScroll
	end := minInt(start+viewport, len(rows))

	filtering := a.gitGraphReachable != nil
	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		lines = append(lines, a.renderGitGraphRow(rows[i], filtering, width-2))
	}
	return renderApiTitledBox("COMMITS", fitExactLines(lines, inner), width, height, a.gitGraphFocusGraph)
}

func (a *App) renderGitGraphRow(row collectors.GitGraphRow, filtering bool, width int) string {
	prefix := colorizeGraphPrefix(row.Prefix)
	if row.Hash == "" {
		return prefix
	}
	dim := filtering && !a.gitGraphReachable[row.Hash]

	lane := graphRowLaneColor(row.Prefix)
	if dim {
		lane = ColorMuted
	}
	// Colored pill around the short hash, matching the commit's own lane
	// color — the closest terminal-safe echo of the colored badges in a GUI
	// graph view.
	hashBadge := lipgloss.NewStyle().Foreground(ColorBg).Background(lane).Bold(true).Render(" " + row.Short + " ")

	subjStyle := StyleNormal
	if dim {
		subjStyle = StyleMuted
	}
	rest := hashBadge + " " + subjStyle.Render(row.Subject)
	if row.Refs != "" {
		refBg := ColorWarning
		if dim {
			refBg = ColorMuted
		}
		refBadge := lipgloss.NewStyle().Foreground(ColorBg).Background(refBg).Bold(true).Render(" " + row.Refs + " ")
		rest += " " + refBadge
	}
	avail := maxInt(4, width-lipgloss.Width(row.Prefix))
	if lipgloss.Width(rest) > avail {
		rest = ansi.Truncate(rest, avail, "…")
	}
	return prefix + rest
}
