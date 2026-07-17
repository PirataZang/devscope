package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func InitTheme(theme string) {
	if theme == "" {
		theme = "auto"
	}
	if theme == "auto" {
		if os.Getenv("COLORFGBG") != "" {
			parts := strings.Split(os.Getenv("COLORFGBG"), ";")
			if len(parts) > 0 && parts[0] == "15" {
				applyLightTheme()
				return
			}
		}
		applyDarkTheme()
		return
	}
	if theme == "light" {
		applyLightTheme()
		return
	}
	applyDarkTheme()
}

var (
	colorDiffAddBg    = lipgloss.Color("#052E16")
	colorDiffRemoveBg = lipgloss.Color("#4A044E")
	colorDiffMatchBg  = lipgloss.Color("#854D0E")
)

func applyDarkTheme() {
	ColorPrimary = lipgloss.Color("#7C3AED")
	ColorAccent = lipgloss.Color("#60A5FA")
	ColorSuccess = lipgloss.Color("#22C55E")
	ColorWarning = lipgloss.Color("#EAB308")
	ColorDanger = lipgloss.Color("#EF4444")
	ColorPink = lipgloss.Color("#F472B6")
	ColorMuted = lipgloss.Color("#6B7280")
	ColorText = lipgloss.Color("#F9FAFB")
	ColorSubtext = lipgloss.Color("#9CA3AF")
	ColorBorder = lipgloss.Color("#374151")
	ColorBg = lipgloss.Color("#111827")
	ColorBgPanel = lipgloss.Color("#1F2937")
	ColorHighlight = lipgloss.Color("#A78BFA")
	colorDiffAddBg = lipgloss.Color("#052E16")
	colorDiffRemoveBg = lipgloss.Color("#4A044E")
	colorDiffMatchBg = lipgloss.Color("#854D0E")
	rebuildStyles()
}

func applyLightTheme() {
	ColorPrimary = lipgloss.Color("#6D28D9")
	ColorAccent = lipgloss.Color("#2563EB")
	ColorSuccess = lipgloss.Color("#16A34A")
	ColorWarning = lipgloss.Color("#CA8A04")
	ColorDanger = lipgloss.Color("#DC2626")
	ColorPink = lipgloss.Color("#DB2777")
	ColorMuted = lipgloss.Color("#6B7280")
	ColorText = lipgloss.Color("#111827")
	ColorSubtext = lipgloss.Color("#4B5563")
	ColorBorder = lipgloss.Color("#D1D5DB")
	ColorBg = lipgloss.Color("#F9FAFB")
	ColorBgPanel = lipgloss.Color("#F3F4F6")
	ColorHighlight = lipgloss.Color("#5B21B6")
	colorDiffAddBg = lipgloss.Color("#DCFCE7")
	colorDiffRemoveBg = lipgloss.Color("#FCE7F3")
	colorDiffMatchBg = lipgloss.Color("#FEF3C7")
	rebuildStyles()
}

func rebuildStyles() {
	StyleBrand = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	StyleSubtitle = lipgloss.NewStyle().Foreground(ColorSubtext)
	StyleClock = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	StyleDashboard = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	StyleInnerPanel = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	StyleTableHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Background(ColorBgPanel)
	StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorText).Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	StyleStatusBar = lipgloss.NewStyle().Foreground(ColorSubtext).Background(ColorBgPanel).Padding(0, 1)
	StyleSelected = lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Background(lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#312E81"})
	StyleNormal = lipgloss.NewStyle().Foreground(ColorText)
	StyleMuted = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleHealthy = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleUnhealthy = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	StyleStopped = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	StyleRunning = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleMetric = lipgloss.NewStyle().Foreground(ColorSubtext)
	StyleMetricCPU = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleMetricRAM = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleMetricDisk = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	StylePanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	StyleTabActive = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Underline(true)
	StyleTab = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleDiffAdd = lipgloss.NewStyle().Foreground(ColorSuccess).Background(colorDiffAddBg)
	StyleDiffRemove = lipgloss.NewStyle().Foreground(ColorPink).Background(colorDiffRemoveBg)
	StyleDiffHunk = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Background(ColorBgPanel)
	StyleDiffMeta = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleDiffNum = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleDiffMatch = lipgloss.NewStyle().Foreground(ColorText).Background(colorDiffMatchBg).Bold(true)
	StyleSection = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	StyleKey = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	StyleKeyDesc = lipgloss.NewStyle().Foreground(ColorSubtext)
	StyleAccent = lipgloss.NewStyle().Foreground(ColorAccent).Italic(true)
}
