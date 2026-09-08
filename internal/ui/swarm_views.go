package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderSwarmLanding(p *core.Project) string {
	w, h := a.moduleSize()
	info := a.landingSwarm
	compose := a.landingSwarmCompose
	status := "…"
	if a.landingSwarmOK {
		status = "offline"
		if a.landingSwarmAvail {
			switch {
			case info.State == "unavailable":
				status = "unavailable"
			case info.Active:
				status = "active"
			default:
				status = "inactive"
			}
		}
	}
	ctx := a.renderModuleContext(p, w, "SWARM", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)
	openH := maxInt(7, bodyH*42/100)
	featH := maxInt(6, bodyH-openH)

	openLines := []string{
		StyleMuted.Render("cluster · services · nodes · tasks · stacks"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingSwarmOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando ambiente…"))
	case !a.landingSwarmAvail:
		openLines = append(openLines, "", StyleUnhealthy.Render("docker não encontrado no PATH"))
	case info.Error != "":
		openLines = append(openLines, "", StyleUnhealthy.Render(truncate(info.Error, 40)))
	case info.Active:
		openLines = append(openLines, "",
			StyleHealthy.Render(a.pulse()+" ACTIVE")+
				StyleMuted.Render(fmt.Sprintf("  ·  %d mgr  ·  %d nodes", info.Managers, info.Nodes)))
	default:
		openLines = append(openLines, "", StyleWarning.Render("○ INACTIVE — i inicia o cluster"))
	}

	featLines := []string{
		StyleMuted.Render("Control Center: observe → operate → deploy"),
		StyleMuted.Render("scale · update · logs · promote · join-token"),
		StyleMuted.Render("stack deploy ligado ao compose do projeto"),
	}
	if compose != "" {
		featLines = append(featLines, StyleMuted.Render("compose  "+swarmComposeBase(compose)))
	}

	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DOCKER SWARM", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	cliLabel, nodesLabel := "…", "…"
	if a.landingSwarmOK {
		cliLabel = boolLabel(a.landingSwarmAvail)
		nodesLabel = fmt.Sprintf("%d", info.Nodes)
	}
	details := []string{
		StyleMuted.Render("CLI    ") + StyleNormal.Render(cliLabel),
		StyleMuted.Render("Swarm  ") + StyleMuted.Render(status),
		StyleMuted.Render("Nodes  ") + StyleNormal.Render(nodesLabel),
	}
	if compose != "" {
		details = append(details, StyleMuted.Render("Stack  ")+StyleMuted.Render(swarmComposeBase(compose)))
	}
	actions := moduleActionLines(
		[2]string{"enter", "control center"},
		[2]string{"i", "swarm init"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderSwarmTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	view := a.renderSwarmCluster(p, w, h)
	switch a.swarmScreen {
	case swarmScrForm:
		view = overlayCentered(view, a.renderSwarmFormBox(), w, h)
	case swarmScrLogs:
		view = overlayCentered(view, a.renderSwarmLogsBox(w, h), w, h)
	case swarmScrDetail:
		view = overlayCentered(view, a.renderSwarmDetailBox(w, h), w, h)
	}
	if a.swarmConfirm {
		box := renderDeleteConfirmBox(a.swarmConfirmOpts(), w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) swarmConfirmOpts() deleteConfirmOpts {
	act := a.swarmConfirmAction
	opts := deleteConfirmOpts{
		Brand: "SWARM",
		Color: tabAccentColor(TabSwarm),
	}
	switch {
	case strings.HasPrefix(act, "rm-service:"):
		opts.Title, opts.Subtitle, opts.Label = "Excluir service", "docker service rm", "service"
		opts.Target = strings.TrimPrefix(act, "rm-service:")
	case strings.HasPrefix(act, "rm-stack:"):
		opts.Title, opts.Subtitle, opts.Label = "Excluir stack", "docker stack rm", "stack"
		opts.Target = strings.TrimPrefix(act, "rm-stack:")
	case strings.HasPrefix(act, "rm-node:"):
		opts.Title, opts.Subtitle, opts.Label = "Excluir node", "docker node rm — force", "node"
		opts.Target = strings.TrimPrefix(act, "rm-node:")
	case strings.HasPrefix(act, "rm-secret:"):
		opts.Title, opts.Subtitle, opts.Label = "Excluir secret", "docker secret rm", "secret"
		opts.Target = strings.TrimPrefix(act, "rm-secret:")
	case strings.HasPrefix(act, "rm-config:"):
		opts.Title, opts.Subtitle, opts.Label = "Excluir config", "docker config rm", "config"
		opts.Target = strings.TrimPrefix(act, "rm-config:")
	case strings.HasPrefix(act, "demote:"):
		opts.Title, opts.Subtitle, opts.Label = "Demote manager", "docker node demote", "node"
		opts.Target = strings.TrimPrefix(act, "demote:")
	case act == "leave":
		opts.Title, opts.Subtitle, opts.Label = "Leave swarm", "docker swarm leave — force", "ação"
		opts.Target = "sair do swarm"
		opts.Detail = firstNonEmpty(a.swarmStatus, "remove este node do cluster")
	case act == "prune":
		opts.Title, opts.Subtitle, opts.Label = "Prune networks", "docker network prune", "ação"
		opts.Target = "prune networks"
	default:
		opts.Title, opts.Subtitle, opts.Label = "Confirmar", "ação destrutiva no swarm", "ação"
		opts.Target = firstNonEmpty(act, "—")
	}
	return opts
}

func (a *App) renderSwarmCluster(p *core.Project, w, h int) string {
	header := a.renderSwarmHeader(w, p)
	tabs := a.renderSwarmKindTabs(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(tabs) + 2
	bodyH := maxInt(8, h-chromeH-2)

	rightW := maxInt(22, w*24/100)
	if rightW > 34 {
		rightW = 34
	}
	mainW := maxInt(40, w-rightW)
	tableH := maxInt(6, bodyH*55/100)
	detailH := maxInt(4, bodyH-tableH)

	table := a.renderSwarmTable(mainW, tableH)
	detail := a.renderSwarmSummary(mainW, detailH)
	center := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	right := a.renderSwarmRightRail(rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, center, right)

	hints := a.swarmHints()
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, a.renderStatusBar(hints))
}

// renderSwarmHeader concentra o que estava espalhado entre header, linha de
// status e os seis cards — os três repetiam as mesmas contagens.
func (a *App) renderSwarmHeader(width int, p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabSwarm)).Bold(true)
	left := accent.Render("⬡ DOCKER SWARM")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 24))
	}
	left += "   " + a.swarmStateChip()

	var right []string
	info := a.swarmInfo
	if info.Active {
		right = append(right, StyleMuted.Render(fmt.Sprintf("%d manager · %d worker", info.Managers, info.Workers)))
	}
	if info.EngineVersion != "" {
		right = append(right, StyleMuted.Render("engine "+info.EngineVersion))
	}
	if a.swarmLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

func (a *App) swarmStateChip() string {
	info := a.swarmInfo
	switch {
	case !info.Active && info.Error != "":
		return StyleUnhealthy.Render("✕ " + truncate(info.Error, 34))
	case !info.Active:
		return StyleWarning.Render("○ swarm inativo · i inicia")
	case a.swarmNodesDown() > 0:
		return StyleWarning.Render(fmt.Sprintf("◐ %d nó(s) fora", a.swarmNodesDown()))
	default:
		return StyleHealthy.Render("● cluster ativo")
	}
}

func (a *App) swarmNodesDown() int {
	n := 0
	for _, node := range a.swarmNodes {
		if !strings.EqualFold(node.Status, "Ready") {
			n++
		}
	}
	return n
}

// renderSwarmKindTabs mostra as teclas 1-8 (existem no handler) e a contagem de
// cada tipo — que antes eram seis caixas de 3 linhas para exibir um dígito.
func (a *App) renderSwarmKindTabs(width int) string {
	kinds := []swarmKind{
		swarmKindServices, swarmKindNodes, swarmKindTasks, swarmKindStacks,
		swarmKindNetworks, swarmKindSecrets, swarmKindConfigs, swarmKindEvents,
	}
	parts := make([]string, 0, len(kinds))
	for i, k := range kinds {
		name := strings.ToUpper(swarmKindLabel(k))
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if n := a.swarmCountFor(k); n > 0 {
			label = fmt.Sprintf(" %d %s %d ", i+1, name, n)
		}
		if k == a.swarmKind {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	ok, degraded, down := a.swarmServiceHealth()
	var chips []string
	if ok > 0 {
		chips = append(chips, StyleHealthy.Render(fmt.Sprintf("● %d", ok))+StyleMuted.Render(" ok"))
	}
	if degraded > 0 {
		chips = append(chips, StyleWarning.Render(fmt.Sprintf("◐ %d", degraded))+StyleMuted.Render(" incompleto"))
	}
	if down > 0 {
		chips = append(chips, StyleUnhealthy.Render(fmt.Sprintf("○ %d", down))+StyleMuted.Render(" parado"))
	}
	joined := strings.Join(chips, "  ") + " "
	// Sem espaço para os dois, a régua de tipos ganha — é a navegação.
	if len(chips) == 0 || lipgloss.Width(left)+lipgloss.Width(joined)+2 > width {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, joined, width)
}

func swarmKindLabel(k swarmKind) string {
	switch k {
	case swarmKindServices:
		return "Serviços"
	case swarmKindNodes:
		return "Nós"
	case swarmKindTasks:
		return "Tarefas"
	case swarmKindStacks:
		return "Stacks"
	case swarmKindNetworks:
		return "Redes"
	case swarmKindSecrets:
		return "Secrets"
	case swarmKindConfigs:
		return "Configs"
	case swarmKindEvents:
		return "Eventos"
	}
	return "Serviços"
}

func (a *App) swarmCountFor(k swarmKind) int {
	switch k {
	case swarmKindServices:
		return len(a.swarmServices)
	case swarmKindNodes:
		return len(a.swarmNodes)
	case swarmKindTasks:
		return len(a.swarmTasks)
	case swarmKindStacks:
		return len(a.swarmStacks)
	case swarmKindNetworks:
		return len(a.swarmNetworks)
	case swarmKindSecrets:
		return len(a.swarmSecrets)
	case swarmKindConfigs:
		return len(a.swarmConfigs)
	case swarmKindEvents:
		return len(a.swarmEvents)
	}
	return 0
}

// swarmServiceHealth lê "3/3" vs "1/2" vs "0/2" — é o que se procura numa lista
// de serviços do swarm.
func (a *App) swarmServiceHealth() (ok, degraded, down int) {
	for _, s := range a.swarmServices {
		running, want := swarmReplicaCounts(s.Replicas)
		switch {
		case want > 0 && running == 0:
			down++
		case want > 0 && running < want:
			degraded++
		default:
			ok++
		}
	}
	return
}

func swarmReplicaCounts(replicas string) (running, want int) {
	parts := strings.SplitN(strings.TrimSpace(replicas), "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	running, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	want, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	return
}

func swarmReplicaStyle(replicas string) lipgloss.Style {
	running, want := swarmReplicaCounts(replicas)
	switch {
	case want == 0:
		return StyleMuted
	case running == 0:
		return StyleUnhealthy
	case running < want:
		return StyleWarning
	default:
		return StyleHealthy
	}
}

func (a *App) renderSwarmTable(width, height int) string {
	title := strings.ToUpper(swarmKindLabel(a.swarmKind))
	n := a.swarmRowCount()
	if n > 0 {
		title = fmt.Sprintf("%s (%d)", title, n)
	}
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	inner2 := maxInt(8, width-2)
	lines := []string{
		a.swarmTableHeader(inner2),
		StyleMuted.Render(strings.Repeat("─", inner2)),
	}
	if n == 0 {
		msg := "nenhum item"
		if !a.swarmInfo.Active {
			msg = "swarm inactive — pressione i para init"
		}
		lines = append(lines, StyleMuted.Render("  "+msg))
		return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.swarmFocus == 0)
	}
	a.swarmScroll = ensureVisible(a.swarmCursor, a.swarmScroll, viewport, n)
	start := a.swarmScroll
	end := minInt(start+viewport, n)
	for i := start; i < end; i++ {
		lines = append(lines, a.renderSwarmRow(i, inner2, i == a.swarmCursor && a.swarmFocus == 0))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.swarmFocus == 0)
}

// swarmCols distribui as colunas pela largura útil. Antes o cabeçalho era uma
// string fixa e as linhas usavam %-Ns próprios — os dois discordavam.
type swarmCols struct{ mark, a, b, c, d, e int }

func (a *App) swarmColumns(width int) swarmCols {
	w := maxInt(30, width)
	switch a.swarmKind {
	case swarmKindServices:
		c := swarmCols{mark: 1, a: minInt(24, maxInt(10, w*20/100)), c: 10, d: 8}
		c.b = minInt(28, maxInt(10, w*22/100))
		c.e = maxInt(8, w-c.mark-c.a-c.b-c.c-c.d-5)
		return c
	case swarmKindNodes:
		c := swarmCols{a: minInt(20, maxInt(10, w*18/100)), b: 9, c: 8, d: 10}
		c.e = maxInt(8, w-c.a-c.b-c.c-c.d-4)
		return c
	case swarmKindTasks:
		c := swarmCols{a: minInt(24, maxInt(10, w*20/100)), b: minInt(18, maxInt(8, w*14/100)), c: 12, d: 9}
		c.e = maxInt(10, w-c.a-c.b-c.c-c.d-4)
		return c
	default:
		c := swarmCols{a: minInt(30, maxInt(12, w*26/100)), b: 12}
		c.c = maxInt(10, w-c.a-c.b-2)
		return c
	}
}

func (a *App) swarmTableHeader(width int) string {
	c := a.swarmColumns(width)
	head := StyleMuted.Bold(true)
	cell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return head.Render(padRight(truncate(t, n), n))
	}
	switch a.swarmKind {
	case swarmKindNodes:
		return joinNonEmpty(" ", cell("HOSTNAME", c.a), cell("PAPEL", c.b), cell("ESTADO", c.c),
			cell("DISPONIBIL.", c.d), cell("ENDEREÇO", c.e))
	case swarmKindServices:
		return joinNonEmpty(" ", cell("", c.mark), cell("SERVIÇO", c.a), cell("IMAGEM", c.b),
			cell("MODO", c.c), cell("RÉPLICAS", c.d), cell("PORTAS", c.e))
	case swarmKindTasks:
		return joinNonEmpty(" ", cell("TAREFA", c.a), cell("SERVIÇO", c.b), cell("NÓ", c.c),
			cell("DESEJADO", c.d), cell("ESTADO ATUAL", c.e))
	case swarmKindStacks:
		return joinNonEmpty(" ", cell("STACK", c.a), cell("SERVIÇOS", c.b), cell("ORQUESTRADOR", c.c))
	case swarmKindNetworks:
		return joinNonEmpty(" ", cell("REDE", c.a), cell("DRIVER", c.b), cell("ESCOPO", c.c))
	case swarmKindSecrets, swarmKindConfigs:
		return joinNonEmpty(" ", cell("NOME", c.a), cell("CRIADO", c.b+c.c))
	case swarmKindEvents:
		return joinNonEmpty(" ", cell("QUANDO", c.a), cell("TIPO", c.b), cell("O QUE ACONTECEU", c.c))
	}
	return ""
}

func (a *App) renderSwarmRow(i, width int, selected bool) string {
	c := a.swarmColumns(width)
	var cells []dashCell

	switch a.swarmKind {
	case swarmKindNodes:
		n := a.swarmNodes[i]
		role := strings.ToUpper(n.Role)
		if strings.EqualFold(n.Manager, "Leader") {
			role = "LEADER"
		}
		glyph, st := swarmNodeDot(n, a.animFrame)
		cells = []dashCell{
			{text: glyph + " " + n.Hostname, width: c.a, style: st},
			{text: role, width: c.b, style: swarmRoleStyle(role)},
			{text: n.Status, width: c.c, style: st},
			{text: n.Availability, width: c.d, style: swarmAvailStyle(n.Availability)},
			{text: firstNonEmpty(n.Addr, n.Engine, emDash), width: c.e, style: StyleMuted},
		}
	case swarmKindServices:
		s := a.swarmServices[i]
		mark := " "
		if collectors.SwarmBelongsToProject(s.Name, a.swarmProject) {
			mark = "▸" // pertence a este projeto
		}
		cells = []dashCell{
			{text: mark, width: c.mark, style: StyleAccent},
			{text: s.Name, width: c.a, style: StyleNormal.Bold(true)},
			{text: elideLeft(s.Image, maxInt(1, c.b)), width: c.b, style: StyleMuted},
			{text: s.Mode, width: c.c, style: StyleMuted},
			{text: s.Replicas, width: c.d, style: swarmReplicaStyle(s.Replicas)},
			{text: firstNonEmpty(s.Ports, emDash), width: c.e, style: lipgloss.NewStyle().Foreground(ColorAccent)},
		}
	case swarmKindTasks:
		t := a.swarmTasks[i]
		// O erro da tarefa é a razão de olhar a aba; era coletado e nunca exibido.
		current := t.CurrentState
		curStyle := StyleMuted
		if t.Error != "" {
			current = t.Error
			curStyle = StyleUnhealthy
		} else if swarmStateFailed(t.CurrentState) {
			curStyle = StyleUnhealthy
		}
		cells = []dashCell{
			{text: t.Name, width: c.a, style: StyleNormal},
			{text: t.Service, width: c.b, style: StyleMuted},
			{text: t.Node, width: c.c, style: StyleMuted},
			{text: t.DesiredState, width: c.d, style: StyleMuted},
			{text: current, width: c.e, style: curStyle},
		}
	case swarmKindStacks:
		st := a.swarmStacks[i]
		mark := ""
		if collectors.SwarmBelongsToProject(st.Name, a.swarmProject) {
			mark = "▸ "
		}
		cells = []dashCell{
			{text: mark + st.Name, width: c.a, style: StyleNormal.Bold(true)},
			{text: fmt.Sprintf("%d", st.Services), width: c.b, style: StyleMuted},
			{text: st.Orchestr, width: c.c, style: StyleMuted},
		}
	case swarmKindNetworks:
		n := a.swarmNetworks[i]
		cells = []dashCell{
			{text: n.Name, width: c.a, style: StyleNormal},
			{text: n.Driver, width: c.b, style: StyleMuted},
			{text: n.Scope, width: c.c, style: StyleMuted},
		}
	case swarmKindSecrets:
		sec := a.swarmSecrets[i]
		cells = []dashCell{
			{text: sec.Name, width: c.a, style: StyleNormal},
			{text: sec.CreatedAt, width: c.b + c.c, style: StyleMuted},
		}
	case swarmKindConfigs:
		cfg := a.swarmConfigs[i]
		cells = []dashCell{
			{text: cfg.Name, width: c.a, style: StyleNormal},
			{text: cfg.CreatedAt, width: c.b + c.c, style: StyleMuted},
		}
	case swarmKindEvents:
		e := a.swarmEvents[i]
		cells = []dashCell{
			{text: e.Time, width: c.a, style: StyleMuted},
			{text: e.Type, width: c.b, style: StyleMuted},
			{text: strings.TrimSpace(e.Action + "  " + e.Resource), width: c.c, style: StyleNormal},
		}
	}
	return renderCells(selected, cells)
}

func swarmNodeDot(n collectors.SwarmNode, frame int) (string, lipgloss.Style) {
	switch {
	case strings.EqualFold(n.Status, "Ready") && strings.EqualFold(n.Availability, "Active"):
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case strings.EqualFold(n.Status, "Ready"):
		return pulseGlyph(pulseWarn, frame), StyleWarning
	default:
		return pulseGlyph(pulseBad, frame), StyleUnhealthy
	}
}

func swarmRoleStyle(role string) lipgloss.Style {
	if role == "LEADER" || role == "MANAGER" {
		return lipgloss.NewStyle().Foreground(ColorAccent)
	}
	return StyleMuted
}

// swarmAvailStyle: Drain e Pause tiram o nó do escalonamento — some com as
// réplicas sem o serviço parecer quebrado.
func swarmAvailStyle(avail string) lipgloss.Style {
	if strings.EqualFold(avail, "Active") {
		return StyleMuted
	}
	return StyleWarning
}

func swarmStateFailed(state string) bool {
	s := strings.ToLower(state)
	return strings.Contains(s, "fail") || strings.Contains(s, "reject") ||
		strings.Contains(s, "orphan") || strings.Contains(s, "shutdown")
}

func (a *App) renderSwarmSummary(width, height int) string {
	inner := maxInt(2, height-2)
	body := a.swarmDetail
	title := "RESUMO"
	if a.swarmStatus != "" {
		title = "STATUS"
		if body == "" {
			body = a.swarmStatus
		}
	}
	if strings.TrimSpace(body) == "" {
		body = "enter detalhes  ·  s scale  ·  l logs  ·  [] recursos"
	}
	raw := strings.Split(body, "\n")
	viewport := maxInt(1, inner)
	a.swarmDetailScroll = clampScroll(a.swarmDetailScroll, viewport, len(raw))
	end := minInt(a.swarmDetailScroll+viewport, len(raw))
	lines := make([]string, 0, viewport)
	for i := a.swarmDetailScroll; i < end; i++ {
		line := sanitizeTerminalLine(raw[i])
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "failed") {
			lines = append(lines, StyleUnhealthy.Render(truncate(line, width-4)))
		} else {
			lines = append(lines, StyleMuted.Render(truncate(line, width-4)))
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderSwarmRightRail(width, height int) string {
	nodesH := maxInt(6, height*38/100)
	eventsH := maxInt(5, height*32/100)
	actionsH := maxInt(5, height-nodesH-eventsH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderSwarmNodesPanel(width, nodesH),
		a.renderSwarmEventsPanel(width, eventsH),
		a.renderSwarmActionsPanel(width, actionsH),
	)
}

func (a *App) renderSwarmNodesPanel(width, height int) string {
	online := 0
	for _, n := range a.swarmNodes {
		if strings.EqualFold(n.Status, "Ready") {
			online++
		}
	}
	title := fmt.Sprintf("NÓS %d/%d PRONTOS", online, len(a.swarmNodes))
	inner := maxInt(2, height-2)
	lines := []string{}
	managers := []collectors.SwarmNode{}
	workers := []collectors.SwarmNode{}
	for _, n := range a.swarmNodes {
		if n.Role == "manager" {
			managers = append(managers, n)
		} else {
			workers = append(workers, n)
		}
	}
	if len(managers) > 0 {
		lines = append(lines, StyleMuted.Render("MANAGERS"))
		for _, n := range managers {
			dot := a.swarmNodeDotStyled(n)
			if !strings.EqualFold(n.Status, "Ready") {
				dot = StyleUnhealthy.Render(pulseGlyph(pulseBad, a.animFrame))
			}
			role := n.Manager
			if role == "" {
				role = "Manager"
			}
			lines = append(lines, dot+" "+StyleNormal.Render(truncate(n.Hostname, width-8)))
			lines = append(lines, StyleMuted.Render("  "+truncate(role+" · "+n.Availability, width-6)))
		}
	}
	if len(workers) > 0 {
		lines = append(lines, StyleMuted.Render("WORKERS"))
		for _, n := range workers {
			dot := a.swarmNodeDotStyled(n)
			if !strings.EqualFold(n.Status, "Ready") {
				dot = StyleUnhealthy.Render(pulseGlyph(pulseBad, a.animFrame))
			}
			lines = append(lines, dot+" "+StyleNormal.Render(truncate(n.Hostname, width-8)))
			lines = append(lines, StyleMuted.Render("  "+truncate(n.Status+" · "+n.Availability, width-6)))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("sem nodes"))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.swarmFocus == 1)
}

func (a *App) renderSwarmEventsPanel(width, height int) string {
	inner := maxInt(2, height-2)
	lines := []string{}
	limit := minInt(12, len(a.swarmEvents))
	for i := 0; i < limit; i++ {
		e := a.swarmEvents[i]
		res := firstNonEmpty(e.Resource, e.Type)
		lines = append(lines, StyleHealthy.Render("●")+" "+StyleMuted.Render(truncate(res+"  "+e.Action, width-6)))
	}
	if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("sem eventos recentes"))
	}
	return renderApiTitledBox("EVENTS", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderSwarmActionsPanel(width, height int) string {
	items := a.swarmQuickActionItems()
	if a.swarmActionIdx >= len(items) {
		a.swarmActionIdx = maxInt(0, len(items)-1)
	}
	inner := maxInt(2, height-2)
	lines := make([]string, 0, len(items))
	for i, it := range items {
		prefix := "  "
		style := StyleMuted
		if i == a.swarmActionIdx && a.swarmFocus == 2 {
			prefix = StyleAccent.Render("› ")
			style = StyleNormal
		}
		lines = append(lines, prefix+StyleKey.Render(it[0])+" "+style.Render(it[1]))
	}
	return renderApiTitledBox("AÇÕES RÁPIDAS", fitExactLines(lines, inner), width, height, a.swarmFocus == 2)
}

func (a *App) swarmHints() string {
	if a.swarmConfirm {
		return "modal  y confirma  n/esc cancela"
	}
	if a.swarmScreen == swarmScrForm {
		return "form  enter confirma  esc cancela  tab campo"
	}
	if a.swarmScreen == swarmScrLogs {
		return "logs  f refresh  c clear  ↑↓ scroll  esc voltar"
	}
	if a.swarmScreen == swarmScrDetail {
		return "detalhe  l logs  s scale  u update  R force  b rollback  esc voltar"
	}
	if !a.swarmInfo.Active {
		return "i init  t token  r refresh  esc landing"
	}
	base := "[] recurso  enter detalhe  D remover  X leave swarm  tab painel  esc"
	if a.swarmKind == swarmKindNodes {
		base = "NODES  D/remover→leave se leader  X leave swarm  a avail  p promote  m demote  esc"
	}
	if a.swarmStatus != "" {
		return truncate(a.swarmStatus+"  ·  "+base, maxInt(40, a.width-4))
	}
	return base
}

func (a *App) renderSwarmFormBox() string {
	w := 56
	var lines []string
	title := "FORM"
	switch a.swarmForm {
	case swarmFormScale:
		title = "SCALE SERVICE"
		lines = []string{
			StyleMuted.Render("Service  ") + StyleNormal.Render(a.swarmFormName),
			"",
			StyleMuted.Render("New replicas:"),
			StyleSelected.Render("  [ " + a.swarmFormInput + " ]"),
			"",
			StyleMuted.Render("enter confirma  ·  esc cancela"),
		}
	case swarmFormUpdate:
		title = "UPDATE SERVICE"
		img := a.swarmFormImage
		rep := a.swarmFormReplicas
		if a.swarmFormField == 0 {
			img = a.swarmFormInput
		} else {
			rep = a.swarmFormInput
		}
		lines = []string{
			StyleMuted.Render("Service  ") + StyleNormal.Render(a.swarmFormName),
			swarmFormFieldLine(0, a.swarmFormField, "Image", img),
			swarmFormFieldLine(1, a.swarmFormField, "Replicas", rep),
			"",
			StyleMuted.Render("enter aplica  ·  tab campo  ·  esc cancela"),
		}
	case swarmFormCreate:
		title = "CREATE SERVICE"
		vals := []string{a.swarmFormName, a.swarmFormImage, a.swarmFormReplicas, a.swarmFormPort, a.swarmFormNetwork}
		labels := []string{"Name", "Image", "Replicas", "Publish", "Network"}
		vals[a.swarmFormField] = a.swarmFormInput
		lines = []string{}
		for i, lab := range labels {
			lines = append(lines, swarmFormFieldLine(i, a.swarmFormField, lab, vals[i]))
		}
		lines = append(lines, "", StyleMuted.Render("enter cria  ·  tab campo  ·  esc cancela"))
	case swarmFormDeploy:
		title = "DEPLOY STACK"
		lines = []string{
			StyleMuted.Render("Compose  ") + StyleNormal.Render(truncate(a.swarmCompose, 40)),
			StyleMuted.Render("Stack    ") + StyleNormal.Render(a.swarmFormName),
			"",
			StyleMuted.Render("Preview:"),
			StyleNormal.Render("docker stack deploy -c " + swarmComposeBase(a.swarmCompose) + " " + a.swarmFormName),
			"",
			StyleMuted.Render("y/enter deploy  ·  n/esc cancela"),
		}
	case swarmFormInit:
		title = "SWARM INIT"
		lines = []string{
			StyleMuted.Render("Docker is running but Swarm may be inactive."),
			"",
			StyleMuted.Render("Advertise Address (opcional):"),
			StyleSelected.Render("  [ " + a.swarmFormInput + " ]"),
			"",
			StyleMuted.Render("enter initialize  ·  esc cancela"),
		}
	case swarmFormToken:
		title = "JOIN CLUSTER · " + strings.ToUpper(a.swarmFormName)
		body := a.swarmDetail
		if body == "" {
			body = a.spinner() + " carregando…"
		}
		for _, ln := range strings.Split(body, "\n") {
			lines = append(lines, StyleMuted.Render(truncate(ln, w-6)))
		}
		lines = append(lines, "", StyleMuted.Render("w worker  ·  m manager  ·  esc fecha"))
	case swarmFormAvail:
		title = "NODE AVAILABILITY"
		lines = []string{
			StyleMuted.Render("Node  ") + StyleNormal.Render(a.swarmFormName),
			"",
			StyleMuted.Render("Availability:"),
			StyleSelected.Render("  [ " + a.swarmFormAvail + " ]"),
			StyleMuted.Render("  ←→ active · pause · drain"),
			"",
			StyleMuted.Render("enter aplica  ·  esc cancela"),
		}
	}
	inner := fitExactLines(lines, maxInt(6, len(lines)))
	return renderApiTitledBox(title, inner, w, len(inner)+2, true)
}

func swarmFormFieldLine(idx, cur int, label, value string) string {
	lab := StyleMuted.Render(fmt.Sprintf("%-10s", label))
	if idx == cur {
		return lab + StyleSelected.Render("[ "+value+" ]")
	}
	return lab + StyleNormal.Render("  "+value)
}

func (a *App) renderSwarmLogsBox(termW, termH int) string {
	w := minInt(termW-4, 80)
	h := minInt(termH-4, 24)
	inner := maxInt(4, h-2)
	raw := strings.Split(a.swarmDetail, "\n")
	if a.swarmLogs != "" {
		raw = strings.Split(a.swarmLogs, "\n")
	}
	// reuse detail body filled by swarmDetailMsg while in logs screen
	a.swarmDetailScroll = clampScroll(a.swarmDetailScroll, inner, len(raw))
	end := minInt(a.swarmDetailScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	name := a.swarmSelectedName()
	for i := a.swarmDetailScroll; i < end; i++ {
		ln := sanitizeTerminalLine(raw[i])
		low := strings.ToLower(ln)
		switch {
		case strings.Contains(low, "error"), strings.Contains(low, "fatal"):
			lines = append(lines, StyleUnhealthy.Render(truncate(ln, w-4)))
		case strings.Contains(low, "warn"):
			lines = append(lines, StyleWarning.Render(truncate(ln, w-4)))
		default:
			lines = append(lines, StyleMuted.Render(truncate(ln, w-4)))
		}
	}
	title := "SERVICE LOGS"
	if name != "" {
		title += " · " + name
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), w, h, true)
}

func (a *App) renderSwarmDetailBox(termW, termH int) string {
	w := minInt(termW-4, 84)
	h := minInt(termH-4, 28)
	inner := maxInt(6, h-2)
	raw := strings.Split(a.swarmDetail, "\n")
	a.swarmDetailScroll = clampScroll(a.swarmDetailScroll, inner, len(raw))
	end := minInt(a.swarmDetailScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.swarmDetailScroll; i < end; i++ {
		lines = append(lines, StyleMuted.Render(truncate(sanitizeTerminalLine(raw[i]), w-4)))
	}
	name := a.swarmSelectedName()
	title := "DETAILS"
	if name != "" {
		title = strings.ToUpper(a.swarmKind.String()) + " · " + name
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), w, h, true)
}

func (a *App) swarmNodeDotStyled(n collectors.SwarmNode) string {
	glyph, st := swarmNodeDot(n, a.animFrame)
	return st.Render(glyph)
}
