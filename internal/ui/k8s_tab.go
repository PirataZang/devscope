package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type k8sKind int

const (
	k8sKindPods k8sKind = iota
	k8sKindDeploys
	k8sKindServices
	k8sKindManifests
)

type k8sSubTab int

const (
	k8sTabOverview k8sSubTab = iota
	k8sTabWorkloads
	k8sTabNetworking
	k8sTabConfig
	k8sTabEvents
)

type k8sFocus int

const (
	// k8sFocusExplorer saiu junto com a coluna do explorer; o tipo de recurso
	// agora se troca por [ e ], que aparecem na régua.
	k8sFocusTable k8sFocus = iota
	k8sFocusLogs
	k8sFocusYAML
	k8sFocusDetail
)

type k8sPane int

const (
	k8sPaneList k8sPane = iota
	k8sPaneDetail
	k8sPaneEditor
)

type k8sLoadedMsg struct {
	resources []collectors.K8sResource
	manifests []string
	err       string
}

type k8sActionMsg struct {
	out string
	err string
}

type k8sDetailMsg struct {
	body string
	err  string
}

type k8sNsMsg struct {
	ns  string
	err string
}

type k8sEditReadyMsg struct {
	yaml   string
	status string
	err    string
}

type k8sInspectMsg struct {
	name   string
	detail string
	logs   string
	yaml   string
	events string
	err    string
}

type k8sMetaMsg struct {
	version string
	nodes   int
}

func (a *App) enterK8sTab(_ *core.Project) {
	a.tab = TabKubernetes
	a.tabCursor = 0
	a.k8sOpen = false
	a.k8sEditing = false
	a.k8sConfirmDelete = false
	a.k8sFilterOn = false
}

func (a *App) openK8sClient(p *core.Project) tea.Cmd {
	a.k8sOpen = true
	a.k8sEditing = false
	a.k8sConfirmDelete = false
	a.k8sFilterOn = false
	a.k8sFilter = ""
	a.k8sPane = k8sPaneList
	a.k8sFocus = k8sFocusTable
	a.k8sSubTab = k8sTabOverview
	a.k8sKind = k8sKindPods
	a.k8sCursor = 0
	a.k8sScroll = 0
	a.k8sDetailScroll = 0
	a.k8sLogsScroll = 0
	a.k8sYAMLScroll = 0
	a.k8sDetail = ""
	a.k8sLogs = ""
	a.k8sYAML = ""
	a.k8sEvents = ""
	a.k8sErr = ""
	a.k8sStatus = ""
	a.k8sInspectName = ""
	if a.k8sNamespace == "" {
		a.k8sNamespace = "default"
	}
	a.k8sContext = collectors.K8sCurrentContext()
	a.k8sManifests = collectors.DiscoverProjectManifests(p.Path)
	return tea.Batch(a.refreshK8s(p), a.loadK8sMeta())
}

func (a *App) leaveK8sTab() tea.Cmd {
	a.k8sOpen = false
	a.k8sEditing = false
	a.k8sConfirmDelete = false
	a.k8sFilterOn = false
	a.tab = TabKubernetes
	a.tabCursor = 0
	return nil
}

func (a *App) renderK8sLanding(p *core.Project) string {
	available, kctx, n := a.landingK8sAvail, a.landingK8sCtx, a.landingK8sManifests
	state := landingToolState(a.landingK8sOK, available, "kubectl", "")
	if a.landingK8sOK && available && kctx != "" {
		state = StyleHealthy.Render(a.okPulse() + " kubectl conectado")
	}
	var facts [][2]string
	if a.landingK8sOK && available {
		facts = [][2]string{
			{"contexto", StyleNormal.Render(firstNonEmpty(truncate(kctx, 40), emDash))},
			{"manifest", StyleNormal.Render(fmt.Sprintf("%d", n)) + StyleMuted.Render("  em k8s/ do projeto")},
		}
	}
	return a.renderModuleLanding(p, moduleLanding{
		title:        "KUBERNETES",
		tagline:      "pods, deployments e services do contexto atual — logs, yaml e apply",
		state:        state,
		facts:        facts,
		previewTitle: "MANIFESTS DESTE PROJETO",
		preview:      a.landingFileRows(a.landingK8sNames),
		previewEmpty: "nenhum .yaml em k8s/, kubernetes/, manifests/ ou deploy/",
		previewFoot:  k8sManifestFoot(n),
		actions:      [][2]string{{"enter", "abrir console"}, {"r", "refresh"}, {"esc", "voltar"}},
	})
}

func (a *App) loadK8sMeta() tea.Cmd {
	return func() tea.Msg {
		m := collectors.K8sClusterMetaInfo()
		return k8sMetaMsg{version: m.Version, nodes: m.Nodes}
	}
}

func (a *App) refreshK8s(p *core.Project) tea.Cmd {
	a.k8sLoading = true
	a.k8sErr = ""
	ns := a.k8sNamespace
	kind := a.k8sKind
	path := ""
	if p != nil {
		path = p.Path
	}
	return func() tea.Msg {
		var (
			resources []collectors.K8sResource
			err       error
		)
		switch kind {
		case k8sKindPods:
			resources, err = collectors.K8sListPods(ns)
		case k8sKindDeploys:
			resources, err = collectors.K8sListDeployments(ns)
		case k8sKindServices:
			resources, err = collectors.K8sListServices(ns)
		case k8sKindManifests:
			return k8sLoadedMsg{manifests: collectors.DiscoverProjectManifests(path)}
		}
		if err != nil {
			return k8sLoadedMsg{err: err.Error()}
		}
		return k8sLoadedMsg{resources: resources, manifests: collectors.DiscoverProjectManifests(path)}
	}
}

func (a *App) handleK8sMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case k8sLoadedMsg:
		a.k8sLoading = false
		if m.err != "" {
			a.k8sErr = m.err
			a.k8sResources = nil
			return a, nil
		}
		a.k8sErr = ""
		a.k8sResources = m.resources
		if len(m.manifests) > 0 {
			a.k8sManifests = m.manifests
		}
		max := a.k8sListLen() - 1
		if a.k8sCursor > max {
			a.k8sCursor = maxInt(0, max)
		}
		return a, a.k8sInspectSelected()
	case k8sActionMsg:
		a.k8sLoading = false
		a.k8sConfirmDelete = false
		if m.err != "" {
			a.k8sErr = m.err
			a.k8sStatus = ""
			return a, nil
		}
		a.k8sErr = ""
		a.k8sStatus = truncate(m.out, 80)
		return a, a.refreshK8s(a.currentProject())
	case k8sDetailMsg:
		a.k8sLoading = false
		a.k8sDetailScroll = 0
		if m.err != "" {
			a.k8sErr = m.err
			a.k8sDetail = m.body
			return a, nil
		}
		a.k8sErr = ""
		a.k8sDetail = m.body
		a.k8sPane = k8sPaneDetail
		a.k8sFocus = k8sFocusDetail
	case k8sInspectMsg:
		if m.name != "" && m.name != a.k8sInspectName {
			return a, nil
		}
		a.k8sLoading = false
		if m.err != "" {
			a.k8sErr = m.err
		}
		if m.detail != "" {
			a.k8sDetail = m.detail
		}
		a.k8sLogs = m.logs
		if !a.k8sEditing {
			a.k8sYAML = m.yaml
		}
		if m.events != "" {
			a.k8sEvents = m.events
		}
	case k8sMetaMsg:
		a.k8sVersion = m.version
		a.k8sNodeCount = m.nodes
	case k8sNsMsg:
		a.k8sLoading = false
		if m.err != "" {
			a.k8sErr = m.err
			return a, nil
		}
		a.k8sNamespace = m.ns
		a.k8sStatus = "ns → " + m.ns
		a.k8sCursor = 0
		return a, a.refreshK8s(a.currentProject())
	case k8sEditReadyMsg:
		a.k8sLoading = false
		if m.err != "" {
			a.k8sErr = m.err
			return a, nil
		}
		a.k8sYAML = m.yaml
		a.k8sEditorCursor = 0
		a.k8sEditing = true
		a.k8sPane = k8sPaneEditor
		a.k8sFocus = k8sFocusYAML
		a.k8sDetailScroll = 0
		a.k8sYAMLScroll = 0
		a.k8sErr = ""
		if m.status != "" {
			a.k8sStatus = m.status + " · ctrl+s aplica"
		} else {
			a.k8sStatus = "editando · ctrl+s aplica"
		}
	}
	return a, nil
}

func (a *App) k8sFilteredResources() []collectors.K8sResource {
	if a.k8sFilter == "" {
		return a.k8sResources
	}
	q := strings.ToLower(a.k8sFilter)
	var out []collectors.K8sResource
	for _, r := range a.k8sResources {
		if strings.Contains(strings.ToLower(r.Name), q) || strings.Contains(strings.ToLower(r.Status), q) {
			out = append(out, r)
		}
	}
	return out
}

func (a *App) k8sFilteredManifests() []string {
	if a.k8sFilter == "" {
		return a.k8sManifests
	}
	q := strings.ToLower(a.k8sFilter)
	var out []string
	for _, m := range a.k8sManifests {
		if strings.Contains(strings.ToLower(filepath.Base(m)), q) {
			out = append(out, m)
		}
	}
	return out
}

func (a *App) k8sListLen() int {
	if a.k8sKind == k8sKindManifests {
		return len(a.k8sFilteredManifests())
	}
	return len(a.k8sFilteredResources())
}

func (a *App) k8sSelectedResource() (collectors.K8sResource, bool) {
	items := a.k8sFilteredResources()
	if a.k8sCursor < 0 || a.k8sCursor >= len(items) {
		return collectors.K8sResource{}, false
	}
	return items[a.k8sCursor], true
}

func (a *App) k8sSelectedManifest() (string, bool) {
	items := a.k8sFilteredManifests()
	if a.k8sCursor < 0 || a.k8sCursor >= len(items) {
		return "", false
	}
	return items[a.k8sCursor], true
}

func (a *App) renderK8sTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderK8sHeader(w)
	tabs := a.renderK8sSubTabs(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(tabs)
	bodyH := maxInt(10, h-headerH-2)

	var body string
	if a.k8sEditing {
		body = a.renderK8sEditor(w, bodyH)
	} else if a.k8sSubTab == k8sTabEvents {
		body = a.renderK8sEventsView(w, bodyH)
	} else {
		body = a.renderK8sOverview(w, bodyH)
	}

	rel := a.renderK8sRelation(w)
	hints := a.k8sHints()
	view := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, rel, a.renderStatusBar(hints))
	if a.k8sConfirmDelete {
		target, detail := a.k8sDeleteConfirmLabels()
		box := renderDeleteConfirmBox(deleteConfirmOpts{
			Brand:    "KUBERNETES",
			Color:    tabAccentColor(TabKubernetes),
			Title:    "Excluir recurso",
			Subtitle: "kubectl delete no cluster",
			Label:    "recurso",
			Target:   target,
			Detail:   detail,
		}, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) k8sDeleteConfirmLabels() (target, detail string) {
	r, ok := a.k8sSelectedResource()
	if !ok {
		return "—", ""
	}
	detail = strings.ToLower(r.Kind)
	if a.k8sNamespace != "" {
		detail += "  ·  ns " + a.k8sNamespace
	}
	return r.Name, detail
}

func (a *App) k8sHints() string {
	if a.k8sConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.k8sEditing {
		return "editando YAML  enter=nova linha  ctrl+s=aplicar  esc=pausar"
	}
	if a.k8sFilterOn {
		return "filter  enter aplicar  esc limpar  ·  " + a.k8sFilter + "█"
	}
	base := "0-4 seção  ·  [ ] tipo  ·  n/p namespace  ·  b filtrar  ·  tab painel  ·  enter detalhe  ·  l logs  ·  y yaml  ·  e editar  ·  d excluir  ·  r atualizar  ·  esc"
	if a.k8sLoading {
		base = a.spinner() + " carregando…  " + base
	}
	if a.k8sStatus != "" {
		return truncate(a.k8sStatus, 40) + "  ·  " + base
	}
	return base
}

// renderK8sHeader põe contexto › namespace em destaque: rodar kubectl no
// contexto errado é o acidente clássico, e o aviso de produção fica em vermelho.
func (a *App) renderK8sHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabKubernetes)).Bold(true)
	ctx := firstNonEmpty(a.k8sContext, "?")
	ctxStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	if k8sContextLooksProd(ctx) {
		ctxStyle = StyleUnhealthy.Bold(true)
	}
	left := accent.Render("⎈ KUBERNETES") + "   " +
		ctxStyle.Render(truncate(ctx, 34)) +
		StyleMuted.Render(" › ") +
		StyleNormal.Bold(true).Render(truncate(firstNonEmpty(a.k8sNamespace, "default"), 20))
	if k8sContextLooksProd(ctx) {
		left += StyleUnhealthy.Render("  ⚠ produção")
	}

	var right []string
	if a.k8sVersion != "" {
		right = append(right, StyleMuted.Render(a.k8sVersion))
	}
	if a.k8sNodeCount > 0 {
		right = append(right, StyleMuted.Render(fmt.Sprintf("%d nós", a.k8sNodeCount)))
	}
	if a.k8sLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	if a.k8sErr != "" {
		right = append(right, StyleUnhealthy.Render(truncate(a.k8sErr, 30)))
	}
	if len(right) == 0 {
		return truncateVisible(left, width)
	}
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// k8sContextLooksProd marca contextos que parecem produção. Heurística de
// nome — é o que se tem sem consultar a API.
func k8sContextLooksProd(ctx string) bool {
	c := strings.ToLower(ctx)
	for _, needle := range []string{"prod", "prd", "live"} {
		if strings.Contains(c, needle) {
			return !strings.Contains(c, "nonprod") && !strings.Contains(c, "non-prod")
		}
	}
	return false
}

// renderK8sSubTabs devolve duas linhas: as seções (teclas 0-4) e os tipos de
// recurso com contagem. O seletor de tipo era uma coluna de 26 colunas ao lado
// da tabela — que é justamente quem precisa de largura.
func (a *App) renderK8sSubTabs(width int) string {
	names := []string{"VISÃO GERAL", "WORKLOADS", "REDE", "CONFIG", "EVENTOS"}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		label := fmt.Sprintf(" %d %s ", i, n)
		if k8sSubTab(i) == a.k8sSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	first := padRightVisible(strings.Join(parts, StyleMuted.Render("│")), width)
	return lipgloss.JoinVertical(lipgloss.Left, first, a.renderK8sKindStrip(width))
}

// renderK8sKindStrip: a contagem saiu (só o tipo carregado tinha uma, e a caixa
// logo abaixo já diz "PODS (4)"), o separador virou o mesmo da linha de cima, e
// o tipo ativo é sublinhado em vez de bloco invertido — dois blocos invertidos
// empilhados não deixavam claro qual era o atual.
func (a *App) renderK8sKindStrip(width int) string {
	kinds := []struct {
		kind  k8sKind
		label string
	}{
		{k8sKindPods, "PODS"}, {k8sKindDeploys, "DEPLOYMENTS"},
		{k8sKindServices, "SERVICES"}, {k8sKindManifests, "MANIFESTOS"},
	}
	active := lipgloss.NewStyle().Foreground(tabAccentColor(TabKubernetes)).Bold(true).Underline(true)
	parts := make([]string, 0, len(kinds))
	for _, item := range kinds {
		if item.kind == a.k8sKind {
			parts = append(parts, active.Render(" "+item.label+" "))
		} else {
			parts = append(parts, StyleMuted.Render(" "+item.label+" "))
		}
	}
	// Alinha com a régua de seções logo acima (mesmo recuo, mesmo separador).
	left := strings.Join(parts, StyleMuted.Render("│")) +
		StyleMuted.Render("   ") + StyleKey.Render("[ ]") + StyleMuted.Render(" troca o tipo")

	var chips []string
	running, pending, failed := a.k8sPodHealth()
	if running > 0 {
		chips = append(chips, StyleHealthy.Render(fmt.Sprintf("● %d", running))+StyleMuted.Render(" running"))
	}
	if pending > 0 {
		chips = append(chips, StyleWarning.Render(fmt.Sprintf("◐ %d", pending))+StyleMuted.Render(" pending"))
	}
	if failed > 0 {
		chips = append(chips, StyleUnhealthy.Render(fmt.Sprintf("✕ %d", failed))+StyleMuted.Render(" com falha"))
	}
	if a.k8sFilter != "" {
		chips = append(chips, StyleWarning.Render("filtro "+truncate(a.k8sFilter, 16)))
	}
	joined := strings.Join(chips, "  ") + " "
	if len(chips) == 0 || lipgloss.Width(left)+lipgloss.Width(joined)+2 > width {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, joined, width)
}

func (a *App) k8sPodHealth() (running, pending, failed int) {
	if a.k8sKind != k8sKindPods {
		return
	}
	for _, r := range a.k8sResources {
		switch r.Status {
		case "Running", "Succeeded", "Completed":
			running++
		case "Pending", "ContainerCreating", "Terminating":
			pending++
		default:
			failed++
		}
	}
	return
}

// renderK8sOverview: tabela em largura cheia. Antes eram quatro colunas lado a
// lado (explorer, tabela, detalhe, ações) e sobravam ~70 colunas para a tabela,
// que é onde o nome do pod precisa caber inteiro.
func (a *App) renderK8sOverview(width, height int) string {
	cmdW := actionsCmdWidth(width)
	mainW := maxInt(40, width-cmdW)

	bottomH := maxInt(7, height*34/100)
	tableH := maxInt(6, height-bottomH)

	detailW := maxInt(28, mainW*34/100)
	rest := maxInt(20, mainW-detailW)
	logsW := rest / 2

	main := lipgloss.JoinVertical(lipgloss.Left,
		a.renderK8sTable(mainW, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderK8sDetailPane(detailW, bottomH),
			a.renderK8sLogsPane(logsW, bottomH),
			a.renderK8sYAMLPane(rest-logsW, bottomH),
		),
	)
	actions := renderActionsBox(cmdW, height,
		[2]string{"enter", "detalhe"},
		[2]string{"l", "logs"},
		[2]string{"y", "yaml"},
		[2]string{"e", "editar"},
		[2]string{"c", "criar"},
		[2]string{"d", "excluir"},
		[2]string{"n/p", "namespace"},
		[2]string{"b", "filtrar"},
		[2]string{"r", "atualizar"},
		[2]string{"tab", "painel"},
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, main, actions)
}

func (a *App) renderK8sTable(width, height int) string {
	focus := a.k8sFocus == k8sFocusTable && !a.k8sEditing
	n := a.k8sListLen()
	inner := maxInt(20, width-2)
	viewport := maxInt(1, height-4)
	a.k8sScroll = ensureVisible(a.k8sCursor, a.k8sScroll, viewport, n)

	lines := []string{
		"  " + a.k8sTableHeader(inner-2),
		rule(inner),
	}
	if n == 0 {
		if a.k8sLoading {
			lines = append(lines, "", "  "+a.loadingMuted("consultando o cluster…"))
		} else {
			lines = append(lines, "", "  "+StyleMuted.Render("nada em ")+
				StyleNormal.Render(firstNonEmpty(a.k8sNamespace, "default"))+
				StyleMuted.Render(" — ")+StyleKey.Render("n/p")+StyleMuted.Render(" troca de namespace"))
		}
	} else {
		for i := a.k8sScroll; i < minInt(a.k8sScroll+viewport, n); i++ {
			lines = append(lines, a.renderK8sRow(i, inner-2, i == a.k8sCursor, focus))
		}
	}
	return panelBox(a.k8sTableTitle(n), fitExactLines(lines, maxInt(1, height-2)), width, height, focus)
}

type k8sCols struct{ dot, name, status, ready, restarts, node, ip, age int }

func (a *App) k8sColumns(width int) k8sCols {
	w := maxInt(30, width)
	switch a.k8sKind {
	case k8sKindPods:
		c := k8sCols{dot: 1, status: 18, ready: 6, restarts: 9, age: 5}
		c.node = minInt(20, maxInt(0, w*14/100))
		c.ip = minInt(15, maxInt(0, w*10/100))
		if w < 96 {
			c.ip = 0
		}
		if w < 80 {
			c.node = 0
		}
		used := c.dot + c.status + c.ready + c.restarts + c.node + c.ip + c.age
		gaps := 5 // 6 colunas fixas → 5 separadores
		if c.node > 0 {
			gaps++
		}
		if c.ip > 0 {
			gaps++
		}
		// Nome de pod raramente passa de ~44; a sobra vale mais no nó e no IP,
		// que são o que se cruza quando um nó está com problema.
		c.name = maxInt(16, w-used-gaps)
		if extra := c.name - 44; extra > 0 {
			c.name = 44
			if c.node > 0 {
				grow := minInt(extra, 12)
				c.node += grow
				extra -= grow
			}
			if c.ip > 0 && extra > 0 {
				c.ip += minInt(extra, 4)
			}
		}
		return c
	case k8sKindDeploys:
		c := k8sCols{dot: 1, ready: 10, age: 6}
		c.name = maxInt(16, w-c.dot-c.ready-c.age-3)
		return c
	case k8sKindServices:
		c := k8sCols{dot: 1, status: 14, ip: 18, age: 6}
		c.name = maxInt(16, w-c.dot-c.status-c.ip-c.age-4)
		return c
	default:
		return k8sCols{name: w}
	}
}

func (a *App) k8sTableHeader(width int) string {
	c := a.k8sColumns(width)
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
	switch a.k8sKind {
	case k8sKindPods:
		return joinNonEmpty(" ", cell("", c.dot), cell("NOME", c.name), cell("ESTADO", c.status),
			rcell("READY", c.ready), rcell("RESTARTS", c.restarts), cell("NÓ", c.node),
			cell("IP", c.ip), rcell("IDADE", c.age))
	case k8sKindDeploys:
		return joinNonEmpty(" ", cell("", c.dot), cell("NOME", c.name),
			rcell("READY", c.ready), rcell("IDADE", c.age))
	case k8sKindServices:
		return joinNonEmpty(" ", cell("", c.dot), cell("NOME", c.name), cell("TIPO", c.status),
			cell("CLUSTER-IP", c.ip), rcell("IDADE", c.age))
	default:
		return head.Render("ARQUIVO")
	}
}

func (a *App) renderK8sRow(i, width int, cursor, focus bool) string {
	sel := cursor && focus
	c := a.k8sColumns(width)

	if a.k8sKind == k8sKindManifests {
		items := a.k8sFilteredManifests()
		if i < 0 || i >= len(items) {
			return ""
		}
		row := renderCells(sel, []dashCell{{text: items[i], width: c.name, style: StyleNormal}})
		return k8sRowPrefix(cursor, sel) + row
	}

	items := a.k8sFilteredResources()
	if i < 0 || i >= len(items) {
		return ""
	}
	r := items[i]
	glyph, dotStyle := k8sStatusDot(r.Status, a.animFrame)

	var cells []dashCell
	switch a.k8sKind {
	case k8sKindPods:
		cells = []dashCell{
			{text: glyph, width: c.dot, style: dotStyle},
			{text: r.Name, width: c.name, style: StyleNormal.Bold(true)},
			{text: r.Status, width: c.status, style: dotStyle},
			{text: r.Ready, width: c.ready, style: k8sReadyStyle(r.Ready), right: true},
			{text: r.Restarts, width: c.restarts, style: k8sRestartStyle(r.Restarts), right: true},
			{text: firstNonEmpty(r.Node, emDash), width: c.node, style: StyleMuted},
			{text: firstNonEmpty(r.IP, emDash), width: c.ip, style: StyleMuted},
			{text: r.Age, width: c.age, style: StyleMuted, right: true},
		}
	case k8sKindDeploys:
		cells = []dashCell{
			{text: glyph, width: c.dot, style: dotStyle},
			{text: r.Name, width: c.name, style: StyleNormal.Bold(true)},
			{text: r.Ready, width: c.ready, style: k8sReadyStyle(r.Ready), right: true},
			{text: r.Age, width: c.age, style: StyleMuted, right: true},
		}
	case k8sKindServices:
		cells = []dashCell{
			{text: glyph, width: c.dot, style: dotStyle},
			{text: r.Name, width: c.name, style: StyleNormal.Bold(true)},
			{text: r.Status, width: c.status, style: StyleMuted},
			{text: firstNonEmpty(r.IP, emDash), width: c.ip, style: StyleMuted},
			{text: r.Age, width: c.age, style: StyleMuted, right: true},
		}
	}
	return k8sRowPrefix(cursor, sel) + renderCells(sel, cells)
}

func k8sRowPrefix(cursor, sel bool) string {
	if sel {
		return StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ")
	}
	if cursor {
		return StyleKey.Render("▌") + " "
	}
	return "  "
}

// k8sStatusDot: CrashLoopBackOff e ImagePullBackOff são o motivo de abrir a
// tela — não podem depender de caber numa coluna truncada.
func k8sStatusDot(status string, frame int) (string, lipgloss.Style) {
	switch status {
	case "Running", "Succeeded", "Completed", "Active", "Bound":
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating":
		return pulseGlyph(pulseWarn, frame), StyleWarning
	case "":
		return pulseGlyph(pulseIdle, frame), StyleMuted
	default:
		return pulseGlyph(pulseBad, frame), StyleUnhealthy
	}
}

// k8sReadyStyle lê "1/1" vs "0/1": um pod pronto pela metade não está no ar.
func k8sReadyStyle(ready string) lipgloss.Style {
	parts := strings.SplitN(strings.TrimSpace(ready), "/", 2)
	if len(parts) != 2 {
		return StyleMuted
	}
	have, _ := strconv.Atoi(parts[0])
	want, _ := strconv.Atoi(parts[1])
	switch {
	case want == 0:
		return StyleMuted
	case have == 0:
		return StyleUnhealthy
	case have < want:
		return StyleWarning
	default:
		return StyleHealthy
	}
}

// k8sRestartStyle: contagem de restarts é o sintoma mais barato de instabilidade.
func k8sRestartStyle(restarts string) lipgloss.Style {
	n, err := strconv.Atoi(strings.Fields(strings.TrimSpace(restarts))[0])
	if err != nil {
		return StyleMuted
	}
	switch {
	case n >= 5:
		return StyleUnhealthy
	case n > 0:
		return StyleWarning
	default:
		return StyleMuted
	}
}

func (a *App) k8sTableTitle(n int) string {
	switch a.k8sKind {
	case k8sKindPods:
		return panelTitle("PODS", fmt.Sprint(n))
	case k8sKindDeploys:
		return panelTitle("DEPLOYMENTS", fmt.Sprint(n))
	case k8sKindServices:
		return panelTitle("SERVICES", fmt.Sprint(n))
	default:
		return panelTitle("MANIFESTS", fmt.Sprint(n))
	}
}

func k8sStatusLabel(status string, frame int) string {
	switch status {
	case "Running":
		return pulseGlyph(pulseOK, frame) + " Running"
	case "Pending":
		return "● Pending"
	case "Succeeded", "Completed":
		return "○ Completed"
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff":
		return "● " + status
	default:
		if status == "" {
			return "—"
		}
		return "● " + status
	}
}

func (a *App) renderK8sLogsPane(width, height int) string {
	focus := a.k8sFocus == k8sFocusLogs && !a.k8sEditing
	body := a.k8sLogs
	if strings.TrimSpace(body) == "" {
		body = "l  carrega logs do pod\n(selecione um pod)"
	}
	raw := strings.Split(body, "\n")
	a.k8sLogsScroll = clampScroll(a.k8sLogsScroll, height-2, len(raw))
	start := a.k8sLogsScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		lines = append(lines, a.k8sColorLogLine(truncate(sanitizeTerminalLine(line), width-2), focus))
	}
	title := "POD LOGS"
	if focus {
		title = "> POD LOGS"
	}
	return panelBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) k8sColorLogLine(line string, focus bool) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "[error]"):
		return StyleUnhealthy.Render(line)
	case strings.Contains(lower, "warn") || strings.Contains(lower, "[warn]"):
		return StyleWarning.Render(line)
	case focus:
		return StyleNormal.Render(line)
	default:
		return StyleMuted.Render(line)
	}
}

func (a *App) renderK8sYAMLPane(width, height int) string {
	focus := a.k8sFocus == k8sFocusYAML && !a.k8sEditing
	body := a.k8sYAML
	if strings.TrimSpace(body) == "" {
		body = "y  carrega yaml\ne  edita recurso"
	}
	raw := strings.Split(body, "\n")
	a.k8sYAMLScroll = clampScroll(a.k8sYAMLScroll, height-2, len(raw))
	start := a.k8sYAMLScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		style := StyleMuted
		if focus {
			style = StyleNormal
		}
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	title := "YAML"
	if focus {
		title = "> YAML"
	}
	return panelBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderK8sDetailPane(width, height int) string {
	focus := a.k8sFocus == k8sFocusDetail && !a.k8sEditing
	body := a.k8sDetail
	if strings.TrimSpace(body) == "" {
		body = "selecione um recurso\nenter describe\nl logs  y yaml  e edit"
	}
	// Prefer a compact summary when we have a selected pod.
	if r, ok := a.k8sSelectedResource(); ok && a.k8sKind != k8sKindManifests {
		summary := a.k8sResourceSummary(r)
		if strings.TrimSpace(a.k8sEvents) != "" {
			summary += "\n\nEvents\n" + a.k8sEvents
		} else if body != "" && !strings.HasPrefix(body, "selecione") {
			summary += "\n\n" + body
		}
		body = summary
	}
	raw := strings.Split(body, "\n")
	a.k8sDetailScroll = clampScroll(a.k8sDetailScroll, height-2, len(raw))
	start := a.k8sDetailScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		style := StyleMuted
		if focus {
			style = StyleNormal
		}
		if strings.HasPrefix(strings.TrimSpace(line), "Events") || strings.HasPrefix(line, "Pod ") || strings.HasPrefix(line, "Deploy") || strings.HasPrefix(line, "Service") {
			style = StyleSection
		}
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	title := "DETAILS"
	if focus {
		title = "> DETAILS"
	}
	return panelBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) k8sResourceSummary(r collectors.K8sResource) string {
	var b strings.Builder
	b.WriteString(r.Kind + "  " + r.Name + "\n")
	b.WriteString("Namespace  " + a.k8sNamespace + "\n")
	if r.Node != "" {
		b.WriteString("Node       " + r.Node + "\n")
	}
	if r.IP != "" {
		b.WriteString("IP         " + r.IP + "\n")
	}
	if r.Status != "" {
		b.WriteString("Status     " + r.Status + "\n")
	}
	if r.Ready != "" {
		b.WriteString("Ready      " + r.Ready + "\n")
	}
	if r.Restarts != "" {
		b.WriteString("Restarts   " + r.Restarts + "\n")
	}
	if r.Age != "" {
		b.WriteString("Age        " + r.Age + "\n")
	}
	return b.String()
}

func (a *App) renderK8sEventsView(width, height int) string {
	body := a.k8sEvents
	if strings.TrimSpace(body) == "" {
		body = "r  carrega events do namespace\n(nenhum event ainda)"
	}
	raw := strings.Split(body, "\n")
	a.k8sDetailScroll = clampScroll(a.k8sDetailScroll, height-2, len(raw))
	start := a.k8sDetailScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		lines = append(lines, StyleNormal.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	return panelBox(panelTitle("EVENTS", a.k8sNamespace), fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderK8sRelation(width int) string {
	chain := "Ingress → Service → Deployment → ReplicaSet → Pods → Containers"
	switch a.k8sKind {
	case k8sKindServices:
		chain = "Service → Endpoints → Pods"
	case k8sKindDeploys:
		chain = "Deployment → ReplicaSet → Pods → Containers"
	case k8sKindManifests:
		chain = "Manifest → kubectl apply → Cluster"
	}
	line := StyleMuted.Render("RELATION  ") + StyleNormal.Render(truncate(chain, maxInt(20, width-12)))
	return line
}

func (a *App) renderK8sEditor(width, height int) string {
	content := a.k8sYAML
	if a.k8sEditing {
		content = renderApiCursor(a.k8sYAML, a.k8sEditorCursor)
	}
	raw := strings.Split(content, "\n")
	cursorLine := 0
	for i, r := range []rune(a.k8sYAML) {
		if i >= a.k8sEditorCursor {
			break
		}
		if r == '\n' {
			cursorLine++
		}
	}
	scroll := ensureVisible(cursorLine, a.k8sYAMLScroll, height-2, len(raw))
	a.k8sYAMLScroll = scroll
	start := scroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		lines = append(lines, StyleSelected.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	return panelBox("[yaml edit]", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) handleK8sKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.k8sConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			return a, a.k8sDoDelete()
		case "n", "N", "esc":
			a.k8sConfirmDelete = false
			a.k8sStatus = "delete cancelado"
			return a, nil
		}
		return a, nil
	}
	if a.k8sFilterOn {
		return a.updateK8sFilter(msg, p)
	}
	if a.k8sEditing {
		return a.updateK8sEdit(msg, p)
	}

	switch msg.String() {
	case "esc":
		if a.k8sFocus != k8sFocusTable {
			a.k8sFocus = k8sFocusTable
			a.k8sPane = k8sPaneList
			return a, nil
		}
		return a, a.leaveK8sTab()
	case "tab":
		a.k8sFocus = (a.k8sFocus + 1) % 4
		return a, nil
	case "shift+tab":
		a.k8sFocus = (a.k8sFocus + 3) % 4
		return a, nil
	case "0":
		a.k8sSubTab = k8sTabOverview
	case "1":
		return a, a.k8sSetSubTab(k8sTabWorkloads, p)
	case "2":
		return a, a.k8sSetSubTab(k8sTabNetworking, p)
	case "3":
		return a, a.k8sSetSubTab(k8sTabConfig, p)
	case "4":
		return a, a.k8sSetSubTab(k8sTabEvents, p)
	case "[":
		return a, a.k8sShiftKind(-1)
	case "]":
		return a, a.k8sShiftKind(1)
	case "n":
		return a, a.k8sCycleNamespace(1)
	case "N", "p", "P":
		return a, a.k8sCycleNamespace(-1)
	case "b", "/":
		a.k8sFilterOn = true
		return a, nil
	case "r", "ctrl+r":
		return a, tea.Batch(a.refreshK8s(p), a.loadK8sMeta(), a.k8sLoadEvents())
	case "up", "k":
		return a, a.k8sMove(-1)
	case "down", "j":
		return a, a.k8sMove(1)
	case "pgup":
		a.k8sScrollPane(-10)
	case "pgdown":
		a.k8sScrollPane(10)
	case "enter":
		return a, a.k8sShowDetail()
	case "y":
		return a, a.k8sLoadYAML()
	case "a":
		if strings.TrimSpace(a.k8sYAML) != "" && a.k8sPane == k8sPaneEditor {
			return a, a.k8sApplyEditedYAML()
		}
		return a, a.k8sApplyCurrent(p)
	case "c":
		return a, a.k8sBeginCreate()
	case "e":
		return a, a.k8sBeginEdit()
	case "d":
		if a.k8sKind == k8sKindManifests {
			a.k8sErr = "use delete no cluster (pods/deploy/svc)"
			return a, nil
		}
		if r, ok := a.k8sSelectedResource(); ok {
			a.k8sConfirmDelete = true
			a.k8sStatus = "modal delete · " + r.Name
		}
	case "l":
		return a, a.k8sShowLogs()
	case "+":
		return a, a.k8sScale(1)
	case "-":
		return a, a.k8sScale(-1)
	case "left", "h":
		return a, a.k8sShiftKind(-1)
	case "right":
		return a, a.k8sShiftKind(1)
	}
	return a, nil
}

func (a *App) updateK8sFilter(msg tea.KeyMsg, _ *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.k8sFilterOn = false
		a.k8sFilter = ""
		a.k8sCursor = 0
		return a, nil
	case "enter":
		a.k8sFilterOn = false
		a.k8sCursor = 0
		return a, a.k8sInspectSelected()
	case "backspace":
		if a.k8sFilter != "" {
			r := []rune(a.k8sFilter)
			a.k8sFilter = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			a.k8sFilter += string(msg.Runes)
		} else if s := msg.String(); len(s) == 1 {
			a.k8sFilter += s
		}
	}
	return a, nil
}

func (a *App) k8sSetSubTab(tab k8sSubTab, p *core.Project) tea.Cmd {
	a.k8sSubTab = tab
	a.k8sCursor = 0
	a.k8sScroll = 0
	switch tab {
	case k8sTabNetworking:
		a.k8sKind = k8sKindServices
		return a.refreshK8s(p)
	case k8sTabConfig:
		a.k8sKind = k8sKindManifests
		return a.refreshK8s(p)
	case k8sTabEvents:
		return a.k8sLoadEvents()
	case k8sTabWorkloads:
		if a.k8sKind != k8sKindPods && a.k8sKind != k8sKindDeploys {
			a.k8sKind = k8sKindPods
		}
		return a.refreshK8s(p)
	default:
		if a.k8sKind == k8sKindManifests {
			a.k8sKind = k8sKindPods
			return a.refreshK8s(p)
		}
	}
	return nil
}

func (a *App) syncK8sSubTabFromKind() {
	switch a.k8sKind {
	case k8sKindServices:
		a.k8sSubTab = k8sTabNetworking
	case k8sKindManifests:
		a.k8sSubTab = k8sTabConfig
	default:
		if a.k8sSubTab == k8sTabNetworking || a.k8sSubTab == k8sTabConfig || a.k8sSubTab == k8sTabEvents {
			a.k8sSubTab = k8sTabWorkloads
		}
	}
}

// k8sShiftKind percorre os tipos em ciclo — [ e ] na régua, ← → na tabela.
func (a *App) k8sShiftKind(delta int) tea.Cmd {
	a.k8sKind = k8sKind((int(a.k8sKind) + delta + 4) % 4)
	a.k8sCursor = 0
	a.k8sScroll = 0
	a.syncK8sSubTabFromKind()
	return a.refreshK8s(a.currentProject())
}

func (a *App) k8sMove(delta int) tea.Cmd {
	switch a.k8sFocus {
	case k8sFocusLogs:
		a.k8sLogsScroll += delta
		if a.k8sLogsScroll < 0 {
			a.k8sLogsScroll = 0
		}
	case k8sFocusYAML:
		a.k8sYAMLScroll += delta
		if a.k8sYAMLScroll < 0 {
			a.k8sYAMLScroll = 0
		}
	case k8sFocusDetail:
		a.k8sDetailScroll += delta
		if a.k8sDetailScroll < 0 {
			a.k8sDetailScroll = 0
		}
	default:
		n := a.k8sListLen()
		a.k8sCursor += delta
		if a.k8sCursor < 0 {
			a.k8sCursor = 0
		}
		if a.k8sCursor > n-1 {
			a.k8sCursor = maxInt(0, n-1)
		}
		return a.k8sInspectSelected()
	}
	return nil
}

func (a *App) k8sScrollPane(delta int) {
	switch a.k8sFocus {
	case k8sFocusLogs:
		a.k8sLogsScroll += delta
		if a.k8sLogsScroll < 0 {
			a.k8sLogsScroll = 0
		}
	case k8sFocusYAML:
		a.k8sYAMLScroll += delta
		if a.k8sYAMLScroll < 0 {
			a.k8sYAMLScroll = 0
		}
	default:
		a.k8sDetailScroll += delta
		if a.k8sDetailScroll < 0 {
			a.k8sDetailScroll = 0
		}
	}
}

func (a *App) k8sCycleNamespace(delta int) tea.Cmd {
	a.k8sLoading = true
	cur := a.k8sNamespace
	return func() tea.Msg {
		nsList, err := collectors.K8sNamespaces()
		if err != nil || len(nsList) == 0 {
			return k8sNsMsg{err: "não foi possível listar namespaces"}
		}
		idx := 0
		for i, ns := range nsList {
			if ns == cur {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(nsList)) % len(nsList)
		return k8sNsMsg{ns: nsList[idx]}
	}
}

func (a *App) k8sInspectSelected() tea.Cmd {
	if a.k8sKind == k8sKindManifests {
		path, ok := a.k8sSelectedManifest()
		if !ok {
			return nil
		}
		a.k8sInspectName = path
		name := path
		return func() tea.Msg {
			b, err := os.ReadFile(path)
			if err != nil {
				return k8sInspectMsg{name: name, err: err.Error()}
			}
			return k8sInspectMsg{name: name, yaml: string(b), detail: filepath.Base(path)}
		}
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	kind := strings.ToLower(r.Kind)
	ns := a.k8sNamespace
	name := r.Name
	a.k8sInspectName = name
	wantLogs := a.k8sKind == k8sKindPods
	return func() tea.Msg {
		detail, _ := collectors.K8sDescribe(kind, name, ns)
		yaml, _ := collectors.K8sGetYAML(kind, name, ns)
		logs := ""
		if wantLogs {
			logs, _ = collectors.K8sPodLogs(name, ns, 80)
		}
		events, _ := collectors.K8sListEvents(ns, 8)
		return k8sInspectMsg{name: name, detail: detail, logs: logs, yaml: yaml, events: events}
	}
}

func (a *App) k8sLoadEvents() tea.Cmd {
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sListEvents(ns, 40)
		if err != nil {
			return k8sInspectMsg{events: out, err: err.Error()}
		}
		return k8sInspectMsg{events: out}
	}
}

func (a *App) k8sLoadYAML() tea.Cmd {
	a.k8sFocus = k8sFocusYAML
	if a.k8sKind == k8sKindManifests {
		path, ok := a.k8sSelectedManifest()
		if !ok {
			return nil
		}
		a.k8sLoading = true
		return func() tea.Msg {
			b, err := os.ReadFile(path)
			if err != nil {
				return k8sInspectMsg{err: err.Error()}
			}
			return k8sInspectMsg{yaml: string(b)}
		}
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	kind := strings.ToLower(r.Kind)
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sGetYAML(kind, r.Name, ns)
		if err != nil {
			return k8sInspectMsg{yaml: out, err: err.Error()}
		}
		return k8sInspectMsg{yaml: out}
	}
}

func (a *App) k8sShowDetail() tea.Cmd {
	a.k8sFocus = k8sFocusDetail
	a.k8sPane = k8sPaneDetail
	if a.k8sKind == k8sKindManifests {
		path, ok := a.k8sSelectedManifest()
		if !ok {
			return nil
		}
		a.k8sLoading = true
		return func() tea.Msg {
			b, err := os.ReadFile(path)
			if err != nil {
				return k8sDetailMsg{err: err.Error()}
			}
			return k8sDetailMsg{body: string(b)}
		}
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	kind := strings.ToLower(r.Kind)
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sDescribe(kind, r.Name, ns)
		if err != nil {
			return k8sDetailMsg{body: out, err: err.Error()}
		}
		return k8sDetailMsg{body: out}
	}
}

func (a *App) k8sShowLogs() tea.Cmd {
	a.k8sFocus = k8sFocusLogs
	if a.k8sKind != k8sKindPods {
		a.k8sErr = "logs só para pods"
		return nil
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sPodLogs(r.Name, ns, 120)
		if err != nil {
			return k8sInspectMsg{logs: out, err: err.Error()}
		}
		return k8sInspectMsg{logs: out}
	}
}

func (a *App) k8sApplyCurrent(p *core.Project) tea.Cmd {
	if a.k8sKind != k8sKindManifests {
		a.k8sErr = "] até Config e a para apply"
		return nil
	}
	path, ok := a.k8sSelectedManifest()
	if !ok {
		return nil
	}
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sApplyFile(path)
		if err != nil {
			return k8sActionMsg{out: out, err: err.Error()}
		}
		return k8sActionMsg{out: out}
	}
}

func (a *App) k8sBeginCreate() tea.Cmd {
	name := "app"
	if p := a.currentProject(); p != nil && p.Name != "" {
		name = sanitizeK8sName(p.Name)
	}
	a.k8sYAML = collectors.K8sDeploymentTemplate(name, a.k8sNamespace, "nginx:alpine") +
		"---\n" +
		collectors.K8sServiceTemplate(name, a.k8sNamespace, 80)
	a.k8sEditorCursor = 0
	a.k8sEditing = true
	a.k8sPane = k8sPaneEditor
	a.k8sFocus = k8sFocusYAML
	a.k8sYAMLScroll = 0
	a.k8sErr = ""
	a.k8sStatus = "criar — edite o YAML · ctrl+s (ou a) aplica"
	return nil
}

func (a *App) k8sBeginEdit() tea.Cmd {
	if a.k8sKind == k8sKindManifests {
		path, ok := a.k8sSelectedManifest()
		if !ok {
			return nil
		}
		a.k8sLoading = true
		return func() tea.Msg {
			b, err := os.ReadFile(path)
			if err != nil {
				return k8sEditReadyMsg{err: err.Error()}
			}
			return k8sEditReadyMsg{yaml: string(b), status: "editando " + filepath.Base(path)}
		}
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	kind := strings.ToLower(r.Kind)
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sGetYAML(kind, r.Name, ns)
		if err != nil {
			return k8sEditReadyMsg{err: err.Error()}
		}
		return k8sEditReadyMsg{yaml: out, status: "editando " + r.Name}
	}
}

func (a *App) k8sDoDelete() tea.Cmd {
	r, ok := a.k8sSelectedResource()
	if !ok {
		a.k8sConfirmDelete = false
		return nil
	}
	kind := strings.ToLower(r.Kind)
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		err := collectors.K8sDelete(kind, r.Name, ns)
		if err != nil {
			return k8sActionMsg{err: err.Error()}
		}
		return k8sActionMsg{out: "deleted " + r.Name}
	}
}

func (a *App) k8sScale(delta int) tea.Cmd {
	if a.k8sKind != k8sKindDeploys {
		a.k8sErr = "+/- só em deploy"
		return nil
	}
	r, ok := a.k8sSelectedResource()
	if !ok {
		return nil
	}
	replicas := 1
	if parts := strings.Split(r.Ready, "/"); len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &replicas)
	} else if parts := strings.Split(r.Status, "/"); len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &replicas)
	}
	replicas += delta
	if replicas < 0 {
		replicas = 0
	}
	ns := a.k8sNamespace
	a.k8sLoading = true
	return func() tea.Msg {
		out, err := collectors.K8sScale(r.Name, ns, replicas)
		if err != nil {
			return k8sActionMsg{out: out, err: err.Error()}
		}
		return k8sActionMsg{out: out}
	}
}

func isK8sApplyKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+s", "ctrl+enter", "alt+enter", "ctrl+j":
		return true
	}
	return msg.Type == tea.KeyEnter && msg.Alt
}

func (a *App) k8sApplyEditedYAML() tea.Cmd {
	yaml := a.k8sYAML
	a.k8sEditing = false
	a.k8sPane = k8sPaneDetail
	a.k8sFocus = k8sFocusYAML
	a.k8sLoading = true
	a.k8sErr = ""
	a.k8sStatus = "aplicando…"
	return func() tea.Msg {
		out, err := collectors.K8sApplyYAML(yaml)
		if err != nil {
			return k8sActionMsg{out: out, err: err.Error()}
		}
		return k8sActionMsg{out: out}
	}
}

func (a *App) updateK8sEdit(msg tea.KeyMsg, _ *core.Project) (tea.Model, tea.Cmd) {
	if isK8sApplyKey(msg) {
		return a, a.k8sApplyEditedYAML()
	}

	runes := []rune(a.k8sYAML)
	cursor := a.k8sEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	switch msg.String() {
	case "esc":
		a.k8sEditing = false
		a.k8sPane = k8sPaneEditor
		a.k8sFocus = k8sFocusYAML
		a.k8sStatus = "edição pausada — a aplica · e volta a editar · esc sai"
		return a, nil
	case "enter":
		runes = append(runes[:cursor], append([]rune{'\n'}, runes[cursor:]...)...)
		cursor++
	case "left":
		if cursor > 0 {
			cursor--
		}
	case "right":
		if cursor < len(runes) {
			cursor++
		}
	case "up":
		cursor = apiMoveLine(runes, cursor, -1)
	case "down":
		cursor = apiMoveLine(runes, cursor, 1)
	case "home":
		cursor = apiLineStart(runes, cursor)
	case "end":
		cursor = apiLineEnd(runes, cursor)
	case "backspace":
		if cursor > 0 {
			runes = append(runes[:cursor-1], runes[cursor:]...)
			cursor--
		}
	case "delete":
		if cursor < len(runes) {
			runes = append(runes[:cursor], runes[cursor+1:]...)
		}
	case "tab":
		runes = append(runes[:cursor], append([]rune("  "), runes[cursor:]...)...)
		cursor += 2
	default:
		var inserted []rune
		if len(msg.Runes) > 0 {
			inserted = msg.Runes
		} else if s := msg.String(); len(s) == 1 {
			inserted = []rune(s)
		}
		if len(inserted) > 0 {
			runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
			cursor += len(inserted)
		}
	}
	a.k8sYAML = string(runes)
	a.k8sEditorCursor = cursor
	return a, nil
}

func sanitizeK8sName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
