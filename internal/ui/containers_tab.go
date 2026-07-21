package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type containerSubview int

const (
	containerSubviewList containerSubview = iota
	containerSubviewDetail
	containerSubviewShellReturn
)

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
	a.containerPreviewID = ""
	a.containerPreviewLogs = ""
	a.containerPreviewStats = ""
	a.containerPreviewVolumes = nil
	a.containerCPUHistory = nil
	a.containerMemHistory = nil
	a.containerNetHistory = nil
}

func (a *App) renderContainersTab(p *core.Project) string {
	switch a.containerSubview {
	case containerSubviewDetail:
		return a.renderContainerDetail(p)
	case containerSubviewShellReturn:
		return renderShellReturnMessage(a.containerShellExitErr)
	default:
		return a.renderContainerList(p)
	}
}

func (a *App) dismissContainerShellReturn() tea.Cmd {
	a.containerSubview = containerSubviewList
	if a.containerShellExitErr != "" {
		a.containerStatusMsg = a.containerShellExitErr
	}
	a.containerShellExitErr = ""
	containers := a.filteredContainers(a.currentProject())
	if len(containers) > 0 {
		a.tabCursor = clampCursor(a.tabCursor, len(containers))
		a.syncContainerScroll(len(containers))
	}
	return tea.Batch(
		tea.ClearScreen,
		a.refreshDocker(),
		a.requestContainerPreview(),
	)
}

func (a *App) renderContainerList(p *core.Project) string {
	w := maxInt(60, a.width)
	h := maxInt(18, a.projectPanelHeight())

	if a.projectDockerLoading && len(p.Containers) == 0 {
		return renderApiTitledBox("CONTAINERS", fitExactLines([]string{StyleMuted.Render("Carregando containers...")}, h-2), w, h, true)
	}
	if len(p.Containers) == 0 {
		return renderApiTitledBox("CONTAINERS", fitExactLines([]string{
			StyleMuted.Render("Nenhum container vinculado a este projeto."),
			StyleMuted.Render("Vinculamos por docker-compose working_dir, config e volumes."),
		}, h-2), w, h, true)
	}

	containers := a.filteredContainers(p)
	running, stopped := 0, 0
	for _, c := range p.Containers {
		if strings.EqualFold(c.Status, "running") {
			running++
		} else {
			stopped++
		}
	}

	header := a.renderContainersHeader(p, running, stopped, w)
	stats := a.renderContainersStatsRow(p, w)
	search := a.renderContainersSearch(w)
	notif := a.renderContainersNotif()
	chromeH := lipgloss.Height(header) + lipgloss.Height(stats) + lipgloss.Height(search) + lipgloss.Height(notif) + 1
	bodyH := maxInt(10, h-chromeH-1)
	bottomH := maxInt(5, bodyH*28/100)
	tableH := maxInt(6, bodyH-bottomH)

	table := a.renderContainersTable(containers, w, tableH)
	actions := StyleMuted.Render("enter detalhe  s stop  r restart  p pause  d remove  e shell  l logs  g stats  / buscar")
	bottom := a.renderContainersBottom(w, bottomH)

	return lipgloss.JoinVertical(lipgloss.Left, header, stats, search, notif, table, actions, bottom)
}

func (a *App) renderContainersHeader(p *core.Project, running, stopped, width int) string {
	left := StyleSection.Render("CONTAINERS") + StyleMuted.Render("  "+shortenPath(p.Path))
	right := StyleHealthy.Render(fmt.Sprintf("%d running", running)) + StyleMuted.Render("  ") + StyleStopped.Render(fmt.Sprintf("%d stopped", stopped))
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderContainersStatsRow(p *core.Project, width int) string {
	host := a.snapshot.HostMetrics
	cpu := host.CPUPercent
	if p.Metrics.CPUPercent > 0 {
		cpu = p.Metrics.CPUPercent
	}
	ram := fmt.Sprintf("%.0f%%", host.MemoryPercent)
	if p.Metrics.MemoryMB > 0 {
		ram = fmt.Sprintf("%dM", p.Metrics.MemoryMB)
	}
	boxW := maxInt(10, width/4)
	cards := []struct{ title, value string }{
		{"CPU", fmt.Sprintf("%.1f%%", cpu)},
		{"RAM", ram},
		{"DISK", fmt.Sprintf("%.0f%%", host.DiskPercent)},
		{"NET", a.containerNetSummary()},
	}
	var parts []string
	for _, c := range cards {
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines([]string{StyleNormal.Render(truncate(c.value, boxW-4))}, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
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
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2) // header + separator
	if len(containers) == 0 {
		return renderApiTitledBox("LISTA", fitExactLines([]string{StyleMuted.Render("nenhum resultado")}, inner), width, height, true)
	}
	a.containerScroll = ensureVisible(a.tabCursor, a.containerScroll, viewport, len(containers))
	start := a.containerScroll
	end := minInt(start+viewport, len(containers))

	lines := []string{a.renderContainerHeader(), StyleMuted.Render(strings.Repeat("─", maxInt(20, width-6)))}
	if start > 0 {
		lines[1] = StyleMuted.Render(fmt.Sprintf("↑ %d  ", start) + strings.Repeat("─", maxInt(10, width-14)))
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
	return renderApiTitledBox(fmt.Sprintf("LISTA (%d)", len(containers)), fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderContainersBottom(width, height int) string {
	w1 := width * 42 / 100
	w2 := width * 30 / 100
	w3 := width - w1 - w2
	inner := maxInt(2, height-2)

	logs := a.containerPreviewLogLines(inner, w1-4)
	stats := a.containerPreviewStatLines(inner, w2-4)
	vols := a.containerPreviewVolumeLines(inner, w3-4)

	title := "LOGS"
	if a.containerPreviewID != "" {
		if c, ok := a.selectedContainer(a.currentProject()); ok {
			title = "LOGS · " + truncate(c.Name, 18)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderApiTitledBox(title, fitExactLines(logs, inner), w1, height, false),
		renderApiTitledBox(a.containerStatsTitle(), fitExactLines(stats, inner), w2, height, false),
		renderApiTitledBox("VOLUMES", fitExactLines(vols, inner), w3, height, false),
	)
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

	lines := make([]string, 0, maxLines)
	if showCPU {
		lines = append(lines, StyleMuted.Render("CPU ")+StyleAccent.Render(renderMetricSparkline(a.containerCPUHistory, sparkW, 100)))
	}
	if showMem {
		lines = append(lines, StyleMuted.Render("MEM ")+StyleHealthy.Render(renderMetricSparkline(a.containerMemHistory, sparkW, 100)))
	}
	if showNet {
		lines = append(lines, StyleMuted.Render("NET ")+StyleWarning.Render(renderMetricSparkline(a.containerNetHistory, sparkW, 0)))
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
			StyleNormal.Render(fmt.Sprintf("CPU %.1f%%", cpu)),
			StyleNormal.Render("MEM "+mem+memPct),
			StyleNormal.Render("NET "+net),
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
func renderMetricSparkline(hist []float64, width int, maxHint float64) string {
	n := maxInt(8, width)
	if len(hist) == 0 {
		return strings.Repeat("·", n)
	}
	if len(hist) > n {
		hist = hist[len(hist)-n:]
	}
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
	bars := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range hist {
		idx := int(v / maxV * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		b.WriteRune(bars[idx])
	}
	for i := len(hist); i < n; i++ {
		b.WriteRune('·')
	}
	return b.String()
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
		lines = append(lines, StyleNormal.Render("● "+truncate(v, width-2)))
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
	if p == nil {
		return nil
	}
	if a.containerFilter == "" {
		return p.Containers
	}
	f := strings.ToLower(a.containerFilter)
	var out []core.Container
	for _, c := range p.Containers {
		if strings.Contains(strings.ToLower(c.Name), f) ||
			strings.Contains(strings.ToLower(c.Image), f) ||
			strings.Contains(strings.ToLower(c.Ports), f) {
			out = append(out, c)
		}
	}
	return out
}

func (a *App) renderContainerRow(c core.Container, selected bool) string {
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	cols := a.containerColumns()
	gap := lipgloss.NewStyle().Width(1).Render("")
	cell := func(width int, text string) string {
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	}
	state := a.containerStateCell(c, selected)
	parts := []string{
		lipgloss.NewStyle().Width(1).Render(""),
		state,
		gap,
		cell(cols.name, c.Name),
		gap,
		cell(cols.image, c.Image),
	}
	if cols.ports > 0 {
		parts = append(parts, gap, cell(cols.ports, c.Ports))
	}
	if cols.cpu > 0 {
		parts = append(parts, gap, cell(cols.cpu, fmt.Sprintf("%.1f%%", c.CPU)))
	}
	if cols.mem > 0 {
		parts = append(parts, gap, cell(cols.mem, formatContainerMem(c.Memory)))
	}
	if cols.uptime > 0 {
		parts = append(parts, gap, cell(cols.uptime, compactContainerUptime(c.State)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

type containerCols struct {
	state, name, image, ports, cpu, mem, uptime int
}

func (a *App) containerColumns() containerCols {
	tableWidth := maxInt(38, a.width-8)
	cols := containerCols{state: 9}
	flexible := tableWidth - 1 - cols.state - 2
	if a.width < 90 {
		cols.name = flexible * 40 / 100
		cols.image = flexible - cols.name
		return cols
	}
	// wide: name image ports cpu mem uptime
	cols.cpu = 6
	cols.mem = 6
	cols.uptime = 8
	flexible -= cols.cpu + cols.mem + cols.uptime + 3
	cols.name = flexible * 28 / 100
	cols.image = flexible * 28 / 100
	cols.ports = flexible - cols.name - cols.image
	if cols.ports < 8 {
		cols.ports = 0
		cols.name = flexible * 40 / 100
		cols.image = flexible - cols.name
	}
	return cols
}

func (a *App) renderContainerHeader() string {
	cols := a.containerColumns()
	style := StyleTableHeader
	gap := lipgloss.NewStyle().Width(1).Render("")
	parts := []string{
		lipgloss.NewStyle().Width(1).Render(""),
		style.Width(cols.state).Render("STATE"),
		gap,
		style.Width(cols.name).Render("NAME"),
		gap,
		style.Width(cols.image).Render("IMAGE"),
	}
	if cols.ports > 0 {
		parts = append(parts, gap, style.Width(cols.ports).Render("PORTS"))
	}
	if cols.cpu > 0 {
		parts = append(parts, gap, style.Width(cols.cpu).Render("CPU"))
	}
	if cols.mem > 0 {
		parts = append(parts, gap, style.Width(cols.mem).Render("MEM"))
	}
	if cols.uptime > 0 {
		parts = append(parts, gap, style.Width(cols.uptime).Render("UPTIME"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
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

func (a *App) containerStateCell(c core.Container, selected bool) string {
	width := a.containerColumns().state
	if kind := a.containerActionKind(c.Name); kind != "" {
		var label string
		switch kind {
		case "stop":
			label = "◌ stop"
		case "start":
			label = "▶ start"
		case "restart":
			label = "⟳ rest"
		case "pause":
			label = "⏸ pause"
		case "unpause":
			label = "▶ resume"
		default:
			label = kind
		}
		s := StyleWarning.Bold(true)
		if selected {
			s = StyleWarning.Bold(true).Background(lipgloss.Color("#78350F"))
		}
		return s.Width(width).MaxWidth(width).Render(truncate(label, width))
	}
	if selected {
		return styleSelectedState(c.Status, width)
	}
	return containerStateStyled(c.Status, width)
}

func styleSelectedState(status string, width int) string {
	switch strings.ToLower(status) {
	case "running":
		return StyleSelected.Width(width).MaxWidth(width).Render("RUNNING")
	case "exited", "stopped":
		return StyleSelected.Width(width).MaxWidth(width).Render("EXITED")
	case "paused":
		return StyleSelected.Width(width).MaxWidth(width).Render("PAUSED")
	default:
		return StyleSelected.Width(width).MaxWidth(width).Render(strings.ToUpper(truncate(status, width)))
	}
}

func containerStateStyled(status string, width int) string {
	switch strings.ToLower(status) {
	case "running":
		return StyleRunning.Width(width).Render("running")
	case "exited", "stopped":
		return StyleStopped.Width(width).Render("exited")
	case "paused":
		return StyleWarning.Width(width).Render("paused")
	default:
		return StyleMuted.Width(width).Render(truncate(status, width))
	}
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
