package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/pkg/version"
)

// Premium left rail — brand, grouped nav, live badges, footer meters.

func (a *App) renderProjectSidebar() string {
	return a.renderProjectSidebarH(a.projectPanelHeight())
}

func (a *App) renderProjectSidebarH(height int) string {
	width := 26
	if a.projectCompact() {
		width = 22
	}
	inner := maxInt(16, width-2)
	p := a.currentProject()
	accent := tabAccentColor(a.tab)

	top := make([]string, 0, 24)
	top = append(top, a.sidebarBrandBlock(p, inner)...)
	top = append(top, sidebarRule(inner, accent))
	top = append(top, a.sidebarNavBlock(p, inner)...)

	foot := a.sidebarFooterLines(p, accent)
	topH := len(top)
	footH := len(foot)
	blank := maxInt(0, height-2-topH-1-footH) // borders + divider before footer

	rows := make([]string, 0, topH+blank+1+footH)
	rows = append(rows, top...)
	for i := 0; i < blank; i++ {
		rows = append(rows, "")
	}
	rows = append(rows, sidebarRule(inner, ColorBorder))
	rows = append(rows, foot...)

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(width).
		Height(maxInt(height, lipgloss.Height(body)+2)).
		Align(lipgloss.Left, lipgloss.Top).
		Render(body)
}

func (a *App) sidebarBrandBlock(p *core.Project, width int) []string {
	accent := tabAccentColor(a.tab)
	mark := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("◆")
	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("devscope")
	ver := StyleMuted.Render("v" + version.Version)
	rows := []string{mark + " " + title + " " + ver}
	if p == nil {
		return rows
	}
	rows = append(rows,
		StyleMuted.Render(truncate(p.Name, width)),
		projectStatusStyle(p.Status).Render(statusText(p.Status))+
			StyleMuted.Render("  ")+
			healthDot(p.Health)+" "+healthShort(p.Health),
	)
	if branch := sidebarBranchLine(p, width); branch != "" {
		rows = append(rows, branch)
	}
	return rows
}

func healthDot(h core.HealthStatus) string {
	switch h {
	case core.HealthHealthy:
		return StyleHealthy.Render("●")
	case core.HealthUnhealthy:
		return StyleUnhealthy.Render("●")
	default:
		return StyleMuted.Render("○")
	}
}

func healthShort(h core.HealthStatus) string {
	switch h {
	case core.HealthHealthy:
		return StyleHealthy.Render("ok")
	case core.HealthUnhealthy:
		return StyleUnhealthy.Render("bad")
	default:
		return StyleMuted.Render("n/a")
	}
}

func sidebarBranchLine(p *core.Project, width int) string {
	if p.Git == nil || !p.Git.IsRepo || p.Git.Branch == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(ColorWarning).Render("⑂ " + truncate(p.Git.Branch, maxInt(8, width-3)))
}

func (a *App) sidebarNavBlock(p *core.Project, width int) []string {
	groups := []struct {
		title string
		tabs  []Tab
	}{
		{"SCOPE", []Tab{TabOverview, TabGit, TabContainers, TabKubernetes}},
		{"WATCH", []Tab{TabHealth, TabLogs, TabMetrics}},
		{"TOOLS", []Tab{TabAPI, TabDatabase, TabWebSocket, TabNgrok, TabJenkins}},
		{"UTILS", []Tab{TabRoutes}},
	}
	var rows []string
	for gi, g := range groups {
		if gi > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, sidebarGroupLabel(g.title, width, tabAccentColor(g.tabs[0])))
		for _, t := range g.tabs {
			rows = append(rows, a.renderProjectSidebarRow(t, width, p))
		}
	}
	return rows
}

func (a *App) sidebarFooterLines(p *core.Project, accent lipgloss.Color) []string {
	m := a.snapshot.HostMetrics
	cpu := m.CPUPercent
	ramPct := m.MemoryPercent
	disk := m.DiskPercent
	ramLabel := fmt.Sprintf("%.0f%%", ramPct)
	if m.MemoryTotalMB > 0 {
		ramLabel = fmt.Sprintf("%.1fG/%.0fG", float64(m.MemoryUsedMB)/1024, float64(m.MemoryTotalMB)/1024)
	}
	_ = p
	return []string{
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render("RESUMO RÁPIDO"),
		StyleMuted.Render("CPU  ") + meterBar(cpu, 8) + StyleMuted.Render(fmt.Sprintf(" %.0f%%", cpu)),
		StyleMuted.Render("RAM  ") + meterBar(ramPct, 8) + StyleMuted.Render(" "+ramLabel),
		StyleMuted.Render("DISK ") + meterBar(disk, 8) + StyleMuted.Render(fmt.Sprintf(" %.0f%%", disk)),
		StyleMuted.Render("tab · shift+tab · esc"),
	}
}

func meterBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int((pct/100.0)*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	st := StyleMetricCPU
	switch {
	case pct >= 80:
		st = StyleUnhealthy
	case pct >= 50:
		st = StyleMetricRAM
	}
	return st.Render(bar)
}

func sidebarGroupLabel(title string, width int, accent lipgloss.Color) string {
	label := lipgloss.NewStyle().Foreground(accent).Faint(true).Bold(true).Render(title)
	gap := width - lipgloss.Width(title) - 1
	if gap < 1 {
		gap = 1
	}
	return label + " " + StyleMuted.Render(strings.Repeat("·", gap))
}

func sidebarRule(width int, accent lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(accent).Faint(true).Render(strings.Repeat("─", width))
}

func tabAccentColor(t Tab) lipgloss.Color {
	switch t {
	case TabGit:
		return ColorWarning
	case TabContainers:
		return ColorDocker
	case TabKubernetes:
		return ColorK8s
	case TabHealth:
		return ColorSuccess
	case TabLogs:
		return ColorAccent
	case TabMetrics:
		return ColorPython
	case TabAPI:
		return ColorPrimary
	case TabDatabase:
		return ColorDocker
	case TabJSON:
		return ColorWarning
	case TabJWT:
		return ColorSuccess
	case TabRoutes:
		return ColorPrimary
	case TabWebSocket:
		return ColorAccent
	case TabNgrok:
		return ColorSuccess
	case TabJenkins:
		return ColorK8s
	default:
		return ColorHighlight
	}
}

func tabGlyph(t Tab) string {
	switch t {
	case TabOverview:
		return "⌂"
	case TabGit:
		return "⑂"
	case TabContainers:
		return "▣"
	case TabKubernetes:
		return "⎈"
	case TabHealth:
		return "✚"
	case TabLogs:
		return "☰"
	case TabMetrics:
		return "▦"
	case TabAPI:
		return "↯"
	case TabDatabase:
		return "▤"
	case TabJSON:
		return "{"
	case TabJWT:
		return "⚿"
	case TabRoutes:
		return "⇄"
	case TabWebSocket:
		return "⚡"
	case TabNgrok:
		return "⇪"
	case TabJenkins:
		return "⚙"
	default:
		return "·"
	}
}

func tabActiveBg(_ Tab) lipgloss.Color {
	// Theme-driven so light/dracula don't keep dark-only tints.
	return ColorSelBg
}

func (a *App) renderProjectSidebarRow(t Tab, width int, _ *core.Project) string {
	accentCol := tabAccentColor(t)
	accent := lipgloss.NewStyle().Foreground(accentCol).Bold(true)
	name := t.String()

	if t == a.tab {
		left := "▌" + tabGlyph(t) + " " + name
		pad := width - lipgloss.Width(left)
		if pad < 0 {
			pad = 0
		}
		line := accent.Render("▌"+tabGlyph(t)) + " " +
			lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(name) +
			strings.Repeat(" ", pad)
		return lipgloss.NewStyle().Width(width).Background(tabActiveBg(t)).Render(line)
	}

	left := " " + tabGlyph(t) + " " + name
	pad := width - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	line := " " + accent.Render(tabGlyph(t)) + " " + StyleMuted.Render(name) +
		strings.Repeat(" ", pad)
	return lipgloss.NewStyle().Width(width).Render(line)
}
