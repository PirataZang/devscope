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

// renderContainerStatsScreen usa a mesma moldura das abas de texto — antes
// tinha cabeçalho próprio, moldura arredondada, coluna AÇÕES e um rodapé
// repetindo os mesmos três atalhos.
func (a *App) renderContainerStatsScreen() string {
	w := maxInt(40, a.width)
	bodyH := a.containerDetailBodyHeight()
	var body string
	if a.containerDetailLoading && len(a.containerDetailCPUHist) == 0 {
		body = renderApiTitledBox("MÉTRICAS",
			fitExactLines([]string{a.loadingText("coletando métricas do docker…")}, bodyH-2),
			w, bodyH, true)
	} else {
		body = a.renderContainerStatsDashboard(w, bodyH)
	}
	return a.renderContainerDetailChrome(body)
}

func (a *App) renderContainerStatsDashboard(width, height int) string {
	s := a.containerDetailStats
	rows := []string{a.renderContainerStatsStrip(width, s)}
	remain := height - 1
	if s.CPU == 0 && s.MemPct == 0 && s.PIDs == 0 && len(a.containerDetailCPUHist) <= 1 {
		rows = append(rows, renderApiTitledBox("STATUS", fitExactLines([]string{
			StyleWarning.Render("sem amostra útil — container parado ou docker stats indisponível"),
			StyleMuted.Render("mantenha a aba aberta com o container running · r recarrega"),
		}, 2), width, 4, false))
		remain -= 4
	}
	rows = append(rows, a.renderContainerStatsCharts(width, maxInt(10, remain), s))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderContainerStatsStrip: os quatro cards de um número viraram uma régua.
// A altura que eles comiam volta para o histórico, que é o que se olha.
func (a *App) renderContainerStatsStrip(width int, s dockerStatsSample) string {
	cpu := statsLoadStyle(s.CPU, 50, 80)
	mem := statsLoadStyle(s.MemPct, 60, 85)
	cells := []string{
		StyleMuted.Render("CPU ") + cpu.Render(fmt.Sprintf("%6.2f%%", s.CPU)) + " " + meterBar(clampPct(s.CPU), 8),
		StyleMuted.Render("MEM ") + mem.Render(fmt.Sprintf("%6.2f%%", s.MemPct)) + " " + meterBar(clampPct(s.MemPct), 8),
		StyleMuted.Render("REDE ") + StyleNormal.Render(formatNetKB(s.NetRX+s.NetTX)),
		StyleMuted.Render("BLOCO ") + StyleNormal.Render(formatNetKB(s.BlkR+s.BlkW)),
		StyleMuted.Render("PIDS ") + StyleNormal.Render(strconv.Itoa(s.PIDs)),
	}
	left := strings.Join(cells, StyleMuted.Render("  ·  "))

	right := StyleMuted.Render(fmt.Sprintf("%d amostras · janela ~%ds",
		len(a.containerDetailCPUHist), len(a.containerDetailCPUHist)*2))
	return joinWithSpacer(truncateVisible(left, width), right, width)
}

// statsScaleTop escolhe o teto do gráfico em degraus fixos: assim a escala não
// dança a cada amostra, mas ainda mostra relevo em container ocioso.
func statsScaleTop(hist []float64) float64 {
	switch peak := maxFloats(hist); {
	case peak <= 10:
		return 10
	case peak <= 25:
		return 25
	case peak <= 50:
		return 50
	default:
		return 100
	}
}

func statsLoadStyle(v, warn, bad float64) lipgloss.Style {
	switch {
	case v >= bad:
		return StyleUnhealthy
	case v >= warn:
		return StyleWarning
	default:
		return StyleHealthy
	}
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
	botH := 8 // 3 séries + identidade + moldura
	halfH := maxInt(6, height-botH)
	botH = maxInt(6, height-halfH)

	// Teto fixo em 100 desenhava 4% de CPU como uma linha no fundo de 20 linhas
	// vazias. O teto acompanha a janela — e vai escrito no título, senão o
	// gráfico mente sobre a escala.
	cpuTop := statsScaleTop(a.containerDetailCPUHist)
	memTop := statsScaleTop(a.containerDetailMemHist)
	cpuBox := renderApiTitledBox(fmt.Sprintf("CPU %% · 0-%.0f%%", cpuTop),
		fitExactLines(statsHistoryLines(a.containerDetailCPUHist, leftW-2, halfH-2, cpuTop, StyleAccent), halfH-2),
		leftW, halfH, false)
	memBox := renderApiTitledBox(fmt.Sprintf("MEM %% · 0-%.0f%%", memTop),
		fitExactLines(statsHistoryLines(a.containerDetailMemHist, rightW-2, halfH-2, memTop, StyleHealthy), halfH-2),
		rightW, halfH, false)
	top := lipgloss.JoinHorizontal(lipgloss.Top, cpuBox, memBox)

	// Série curta espalhada por 120 colunas vira uma linha vazia com um risco no
	// fim; estreitar a faixa é o que faz o desenho voltar a dizer algo.
	sparkW := maxInt(12, minInt(48, width/3))
	label := func(name, spark, detail string) string {
		return StyleMuted.Render(padRight(name, 5)) + spark + StyleMuted.Render("   ") + detail
	}
	botLines := []string{
		label("NET", StyleWarning.Render(renderMetricSparkline(a.containerDetailNetHist, sparkW, 0)),
			StyleMuted.Render("rx ")+StyleNormal.Render(formatNetKB(s.NetRX))+
				StyleMuted.Render("  tx ")+StyleNormal.Render(formatNetKB(s.NetTX))),
		label("BLK", StyleUnhealthy.Render(renderMetricSparkline(a.containerDetailBlkHist, sparkW, 0)),
			StyleMuted.Render("leitura ")+StyleNormal.Render(formatNetKB(s.BlkR))+
				StyleMuted.Render("  escrita ")+StyleNormal.Render(formatNetKB(s.BlkW))),
		label("PIDS", StyleAccent.Render(renderMetricSparkline(a.containerDetailPIDHist, sparkW, 0)),
			StyleNormal.Render(strconv.Itoa(s.PIDs))),
		"",
		StyleMuted.Render("mem   ") + StyleNormal.Render(firstNonEmpty(s.MemLabel, emDash)) +
			StyleMuted.Render("   id  ") + StyleNormal.Render(truncate(firstNonEmpty(a.containerDetailID, emDash), 12)),
	}
	bottom := renderApiTitledBox("I/O · PROCESSOS", fitExactLines(botLines, botH-2), width, botH, false)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

// statsHistoryLines: só as barras e o resumo. A sparkline que ficava no topo
// desenhava a mesma série logo acima do histograma dela.
func statsHistoryLines(hist []float64, width, rows int, maxHint float64, style lipgloss.Style) []string {
	if rows < 3 {
		rows = 3
	}
	lines := renderHistoryBarRows(hist, maxInt(8, width), rows-1, maxHint, style)
	cur := 0.0
	if n := len(hist); n > 0 {
		cur = hist[n-1]
	}
	return append(lines, StyleMuted.Render(fmt.Sprintf("agora %.2f  ·  média %.2f  ·  pico %.2f",
		cur, avgFloats(hist), maxFloats(hist))))
}

// renderHistoryBarRows estica a janela para a largura da caixa: são 40
// amostras no máximo, e uma coluna por amostra deixava 2/3 do gráfico vazio.
func renderHistoryBarRows(hist []float64, width, rows int, maxHint float64, style lipgloss.Style) []string {
	if len(hist) == 0 || width <= 0 {
		return []string{StyleMuted.Render(strings.Repeat("·", minInt(maxInt(width, 1), 24)))}
	}
	maxV := maxHint
	if maxV <= 0 {
		maxV = maxFloats(hist)
		if maxV <= 0 {
			maxV = 1
		}
	}
	cols := make([]float64, width)
	for c := range cols {
		cols[c] = hist[c*len(hist)/width] / maxV
	}
	out := make([]string, 0, rows)
	for r := rows - 1; r >= 0; r-- {
		threshold := float64(r+1) / float64(rows)
		var b strings.Builder
		for _, v := range cols {
			if v >= threshold-1e-9 {
				b.WriteString("█")
			} else {
				b.WriteString(" ")
			}
		}
		out = append(out, style.Render(b.String()))
	}
	return out
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
