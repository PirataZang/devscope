package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/routeutil"
)

type routesLoadedMsg struct {
	routes []routeutil.Route
	stacks []string
	err    error
}

func (a *App) enterRoutesTab(_ *core.Project) {
	a.tab = TabRoutes
	a.tabCursor = 0
	a.routesOpen = false
	a.routesFilterOn = false
}

func (a *App) openRoutesClient(p *core.Project) tea.Cmd {
	a.routesOpen = true
	a.routesCursor = 0
	a.routesScroll = 0
	a.routesErr = ""
	a.routesStatus = "scanning…"
	a.routesLoading = true
	a.routesFilterOn = false
	return a.scanProjectRoutes(p)
}

func (a *App) leaveRoutesTab() tea.Cmd {
	a.routesOpen = false
	a.routesLoading = false
	a.routesFilterOn = false
	a.tab = TabRoutes
	a.tabCursor = 0
	return nil
}

func (a *App) scanProjectRoutes(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	ports := append([]int(nil), p.Ports...)
	var frameworks []string
	if p.Framework.Name != "" {
		frameworks = append(frameworks, p.Framework.Name)
	}
	for _, f := range p.Frameworks {
		frameworks = append(frameworks, f.Name)
	}
	a.routesLoading = true
	a.routesStatus = "scanning…"
	a.routesErr = ""
	return func() tea.Msg {
		routes, stacks, err := routeutil.Discover(path, ports, frameworks)
		return routesLoadedMsg{routes: routes, stacks: stacks, err: err}
	}
}

func (a *App) handleRoutesLoaded(msg routesLoadedMsg) {
	a.routesLoading = false
	if msg.err != nil {
		a.routesErr = msg.err.Error()
		a.routesStatus = ""
		return
	}
	a.routes = msg.routes
	a.routesErr = ""
	stackHint := ""
	if len(msg.stacks) > 0 {
		var shown []string
		for _, s := range msg.stacks {
			switch s {
			case "node", "python", "java", "php", "rust", "go":
				continue
			default:
				shown = append(shown, s)
			}
		}
		if len(shown) == 0 {
			shown = msg.stacks
		}
		if len(shown) > 4 {
			shown = shown[:4]
		}
		stackHint = strings.Join(shown, "+") + " · "
	}
	a.routesStatus = stackHint + a.routesCountLabel()
	a.syncRoutesCursor()
}

func (a *App) routesCountLabel() string {
	all := len(a.routes)
	vis := a.filteredRoutes()
	auth := 0
	for _, r := range a.routes {
		if r.Auth {
			auth++
		}
	}
	if q := strings.TrimSpace(a.routesFilter); q != "" {
		return fmt.Sprintf("%d/%d rotas · \"%s\"", len(vis), all, q)
	}
	if all == 0 {
		return "nenhuma rota encontrada"
	}
	if auth > 0 {
		return fmt.Sprintf("%d rotas · %d auth", all, auth)
	}
	return fmt.Sprintf("%d rotas", all)
}

func (a *App) filteredRoutes() []routeutil.Route {
	q := strings.ToLower(strings.TrimSpace(a.routesFilter))
	if q == "" {
		return a.routes
	}
	out := make([]routeutil.Route, 0, len(a.routes))
	for _, r := range a.routes {
		if routeMatchesFilter(r, q) {
			out = append(out, r)
		}
	}
	return out
}

func routeMatchesFilter(r routeutil.Route, q string) bool {
	if q == "auth" || q == "private" || q == "protegida" || q == "secured" {
		return r.Auth
	}
	if q == "public" || q == "aberta" {
		return !r.Auth
	}
	hay := strings.ToLower(strings.Join([]string{
		r.Path, r.Method, r.Summary, r.Source, r.File,
	}, " "))
	if r.Auth {
		hay += " auth private"
	}
	return strings.Contains(hay, q)
}

func (a *App) syncRoutesCursor() {
	n := len(a.filteredRoutes())
	if n == 0 {
		a.routesCursor = 0
		a.routesScroll = 0
		return
	}
	if a.routesCursor >= n {
		a.routesCursor = n - 1
	}
	if a.routesCursor < 0 {
		a.routesCursor = 0
	}
}

func (a *App) updateRoutesFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.routesFilterOn = false
		a.routesFilterInput = ""
		a.routesFilter = ""
		a.syncRoutesCursor()
		a.refreshRoutesStatusFilter()
	case "enter":
		a.routesFilterOn = false
		a.routesFilter = strings.TrimSpace(a.routesFilterInput)
		a.routesFilterInput = ""
		a.routesCursor = 0
		a.routesScroll = 0
		a.syncRoutesCursor()
		a.refreshRoutesStatusFilter()
	case "backspace":
		if len(a.routesFilterInput) > 0 {
			r := []rune(a.routesFilterInput)
			a.routesFilterInput = string(r[:len(r)-1])
		}
		a.routesFilter = strings.TrimSpace(a.routesFilterInput)
		a.routesCursor = 0
		a.syncRoutesCursor()
	default:
		if len(msg.String()) == 1 {
			a.routesFilterInput += msg.String()
			a.routesFilter = strings.TrimSpace(a.routesFilterInput)
			a.routesCursor = 0
			a.syncRoutesCursor()
		}
	}
	return a, nil
}

func (a *App) refreshRoutesStatusFilter() {
	base := a.routesStatus
	if i := strings.Index(base, " · "); i >= 0 {
		prefix := base[:i+3]
		if !strings.Contains(prefix, "rotas") {
			a.routesStatus = prefix + a.routesCountLabel()
			return
		}
	}
	a.routesStatus = a.routesCountLabel()
}

func (a *App) renderRoutesLanding(p *core.Project) string {
	w, h := a.moduleSize()
	ctx := a.renderModuleContext(p, w, "Rotas", "discovery")
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(5, bodyH*26/100)
	srcH := maxInt(6, bodyH*40/100)
	keysH := maxInt(5, bodyH-openH-srcH)
	openLines := append([]string{StyleMuted.Render("descobre endpoints públicos e privados")}, moduleOpenHint()...)
	openLines[1] = StyleNormal.Render("pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para escanear")
	srcLines := []string{
		StyleMuted.Render("OpenAPI/Swagger (arquivo ou localhost)"),
		StyleMuted.Render("Laravel · Nest · Express · Fastify"),
		StyleMuted.Render("Next · Nuxt · Django · Flask · FastAPI"),
		StyleMuted.Render("Rails · Go · Spring · Axum/Actix"),
		StyleMuted.Render("auth/sanctum/guards detectados no código"),
	}
	keyLines := []string{
		StyleMuted.Render("↑↓ / j k   navegar"),
		StyleMuted.Render("b          filtrar (path/auth/source)"),
		StyleMuted.Render("enter      abrir na API"),
		StyleMuted.Render("r          reescanear"),
		StyleMuted.Render("esc        voltar"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("ROTAS", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("FONTES", fitExactLines(srcLines, srcH-2), centerW, srcH, false),
		renderApiTitledBox("ATALHOS NO CLIENTE", fitExactLines(keyLines, keysH-2), centerW, keysH, false),
	)
	stack := "—"
	if p != nil && p.Framework.Name != "" {
		stack = p.Framework.Name
	}
	details := []string{
		StyleMuted.Render("Stack   ") + StyleNormal.Render(truncate(stack, rightW-10)),
		StyleMuted.Render("Modo    ") + StyleMuted.Render("scan + OpenAPI"),
		StyleMuted.Render("Inclui  ") + StyleMuted.Render("rotas auth/privadas"),
		StyleMuted.Render("Destino ") + StyleMuted.Render("aba API"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "escanear rotas"},
		[2]string{"8", "abrir API"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderRoutesTab(p *core.Project) string {
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
	visible := a.filteredRoutes()
	if !a.routesLoading && len(visible) > 0 {
		a.routesCursor = clampCursor(a.routesCursor, len(visible))
	}

	header := a.renderRoutesHeader(p, w)
	cards := a.renderRoutesCards(w)
	filterLine := a.renderRoutesFilterLine(w)

	chromeH := lipgloss.Height(header) + lipgloss.Height(cards) + lipgloss.Height(filterLine) + 2
	bodyH := maxInt(8, h-chromeH-2)

	rightW := maxInt(24, w*28/100)
	if rightW > 40 {
		rightW = 40
	}
	leftW := maxInt(36, w-rightW-1)
	list := a.renderRoutesTable(visible, leftW, bodyH)
	detail := a.renderRoutesInspector(visible, rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)

	hints := "↑↓ navegar  b filtrar  enter → API  r rescan  esc"
	if a.routesLoading {
		hints = "scanning…  esc"
	} else if a.routesFilterOn {
		hints = "filtro: path · auth · source · file  enter aplicar  esc limpar"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		header, cards, filterLine, body,
		a.renderStatusBar(hints),
	)
}

func (a *App) renderRoutesHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabRoutes)).Bold(true)
	left := accent.Render("devscope") + StyleMuted.Render(" › rotas")
	if p != nil {
		left += StyleMuted.Render("  ") + StyleNormal.Render(truncate(p.Name, 28))
	}
	right := StyleMuted.Render(a.routesStatus)
	if a.routesErr != "" {
		right = StyleUnhealthy.Render(truncate(a.routesErr, 36))
	} else if a.routesLoading {
		right = StyleMuted.Render("escaneando…")
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderRoutesCards(width int) string {
	byMethod := map[string]int{}
	sources := map[string]int{}
	authN := 0
	for _, r := range a.routes {
		byMethod[r.Method]++
		sources[r.Source]++
		if r.Auth {
			authN++
		}
	}
	topSrc := "—"
	topN := 0
	for s, n := range sources {
		if s == "" {
			s = "?"
		}
		if n > topN {
			topN = n
			topSrc = s
		}
	}
	meth := fmt.Sprintf("G%d P%d U%d D%d",
		byMethod["GET"], byMethod["POST"], byMethod["PUT"]+byMethod["PATCH"], byMethod["DELETE"])
	vis := len(a.filteredRoutes())
	boxW := maxInt(12, width/5)
	cards := []struct{ title, value string; warn bool }{
		{"TOTAL", fmt.Sprintf("%d", len(a.routes)), false},
		{"VISÍVEIS", fmt.Sprintf("%d", vis), false},
		{"AUTH", fmt.Sprintf("%d", authN), authN > 0},
		{"MÉTODOS", meth, false},
		{"FONTE", topSrc, false},
	}
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		val := StyleNormal.Render(truncate(c.value, boxW-4))
		if c.title == "AUTH" && c.warn {
			val = StyleWarning.Render(truncate(c.value, boxW-4))
		}
		if c.title == "TOTAL" {
			val = StyleHealthy.Render(truncate(c.value, boxW-4))
		}
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines([]string{val}, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderRoutesFilterLine(width int) string {
	if a.routesFilterOn {
		return StyleKey.Render("filter ") + StyleSelected.Render(a.routesFilterInput+"▌") +
			StyleMuted.Render("  (auth · public · path · source)")
	}
	if q := strings.TrimSpace(a.routesFilter); q != "" {
		return StyleMuted.Render("filter: ") + StyleNormal.Render(q) +
			StyleMuted.Render("  (b editar · esc limpar)")
	}
	return StyleMuted.Render(truncate("b filtrar · tip: \"auth\" mostra só privadas", maxInt(20, width-2)))
}

func (a *App) renderRoutesTable(visible []routeutil.Route, width, height int) string {
	viewport := maxInt(1, height-2)
	lines := make([]string, 0, viewport)

	if a.routesLoading && len(a.routes) == 0 {
		lines = append(lines, StyleMuted.Render("  escaneando OpenAPI e código…"))
	} else if len(a.routes) == 0 {
		msg := "nenhuma rota encontrada"
		if a.routesErr != "" {
			msg = a.routesErr
		}
		lines = append(lines,
			StyleMuted.Render("  "+msg),
			StyleMuted.Render("  dica: openapi.json ou rotas no código"),
			StyleMuted.Render("  r para tentar de novo"),
		)
	} else if len(visible) == 0 {
		lines = append(lines,
			StyleMuted.Render(fmt.Sprintf("  nenhuma rota com \"%s\"", strings.TrimSpace(a.routesFilter))),
			StyleMuted.Render("  b mudar · esc limpar"),
		)
	} else {
		a.routesScroll = ensureVisible(a.routesCursor, a.routesScroll, viewport-1, len(visible))
		header := StyleMuted.Render(truncate(
			fmt.Sprintf("  %-6s %-28s %-5s %-10s %s", "METHOD", "PATH", "AUTH", "SOURCE", "FILE"),
			width-2,
		))
		lines = append(lines, header)
		start := a.routesScroll
		end := minInt(start+viewport-1, len(visible))
		for i := start; i < end; i++ {
			lines = append(lines, a.renderRoutesTableLine(visible[i], i, width-2))
		}
	}
	title := fmt.Sprintf("ROTAS (%d)", len(visible))
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, true)
}

func (a *App) renderRoutesTableLine(r routeutil.Route, idx, width int) string {
	marker := "  "
	if a.routesCursor == idx {
		marker = "▶ "
	}
	auth := "·"
	if r.Auth {
		auth = "auth"
	}
	src := r.Source
	if src == "" {
		src = "?"
	}
	file := r.File
	if r.Line > 0 {
		file = fmt.Sprintf("%s:%d", r.File, r.Line)
	}
	if file == "" || file == ":0" {
		file = "—"
	}
	pathW := maxInt(10, width-48)
	plain := fmt.Sprintf("%s%-6s %-28s %-5s %-10s %s",
		marker,
		truncate(r.Method, 6),
		truncate(r.Path, 28),
		auth,
		truncate(src, 10),
		truncate(file, pathW),
	)
	plain = truncate(plain, width)
	if a.routesCursor == idx {
		return StyleSelected.Render(plain)
	}
	methodSt := routeMethodStyle(r.Method)
	authSt := StyleMuted
	if r.Auth {
		authSt = StyleWarning
	}
	return marker +
		methodSt.Render(fmt.Sprintf("%-6s", truncate(r.Method, 6))) + " " +
		StyleNormal.Render(fmt.Sprintf("%-28s", truncate(r.Path, 28))) + " " +
		authSt.Render(fmt.Sprintf("%-5s", auth)) + " " +
		StyleMuted.Render(fmt.Sprintf("%-10s", truncate(src, 10))) + " " +
		StyleMuted.Render(truncate(file, pathW))
}

func (a *App) renderRoutesInspector(visible []routeutil.Route, width, height int) string {
	detH := maxInt(8, height*55/100)
	actH := maxInt(5, height*22/100)
	statH := maxInt(4, height-detH-actH)

	details := []string{StyleMuted.Render("(nenhuma rota)")}
	if a.routesLoading && len(a.routes) == 0 {
		details = []string{StyleMuted.Render("carregando…")}
	} else if len(visible) > 0 && a.routesCursor < len(visible) {
		r := visible[a.routesCursor]
		access := StyleHealthy.Render("pública")
		if r.Auth {
			access = StyleWarning.Render("privada / auth")
		}
		file := r.File
		if r.Line > 0 {
			file = fmt.Sprintf("%s:%d", r.File, r.Line)
		}
		if file == "" {
			file = "—"
		}
		details = []string{
			StyleMuted.Render("Method  ") + routeMethodStyle(r.Method).Render(r.Method),
			StyleMuted.Render("Path    ") + StyleNormal.Render(truncate(r.Path, width-12)),
			StyleMuted.Render("Access  ") + access,
			StyleMuted.Render("Source  ") + StyleMuted.Render(firstNonEmpty(r.Source, "—")),
			StyleMuted.Render("File    ") + StyleMuted.Render(truncate(file, width-12)),
		}
		if r.Summary != "" {
			details = append(details, "",
				StyleMuted.Render("Summary"),
				StyleNormal.Render(truncate(r.Summary, width-4)),
			)
		}
	}

	actions := moduleActionLines(
		[2]string{"enter", "abrir na API"},
		[2]string{"b", "filtrar"},
		[2]string{"r", "reescanear"},
		[2]string{"esc", "voltar"},
	)

	statLines := a.routesSourceStats(width)

	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DETALHES", fitExactLines(details, detH-2), width, detH, false),
		renderApiTitledBox("AÇÕES", fitExactLines(actions, actH-2), width, actH, false),
		renderApiTitledBox("FONTES", fitExactLines(statLines, statH-2), width, statH, false),
	)
}

func (a *App) routesSourceStats(width int) []string {
	type pair struct {
		name string
		n    int
	}
	counts := map[string]int{}
	for _, r := range a.routes {
		s := r.Source
		if s == "" {
			s = "?"
		}
		counts[s]++
	}
	if len(counts) == 0 {
		return []string{StyleMuted.Render("(sem fontes)")}
	}
	list := make([]pair, 0, len(counts))
	maxN := 0
	for name, n := range counts {
		list = append(list, pair{name, n})
		if n > maxN {
			maxN = n
		}
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].n > list[i].n {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	barW := maxInt(6, width-16)
	out := make([]string, 0, minInt(6, len(list)))
	for i := 0; i < len(list) && i < 6; i++ {
		p := list[i]
		pct := 100.0 * float64(p.n) / float64(maxN)
		out = append(out,
			StyleMuted.Render(fmt.Sprintf("%-10s ", truncate(p.name, 10)))+
				meterBar(pct, barW)+
				StyleMuted.Render(fmt.Sprintf(" %d", p.n)),
		)
	}
	return out
}

func routeMethodStyle(m string) lipgloss.Style {
	switch strings.ToUpper(m) {
	case "GET":
		return lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	case "POST":
		return lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	case "PUT", "PATCH":
		return lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	case "DELETE":
		return lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	default:
		return StyleNormal.Bold(true)
	}
}

func (a *App) handleRoutesKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if a.routesFilter != "" {
			a.routesFilter = ""
			a.routesFilterInput = ""
			a.syncRoutesCursor()
			a.refreshRoutesStatusFilter()
			return a, nil
		}
		return a, a.leaveRoutesTab()
	case "b":
		a.routesFilterOn = true
		a.routesFilterInput = a.routesFilter
		return a, nil
	case "r":
		return a, a.scanProjectRoutes(p)
	case "up", "k":
		if a.routesCursor > 0 {
			a.routesCursor--
		}
	case "down", "j":
		vis := a.filteredRoutes()
		if a.routesCursor < len(vis)-1 {
			a.routesCursor++
		}
	case "pgup":
		a.routesCursor = maxInt(0, a.routesCursor-10)
	case "pgdown":
		vis := a.filteredRoutes()
		a.routesCursor = minInt(len(vis)-1, a.routesCursor+10)
	case "enter":
		vis := a.filteredRoutes()
		if a.routesLoading || len(vis) == 0 {
			return a, nil
		}
		if a.routesCursor < 0 || a.routesCursor >= len(vis) {
			return a, nil
		}
		r := vis[a.routesCursor]
		return a, a.openApiWithPreset(p, r.Method, r.Path)
	}
	return a, nil
}
