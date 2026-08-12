package ui

import (
	"slices"
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
		width = 20
	}
	if a.projectTiny() {
		width = 16
	}
	inner := maxInt(12, width-2)
	p := a.currentProject()
	accent := tabAccentColor(a.tab)
	// lipgloss Height is content rows; border adds ±2 to the measured box.
	// Keep outer lipgloss.Height(sidebar) == height so it fits the panel.
	contentH := maxInt(1, height-2)

	top := make([]string, 0, 24)
	top = append(top, a.sidebarBrandBlock(p, inner)...)
	top = append(top, sidebarRule(inner, accent))
	top = append(top, a.sidebarNavBlock(p, inner)...)

	foot := a.sidebarFooterLines(p, accent)
	// Prefer nav over meters when vertical space is scarce (VS Code terminal).
	if len(top)+1+len(foot) > contentH {
		foot = []string{StyleMuted.Render("tab · esc")}
	}
	if len(top)+1+len(foot) > contentH {
		foot = nil
	}
	// Still too tall: drop blank group separators, then trim brand.
	if len(top)+len(foot) > contentH {
		top = a.sidebarBrandBlock(p, inner)
		if !a.projectTiny() {
			top = append(top, sidebarRule(inner, accent))
		}
		top = append(top, a.sidebarNavBlockDense(p, inner)...)
	}
	if len(top)+len(foot) > contentH && len(foot) > 0 {
		foot = nil
	}

	blank := 0
	if foot != nil {
		blank = maxInt(0, contentH-len(top)-1-len(foot))
	} else {
		blank = maxInt(0, contentH-len(top))
	}

	rows := make([]string, 0, contentH)
	rows = append(rows, top...)
	for i := 0; i < blank; i++ {
		rows = append(rows, "")
	}
	if foot != nil {
		rows = append(rows, sidebarRule(inner, ColorBorder))
		rows = append(rows, foot...)
	}
	if len(rows) > contentH {
		rows = sidebarWindow(rows, slices.Index(rows, a.renderProjectSidebarRow(a.tab, inner, p)), contentH)
	}

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(width).
		Height(contentH).
		Align(lipgloss.Left, lipgloss.Top).
		Render(body)
}

// sidebarWindow scrolls the nav so the active row stays visible when the rail
// is taller than the terminal.
func sidebarWindow(rows []string, focus, height int) []string {
	if height <= 0 || len(rows) <= height {
		return rows
	}
	start := focus - height/2
	if start > len(rows)-height {
		start = len(rows) - height
	}
	if start < 0 {
		start = 0
	}
	return rows[start : start+height]
}

func (a *App) sidebarBrandBlock(p *core.Project, width int) []string {
	accent := tabAccentColor(a.tab)
	mark := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("◆")
	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("devscope")
	if a.projectTiny() {
		name := "project"
		if p != nil {
			name = p.Name
		}
		return []string{
			mark + " " + StyleNormal.Render(truncate(name, maxInt(6, width-2))),
		}
	}
	ver := StyleMuted.Render("v" + version.Version)
	rows := []string{mark + " " + title + " " + ver}
	if p == nil {
		return rows
	}
	rows = append(rows,
		StyleMuted.Render(truncate(p.Name, width)),
		projectStatusStyle(p.Status).Render(statusText(p.Status))+
			StyleMuted.Render("  ")+
			a.healthDotAnim(p.Health)+" "+healthShort(p.Health),
	)
	if !a.projectCompact() {
		if branch := sidebarBranchLine(p, width); branch != "" {
			rows = append(rows, branch)
		}
	}
	return rows
}

func healthDot(h core.HealthStatus) string {
	return healthDotFrame(h, 0)
}

func (a *App) healthDotAnim(h core.HealthStatus) string {
	return healthDotFrame(h, a.animFrame)
}

func healthDotFrame(h core.HealthStatus, frame int) string {
	switch h {
	case core.HealthHealthy:
		return StyleHealthy.Render(animPulse(frame))
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
	groups := sidebarGroups()
	var rows []string
	for gi, g := range groups {
		if gi > 0 && !a.projectTiny() {
			rows = append(rows, "")
		}
		rows = append(rows, sidebarGroupLabel(g.title, width, g.color))
		for _, t := range g.tabs {
			rows = append(rows, a.renderProjectSidebarRow(t, width, p))
		}
	}
	return rows
}

// sidebarNavBlockDense drops blank separators between groups (short terminals).
func (a *App) sidebarNavBlockDense(p *core.Project, width int) []string {
	groups := sidebarGroups()
	var rows []string
	for _, g := range groups {
		rows = append(rows, sidebarGroupLabel(g.title, width, g.color))
		for _, t := range g.tabs {
			rows = append(rows, a.renderProjectSidebarRow(t, width, p))
		}
	}
	return rows
}

type sidebarGroup struct {
	title string
	color lipgloss.Color
	tabs  []Tab
}

func sidebarGroups() []sidebarGroup {
	return []sidebarGroup{
		{"WATCH", ColorAccent, []Tab{TabOverview, TabMetrics, TabHealth}},
		{"SCOPE", ColorWarning, []Tab{TabGit, TabContainers}},
		{"AUTOMATION", ColorPrimary, []Tab{TabActions, TabJenkins}},
		{"MANAGER", ColorDocker, []Tab{TabSwarm, TabKubernetes}},
		{"TUNNEL", ColorSuccess, []Tab{TabNgrok, TabSSH, TabCFTunnel}},
		{"TOOLS", ColorPink, []Tab{TabRoutes, TabAPI, TabDatabase, TabWebSocket}},
	}
}

func sidebarGroupColorForTab(t Tab) lipgloss.Color {
	for _, g := range sidebarGroups() {
		for _, tab := range g.tabs {
			if tab == t {
				return g.color
			}
		}
	}
	return ColorHighlight
}

func (a *App) sidebarFooterLines(p *core.Project, accent lipgloss.Color) []string {
	_ = p
	_ = accent
	if a.projectTiny() {
		return []string{StyleMuted.Render("tab · esc")}
	}
	return []string{StyleMuted.Render("tab · shift+tab · esc")}
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
	// Abas ocultas do menu (Logs/JSON/JWT) mantêm cor própria.
	switch t {
	case TabLogs:
		return ColorAccent
	case TabJSON:
		return ColorWarning
	case TabJWT:
		return ColorSuccess
	}
	return sidebarGroupColorForTab(t)
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
	case TabSwarm:
		return "⬡"
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
	case TabCFTunnel:
		return "☁"
	case TabSSH:
		return "⇌"
	case TabJenkins:
		return "⚙"
	case TabActions:
		return "▶"
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
	if a.projectTiny() {
		name = truncate(name, maxInt(6, width-4))
	}

	if t == a.tab {
		left := "▌" + tabGlyph(t) + " " + name
		pad := width - lipgloss.Width(left)
		if pad < 0 {
			pad = 0
		}
		return lipgloss.NewStyle().
			Foreground(ColorText).
			Background(tabActiveBg(t)).
			Bold(true).
			Render(left + strings.Repeat(" ", pad))
	}
	left := " " + tabGlyph(t) + " " + name
	pad := width - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	return " " + accent.Render(tabGlyph(t)) + " " + StyleMuted.Render(name) + strings.Repeat(" ", pad)
}
