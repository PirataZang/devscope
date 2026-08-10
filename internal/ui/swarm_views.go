package ui

import (
	"fmt"
	"strings"
	"time"

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
			StyleHealthy.Render("● ACTIVE")+
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
	status := a.renderSwarmStatusRow(w)
	cards := a.renderSwarmCards(w)
	tabs := a.renderSwarmKindTabs(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(status) + lipgloss.Height(cards) + lipgloss.Height(tabs) + 2
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
	return lipgloss.JoinVertical(lipgloss.Left, header, status, cards, tabs, body, a.renderStatusBar(hints))
}

func (a *App) renderSwarmHeader(width int, p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabSwarm)).Bold(true)
	proj := "project"
	if p != nil && p.Name != "" {
		proj = p.Name
	} else if a.swarmProject != "" {
		proj = a.swarmProject
	}
	left := accent.Render("DOCKER SWARM") + StyleMuted.Render(" › CLUSTER") +
		StyleMuted.Render("  ·  ") + StyleNormal.Render(truncate(proj, 20))
	right := StyleMuted.Render(time.Now().Format("15:04:05"))
	if a.swarmLoading {
		right = a.loadingMuted("Loading…")
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderSwarmStatusRow(width int) string {
	var badge string
	switch {
	case a.swarmInfo.State == "unavailable" || (!collectors.SwarmAvailable() && a.swarmErr != ""):
		badge = StyleUnhealthy.Render("✕ UNAVAILABLE")
	case a.swarmInfo.State == "degraded":
		badge = StyleWarning.Render("⚠ DEGRADED")
	case a.swarmInfo.Active:
		badge = StyleHealthy.Render("● ACTIVE")
	default:
		badge = StyleMuted.Render("○ INACTIVE")
	}
	meta := StyleMuted.Render(fmt.Sprintf("  %d mgr  %d workers  cluster %s",
		a.swarmInfo.Managers, a.swarmInfo.Workers, truncate(firstNonEmpty(a.swarmInfo.ClusterID, "—"), 12)))
	if a.swarmInfo.EngineVersion != "" {
		meta += StyleMuted.Render("  engine " + a.swarmInfo.EngineVersion)
	}
	line := badge + meta
	if a.swarmErr != "" {
		line += StyleMuted.Render("  ") + StyleUnhealthy.Render(truncate(a.swarmErr, 28))
	}
	return truncate(line, width)
}

func (a *App) renderSwarmCards(width int) string {
	boxW := maxInt(10, width/6)
	cards := []struct {
		title, value string
		style        lipgloss.Style
	}{
		{"MANAGERS", fmt.Sprintf("%d", a.swarmInfo.Managers), StyleAccent},
		{"WORKERS", fmt.Sprintf("%d", a.swarmInfo.Workers), StyleNormal},
		{"SERVICES", fmt.Sprintf("%d", len(a.swarmServices)), StyleAccent},
		{"TASKS", fmt.Sprintf("%d", len(a.swarmTasks)), StyleNormal},
		{"NETWORKS", fmt.Sprintf("%d", len(a.swarmNetworks)), StyleMuted},
		{"STACKS", fmt.Sprintf("%d", len(a.swarmStacks)), StyleMuted},
	}
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		val := c.style.Render(truncate(c.value, boxW-4))
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines([]string{val}, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderSwarmKindTabs(width int) string {
	kinds := []swarmKind{
		swarmKindServices, swarmKindNodes, swarmKindTasks, swarmKindStacks,
		swarmKindNetworks, swarmKindSecrets, swarmKindConfigs, swarmKindEvents,
	}
	labels := make([]string, len(kinds))
	for i, k := range kinds {
		labels[i] = strings.ToUpper(k.String())
	}
	// Compact labels if full names don't fit the terminal width.
	fullW := 0
	for _, lab := range labels {
		fullW += len(lab) + 3 // spaces + separator
	}
	if width > 0 && fullW > width {
		short := []string{"SVC", "NODES", "TASKS", "STACKS", "NETS", "SECR", "CFGS", "EVTS"}
		copy(labels, short)
	}
	parts := make([]string, 0, len(kinds))
	for i, k := range kinds {
		label := " " + labels[i] + " "
		if k == a.swarmKind {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	line := strings.Join(parts, StyleMuted.Render("│"))
	pad := width - lipgloss.Width(stripANSI(line))
	if pad < 0 {
		pad = 0
	}
	return line + strings.Repeat(" ", pad)
}

func (a *App) renderSwarmTable(width, height int) string {
	title := strings.ToUpper(a.swarmKind.String())
	n := a.swarmRowCount()
	if n > 0 {
		title = fmt.Sprintf("%s (%d)", title, n)
	}
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	lines := []string{
		StyleTableHeader.Render(truncate(a.swarmTableHeader(), width-4)),
		StyleMuted.Render(strings.Repeat("─", maxInt(8, width-6))),
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
		lines = append(lines, a.renderSwarmRow(i, width-4, i == a.swarmCursor && a.swarmFocus == 0))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.swarmFocus == 0)
}

func (a *App) swarmTableHeader() string {
	switch a.swarmKind {
	case swarmKindNodes:
		return "HOSTNAME          ROLE      STATUS   AVAIL     ENGINE"
	case swarmKindServices:
		return "NAME                 IMAGE                    MODE        REPLICAS  STATUS   PORTS"
	case swarmKindTasks:
		return "TASK                 SERVICE        NODE         DESIRED   CURRENT"
	case swarmKindStacks:
		return "STACK                SERVICES   ORCHESTRATOR"
	case swarmKindNetworks:
		return "NETWORK              DRIVER     SCOPE"
	case swarmKindSecrets:
		return "NAME                            CREATED"
	case swarmKindConfigs:
		return "NAME                            CREATED"
	case swarmKindEvents:
		return "TIME                 TYPE       ACTION              RESOURCE"
	}
	return ""
}

func (a *App) renderSwarmRow(i, width int, selected bool) string {
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	projMark := ""
	var text string
	switch a.swarmKind {
	case swarmKindNodes:
		n := a.swarmNodes[i]
		role := strings.ToUpper(n.Role)
		if strings.EqualFold(n.Manager, "Leader") {
			role = "LEADER"
		}
		st := n.Status
		text = fmt.Sprintf("%-16s  %-8s  %-7s  %-8s  %s",
			truncate(n.Hostname, 16), truncate(role, 8), truncate(st, 7), truncate(n.Availability, 8), truncate(n.Engine, 8))
	case swarmKindServices:
		s := a.swarmServices[i]
		if collectors.SwarmBelongsToProject(s.Name, a.swarmProject) {
			projMark = "·"
		}
		st := collectors.SwarmServiceStatus(s.Replicas)
		text = fmt.Sprintf("%s%-18s  %-22s  %-10s  %-8s  %-7s  %s",
			projMark, truncate(s.Name, 18), truncate(s.Image, 22), truncate(s.Mode, 10),
			truncate(s.Replicas, 8), truncate(st, 7), truncate(s.Ports, 16))
	case swarmKindTasks:
		t := a.swarmTasks[i]
		text = fmt.Sprintf("%-18s  %-12s  %-12s  %-8s  %s",
			truncate(t.Name, 18), truncate(t.Service, 12), truncate(t.Node, 12),
			truncate(t.DesiredState, 8), truncate(t.CurrentState, 16))
	case swarmKindStacks:
		s := a.swarmStacks[i]
		if collectors.SwarmBelongsToProject(s.Name, a.swarmProject) {
			projMark = "·"
		}
		text = fmt.Sprintf("%s%-20s  %-9d  %s", projMark, truncate(s.Name, 20), s.Services, truncate(s.Orchestr, 12))
	case swarmKindNetworks:
		n := a.swarmNetworks[i]
		text = fmt.Sprintf("%-18s  %-9s  %s", truncate(n.Name, 18), truncate(n.Driver, 9), truncate(n.Scope, 8))
	case swarmKindSecrets:
		s := a.swarmSecrets[i]
		text = fmt.Sprintf("%-28s  %s", truncate(s.Name, 28), truncate(s.CreatedAt, 20))
	case swarmKindConfigs:
		c := a.swarmConfigs[i]
		text = fmt.Sprintf("%-28s  %s", truncate(c.Name, 28), truncate(c.CreatedAt, 20))
	case swarmKindEvents:
		e := a.swarmEvents[i]
		text = fmt.Sprintf("%-18s  %-9s  %-18s  %s",
			truncate(e.Time, 18), truncate(e.Type, 9), truncate(e.Action, 18), truncate(e.Resource, 20))
	}
	return style.Width(width).MaxWidth(width).Render(truncate(text, width))
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
	title := fmt.Sprintf("NODES %d/%d ONLINE", online, len(a.swarmNodes))
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
			dot := StyleHealthy.Render("●")
			if !strings.EqualFold(n.Status, "Ready") {
				dot = StyleUnhealthy.Render("●")
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
			dot := StyleHealthy.Render("●")
			if !strings.EqualFold(n.Status, "Ready") {
				dot = StyleUnhealthy.Render("○")
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
			body = "carregando…"
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
