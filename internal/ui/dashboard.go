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
	if apps := a.renderAppsStrip(tableW); apps != "" {
		footer = apps + "\n" + footer
	}
	// Empurra o rodapé para a última linha; StyleDashboard come 2 no padding.
	fill := 1
	if a.height > 0 {
		fill = maxInt(0, a.height-2-lipgloss.Height(body)-lipgloss.Height(footer))
	}
	// O painel do selecionado vem LOGO ABAIXO da lista, não colado no rodapé.
	//
	// Com doze projetos numa tela de sessenta linhas, ancorá-lo embaixo abria
	// vinte linhas de vazio NO MEIO da tela — e vazio no meio lê como layout
	// quebrado, enquanto o mesmo vazio embaixo lê como página que terminou.
	// Uma sobra só, num lugar só, acima do rodapé.
	if strip := a.renderSelectedStrip(projects, tableW); strip != nil && fill >= len(strip)+1 {
		body += "\n" + strings.Join(strip, "\n")
		fill -= len(strip)
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
		if len(hist.samples) == 0 {
			// Antes da primeira amostra o histórico é uma faixa em branco.
			return StyleMuted.Render(label+" ") + st.Render(fmt.Sprintf("%.0f%%", pct))
		}
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
		rule(tableW),
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

	row := renderCells(selected, []dashCell{
		{text: glyph, width: c.dot, style: dotStyle},
		{text: p.Name, width: c.name, style: StyleNormal.Bold(true)},
		{text: branch, width: c.branch, style: lipgloss.NewStyle().Foreground(ColorAccent)},
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
		{text: "BRANCH", width: c.branch, style: head},
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

// selectedStripHeight é o piso do painel do selecionado: régua + identidade +
// stack + runtime + git. Fixo porque dashboardProjectsViewport reserva as
// linhas antes de saber o que há no projeto sob o cursor.
const selectedStripHeight = 5

// selectedStripHeightFor é a ESCADA DE EXPANSÃO do painel (docs/DESIGN.md §1.6).
//
// Uma tela grande não é a pequena esticada. Em 200×60 a lista de doze projetos
// ocupava vinte e duas linhas e deixava trinta e oito em branco: a altura extra
// virava vão, não informação. Aqui ela vira detalhe do projeto sob o cursor —
// que é secundário (visível quando relevante), não terciário.
//
//	altura      o painel ganha
//	──────────  ────────────────────────────────────────────
//	< 32        nada — o painel não aparece, a lista leva tudo
//	>= 32       os três fatos: stack · runtime · git
//	>= 44       + os containers do projeto, um por linha
//	>= 54       + as probes de saúde
//
// A lista continua tendo prioridade: ela pega a altura primeiro, e o painel só
// cresce com o que sobrar depois dela.
func selectedStripHeightFor(termH, projectCount int) int {
	h := selectedStripHeight
	if termH < 32 {
		return 0
	}
	// A lista pede uma linha por projeto mais o cromo fixo; só o que passar
	// disso é que o painel pode tomar.
	slack := termH - projectCount - 15
	if termH >= 44 && slack >= selectedContainerRows+4 {
		h += selectedContainerRows + 1 // cabeçalho de seção + linhas
	}
	if termH >= 54 && slack >= selectedContainerRows+selectedProbeRows+8 {
		h += selectedProbeRows + 1
	}
	return h
}

const (
	selectedContainerRows = 4
	selectedProbeRows     = 3
)

// renderSelectedStrip é o painel do projeto sob o cursor — o "preview
// contextual". Ele é o outro lado da lista enxuta: as colunas STACK, CTR e
// COMMIT saíram da tabela e chegaram aqui, onde cabem inteiras.
//
// Três linhas rotuladas, sempre as mesmas três, nesta ordem:
//
//	stack    o que este projeto É        todas as stacks, não só a principal
//	runtime  o que está NO AR            containers, portas/domínios, compose
//	git      em que ponto ELE ESTÁ       branch, divergência, alterações, commit
//
// A altura é fixa de propósito: ausência vira "—" em vez de sumir com a linha.
// Painel que muda de tamanho conforme o cursor anda pisca a tela inteira e
// obriga o olho a reencontrar cada rótulo.
func (a *App) renderSelectedStrip(projects []core.Project, width int) []string {
	if a.cursor < 0 || a.cursor >= len(projects) {
		return nil
	}
	p := projects[a.cursor]
	glyph, dotStyle := statusDot(p.Status, a.animFrame)

	// Identidade: estado + nome + saúde à esquerda, caminho colado à direita.
	// O caminho corta pela esquerda — a cauda é o que identifica o projeto.
	left := dotStyle.Render(glyph) + " " + StyleNormal.Bold(true).Render(p.Name) +
		StyleMuted.Render("  ") + projectStatusStyle(p.Status).Render(projectStatusWord(p.Status))
	pathW := maxInt(12, width-lipgloss.Width(left)-3)
	head := joinWithSpacer(left, StyleMuted.Render(elideLeft(shortenPath(p.Path), pathW)), width)

	rows := []string{
		rule(width),
		head,
		factLine("stack", selectedStackFacts(p), width),
		factLine("runtime", selectedRuntimeFacts(p), width),
		factLine("git", selectedGitFacts(p, width), width),
	}
	// Degraus de expansão: só entram quando a altura sobra depois da lista.
	want := selectedStripHeightFor(a.height, len(projects))
	if want > len(rows) {
		rows = append(rows, a.selectedContainerBlock(p, width, want-len(rows))...)
	}
	if want > len(rows) {
		rows = append(rows, a.selectedProbeBlock(p, width, want-len(rows))...)
	}
	for len(rows) < want {
		rows = append(rows, "")
	}
	return rows
}

// selectedContainerBlock: os containers do projeto sob o cursor, no mesmo
// vocabulário da tela de Containers — quem aprendeu a ler lá lê aqui.
func (a *App) selectedContainerBlock(p core.Project, width, budget int) []string {
	if budget < 2 {
		return nil
	}
	rows := []string{"  " + sectionHeader("CONTAINERS", maxInt(10, width-2), ColorDocker)}
	if len(p.Containers) == 0 {
		return append(rows, "  "+StyleMuted.Render("nenhum container — shift+U sobe o compose"))
	}
	// A coluna do nome sai do nome MAIS LONGO, não de uma fração da largura.
	// Com width/3 numa tela de 140, quatro nomes de 28 colunas ficavam numa
	// coluna de 44 e abriam dezesseis colunas mortas antes do estado.
	nameW := 12
	for _, c := range p.Containers {
		if n := lipgloss.Width(sanitizeTerminalLine(c.Name)); n > nameW {
			nameW = n
		}
	}
	nameW = minInt(nameW, maxInt(12, width/3))
	for i, c := range p.Containers {
		if len(rows) >= budget {
			break
		}
		if i == budget-2 && len(p.Containers) > budget-1 {
			rows = append(rows, "  "+StyleMuted.Render(fmt.Sprintf("+%d", len(p.Containers)-i)))
			break
		}
		wave, waveStyle, label, labelStyle := a.containerStateVisual(c)
		rows = append(rows, "  "+waveStyle.Render(wave)+" "+
			StyleNormal.Render(padRight(truncate(sanitizeTerminalLine(c.Name), nameW), nameW))+"  "+
			labelStyle.Render(padRight(label, 11))+"  "+
			StyleMuted.Render(truncate(sanitizeTerminalLine(c.Image), maxInt(10, width/4))))
	}
	return rows
}

// selectedProbeBlock: o que o DevScope mediu do projeto — probes de saúde e
// workers. É a resposta a "está degradado por quê?" sem abrir o módulo.
func (a *App) selectedProbeBlock(p core.Project, width, budget int) []string {
	if budget < 2 {
		return nil
	}
	rows := []string{"  " + sectionHeader("SAÚDE", maxInt(10, width-2), ColorSuccess)}
	var lines []string
	for _, hc := range p.HealthChecks {
		glyph, st := healthGlyph(hc.Status, a.animFrame)
		lat := emDash
		if hc.LatencyMS > 0 {
			lat = fmt.Sprintf("%dms", hc.LatencyMS)
		}
		lines = append(lines, st.Render(glyph)+" "+
			StyleNormal.Render(truncate(hc.URL, maxInt(16, width/2)))+
			StyleMuted.Render("   "+lat))
	}
	for _, wk := range p.Workers {
		lines = append(lines, StyleMuted.Render("⚙ ")+StyleNormal.Render(truncate(wk.Name, maxInt(16, width/2)))+
			StyleMuted.Render("   "+wk.Status))
	}
	if len(lines) == 0 {
		lines = []string{StyleMuted.Render("nenhuma probe configurada neste projeto")}
	}
	for i, l := range lines {
		if len(rows) >= budget {
			break
		}
		if i == budget-2 && len(lines) > budget-1 {
			rows = append(rows, "  "+StyleMuted.Render(fmt.Sprintf("+%d", len(lines)-i)))
			break
		}
		rows = append(rows, "  "+l)
	}
	return rows
}

// selectedStackFacts lista TODAS as stacks detectadas, cada uma na sua cor.
// A tabela mostrava só a principal — um Laravel com Vue no front aparecia
// como "Laravel" e nada dizia que havia um front junto.
func selectedStackFacts(p core.Project) []string {
	fws := projectFrameworks(p)
	if len(fws) == 0 {
		return nil
	}
	out := make([]string, 0, len(fws))
	for _, fw := range fws {
		name := fw.Name
		if fw.Version != "" {
			name += " " + fw.Version
		}
		out = append(out, stackStyle(fw.Name).Render(name))
	}
	return out
}

// selectedRuntimeFacts é o que está no ar: containers, por onde se chega e como
// sobe.
func selectedRuntimeFacts(p core.Project) []string {
	var out []string
	if p.ContainerCount > 0 {
		word := "containers"
		if p.ContainerCount == 1 {
			word = "container"
		}
		out = append(out, StyleNormal.Render(fmt.Sprintf("%d %s", p.ContainerCount, word)))
	}
	if e := endpointLabel(p); e != emDash {
		out = append(out, StyleAccent.Render(e))
	}
	switch {
	case p.HasDockerCompose:
		out = append(out, StyleMuted.Render("compose"))
	case p.HasDockerfile:
		out = append(out, StyleMuted.Render("Dockerfile"))
	}
	return out
}

// selectedGitFacts: onde a branch está em relação ao remoto, o que há por
// commitar e qual foi o último commit — com a MENSAGEM, que a coluna COMMIT da
// tabela não tinha espaço para mostrar (só cabia a idade).
func selectedGitFacts(p core.Project, width int) []string {
	g := p.Git
	if g == nil || !g.IsRepo {
		return nil
	}
	var out []string
	if g.Branch != "" {
		branch := g.Branch
		if g.Ahead > 0 {
			branch += fmt.Sprintf(" ↑%d", g.Ahead)
		}
		if g.Behind > 0 {
			branch += fmt.Sprintf(" ↓%d", g.Behind)
		}
		out = append(out, lipgloss.NewStyle().Foreground(ColorAccent).Render(branch))
	}
	if n := g.Modified + g.Staged + g.Untracked; n > 0 {
		word := "alterados"
		if n == 1 {
			word = "alterado"
		}
		out = append(out, StyleWarning.Render(fmt.Sprintf("%d %s", n, word)))
	}
	if msg := strings.TrimSpace(g.LastCommitMsg); msg != "" {
		// O que sobra da linha depois dos outros fatos — a mensagem é o fato
		// mais longo e o único que pode encolher sem virar outra informação.
		room := maxInt(16, width-factLabelW-32)
		out = append(out, StyleMuted.Render(truncate(msg, room)+"  "+commitAge(p)))
	}
	return out
}

// projectStatusWord é a ÚNICA fonte da palavra de estado do app.
//
// Havia duas: statusLabel escrevia "Running/Degraded/Stopped/Unknown" na
// sidebar e no cabeçalho de módulo, e esta escrevia "rodando/degradado/parado"
// no dashboard. O mesmo projeto, na mesma tela, com dois nomes para o mesmo
// estado — e um deles em inglês, num app em português (§10).
//
// Sem o texto, dois estados dependeriam só da cor (§1).
func projectStatusWord(s core.ProjectStatus) string {
	switch s {
	case core.StatusRunning:
		return "rodando"
	case core.StatusStopped:
		return "parado"
	case core.StatusDegraded:
		return "degradado"
	default:
		return "sem status"
	}
}

// ─── colunas ────────────────────────────────────────────────────────────────

type tableCols struct {
	dot, name, branch, path int
	total                   int
}

// tableColumns distribui as colunas da lista. STACK, CTR, COMMIT e PORTAS
// saíram daqui: nenhuma das quatro muda a decisão de "em qual projeto eu entro
// agora" — elas respondem "o que é este projeto" e "por onde eu chego nele",
// que são perguntas do detalhe. As quatro vivem no painel do selecionado
// (renderSelectedStrip), onde cabem inteiras: TODAS as stacks em vez de só a
// principal, a mensagem do commit em vez de só a idade, e a lista de portas
// sem o "+14" de quando não cabia.
//
// Restam as quatro que DECIDEM, e só elas:
//
//	glifo    como está          é o que faz o olho parar na linha
//	NOME     qual é             é como se procura
//	BRANCH   em que estou       é o que muda entre duas janelas do mesmo projeto
//	CAMINHO  qual dos homônimos  três "digiliza" só se separam pelo caminho
//
// Sem as opcionais não há mais faixas de largura: as quatro entram sempre, e
// a largura que sobra vai para CAMINHO, que era quem mais apanhava.
func tableColumns(tableW int) tableCols {
	const (
		minName       = 16
		maxName       = 32
		minPath       = 14
		goodPath      = 40
		maxBranchGrow = 12
	)
	c := tableCols{dot: projectWaveWidth, branch: 18, total: tableW}

	// 2 colunas p/ a barra de seleção, 1 espaço + 1 p/ a scrollbar.
	avail := maxInt(24, tableW-4)
	if avail < 70 {
		c.branch = maxInt(9, avail/5)
	}
	// 3 gaps entre as 4 colunas.
	flex := avail - c.dot - c.branch - 3

	// NOME tem teto: acima de ~32 colunas ele só acumula espaço em branco, e
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
		[2]string{"ctrl+p", "fuzzy"},
		[2]string{"shift+E", "terminal"},
		[2]string{"ctrl+o", a.aiToolLabel()},
		[2]string{"shift+C", "preferências"},
		[2]string{"r", "atualizar"},
		[2]string{"shift+T", "tema"},
		[2]string{"ctrl+t", "relax"},
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
		b.WriteString(keyHint(it[0], it[1]))
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

// ─── layout / estado ────────────────────────────────────────────────────────

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
	// Em tela alta o painel do selecionado é fixo: layout que muda de forma
	// conforme a lista cresce lê pior do que linhas sempre reservadas. São as
	// 5 de renderSelectedStrip — régua, identidade e os três fatos rotulados.
	reserved += selectedStripHeightFor(h, len(a.snapshot.Projects))
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
	return glyph + " " + projectStatusWord(s)
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

// renderAppsStrip lista os programas externos cadastrados no config, com a
// tecla que os abre. Só aparece se houver algum: sem atalho cadastrado, uma
// faixa vazia dizendo "configure atalhos" seria ocupar linha à toa.
func (a *App) renderAppsStrip(width int) string {
	if a.cfg == nil {
		return ""
	}
	apps := a.cfg.AppShortcuts()
	if len(apps) == 0 {
		return ""
	}
	cells := make([][2]string, 0, len(apps))
	for _, app := range apps {
		cells = append(cells, [2]string{app.Key, app.Name})
	}
	inner := maxInt(10, width-2)
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(inner, 2, cells...))
}
