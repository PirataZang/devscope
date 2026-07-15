package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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
	if h > 28 {
		return 28 // prevent absurdly tall panels on huge monitors
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
	if runewidth.StringWidth(s) <= max {
		return s
	}
	return runewidth.Truncate(s, max, "…")
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
	cpuStyle := StyleMetricCPU
	if m.CPUPercent > 80 {
		cpuStyle = StyleUnhealthy
	} else if m.CPUPercent > 50 {
		cpuStyle = StyleMetricRAM
	}
	ramStyle := StyleMetricRAM
	if m.MemoryPercent > 90 {
		ramStyle = StyleUnhealthy
	}
	return strings.Join([]string{
		cpuStyle.Render(fmt.Sprintf("CPU %.0f%%", m.CPUPercent)),
		ramStyle.Render(fmt.Sprintf("RAM %.0f%%", m.MemoryPercent)),
		StyleMetricDisk.Render(fmt.Sprintf("DISK %.0f%%", m.DiskPercent)),
	}, "  ")
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
