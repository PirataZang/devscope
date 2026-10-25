package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// ─── tela ────────────────────────────────────────────────────────────────────

func (a *App) renderCloudflareTab(p *core.Project) string {
	w, h := a.moduleSize()
	c := a.cfd
	lines := []string{a.renderModuleContext(p, w, "Cloudflare", a.cfdStatusText())}
	keys := [][2]string{{"r", "atualizar"}}
	switch {
	case !c.acctOK:
	case !c.token:
		keys = [][2]string{{"o", "criar o token"}, {"r", "conferir de novo"}}
	case c.acctErr != "":
	case c.open:
		keys = [][2]string{{"1 2 3", "tráfego · DNS · segurança"}, {"esc", "domínios"}, {"w", "abrir o site"}, {"o", "abrir na Cloudflare"}}
		if c.sub == cfdTraffic {
			keys = append(keys, [2]string{"←→", "7 / 30 dias"})
		} else {
			keys = append(keys, [2]string{"↑↓", "rolar"})
		}
		keys = append(keys, [2]string{"r", "atualizar"})
	default:
		keys = [][2]string{{"enter", "abrir o domínio"}, {"↑↓", "escolher"}, {"w", "abrir o site"}, {"o", "abrir na Cloudflare"}}
		if rows := a.cfdRows(); c.cursor < len(rows) && rows[c.cursor].link == nil {
			keys = append(keys, [2]string{"v", "vincular a este projeto"})
		}
		keys = append(keys, [2]string{"r", "atualizar"})
	}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2, keys...))
	room := h - len(lines) - lipgloss.Height(cmdBar)

	switch {
	case !c.acctOK:
		lines = append(lines, "", "  "+a.loadingText("lendo a conta da Cloudflare…"))
	case !c.token:
		lines = append(lines, "")
		lines = append(lines, cfdSetupLines(w)...)
	case c.acctErr != "":
		lines = append(lines, "", "  "+StyleUnhealthy.Render("⚠ "+truncate(c.acctErr, w-6)),
			"  "+StyleMuted.Render(truncate("confira o token com devscope cloudflare status", w-4)))
	case c.open:
		lines = append(lines, a.cfdDetailLines(w, room)...)
	default:
		lines = append(lines, a.cfdListLines(w, room)...)
	}
	for i := range lines {
		lines[i] = truncateVisible(lines[i], w)
	}
	return a.fillReleaseScreen(lines, cmdBar, h)
}

func (a *App) cfdStatusText() string {
	if a.cfd.status == "" {
		return ""
	}
	return StyleWarning.Render(a.cfd.status)
}

func cfdSetupLines(w int) []string {
	return []string{
		sectionHeader("CONECTAR A CLOUDFLARE", w, tabAccentColor(TabCloudflare)),
		"  " + releaseGuideLine(relGuide{false, "token", "obrigatório: nenhum token da Cloudflare nesta máquina"}, w-2),
		"",
		factLine("1. criar", []string{StyleNormal.Render("o abre a página de tokens · modelo \"Read all resources\"")}, w),
		factLine("2. salvar", []string{StyleKey.Render("devscope cloudflare login"), StyleMuted.Render("cole o token")}, w),
		factLine("ou", []string{StyleKey.Render("export CLOUDFLARE_API_TOKEN=…")}, w),
		"",
		"  " + StyleMuted.Render(truncate("só leitura: Zone, DNS, Analytics, SSL e Pages. O DevScope nunca muda nada na conta.", w-4)),
	}
}

func cfdAccent() lipgloss.Style { return lipgloss.NewStyle().Foreground(tabAccentColor(TabCloudflare)) }

// ─── lista de domínios ───────────────────────────────────────────────────────

// cfdListLines é a tela de entrada: os domínios da conta, como no painel da
// Cloudflare, com a semana de visitantes de cada um num relance.
func (a *App) cfdListLines(w, room int) []string {
	c := a.cfd
	rows := a.cfdRows()
	out := []string{""}
	if len(c.links) == 0 && !c.listBusy {
		out = append(out, "  "+StyleWarning.Render("◌ nenhum domínio da conta aparece neste projeto")+
			StyleMuted.Render(" — v vincula o selecionado"), "")
	}
	out = append(out, sectionHeader(panelTitle("DOMÍNIOS", fmt.Sprint(len(rows))), w, tabAccentColor(TabCloudflare)))
	if len(rows) == 0 {
		return append(out, "  "+StyleMuted.Render("o token não enxerga nenhum domínio: confira a permissão Zone Read"))
	}

	nameW := clampInt(w/4, 14, 30)
	wide := w >= 96
	head := "    " + padRight("DOMÍNIO", nameW) + " " + padRight("STATUS", 10) + " " + padRight("VISITANTES · 7 DIAS", 24)
	if wide {
		head += " " + padRight("PLANO", 8) + " PROJETO"
	}
	out = append(out, StyleMuted.Render(truncate(head, w)))
	for i, r := range rows {
		mark, nameSt := "    ", StyleNormal
		if i == c.cursor {
			mark, nameSt = "  "+StyleKey.Render("▸ "), StyleNormal.Bold(true)
		}
		status := StyleHealthy.Render(padRight("◆ ativo", 10))
		if r.zone.Status != "active" {
			status = StyleWarning.Render(padRight("◌ "+r.zone.Status, 10))
		}
		line := mark + nameSt.Render(padRight(truncate(r.zone.Name, nameW), nameW)) + " " + status + " " + a.cfdWeekCell(r.zone.ID)
		if wide {
			project := StyleMuted.Render(emDash)
			if r.link != nil {
				project = cfdAccent().Render("◆ este projeto")
			}
			line += " " + StyleMuted.Render(padRight(cfdPlan(r.zone.Plan), 8)) + " " + project
		}
		out = append(out, truncateVisible(line, w))
	}

	// O selecionado, por extenso: o que o enter vai abrir.
	r := rows[clampInt(c.cursor, 0, len(rows)-1)]
	from := StyleMuted.Render("nenhum — v liga este domínio ao projeto")
	if r.link != nil {
		from = StyleNormal.Render(r.link.From)
	}
	detail := []string{
		"",
		sectionHeader(r.zone.Name, w, ColorMuted),
		factLine("site", []string{lipgloss.NewStyle().Foreground(ColorPrimary).Underline(true).Render("https://" + r.host())}, w),
		factLine("vínculo", []string{from}, w),
		factLine("DNS em", []string{StyleMuted.Render(strings.Join(r.zone.NameServers, " · "))}, w),
	}
	if t, ok := c.week[r.zone.ID]; ok && len(t.Days) > 0 {
		detail = append(detail, "", sectionHeader(panelTitle("ÚLTIMOS 7 DIAS", "visitantes"), w, tabAccentColor(TabCloudflare)))
		detail = append(detail, a.cfdDayLines(t, w)...)
	}
	detail = append(detail, "", "  "+StyleMuted.Render(truncate("enter abre o domínio: tráfego, DNS e segurança · esc volta para cá", w-4)))
	if len(out)+len(detail) > room {
		detail = detail[:maxInt(0, room-len(out))]
	}
	return append(out, detail...)
}

// cfdWeekCell: minigráfico dos 7 dias, total e a tendência contra a semana
// anterior — 24 colunas.
func (a *App) cfdWeekCell(zoneID string) string {
	t, ok := a.cfd.week[zoneID]
	if !ok {
		if a.cfd.listBusy {
			return padRightVisible(a.loadingText("lendo…"), 24)
		}
		return StyleMuted.Render(padRight(emDash, 24))
	}
	vals := make([]int64, len(t.Days))
	for i, d := range t.Days {
		vals[i] = d.Visits
	}
	tot := t.Total()
	cell := cfdAccent().Render(padRight(cfdSpark(vals), 7)) + " " + StyleNormal.Bold(true).Render(padLeft(collectors.FormatHubCount(tot.Visits), 5))
	if t.HasPrev {
		cell += " " + cfdTrend(tot.Visits, t.Prev.Visits)
	}
	return padRightVisible(cell, 24)
}

func cfdPlan(p string) string {
	return strings.TrimSuffix(strings.TrimSuffix(firstNonEmpty(p, emDash), " Website"), " Plan")
}

// ─── domínio aberto ──────────────────────────────────────────────────────────

func (a *App) cfdDetailLines(w, room int) []string {
	c := a.cfd
	row := a.cfdOpenRow()
	status := StyleHealthy.Render(a.okPulse() + " ativo")
	if c.zone.Status != "active" {
		status = StyleWarning.Render("◌ " + c.zone.Status)
	}
	ident := "  " + StyleMuted.Render("‹ esc  ") + cfdAccent().Bold(true).Render(c.zone.Name) + StyleMuted.Render("  ·  ") + status +
		StyleMuted.Render("  ·  plano ") + StyleNormal.Render(cfdPlan(c.zone.Plan)) + StyleMuted.Render("  ·  ") +
		lipgloss.NewStyle().Foreground(ColorPrimary).Underline(true).Render("https://"+row.host())

	var tabs []string
	for i, name := range cfdSubNames {
		label := fmt.Sprintf(" %d %s ", i+1, strings.ToUpper(name))
		if i == c.sub {
			tabs = append(tabs, StyleSelected.Render(label))
		} else {
			tabs = append(tabs, StyleMuted.Render(label))
		}
	}
	right := ""
	if c.loading {
		right = a.loadingText("atualizando…")
	} else if row.link != nil {
		right = StyleMuted.Render("vínculo " + row.link.From)
	}
	strip := "  " + strings.Join(tabs, StyleMuted.Render("│"))
	out := []string{"", truncateVisible(ident, w), "", joinWithSpacer(strip, right, w)}
	room -= len(out)

	var body []string
	switch {
	case c.loading && len(c.traffic.Days) == 0 && c.err == "":
		body = []string{"", "  " + a.loadingText("lendo "+c.zone.Name+"…")}
	case c.sub == cfdTraffic:
		body = a.cfdTrafficLines(w, room)
	case c.sub == cfdDNS:
		body = a.cfdScroll(a.cfdDNSLines(w), room)
	default:
		body = a.cfdScroll(a.cfdSecurityLines(w), room)
	}
	return append(out, body...)
}

// cfdScroll mostra a janela da rolagem; a seta diz que há mais.
func (a *App) cfdScroll(lines []string, room int) []string {
	if len(lines) <= room || room < 3 {
		a.cfd.scroll = 0
		return lines
	}
	maxScroll := len(lines) - room + 1
	a.cfd.scroll = clampInt(a.cfd.scroll, 0, maxScroll)
	view := append([]string{}, lines[a.cfd.scroll:a.cfd.scroll+room-1]...)
	more := StyleMuted.Render(fmt.Sprintf("  ↓ mais %d linhas", len(lines)-a.cfd.scroll-room+1))
	if a.cfd.scroll >= maxScroll {
		more = StyleMuted.Render("  ↑ início")
	}
	return append(view, more)
}

// ─── 1 tráfego ───────────────────────────────────────────────────────────────

// cfdTrafficLines responde, nesta ordem: está recebendo visitas, mais ou menos
// que antes, está dando erro, de onde vêm (RESUMO); como foi o dia hora a hora
// (24 HORAS); a semana dia a dia (POR DIA); e o que mais se acessa e o que
// mais falha (PÁGINAS). Cada bloco só entra se couber inteiro o bastante.
func (a *App) cfdTrafficLines(w, room int) []string {
	c := a.cfd
	switch {
	case c.err != "" && len(c.traffic.Days) == 0:
		return []string{"", "  " + StyleUnhealthy.Render("⚠ "+truncate(c.err, w-6))}
	case len(c.traffic.Days) == 0:
		return []string{"", "  " + StyleMuted.Render(fmt.Sprintf("nenhum acesso nos últimos %d dias", c.days))}
	}
	accent := tabAccentColor(TabCloudflare)
	out := []string{"", sectionHeader(panelTitle("RESUMO", fmt.Sprintf("últimos %d dias", c.days)), w, accent)}
	out = append(out, a.cfdSummary(w)...)

	add := func(block []string, min int) {
		if left := room - len(out); len(block) > 0 && left >= min {
			out = append(out, block[:minInt(len(block), left)]...)
		}
	}
	// O gráfico cresce com a altura que sobra depois de reservar uma semana
	// de POR DIA e as páginas: 4 a 10 linhas.
	spare := room - len(out) - 4 - (2 + minInt(7, len(c.traffic.Days))) - cfdPathsRows(c)
	if chart := a.cfdHourLines(w, clampInt(spare, 4, 10)); len(chart) > 0 {
		add(append([]string{"", sectionHeader(a.cfdHourTitle(), w, accent)}, chart...), len(chart)+2)
	}
	days := a.cfdDayLines(c.traffic, w)
	left := room - len(out) - 2
	show := minInt(len(days), maxInt(0, left-cfdPathsRows(c)))
	if show < minInt(len(days), 3) {
		show = minInt(len(days), maxInt(0, left))
	}
	if show > 0 {
		add(append([]string{"", sectionHeader(panelTitle("POR DIA", "visitantes"), w, accent)}, days[:show]...), 3)
	}
	add(a.cfdPathLines(w), 4)
	return out
}

func cfdPathsRows(c cloudflareState) int {
	if len(c.pages) == 0 && len(c.failed) == 0 {
		return 0
	}
	return 2 + minInt(5, maxInt(len(c.pages), len(c.failed)))
}

// cfdSummary: um número por linha, com a comparação com o período anterior
// ao lado — é ela que diz se o número é bom.
func (a *App) cfdSummary(w int) []string {
	t := a.cfd.traffic
	tot := t.Total()
	label := func(s string) string { return "  " + StyleMuted.Render(padRight(s, 12)) }
	big := func(n int64) string { return StyleNormal.Bold(true).Render(padRight(collectors.FormatHubCount(n), 7)) }
	trend := func(now, before int64) string {
		if !t.HasPrev {
			return ""
		}
		return "  " + cfdTrend(now, before) + StyleMuted.Render(fmt.Sprintf(" vs %d dias antes", a.cfd.days))
	}

	threats := StyleHealthy.Render("nenhuma bloqueada")
	if tot.Threats > 0 {
		threats = StyleWarning.Render(collectors.FormatHubCount(tot.Threats) + " bloqueadas pela Cloudflare")
	}
	var from []string
	for i, c := range t.Countries {
		if i == 4 {
			break
		}
		from = append(from, StyleNormal.Render(cfdCountry(c.Name))+" "+StyleMuted.Render(fmt.Sprintf("%.0f%%", cfdShare(c.Requests, tot.Requests))))
	}
	if len(from) == 0 {
		from = []string{StyleMuted.Render(emDash)}
	}

	lines := []string{
		label("visitantes") + big(tot.Visits) + trend(tot.Visits, t.Prev.Visits),
		label("páginas") + big(tot.PageViews) + trend(tot.PageViews, t.Prev.PageViews),
		label("requisições") + big(tot.Requests) + StyleMuted.Render(fmt.Sprintf("  %.0f%% servidas do cache · %s de banda",
			cfdShare(tot.Cached, tot.Requests), collectors.FormatHubBytes(tot.Bytes))),
		label("respostas") + cfdStatusBar(t.Status, clampInt(w-60, 12, 30)),
		label("ameaças") + threats,
		label("de onde") + strings.Join(from, StyleMuted.Render(" · ")),
	}
	for i := range lines {
		lines[i] = truncateVisible(lines[i], w)
	}
	return lines
}

// cfdStatusStyle: 2xx verde, 3xx azul, 4xx amarelo, 5xx vermelho — e negrito
// quando a fatia pede atenção (5xx acima de 1%, 4xx acima de 10%).
func cfdStatusStyle(class string, share float64) lipgloss.Style {
	switch class {
	case "2xx":
		return StyleHealthy
	case "3xx":
		return lipgloss.NewStyle().Foreground(ColorPrimary)
	case "4xx":
		if share > 10 {
			return StyleWarning.Bold(true)
		}
		return StyleWarning
	case "5xx":
		if share > 1 {
			return StyleUnhealthy.Bold(true)
		}
		return StyleUnhealthy
	}
	return StyleMuted
}

// cfdStatusBar: uma barra só, dividida pela fatia de cada classe, e a legenda
// ao lado. Classe com resposta nunca some da barra, nem que seja um traço.
func cfdStatusBar(status []collectors.CFCount, width int) string {
	var total int64
	for _, s := range status {
		total += s.Requests
	}
	if total == 0 {
		return StyleMuted.Render(emDash)
	}
	cols := make([]int, len(status))
	used, biggest := 0, 0
	for i, s := range status {
		if s.Requests > 0 {
			cols[i] = maxInt(1, int(float64(s.Requests)*float64(width)/float64(total)))
		}
		used += cols[i]
		if cols[i] > cols[biggest] {
			biggest = i
		}
	}
	cols[biggest] += width - used
	var bar strings.Builder
	var legend []string
	for i, s := range status {
		if s.Requests == 0 {
			continue
		}
		share := cfdShare(s.Requests, total)
		st := cfdStatusStyle(s.Name, share)
		bar.WriteString(st.Render(strings.Repeat("█", maxInt(0, cols[i]))))
		legend = append(legend, st.Render(fmt.Sprintf("%s %s", s.Name, cfdPct(share))))
	}
	return bar.String() + "  " + strings.Join(legend, StyleMuted.Render(" · "))
}

func cfdPct(share float64) string {
	if share >= 10 || share == 0 {
		return fmt.Sprintf("%.0f%%", share)
	}
	return fmt.Sprintf("%.1f%%", share)
}

// cfdTrend: subir é verde, cair é amarelo (menos visita não é defeito, mas
// pede um olhar), e diferença abaixo de 1% é estável.
func cfdTrend(now, before int64) string {
	if before <= 0 {
		if now > 0 {
			return StyleHealthy.Render("↑ novo")
		}
		return StyleMuted.Render("→ igual")
	}
	d := float64(now-before) * 100 / float64(before)
	switch {
	case d >= 1:
		return StyleHealthy.Render(fmt.Sprintf("↑ %.0f%%", d))
	case d <= -1:
		return StyleWarning.Render(fmt.Sprintf("↓ %.0f%%", -d))
	}
	return StyleMuted.Render("→ estável")
}

func cfdShare(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

// ─── gráficos ────────────────────────────────────────────────────────────────

// Blocos de oitavo: a altura sobe de 1/8 em 1/8 de célula, e a curva sai lisa
// em vez de em degraus de célula inteira.
var (
	cfdUp    = [9]string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	cfdRight = [8]string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
)

// cfdSpark: um caractere por valor, do menor (▁) ao pico (█).
func cfdSpark(vals []int64) string {
	var peak int64
	for _, v := range vals {
		peak = max(peak, v)
	}
	var b strings.Builder
	for _, v := range vals {
		lvl := 1
		if peak > 0 {
			lvl = clampInt(int(float64(v)/float64(peak)*8+0.5), 1, 8)
		}
		b.WriteString(cfdUp[lvl])
	}
	return b.String()
}

// cfdHBar é a barra deitada: blocos cheios, a ponta em oitavos e o resto em
// trilho apagado, para a barra ter sempre o mesmo comprimento.
func cfdHBar(v, peak int64, width int, st lipgloss.Style) string {
	eighths := 0
	if peak > 0 {
		eighths = int(float64(v) / float64(peak) * float64(width*8))
	}
	if v > 0 && eighths == 0 {
		eighths = 1
	}
	full, part := eighths/8, eighths%8
	bar := strings.Repeat("█", full) + cfdRight[part]
	n := full
	if part > 0 {
		n++
	}
	return st.Render(bar) + StyleMuted.Render(strings.Repeat("░", maxInt(0, width-n)))
}

func (a *App) cfdHourTitle() string {
	var peak collectors.CFHour
	for _, h := range a.cfd.hourly {
		if h.Visits > peak.Visits {
			peak = h
		}
	}
	title := panelTitle("ÚLTIMAS 24 HORAS", "visitantes por hora")
	if peak.Visits > 0 {
		title = panelTitle(title, fmt.Sprintf("pico %s às %s", collectors.FormatHubCount(peak.Visits), peak.Time.Local().Format("15h")))
	}
	return title
}

// cfdHourLines: colunas de pé, uma por hora, com a escala à esquerda e as
// horas embaixo. A última coluna é a hora corrente, ainda pela metade.
func (a *App) cfdHourLines(w, rows int) []string {
	hours := a.cfd.hourly
	if len(hours) == 0 {
		return nil
	}
	var peak int64
	for _, h := range hours {
		peak = max(peak, h.Visits)
	}
	if peak == 0 {
		return []string{"  " + StyleMuted.Render("nenhum visitante nas últimas 24 horas")}
	}
	axisW := len(collectors.FormatHubCount(peak))
	chartW := minInt(w-axisW-6, 24*4)
	slot := maxInt(1, chartW/len(hours))
	barW := slot
	if slot >= 3 {
		barW = slot - 1
	}
	accent := cfdAccent()
	// A linha de cima fica livre para o número da coluna mais alta: cada
	// coluna leva em cima quantos visitantes teve, que é o que se quer ler.
	barRows := maxInt(1, rows-1)
	eighths := make([]int, len(hours))
	for i, h := range hours {
		eighths[i] = int(float64(h.Visits)/float64(peak)*float64(barRows*8) + 0.5)
		if h.Visits > 0 && eighths[i] == 0 {
			eighths[i] = 1
		}
	}
	var out []string
	for r := 0; r < rows; r++ {
		axis := strings.Repeat(" ", axisW) + " │"
		switch r {
		case 1:
			axis = padLeft(collectors.FormatHubCount(peak), axisW) + " ┤"
		case rows - 1:
			axis = padLeft("0", axisW) + " ┤"
		}
		var b strings.Builder
		for i, h := range hours {
			st := accent
			if h.Visits == peak {
				st = accent.Bold(true)
			}
			fill := eighths[i] - (rows-1-r)*8
			top := rows - 1 - (eighths[i]-1)/8 // linha da ponta da coluna
			if n := collectors.FormatHubCount(h.Visits); h.Visits > 0 && r == top-1 && len(n) <= slot {
				numSt := StyleNormal
				if h.Visits == peak {
					numSt = st
				}
				b.WriteString(numSt.Render(padRight(padLeft(n, (barW+len(n)+1)/2), slot)))
				continue
			}
			b.WriteString(st.Render(strings.Repeat(cfdUp[clampInt(fill, 0, 8)], barW)) + strings.Repeat(" ", slot-barW))
		}
		out = append(out, "  "+StyleMuted.Render(axis)+b.String())
	}
	out = append(out, "  "+StyleMuted.Render(strings.Repeat(" ", axisW)+" └"+strings.Repeat("─", slot*len(hours))))

	// Hora local a cada 6 horas; a última é "agora".
	label := []rune(strings.Repeat(" ", slot*len(hours)+4))
	put := func(at int, s string) {
		for i, ch := range []rune(s) {
			if at+i < len(label) {
				label[at+i] = ch
			}
		}
	}
	for i, h := range hours {
		if i%6 == 0 && i < len(hours)-3 {
			put(i*slot, h.Time.Local().Format("15h"))
		}
	}
	put(maxInt(0, (len(hours)-1)*slot+barW-5), "agora")
	out = append(out, "  "+StyleMuted.Render(strings.Repeat(" ", axisW+2)+string(label)))
	return out
}

// cfdDayLines: um dia por linha, o mais recente em cima — "hoje" e "ontem"
// pelo nome, que é como se pensa. A barra é dos visitantes; o pico leva a
// marca, e dia com erro do servidor diz quantos.
func (a *App) cfdDayLines(t collectors.CFTraffic, w int) []string {
	days := t.Days
	var peak int64
	for _, d := range days {
		peak = max(peak, d.Visits)
	}
	barW := clampInt(w-52, 8, 50)
	today := a.cfd.today
	var out []string
	for i := len(days) - 1; i >= 0; i-- {
		d := days[i]
		label := cfdWeekdays[d.Date.Weekday()] + " " + d.Date.Format("02/01")
		labelSt := StyleMuted
		switch d.Date.Format("2006-01-02") {
		case today.Format("2006-01-02"):
			label, labelSt = "hoje", StyleNormal
		case today.AddDate(0, 0, -1).Format("2006-01-02"):
			label, labelSt = "ontem", StyleNormal
		}
		barSt := cfdAccent()
		if d.Visits == peak && peak > 0 {
			barSt = barSt.Bold(true)
		}
		line := "  " + labelSt.Render(padRight(label, 10)) + cfdHBar(d.Visits, peak, barW, barSt) + "  " +
			StyleNormal.Bold(true).Render(padLeft(collectors.FormatHubCount(d.Visits), 5)) +
			StyleMuted.Render("  "+padLeft(collectors.FormatHubCount(d.Requests), 5)+" req")
		switch {
		case d.Errors > 0:
			line += "  " + StyleUnhealthy.Render(countOf(int(d.Errors), "erro 5xx", "erros 5xx"))
		case d.Visits == peak && peak > 0:
			line += "  " + StyleMuted.Render("pico")
		}
		out = append(out, truncateVisible(line, w))
	}
	return out
}

// cfdPathLines: as páginas mais pedidas e os caminhos que mais falharam nas
// últimas 24h — lado a lado com largura, um embaixo do outro sem.
func (a *App) cfdPathLines(w int) []string {
	c := a.cfd
	if len(c.pages) == 0 && len(c.failed) == 0 {
		return nil
	}
	const rows = 5
	colW := w
	if w >= 110 {
		colW = w/2 - 1
	}
	var pages, failed []string
	pages = append(pages, sectionHeader(panelTitle("PÁGINAS MAIS ACESSADAS", "24h"), colW, tabAccentColor(TabCloudflare)))
	var peak int64
	for _, p := range c.pages {
		peak = max(peak, p.Requests)
	}
	pathW := clampInt(colW/3, 10, 36)
	for i, p := range c.pages {
		if i == rows {
			break
		}
		pages = append(pages, "  "+StyleNormal.Render(padRight(truncate(p.Name, pathW), pathW))+" "+
			cfdHBar(p.Requests, peak, clampInt(colW-pathW-12, 4, 24), cfdAccent())+" "+
			StyleNormal.Bold(true).Render(padLeft(collectors.FormatHubCount(p.Requests), 5)))
	}
	if len(c.pages) == 0 {
		pages = append(pages, "  "+StyleMuted.Render(emDash))
	}
	failed = append(failed, sectionHeader(panelTitle("CAMINHOS COM ERRO", "24h"), colW, ColorMuted))
	for i, f := range c.failed {
		if i == rows {
			break
		}
		st := cfdStatusStyle(fmt.Sprintf("%dxx", f.Status/100), 0)
		failed = append(failed, "  "+st.Render(fmt.Sprintf("%d", f.Status))+"  "+
			StyleNormal.Render(padRight(truncate(f.Path, maxInt(10, colW-18)), maxInt(10, colW-18)))+" "+
			StyleMuted.Render(padLeft(collectors.FormatHubCount(f.Count), 5)))
	}
	if len(c.failed) == 0 {
		failed = append(failed, "  "+StyleHealthy.Render("nenhuma resposta 4xx ou 5xx"))
	}
	if w < 110 {
		return append(append(append([]string{""}, pages...), ""), failed...)
	}
	out := []string{""}
	for i := 0; i < maxInt(len(pages), len(failed)); i++ {
		l, r := "", ""
		if i < len(pages) {
			l = pages[i]
		}
		if i < len(failed) {
			r = failed[i]
		}
		out = append(out, padRightVisible(truncateVisible(l, colW), colW)+"  "+truncateVisible(r, colW))
	}
	return out
}

// ─── 2 DNS ───────────────────────────────────────────────────────────────────

func (a *App) cfdDNSLines(w int) []string {
	c := a.cfd
	row := a.cfdOpenRow()
	proxied := 0
	for _, r := range c.dns {
		if r.Proxied {
			proxied++
		}
	}
	out := []string{"", sectionHeader(panelTitle("REGISTROS", fmt.Sprint(len(c.dns))), w, tabAccentColor(TabCloudflare))}
	if len(c.dns) == 0 {
		return append(out, "  "+StyleMuted.Render("nenhum registro (ou o token não tem DNS Read)"))
	}
	out = append(out, "  "+cfdAccent().Render(fmt.Sprintf("◆ %d passam pelo proxy", proxied))+
		StyleMuted.Render(fmt.Sprintf(" — escondem o IP do servidor e ganham cache  ·  %d só DNS", len(c.dns)-proxied)), "")
	nameW := clampInt(w/4, 6, 28)
	contentW := clampInt(w-6-1-nameW-1-1-10, 6, 48)
	out = append(out, StyleMuted.Render("  "+padRight("TIPO", 6)+" "+padRight("NOME", nameW)+" "+padRight("APONTA PARA", contentW)+" PROXY"))
	for _, r := range c.dns {
		out = append(out, "  "+cfdRecordLine(r, row, nameW, contentW))
	}
	return out
}

func cfdRecordLine(r collectors.CFRecord, row cfdRow, nameW, contentW int) string {
	name := strings.TrimSuffix(r.Name, "."+row.zone.Name)
	if r.Name == row.zone.Name {
		name = "@"
	}
	st := StyleNormal
	if r.Name == row.host() {
		st = StyleNormal.Bold(true)
	}
	proxy := StyleMuted.Render("· só DNS")
	if r.Proxied {
		proxy = cfdAccent().Render("◆ proxy")
	}
	return StyleMuted.Render(padRight(r.Type, 6)) + " " + st.Render(padRight(truncate(name, nameW), nameW)) + " " +
		StyleNormal.Render(padRight(truncate(r.Content, contentW), contentW)) + " " + proxy
}

// ─── 3 segurança ─────────────────────────────────────────────────────────────

// cfdSecurityLines: o que diz se o domínio está bem protegido, cada linha com
// o veredito em palavras — "Flexible" não diz nada a quem não decorou a
// documentação; "do servidor para a Cloudflare vai sem criptografia" diz.
func (a *App) cfdSecurityLines(w int) []string {
	c := a.cfd
	s := c.sec
	accent := tabAccentColor(TabCloudflare)
	label := func(l string) string { return "  " + StyleMuted.Render(padRight(l, 12)) }
	set := func(id string) string { return s.Settings[id] }

	out := []string{"", sectionHeader("SSL / TLS", w, accent)}
	if s.Settings != nil {
		var mode string
		switch set("ssl") {
		case "strict":
			mode = StyleHealthy.Render("◆ Full (strict)") + StyleMuted.Render(" — criptografado até o servidor, certificado conferido")
		case "full":
			mode = StyleHealthy.Render("◆ Full") + StyleMuted.Render(" — criptografado até o servidor; strict também confere o certificado")
		case "flexible":
			mode = StyleWarning.Render("⚠ Flexible") + StyleMuted.Render(" — da Cloudflare até o servidor vai sem criptografia")
		case "off":
			mode = StyleUnhealthy.Render("⚠ desligado") + StyleMuted.Render(" — o site não tem HTTPS")
		default:
			mode = StyleMuted.Render(firstNonEmpty(set("ssl"), emDash))
		}
		https := StyleWarning.Render("◌ aceita HTTP também") + StyleMuted.Render(" — ligue Always Use HTTPS")
		if set("always_use_https") == "on" {
			https = StyleHealthy.Render("◆ sempre redireciona para HTTPS")
		}
		tls := StyleNormal.Render(firstNonEmpty(set("min_tls_version"), emDash))
		if v := set("min_tls_version"); v == "1.0" || v == "1.1" {
			tls = StyleWarning.Render(v) + StyleMuted.Render(" — versões antigas e inseguras ainda aceitas")
		}
		out = append(out, label("modo")+mode, label("HTTPS")+https, label("TLS mínimo")+tls)
	}
	for _, cert := range s.Certs {
		facts := []string{StyleNormal.Render(firstNonEmpty(cfdCertType(cert.Type), emDash))}
		if cert.Authority != "" {
			facts = append(facts, StyleMuted.Render(cert.Authority))
		}
		if !cert.Expires.IsZero() {
			facts = append(facts, a.cfdExpiry("vence", cert.Expires, 14))
		}
		out = append(out, label("certificado")+strings.Join(facts, StyleMuted.Render(" · ")))
	}
	if s.Settings == nil && len(s.Certs) == 0 {
		out = append(out, "  "+StyleMuted.Render("sem leitura — o token precisa de Zone Settings Read e SSL Read"))
	}

	out = append(out, "", sectionHeader(panelTitle("AVISOS DE SEGURANÇA", fmt.Sprint(len(s.Insights))), w, accent))
	if len(s.Insights) == 0 {
		out = append(out, "  "+StyleHealthy.Render("◆ nenhum aviso aberto no Security Center"))
	}
	for _, in := range s.Insights {
		glyph, st, sev := "·", StyleMuted, "baixo"
		switch in.Severity {
		case "critical":
			glyph, st, sev = "⚠", StyleUnhealthy.Bold(true), "crítico"
		case "moderate":
			glyph, st, sev = "◌", StyleWarning, "moderado"
		}
		text := in.Subject
		if in.Resolve != "" {
			text += " — " + in.Resolve
		}
		out = append(out, truncateVisible("  "+st.Render(padRight(glyph+" "+sev, 11))+" "+StyleNormal.Render(text), w))
	}

	out = append(out, "", sectionHeader("DOMÍNIO", w, accent))
	if !s.Expires.IsZero() {
		out = append(out, label("registro")+a.cfdExpiry("vence", s.Expires, 30)+StyleMuted.Render(" · "+s.Registry))
	}
	out = append(out, label("DNS em")+StyleMuted.Render(strings.Join(c.zone.NameServers, " · ")))
	var prot []string
	if v := set("security_level"); v != "" {
		prot = append(prot, StyleNormal.Render("nível "+v))
	}
	if set("development_mode") == "on" {
		prot = append(prot, StyleWarning.Render("modo desenvolvimento ligado (sem cache)"))
	}
	if v := set("http3"); v != "" {
		prot = append(prot, StyleMuted.Render("HTTP/3 "+cfdOnOff(v)))
	}
	if len(prot) > 0 {
		out = append(out, label("proteção")+strings.Join(prot, StyleMuted.Render(" · ")))
	}
	if tot := c.traffic.Total(); len(c.traffic.Days) > 0 {
		threats := StyleHealthy.Render("nenhuma bloqueada")
		if tot.Threats > 0 {
			threats = StyleWarning.Render(collectors.FormatHubCount(tot.Threats) + " bloqueadas")
		}
		out = append(out, label("ameaças")+threats+StyleMuted.Render(fmt.Sprintf(" nos últimos %d dias", c.days)))
	}
	for _, n := range s.Notes {
		out = append(out, "  "+StyleMuted.Render(truncate("não li "+n, w-4)))
	}
	return out
}

// cfdExpiry: data e quanto falta; perto do fim fica amarelo, vencido vermelho.
func (a *App) cfdExpiry(verb string, t time.Time, warnDays int) string {
	today := a.cfd.today
	if today.IsZero() {
		today = time.Now()
	}
	days := int(t.Sub(today).Hours() / 24)
	text := fmt.Sprintf("%s %s (em %d dias)", verb, t.Local().Format("02/01/2006"), days)
	switch {
	case days < 0:
		return StyleUnhealthy.Bold(true).Render(fmt.Sprintf("venceu em %s", t.Local().Format("02/01/2006")))
	case days <= warnDays:
		return StyleWarning.Render(text)
	}
	return StyleHealthy.Render(text)
}

func cfdCertType(t string) string {
	switch t {
	case "universal":
		return "Universal (grátis)"
	case "advanced":
		return "Advanced"
	case "custom":
		return "próprio"
	}
	return t
}

func cfdOnOff(v string) string {
	if v == "on" {
		return "ligado"
	}
	if v == "off" {
		return "desligado"
	}
	return v
}

// ─── miúdos ──────────────────────────────────────────────────────────────────

var cfdWeekdays = [7]string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}

// cfdCountry dá nome aos países que mais aparecem; o resto fica no código.
func cfdCountry(code string) string {
	if n, ok := cfdCountryNames[code]; ok {
		return n
	}
	return code
}

var cfdCountryNames = map[string]string{
	"BR": "Brasil", "US": "Estados Unidos", "PT": "Portugal", "AR": "Argentina", "DE": "Alemanha",
	"FR": "França", "GB": "Reino Unido", "NL": "Holanda", "CA": "Canadá", "MX": "México",
	"ES": "Espanha", "IT": "Itália", "IE": "Irlanda", "SG": "Singapura", "JP": "Japão",
	"CN": "China", "IN": "Índia", "RU": "Rússia", "CL": "Chile", "CO": "Colômbia",
	"UY": "Uruguai", "PY": "Paraguai", "FI": "Finlândia", "SE": "Suécia", "PL": "Polônia",
}
