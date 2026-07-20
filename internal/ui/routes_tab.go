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
	if q := strings.TrimSpace(a.routesFilter); q != "" {
		return fmt.Sprintf("%d/%d rotas · \"%s\"", len(vis), all, q)
	}
	if all == 0 {
		return "nenhuma rota encontrada"
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
		if strings.Contains(strings.ToLower(r.Path), q) ||
			strings.Contains(strings.ToLower(r.Method), q) ||
			strings.Contains(strings.ToLower(r.Summary), q) ||
			strings.Contains(strings.ToLower(r.Source), q) {
			out = append(out, r)
		}
	}
	return out
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
		// Live preview while typing.
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
	// Keep stack prefix if present (before first · count).
	base := a.routesStatus
	if i := strings.Index(base, " · "); i >= 0 {
		prefix := base[:i+3]
		// Only keep if it looks like stacks (no "rotas" before).
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
	openLines := append([]string{StyleMuted.Render("descobre endpoints e abre na aba API")}, moduleOpenHint()...)
	// override enter hint wording for routes
	openLines[1] = StyleNormal.Render("pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para escanear")
	srcLines := []string{
		StyleMuted.Render("OpenAPI/Swagger (arquivo ou localhost)"),
		StyleMuted.Render("Nest · Express · Fastify · Hono"),
		StyleMuted.Render("Next · Nuxt · Laravel · Django"),
		StyleMuted.Render("Flask · FastAPI · Rails"),
		StyleMuted.Render("Go (Gin/Echo/Fiber/Chi)"),
		StyleMuted.Render("Spring · Axum/Actix"),
	}
	keyLines := []string{
		StyleMuted.Render("↑↓ / j k   navegar"),
		StyleMuted.Render("b          filtrar path"),
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
	w := maxInt(60, a.width)
	h := maxInt(18, a.height-2)
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabRoutes)).Bold(true)

	header := accent.Render("⇄ Rotas")
	if p != nil {
		header += StyleMuted.Render("  "+truncate(p.Name, 24))
	}
	if a.routesErr != "" {
		header += "  " + StyleUnhealthy.Render(truncate(a.routesErr, 40))
	} else if a.routesStatus != "" {
		header += "  " + StyleHealthy.Render(a.routesStatus)
	}

	bodyH := maxInt(4, h-3)
	if a.routesFilterOn {
		bodyH = maxInt(3, bodyH-1)
	}
	innerW := maxInt(20, w-4)
	lines := a.renderRoutesList(innerW, bodyH)
	body := StylePanel.Width(w - 2).Render(strings.Join(lines, "\n"))

	var parts []string
	parts = append(parts, header)
	if a.routesFilterOn {
		prompt := StyleKey.Render("filter ") + StyleSelected.Render(a.routesFilterInput+"▌")
		parts = append(parts, prompt)
	} else if q := strings.TrimSpace(a.routesFilter); q != "" {
		parts = append(parts, StyleMuted.Render("filter: ")+StyleNormal.Render(q)+StyleMuted.Render("  (b editar · esc limpar)"))
	}
	parts = append(parts, body)

	hints := "↑↓ navegar  b filtrar  enter → API  r rescan  esc"
	if a.routesLoading {
		hints = "scanning…  esc"
	} else if a.routesFilterOn {
		hints = "digite o filtro  enter aplicar  esc limpar"
	}
	parts = append(parts, a.renderStatusBar(hints))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (a *App) renderRoutesList(width, height int) []string {
	if a.routesLoading && len(a.routes) == 0 {
		return fitExactLines([]string{StyleMuted.Render("escaneando OpenAPI e código…")}, height)
	}
	visible := a.filteredRoutes()
	if len(a.routes) == 0 {
		msg := "nenhuma rota encontrada"
		if a.routesErr != "" {
			msg = a.routesErr
		}
		return fitExactLines([]string{
			StyleMuted.Render(msg),
			"",
			StyleMuted.Render("dica: openapi.json ou rotas no código da stack detectada"),
			StyleMuted.Render("pressione r para tentar de novo"),
		}, height)
	}
	if len(visible) == 0 {
		q := strings.TrimSpace(a.routesFilter)
		return fitExactLines([]string{
			StyleMuted.Render(fmt.Sprintf("nenhuma rota com \"%s\"", q)),
			"",
			StyleMuted.Render("b para mudar o filtro · esc no filtro limpa"),
		}, height)
	}

	if a.routesCursor < a.routesScroll {
		a.routesScroll = a.routesCursor
	}
	if a.routesCursor >= a.routesScroll+height {
		a.routesScroll = a.routesCursor - height + 1
	}
	if a.routesScroll < 0 {
		a.routesScroll = 0
	}

	out := make([]string, 0, height)
	end := minInt(a.routesScroll+height, len(visible))
	for i := a.routesScroll; i < end; i++ {
		r := visible[i]
		method := fmt.Sprintf("%-6s", r.Method)
		methodSt := routeMethodStyle(r.Method)
		path := truncate(r.Path, maxInt(8, width-18))
		src := StyleMuted.Render(r.Source)
		line := methodSt.Render(method) + " " + StyleNormal.Render(path)
		if r.Summary != "" {
			line += "  " + StyleMuted.Render(truncate(r.Summary, 24))
		} else {
			line += "  " + src
		}
		if i == a.routesCursor {
			line = StyleSelected.Render("▸ ") + line
		} else {
			line = "  " + line
		}
		out = append(out, line)
	}
	return fitExactLines(out, height)
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
