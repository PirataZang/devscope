package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/pkg/version"
	"github.com/mattn/go-runewidth"
)

// layoutOverhead é a moldura horizontal do dashboard: padding lateral de
// StyleDashboard (2+2) mais o filete decorativo da esquerda (2).
const layoutOverhead = 6

const emDash = "—"

func (a *App) renderDashboard() string {
	projects := filterNestedProjects(sortProjects(a.filteredProjects()))
	tableW := safeTableWidth(a.width)
	compact := a.dashboardCompact()

	gap := []string{""}
	if compact {
		gap = nil
	}

	sections := a.renderDashboardHeader(a.snapshot.HostMetrics, tableW)
	if !compact {
		sections = append(sections, "", a.renderHostStrip(a.snapshot.HostMetrics, tableW))
	}
	sections = append(sections, gap...)
	sections = append(sections, a.renderProjectsBar(projects, tableW))
	sections = append(sections, gap...)
	sections = append(sections, a.renderProjectsList(projects, tableW))

	body := strings.Join(sections, "\n")
	footer := a.renderDashboardFooter(tableW)
	// Empurra o rodapé para a última linha; StyleDashboard come 2 no padding.
	fill := 1
	if a.height > 0 {
		fill = maxInt(0, a.height-2-lipgloss.Height(body)-lipgloss.Height(footer))
	}
	// Só ocupa o vão que sobrou — o detalhe nunca tira linha da lista.
	if strip := a.renderSelectedStrip(projects, tableW); strip != nil && fill >= len(strip)+1 {
		body += strings.Repeat("\n", fill-len(strip)) + strings.Join(strip, "\n")
		fill = 0
	}
	body += strings.Repeat("\n", fill+1) + footer
	body = a.railed(body, projects)

	out := StyleDashboard.Render(body)
	// Garante que nada empurre o rodapé para fora da viewport.
	if a.height > 0 && lipgloss.Height(out) > a.height {
		lines := strings.Split(out, "\n")
		out = strings.Join(lines[:a.height], "\n")
	}
	return out
}

// ─── cabeçalho ──────────────────────────────────────────────────────────────

func (a *App) renderDashboardHeader(m core.HostMetrics, width int) []string {
	brand := StyleBrand.Render("DevScope v" + version.Version)
	subtitle := StyleSubtitle.Render("  Developer Command Center")
	clock := StyleClock.Render(a.now.Format("15:04:05"))

	// Em tela alta os medidores ganham a própria faixa logo abaixo; em compacta
	// voltam para o canto, que é o único lugar que sobra.
	right := clock
	if a.dashboardCompact() {
		right = renderMetricPills(m) + "   " + clock
	}
	if lipgloss.Width(brand)+lipgloss.Width(subtitle)+lipgloss.Width(right)+2 <= width {
		brand += subtitle
	}

	return []string{
		joinWithSpacer(brand, right, width),
		joinWithSpacer(
			StyleMuted.Render(truncate(hostSummary(m), maxInt(20, width-22))),
			a.renderScanAge(), width),
	}
}

// renderScanAge diz há quanto tempo os dados na tela foram colhidos — sem isso
// não dá para saber se a lista está fresca ou congelada.
func (a *App) renderScanAge() string {
	if a.snapshot.ScannedAt.IsZero() {
		return StyleMuted.Render(a.spinner() + " varrendo…")
	}
	when := relTime(a.snapshot.ScannedAt)
	text := "varrido há " + when
	if when == "agora" {
		text = "varrido agora"
	}
	if time.Since(a.snapshot.ScannedAt) > 2*time.Minute {
		return StyleWarning.Render(text)
	}
	return StyleMuted.Render(text)
}

// renderHostStrip é a faixa de métricas do host: valor primeiro (é o que se
// lê), tendência em Braille depois (é o que se sente). Cada célula do
// sparkline carrega duas amostras, então 8 colunas mostram ~5 s de histórico.
func (a *App) renderHostStrip(m core.HostMetrics, width int) string {
	meter := func(label string, pct float64, hist sparkHistory) string {
		st := metricUsageStyle(pct)
		return StyleMuted.Render(label+" ") +
			st.Render(fmt.Sprintf("%.0f%%", pct)) + " " +
			st.Render(brailleSpark(hist.samples, sparkCells))
	}
	left := "  " + strings.Join([]string{
		meter("CPU", m.CPUPercent, a.hostCPUHist),
		meter("RAM", m.MemoryPercent, a.hostRAMHist),
		meter("DISK", m.DiskPercent, a.hostDiskHist),
	}, StyleMuted.Render("     "))

	if alert := hostAlert(m); alert != "" {
		if lipgloss.Width(left)+lipgloss.Width(alert)+3 <= width {
			return joinWithSpacer(left, alert+" ", width)
		}
	}
	return padRightVisible(left, width)
}

// hostAlert só existe quando algo está prestes a doer — disco cheio derruba
// build, container e log ao mesmo tempo.
func hostAlert(m core.HostMetrics) string {
	switch {
	case m.DiskPercent >= 95:
		return StyleUnhealthy.Bold(true).Render("⚠ disco quase cheio")
	case m.MemoryPercent >= 92:
		return StyleUnhealthy.Bold(true).Render("⚠ memória no limite")
	case m.DiskPercent >= 85:
		return StyleWarning.Render("⚠ disco acima de 85%")
	default:
		return ""
	}
}

// hostSummary substitui a antiga caixa SYSTEM OVERVIEW por uma linha só.
func hostSummary(m core.HostMetrics) string {
	parts := make([]string, 0, 5)
	if m.OSInfo != "" {
		parts = append(parts, m.OSInfo)
	}
	parts = append(parts, "up "+formatUptime(m.Uptime))
	if f := strings.Fields(m.LoadAvg); len(f) > 0 {
		parts = append(parts, "load "+f[0])
	}
	parts = append(parts, fmt.Sprintf("docker %d", m.DockerRunning))
	if m.MemoryTotalMB > 0 {
		// O percentual está na faixa de medidores; aqui vai o absoluto, que é
		// o que diz se 51% são 8 GB ou 60 GB.
		parts = append(parts, formatRAM(m.MemoryUsedMB, m.MemoryTotalMB))
	}
	return strings.Join(parts, " · ")
}

func formatRAM(used, total int64) string {
	if total >= 1024 {
		return fmt.Sprintf("%.1f/%.1f GB", float64(used)/1024, float64(total)/1024)
	}
	return fmt.Sprintf("%d/%d MB", used, total)
}

// ─── barra da seção ─────────────────────────────────────────────────────────

// renderProjectsBar funde numa linha só o que antes eram três: o título
// "PROJECTS (n)", o rodapé Total/Running/Degraded/Stopped e a linha de filtro.
func (a *App) renderProjectsBar(projects []core.Project, width int) string {
	left := StyleSection.Render("PROJETOS") + StyleMuted.Render(" "+strconv.Itoa(len(projects)))
	if pills := renderStatusPills(projects, a.animFrame); pills != "" {
		left += "   " + pills
	}
	if a.filter != "" && !a.filterOn {
		left += StyleMuted.Render("   de " + strconv.Itoa(len(a.snapshot.Projects)))
	}
	rightW := width - lipgloss.Width(left) - 3
	if rightW < 12 {
		return truncateVisible(left, width)
	}
	return joinWithSpacer(left, a.renderProjectsFilterLine(rightW), width)
}

func renderStatusPills(projects []core.Project, frame int) string {
	running, stopped, degraded := countProjectStatuses(projects)
	unknown := len(projects) - running - stopped - degraded
	var parts []string
	add := func(st lipgloss.Style, level pulseLevel, label string, n int) {
		if n > 0 {
			parts = append(parts, st.Render(pulseGlyph(level, frame)+" "+strconv.Itoa(n))+
				StyleMuted.Render(" "+label))
		}
	}
	add(StyleRunning, pulseOK, "rodando", running)
	add(StyleWarning, pulseWarn, "degradado", degraded)
	add(StyleStopped, pulseBad, "parado", stopped)
	add(StyleMuted, pulseIdle, "sem status", unknown)
	return strings.Join(parts, "  ")
}

// railed desenha o filete vertical da esquerda: um colchete que abre no topo e
// fecha no rodapé, com um nó na linha da barra PROJETOS. Ecoa a borda da
// sidebar do projeto e dá moldura à tela sem voltar a encaixotá-la. A cor
// segue a saúde geral: verde tudo no ar, âmbar com algo degradado.
func (a *App) railed(body string, projects []core.Project) string {
	lines := strings.Split(body, "\n")
	if len(lines) < 2 {
		return body
	}
	st := lipgloss.NewStyle().Foreground(a.railColor(projects))
	knot := a.railKnotIndex(lines)

	out := make([]string, len(lines))
	for i, line := range lines {
		glyph := "│"
		switch {
		case i == 0:
			glyph = "╭"
		case i == len(lines)-1:
			glyph = "╰"
		case i == knot:
			glyph = "├"
		}
		out[i] = st.Render(glyph) + " " + line
	}
	return strings.Join(out, "\n")
}

// railKnotIndex acha a linha da barra PROJETOS para o filete marcar ali.
func (a *App) railKnotIndex(lines []string) int {
	for i, line := range lines {
		if strings.Contains(stripANSI(line), "PROJETOS") {
			return i
		}
	}
	return -1
}

func (a *App) railColor(projects []core.Project) lipgloss.Color {
	running, stopped, degraded := countProjectStatuses(projects)
	switch {
	case degraded > 0:
		return ColorWarning
	case running > 0 && stopped == 0:
		return ColorSuccess
	default:
		return ColorBorder
	}
}

func (a *App) renderProjectsFilterLine(width int) string {
	if a.filterOn {
		return StyleKey.Render("filter ") + StyleSelected.Render(a.filterInput+"▌")
	}
	if q := strings.TrimSpace(a.filter); q != "" {
		return StyleMuted.Render("filter ") + StyleNormal.Render(q) +
			StyleMuted.Render("  esc limpa")
	}
	hint := "/ filtrar · nome, path, branch ou framework"
	if width < runewidth.StringWidth(hint) {
		hint = "/ filtrar"
	}
	return StyleMuted.Render(truncate(hint, maxInt(9, width)))
}

// ─── tabela ─────────────────────────────────────────────────────────────────

func (a *App) renderProjectsList(projects []core.Project, tableW int) string {
	cols := tableColumns(tableW)
	viewport := a.dashboardProjectsViewport()

	lines := []string{
		renderTableHeader(cols),
		StyleMuted.Render(strings.Repeat("─", tableW)),
	}

	if len(projects) == 0 {
		return strings.Join(append(lines, a.renderProjectsEmpty(tableW)...), "\n")
	}

	start := clampCursor(a.dashboardScroll, len(projects))
	if len(projects) <= viewport {
		start = 0
	}
	end := minInt(start+viewport, len(projects))
	bar := scrollbarColumn(len(projects), viewport, start, end-start)

	for i := start; i < end; i++ {
		lines = append(lines, a.renderProjectRow(cols, projects[i], i == a.cursor)+bar[i-start])
	}
	return strings.Join(lines, "\n")
}

func (a *App) renderProjectsEmpty(width int) []string {
	if q := strings.TrimSpace(a.filter); q != "" {
		return []string{
			"",
			"  " + StyleWarning.Render("Nenhum projeto para ") + StyleNormal.Render("«"+q+"»"),
			"  " + StyleMuted.Render("ESC limpa o filtro · / edita a busca"),
		}
	}
	lines := []string{
		"",
		"  " + StyleKey.Render(animSpinner(a.animFrame)) + "  " + StyleNormal.Render("Varrendo projetos…"),
	}
	if paths := a.snapshot.ScanPaths; len(paths) > 0 {
		short := make([]string, 0, len(paths))
		for _, p := range paths {
			short = append(short, shortenPath(p))
		}
		lines = append(lines, "     "+StyleMuted.Render(truncate(strings.Join(short, " · "), maxInt(10, width-5))))
	}
	return lines
}

func (a *App) renderProjectRow(c tableCols, p core.Project, selected bool) string {
	glyph, dotStyle := statusDot(p.Status, a.animFrame)
	branch := emDash
	if p.Git != nil && p.Git.IsRepo && p.Git.Branch != "" {
		branch = p.Git.Branch
	}
	ctrs := emDash
	if p.ContainerCount > 0 {
		ctrs = strconv.Itoa(p.ContainerCount)
	}

	row := renderCells(selected, []dashCell{
		{text: glyph, width: c.dot, style: dotStyle},
		{text: p.Name, width: c.name, style: StyleNormal.Bold(true)},
		{text: stackLabel(p), width: c.stack, style: stackStyle(p.Framework.Name)},
		{text: branch, width: c.branch, style: lipgloss.NewStyle().Foreground(ColorAccent)},
		{text: ctrs, width: c.ctrs, style: StyleMuted, right: true},
		{text: endpointLabel(p), width: c.ports, style: StyleMuted},
		{text: commitAge(p), width: c.commit, style: StyleMuted, right: true},
		// A cauda do caminho é a parte que identifica; corta pela esquerda.
		{text: elideLeft(shortenPath(p.Path), c.path), width: c.path, style: StyleMuted},
	})

	if selected {
		return StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
	}
	return "  " + row
}

func renderTableHeader(c tableCols) string {
	head := StyleMuted.Bold(true)
	return "  " + renderCells(false, []dashCell{
		{text: "", width: c.dot},
		{text: "NOME", width: c.name, style: head},
		{text: "STACK", width: c.stack, style: head},
		{text: "BRANCH", width: c.branch, style: head},
		{text: "CTR", width: c.ctrs, style: head, right: true},
		{text: "PORTAS", width: c.ports, style: head},
		{text: "COMMIT", width: c.commit, style: head, right: true},
		{text: "CAMINHO", width: c.path, style: head},
	})
}

// dashCell é uma célula de largura fixa da tabela de projetos.
type dashCell struct {
	text  string
	width int
	style lipgloss.Style
	right bool
}

// renderCells desenha a linha inteira com um único fundo quando selecionada —
// estilizar célula a célula deixava o realce serrilhado.
func renderCells(selected bool, cells []dashCell) string {
	parts := make([]string, 0, len(cells))
	for _, cell := range cells {
		if cell.width <= 0 {
			continue
		}
		st := cell.style
		if selected {
			st = st.Background(ColorSelBg).Bold(true)
		}
		txt := truncate(cell.text, cell.width)
		if pad := cell.width - runewidth.StringWidth(txt); pad > 0 {
			if cell.right {
				txt = strings.Repeat(" ", pad) + txt
			} else {
				txt += strings.Repeat(" ", pad)
			}
		}
		parts = append(parts, st.Render(txt))
	}
	sep := " "
	if selected {
		sep = lipgloss.NewStyle().Background(ColorSelBg).Render(" ")
	}
	return strings.Join(parts, sep)
}

// ─── detalhe do selecionado ─────────────────────────────────────────────────

// renderSelectedStrip preenche o vão entre a lista e o rodapé com o projeto sob
// o cursor. Em tela alta esse espaço ficava vazio.
func (a *App) renderSelectedStrip(projects []core.Project, width int) []string {
	if a.cursor < 0 || a.cursor >= len(projects) {
		return nil
	}
	p := projects[a.cursor]
	glyph, dotStyle := statusDot(p.Status, a.animFrame)

	facts := make([]string, 0, 5)
	if p.Framework.Name != "" {
		facts = append(facts, stackStyle(p.Framework.Name).Render(p.Framework.Name))
	}
	if p.Git != nil && p.Git.IsRepo && p.Git.Branch != "" {
		branch := p.Git.Branch
		if p.Git.Ahead > 0 {
			branch += fmt.Sprintf(" ↑%d", p.Git.Ahead)
		}
		if p.Git.Behind > 0 {
			branch += fmt.Sprintf(" ↓%d", p.Git.Behind)
		}
		facts = append(facts, lipgloss.NewStyle().Foreground(ColorAccent).Render(branch))
		if n := p.Git.Modified + p.Git.Staged + p.Git.Untracked; n > 0 {
			facts = append(facts, StyleWarning.Render(fmt.Sprintf("%d alterados", n)))
		}
	}
	if p.ContainerCount > 0 {
		facts = append(facts, StyleNormal.Render(fmt.Sprintf("%d containers", p.ContainerCount)))
	}
	if e := endpointLabel(p); e != emDash {
		facts = append(facts, StyleNormal.Render(e))
	}

	commit := ""
	if p.Git != nil && strings.TrimSpace(p.Git.LastCommitMsg) != "" {
		commit = StyleMuted.Render(truncate(p.Git.LastCommitMsg, maxInt(12, width/3)) + "  " + commitAge(p))
	}
	details := strings.Join(facts, StyleMuted.Render(" · "))
	if lipgloss.Width(details)+lipgloss.Width(commit)+2 > width {
		commit = ""
	}

	return []string{
		StyleMuted.Render(strings.Repeat("─", width)),
		truncateVisible(dotStyle.Render(glyph)+" "+StyleNormal.Bold(true).Render(p.Name)+
			StyleMuted.Render("   "+shortenPath(p.Path)), width),
		joinWithSpacer(truncateVisible(details, width), commit, width),
	}
}

// ─── colunas ────────────────────────────────────────────────────────────────

type tableCols struct {
	dot, name, stack, branch, ctrs, ports, commit, path int
	total                                               int
}

// tableColumns liga as colunas opcionais por faixa de largura: primeiro STACK,
// depois COMMIT, depois PORTAS — sempre preservando NOME e CAMINHO legíveis.
func tableColumns(tableW int) tableCols {
	const (
		minName       = 16
		maxName       = 28
		minPath       = 14
		goodPath      = 40
		maxBranchGrow = 8
	)
	c := tableCols{dot: projectWaveWidth, branch: 18, ctrs: 3, total: tableW}

	// 2 colunas p/ a barra de seleção, 1 espaço + 1 p/ a scrollbar.
	avail := maxInt(24, tableW-4)
	if avail < 70 {
		c.branch = maxInt(9, avail/5)
	}
	// 4 gaps entre as 5 colunas sempre visíveis (dot, nome, branch, ctr, path).
	flex := avail - c.dot - c.branch - c.ctrs - 4

	// keep = o que precisa sobrar para NOME + CAMINHO depois de ligar a coluna.
	for _, opt := range []struct {
		w, keep int
		dst     *int
	}{{9, 36, &c.stack}, {6, 32, &c.commit}, {19, 44, &c.ports}} {
		if flex-(opt.w+1) >= opt.keep {
			*opt.dst = opt.w
			flex -= opt.w + 1
		}
	}

	// NOME tem teto: acima de ~28 colunas ele só acumula espaço em branco, e
	// quem ainda precisa de largura é BRANCH (feature/…) e CAMINHO.
	c.name = minInt(maxName, maxInt(minName, flex*48/100))
	grow := maxInt(0, minInt(maxBranchGrow, flex-c.name-goodPath))
	c.branch += grow
	c.path = maxInt(minPath, flex-c.name-grow)
	return c
}

func safeTableWidth(termWidth int) int {
	if termWidth <= 0 {
		return 78
	}
	return maxInt(40, termWidth-layoutOverhead)
}

// ─── scrollbar ──────────────────────────────────────────────────────────────

// scrollbarColumn devolve uma coluna de `rows` células — polegar + trilho —
// no lugar das antigas linhas "↑ N acima" / "↓ N abaixo", que custavam 2 linhas.
func scrollbarColumn(total, viewport, start, rows int) []string {
	out := make([]string, rows)
	if rows <= 0 {
		return out
	}
	if total <= viewport {
		for i := range out {
			out[i] = " "
		}
		return out
	}
	thumb := maxInt(1, rows*viewport/total)
	pos := 0
	if maxStart := total - viewport; maxStart > 0 {
		pos = start * (rows - thumb) / maxStart
	}
	track := StyleMuted.Render("│")
	grip := lipgloss.NewStyle().Foreground(ColorPrimary).Render("█")
	for i := range out {
		if i >= pos && i < pos+thumb {
			out[i] = grip
		} else {
			out[i] = track
		}
	}
	return out
}

// ─── rodapé ─────────────────────────────────────────────────────────────────

func (a *App) renderDashboardFooter(width int) string {
	inner := maxInt(10, width-2) // StyleStatusBar tem Padding(0, 1)
	if a.filterOn {
		return StyleStatusBar.Width(width).Render(fitKeybinds(inner,
			[2]string{"digite", "filtrar"},
			[2]string{"ENTER", "aplicar"},
			[2]string{"ESC", "limpar"},
		))
	}
	// Ajuda e saída ficam ancoradas à direita: são o que não pode sumir quando
	// a barra encolhe.
	tail := fitKeybinds(inner, [2]string{"?", "ajuda"}, [2]string{"q", "sair"})
	head := fitKeybinds(inner-lipgloss.Width(tail)-3,
		[2]string{"↑↓", "navegar"},
		[2]string{"ENTER", "git"},
		[2]string{"c", "containers"},
		[2]string{"/", "filtrar"},
		[2]string{"^p", "fuzzy"},
		[2]string{"S-E", "terminal"},
		[2]string{"S-O", "opencode"},
		[2]string{"r", "refresh"},
		[2]string{"T", "tema"},
		[2]string{"^t", "relax"},
	)
	if head == "" {
		return StyleStatusBar.Width(width).Render(tail)
	}
	return StyleStatusBar.Width(width).Render(joinWithSpacer(head, tail, inner))
}

// fitKeybindsWrap distribui os atalhos em até maxLines linhas. Numa barra
// estreita, encurtar a lista esconde comando; quebrar em duas linhas mostra
// todos sem estourar a largura.
func fitKeybindsWrap(width, maxLines int, items ...[2]string) string {
	if maxLines < 1 {
		maxLines = 1
	}
	var lines []string
	rest := items
	for len(rest) > 0 && len(lines) < maxLines {
		line, used := fitKeybindsCount(width, rest...)
		if used == 0 {
			break
		}
		lines = append(lines, line)
		rest = rest[used:]
	}
	return strings.Join(lines, "\n")
}

// fitKeybinds encaixa quantos atalhos couberem — em terminal estreito a barra
// encurta em vez de estourar a largura.
func fitKeybinds(width int, items ...[2]string) string {
	line, _ := fitKeybindsCount(width, items...)
	return line
}

func fitKeybindsCount(width int, items ...[2]string) (string, int) {
	var b strings.Builder
	used, n := 0, 0
	for _, it := range items {
		need := runewidth.StringWidth(it[0]) + 1 + runewidth.StringWidth(it[1])
		if used > 0 {
			need += 3
		}
		if used+need > width {
			break
		}
		if used > 0 {
			b.WriteString(StyleMuted.Render(" · "))
		}
		b.WriteString(renderKeybind(it[0], it[1]))
		used += need
		n++
	}
	return b.String(), n
}

// ─── células ────────────────────────────────────────────────────────────────

// projectWaveCols são as COLUNAS DE PONTO da faixa de status na lista de
// projetos: 7 pontos = 4 caracteres Braille (a última metade fica vazia).
// Um caractere só não dava forma suficiente para separar os quatro estados.
const (
	projectWaveCols  = 7
	projectWaveWidth = (projectWaveCols + 1) / 2
)

// statusDot devolve a faixa de status do projeto. A forma diz o estado, a cor
// a gravidade — mesmo vocabulário da tela de containers.
func statusDot(s core.ProjectStatus, frame int) (string, lipgloss.Style) {
	kind, style := statusWaveKind(s)
	return statusWave(kind, projectWaveCols, frame), style
}

func statusWaveKind(s core.ProjectStatus) (string, lipgloss.Style) {
	switch s {
	case core.StatusRunning:
		return "running", StyleRunning
	case core.StatusDegraded:
		return "unhealthy", StyleWarning
	case core.StatusStopped:
		return "exited", StyleStopped
	default:
		return "idle", StyleMuted
	}
}

func statusLevel(s core.ProjectStatus) (pulseLevel, lipgloss.Style) {
	switch s {
	case core.StatusRunning:
		return pulseOK, StyleRunning
	case core.StatusDegraded:
		return pulseWarn, StyleWarning
	case core.StatusStopped:
		return pulseBad, StyleStopped
	default:
		return pulseIdle, StyleMuted
	}
}

func stackLabel(p core.Project) string {
	if p.Framework.Name == "" {
		return emDash
	}
	return p.Framework.Name
}

func stackStyle(name string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(stackColor(name))
}

func stackColor(name string) lipgloss.Color {
	switch strings.ToLower(name) {
	case "go":
		return ColorGo
	case "docker":
		return ColorDocker
	case "vue", "nuxt":
		return ColorVue
	case "laravel":
		return ColorLaravel
	case "node.js", "nestjs", "next.js", "react":
		return ColorNode
	case "php":
		return ColorPHP
	case "python", "django":
		return ColorPython
	case "rust":
		return ColorRust
	case "maven", "gradle":
		return ColorWarning
	default:
		return ColorSubtext
	}
}

// endpointLabel prefere o domínio Nginx à porta crua — é o que se digita no
// navegador.
func endpointLabel(p core.Project) string {
	if len(p.Domains) > 0 {
		host := p.Domains[0].Host
		if extra := len(p.Domains) - 1; extra > 0 {
			host += fmt.Sprintf(" +%d", extra)
		}
		return host
	}
	if ports := collectors.FormatPortsShort(p.Ports, 3); ports != "-" {
		return ports
	}
	return emDash
}

func commitAge(p core.Project) string {
	if p.Git == nil || p.Git.LastCommitDate.IsZero() {
		return emDash
	}
	return relTime(p.Git.LastCommitDate)
}

// elideLeft corta pela esquerda: em ~/Área de Trabalho/digiliza-checkout o que
// identifica o projeto é a cauda, não o prefixo repetido em toda linha.
func elideLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := runewidth.StringWidth(s)
	if w <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return "…" + runewidth.TruncateLeft(s, w-(width-1), "")
}

func truncateVisible(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return padRightVisible(s, width)
}

// ─── layout / estado ────────────────────────────────────────────────────────

func (a *App) dashboardCompact() bool {
	return a.height > 0 && a.height < 28
}

func (a *App) dashboardProjectsViewport() int {
	h := a.height
	if h <= 0 {
		return 6
	}
	// Chrome fixo: padding vertical (2) + cabeçalho (2) + faixa do host (2) +
	// barra da seção (1) + cabeçalho da tabela (1) + régua (1) + rodapé (1) +
	// 3 linhas em branco.
	reserved := 13
	if a.dashboardCompact() {
		reserved = 9 // sem faixa do host: os medidores voltam para o canto
	}
	// Em tela alta o detalhe do selecionado é fixo: layout que muda de forma
	// conforme a lista cresce lê pior do que 3 linhas sempre reservadas.
	if h >= 32 {
		reserved += 3
	}
	if v := h - reserved; v > 3 {
		return v
	}
	return 3
}

func (a *App) syncDashboardScroll(projectCount int) {
	viewport := a.dashboardProjectsViewport()
	a.dashboardScroll = ensureVisible(a.cursor, a.dashboardScroll, viewport, projectCount)
}

func countProjectStatuses(projects []core.Project) (running, stopped, degraded int) {
	for _, p := range projects {
		switch p.Status {
		case core.StatusRunning:
			running++
		case core.StatusDegraded:
			degraded++
		case core.StatusStopped:
			stopped++
		}
	}
	return running, stopped, degraded
}

func projectStatusStyle(s core.ProjectStatus) lipgloss.Style {
	switch s {
	case core.StatusRunning:
		return StyleRunning
	case core.StatusStopped:
		return StyleStopped
	case core.StatusDegraded:
		return StyleWarning
	default:
		return StyleMuted
	}
}

// statusLabel é o rótulo usado fora da tabela (sidebar, barra de módulo). Mesmo
// vocabulário do dashboard: o pulso em Braille virava "⋯ Stop" em fonte pequena.
func statusLabel(s core.ProjectStatus, frame int) string {
	glyph, _ := statusDot(s, frame)
	switch s {
	case core.StatusRunning:
		return glyph + " Running"
	case core.StatusStopped:
		return glyph + " Stopped"
	case core.StatusDegraded:
		return glyph + " Degraded"
	default:
		return glyph + " Unknown"
	}
}

// barSolid é a barra de progresso das telas novas: blocos cheios leem em
// qualquer fonte, ao contrário da Braille de meterBar, que some em 8 colunas.
// meterBar segue intocada — Git e Docker dependem dela.
func barSolid(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	pct = math.Max(0, math.Min(100, pct))
	on := int(math.Round(pct / 100 * float64(width)))
	if on == 0 && pct > 0 {
		on = 1 // barra vazia com valor > 0 parece bug, não "quase nada"
	}
	st := StyleRunning
	switch {
	case pct >= 80:
		st = StyleUnhealthy
	case pct >= 50:
		st = StyleWarning
	}
	return st.Render(strings.Repeat("█", on)) + StyleMuted.Render(strings.Repeat("░", width-on))
}

func filterNestedProjects(projects []core.Project) []core.Project {
	if len(projects) < 2 {
		return projects
	}
	projects = dedupProjectsByPath(projects)
	var result []core.Project
	for _, p := range projects {
		if isNestedProject(p.Path, projects) {
			continue
		}
		result = append(result, p)
	}
	return result
}

// dedupProjectsByPath remove entradas com o mesmo path exato (ex: o mesmo
// projeto encontrado pela varredura normal e de novo via containers Docker
// parados), mantendo a primeira ocorrencia.
func dedupProjectsByPath(projects []core.Project) []core.Project {
	seen := make(map[string]bool, len(projects))
	result := make([]core.Project, 0, len(projects))
	for _, p := range projects {
		key := filepath.Clean(p.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, p)
	}
	return result
}

func isNestedProject(path string, projects []core.Project) bool {
	path = filepath.Clean(path)
	for _, other := range projects {
		otherPath := filepath.Clean(other.Path)
		if path == otherPath {
			continue
		}
		if strings.HasPrefix(path, otherPath+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// joinWithSpacer nunca estoura totalWidth: quando os dois lados não cabem
// juntos (nome de projeto comprido + vários chips de status), encolhe a
// esquerda em vez de deixar a linha mais larga que a tela.
func joinWithSpacer(left, right string, totalWidth int) string {
	rightW := lipgloss.Width(right)
	if rightW > totalWidth {
		right = truncateVisible(right, totalWidth)
		rightW = lipgloss.Width(right)
	}
	spacer := totalWidth - lipgloss.Width(left) - rightW
	if spacer < 1 {
		left = truncateVisible(left, maxInt(0, totalWidth-rightW-1))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", spacer), right)
}
