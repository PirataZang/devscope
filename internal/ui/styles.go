package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7C3AED")
	ColorAccent    = lipgloss.Color("#60A5FA")
	ColorSuccess   = lipgloss.Color("#22C55E")
	ColorWarning   = lipgloss.Color("#EAB308")
	ColorDanger    = lipgloss.Color("#EF4444")
	ColorPink      = lipgloss.Color("#F472B6")
	ColorMuted     = lipgloss.Color("#6B7280")
	ColorText      = lipgloss.Color("#F9FAFB")
	ColorSubtext   = lipgloss.Color("#9CA3AF")
	ColorBorder    = lipgloss.Color("#374151")
	ColorBg        = lipgloss.Color("#111827")
	ColorBgPanel   = lipgloss.Color("#1F2937")
	ColorHighlight = lipgloss.Color("#A78BFA")

	ColorGo      = lipgloss.Color("#00ADD8")
	ColorDocker  = lipgloss.Color("#2496ED")
	ColorVue     = lipgloss.Color("#42B883")
	ColorLaravel = lipgloss.Color("#FF2D20")
	ColorNode    = lipgloss.Color("#68A063")
	ColorPHP     = lipgloss.Color("#777BB4")
	ColorPython  = lipgloss.Color("#FFD43B")
	ColorRust    = lipgloss.Color("#DEA584")
)

var (
	StyleBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	StyleClock = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	StyleDashboard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	StyleInnerPanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	StyleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent).
				Background(ColorBgPanel)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Background(ColorBgPanel).
			Padding(0, 1)

	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight).
			Background(lipgloss.Color("#312E81"))

	StyleApiSel = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorAccent)

	// Body JSON syntax (VS Code-ish dark).
	StyleJSONKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CDCFE"))
	StyleJSONString = lipgloss.NewStyle().
			Foreground(ColorText)
	StyleJSONNumber = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B5CEA8"))
	StyleJSONKeyword = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C586C0"))
	StyleJSONPunct = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
	StyleJSONSep = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleNormal = lipgloss.NewStyle().
			Foreground(ColorText)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleHealthy = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleUnhealthy = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	StyleStopped = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	StyleRunning = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleMetric = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	StyleMetricCPU = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleMetricRAM = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	StyleMetricDisk = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	StyleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Underline(true)

	StyleTab = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorWarning)

	StyleDiffAdd = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Background(lipgloss.Color("#052E16"))

	StyleDiffRemove = lipgloss.NewStyle().
			Foreground(ColorPink).
			Background(lipgloss.Color("#4A044E"))

	StyleDiffHunk = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Background(ColorBgPanel)

	StyleDiffMeta = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleDiffNum = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleDiffMatch = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(lipgloss.Color("#854D0E")).
			Bold(true)

	StyleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	StyleIconGo      = lipgloss.NewStyle().Foreground(ColorGo).Bold(true)
	StyleIconDocker  = lipgloss.NewStyle().Foreground(ColorDocker).Bold(true)
	StyleIconVue     = lipgloss.NewStyle().Foreground(ColorVue).Bold(true)
	StyleIconLaravel = lipgloss.NewStyle().Foreground(ColorLaravel).Bold(true)
	StyleIconNode    = lipgloss.NewStyle().Foreground(ColorNode).Bold(true)
	StyleIconPHP     = lipgloss.NewStyle().Foreground(ColorPHP).Bold(true)
	StyleIconPython  = lipgloss.NewStyle().Foreground(ColorPython).Bold(true)
	StyleIconRust    = lipgloss.NewStyle().Foreground(ColorRust).Bold(true)
	StyleIconDefault = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	StyleKey = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleKeyDesc = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	StyleAccent = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Italic(true)

	StyleGitSelected = lipgloss.NewStyle().
				Foreground(ColorHighlight).
				Bold(true)

	StyleGitCherry = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true).
			Background(lipgloss.Color("#4C1D95"))

	StyleGitCherryCursor = lipgloss.NewStyle().
				Foreground(ColorText).
				Bold(true).
				Background(lipgloss.Color("#6D28D9"))

	StyleGitBranchHead = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StyleGitMarked = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	StyleGitMarkedCursor = lipgloss.NewStyle().
				Foreground(ColorText).
				Bold(true).
				Background(lipgloss.Color("#7F1D1D"))

	StyleGitColumn = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)
)

func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running", "Healthy", "running":
		return StyleRunning
	case "Stopped", "stopped", "exited":
		return StyleStopped
	case "Degraded", "Unhealthy":
		return StyleUnhealthy
	default:
		return StyleMuted
	}
}

func StatusDot(status string) string {
	switch status {
	case "Running", "Healthy", "running":
		return StyleHealthy.Render("●")
	case "Stopped", "stopped", "exited":
		return StyleUnhealthy.Render("●")
	case "Degraded", "Unhealthy":
		return StyleUnhealthy.Render("●")
	default:
		return StyleMuted.Render("◌")
	}
}
