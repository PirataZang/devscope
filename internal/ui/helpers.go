package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/mattn/go-runewidth"
)

// contentPanelHeight returns the fixed height (in lines) available for the
// scrollable content area inside any detail panel (Git, Container, etc.).
// It subtracts the constant chrome: header (2 lines) + project name/status (3)
// + tabs (1) + blank gaps (4) + status bar (1) + StylePanel border+padding (4).
// The result is capped so that the UI looks consistent on both small (24-line)
// and large terminals.
func (a *App) contentPanelHeight() int {
	if a.height <= 0 {
		return 18
	}
	h := a.height - 15 // ~15 lines of fixed chrome outside the panel
	if h < 10 {
		return 10
	}
	if h > 18 {
		return 18 // prevent absurdly tall panels on huge monitors, keep it compact and clean
	}
	return h
}

func padRight(s string, width int) string {
	n := runewidth.StringWidth(s)
	if n >= width {
		return runewidth.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", width-n)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= max {
		return s
	}
	return runewidth.Truncate(s, max, "…")
}

// sliceColumns returns a horizontal window of s starting at column `start`.
func sliceColumns(s string, start, width int) string {
	if width <= 0 {
		return ""
	}
	if start < 0 {
		start = 0
	}
	if start == 0 {
		return padRight(truncate(s, width), width)
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if col+w <= start {
			col += w
			continue
		}
		if col < start {
			// rune straddles the start edge; skip it
			col += w
			continue
		}
		if runewidth.StringWidth(b.String())+w > width {
			break
		}
		b.WriteRune(r)
		col += w
	}
	return padRight(b.String(), width)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ensureVisible(cursor, scroll, viewport, total int) int {
	if viewport <= 0 || total <= 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	if cursor < scroll {
		return cursor
	}
	if cursor >= scroll+viewport {
		return cursor - viewport + 1
	}
	return scroll
}

func wrapText(text string, width int) []string {
	if width < 10 {
		width = 10
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{"(sem mensagem)"}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		line := ""
		for _, word := range words {
			if line == "" {
				line = word
				continue
			}
			if runewidth.StringWidth(line+" "+word) <= width {
				line += " " + word
			} else {
				lines = append(lines, line)
				line = word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func clampCursor(cursor, length int) int {
	if length == 0 {
		return 0
	}
	if cursor >= length {
		return length - 1
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}

func sortProjects(projects []core.Project) []core.Project {
	out := make([]core.Project, len(projects))
	copy(out, projects)
	sort.Slice(out, func(i, j int) bool {
		si, sj := statusRank(out[i].Status), statusRank(out[j].Status)
		if si != sj {
			return si < sj
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func statusRank(s core.ProjectStatus) int {
	switch s {
	case core.StatusRunning:
		return 0
	case core.StatusDegraded:
		return 1
	case core.StatusStopped:
		return 2
	default:
		return 3
	}
}

func gitStatusLabel(staging, worktree string) string {
	if staging == "?" || worktree == "?" {
		return "??"
	}
	if worktree != " " {
		return worktree
	}
	return staging
}

func gitFileStaged(f core.GitFileStatus) bool {
	// Index column from `git status --porcelain` (not worktree-only changes).
	switch f.Staging {
	case "M", "A", "D", "R", "C", "U", "T":
		return true
	default:
		return false
	}
}

func gitStatusStyle(code string) string {
	switch code {
	case "M":
		return StyleRunning.Render(code)
	case "A":
		return StyleHealthy.Render(code)
	case "D":
		return StyleUnhealthy.Render(code)
	case "??":
		return StyleWarning.Render(code)
	default:
		return StyleMuted.Render(code)
	}
}

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func formatUptime(d time.Duration) string {
	if d <= 0 {
		return "unknown"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func frameworkIcon(name string) string {
	switch strings.ToLower(name) {
	case "go":
		return StyleIconGo.Render("◆")
	case "docker":
		return StyleIconDocker.Render("🐳")
	case "vue":
		return StyleIconVue.Render("V")
	case "laravel":
		return StyleIconLaravel.Render("L")
	case "node", "nestjs", "next.js", "react", "nuxt.js":
		return StyleIconNode.Render("⬡")
	case "php":
		return StyleIconPHP.Render("P")
	case "python", "django":
		return StyleIconPython.Render("Py")
	case "rust":
		return StyleIconRust.Render("R")
	default:
		return StyleIconDefault.Render("•")
	}
}

func renderMetricPills(m core.HostMetrics) string {
	return strings.Join([]string{
		metricUsageStyle(m.CPUPercent).Render(fmt.Sprintf("CPU %.0f%%", m.CPUPercent)),
		metricUsageStyle(m.MemoryPercent).Render(fmt.Sprintf("RAM %.0f%%", m.MemoryPercent)),
		metricUsageStyle(m.DiskPercent).Render(fmt.Sprintf("DISK %.0f%%", m.DiskPercent)),
	}, "  ")
}

// metricUsageStyle: verde ≤30%, laranja 31–70%, vermelho >70%.
func metricUsageStyle(pct float64) lipgloss.Style {
	switch {
	case pct > 70:
		return lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	case pct >= 31:
		return lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	}
}

func renderKeybind(keys, desc string) string {
	return StyleKey.Render(keys) + " " + StyleKeyDesc.Render(desc)
}

func frameworkIconPlain(name string) string {
	switch strings.ToLower(name) {
	case "go":
		return "◆"
	case "docker":
		return "◆"
	case "vue":
		return "V"
	case "laravel":
		return "L"
	case "node", "nestjs", "next.js", "react", "nuxt.js":
		return "⬡"
	case "php":
		return "P"
	case "python", "django":
		return "Y"
	case "rust":
		return "R"
	default:
		return "•"
	}
}

// tunnelCmdWidth keeps the tunnel-table call sites; same as actionsCmdWidth.
func tunnelCmdWidth(total int) int { return actionsCmdWidth(total) }

// tunnelTableCols spreads tunnel columns across the available table width.
type tunnelCols struct {
	name, project, port, mode, host, uptime, pid int
}

func tunnelTableCols(width int) tunnelCols {
	inner := maxInt(40, width-6) // borders + status prefix
	// Host/URL gets most of the stretch — trycloudflare hostnames are long.
	c := tunnelCols{name: 14, project: 14, port: 5, mode: 6, host: 36, uptime: 8, pid: 6}
	used := 3 + c.name + c.project + c.port + c.mode + c.host + c.uptime + c.pid + 7 // spaces
	extra := inner - used
	if extra > 0 {
		hostExtra := extra * 70 / 100
		projExtra := extra * 15 / 100
		nameExtra := extra - hostExtra - projExtra
		c.host += hostExtra
		c.project += projExtra
		c.name += nameExtra
	} else if extra < 0 {
		need := -extra
		take := minInt(need, maxInt(0, c.host-24))
		c.host -= take
		need -= take
		take = minInt(need, maxInt(0, c.project-10))
		c.project -= take
		need -= take
		take = minInt(need, maxInt(0, c.name-10))
		c.name -= take
	}
	return c
}
