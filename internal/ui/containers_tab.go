package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type containerSubview int

const (
	containerSubviewList containerSubview = iota
	containerSubviewDetail
	containerSubviewShellReturn
	containerSubviewPorts
	containerSubviewImages
	containerSubviewDeps
)

type containerPortPreviewMsg struct {
	port int
	gen  int
	body string
}

type containerPreviewMsg struct {
	id      string
	gen     int
	logs    string
	stats   string
	volumes []string
	cpu     float64
	mem     float64
	net     float64
}

func (a *App) initContainersTab() {
	a.containerSubview = containerSubviewList
	a.containerScroll = 0
	a.tabCursor = 0
	a.containerDetailCache = nil
	a.containerDetailScroll = 0
	a.containerDetailContent = ""
	a.containerDetailLoading = false
	a.containerStatusMsg = ""
	a.containerActions = nil
	a.containerFilterOn = false
	a.containerFilterInput = ""
	a.containerFilter = ""
	a.containerShowAll = false
	a.containerOnlyDocker = true
	a.containerPreviewID = ""
	a.containerPreviewLogs = ""
	a.containerPreviewStats = ""
	a.containerPreviewVolumes = nil
	a.containerCPUHistory = nil
	a.containerMemHistory = nil
	a.containerNetHistory = nil
	a.imageScope = imageScopeContainer
	a.imageAll = nil
	a.imageCursor = 0
	a.imageScroll = 0
	a.imageLoading = false
	a.imageStatusMsg = ""
	a.imageConfirmRemove = false
	a.imageConfirmCursor = 0
	a.containerDeps = nil
	a.containerDepsCursor = 0
	a.containerDepsScroll = 0
	a.resetContainerPortsView()
}

func (a *App) resetContainerPortsView() {
	a.containerPortCursor = 0
	a.containerPortPreview = ""
	a.containerPortPreviewPort = 0
	a.containerPortLoading = false
	a.containerConfirmClosePort = false
	a.containerPortGen++
}

func (a *App) renderContainersTab(p *core.Project) string {
	var view string
	switch a.containerSubview {
	case containerSubviewDetail:
		view = a.renderContainerDetail(p)
	case containerSubviewPorts:
		view = a.renderContainerPorts(p)
	case containerSubviewImages:
		view = a.renderContainerImages(p)
	case containerSubviewDeps:
		view = a.renderContainerDeps(p)
	case containerSubviewShellReturn:
		view = renderShellReturnMessage(a.containerShellExitErr)
	default:
		view = a.renderContainerList(p)
	}
	if a.containerConfirmRemove {
		w, h := maxInt(40, a.width), maxInt(12, a.height)
		target, detail := a.containerDeleteConfirmLabels(p)
		box := renderDeleteConfirmBox(deleteConfirmOpts{
			Brand:    "DOCKER",
			Color:    tabAccentColor(TabContainers),
			Title:    "Excluir container",
			Subtitle: "docker rm — remove o container",
			Label:    "container",
			Target:   target,
			Detail:   detail,
		}, w, h)
		view = overlayCentered(view, box, w, h)
	}
	if a.containerConfirmClosePort {
		w, h := maxInt(40, a.width), maxInt(12, a.height)
		port, ok := a.selectedContainerPort(p)
		target := "—"
		if ok {
			target = fmt.Sprintf(":%d", port.HostPort)
		}
		box := renderDeleteConfirmBox(deleteConfirmOpts{
			Brand:    "DOCKER",
			Color:    tabAccentColor(TabContainers),
			Title:    "Fechar porta",
			Subtitle: "recria o container sem publicar esta porta",
			Label:    "porta",
			Target:   target,
			Detail:   "docker stop → recreate sem -p " + target,
		}, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) containerDeleteConfirmLabels(p *core.Project) (target, detail string) {
	if p == nil {
		return "—", ""
	}
	c, ok := a.selectedContainer(p)
	if !ok {
		return "—", ""
	}
	detail = truncate(firstNonEmpty(c.Image, c.ID), 48)
	if c.Status != "" {
		detail = c.Status + "  ·  " + detail
	}
	return c.Name, detail
}

func (a *App) dismissContainerShellReturn() tea.Cmd {
	a.containerSubview = containerSubviewList
	if a.containerShellExitErr != "" {
		a.containerStatusMsg = a.containerShellExitErr
	}
	a.containerShellExitErr = ""
	a.restoreContainerCursor(a.containerPreviewID)
	return tea.Batch(
		tea.ClearScreen,
		a.refreshDocker(),
		a.requestContainerPreview(),
	)
}

func (a *App) renderContainerList(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())

	containers := a.filteredContainers(p)
	if a.projectDockerLoading && len(containers) == 0 {
		return renderApiTitledBox("CONTAINERS", fitExactLines([]string{a.loadingText("Carregando containers…")}, h-2), w, h, true)
	}
	if len(containers) == 0 {
		msg := []string{
			StyleMuted.Render("Nenhum container vinculado a este projeto."),
			StyleMuted.Render("Vinculamos por docker-compose working_dir, config e volumes."),
			StyleMuted.Render("A · ver containers de todos os projetos"),
		}
		if a.containerShowAll {
			msg = []string{StyleMuted.Render("Nenhum container no docker (ps -a) nem nos projetos."), StyleMuted.Render("A · voltar ao projeto atual")}
		}
		return renderApiTitledBox("CONTAINERS", fitExactLines(msg, h-2), w, h, true)
	}

	header := a.renderContainersHeader(p, w)
	strip := a.renderContainersStrip(containers, w)
	notif := a.renderContainersNotif()
	// A coluna AÇÕES de 30 colunas guardava 17 atalhos cortados; virou barra
	// larga no rodapé, como no Git.
	cmdBar := a.renderContainersCommandBar(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(strip) + lipgloss.Height(notif) + 1
	bodyH := maxInt(10, h-chromeH-lipgloss.Height(cmdBar))
	bottomH := maxInt(6, bodyH*34/100)
	tableH := maxInt(6, bodyH-bottomH)

	table := a.renderContainersTable(containers, w, tableH)
	bottom := a.renderContainersBottom(w, bottomH)

	stack := lipgloss.JoinVertical(lipgloss.Left, table, bottom)
	if fill := h - chromeH - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, strip, notif, stack, cmdBar)
}

func (a *App) renderContainersHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabContainers)).Bold(true)
	left := accent.Render("▣ CONTAINERS")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 24))
	}
	if a.containerShowAll {
		left += "   " + StyleAccent.Render("todos os projetos + órfãos")
	}
	if a.containerOnlyDocker {
		left += StyleMuted.Render("   só docker")
	}

	var right []string
	if p != nil && p.HasDockerCompose {
		right = append(right, StyleMuted.Render("compose"))
	}
	if a.projectDockerLoading {
		right = append(right, a.loadingMuted("atualizando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// renderContainersStrip: as quatro caixas CPU/RAM/DISK/NET mostravam métrica do
// HOST — que é assunto do dashboard, e aqui aparecia como DISK 0% / NET —.
// No lugar entram os contadores por estado, que é o que se olha numa lista de
// containers, e a linha de busca.
func (a *App) renderContainersStrip(containers []core.Container, width int) string {
	running, restarting, unhealthy, stopped, paused := containerCounts(containers)
	var chips []string
	add := func(st lipgloss.Style, level pulseLevel, n int, label string) {
		if n > 0 {
			chips = append(chips, st.Render(fmt.Sprintf("%s %d", pulseGlyph(level, a.animFrame), n))+
				StyleMuted.Render(" "+label))
		}
	}
	add(StyleHealthy, pulseOK, running, "no ar")
	add(StyleWarning, pulseWarn, restarting, "reiniciando")
	add(StyleUnhealthy, pulseBad, unhealthy, "unhealthy")
	add(StyleWarning, pulseWarn, paused, "pausado")
	add(StyleMuted, pulseBad, stopped, "parado")
	if n := len(a.snapshot.OrphanContainers); a.containerShowAll && n > 0 {
		chips = append(chips, StyleWarning.Render(fmt.Sprintf("%d órfãos", n)))
	}
	left := "  "
	if len(chips) == 0 {
		left += StyleMuted.Render("nenhum container")
	} else {
		left += strings.Join(chips, StyleMuted.Render("  ·  "))
	}

	right := a.containersFilterLabel()
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, right, width)
}

func (a *App) containersFilterLabel() string {
	switch {
	case a.containerFilterOn:
		return StyleKey.Render("/ ") + StyleSelected.Render(a.containerFilterInput+"▌") + " "
	case a.containerFilter != "":
		return StyleMuted.Render("filtro ") + StyleNormal.Render(a.containerFilter) +
			StyleMuted.Render("  esc limpa ")
	default:
		return StyleKey.Render("/") + StyleMuted.Render(" buscar  ·  ") +
			StyleKey.Render("A") + StyleMuted.Render(" escopo  ·  ") +
			StyleKey.Render("g") + StyleMuted.Render(" métrica ")
	}
}

// containerCounts classifica pelo State do docker. A contagem antiga comparava
// c.Status ("Up 2 days") com "running" e por isso dizia sempre "0 running".
func containerCounts(containers []core.Container) (running, restarting, unhealthy, stopped, paused int) {
	for _, c := range containers {
		if strings.EqualFold(c.Health, "unhealthy") {
			unhealthy++
			continue
		}
		switch containerStateKind(c) {
		case "running":
			running++
		case "restarting":
			restarting++
		case "paused":
			paused++
		default: // exited, created, missing — nenhum está no ar
			stopped++
		}
	}
	return
}

// containerStateKind normaliza State/Status num dos estados que a tela mostra.
func containerStateKind(c core.Container) string {
	s := strings.ToLower(strings.TrimSpace(c.State))
	if s == "" {
		s = strings.ToLower(c.Status)
	}
	switch {
	case strings.Contains(s, "restart"):
		return "restarting"
	case strings.Contains(s, "paus"):
		return "paused"
	case strings.Contains(s, "created"):
		return "created" // nunca subiu: é diferente de ter subido e caído
	case strings.Contains(s, "missing"):
		return "missing"
	case strings.Contains(s, "exit"), strings.Contains(s, "dead"):
		return "stopped"
	case strings.Contains(s, "run"), strings.HasPrefix(s, "up "):
		return "running"
	default:
		return "stopped"
	}
}

// renderContainersCommandBar: os 17 atalhos por extenso numa barra larga, em
// até duas linhas. Na coluna de 30 colunas saíam "r start/rest" e "S-R ∞/off".
func (a *App) renderContainersCommandBar(width int) string {
	scope := "todos os projetos"
	if a.containerShowAll {
		scope = "só este projeto"
	}
	only := "só docker"
	if a.containerOnlyDocker {
		only = "incluir ausentes"
	}
	items := [][2]string{
		{"enter", "portas"},
		{"m", "detalhe"},
		{"e", "shell"},
		{"r", "iniciar"},
		{"s", "parar"},
		{"R", "reiniciar"},
		{"p", "pausar"},
		{"d", "remover"},
		{"i", "imagens"},
		{"n", "novo svc"},
		{"S-R", "reinício ∞/off"},
		{"S-U", "compose ↑"},
		{"S-D", "compose ↓"},
		{"^g", "deps"},
		{"A", scope},
		{"v", only},
	}
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
}

func (a *App) containerNetSummary() string {
	for _, line := range strings.Split(a.containerPreviewStats, "\n") {
		if strings.Contains(line, "Net I/O") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "—"
}

func (a *App) renderContainersSearch(width int) string {
	label := StyleMuted.Render("Buscar containers…  /")
	if a.containerFilterOn {
		label = StyleAccent.Render(truncate(a.containerFilterInput+"▌", width-8))
	} else if a.containerFilter != "" {
		label = StyleAccent.Render(truncate("/"+a.containerFilter, width-8))
	}
	pad := maxInt(1, width-lipgloss.Width(stripANSI(label))-4)
	return StyleMuted.Render("╱ ") + label + strings.Repeat(" ", pad)
}

func (a *App) renderContainersNotif() string {
	if a.containerStatusMsg == "" {
		return StyleMuted.Render(" ")
	}
	style := StyleWarning
	if strings.Contains(a.containerStatusMsg, "✓") {
		style = StyleHealthy
	}
	return style.Render(truncate(a.containerStatusMsg, maxInt(40, a.width-4)))
}

func (a *App) renderContainersTable(containers []core.Container, width, height int) string {
	a.containerTableWidth = maxInt(20, width-3)
	defer func() { a.containerTableWidth = 0 }()
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2) // header + separator
	if len(containers) == 0 {
		return renderApiTitledBox("LISTA", fitExactLines([]string{StyleMuted.Render("nenhum resultado")}, inner), width, height, true)
	}
	a.containerScroll = ensureVisible(a.tabCursor, a.containerScroll, viewport, len(containers))
	start := a.containerScroll
	end := minInt(start+viewport, len(containers))

	lines := []string{a.renderContainerHeader(), StyleMuted.Render(strings.Repeat("─", maxInt(20, width-2)))}
	if start > 0 {
		lines[1] = StyleMuted.Render(fmt.Sprintf("↑ %d  ", start) + strings.Repeat("─", maxInt(10, width-10)))
	}
	for i := start; i < end; i++ {
		lines = append(lines, a.renderContainerRow(containers[i], i == a.tabCursor))
	}
	for i := end - start; i < viewport; i++ {
		lines = append(lines, "")
	}
	if rem := len(containers) - end; rem > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("↓ %d abaixo", rem)))
	}
	title := fmt.Sprintf("LISTA (%d)", len(containers))
	if a.containerShowAll {
		title = fmt.Sprintf("LISTA · TODOS (%d)", len(containers))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
}

func (a *App) containerActionItems() [][2]string {
	scope := "todos"
	if a.containerShowAll {
		scope = "projeto"
	}
	only := "só docker"
	if a.containerOnlyDocker {
		only = "c/ missing"
	}
	return [][2]string{
		{"enter", "portas"},
		{"m", "detalhe"},
		{"i", "imagens"},
		{"C-g", "deps"},
		{"n", "novo svc"},
		{"s", "stop"},
		{"r", "start/rest"},
		{"S-R", "∞/off"},
		{"p", "pause"},
		{"d", "remove"},
		{"e", "shell"},
		{"A", scope},
		{"v", only},
		{"g", "métrica"},
		{"S-U", "compose↑"},
		{"S-D", "compose↓"},
		{"/", "buscar"},
	}
}

func containersActionsWidth(total int) int {
	if total < 70 {
		return 0
	}
	w := 28
	if total >= 120 {
		w = 30
	}
	if total >= 160 {
		w = 32
	}
	if w > total*34/100 {
		w = maxInt(24, total*34/100)
	}
	return w
}

func (a *App) renderContainersBottom(width, height int) string {
	rest := maxInt(12, width)
	w1 := maxInt(10, rest*46/100)
	w2 := maxInt(10, rest*28/100)
	w3 := maxInt(10, rest-w1-w2)
	// Keep columns exact so JoinHorizontal never wraps the terminal line.
	if w1+w2+w3 > rest {
		w3 = maxInt(8, rest-w1-w2)
	}
	inner := maxInt(2, height-2)

	logs := a.containerPreviewLogLines(inner, maxInt(4, w1-4))
	stats := a.containerPreviewStatLines(inner, maxInt(4, w2-4))
	ports := a.containerPreviewPortLines(inner, maxInt(4, w3-4))

	title := "LOGS"
	if a.containerPreviewID != "" {
		if c, ok := a.selectedContainer(a.currentProject()); ok {
			title = "LOGS · " + truncate(sanitizeTerminalLine(c.Name), 18)
		}
	}
	actions := a.containerActionItems()
	parts := []string{
		renderApiTitledBox(title, fitExactLines(logs, inner), w1, height, false),
		renderApiTitledBox(a.containerStatsTitle(), fitExactLines(stats, inner), w2, height, false),
		renderApiTitledBox("PORTAS", fitExactLines(ports, inner), w3, height, false),
	}
	if false {
		parts = append(parts, renderContainersActionsBox(0, height, actions...))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderContainersActionsBox like renderActionsBox but keeps the given height so
// every shortcut stays visible (shared helper shrinks to item count).
func renderContainersActionsBox(width, height int, items ...[2]string) string {
	if width < 12 {
		return ""
	}
	innerW := maxInt(4, width-2)
	lines := moduleActionLinesWidth(innerW, items...)
	if height < len(lines)+2 {
		height = len(lines) + 2
	}
	return renderApiTitledBox("AÇÕES", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) containerStatsTitle() string {
	switch a.containerStatsMode {
	case 1:
		return "STATS · CPU  g"
	case 2:
		return "STATS · MEM  g"
	case 3:
		return "STATS · NET  g"
	default:
		return "STATS · ALL  g"
	}
}

func (a *App) containerPreviewLogLines(maxLines, width int) []string {
	if strings.TrimSpace(a.containerPreviewLogs) == "" {
		return []string{StyleMuted.Render("selecione um container")}
	}
	raw := strings.Split(strings.TrimRight(a.containerPreviewLogs, "\n"), "\n")
	if len(raw) > maxLines {
		raw = raw[len(raw)-maxLines:]
	}
	lines := make([]string, 0, maxLines)
	for _, line := range raw {
		line = sanitizeTerminalLine(line)
		style := StyleMuted
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "error") || strings.Contains(low, "err "):
			style = StyleUnhealthy
		case strings.Contains(low, "warn"):
			style = StyleWarning
		case strings.Contains(low, "info"):
			style = StyleAccent
		}
		lines = append(lines, style.Render(truncate(line, width)))
	}
	return lines
}

func (a *App) containerPreviewStatLines(maxLines, width int) []string {
	sparkW := maxInt(8, width-5)
	showCPU := a.containerStatsMode == 0 || a.containerStatsMode == 1
	showMem := a.containerStatsMode == 0 || a.containerStatsMode == 2
	showNet := a.containerStatsMode == 0 || a.containerStatsMode == 3

	fit := func(s string) string {
		if width <= 0 {
			return s
		}
		if lipgloss.Width(s) > width {
			return ansi.Truncate(s, width, "…")
		}
		return s
	}
	lines := make([]string, 0, maxLines)
	if showCPU {
		lines = append(lines, fit(StyleMuted.Render("CPU ")+StyleAccent.Render(renderMetricSparkline(a.containerCPUHistory, sparkW, 100))))
	}
	if showMem {
		lines = append(lines, fit(StyleMuted.Render("MEM ")+StyleHealthy.Render(renderMetricSparkline(a.containerMemHistory, sparkW, 100))))
	}
	if showNet {
		lines = append(lines, fit(StyleMuted.Render("NET ")+StyleWarning.Render(renderMetricSparkline(a.containerNetHistory, sparkW, 0))))
	}

	if c, ok := a.selectedContainer(a.currentProject()); ok {
		cpu := c.CPU
		if n := len(a.containerCPUHistory); n > 0 {
			cpu = a.containerCPUHistory[n-1]
		}
		mem := formatContainerMem(c.Memory)
		memPct := ""
		if n := len(a.containerMemHistory); n > 0 {
			memPct = fmt.Sprintf(" (%.1f%%)", a.containerMemHistory[n-1])
		}
		net := "—"
		if n := len(a.containerNetHistory); n > 0 {
			net = formatNetKB(a.containerNetHistory[n-1])
		}
		lines = append(lines,
			fit(StyleNormal.Render(fmt.Sprintf("CPU %.1f%%", cpu))),
			fit(StyleNormal.Render("MEM "+mem+memPct)),
			fit(StyleNormal.Render("NET "+net)),
		)
	} else if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("selecione um container"))
	}
	lines = append(lines, StyleMuted.Render("g cicla métrica"))
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// renderMetricSparkline: maxHint>0 scales against that ceiling; 0 = relative to window max.
// renderMetricSparkline usa o Braille do resto do app: cada célula carrega duas
// amostras, então mostra o dobro de histórico das barras de bloco que havia
// aqui — e o vocabulário fica igual ao do dashboard.
func renderMetricSparkline(hist []float64, width int, maxHint float64) string {
	cells := maxInt(4, width)
	if len(hist) == 0 {
		return brailleSpark(nil, cells)
	}
	// NET não tem teto fixo: normaliza pelo pico da própria janela.
	maxV := maxHint
	if maxV <= 0 {
		for _, v := range hist {
			if v > maxV {
				maxV = v
			}
		}
		if maxV <= 0 {
			maxV = 1
		}
	}
	pct := make([]float64, 0, len(hist))
	for _, v := range hist {
		pct = append(pct, v/maxV*100)
	}
	return brailleSpark(pct, cells)
}

func formatNetKB(kb float64) string {
	if kb < 1024 {
		return fmt.Sprintf("%.0f KB", kb)
	}
	return fmt.Sprintf("%.2f MB", kb/1024)
}

func parseDockerStatsSample(stats string) (cpu, mem, net float64) {
	for _, line := range strings.Split(stats, "\n") {
		switch {
		case strings.Contains(line, "CPU"):
			cpu = firstFloatIn(line)
		case strings.Contains(line, "Memory") || strings.Contains(line, "MEM"):
			// prefer MemPerc inside (...)
			if i := strings.LastIndex(line, "("); i >= 0 {
				mem = firstFloatIn(line[i:])
			} else {
				mem = firstFloatIn(line)
			}
		case strings.Contains(line, "Net"):
			rest := line
			if i := strings.Index(line, ":"); i >= 0 {
				rest = line[i+1:]
			}
			for _, p := range strings.Split(rest, "/") {
				net += parseDockerBytesToKB(p)
			}
		}
	}
	return
}

func firstFloatIn(s string) float64 {
	fields := strings.Fields(strings.ReplaceAll(s, "%", " "))
	for _, f := range fields {
		f = strings.Trim(f, "():,")
		var v float64
		if _, err := fmt.Sscanf(f, "%f", &v); err == nil {
			return v
		}
	}
	return 0
}

func parseDockerBytesToKB(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "net i/o:")
	s = strings.TrimSpace(s)
	var v float64
	var unit string
	if _, err := fmt.Sscanf(s, "%f%s", &v, &unit); err != nil {
		if _, err2 := fmt.Sscanf(s, "%f", &v); err2 != nil {
			return 0
		}
	}
	switch {
	case strings.HasPrefix(unit, "b") && !strings.HasPrefix(unit, "bi"):
		return v / 1024
	case strings.HasPrefix(unit, "kb") || strings.HasPrefix(unit, "kib"):
		return v
	case strings.HasPrefix(unit, "mb") || strings.HasPrefix(unit, "mib"):
		return v * 1024
	case strings.HasPrefix(unit, "gb") || strings.HasPrefix(unit, "gib"):
		return v * 1024 * 1024
	default:
		return v
	}
}

func (a *App) containerPreviewVolumeLines(maxLines, width int) []string {
	if len(a.containerPreviewVolumes) == 0 {
		return []string{StyleMuted.Render("(sem volumes)")}
	}
	lines := make([]string, 0, maxLines)
	for i, v := range a.containerPreviewVolumes {
		if i >= maxLines {
			break
		}
		v = sanitizeTerminalLine(v)
		lines = append(lines, StyleNormal.Render("● "+truncate(v, maxInt(1, width-2))))
	}
	return lines
}

func (a *App) containerPreviewPortLines(maxLines, width int) []string {
	c, ok := a.selectedContainer(a.currentProject())
	if !ok {
		return []string{StyleMuted.Render("selecione um container")}
	}
	ports := collectors.ParseContainerPortMappings(c.Ports)
	if len(ports) == 0 {
		return []string{StyleMuted.Render("(sem portas)"), StyleMuted.Render("enter · abrir")}
	}
	lines := make([]string, 0, maxLines)
	for i, p := range ports {
		if i >= maxLines-1 {
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("+%d  enter abrir", len(ports)-i)))
			break
		}
		lines = append(lines, StyleAccent.Render(truncate(fmt.Sprintf("● :%d → %d/%s", p.HostPort, p.ContainerPort, p.Proto), maxInt(1, width))))
	}
	return lines
}

func formatContainerMem(b int64) string {
	if b <= 0 {
		return "—"
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%dK", b/1024)
	}
	return fmt.Sprintf("%.0fM", float64(b)/(1024*1024))
}

func (a *App) filteredContainers(p *core.Project) []core.Container {
	var base []core.Container
	if a.containerShowAll {
		base = a.allProjectContainers()
	} else if p != nil {
		base = p.Containers
	}
	out := base
	if a.containerOnlyDocker {
		only := make([]core.Container, 0, len(out))
		for _, c := range out {
			if containerIsDockerInstance(c) {
				only = append(only, c)
			}
		}
		out = only
	}
	if a.containerFilter == "" {
		return out
	}
	f := strings.ToLower(a.containerFilter)
	filtered := make([]core.Container, 0, len(out))
	for _, c := range out {
		proj := strings.ToLower(a.containerProjectLabel(c))
		if strings.Contains(strings.ToLower(c.Name), f) ||
			strings.Contains(strings.ToLower(c.Image), f) ||
			strings.Contains(strings.ToLower(c.Ports), f) ||
			strings.Contains(proj, f) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// containerIsDockerInstance is a real docker ps row (not a synthetic compose "missing").
func containerIsDockerInstance(c core.Container) bool {
	return c.ID != "" && !strings.EqualFold(c.Status, "missing")
}

func (a *App) allProjectContainers() []core.Container {
	type row struct {
		proj string
		c    core.Container
	}
	rows := make([]row, 0)
	for _, p := range a.snapshot.Projects {
		for _, c := range p.Containers {
			cc := c
			// Always the project root — compose cwd may differ from p.Path.
			cc.ProjectPath = p.Path
			rows = append(rows, row{proj: p.Name, c: cc})
		}
	}
	for _, c := range a.snapshot.OrphanContainers {
		cc := c
		if cc.ProjectPath == "" {
			cc.ProjectPath = "—"
		}
		rows = append(rows, row{proj: "—", c: cc})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].proj != rows[j].proj {
			return rows[i].proj < rows[j].proj
		}
		return rows[i].c.Name < rows[j].c.Name
	})
	out := make([]core.Container, len(rows))
	for i, r := range rows {
		out[i] = r.c
	}
	return out
}

func (a *App) containerProjectLabel(c core.Container) string {
	if c.ProjectPath == "—" {
		return "órfão"
	}
	for _, p := range a.snapshot.Projects {
		for _, pc := range p.Containers {
			if pc.ID != "" && pc.ID == c.ID {
				return p.Name
			}
			if pc.ID == "" && c.ID == "" && pc.Name == c.Name && pathsMatch(p.Path, c.ProjectPath) {
				return p.Name
			}
		}
		if c.ProjectPath != "" && pathsMatch(p.Path, c.ProjectPath) {
			return p.Name
		}
	}
	if c.ProjectPath != "" && c.ProjectPath != "—" {
		return shortenPath(c.ProjectPath)
	}
	return "órfão"
}

func (a *App) renderContainerRow(c core.Container, selected bool) string {
	cols := a.containerColumns()
	wave, waveStyle, stateLabel, stateStyle := a.containerStateVisual(c)
	if cols.dot < containerWaveWidth {
		wave = statusWave(containerStateWaveKind(a, c), maxInt(2, cols.dot*2), a.animFrame)
	}

	nameStyle := StyleNormal.Bold(true)
	if !a.containerBelongsToOpenProject(c) {
		nameStyle = StyleNormal
	}
	// A faixa azul já marca a política; o ∞ no nome só confirma sem repintar.
	name := c.Name
	if containerRestartAlways(c) {
		name = "∞ " + name
	}

	cells := []dashCell{
		{text: wave, width: cols.dot, style: waveStyle},
		{text: stateLabel, width: cols.state, style: stateStyle},
		{text: a.containerProjectLabel(c), width: cols.project, style: a.containerProjectStyle(c)},
		{text: name, width: cols.name, style: nameStyle},
		{text: elideLeft(c.Image, maxInt(1, cols.image)), width: cols.image, style: StyleMuted},
		{text: containerPortsLabel(c), width: cols.ports, style: lipgloss.NewStyle().Foreground(ColorAccent)},
		{text: containerCPULabel(c), width: cols.cpu, style: StyleMuted, right: true},
		{text: formatContainerMem(c.Memory), width: cols.mem, style: StyleMuted, right: true},
		// Era o State cru ("running") sob o título UPTIME; agora é o tempo de
		// fato, tirado do Status do docker.
		{text: containerUptimeLabel(c), width: cols.uptime, style: StyleMuted, right: true},
	}
	return " " + renderCells(selected, cells)
}

// containerWaveCols são as COLUNAS DE PONTO da faixa de status: cada caractere
// Braille tem 2, então 10 pontos ocupam 5 caracteres no terminal. Com 2 pontos
// (um caractere só) não havia forma suficiente para separar seis estados.
const (
	containerWaveCols  = 10
	containerWaveWidth = containerWaveCols / 2 // largura em caracteres
)

// containerStateVisual devolve a faixa de status, a palavra e a cor.
//
// A FORMA diz o estado:
//
//	running     onda que sobe e desce, caminhando  (está trabalhando)
//	unhealthy   linha plana que respira baixo      (está no ar, mas mal)
//	restarting  linha baixa com um pico caminhando (algo atravessando)
//	paused      todas as colunas iguais, paradas   (congelado)
//	exited      todas rasteiras, paradas           (sem energia)
//
// A COR diz a gravidade: verde no ar, amarelo atenção, vermelho parado,
// cinza nunca subiu. E azul quando a política é `always`, que se sobrepõe.
func (a *App) containerStateVisual(c core.Container) (wave string, waveStyle lipgloss.Style, label string, labelStyle lipgloss.Style) {
	kind, label, style := a.containerStateKindLabel(c)
	wave = statusWave(kind, containerWaveCols, a.animFrame)
	waveStyle = style
	// `always` repinta só a FAIXA: a política é propriedade do container, o
	// estado é do momento. Pintar a palavra também apagaria o estado.
	if containerRestartAlways(c) {
		waveStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	}
	return wave, waveStyle, label, style
}

func clampWave(h int) int {
	if h < 1 {
		return 1
	}
	if h > 4 {
		return 4
	}
	return h
}

// containerProjectStyle pinta de amarelo o container que NÃO é do projeto
// aberto. Com Shift+A a lista mistura tudo que roda na máquina, e sem essa
// distinção não dá para saber em qual projeto você está prestes a agir.
func (a *App) containerProjectStyle(c core.Container) lipgloss.Style {
	if a.containerBelongsToOpenProject(c) {
		return StyleAccent
	}
	return StyleWarning
}

func (a *App) containerBelongsToOpenProject(c core.Container) bool {
	p := a.currentProject()
	if p == nil {
		return true
	}
	if c.ProjectPath == "" {
		// Órfão do docker: não é de projeto nenhum, muito menos deste.
		return false
	}
	return pathsMatch(p.Path, c.ProjectPath)
}

func containerStateWaveKind(a *App, c core.Container) string {
	kind, _, _ := a.containerStateKindLabel(c)
	return kind
}

func (a *App) containerStateKindLabel(c core.Container) (kind, label string, style lipgloss.Style) {
	// Ação pendente primeiro: é o retorno do comando que você acabou de dar.
	if k := a.containerActionKind(c.Name); k != "" {
		switch k {
		case "stop":
			return "stopping", "stopping", StyleWarning
		case "start":
			return "starting", "starting", StyleAccent
		case "restart":
			return "restarting", "restarting", StyleWarning
		case "pause":
			return "stopping", "pausing", StyleWarning
		case "unpause":
			return "starting", "resuming", StyleAccent
		case "always":
			return "restarting", "∞ always", StyleAccent
		case "no-always":
			return "restarting", "∞ off", StyleAccent
		default:
			return "restarting", truncate(k, 11), StyleWarning
		}
	}
	if strings.EqualFold(c.Health, "unhealthy") {
		return "unhealthy", "unhealthy", StyleWarning
	}
	switch containerStateKind(c) {
	case "running":
		return "running", "running", StyleRunning
	case "restarting":
		return "restarting", "restarting", StyleWarning
	case "paused":
		return "paused", "paused", StyleWarning
	case "created":
		return "created", "created", StyleMuted
	case "missing":
		return "paused", "missing", StyleWarning
	default:
		return "exited", "exited", StyleStopped
	}
}

// containerWave desenha a faixa de cada estado. As formas são disjuntas: em
// qualquer quadro dá para separar os três amarelos só pelo desenho.
func containerWave(kind string, cells, frame int) string {
	cols := cells * 2
	if frame < 0 {
		frame = -frame
	}
	switch kind {
	case "running":
		// Onda caminhando, meia altura: entre 1 e 4, centrada em 2.
		return brailleWave(cells, func(col int) int {
			return clampWave(2 + int(math.Round(1.6*math.Sin(float64(col+frame)/1.7))))
		})
	case "unhealthy":
		// Plana e uniforme, respirando junto: está no ar, mas sem saúde.
		// Fica acima das rasteiras (exited/created) e abaixo da travada.
		return brailleWave(cells, func(int) int { return 2 + (frame/4)%2 })
	case "restarting":
		// Base rasteira com um pico atravessando: algo em trânsito.
		peak := frame % cols
		return brailleWave(cells, func(col int) int {
			switch d := ((col-peak)%cols + cols) % cols; {
			case d == 0:
				return 4
			case d == 1 || d == cols-1:
				return 2
			default:
				return 1
			}
		})
	case "paused":
		// Todas no mesmo tamanho e imóveis: congelado.
		return brailleWave(cells, func(int) int { return 4 })
	case "created":
		return brailleWave(cells, func(int) int { return 1 })
	default: // exited
		return brailleWave(cells, func(int) int { return 1 })
	}
}

func containerCPULabel(c core.Container) string {
	if containerStateKind(c) != "running" {
		return emDash
	}
	return fmt.Sprintf("%.1f%%", c.CPU)
}

func containerPortsLabel(c core.Container) string {
	maps := collectors.ParseContainerPortMappings(c.Ports)
	if len(maps) == 0 {
		if strings.TrimSpace(c.Ports) == "" {
			return emDash
		}
		return c.Ports
	}
	seen := make(map[int]bool, len(maps))
	parts := make([]string, 0, 3)
	for _, m := range maps {
		if seen[m.HostPort] {
			continue
		}
		seen[m.HostPort] = true
		if len(parts) == 3 {
			parts = append(parts, fmt.Sprintf("+%d", len(maps)-3))
			break
		}
		parts = append(parts, fmt.Sprintf(":%d", m.HostPort))
	}
	return strings.Join(parts, " ")
}

// containerUptimeLabel extrai o tempo do Status do docker, que vem em vários
// formatos: "Up 2 days", "Up 2 days (healthy)", "Exited (0) 3 hours ago",
// "Restarting (1) 12 seconds ago", "About a minute".
func containerUptimeLabel(c core.Container) string {
	s := strings.TrimSpace(c.Status)
	if s == "" {
		return emDash
	}
	// "Exited (0) 3 hours ago" → "3 hours ago"
	if i := strings.Index(s, ") "); i > 0 && strings.Contains(s[:i], "(") {
		s = s[i+2:]
	}
	s = strings.TrimPrefix(s, "Up ")
	// "2 days (healthy)" → "2 days"
	if j := strings.Index(s, " ("); j > 0 {
		s = s[:j]
	}
	s = strings.TrimSuffix(s, " ago")
	s = strings.TrimPrefix(s, "About ")
	s = strings.TrimPrefix(s, "Less than ")
	s = strings.TrimSpace(s)
	for _, one := range []string{"a ", "an ", "A ", "An "} {
		if strings.HasPrefix(s, one) {
			s = "1 " + strings.TrimPrefix(s, one)
			break
		}
	}
	// "Created", "Paused" e afins não carregam duração nenhuma.
	if !strings.ContainsAny(s, "0123456789") {
		return emDash
	}
	return compactDuration(s)
}

// compactDuration: "2 days" → "2d", "12 seconds" → "12s". A coluna tem 9
// colunas; por extenso não cabe nem "3 minutes".
func compactDuration(s string) string {
	f := strings.Fields(s)
	if len(f) != 2 {
		return truncate(s, 9)
	}
	n := f[0]
	if n == "a" || n == "an" || n == "A" || n == "An" {
		n = "1"
	}
	short := map[string]string{
		"second": "s", "minute": "min", "hour": "h",
		"day": "d", "week": "sem", "month": "mes", "year": "a",
	}
	if u, ok := short[strings.TrimSuffix(strings.ToLower(f[1]), "s")]; ok {
		return n + u
	}
	return truncate(s, 9)
}

type containerCols struct {
	dot, state, project, name, image, ports, cpu, mem, uptime int
}

// containerColumns distribui a largura do painel. Antes derivava de a.width
// (terminal inteiro, com a sidebar), e o texto era dimensionado por uma conta
// e cortado por outra.
func (a *App) containerColumns() containerCols {
	tableWidth := a.containerTableWidth
	if tableWidth <= 0 {
		tableWidth = maxInt(38, a.width-8)
	}
	cols := containerCols{dot: containerWaveWidth, state: 12}
	if tableWidth < 92 {
		cols.state = 0 // a faixa já diz o estado; a palavra volta quando couber
	}
	if tableWidth < 64 {
		cols.dot = 3 // faixa curta: 3 células ainda mostram a forma
	}
	flexible := tableWidth - cols.dot - cols.state - 3
	if a.containerShowAll {
		cols.project = maxInt(10, flexible*16/100)
		flexible -= cols.project + 1
	}
	if tableWidth < 82 {
		cols.name = maxInt(12, flexible*45/100)
		cols.image = maxInt(10, flexible-cols.name-1)
		return cols
	}
	cols.cpu, cols.mem, cols.uptime = 6, 7, 9
	flexible -= cols.cpu + cols.mem + cols.uptime + 3
	cols.name = minInt(24, maxInt(12, flexible*30/100))
	cols.image = minInt(30, maxInt(12, flexible*32/100))
	cols.ports = flexible - cols.name - cols.image - 2
	if cols.ports < 10 {
		cols.ports = 0
		cols.name = maxInt(12, flexible*45/100)
		cols.image = maxInt(12, flexible-cols.name-1)
	}
	return cols
}

func (a *App) renderContainerHeader() string {
	cols := a.containerColumns()
	head := StyleMuted.Bold(true)
	cell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return head.Render(padRight(truncate(t, n), n))
	}
	rcell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return head.Render(padLeft(truncate(t, n), n))
	}
	return " " + joinNonEmpty(" ",
		cell("", cols.dot), cell("ESTADO", cols.state), cell("PROJETO", cols.project),
		cell("NOME", cols.name), cell("IMAGEM", cols.image), cell("PORTAS", cols.ports),
		rcell("CPU", cols.cpu), rcell("MEM", cols.mem), rcell("TEMPO", cols.uptime))
}

func compactContainerUptime(state string) string {
	// docker State field in our model is often the Status text ("Up 2 hours") from ps.
	s := strings.TrimSpace(state)
	if s == "" {
		return "—"
	}
	s = strings.TrimPrefix(s, "Up ")
	s = strings.TrimPrefix(s, "Exited ")
	return truncate(s, 10)
}

func containerRestartAlways(c core.Container) bool {
	return strings.EqualFold(strings.TrimSpace(c.Restart), "always")
}

func (a *App) containerListViewport() int {
	// Used by scroll helpers; approximate visible rows in new layout.
	v := a.projectPanelHeight()*45/100 - 4
	if v < 4 {
		return 4
	}
	return v
}

func (a *App) syncContainerScroll(count int) {
	viewport := a.containerListViewport()
	a.containerScroll = ensureVisible(a.tabCursor, a.containerScroll, viewport, count)
}

func (a *App) updateContainerCursor(delta int, p *core.Project) tea.Cmd {
	if a.containerSubview == containerSubviewDetail {
		a.containerDetailScrollBy(delta)
		return nil
	}

	containers := a.filteredContainers(p)
	if len(containers) == 0 {
		return nil
	}
	prev := a.tabCursor
	a.tabCursor = clampCursor(a.tabCursor+delta, len(containers))
	a.syncContainerScroll(len(containers))
	if a.tabCursor != prev {
		// Pin selection immediately so async docker refresh can't yank the cursor back.
		if c, ok := a.selectedContainer(p); ok {
			a.containerPreviewID = c.ID
		}
		return a.requestContainerPreview()
	}
	return nil
}

func (a *App) selectedContainer(p *core.Project) (core.Container, bool) {
	containers := a.filteredContainers(p)
	if a.tabCursor >= len(containers) {
		return core.Container{}, false
	}
	return containers[a.tabCursor], true
}

func (a *App) requireDockerContainer(c core.Container) bool {
	if c.ID != "" && !strings.EqualFold(c.Status, "missing") {
		return true
	}
	a.containerStatusMsg = c.Name + " ainda não criado — shift+u sobe o compose (ou remova o conflito no docker)"
	return false
}

func (a *App) containersCount(p *core.Project) int {
	return len(a.filteredContainers(p))
}

func (a *App) requestContainerPreview() tea.Cmd {
	p := a.currentProject()
	c, ok := a.selectedContainer(p)
	if !ok {
		a.containerPreviewID = ""
		a.containerPreviewLogs = ""
		a.containerPreviewStats = ""
		a.containerPreviewVolumes = nil
		return nil
	}
	if a.containerPreviewID != "" && a.containerPreviewID != c.ID {
		a.containerCPUHistory = nil
		a.containerMemHistory = nil
		a.containerNetHistory = nil
	}
	a.containerPreviewGen++
	gen := a.containerPreviewGen
	a.containerPreviewID = c.ID
	id := c.ID
	target := collectors.DockerExecTarget(c)
	return func() tea.Msg {
		logs, _ := collectors.DockerLogs(id, 30)
		stats, _ := collectors.DockerContainerStats(target)
		vols := collectors.DockerContainerVolumes(target)
		cpu, mem, net := parseDockerStatsSample(stats)
		return containerPreviewMsg{id: id, gen: gen, logs: logs, stats: stats, volumes: vols, cpu: cpu, mem: mem, net: net}
	}
}

func (a *App) handleContainerPreview(msg containerPreviewMsg) {
	if msg.gen != a.containerPreviewGen || msg.id != a.containerPreviewID {
		return
	}
	a.containerPreviewLogs = msg.logs
	a.containerPreviewStats = msg.stats
	a.containerPreviewVolumes = msg.volumes
	a.containerCPUHistory = appendMetricHistory(a.containerCPUHistory, msg.cpu)
	a.containerMemHistory = appendMetricHistory(a.containerMemHistory, msg.mem)
	a.containerNetHistory = appendMetricHistory(a.containerNetHistory, msg.net)
}

func appendMetricHistory(hist []float64, v float64) []float64 {
	hist = append(hist, v)
	if len(hist) > 40 {
		hist = hist[len(hist)-40:]
	}
	return hist
}

func (a *App) updateContainerFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.containerFilterOn = false
		a.containerFilterInput = a.containerFilter
		return a, nil
	case "enter":
		a.containerFilter = strings.TrimSpace(a.containerFilterInput)
		a.containerFilterOn = false
		a.tabCursor = 0
		a.containerScroll = 0
		return a, a.requestContainerPreview()
	case "backspace":
		if len(a.containerFilterInput) > 0 {
			r := []rune(a.containerFilterInput)
			a.containerFilterInput = string(r[:len(r)-1])
		}
		return a, nil
	default:
		if msg.Type == tea.KeyRunes {
			a.containerFilterInput += string(msg.Runes)
		}
		return a, nil
	}
}
