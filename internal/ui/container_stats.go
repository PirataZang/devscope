package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type dockerStatsSample struct {
	CPU      float64
	MemPct   float64
	MemLabel string
	NetRX    float64
	NetTX    float64
	NetLabel string
	BlkR     float64
	BlkW     float64
	BlkLabel string
	PIDs     int
	Raw      string
}

type containerDetailStatsMsg struct {
	id     string
	gen    int
	sample dockerStatsSample
	err    string
}

func (a *App) renderContainerStatsScreen() string {
	height := maxInt(12, a.height-2)
	panelW := maxInt(40, a.width)
	innerW := maxInt(36, panelW-2)

	nameW := maxInt(8, innerW/2)
	title := StyleSection.Render(truncate(a.containerDetailName, nameW))
	status := a.containerDetailStatusBadge()
	if a.containerDetailStatsLive {
		status += "  " + StyleHealthy.Render("● live")
	}
	header := title + "  " + status
	tabs := a.renderContainerDetailTabBar(innerW)

	bodyH := maxInt(8, height-6)
	var body string
	if a.containerDetailLoading && len(a.containerDetailCPUHist) == 0 {
		body = renderApiTitledBox("STATS", fitExactLines([]string{StyleMuted.Render("Coletando métricas do Docker…")}, bodyH-2), innerW, bodyH, true)
	} else {
		body = a.renderContainerStatsDashboard(innerW, bodyH)
	}

	footer := StyleMuted.Render(truncate("r refresh  ↔ abas  esc lista  ·  poll 2s", innerW))
	content := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tabAccentColor(TabContainers)).
		Width(panelW).
		MaxWidth(panelW).
		Render(content)
	panel = clampRenderedHeight(panel, height)
	statusBar := truncate(a.renderStatusBar("container · Stats"), panelW)
	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

func (a *App) renderContainerStatsDashboard(width, height int) string {
	s := a.containerDetailStats
	bannerH := 0
	var banner string
	if s.Raw == "" || (s.CPU == 0 && s.MemPct == 0 && s.PIDs == 0 && len(a.containerDetailCPUHist) <= 1) {
		bannerH = 3
		banner = renderApiTitledBox("STATUS", fitExactLines([]string{
			StyleWarning.Render("sem amostra útil — container parado ou docker stats indisponível"),
			StyleMuted.Render("mantenha a aba aberta com o container running · r refresh"),
		}, 2), width, bannerH, false)
	}
	remain := height - bannerH
	cardH := maxInt(5, remain*22/100)
	chartH := maxInt(8, remain*42/100)
	bottomH := maxInt(6, remain-cardH-chartH)

	cards := a.renderContainerStatsCards(width, cardH, s)
	charts := a.renderContainerStatsCharts(width, chartH, s)
	details := a.renderContainerStatsDetails(width, bottomH, s)
	if bannerH > 0 {
		return lipgloss.JoinVertical(lipgloss.Left, banner, cards, charts, details)
	}
	return lipgloss.JoinVertical(lipgloss.Left, cards, charts, details)
}

func (a *App) renderContainerStatsCards(width, height int, s dockerStatsSample) string {
	n := 4
	gap := 1
	cw := (width - gap*(n-1)) / n
	if cw < 14 {
		cw = maxInt(12, (width-(n-1)*gap)/n)
	}
	netTotal := s.NetRX + s.NetTX
	blkTotal := s.BlkR + s.BlkW
	cards := []string{
		renderStatsCard("CPU", fmt.Sprintf("%.2f%%", s.CPU), meterBar(clampPct(s.CPU), cw-4), StyleAccent, cw, height),
		renderStatsCard("MEMÓRIA", fmt.Sprintf("%.2f%%", s.MemPct), meterBar(clampPct(s.MemPct), cw-4), StyleHealthy, cw, height),
		renderStatsCard("REDE", formatNetKB(netTotal), StyleMuted.Render(truncate(s.NetLabel, cw-4)), StyleWarning, cw, height),
		renderStatsCard("BLOCK I/O", formatNetKB(blkTotal), StyleMuted.Render(truncate(s.BlkLabel, cw-4)), StyleUnhealthy, cw, height),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		cards[0], strings.Repeat(" ", gap),
		cards[1], strings.Repeat(" ", gap),
		cards[2], strings.Repeat(" ", gap),
		cards[3],
	)
}

func renderStatsCard(title, value, sub string, valueStyle lipgloss.Style, width, height int) string {
	lines := []string{
		valueStyle.Bold(true).Render(value),
		sub,
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, false)
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (a *App) renderContainerStatsCharts(width, height int, s dockerStatsSample) string {
	leftW := width / 2
	rightW := width - leftW
	halfH := height * 58 / 100
	if halfH < 5 {
		halfH = 5
	}
	botH := height - halfH
	if botH < 4 {
		botH = 4
		halfH = height - botH
	}

	cpuBox := renderApiTitledBox("CPU %",
		fitExactLines(statsHistoryLines(a.containerDetailCPUHist, leftW-4, halfH-2, 100, StyleAccent), halfH-2),
		leftW, halfH, false)
	memBox := renderApiTitledBox("MEM %",
		fitExactLines(statsHistoryLines(a.containerDetailMemHist, rightW-4, halfH-2, 100, StyleHealthy), halfH-2),
		rightW, halfH, false)
	top := lipgloss.JoinHorizontal(lipgloss.Top, cpuBox, memBox)

	netSpark := StyleWarning.Render(renderMetricSparkline(a.containerDetailNetHist, maxInt(8, width-20), 0))
	blkSpark := StyleUnhealthy.Render(renderMetricSparkline(a.containerDetailBlkHist, maxInt(8, width-20), 0))
	pidSpark := StyleAccent.Render(renderMetricSparkline(a.containerDetailPIDHist, maxInt(8, width-20), 0))
	botLines := []string{
		StyleMuted.Render("NET  ") + netSpark,
		StyleMuted.Render("     ") + StyleMuted.Render(fmt.Sprintf("rx %s  tx %s", formatNetKB(s.NetRX), formatNetKB(s.NetTX))),
		StyleMuted.Render("BLK  ") + blkSpark,
		StyleMuted.Render("     ") + StyleMuted.Render(fmt.Sprintf("r %s  w %s", formatNetKB(s.BlkR), formatNetKB(s.BlkW))),
		StyleMuted.Render("PIDS ") + pidSpark + StyleNormal.Render(fmt.Sprintf("  %d", s.PIDs)),
		StyleMuted.Render(fmt.Sprintf("amostras %d  · janela ~%ds", len(a.containerDetailCPUHist), len(a.containerDetailCPUHist)*2)),
	}
	bottom := renderApiTitledBox("I/O · PROCESSOS", fitExactLines(botLines, botH-2), width, botH, false)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func statsHistoryLines(hist []float64, width, rows int, maxHint float64, style lipgloss.Style) []string {
	if rows < 3 {
		rows = 3
	}
	sparkW := maxInt(8, width)
	lines := make([]string, 0, rows)
	lines = append(lines, style.Render(renderMetricSparkline(hist, sparkW, maxHint)))
	barRows := minInt(rows-2, 7)
	if barRows < 2 {
		barRows = 2
	}
	lines = append(lines, renderHistoryBarRows(hist, sparkW, barRows, maxHint, style)...)
	cur := 0.0
	if n := len(hist); n > 0 {
		cur = hist[n-1]
	}
	lines = append(lines, StyleMuted.Render(fmt.Sprintf("now %.2f  avg %.2f  peak %.2f", cur, avgFloats(hist), maxFloats(hist))))
	return lines
}

func renderHistoryBarRows(hist []float64, width, rows int, maxHint float64, style lipgloss.Style) []string {
	if len(hist) == 0 {
		return []string{StyleMuted.Render(strings.Repeat("·", minInt(width, 24)))}
	}
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	maxV := maxHint
	if maxV <= 0 {
		maxV = maxFloats(hist)
		if maxV <= 0 {
			maxV = 1
		}
	}
	out := make([]string, 0, rows)
	for r := rows - 1; r >= 0; r-- {
		threshold := float64(r+1) / float64(rows)
		var b strings.Builder
		for _, v := range hist {
			if v/maxV >= threshold-1e-9 {
				b.WriteString(style.Render("█"))
			} else {
				b.WriteString(" ")
			}
		}
		out = append(out, b.String())
	}
	return out
}

func (a *App) renderContainerStatsDetails(width, height int, s dockerStatsSample) string {
	rightW := maxInt(22, width*28/100)
	leftW := width - rightW
	leftLines := []string{
		StyleMuted.Render("CPU      ") + StyleAccent.Render(fmt.Sprintf("%.2f%%", s.CPU)),
		StyleMuted.Render("Memory   ") + StyleNormal.Render(firstNonEmpty(s.MemLabel, "—")),
		StyleMuted.Render("Mem %    ") + StyleHealthy.Render(fmt.Sprintf("%.2f%%", s.MemPct)),
		StyleMuted.Render("Net I/O  ") + StyleNormal.Render(firstNonEmpty(s.NetLabel, "—")),
		StyleMuted.Render("Block    ") + StyleNormal.Render(firstNonEmpty(s.BlkLabel, "—")),
		StyleMuted.Render("PIDs     ") + StyleNormal.Render(strconv.Itoa(s.PIDs)),
		StyleMuted.Render("Name     ") + StyleMuted.Render(truncate(a.containerDetailName, leftW-12)),
		StyleMuted.Render("ID       ") + StyleMuted.Render(truncate(a.containerDetailID, 16)),
	}
	hints := []string{}
	switch {
	case s.CPU >= 80:
		hints = append(hints, StyleUnhealthy.Render("● CPU alta"))
	case s.CPU >= 50:
		hints = append(hints, StyleWarning.Render("● CPU moderada"))
	default:
		hints = append(hints, StyleHealthy.Render("● CPU ok"))
	}
	switch {
	case s.MemPct >= 85:
		hints = append(hints, StyleUnhealthy.Render("● MEM crítica"))
	case s.MemPct >= 60:
		hints = append(hints, StyleWarning.Render("● MEM elevada"))
	default:
		hints = append(hints, StyleHealthy.Render("● MEM ok"))
	}
	rightLines := append(hints, "")
	rightLines = append(rightLines, moduleActionLines(
		[2]string{"r", "refresh"},
		[2]string{"←→", "outras abas"},
		[2]string{"esc", "lista"},
	)...)

	left := renderApiTitledBox("DETALHES", fitExactLines(leftLines, height-2), leftW, height, false)
	right := renderApiTitledBox("SAÚDE", fitExactLines(rightLines, height-2), rightW, height, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (a *App) startContainerDetailStatsLive() tea.Cmd {
	a.containerDetailStatsLive = true
	a.containerDetailStatsGen++
	// handler schedules the next tick after the first sample
	return a.fetchContainerDetailStatsOnce()
}

func (a *App) stopContainerDetailStatsLive() {
	a.containerDetailStatsLive = false
	a.containerDetailStatsGen++
}

func (a *App) fetchContainerDetailStatsOnce() tea.Cmd {
	if a.containerDetailID == "" {
		return nil
	}
	id := a.containerDetailID
	name := a.containerDetailName
	gen := a.containerDetailStatsGen
	return func() tea.Msg {
		c := core.Container{ID: id, Name: name}
		raw, err := collectors.DockerContainerStats(collectors.DockerExecTarget(c))
		msg := containerDetailStatsMsg{id: id, gen: gen, sample: parseDockerStatsFull(raw)}
		if err != nil {
			msg.err = err.Error()
		}
		return msg
	}
}

func (a *App) scheduleContainerDetailStats() tea.Cmd {
	if !a.containerDetailStatsLive || a.containerDetailID == "" || a.containerDetailTab != containerDetailTabStats {
		return nil
	}
	id := a.containerDetailID
	name := a.containerDetailName
	gen := a.containerDetailStatsGen
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		c := core.Container{ID: id, Name: name}
		raw, err := collectors.DockerContainerStats(collectors.DockerExecTarget(c))
		msg := containerDetailStatsMsg{id: id, gen: gen, sample: parseDockerStatsFull(raw)}
		if err != nil {
			msg.err = err.Error()
		}
		return msg
	})
}

func (a *App) handleContainerDetailStats(msg containerDetailStatsMsg) tea.Cmd {
	if msg.id != a.containerDetailID || msg.gen != a.containerDetailStatsGen {
		return nil
	}
	if a.containerDetailTab != containerDetailTabStats {
		return nil
	}
	a.containerDetailLoading = false
	if msg.err != "" && msg.sample.Raw == "" {
		a.containerDetailContent = "erro: " + msg.err
		return a.scheduleContainerDetailStats()
	}
	a.applyContainerDetailStats(msg.sample)
	return a.scheduleContainerDetailStats()
}

func (a *App) applyContainerDetailStats(s dockerStatsSample) {
	a.containerDetailStats = s
	a.containerDetailContent = s.Raw
	if a.containerDetailCache == nil {
		a.containerDetailCache = make(map[containerDetailTab]string)
	}
	a.containerDetailCache[containerDetailTabStats] = s.Raw
	a.containerDetailCPUHist = appendMetricHistory(a.containerDetailCPUHist, s.CPU)
	a.containerDetailMemHist = appendMetricHistory(a.containerDetailMemHist, s.MemPct)
	a.containerDetailNetHist = appendMetricHistory(a.containerDetailNetHist, s.NetRX+s.NetTX)
	a.containerDetailBlkHist = appendMetricHistory(a.containerDetailBlkHist, s.BlkR+s.BlkW)
	if s.PIDs > 0 {
		a.containerDetailPIDHist = appendMetricHistory(a.containerDetailPIDHist, float64(s.PIDs))
	}
}

func parseDockerStatsFull(raw string) dockerStatsSample {
	s := dockerStatsSample{Raw: strings.TrimSpace(raw)}
	for _, line := range strings.Split(s.Raw, "\n") {
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "cpu"):
			s.CPU = firstFloatIn(line)
		case strings.Contains(low, "memory") || strings.HasPrefix(strings.TrimSpace(low), "mem"):
			if i := strings.Index(line, ":"); i >= 0 {
				s.MemLabel = strings.TrimSpace(line[i+1:])
			} else {
				s.MemLabel = strings.TrimSpace(line)
			}
			if i := strings.LastIndex(line, "("); i >= 0 {
				s.MemPct = firstFloatIn(line[i:])
			}
		case strings.Contains(low, "net"):
			rest := line
			if i := strings.Index(line, ":"); i >= 0 {
				rest = strings.TrimSpace(line[i+1:])
			}
			s.NetLabel = rest
			parts := strings.Split(rest, "/")
			if len(parts) >= 1 {
				s.NetRX = parseDockerBytesToKB(parts[0])
			}
			if len(parts) >= 2 {
				s.NetTX = parseDockerBytesToKB(parts[1])
			}
		case strings.Contains(low, "block"):
			rest := line
			if i := strings.Index(line, ":"); i >= 0 {
				rest = strings.TrimSpace(line[i+1:])
			}
			s.BlkLabel = rest
			parts := strings.Split(rest, "/")
			if len(parts) >= 1 {
				s.BlkR = parseDockerBytesToKB(parts[0])
			}
			if len(parts) >= 2 {
				s.BlkW = parseDockerBytesToKB(parts[1])
			}
		case strings.Contains(low, "pid"):
			s.PIDs = int(firstFloatIn(line))
		}
	}
	return s
}

func avgFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func maxFloats(v []float64) float64 {
	var m float64
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
