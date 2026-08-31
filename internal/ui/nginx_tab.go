package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/nginxutil"
)

const (
	nginxWizName = iota
	nginxWizKind
	nginxWizServerName
	nginxWizTarget
	nginxWizRoot
	nginxWizPort
	nginxWizSSL
	nginxWizHubDirName
)

var nginxKinds = []string{"single", "hub"}

type nginxFocus int

const (
	nginxFocusTable nginxFocus = iota
	nginxFocusDetails
)

type nginxLoadedMsg struct {
	layout  nginxutil.Layout
	sites   []nginxutil.Site
	foreign int
	err     string
}

type nginxIncsLoadedMsg struct {
	incs []nginxutil.Site
	err  string
}

type nginxActionMsg struct {
	out string
	err string
}

func (a *App) enterNginxTab(_ *core.Project) {
	a.tab = TabNginx
	a.tabCursor = 0
	a.nginxOpen = false
}

func (a *App) openNginxClient(p *core.Project) tea.Cmd {
	a.nginxOpen = true
	a.nginxFocus = nginxFocusTable
	a.nginxCursor = 0
	a.nginxScroll = 0
	a.nginxDetailsScroll = 0
	a.nginxErr = ""
	a.nginxStatus = ""
	a.nginxWizard = false
	a.nginxConfirmDelete = false
	a.nginxHub = nil
	a.nginxIncs = nil
	a.nginxTopCursor, a.nginxTopScroll = 0, 0
	if a.nginxNewPortStr == "" {
		a.nginxNewPortStr = "80"
	}
	return a.refreshNginx(p)
}

func (a *App) leaveNginxTab() tea.Cmd {
	a.nginxOpen = false
	a.nginxWizard = false
	a.nginxConfirmDelete = false
	a.nginxHub = nil
	a.nginxIncs = nil
	a.tab = TabNginx
	a.tabCursor = 0
	return nil
}

func (a *App) refreshNginx(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	a.nginxLoading = true
	path := p.Path
	showAll := a.nginxShowAll
	others := make([]core.Project, 0, len(a.snapshot.Projects))
	for _, op := range a.snapshot.Projects {
		if op.Path != path {
			others = append(others, op)
		}
	}
	return func() tea.Msg {
		layout, sites, err := nginxutil.Discover(path)
		if err != nil {
			return nginxLoadedMsg{err: err.Error()}
		}
		var foreign []nginxutil.Site
		for _, op := range others {
			_, fsites, ferr := nginxutil.Discover(op.Path)
			if ferr != nil {
				continue
			}
			for _, s := range fsites {
				s.Project = op.Name
				foreign = append(foreign, s)
			}
		}
		all := sites
		if showAll {
			all = append(append([]nginxutil.Site(nil), sites...), foreign...)
		}
		return nginxLoadedMsg{layout: layout, sites: all, foreign: len(foreign)}
	}
}

// refreshNginxIncs recarrega as .inc do hub atualmente aberto (a.nginxHub).
func (a *App) refreshNginxIncs() tea.Cmd {
	if a.nginxHub == nil {
		return nil
	}
	hub := *a.nginxHub
	path := a.nginxHubProjectPath
	a.nginxLoading = true
	return func() tea.Msg {
		incs, err := nginxutil.DiscoverHubIncs(path, hub)
		if err != nil {
			return nginxIncsLoadedMsg{err: err.Error()}
		}
		return nginxIncsLoadedMsg{incs: incs}
	}
}

func (a *App) handleNginxMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case nginxLoadedMsg:
		a.nginxLoading = false
		if m.err != "" {
			a.nginxErr = m.err
			a.nginxSites = nil
			a.nginxForeign = 0
		} else {
			a.nginxErr = ""
			a.nginxLayout = m.layout
			a.nginxSites = m.sites
			a.nginxForeign = m.foreign
		}
		if a.nginxHub == nil && a.nginxCursor >= len(a.nginxSites) {
			a.nginxCursor = maxInt(0, len(a.nginxSites)-1)
		}
	case nginxIncsLoadedMsg:
		a.nginxLoading = false
		if m.err != "" {
			a.nginxErr = m.err
			a.nginxIncs = nil
		} else {
			a.nginxErr = ""
			a.nginxIncs = m.incs
		}
		if a.nginxHub != nil && a.nginxCursor >= len(a.nginxIncs) {
			a.nginxCursor = maxInt(0, len(a.nginxIncs)-1)
		}
	case nginxActionMsg:
		a.nginxLoading = false
		a.nginxConfirmDelete = false
		if m.err != "" {
			a.nginxErr = m.err
			a.nginxStatus = ""
			return a, nil
		}
		a.nginxErr = ""
		a.nginxStatus = truncate(m.out, 80)
		if a.nginxHub != nil {
			return a, a.refreshNginxIncs()
		}
		return a, a.refreshNginx(a.currentProject())
	}
	return a, nil
}

func (a *App) renderNginxLanding(p *core.Project) string {
	w, h := a.moduleSize()
	found := a.landingNginxFound
	status := "…"
	if a.landingNginxOK {
		status = "não detectado"
		if found {
			status = "detectado"
		}
	}
	ctx := a.renderModuleContext(p, w, "NGINX ROUTES", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(7, bodyH*40/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("main.conf + .conf (single/hub) — cadastro de rotas sem editar arquivo na mão"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingNginxOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando…"))
	case !found:
		openLines = append(openLines, "", StyleWarning.Render("nenhuma config de nginx encontrada"))
		openLines = append(openLines, StyleMuted.Render("crie main.conf/nginx.conf + uma pasta de .conf na raiz do projeto"))
	default:
		openLines = append(openLines, "", StyleHealthy.Render(fmt.Sprintf("%d entrada(s) em %s", a.landingNginxCount, firstNonEmpty(a.landingNginxDir, "?"))))
	}
	featLines := []string{
		StyleMuted.Render("detecta main.conf/nginx.conf na raiz do projeto"),
		StyleMuted.Render("pasta de .conf pode ter qualquer nome"),
		StyleMuted.Render("single = rota direta · hub = pasta de .inc"),
		StyleMuted.Render("enter num hub abre suas .inc pra criar/deletar"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("NGINX", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	detected := "…"
	if a.landingNginxOK {
		detected = boolLabel(found)
	}
	details := []string{
		StyleMuted.Render("Detectado ") + StyleNormal.Render(detected),
		StyleMuted.Render("Entradas  ") + StyleNormal.Render(strconv.Itoa(a.landingNginxCount)),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir console"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderNginxTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderNginxHeader(p, w)
	headerH := lipgloss.Height(header)
	bodyH := maxInt(4, h-headerH-2)

	body := a.renderNginxView(w, bodyH)
	view := lipgloss.JoinVertical(lipgloss.Left, header, body, a.renderStatusBar(a.nginxHints()))
	if a.nginxWizard {
		view = overlayCentered(view, a.renderNginxWizard(p, w, h), w, h)
	}
	if a.nginxConfirmDelete {
		t, _ := a.nginxSelected()
		detail := t.File
		if t.Kind == nginxutil.KindHub {
			detail += "  (a pasta " + filepath.Base(t.HubDir) + " e as .inc dela ficam)"
		}
		box := renderTunnelDeleteConfirmBox("NGINX", tabAccentColor(TabNginx), t.Name, detail, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) nginxHints() string {
	if a.nginxConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.nginxWizard {
		if a.nginxWizardForHub {
			return "modal nova rota (.inc)  tab campo  space ssl  enter salvar  esc"
		}
		return "modal novo .conf  tab campo  space tipo/ssl  enter salvar  esc"
	}
	if a.nginxHub != nil {
		base := "tab lista/detalhes  n nova .inc  d delete  R rescan  esc volta"
		if a.nginxLoading {
			base = a.spinner() + " carregando…  " + base
		}
		if a.nginxStatus != "" {
			return truncate(a.nginxStatus, 72) + "  ·  " + base
		}
		if a.nginxErr != "" {
			return StyleUnhealthy.Render(truncate(a.nginxErr, 60)) + "  ·  " + base
		}
		return base
	}
	scope := "A todos"
	if a.nginxShowAll {
		scope = "A projeto"
	}
	base := "tab lista/detalhes  enter abre hub  n novo .conf  d delete  " + scope + "  R rescan  esc"
	if a.nginxLoading {
		base = a.spinner() + " carregando…  " + base
	}
	if a.nginxStatus != "" {
		return truncate(a.nginxStatus, 72) + "  ·  " + base
	}
	if a.nginxErr != "" {
		return StyleUnhealthy.Render(truncate(a.nginxErr, 60)) + "  ·  " + base
	}
	return base
}

func (a *App) renderNginxHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabNginx)).Bold(true)
	name := "project"
	if p != nil {
		name = p.Name
	}
	left := accent.Render("devscope") + StyleMuted.Render(" › nginx")
	if a.nginxHub != nil {
		left += StyleMuted.Render(" › ") + StyleNormal.Render(a.nginxHub.Name)
	}
	left += StyleMuted.Render("  Projeto: ") + StyleNormal.Render(name)

	scope := StyleMuted.Render("projeto")
	if a.nginxShowAll {
		scope = StyleAccent.Render("TODOS")
	}
	right := StyleMuted.Render(fmt.Sprintf("Confs:%d  ", len(a.nginxSites)))
	if a.nginxLayout.SitesDir != "" {
		right += StyleMuted.Render("pasta: ") + StyleNormal.Render(filepath.Base(a.nginxLayout.SitesDir)) + "  "
	}
	right += scope
	if !a.nginxShowAll && a.nginxForeign > 0 {
		right += StyleMuted.Render(fmt.Sprintf("  (+%d outros · A)", a.nginxForeign))
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderNginxView(width, height int) string {
	if height < 6 {
		height = 6
	}
	leftW := maxInt(32, width*45/100)
	rightW := maxInt(28, width-leftW-1)
	left := a.renderNginxTable(leftW, height)
	right := a.renderNginxDetailsPane(rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

// nginxCurrentList é a lista relevante pro nível atual: as .conf de nível 1,
// ou as .inc do hub aberto.
func (a *App) nginxCurrentList() []nginxutil.Site {
	if a.nginxHub != nil {
		return a.nginxIncs
	}
	return a.nginxSites
}

func (a *App) renderNginxTable(width, height int) string {
	focus := a.nginxFocus == nginxFocusTable
	topLevel := a.nginxHub == nil
	list := a.nginxCurrentList()
	n := len(list)
	a.nginxScroll = ensureVisible(a.nginxCursor, a.nginxScroll, height-3, n)
	nameW := maxInt(8, width-34)
	var header string
	if topLevel {
		header = fmt.Sprintf("%-*s %-6s %-6s %-10s %s", nameW, "FILE", "KIND", "SSL", "PROJETO", "SERVER_NAME")
	} else {
		header = fmt.Sprintf("%-*s %-6s %s", nameW, "FILE", "SSL", "SERVER_NAME")
	}
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		hint := "  (nenhuma entrada encontrada — n para criar)"
		if a.nginxErr != "" {
			hint = "  (" + a.nginxErr + ")"
		}
		lines = append(lines, StyleMuted.Render(hint))
	} else {
		start := a.nginxScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			s := list[i]
			ssl := "—"
			if s.SSL {
				ssl = StyleHealthy.Render("sim")
			}
			var row string
			if topLevel {
				kind := string(s.Kind)
				if s.Kind == nginxutil.KindHub {
					kind = StyleAccent.Render("hub")
				}
				proj := firstNonEmpty(s.Project, "—")
				row = fmt.Sprintf("%-*s %-6s %-6s %-10s %s", nameW, truncate(s.Name, nameW), kind, ssl, truncate(proj, 10), truncate(strings.Join(s.ServerNames, " "), 24))
			} else {
				row = fmt.Sprintf("%-*s %-6s %s", nameW, truncate(s.Name, nameW), ssl, truncate(strings.Join(s.ServerNames, " "), 24))
			}
			prefix := "  "
			style := StyleMuted
			if i == a.nginxCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			lines = append(lines, style.Render(truncate(prefix+row, width-2)))
		}
	}
	title := fmt.Sprintf("CONFS (%d)", n)
	if !topLevel {
		hubName := ""
		if a.nginxHub != nil {
			hubName = a.nginxHub.Name
		}
		title = fmt.Sprintf("INC · %s (%d)", hubName, n)
	}
	if focus {
		title = "> " + title
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderNginxDetailsPane(width, height int) string {
	focus := a.nginxFocus == nginxFocusDetails
	innerW := maxInt(20, width-2)
	var raw []string
	s, ok := a.nginxSelected()
	if !ok {
		raw = []string{StyleMuted.Render("(selecione um item na lista)")}
	} else {
		raw = append(raw,
			StyleNormal.Bold(true).Render(truncate(s.Name, innerW)),
			"",
			tunnelDetailKV("Arquivo", s.File),
			tunnelDetailKV("Projeto", firstNonEmpty(s.Project, "(este)")),
		)
		if s.Kind != "" {
			raw = append(raw, tunnelDetailKV("Kind", string(s.Kind)))
		}
		if s.Kind == nginxutil.KindHub {
			raw = append(raw,
				tunnelDetailKV("Pasta", filepath.Base(s.HubDir)),
				tunnelDetailKV("Dica", "enter abre as rotas dela"),
			)
		} else {
			raw = append(raw,
				tunnelDetailKV("Server", strings.Join(s.ServerNames, " ")),
				tunnelDetailKV("Listen", s.Listen),
				tunnelDetailKV("SSL", boolLabel(s.SSL)),
			)
			if s.ProxyPass != "" {
				raw = append(raw, tunnelDetailKV("ProxyPass", s.ProxyPass))
			}
			if s.Root != "" {
				raw = append(raw, tunnelDetailKV("Root", s.Root))
			}
		}
		raw = append(raw, "", StyleMuted.Render("── raw ──"))
		for _, line := range strings.Split(strings.TrimRight(s.Raw, "\n"), "\n") {
			raw = append(raw, StyleMuted.Render(truncate(line, innerW)))
		}
	}
	a.nginxDetailsScroll = clampScroll(a.nginxDetailsScroll, height-2, len(raw))
	start := a.nginxDetailsScroll
	end := minInt(start+height-2, len(raw))
	lines := raw[start:end]
	title := "DETALHES"
	if focus {
		title = "> DETALHES"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) nginxSelected() (nginxutil.Site, bool) {
	list := a.nginxCurrentList()
	if a.nginxCursor < 0 || a.nginxCursor >= len(list) {
		return nginxutil.Site{}, false
	}
	return list[a.nginxCursor], true
}

func (a *App) resolveProjectPath(name string) string {
	for _, pr := range a.snapshot.Projects {
		if pr.Name == name {
			return pr.Path
		}
	}
	return ""
}

func (a *App) renderNginxWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(54, width*62/100))
	boxH := minInt(height-2, maxInt(24, height*66/100))
	innerW := maxInt(28, boxW-6)
	accent := tabAccentColor(TabNginx)

	title, subtitle := "Novo .conf", "single = rota direta · hub = pasta de .inc"
	if a.nginxWizardForHub {
		hubName := ""
		if a.nginxHub != nil {
			hubName = a.nginxHub.Name
		}
		title, subtitle = "Nova rota (.inc)", "dentro do hub "+hubName
	}
	lines := tunnelModalChrome("NGINX", accent, title, subtitle, proj, innerW)
	lines = append(lines, "")

	for _, f := range a.nginxWizardFieldsOrder() {
		box := a.renderNginxWizardFieldBox(f, innerW)
		lines = append(lines, strings.Split(box, "\n")...)
		lines = append(lines, "")
	}
	lines = append(lines,
		StyleMuted.Render("preencha proxy_pass OU root — o outro fica vazio"),
		StyleMuted.Render("tab campo  ·  space toggle  ·  enter salva  ·  esc"),
	)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) renderNginxWizardFieldBox(field int, innerW int) string {
	focused := a.nginxWizardField == field
	switch field {
	case nginxWizName:
		return renderApiTitledBox("nome (arquivo)", []string{a.renderNginxWizardFieldValue(a.nginxNewName, field)}, innerW, 3, focused)
	case nginxWizKind:
		shown := a.nginxNewKind
		if focused {
			shown += "  ⟨space⟩"
		}
		return renderApiTitledBox("tipo (single/hub)", []string{a.renderNginxWizardFieldValue(shown, field)}, innerW, 3, focused)
	case nginxWizServerName:
		return renderApiTitledBox("server_name", []string{a.renderNginxWizardFieldValue(a.nginxNewServerName, field)}, innerW, 3, focused)
	case nginxWizTarget:
		return renderApiTitledBox("proxy_pass (destino)", []string{a.renderNginxWizardFieldValue(a.nginxNewTarget, field)}, innerW, 3, focused)
	case nginxWizRoot:
		return renderApiTitledBox("root (se estático — deixe proxy_pass vazio)", []string{a.renderNginxWizardFieldValue(a.nginxNewRoot, field)}, innerW, 3, focused)
	case nginxWizPort:
		return renderApiTitledBox("porta", []string{a.renderNginxWizardFieldValue(a.nginxNewPortStr, field)}, innerW, 3, focused)
	case nginxWizSSL:
		shown := boolLabel(a.nginxNewSSL)
		if focused {
			shown += "  ⟨space⟩"
		}
		return renderApiTitledBox("ssl", []string{a.renderNginxWizardFieldValue(shown, field)}, innerW, 3, focused)
	case nginxWizHubDirName:
		return renderApiTitledBox("pasta que vai guardar as .inc", []string{a.renderNginxWizardFieldValue(a.nginxNewHubDirName, field)}, innerW, 3, focused)
	default:
		return ""
	}
}

func (a *App) renderNginxWizardFieldValue(value string, field int) string {
	focused := a.nginxWizardField == field
	if !focused {
		return StyleNormal.Render(value)
	}
	if field == nginxWizSSL || field == nginxWizKind {
		return StyleSelected.Render(value)
	}
	runes := []rune(value)
	cur := a.nginxWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return StyleSelected.Render(shown)
}

// nginxWizardFieldsOrder devolve os campos ativos, na ordem de tab, de
// acordo com o que está sendo criado: um .conf de nível 1 (single ou hub,
// com o campo de tipo) ou uma .inc dentro de um hub (sem campo de tipo).
func (a *App) nginxWizardFieldsOrder() []int {
	if a.nginxWizardForHub {
		return []int{nginxWizName, nginxWizServerName, nginxWizTarget, nginxWizRoot, nginxWizPort, nginxWizSSL}
	}
	if a.nginxNewKind == string(nginxutil.KindHub) {
		return []int{nginxWizName, nginxWizKind, nginxWizHubDirName}
	}
	return []int{nginxWizName, nginxWizKind, nginxWizServerName, nginxWizTarget, nginxWizRoot, nginxWizPort, nginxWizSSL}
}

func (a *App) cycleNginxKind() {
	for i, k := range nginxKinds {
		if k == a.nginxNewKind {
			a.nginxNewKind = nginxKinds[(i+1)%len(nginxKinds)]
			return
		}
	}
	a.nginxNewKind = nginxKinds[0]
}

func (a *App) beginNginxConfWizard() {
	a.nginxNewName = ""
	a.nginxNewKind = string(nginxutil.KindSingle)
	a.nginxNewServerName = ""
	a.nginxNewTarget = ""
	a.nginxNewRoot = ""
	a.nginxNewHubDirName = ""
	if a.nginxNewPortStr == "" {
		a.nginxNewPortStr = "80"
	}
	a.nginxNewSSL = false
	a.nginxWizardForHub = false
	a.nginxWizard = true
	a.nginxWizardField = nginxWizName
	a.nginxWizardCursor = 0
}

func (a *App) beginNginxIncWizard() {
	a.nginxNewName = ""
	a.nginxNewServerName = ""
	a.nginxNewTarget = ""
	a.nginxNewRoot = ""
	if a.nginxNewPortStr == "" {
		a.nginxNewPortStr = "80"
	}
	a.nginxNewSSL = false
	a.nginxWizardForHub = true
	a.nginxWizard = true
	a.nginxWizardField = nginxWizName
	a.nginxWizardCursor = 0
}

func (a *App) nginxWizardText() string {
	switch a.nginxWizardField {
	case nginxWizName:
		return a.nginxNewName
	case nginxWizServerName:
		return a.nginxNewServerName
	case nginxWizTarget:
		return a.nginxNewTarget
	case nginxWizRoot:
		return a.nginxNewRoot
	case nginxWizPort:
		return a.nginxNewPortStr
	case nginxWizHubDirName:
		return a.nginxNewHubDirName
	default:
		return ""
	}
}

func (a *App) setNginxWizardText(s string) {
	switch a.nginxWizardField {
	case nginxWizName:
		a.nginxNewName = s
	case nginxWizServerName:
		a.nginxNewServerName = s
	case nginxWizTarget:
		a.nginxNewTarget = s
	case nginxWizRoot:
		a.nginxNewRoot = s
	case nginxWizPort:
		a.nginxNewPortStr = s
	case nginxWizHubDirName:
		a.nginxNewHubDirName = s
	}
}

// nginxWizardFocusField move o foco delta posições dentro da lista de campos
// ativos no momento (varia com forHub/Kind — ver nginxWizardFieldsOrder).
func (a *App) nginxWizardFocusField(delta int) {
	order := a.nginxWizardFieldsOrder()
	idx := 0
	for i, f := range order {
		if f == a.nginxWizardField {
			idx = i
			break
		}
	}
	idx = ((idx+delta)%len(order) + len(order)) % len(order)
	a.nginxWizardField = order[idx]
	if a.nginxWizardField == nginxWizKind || a.nginxWizardField == nginxWizSSL {
		a.nginxWizardCursor = 0
		return
	}
	a.nginxWizardCursor = len([]rune(a.nginxWizardText()))
}

func (a *App) handleNginxKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.nginxConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			return a, a.nginxDeleteSelected(p)
		case "n", "N", "esc":
			a.nginxConfirmDelete = false
			return a, nil
		}
		return a, nil
	}
	if a.nginxWizard {
		return a.updateNginxWizard(msg, p)
	}
	switch msg.String() {
	case "esc":
		if a.nginxHub != nil {
			a.nginxHub = nil
			a.nginxIncs = nil
			a.nginxHubProjectPath = ""
			a.nginxCursor, a.nginxScroll = a.nginxTopCursor, a.nginxTopScroll
			a.nginxDetailsScroll = 0
			a.nginxStatus = ""
			a.nginxErr = ""
			return a, nil
		}
		return a, a.leaveNginxTab()
	case "tab":
		a.nginxFocus = (a.nginxFocus + 1) % 2
	case "up", "k":
		return a, a.nginxMove(-1)
	case "down", "j":
		return a, a.nginxMove(1)
	case "enter":
		if a.nginxHub != nil || p == nil {
			return a, nil
		}
		s, ok := a.nginxSelected()
		if !ok || s.Kind != nginxutil.KindHub {
			return a, nil
		}
		hubPath := p.Path
		if s.Project != "" {
			if fp := a.resolveProjectPath(s.Project); fp != "" {
				hubPath = fp
			}
		}
		hub := s
		a.nginxTopCursor, a.nginxTopScroll = a.nginxCursor, a.nginxScroll
		a.nginxHub = &hub
		a.nginxHubProjectPath = hubPath
		a.nginxCursor, a.nginxScroll, a.nginxDetailsScroll = 0, 0, 0
		a.nginxFocus = nginxFocusTable
		a.nginxStatus, a.nginxErr = "", ""
		return a, a.refreshNginxIncs()
	case "n":
		if a.nginxHub != nil {
			if a.nginxHub.Project != "" {
				a.nginxStatus = "hub de outro projeto — abra " + a.nginxHub.Project + " para editar"
				return a, nil
			}
			a.beginNginxIncWizard()
		} else {
			a.beginNginxConfWizard()
		}
	case "d":
		s, ok := a.nginxSelected()
		if !ok {
			return a, nil
		}
		foreignOwner := s.Project
		if a.nginxHub != nil {
			foreignOwner = a.nginxHub.Project
		}
		if foreignOwner != "" {
			a.nginxStatus = "item de outro projeto — abra " + foreignOwner + " para editar"
			return a, nil
		}
		a.nginxConfirmDelete = true
		a.nginxStatus = "delete?"
	case "A", "shift+a", "shift+A":
		if a.nginxHub != nil {
			return a, nil
		}
		a.nginxShowAll = !a.nginxShowAll
		if a.nginxShowAll {
			a.nginxStatus = "mostrando confs de todos os projetos"
		} else {
			a.nginxStatus = "filtrando confs do projeto"
		}
		return a, a.refreshNginx(p)
	case "R", "ctrl+r":
		if a.nginxHub != nil {
			return a, a.refreshNginxIncs()
		}
		return a, a.refreshNginx(p)
	}
	return a, nil
}

func (a *App) nginxMove(delta int) tea.Cmd {
	if a.nginxFocus == nginxFocusDetails {
		a.nginxDetailsScroll += delta
		if a.nginxDetailsScroll < 0 {
			a.nginxDetailsScroll = 0
		}
		return nil
	}
	n := len(a.nginxCurrentList())
	prev := a.nginxCursor
	a.nginxCursor += delta
	if a.nginxCursor < 0 {
		a.nginxCursor = 0
	}
	if a.nginxCursor > n-1 {
		a.nginxCursor = maxInt(0, n-1)
	}
	if a.nginxCursor != prev {
		a.nginxDetailsScroll = 0
	}
	return nil
}

func (a *App) updateNginxWizard(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.nginxWizard = false
		return a, nil
	case "enter":
		if a.nginxWizardForHub {
			return a, a.nginxCreateInc()
		}
		return a, a.nginxCreateConf(p)
	case "tab", "down":
		a.nginxWizardFocusField(1)
		return a, nil
	case "shift+tab", "up":
		a.nginxWizardFocusField(-1)
		return a, nil
	}

	switch a.nginxWizardField {
	case nginxWizKind:
		switch msg.String() {
		case " ", "left", "right":
			a.cycleNginxKind()
		}
		return a, nil
	case nginxWizSSL:
		if msg.String() == " " {
			a.nginxNewSSL = !a.nginxNewSSL
		}
		return a, nil
	}

	text := a.nginxWizardText()
	runes := []rune(text)
	cur := a.nginxWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	switch msg.String() {
	case "left":
		if cur > 0 {
			cur--
		}
	case "right":
		if cur < len(runes) {
			cur++
		}
	case "home":
		cur = 0
	case "end":
		cur = len(runes)
	case "backspace":
		if cur > 0 {
			runes = append(runes[:cur-1], runes[cur:]...)
			cur--
			a.setNginxWizardText(string(runes))
		}
	case "delete":
		if cur < len(runes) {
			runes = append(runes[:cur], runes[cur+1:]...)
			a.setNginxWizardText(string(runes))
		}
	default:
		if len(msg.Runes) > 0 {
			inserted := append([]rune(nil), msg.Runes...)
			runes = append(runes[:cur], append(inserted, runes[cur:]...)...)
			cur += len(inserted)
			a.setNginxWizardText(string(runes))
		}
	}
	a.nginxWizardCursor = cur
	return a, nil
}

func (a *App) nginxCreateConf(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	port, _ := strconv.Atoi(strings.TrimSpace(a.nginxNewPortStr))
	kind := nginxutil.KindSingle
	if a.nginxNewKind == string(nginxutil.KindHub) {
		kind = nginxutil.KindHub
	}
	n := nginxutil.NewConf{
		Name: a.nginxNewName,
		Kind: kind,
		NewSite: nginxutil.NewSite{
			Name:       a.nginxNewName,
			ServerName: a.nginxNewServerName,
			Target:     a.nginxNewTarget,
			Root:       a.nginxNewRoot,
			Port:       port,
			SSL:        a.nginxNewSSL,
		},
		HubDirName: a.nginxNewHubDirName,
	}
	path := p.Path
	a.nginxWizard = false
	a.nginxLoading = true
	return func() tea.Msg {
		site, err := nginxutil.CreateConf(path, n)
		if err != nil {
			return nginxActionMsg{err: err.Error()}
		}
		return nginxActionMsg{out: "criado " + site.File}
	}
}

func (a *App) nginxCreateInc() tea.Cmd {
	if a.nginxHub == nil {
		return nil
	}
	port, _ := strconv.Atoi(strings.TrimSpace(a.nginxNewPortStr))
	n := nginxutil.NewSite{
		Name:       a.nginxNewName,
		ServerName: a.nginxNewServerName,
		Target:     a.nginxNewTarget,
		Root:       a.nginxNewRoot,
		Port:       port,
		SSL:        a.nginxNewSSL,
	}
	hub := *a.nginxHub
	path := a.nginxHubProjectPath
	a.nginxWizard = false
	a.nginxLoading = true
	return func() tea.Msg {
		site, err := nginxutil.CreateInc(path, hub, n)
		if err != nil {
			return nginxActionMsg{err: err.Error()}
		}
		return nginxActionMsg{out: "criado " + site.File}
	}
}

func (a *App) nginxDeleteSelected(p *core.Project) tea.Cmd {
	a.nginxConfirmDelete = false
	s, ok := a.nginxSelected()
	if !ok {
		return nil
	}
	path := ""
	switch {
	case a.nginxHub != nil:
		if a.nginxHub.Project != "" {
			return nil
		}
		path = a.nginxHubProjectPath
	case p != nil && s.Project == "":
		path = p.Path
	default:
		return nil
	}
	a.nginxLoading = true
	return func() tea.Msg {
		if err := nginxutil.DeleteSite(path, s.File); err != nil {
			return nginxActionMsg{err: err.Error()}
		}
		return nginxActionMsg{out: "removido " + s.File}
	}
}
