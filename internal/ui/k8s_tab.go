package ui

import (
	"fmt"
	"os"
	"path/filepath"
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
	k8sFocusExplorer k8sFocus = iota
	k8sFocusTable
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
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabKubernetes)).Bold(true)
	available := collectors.K8sAvailable()
	ctx := collectors.K8sCurrentContext()
	manifests := collectors.DiscoverProjectManifests(p.Path)

	lines := []string{
		accent.Render("⎈  Kubernetes"),
		StyleMuted.Render("explorer · workloads · logs · yaml · events"),
		"",
		StyleSection.Render("ABRIR"),
		StyleNormal.Render("  pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para entrar"),
		StyleMuted.Render("  esc no cliente volta para esta aba"),
		"",
		StyleSection.Render("ATALHOS"),
		StyleMuted.Render("  b filter  n/p namespace  l logs  y yaml  e edit"),
		StyleMuted.Render("  c create  ctrl+s apply  d delete  r refresh"),
		"",
		StyleSection.Render("CLUSTER"),
	}
	if !available {
		lines = append(lines,
			StyleUnhealthy.Render("  kubectl não encontrado no PATH"),
			StyleMuted.Render("  instale kubectl e configure o kubeconfig"),
		)
	} else {
		if ctx == "" {
			ctx = "(sem context)"
		}
		lines = append(lines, StyleNormal.Render("  context  ")+StyleWarning.Render(ctx))
	}
	lines = append(lines, "", StyleSection.Render("MANIFESTS DO PROJETO"))
	if len(manifests) == 0 {
		lines = append(lines, StyleMuted.Render("  nenhum yaml em k8s/ kubernetes/ deploy/"))
	} else {
		n := minInt(6, len(manifests))
		for i := 0; i < n; i++ {
			rel := manifests[i]
			if p != nil {
				if r, err := filepath.Rel(p.Path, manifests[i]); err == nil {
					rel = r
				}
			}
			lines = append(lines, StyleMuted.Render("  · "+truncate(rel, 42)))
		}
		if len(manifests) > n {
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  … +%d", len(manifests)-n)))
		}
	}
	return StylePanel.Render(strings.Join(lines, "\n"))
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
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
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
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, rel, a.renderStatusBar(hints))
}

func (a *App) k8sHints() string {
	if a.k8sConfirmDelete {
		return "confirmar delete?  y sim  esc/n cancelar"
	}
	if a.k8sEditing {
		return "editando YAML  enter=nova linha  ctrl+s=aplicar  esc=pausar"
	}
	if a.k8sFilterOn {
		return "filter  enter aplicar  esc limpar  ·  " + a.k8sFilter + "█"
	}
	base := "? help  n/p ns  b filter  tab painel  enter detail  l logs  y yaml  e edit  c create  d delete  r refresh  esc"
	if a.k8sLoading {
		base = "carregando…  " + base
	}
	if a.k8sStatus != "" {
		return truncate(a.k8sStatus, 40) + "  ·  " + base
	}
	return base
}

func (a *App) renderK8sHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabKubernetes)).Bold(true)
	ctx := a.k8sContext
	if ctx == "" {
		ctx = "?"
	}
	left := accent.Render("devscope") + StyleMuted.Render(" › kubernetes") +
		StyleMuted.Render("  Context: ") + StyleWarning.Render(truncate(ctx, 24)) +
		StyleMuted.Render("  Namespace: ") + StyleNormal.Render(a.k8sNamespace)

	ver := a.k8sVersion
	if ver == "" {
		ver = "—"
	}
	right := StyleMuted.Render(ver) +
		StyleMuted.Render(fmt.Sprintf("  nodes:%d", a.k8sNodeCount))
	if a.k8sErr != "" {
		right += "  " + StyleUnhealthy.Render(truncate(a.k8sErr, 28))
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderK8sSubTabs(width int) string {
	names := []string{"Overview", "Workloads", "Networking", "Config", "Events"}
	var parts []string
	for i, n := range names {
		label := " " + n + " "
		if k8sSubTab(i) == a.k8sSubTab {
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

func (a *App) renderK8sOverview(width, height int) string {
	leftW := maxInt(22, width*20/100)
	if leftW > 32 {
		leftW = 32
	}
	rightW := maxInt(24, width*26/100)
	if rightW > 40 {
		rightW = 40
	}
	centerW := maxInt(30, width-leftW-rightW-2)

	bottomH := maxInt(6, height*32/100)
	tableH := maxInt(6, height-bottomH)
	logsW := centerW / 2
	yamlW := centerW - logsW

	left := a.renderK8sExplorer(leftW, height)
	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderK8sTable(centerW, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderK8sLogsPane(logsW, bottomH),
			a.renderK8sYAMLPane(yamlW, bottomH),
		),
	)
	right := a.renderK8sDetailPane(rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (a *App) renderK8sExplorer(width, height int) string {
	statsH := maxInt(7, height*28/100)
	treeH := maxInt(8, height-statsH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderK8sTree(width, treeH),
		a.renderK8sQuickStats(width, statsH),
	)
}

func (a *App) renderK8sTree(width, height int) string {
	focus := a.k8sFocus == k8sFocusExplorer && !a.k8sEditing
	kinds := []struct {
		kind  k8sKind
		label string
		group string
	}{
		{k8sKindPods, "Pods", "Workloads"},
		{k8sKindDeploys, "Deployments", "Workloads"},
		{k8sKindServices, "Services", "Networking"},
		{k8sKindManifests, "Manifests", "Config"},
	}
	lines := make([]string, 0, height-2)
	lastGroup := ""
	for _, item := range kinds {
		if item.group != lastGroup {
			lines = append(lines, StyleSection.Render("  "+item.group))
			lastGroup = item.group
		}
		count := "—"
		switch item.kind {
		case k8sKindPods:
			count = fmt.Sprintf("%d", len(a.k8sResources))
			if a.k8sKind != k8sKindPods {
				count = "·"
			}
		case k8sKindDeploys:
			if a.k8sKind == k8sKindDeploys {
				count = fmt.Sprintf("%d", len(a.k8sResources))
			} else {
				count = "·"
			}
		case k8sKindServices:
			if a.k8sKind == k8sKindServices {
				count = fmt.Sprintf("%d", len(a.k8sResources))
			} else {
				count = "·"
			}
		case k8sKindManifests:
			count = fmt.Sprintf("%d", len(a.k8sManifests))
		}
		mark := "  "
		style := StyleMuted
		if item.kind == a.k8sKind {
			mark = "▸ "
			if focus {
				style = StyleSelected
			} else {
				style = StyleNormal
			}
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%-14s %s", mark, item.label, count)))
	}
	title := "CLUSTER EXPLORER"
	if focus {
		title = "> CLUSTER EXPLORER"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderK8sQuickStats(width, height int) string {
	running, pending, failed, total := 0, 0, 0, 0
	if a.k8sKind == k8sKindPods {
		total = len(a.k8sResources)
		for _, r := range a.k8sResources {
			switch r.Status {
			case "Running":
				running++
			case "Pending":
				pending++
			case "Failed", "CrashLoopBackOff", "Error":
				failed++
			}
		}
	} else {
		total = a.k8sListLen()
	}
	lines := []string{
		StyleNormal.Render(fmt.Sprintf("  total     %d", total)),
		StyleHealthy.Render(fmt.Sprintf("  running   %d", running)),
		StyleWarning.Render(fmt.Sprintf("  pending   %d", pending)),
		StyleUnhealthy.Render(fmt.Sprintf("  failed    %d", failed)),
		StyleMuted.Render(fmt.Sprintf("  manifests %d", len(a.k8sManifests))),
		StyleMuted.Render(fmt.Sprintf("  nodes     %d", a.k8sNodeCount)),
	}
	if a.k8sFilter != "" {
		lines = append(lines, StyleWarning.Render("  filter  "+truncate(a.k8sFilter, width-12)))
	}
	return renderApiTitledBox("QUICK STATS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderK8sTable(width, height int) string {
	focus := a.k8sFocus == k8sFocusTable && !a.k8sEditing
	n := a.k8sListLen()
	title := a.k8sTableTitle(n)
	a.k8sScroll = ensureVisible(a.k8sCursor, a.k8sScroll, height-3, n)

	header := a.k8sTableHeader()
	lines := make([]string, 0, height-2)
	lines = append(lines, StyleMuted.Render(truncate(header, width-2)))

	if n == 0 {
		if a.k8sLoading {
			lines = append(lines, StyleMuted.Render("  carregando..."))
		} else {
			lines = append(lines, StyleMuted.Render("  (vazio)"))
		}
	} else {
		start := a.k8sScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			label := a.k8sRowLabel(i)
			prefix := "  "
			style := StyleMuted
			if i == a.k8sCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			lines = append(lines, style.Render(truncate(prefix+label, width-2)))
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) k8sTableTitle(n int) string {
	switch a.k8sKind {
	case k8sKindPods:
		return fmt.Sprintf("PODS (%d)", n)
	case k8sKindDeploys:
		return fmt.Sprintf("DEPLOYMENTS (%d)", n)
	case k8sKindServices:
		return fmt.Sprintf("SERVICES (%d)", n)
	default:
		return fmt.Sprintf("MANIFESTS (%d)", n)
	}
}

func (a *App) k8sTableHeader() string {
	switch a.k8sKind {
	case k8sKindPods:
		return fmt.Sprintf("%-22s %-16s %-7s %-8s %-12s %-12s %s",
			"NAME", "STATUS", "READY", "RESTARTS", "NODE", "IP", "AGE")
	case k8sKindDeploys:
		return fmt.Sprintf("%-28s %-10s %s", "NAME", "READY", "AGE")
	case k8sKindServices:
		return fmt.Sprintf("%-24s %-12s %-16s %s", "NAME", "TYPE", "CLUSTER-IP", "AGE")
	default:
		return "FILE"
	}
}

func (a *App) k8sRowLabel(i int) string {
	if a.k8sKind == k8sKindManifests {
		items := a.k8sFilteredManifests()
		if i < 0 || i >= len(items) {
			return ""
		}
		return filepath.Base(items[i])
	}
	items := a.k8sFilteredResources()
	if i < 0 || i >= len(items) {
		return ""
	}
	r := items[i]
	switch a.k8sKind {
	case k8sKindPods:
		return fmt.Sprintf("%-22s %-16s %-7s %-8s %-12s %-12s %s",
			truncate(r.Name, 22),
			truncate(k8sStatusLabel(r.Status), 16),
			truncate(r.Ready, 7),
			truncate(r.Restarts, 8),
			truncate(r.Node, 12),
			truncate(r.IP, 12),
			r.Age,
		)
	case k8sKindDeploys:
		return fmt.Sprintf("%-28s %-10s %s", truncate(r.Name, 28), r.Ready, r.Age)
	case k8sKindServices:
		return fmt.Sprintf("%-24s %-12s %-16s %s", truncate(r.Name, 24), r.Status, truncate(r.IP, 16), r.Age)
	default:
		return r.Name
	}
}

func k8sStatusLabel(status string) string {
	switch status {
	case "Running":
		return "● Running"
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
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
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
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
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
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
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
	return renderApiTitledBox(fmt.Sprintf("EVENTS · %s", a.k8sNamespace), fitExactLines(lines, height-2), width, height, true)
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
	return renderApiTitledBox("[yaml edit]", fitExactLines(lines, height-2), width, height, true)
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
		if a.k8sFocus != k8sFocusTable && a.k8sFocus != k8sFocusExplorer {
			a.k8sFocus = k8sFocusTable
			a.k8sPane = k8sPaneList
			return a, nil
		}
		return a, a.leaveK8sTab()
	case "tab":
		a.k8sFocus = (a.k8sFocus + 1) % 5
		return a, nil
	case "shift+tab":
		a.k8sFocus = (a.k8sFocus + 4) % 5
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
		a.k8sKind = k8sKind((int(a.k8sKind) + 3) % 4)
		a.k8sCursor = 0
		a.k8sScroll = 0
		a.syncK8sSubTabFromKind()
		return a, a.refreshK8s(p)
	case "]":
		a.k8sKind = k8sKind((int(a.k8sKind) + 1) % 4)
		a.k8sCursor = 0
		a.k8sScroll = 0
		a.syncK8sSubTabFromKind()
		return a, a.refreshK8s(p)
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
		if a.k8sFocus == k8sFocusExplorer {
			return a, a.refreshK8s(p)
		}
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
			a.k8sStatus = "delete " + r.Name + "?"
		}
	case "l":
		return a, a.k8sShowLogs()
	case "+":
		return a, a.k8sScale(1)
	case "-":
		return a, a.k8sScale(-1)
	case "left", "h":
		a.k8sFocus = k8sFocusExplorer
	case "right":
		a.k8sFocus = k8sFocusTable
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

func (a *App) k8sMove(delta int) tea.Cmd {
	switch a.k8sFocus {
	case k8sFocusExplorer:
		next := int(a.k8sKind) + delta
		if next < 0 {
			next = 0
		}
		if next > 3 {
			next = 3
		}
		if k8sKind(next) != a.k8sKind {
			a.k8sKind = k8sKind(next)
			a.k8sCursor = 0
			a.k8sScroll = 0
			a.syncK8sSubTabFromKind()
			return a.refreshK8s(a.currentProject())
		}
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
