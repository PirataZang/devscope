package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/pkg/version"
)

const layoutOverhead = 28

func (a *App) renderDashboard() string {
	projects := filterNestedProjects(sortProjects(a.filteredProjects()))
	m := a.snapshot.HostMetrics
	tableW := safeTableWidth(a.width)

	var sections []string
	sections = append(sections, a.renderDashboardHeader(m)...)
	sections = append(sections, "", a.renderProjectsList(projects, tableW))
	sections = append(sections, "", a.renderDashboardFooter(projects))

	return StyleDashboard.Render(strings.Join(sections, "\n"))
}

func (a *App) renderDashboardHeader(m core.HostMetrics) []string {
	compact := a.dashboardCompact()
	brand := StyleBrand.Render("DevScope v"+version.Version) +
		StyleSubtitle.Render(" • Developer Command Center")
	metrics := renderMetricPills(m)
	clock := StyleClock.Render(a.now.Format("15:04:05"))

	var lines []string
	if compact || a.width < 100 {
		lines = append(lines, brand)
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, metrics, "  ", clock))
	} else {
		lines = append(lines, joinWithSpacer(
			lipgloss.JoinHorizontal(lipgloss.Top, brand, "  ", metrics),
			clock,
			maxInt(a.width-layoutOverhead, 60),
		))
	}

	sysLine := fmt.Sprintf(
		"Uptime: %s  •  Load: %s  •  Docker: %d  •  RAM: %d/%d MB  •  %s",
		formatUptime(m.Uptime), m.LoadAvg, m.DockerRunning,
		m.MemoryUsedMB, m.MemoryTotalMB, m.OSInfo,
	)
	if compact {
		lines = append(lines, StyleMuted.Render(sysLine))
	} else {
		lines = append(lines, StyleInnerPanel.Render(
			StyleSection.Render("SYSTEM OVERVIEW")+"\n"+StyleNormal.Render(sysLine),
		))
	}
	return lines
}

func (a *App) renderProjectsList(projects []core.Project, tableW int) string {
	cols := tableColumns(tableW)
	separator := StyleMuted.Render(strings.Repeat("─", tableW))
	tableHeader := renderTableRow(cols, tableRow{
		icon: " ", name: "NAME", branch: "BRANCH", status: "STATUS", ctrs: "CTRS", path: "PATH",
	}, StyleTableHeader, nil, false)

	viewport := a.dashboardProjectsViewport()
	start := a.dashboardScroll
	end := minInt(start+viewport, len(projects))

	lines := []string{
		StyleSection.Render(fmt.Sprintf("PROJECTS (%d)", len(projects))),
		"",
		tableHeader,
		separator,
	}

	if start > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
	}

	if len(projects) == 0 {
		lines = append(lines, StyleMuted.Render("  Scanning projects..."))
	} else {
		for i := start; i < end; i++ {
			p := projects[i]
			ctrs := "-"
			if p.ContainerCount > 0 {
				ctrs = fmt.Sprintf("%d", p.ContainerCount)
			}
			branch := "-"
			if p.Git != nil && p.Git.IsRepo && p.Git.Branch != "" {
				branch = p.Git.Branch
			}

			style := StyleNormal
			selected := i == a.cursor
			if selected {
				style = StyleSelected
			}

			lines = append(lines, renderTableRow(cols, tableRow{
				icon:   frameworkIconPlain(p.Framework.Name),
				name:   p.Name,
				path:   p.Path,
				branch: branch,
				ctrs:   ctrs,
			}, style, &p.Status, selected))
		}
	}

	remaining := len(projects) - end
	if remaining > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", remaining)))
	}

	return StyleInnerPanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderDashboardFooter(projects []core.Project) string {
	running, stopped, degraded := countProjectStatuses(projects)
	summary := lipgloss.JoinHorizontal(lipgloss.Top,
		StyleMuted.Render("Total: ")+StyleNormal.Render(fmt.Sprintf("%d", len(projects))),
		"    ",
		StyleMuted.Render("Running: ")+StyleRunning.Render(fmt.Sprintf("%d", running)),
		"    ",
		StyleMuted.Render("Degraded: ")+StyleWarning.Render(fmt.Sprintf("%d", degraded)),
		"    ",
		StyleMuted.Render("Stopped: ")+StyleStopped.Render(fmt.Sprintf("%d", stopped)),
	)
	if a.filter != "" {
		summary += "    " + StyleWarning.Render("(filtered)")
	}

	footer := strings.Join([]string{
		renderKeybind("↑↓", "navigate"),
		renderKeybind("ENTER", "open"),
		renderKeybind("SHIFT+E", "terminal"),
		renderKeybind("SHIFT+O", "opencode"),
		renderKeybind("/", "filter"),
		renderKeybind("g", "git"),
		renderKeybind("c", "containers"),
		renderKeybind("ctrl+p", "fuzzy"),
		renderKeybind("r", "refresh"),
		renderKeybind("?", "help"),
		renderKeybind("q", "quit"),
	}, "  ")

	return summary + "\n\n" + StyleStatusBar.Render(footer)
}

func (a *App) dashboardCompact() bool {
	return a.height > 0 && a.height < 28
}

func (a *App) dashboardProjectsViewport() int {
	h := a.height
	if h <= 0 {
		return 6
	}
	reserved := 13
	if a.dashboardCompact() {
		reserved = 11
	}
	v := h - reserved
	if v < 3 {
		return 3
	}
	return v
}

func (a *App) syncDashboardScroll(projectCount int) {
	viewport := a.dashboardProjectsViewport()
	a.dashboardScroll = ensureVisible(a.cursor, a.dashboardScroll, viewport, projectCount)
}

func countProjectStatuses(projects []core.Project) (running, stopped, degraded int) {
	for _, p := range projects {
		switch p.Status {
		case core.StatusRunning:
			running++
		case core.StatusDegraded:
			degraded++
		case core.StatusStopped:
			stopped++
		}
	}
	return running, stopped, degraded
}

func safeTableWidth(termWidth int) int {
	if termWidth <= 0 {
		return 78
	}
	usable := termWidth - layoutOverhead
	if usable < 72 {
		return 72
	}
	return usable
}

type tableCols struct {
	icon, name, path, status, branch, ctrs, gap, total int
}

type tableRow struct {
	icon, name, path, status, branch, ctrs string
}

func tableColumns(tableW int) tableCols {
	c := tableCols{
		icon:   3,
		status: 10,
		branch: 16,
		ctrs:   4,
		gap:    1,
	}
	flexible := tableW - c.icon - c.status - c.branch - c.ctrs - c.gap*5
	c.name = maxInt(12, flexible/3)
	c.path = maxInt(18, flexible-c.name)
	c.total = tableW
	return c
}

func renderTableRow(c tableCols, r tableRow, style lipgloss.Style, status *core.ProjectStatus, selected bool) string {
	gap := lipgloss.NewStyle().Width(c.gap).Render("")
	cell := func(width int, text string) string {
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	}
	statusCell := func() string {
		if status != nil {
			return renderStatusCell(c.status, *status, selected)
		}
		return cell(c.status, r.status)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		cell(c.icon, r.icon),
		gap,
		cell(c.name, r.name),
		gap,
		cell(c.branch, r.branch),
		gap,
		statusCell(),
		gap,
		cell(c.ctrs, r.ctrs),
		gap,
		cell(c.path, r.path),
	)
}

func renderStatusCell(width int, s core.ProjectStatus, selected bool) string {
	st := projectStatusStyle(s)
	if selected {
		st = st.Bold(true)
	}
	return st.Width(width).MaxWidth(width).Render(statusText(s))
}

func projectStatusStyle(s core.ProjectStatus) lipgloss.Style {
	switch s {
	case core.StatusRunning:
		return StyleRunning
	case core.StatusStopped:
		return StyleStopped
	case core.StatusDegraded:
		return StyleWarning
	default:
		return StyleMuted
	}
}

func statusText(s core.ProjectStatus) string {
	switch s {
	case core.StatusRunning:
		return "● Run"
	case core.StatusStopped:
		return "● Stop"
	case core.StatusDegraded:
		return "● Deg"
	default:
		return "◌ ???"
	}
}

func filterNestedProjects(projects []core.Project) []core.Project {
	if len(projects) < 2 {
		return projects
	}
	var result []core.Project
	for _, p := range projects {
		if isNestedProject(p.Path, projects) {
			continue
		}
		result = append(result, p)
	}
	return result
}

func isNestedProject(path string, projects []core.Project) bool {
	path = filepath.Clean(path)
	for _, other := range projects {
		otherPath := filepath.Clean(other.Path)
		if path == otherPath {
			continue
		}
		if strings.HasPrefix(path, otherPath+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func joinWithSpacer(left, right string, totalWidth int) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	spacer := totalWidth - leftW - rightW
	if spacer < 1 {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", spacer), right)
}
