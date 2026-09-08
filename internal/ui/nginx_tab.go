package ui

import (
	"fmt"
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
	nginxWizPath
	nginxWizLabel
	nginxWizDist
)

var nginxKinds = []string{"single", "hub"}

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
	a.nginxCursor = 0
	a.nginxScroll = 0
	a.nginxView = nginxViewRoutes
	a.nginxFileScroll, a.nginxFileHScroll = 0, 0
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
	hint := "preencha proxy_pass OU root — o outro fica vazio"
	if a.nginxWizardForHub {
		hint = "preencha porta/proxy OU dist — o outro fica vazio"
	}
	lines = append(lines,
		StyleMuted.Render(hint),
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
		label := "proxy_pass (destino)"
		if a.nginxWizardForHub {
			label = "porta ou proxy_pass (ex: 3000)"
		}
		return renderApiTitledBox(label, []string{a.renderNginxWizardFieldValue(a.nginxNewTarget, field)}, innerW, 3, focused)
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
	case nginxWizPath:
		return renderApiTitledBox("path (ex: /portfolio)", []string{a.renderNginxWizardFieldValue(a.nginxNewPath, field)}, innerW, 3, focused)
	case nginxWizLabel:
		return renderApiTitledBox("label (comentário — opcional)", []string{a.renderNginxWizardFieldValue(a.nginxNewLabel, field)}, innerW, 3, focused)
	case nginxWizDist:
		return renderApiTitledBox("dist (pasta estática — se não for proxy)", []string{a.renderNginxWizardFieldValue(a.nginxNewDist, field)}, innerW, 3, focused)
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
// acordo com o que está sendo criado:
//   - .inc dentro de um hub: um location{} — path, label opcional e o
//     destino (porta/proxy ou dist). Não tem server_name/listen/ssl — isso
//     já é do hub.
//   - .conf hub de nível 1: um server{} de verdade (server_name/porta/ssl)
//     mais a pasta que vai guardar as .inc.
//   - .conf single de nível 1: o server{} completo, com o destino direto.
func (a *App) nginxWizardFieldsOrder() []int {
	if a.nginxWizardForHub {
		return []int{nginxWizPath, nginxWizLabel, nginxWizTarget, nginxWizDist}
	}
	if a.nginxNewKind == string(nginxutil.KindHub) {
		return []int{nginxWizName, nginxWizKind, nginxWizServerName, nginxWizPort, nginxWizSSL, nginxWizHubDirName}
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
	a.nginxNewPath = ""
	a.nginxNewLabel = ""
	a.nginxNewTarget = ""
	a.nginxNewDist = ""
	a.nginxWizardForHub = true
	a.nginxWizard = true
	a.nginxWizardField = nginxWizPath
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
	case nginxWizPath:
		return a.nginxNewPath
	case nginxWizLabel:
		return a.nginxNewLabel
	case nginxWizDist:
		return a.nginxNewDist
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
	case nginxWizPath:
		a.nginxNewPath = s
	case nginxWizLabel:
		a.nginxNewLabel = s
	case nginxWizDist:
		a.nginxNewDist = s
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
	case "1", "2":
		a.nginxSetView(nginxView(msg.String()[0] - '1'))
	case "esc":
		if a.nginxView == nginxViewFile {
			a.nginxSetView(nginxViewRoutes)
			return a, nil
		}
		if a.nginxHub != nil {
			a.nginxHub = nil
			a.nginxIncs = nil
			a.nginxHubProjectPath = ""
			a.nginxCursor, a.nginxScroll = a.nginxTopCursor, a.nginxTopScroll
			a.nginxFileScroll, a.nginxFileHScroll = 0, 0
			a.nginxStatus = ""
			a.nginxErr = ""
			return a, nil
		}
		return a, a.leaveNginxTab()
	case "up", "k":
		return a, a.nginxMove(-1)
	case "down", "j":
		return a, a.nginxMove(1)
	case "pgup", "shift+up":
		return a, a.nginxMove(-a.nginxFileViewport())
	case "pgdown", "shift+down":
		return a, a.nginxMove(a.nginxFileViewport())
	case "left", "h":
		a.nginxFileHScrollBy(-8)
	case "right", "l":
		a.nginxFileHScrollBy(8)
	case "0":
		a.nginxFileHScroll = 0
	case "enter":
		if a.nginxView == nginxViewFile {
			return a, nil
		}
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
		a.nginxCursor, a.nginxScroll = 0, 0
		a.nginxFileScroll, a.nginxFileHScroll = 0, 0
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

// nginxMove: na aba ROTAS anda o cursor da lista, na aba ARQUIVO rola o texto.
// A tecla é a mesma; o que ela move é o que está na frente.
func (a *App) nginxMove(delta int) tea.Cmd {
	if a.nginxView == nginxViewFile {
		s, ok := a.nginxSelected()
		if !ok {
			return nil
		}
		a.nginxFileScroll = clampScroll(a.nginxFileScroll+delta,
			a.nginxFileViewport(), len(nginxFileLines(s)))
		return nil
	}
	n := len(a.nginxCurrentList())
	prev := a.nginxCursor
	a.nginxCursor = clampInt(a.nginxCursor+delta, 0, maxInt(0, n-1))
	if a.nginxCursor != prev {
		a.nginxFileScroll, a.nginxFileHScroll = 0, 0
	}
	return nil
}

// nginxSetView troca de aba sem carregar nada: as duas leem o mesmo estado.
func (a *App) nginxSetView(v nginxView) {
	if v < 0 || int(v) >= nginxViewTotal || v == a.nginxView {
		return
	}
	a.nginxView = v
	a.nginxStatus, a.nginxErr = "", ""
}

func (a *App) nginxFileHScrollBy(delta int) {
	s, ok := a.nginxSelected()
	if !ok {
		return
	}
	lines := nginxFileLines(s)
	textW := maxInt(8, a.screenWidth()-8)
	a.nginxFileHScroll = clampInt(a.nginxFileHScroll+delta, 0,
		maxInt(0, nginxMaxLineWidth(lines)-textW))
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
	n := nginxutil.NewLocation{
		Path:   a.nginxNewPath,
		Label:  a.nginxNewLabel,
		Target: a.nginxNewTarget,
		Dist:   a.nginxNewDist,
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
