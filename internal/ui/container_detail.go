package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type containerDetailTab int

const (
	containerDetailTabLogs containerDetailTab = iota
	containerDetailTabStats
	containerDetailTabEnv
	containerDetailTabConfig
	containerDetailTabTop
	containerDetailTabCompose
	containerDetailTabFile
)

const containerDetailTabTotal = int(containerDetailTabFile) + 1

type containerDetailFollowMsg struct {
	id   string
	gen  int
	logs string
}

func (t containerDetailTab) label() string {
	switch t {
	case containerDetailTabLogs:
		return "Registros"
	case containerDetailTabStats:
		return "Estatísticas"
	case containerDetailTabEnv:
		return "Env"
	case containerDetailTabConfig:
		return "Configuração"
	case containerDetailTabTop:
		return "Topo"
	case containerDetailTabCompose:
		return "Compose"
	case containerDetailTabFile:
		return "File"
	default:
		return "?"
	}
}

func (t containerDetailTab) shortLabel() string {
	switch t {
	case containerDetailTabLogs:
		return "Logs"
	case containerDetailTabStats:
		return "Stats"
	case containerDetailTabEnv:
		return "Env"
	case containerDetailTabConfig:
		return "Config"
	case containerDetailTabTop:
		return "Top"
	case containerDetailTabCompose:
		return "Compose"
	case containerDetailTabFile:
		return "File"
	default:
		return "?"
	}
}

func (a *App) renderContainerDetail(p *core.Project) string {
	if a.containerDetailTab == containerDetailTabStats {
		return a.renderContainerStatsScreen()
	}
	return a.renderContainerTextScreen()
}

func (a *App) renderContainerTextScreen() string {
	height := maxInt(12, a.height-2)
	panelW := maxInt(40, a.width)
	innerW := maxInt(36, panelW-2)
	bodyH := maxInt(8, height-6)
	body := a.renderContainerDetailRichBody(innerW, bodyH)
	return a.renderContainerDetailChrome(body, bodyH)
}

func (a *App) containerDetailStatusBadge() string {
	parts := []string{
		StyleMuted.Render(fmt.Sprintf("[%d/%d]", int(a.containerDetailTab)+1, containerDetailTabTotal)),
	}
	if a.containerDetailTab == containerDetailTabLogs {
		switch {
		case a.containerDetailFollow && a.containerDetailFollowPaused:
			parts = append(parts, StyleWarning.Render("PAUSED"))
		case a.containerDetailFollow:
			parts = append(parts, StyleHealthy.Render("LIVE"))
		}
	}
	if a.containerDetailSearchQuery != "" {
		matches := a.containerDetailSearchMatches()
		if len(matches) == 0 {
			parts = append(parts, StyleMuted.Render("/0"))
		} else {
			parts = append(parts, StyleAccent.Render(fmt.Sprintf("/%d/%d", a.containerDetailSearchIdx+1, len(matches))))
		}
	}
	if a.containerDetailHScroll > 0 {
		parts = append(parts, StyleMuted.Render(fmt.Sprintf("↔%d", a.containerDetailHScroll)))
	}
	return strings.Join(parts, "  ")
}

func (a *App) containerDetailFooter(position string) string {
	base := "←→ abas  ↑↓/pg scroll  ,/. lateral  / buscar"
	if a.containerDetailTab == containerDetailTabLogs {
		base += "  f follow  p pausa  r reload"
	}
	if a.containerDetailSearchQuery != "" {
		base += "  N/P match"
	}
	return base + "  " + position + "  esc"
}

func (a *App) renderContainerDetailBodyLines(viewport, width int) []string {
	lines := make([]string, 0, viewport)
	if a.containerDetailLoading {
		lines = append(lines, StyleMuted.Render("  Carregando "+strings.ToLower(a.containerDetailTab.shortLabel())+"..."))
		return fitExactLines(lines, viewport)
	}

	all := a.containerDetailLines()
	textW := maxInt(8, width-2)
	a.containerDetailHScroll = clampScroll(a.containerDetailHScroll, textW, a.containerDetailMaxLineWidth())
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(all))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(all))

	matchSet := a.containerDetailMatchLineSet()
	current := -1
	if matches := a.containerDetailSearchMatches(); len(matches) > 0 && a.containerDetailSearchIdx < len(matches) {
		current = matches[a.containerDetailSearchIdx]
	}

	for i := start; i < end; i++ {
		lines = append(lines, a.renderContainerDetailViewLine(all[i], textW, matchSet[i], i == current))
	}
	return fitExactLines(lines, viewport)
}

func (a *App) renderContainerDetailViewLine(line string, width int, matched, current bool) string {
	line = sanitizeTerminalLine(line)
	if line == "" {
		line = " "
	}
	display := sliceColumns(line, a.containerDetailHScroll, width)

	if a.containerDetailTab == containerDetailTabEnv {
		if key, val, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			visible := sliceColumns(key+"="+val, a.containerDetailHScroll, width)
			if current {
				return StyleDiffMatch.Render(visible)
			}
			if matched {
				return StyleWarning.Render(visible)
			}
			// Re-color key=value within the visible window when possible.
			if eq := strings.IndexByte(visible, '='); eq > 0 && a.containerDetailHScroll == 0 {
				return StyleWarning.Render(visible[:eq]) + StyleNormal.Render(visible[eq:])
			}
			return StyleNormal.Render(visible)
		}
	}

	switch {
	case current:
		return StyleDiffMatch.Render(display)
	case matched:
		return StyleWarning.Render(display)
	default:
		return StyleNormal.Render(display)
	}
}

func (a *App) containerDetailPosition(viewport int) string {
	if a.containerDetailLoading {
		return ""
	}
	n := a.containerDetailContentLen()
	if n == 0 {
		return "0/0"
	}
	start := a.containerDetailScroll
	end := minInt(start+viewport, n)
	return fmt.Sprintf("%d-%d/%d", start+1, end, n)
}

func (a *App) containerDetailContentLen() int {
	switch a.containerDetailTab {
	case containerDetailTabEnv:
		return len(flattenEnvGroups(groupEnvPairs(parseEnvPairs(a.containerDetailContent))))
	case containerDetailTabConfig:
		return maxInt(1, len(parseContainerConfig(a.containerDetailContent).labels))
	case containerDetailTabCompose, containerDetailTabFile:
		raw := a.containerDetailContent
		if strings.HasPrefix(strings.TrimSpace(raw), "#") {
			parts := strings.SplitN(raw, "\n", 2)
			if len(parts) == 2 {
				raw = parts[1]
			}
		}
		return len(strings.Split(raw, "\n"))
	default:
		return len(a.containerDetailLines())
	}
}

func (a *App) renderContainerDetailTabBar(width int) string {
	if width <= 0 {
		width = maxInt(20, a.width-4)
	}
	separator := " │ "
	activePrefix := "▶ "
	if width < 70 {
		separator = " "
		activePrefix = "›"
	}

	labels := make([]string, containerDetailTabTotal)
	for i := 0; i < containerDetailTabTotal; i++ {
		tab := containerDetailTab(i)
		label := tab.shortLabel()
		if tab == a.containerDetailTab {
			labels[i] = StyleTabActive.Render(activePrefix + label)
		} else {
			labels[i] = StyleMuted.Render(label)
		}
	}

	active := int(a.containerDetailTab)
	// Find the leftmost start index so the active tab stays fully visible.
	start := 0
	for start < active {
		if containerTabsWidth(labels[start:], separator) <= width {
			break
		}
		prefix := 0
		if start > 0 {
			prefix = lipgloss.Width(StyleMuted.Render("…" + separator))
		}
		if prefix+containerTabsWidth(labels[start:active+1], separator) <= width {
			break
		}
		start++
	}

	var parts []string
	used := 0
	if start > 0 {
		ell := StyleMuted.Render("…")
		parts = append(parts, ell)
		used = lipgloss.Width(ell)
	}
	for i := start; i < len(labels); i++ {
		need := lipgloss.Width(labels[i])
		if len(parts) > 0 {
			need += lipgloss.Width(separator)
		}
		if used+need > width {
			if i == active {
				parts = []string{labels[i]}
				used = lipgloss.Width(labels[i])
				continue
			}
			if i > active {
				parts = append(parts, StyleMuted.Render("…"))
			}
			break
		}
		parts = append(parts, labels[i])
		used += need
	}
	return strings.Join(parts, separator)
}

func containerTabsWidth(labels []string, sep string) int {
	if len(labels) == 0 {
		return 0
	}
	w := 0
	for i, label := range labels {
		w += lipgloss.Width(label)
		if i > 0 {
			w += lipgloss.Width(sep)
		}
	}
	return w
}

func (a *App) renderContainerDetailLine(tab containerDetailTab, line string) string {
	lineWidth := maxInt(10, a.width-12)
	if tab == containerDetailTabLogs {
		line = sanitizeTerminalLine(line)
	}
	line = truncate(line, lineWidth)
	text := "  " + line
	if tab == containerDetailTabEnv {
		if key, val, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			return "  " + StyleWarning.Render(key) + "=" + StyleNormal.Render(val)
		}
	}
	return StyleNormal.Render(text)
}

func sanitizeTerminalLine(line string) string {
	line = strings.ReplaceAll(line, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, ansi.Strip(line))
}

func (a *App) containerDetailLines() []string {
	content := a.containerDetailContent
	if content == "" {
		return []string{"(vazio)"}
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{"(vazio)"}
	}
	return lines
}

func (a *App) containerDetailMaxLineWidth() int {
	maxW := 0
	for _, line := range a.containerDetailLines() {
		if w := lipgloss.Width(sanitizeTerminalLine(line)); w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (a *App) containerDetailViewport() int {
	// Match rich views: bodyH = height-6, cards ~3, box chrome ~2
	h := maxInt(12, a.height-2)
	bodyH := maxInt(8, h-6)
	restH := maxInt(6, bodyH-3)
	return maxInt(1, restH-2)
}

func (a *App) containerDetailSwitchTab(delta int) tea.Cmd {
	a.stopContainerDetailFollow()
	a.stopContainerDetailStatsLive()
	a.containerDetailSearchQuery = ""
	a.containerDetailSearchIdx = 0
	a.containerDetailHScroll = 0
	n := int(a.containerDetailTab) + delta
	for n < 0 {
		n += containerDetailTabTotal
	}
	a.containerDetailTab = containerDetailTab(n % containerDetailTabTotal)
	a.containerDetailScroll = 0
	return a.loadContainerDetailTab()
}

func (a *App) containerDetailScrollBy(delta int) {
	viewport := a.containerDetailViewport()
	a.containerDetailScroll = clampScroll(a.containerDetailScroll+delta, viewport, a.containerDetailContentLen())
}

func (a *App) containerDetailHScrollBy(delta int) {
	textW := maxInt(8, a.width-6)
	maxH := maxInt(0, a.containerDetailMaxLineWidth()-textW)
	a.containerDetailHScroll += delta
	if a.containerDetailHScroll < 0 {
		a.containerDetailHScroll = 0
	}
	if a.containerDetailHScroll > maxH {
		a.containerDetailHScroll = maxH
	}
}

func (a *App) isContainerDetailAtEnd() bool {
	viewport := a.containerDetailViewport()
	maxScroll := maxInt(0, a.containerDetailContentLen()-viewport)
	return a.containerDetailScroll >= maxScroll
}

func clampScroll(scroll, viewport, total int) int {
	maxScroll := total - viewport
	if maxScroll < 0 {
		return 0
	}
	if scroll < 0 {
		return 0
	}
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

func (a *App) containerDetailSearchMatches() []int {
	q := strings.ToLower(strings.TrimSpace(a.containerDetailSearchQuery))
	if q == "" {
		return nil
	}
	var matches []int
	for i, line := range a.containerDetailLines() {
		if strings.Contains(strings.ToLower(sanitizeTerminalLine(line)), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (a *App) containerDetailMatchLineSet() map[int]bool {
	matches := a.containerDetailSearchMatches()
	if len(matches) == 0 {
		return nil
	}
	set := make(map[int]bool, len(matches))
	for _, i := range matches {
		set[i] = true
	}
	return set
}

func (a *App) jumpContainerDetailSearch(delta int) {
	matches := a.containerDetailSearchMatches()
	if len(matches) == 0 {
		return
	}
	a.containerDetailSearchIdx = (a.containerDetailSearchIdx + delta) % len(matches)
	if a.containerDetailSearchIdx < 0 {
		a.containerDetailSearchIdx += len(matches)
	}
	a.containerDetailScroll = ensureVisible(
		matches[a.containerDetailSearchIdx],
		a.containerDetailScroll,
		a.containerDetailViewport(),
		len(a.containerDetailLines()),
	)
}

func (a *App) applyContainerDetailSearch() {
	a.containerDetailSearchQuery = strings.TrimSpace(a.containerDetailSearchInput)
	a.containerDetailSearchIdx = 0
	a.containerDetailSearchOn = false
	if a.containerDetailSearchQuery == "" {
		return
	}
	a.jumpContainerDetailSearch(0)
}

func (a *App) updateContainerDetailSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.containerDetailSearchOn = false
		a.containerDetailSearchInput = a.containerDetailSearchQuery
		return a, nil
	case tea.KeyEnter:
		a.applyContainerDetailSearch()
		return a, nil
	case tea.KeyBackspace:
		if a.containerDetailSearchInput != "" {
			r := []rune(a.containerDetailSearchInput)
			a.containerDetailSearchInput = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		a.containerDetailSearchInput += string(msg.Runes)
	}
	return a, nil
}

func (a *App) renderContainerDetailSearchPrompt() string {
	content := a.renderContainerTextScreen()
	prompt := StylePanel.Render("Buscar: " + a.containerDetailSearchInput + "█")
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		prompt,
		a.renderStatusBar("digite o termo | enter buscar | esc cancelar"),
	)
}

func (a *App) stopContainerDetailFollow() {
	a.containerDetailFollow = false
	a.containerDetailFollowPaused = false
	a.containerDetailFollowGen++
}

func (a *App) toggleContainerDetailFollow() tea.Cmd {
	if a.containerDetailTab != containerDetailTabLogs || a.containerDetailID == "" {
		return nil
	}
	if a.containerDetailFollow {
		a.stopContainerDetailFollow()
		return nil
	}
	a.containerDetailFollow = true
	a.containerDetailFollowPaused = false
	a.containerDetailFollowGen++
	return a.scheduleContainerDetailFollow()
}

func (a *App) scheduleContainerDetailFollow() tea.Cmd {
	if !a.containerDetailFollow || a.containerDetailFollowPaused || a.containerDetailID == "" {
		return nil
	}
	id := a.containerDetailID
	gen := a.containerDetailFollowGen
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		logs, _ := collectors.DockerLogsSince(id, 2, 80)
		return containerDetailFollowMsg{id: id, gen: gen, logs: logs}
	})
}

func appendContainerLogs(existing, chunk string) string {
	chunk = strings.TrimRight(chunk, "\n")
	if chunk == "" {
		return existing
	}
	trimmed := strings.TrimRight(existing, "\n")
	if trimmed == "" {
		return chunk + "\n"
	}
	if strings.HasSuffix(trimmed, chunk) {
		return existing
	}
	// Avoid duplicating overlapping tails from --since polling.
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasSuffix(trimmed, line) {
			continue
		}
		trimmed += "\n" + line
	}
	return trimmed + "\n"
}

func (a *App) handleContainerDetailFollow(msg containerDetailFollowMsg) tea.Cmd {
	if msg.id != a.containerDetailID || msg.gen != a.containerDetailFollowGen {
		return nil
	}
	if !a.containerDetailFollow || a.containerDetailFollowPaused {
		return nil
	}
	if a.containerDetailTab != containerDetailTabLogs {
		return nil
	}

	atEnd := a.isContainerDetailAtEnd()
	a.containerDetailContent = appendContainerLogs(a.containerDetailContent, msg.logs)
	if a.containerDetailCache != nil {
		a.containerDetailCache[containerDetailTabLogs] = a.containerDetailContent
	}
	if atEnd {
		a.containerDetailScroll = len(a.containerDetailLines())
	}
	return a.scheduleContainerDetailFollow()
}

func (a *App) reloadContainerDetailLogs() tea.Cmd {
	if a.containerDetailCache != nil {
		delete(a.containerDetailCache, containerDetailTabLogs)
	}
	a.containerDetailContent = ""
	a.containerDetailLoading = true
	a.containerDetailScroll = 0
	a.containerDetailHScroll = 0
	return a.loadContainerDetailTab()
}

func (a *App) handleContainerDetailKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if a.containerDetailSearchQuery != "" {
			a.containerDetailSearchQuery = ""
			a.containerDetailSearchIdx = 0
			return a, nil
		}
		a.stopContainerDetailFollow()
		a.stopContainerDetailStatsLive()
		a.containerSubview = containerSubviewList
		a.containerDetailCache = nil
		a.containerDetailSearchOn = false
		return a, nil
	case "left", "h":
		return a, a.containerDetailSwitchTab(-1)
	case "right", "l":
		return a, a.containerDetailSwitchTab(1)
	case ",":
		a.containerDetailHScrollBy(-8)
	case ".":
		a.containerDetailHScrollBy(8)
	case "up", "k":
		a.containerDetailScrollBy(-1)
	case "down", "j":
		a.containerDetailScrollBy(1)
	case "pgup", "shift+up", "shift+k":
		a.containerDetailScrollBy(-a.containerDetailViewport())
	case "pgdown", "shift+down", "shift+j":
		a.containerDetailScrollBy(a.containerDetailViewport())
	case "home", "g":
		a.containerDetailScroll = 0
	case "end", "G":
		a.containerDetailScrollBy(len(a.containerDetailLines()))
	case "/":
		a.containerDetailSearchOn = true
		a.containerDetailSearchInput = a.containerDetailSearchQuery
		return a, nil
	case "N":
		if a.containerDetailSearchQuery != "" {
			a.jumpContainerDetailSearch(1)
		}
	case "P":
		if a.containerDetailSearchQuery != "" {
			a.jumpContainerDetailSearch(-1)
		}
	case "f":
		if a.containerDetailTab == containerDetailTabLogs {
			return a, a.toggleContainerDetailFollow()
		}
	case "p":
		if a.containerDetailTab == containerDetailTabLogs && a.containerDetailFollow {
			a.containerDetailFollowPaused = !a.containerDetailFollowPaused
			if !a.containerDetailFollowPaused {
				return a, a.scheduleContainerDetailFollow()
			}
			return a, nil
		}
	case "r":
		if a.containerDetailTab == containerDetailTabLogs {
			return a, a.reloadContainerDetailLogs()
		}
		if a.containerDetailTab == containerDetailTabStats {
			return a, a.fetchContainerDetailStatsOnce()
		}
	}
	return a, nil
}
