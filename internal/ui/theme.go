package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const ThemeAuto = "auto"

type themePalette struct {
	Primary, Accent, Success, Warning, Danger, Pink string
	Muted, Text, Subtext, Border, Bg, BgPanel       string
	Highlight, SelBg                                string
	DiffAdd, DiffRemove, DiffMatch                  string
	JSONKey, JSONString, JSONNumber, JSONKeyword    string
}

type ThemeInfo struct {
	ID    string
	Label string
	Desc  string
	pal   themePalette
}

// Themes is the selectable catalog (order = modal order).
var Themes = []ThemeInfo{
	{ID: "devscope", Label: "DevScope", Desc: "roxo original do app", pal: palDevscope},
	{ID: "dark", Label: "GitHub Dark", Desc: "cinza, branco, azul limpo", pal: palDark},
	{ID: "tokyo-night", Label: "Tokyo Night", Desc: "azul escuro, roxo, ciano", pal: palTokyoNight},
	{ID: "catppuccin", Label: "Catppuccin", Desc: "mocha — lavanda e rosa", pal: palCatppuccin},
	{ID: "rose-pine", Label: "Rosé Pine", Desc: "rosa, dourado, azul profundo", pal: palRosePine},
	{ID: "solarized", Label: "Solarized", Desc: "azul petróleo, amarelo", pal: palSolarized},
	{ID: "dracula", Label: "Dracula", Desc: "clássico roxo/rosa", pal: palDracula},
	{ID: "dracula-vivid", Label: "Dracula Vivid", Desc: "dracula saturado e vivo", pal: palDraculaVivid},
	{ID: "nord", Label: "Nord", Desc: "frio ártico", pal: palNord},
	{ID: "monokai", Label: "Monokai", Desc: "editor clássico", pal: palMonokai},
	{ID: "gruvbox", Label: "Gruvbox", Desc: "quente retrô", pal: palGruvbox},
	{ID: "light", Label: "Light", Desc: "claro limpo", pal: palLight},
}

var (
	// Original DevScope purple-on-GitHub-canvas palette.
	palDevscope = themePalette{
		Primary: "#7C3AED", Accent: "#60A5FA", Success: "#22C55E", Warning: "#EAB308",
		Danger: "#EF4444", Pink: "#F472B6", Muted: "#6B7280", Text: "#E6EDF3", Subtext: "#8B949E",
		Border: "#30363D", Bg: "#0D1117", BgPanel: "#161B22", Highlight: "#A78BFA", SelBg: "#21262D",
		DiffAdd: "#052E16", DiffRemove: "#4A044E", DiffMatch: "#854D0E",
		JSONKey: "#9CDCFE", JSONString: "#CE9178", JSONNumber: "#B5CEA8", JSONKeyword: "#C586C0",
	}
	// GitHub Primer dark — canvas #0d1117, accent blue.
	palDark = themePalette{
		Primary: "#2F81F7", Accent: "#79C0FF", Success: "#3FB950", Warning: "#D29922",
		Danger: "#F85149", Pink: "#FF7B72", Muted: "#6E7681", Text: "#E6EDF3", Subtext: "#8B949E",
		Border: "#30363D", Bg: "#0D1117", BgPanel: "#161B22", Highlight: "#A5D6FF", SelBg: "#1F2937",
		DiffAdd: "#033A16", DiffRemove: "#67060C", DiffMatch: "#3D2E00",
		JSONKey: "#79C0FF", JSONString: "#A5D6FF", JSONNumber: "#3FB950", JSONKeyword: "#FF7B72",
	}
	// Tokyo Night Storm/Night hybrid — high contrast for TUI.
	palTokyoNight = themePalette{
		Primary: "#BB9AF7", Accent: "#7DCFFF", Success: "#9ECE6A", Warning: "#E0AF68",
		Danger: "#F7768F", Pink: "#FF9E64", Muted: "#565F89", Text: "#C0CAF5", Subtext: "#A9B1D6",
		Border: "#3B4261", Bg: "#1A1B26", BgPanel: "#16161E", Highlight: "#7AA2F7", SelBg: "#292E42",
		DiffAdd: "#1A2F1E", DiffRemove: "#3A1A24", DiffMatch: "#3A3020",
		JSONKey: "#7DCFFF", JSONString: "#9ECE6A", JSONNumber: "#FF9E64", JSONKeyword: "#BB9AF7",
	}
	palDracula = themePalette{
		Primary: "#BD93F9", Accent: "#8BE9FD", Success: "#50FA7B", Warning: "#F1FA8C",
		Danger: "#FF5555", Pink: "#FF79C6", Muted: "#6272A4", Text: "#F8F8F2", Subtext: "#BFBFBF",
		Border: "#44475A", Bg: "#282A36", BgPanel: "#21222C", Highlight: "#FF79C6", SelBg: "#44475A",
		DiffAdd: "#1A3A2A", DiffRemove: "#4A1F2A", DiffMatch: "#4A4520",
		JSONKey: "#8BE9FD", JSONString: "#F1FA8C", JSONNumber: "#BD93F9", JSONKeyword: "#FF79C6",
	}
	// Dracula Vivid — same family, punched saturation (does not replace classic).
	palDraculaVivid = themePalette{
		Primary: "#D4A5FF", Accent: "#5CE1FF", Success: "#4DFF8A", Warning: "#FFFF6E",
		Danger: "#FF3D5A", Pink: "#FF5EC8", Muted: "#7A72B0", Text: "#FFFFFF", Subtext: "#D0D0E8",
		Border: "#5A5278", Bg: "#1C1E26", BgPanel: "#15161D", Highlight: "#FF5EC8", SelBg: "#3A3358",
		DiffAdd: "#0F3A22", DiffRemove: "#4A1528", DiffMatch: "#4A4010",
		JSONKey: "#5CE1FF", JSONString: "#FFFF6E", JSONNumber: "#D4A5FF", JSONKeyword: "#FF5EC8",
	}
	palNord = themePalette{
		Primary: "#88C0D0", Accent: "#81A1C1", Success: "#A3BE8C", Warning: "#EBCB8B",
		Danger: "#BF616A", Pink: "#B48EAD", Muted: "#4C566A", Text: "#ECEFF4", Subtext: "#D8DEE9",
		Border: "#3B4252", Bg: "#2E3440", BgPanel: "#3B4252", Highlight: "#88C0D0", SelBg: "#434C5E",
		DiffAdd: "#2A3B2E", DiffRemove: "#3B2A2E", DiffMatch: "#3B3828",
		JSONKey: "#88C0D0", JSONString: "#A3BE8C", JSONNumber: "#B48EAD", JSONKeyword: "#81A1C1",
	}
	palMonokai = themePalette{
		Primary: "#F92672", Accent: "#66D9EF", Success: "#A6E22E", Warning: "#E6DB74",
		Danger: "#F92672", Pink: "#FD5FF0", Muted: "#75715E", Text: "#F8F8F2", Subtext: "#CFCFC2",
		Border: "#3E3D32", Bg: "#272822", BgPanel: "#1E1F1C", Highlight: "#FD971F", SelBg: "#3E3D32",
		DiffAdd: "#1E2E14", DiffRemove: "#3A1520", DiffMatch: "#3A3014",
		JSONKey: "#66D9EF", JSONString: "#E6DB74", JSONNumber: "#AE81FF", JSONKeyword: "#F92672",
	}
	// Catppuccin Mocha — pastel, soft contrast.
	palCatppuccin = themePalette{
		Primary: "#CBA6F7", Accent: "#89B4FA", Success: "#A6E3A1", Warning: "#F9E2AF",
		Danger: "#F38BA8", Pink: "#F5C2E7", Muted: "#6C7086", Text: "#CDD6F4", Subtext: "#A6ADC8",
		Border: "#45475A", Bg: "#1E1E2E", BgPanel: "#181825", Highlight: "#B4BEFE", SelBg: "#313244",
		DiffAdd: "#1E2F23", DiffRemove: "#3A1F28", DiffMatch: "#3A3420",
		JSONKey: "#89B4FA", JSONString: "#A6E3A1", JSONNumber: "#FAB387", JSONKeyword: "#CBA6F7",
	}
	// Rosé Pine — elegant rose / gold / pine.
	palRosePine = themePalette{
		Primary: "#C4A7E7", Accent: "#9CCFD8", Success: "#31748F", Warning: "#F6C177",
		Danger: "#EB6F92", Pink: "#EBBCBA", Muted: "#6E6A86", Text: "#E0DEF4", Subtext: "#908CAA",
		Border: "#403D52", Bg: "#191724", BgPanel: "#1F1D2E", Highlight: "#EBBCBA", SelBg: "#26233A",
		DiffAdd: "#1A2830", DiffRemove: "#3A1E28", DiffMatch: "#3A3020",
		JSONKey: "#9CCFD8", JSONString: "#F6C177", JSONNumber: "#EBBCBA", JSONKeyword: "#C4A7E7",
	}
	palGruvbox = themePalette{
		Primary: "#FE8019", Accent: "#83A598", Success: "#B8BB26", Warning: "#FABD2F",
		Danger: "#FB4934", Pink: "#D3869B", Muted: "#928374", Text: "#EBDBB2", Subtext: "#D5C4A1",
		Border: "#504945", Bg: "#282828", BgPanel: "#3C3836", Highlight: "#FE8019", SelBg: "#504945",
		DiffAdd: "#2A3318", DiffRemove: "#3A1C1C", DiffMatch: "#3A3018",
		JSONKey: "#83A598", JSONString: "#B8BB26", JSONNumber: "#D3869B", JSONKeyword: "#FE8019",
	}
	// Solarized Dark — teal base, yellow accents; text bumped for TUI readability.
	palSolarized = themePalette{
		Primary: "#268BD2", Accent: "#2AA198", Success: "#859900", Warning: "#B58900",
		Danger: "#DC322F", Pink: "#D33682", Muted: "#586E75", Text: "#93A1A1", Subtext: "#839496",
		Border: "#0A3945", Bg: "#002B36", BgPanel: "#073642", Highlight: "#CB4B16", SelBg: "#094352",
		DiffAdd: "#0A2E1A", DiffRemove: "#3A1010", DiffMatch: "#3A2E0A",
		JSONKey: "#268BD2", JSONString: "#2AA198", JSONNumber: "#6C71C4", JSONKeyword: "#CB4B16",
	}
	palLight = themePalette{
		Primary: "#6D28D9", Accent: "#2563EB", Success: "#16A34A", Warning: "#CA8A04",
		Danger: "#DC2626", Pink: "#DB2777", Muted: "#6B7280", Text: "#1F2328", Subtext: "#4B5563",
		Border: "#D0D7DE", Bg: "#FFFFFF", BgPanel: "#F6F8FA", Highlight: "#5B21B6", SelBg: "#EDE9FE",
		DiffAdd: "#DCFCE7", DiffRemove: "#FCE7F3", DiffMatch: "#FEF3C7",
		JSONKey: "#0451A5", JSONString: "#0A3069", JSONNumber: "#098658", JSONKeyword: "#AF00DB",
	}
)

var (
	currentTheme   = "dark"
	terminalThemed bool

	colorDiffAddBg    = lipgloss.Color("#052E16")
	colorDiffRemoveBg = lipgloss.Color("#4A044E")
	colorDiffMatchBg  = lipgloss.Color("#854D0E")
)

func InitTheme(theme string) { ApplyTheme(theme) }

func CurrentTheme() string { return currentTheme }

func ThemeIndex(id string) int {
	id = normalizeTheme(id)
	for i, t := range Themes {
		if t.ID == id {
			return i
		}
	}
	return 0
}

func ApplyTheme(theme string) {
	name := normalizeTheme(theme)
	currentTheme = name
	for _, t := range Themes {
		if t.ID == name {
			t.pal.apply()
			applyTerminalChrome()
			return
		}
	}
	palDark.apply()
	applyTerminalChrome()
}

func normalizeTheme(theme string) string {
	theme = strings.ToLower(strings.TrimSpace(theme))
	switch theme {
	case "github", "github-dark", "gh-dark":
		theme = "dark"
	case "devscope-dark", "original", "legacy":
		theme = "devscope"
	case "tokyonight", "tokyo", "tokyo_night":
		theme = "tokyo-night"
	case "mocha", "catppuccin-mocha":
		theme = "catppuccin"
	case "rosepine", "rose", "rose_pine":
		theme = "rose-pine"
	case "solarized-dark":
		theme = "solarized"
	case "dracula-pro", "dracula-sat", "dracula_vivid":
		theme = "dracula-vivid"
	}
	if theme == ThemeAuto || theme == "" {
		if os.Getenv("COLORFGBG") != "" {
			parts := strings.Split(os.Getenv("COLORFGBG"), ";")
			if len(parts) > 0 && (parts[0] == "15" || parts[0] == "7") {
				return "light"
			}
		}
		return "dark"
	}
	for _, t := range Themes {
		if t.ID == theme {
			return theme
		}
	}
	return "dark"
}

func (p themePalette) apply() {
	ColorPrimary = lipgloss.Color(p.Primary)
	ColorAccent = lipgloss.Color(p.Accent)
	ColorSuccess = lipgloss.Color(p.Success)
	ColorWarning = lipgloss.Color(p.Warning)
	ColorDanger = lipgloss.Color(p.Danger)
	ColorPink = lipgloss.Color(p.Pink)
	ColorMuted = lipgloss.Color(p.Muted)
	ColorText = lipgloss.Color(p.Text)
	ColorSubtext = lipgloss.Color(p.Subtext)
	ColorBorder = lipgloss.Color(p.Border)
	ColorBg = lipgloss.Color(p.Bg)
	ColorBgPanel = lipgloss.Color(p.BgPanel)
	ColorHighlight = lipgloss.Color(p.Highlight)
	ColorSelBg = lipgloss.Color(p.SelBg)
	colorDiffAddBg = lipgloss.Color(p.DiffAdd)
	colorDiffRemoveBg = lipgloss.Color(p.DiffRemove)
	colorDiffMatchBg = lipgloss.Color(p.DiffMatch)
	ColorJSONKey = lipgloss.Color(p.JSONKey)
	ColorJSONString = lipgloss.Color(p.JSONString)
	ColorJSONNumber = lipgloss.Color(p.JSONNumber)
	ColorJSONKeyword = lipgloss.Color(p.JSONKeyword)
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
	StyleSelected = lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Background(ColorSelBg)
	StyleApiSel = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent)
	StyleJSONKey = lipgloss.NewStyle().Foreground(ColorJSONKey)
	StyleJSONString = lipgloss.NewStyle().Foreground(ColorJSONString)
	StyleJSONNumber = lipgloss.NewStyle().Foreground(ColorJSONNumber)
	StyleJSONKeyword = lipgloss.NewStyle().Foreground(ColorJSONKeyword)
	StyleJSONPunct = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleJSONSep = lipgloss.NewStyle().Foreground(ColorMuted)
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
	StyleGitSelected = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	StyleGitCherry = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Background(ColorSelBg)
	StyleGitCherryCursor = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Background(ColorPrimary)
	StyleGitBranchHead = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleGitMarked = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	StyleGitMarkedCursor = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Background(ColorDanger)
	StyleGitColumn = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder).Padding(0, 1)
}

func applyTerminalChrome() {
	bg := hexRGB(string(ColorBg))
	fg := hexRGB(string(ColorText))
	if bg == nil || fg == nil {
		return
	}
	fmt.Fprintf(os.Stdout, "\x1b]11;rgb:%02x/%02x/%02x\x07", bg[0], bg[1], bg[2])
	fmt.Fprintf(os.Stdout, "\x1b]10;rgb:%02x/%02x/%02x\x07", fg[0], fg[1], fg[2])
	terminalThemed = true
}

func RestoreTerminalTheme() {
	if !terminalThemed {
		return
	}
	fmt.Fprint(os.Stdout, "\x1b]111\x07\x1b]110\x07")
	terminalThemed = false
}

func hexRGB(h string) []uint8 {
	h = strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(h) != 6 {
		return nil
	}
	var n uint32
	if _, err := fmt.Sscanf(h, "%06x", &n); err != nil {
		return nil
	}
	return []uint8{uint8(n >> 16), uint8(n >> 8), uint8(n)}
}

func paintAppFrame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return content
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Left, lipgloss.Top,
		cropBlock(content, width, height),
		lipgloss.WithWhitespaceBackground(ColorBg),
		lipgloss.WithWhitespaceForeground(ColorText),
	)
}

// cropBlock crops a rendered view to the terminal box. lipgloss.Place only pads,
// so an oversized layout would wrap and scroll the top of the UI off screen.
func cropBlock(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		if lipgloss.Width(l) > width {
			lines[i] = ansi.Truncate(l, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func swatch(hex string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("██")
}
