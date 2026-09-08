package ui

import (
	"fmt"
	"strconv"
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
		return "Métricas"
	case containerDetailTabEnv:
		return "Env"
	case containerDetailTabConfig:
		return "Config"
	case containerDetailTabTop:
		return "Processos"
	case containerDetailTabCompose:
		return "Compose"
	case containerDetailTabFile:
		return "Arquivos"
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
	w := maxInt(40, a.width)
	// Cabeçalho + régua de abas (2) e barra de comandos (até 2): a coluna
	// AÇÕES saiu, então o conteúdo fica com a largura inteira.
	bodyH := a.containerDetailBodyHeight()
	return a.renderContainerDetailChrome(a.renderContainerDetailRichBody(w, bodyH))
}

// containerDetailStatusBadge resume o que muda enquanto se lê: modo de
// acompanhamento, busca e posição. O "[n/7]" saiu — a régua de abas já diz.
func (a *App) containerDetailStatusBadge() string {
	var parts []string
	if a.containerDetailTab == containerDetailTabStats {
		// Métricas não tem linhas para rolar nem buscar: "1-1/1" só confundia.
		if a.containerDetailStatsLive {
			return a.livePulse("ao vivo")
		}
		return StyleMuted.Render("pausado · r recarrega")
	}
	if a.containerDetailTab == containerDetailTabLogs {
		switch {
		case a.containerDetailFollow && a.containerDetailFollowPaused:
			parts = append(parts, StyleWarning.Render("⏸ pausado"))
		case a.containerDetailFollow:
			parts = append(parts, StyleHealthy.Render(pulseGlyph(pulseOK, a.animFrame)+" ao vivo"))
		}
	}
	if a.containerDetailSearchQuery != "" {
		matches := a.containerDetailSearchMatches()
		if len(matches) == 0 {
			parts = append(parts, StyleWarning.Render("sem ocorrência"))
		} else {
			parts = append(parts, StyleAccent.Render(
				fmt.Sprintf("%d de %d", a.containerDetailSearchIdx+1, len(matches))))
		}
	}
	if a.containerDetailHScroll > 0 {
		parts = append(parts, StyleMuted.Render(fmt.Sprintf("↔ %d", a.containerDetailHScroll)))
	}
	if pos := a.containerDetailPosition(a.containerDetailViewport()); pos != "" {
		parts = append(parts, StyleMuted.Render(pos))
	}
	return strings.Join(parts, StyleMuted.Render("  ·  "))
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
	return len(a.containerDetailLines())
}

// renderContainerDetailTabBar: abas numeradas, como nas outras telas — sem o
// número não há como saber que dá para pular direto para a aba 4.
func (a *App) renderContainerDetailTabBar(width int) string {
	if width <= 0 {
		width = maxInt(20, a.width-4)
	}
	compact := width < 78

	parts := make([]string, 0, containerDetailTabTotal)
	for i := 0; i < containerDetailTabTotal; i++ {
		tab := containerDetailTab(i)
		label := fmt.Sprintf(" %d %s ", i+1, strings.ToUpper(tab.shortLabel()))
		if compact {
			label = fmt.Sprintf(" %d ", i+1)
			if tab == a.containerDetailTab {
				label = fmt.Sprintf(" %d %s ", i+1, strings.ToUpper(tab.shortLabel()))
			}
		}
		if tab == a.containerDetailTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	right := a.containerDetailStatusBadge()
	if right == "" || lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, right+" ", width)
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
	// O docker devolve tudo terminado em "\n": sem aparar, toda aba mostra uma
	// última linha vazia e conta uma linha a mais do que existe.
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{"(vazio)"}
	}
	return layoutDetailLines(a.containerDetailTab, lines)
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

// containerDetailBodyHeight é a altura do painel de conteúdo — a mesma conta
// que a moldura faz, para a posição no cabeçalho bater com o que se vê.
func (a *App) containerDetailBodyHeight() int {
	return maxInt(6, maxInt(12, a.height-1)-4)
}

func (a *App) containerDetailViewport() int {
	return maxInt(1, a.containerDetailBodyHeight()-2)
}

// containerDetailGotoTab: as setas passaram a rolar o texto de lado, então a
// troca de aba é só pelo número — que a régua já mostra.
func (a *App) containerDetailGotoTab(i int) tea.Cmd {
	if i < 0 || i >= containerDetailTabTotal || containerDetailTab(i) == a.containerDetailTab {
		return nil
	}
	a.stopContainerDetailFollow()
	a.stopContainerDetailStatsLive()
	a.containerDetailSearchQuery = ""
	a.containerDetailSearchIdx = 0
	a.containerDetailHScroll = 0
	a.containerDetailTab = containerDetailTab(i)
	a.containerDetailScroll = 0
	return a.loadContainerDetailTab()
}

func (a *App) containerDetailScrollBy(delta int) {
	viewport := a.containerDetailViewport()
	a.containerDetailScroll = clampScroll(a.containerDetailScroll+delta, viewport, a.containerDetailContentLen())
}

// containerDetailTextWidth: a mesma conta do corpo (moldura, numeração e o
// separador " │ "), para o passo lateral parar exatamente onde o texto acaba.
func (a *App) containerDetailTextWidth() int {
	inner := maxInt(20, maxInt(40, a.width)-2)
	gutter := maxInt(3, len(strconv.Itoa(a.containerDetailContentLen())))
	return maxInt(8, inner-gutter-3)
}

func (a *App) containerDetailHScrollBy(delta int) {
	textW := a.containerDetailTextWidth()
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
	case "1", "2", "3", "4", "5", "6", "7":
		return a, a.containerDetailGotoTab(int(msg.String()[0] - '1'))
	case "left", "h":
		a.containerDetailHScrollBy(-8)
	case "right", "l":
		a.containerDetailHScrollBy(8)
	case "shift+left", "H":
		a.containerDetailHScrollBy(-a.containerDetailTextWidth() / 2)
	case "shift+right", "L":
		a.containerDetailHScrollBy(a.containerDetailTextWidth() / 2)
	case "0":
		a.containerDetailHScroll = 0
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
	case "n":
		if a.containerDetailSearchQuery != "" {
			a.jumpContainerDetailSearch(1)
		}
	case "N":
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
