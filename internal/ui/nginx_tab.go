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
	nginxWizServerName
	nginxWizTarget
	nginxWizRoot
	nginxWizPort
	nginxWizSSL
)

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
	if a.nginxNewPortStr == "" {
		a.nginxNewPortStr = "80"
	}
	return a.refreshNginx(p)
}

func (a *App) leaveNginxTab() tea.Cmd {
	a.nginxOpen = false
	a.nginxWizard = false
	a.nginxConfirmDelete = false
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
		if a.nginxCursor >= len(a.nginxSites) {
			a.nginxCursor = maxInt(0, len(a.nginxSites)-1)
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
		StyleMuted.Render("main.conf + sites/*.inc — cadastro de rotas sem editar arquivo na mão"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingNginxOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando…"))
	case !found:
		openLines = append(openLines, "", StyleWarning.Render("nenhuma config de nginx encontrada"))
		openLines = append(openLines, StyleMuted.Render("crie main.conf/nginx.conf + sites/ na raiz do projeto"))
	default:
		openLines = append(openLines, "", StyleHealthy.Render(fmt.Sprintf("%d rota(s) em %s", a.landingNginxCount, firstNonEmpty(a.landingNginxDir, "?"))))
	}
	featLines := []string{
		StyleMuted.Render("detecta main.conf/nginx.conf na raiz do projeto"),
		StyleMuted.Render("sites/, conf.d/, sites-available/ — o que existir"),
		StyleMuted.Render("n cria rota nova (proxy_pass ou static root)"),
		StyleMuted.Render("garante o include no main.conf quando falta"),
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
		StyleMuted.Render("Rotas     ") + StyleNormal.Render(strconv.Itoa(a.landingNginxCount)),
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
		box := renderTunnelDeleteConfirmBox("NGINX", tabAccentColor(TabNginx), t.Name, t.File, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) nginxHints() string {
	if a.nginxConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.nginxWizard {
		return "modal nova rota  tab campo  space ssl  enter salvar  esc"
	}
	scope := "A todos"
	if a.nginxShowAll {
		scope = "A projeto"
	}
	base := "tab lista/detalhes  n nova rota  d delete  " + scope + "  R rescan  esc"
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
	left := accent.Render("devscope") + StyleMuted.Render(" › nginx") +
		StyleMuted.Render("  Projeto: ") + StyleNormal.Render(name)
	scope := StyleMuted.Render("projeto")
	if a.nginxShowAll {
		scope = StyleAccent.Render("TODOS")
	}
	right := StyleMuted.Render(fmt.Sprintf("Rotas:%d  ", len(a.nginxSites)))
	if a.nginxLayout.SitesDir != "" {
		right += StyleMuted.Render("sites: ") + StyleNormal.Render(filepath.Base(a.nginxLayout.SitesDir)) + "  "
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

func (a *App) renderNginxTable(width, height int) string {
	focus := a.nginxFocus == nginxFocusTable
	n := len(a.nginxSites)
	a.nginxScroll = ensureVisible(a.nginxCursor, a.nginxScroll, height-3, n)
	nameW := maxInt(8, width-34)
	header := fmt.Sprintf("%-*s %-6s %-10s %s", nameW, "FILE", "SSL", "PROJETO", "SERVER_NAME")
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		hint := "  (nenhuma rota encontrada — n para criar)"
		if a.nginxErr != "" {
			hint = "  (" + a.nginxErr + ")"
		}
		lines = append(lines, StyleMuted.Render(hint))
	} else {
		start := a.nginxScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			s := a.nginxSites[i]
			ssl := "—"
			if s.SSL {
				ssl = StyleHealthy.Render("sim")
			}
			proj := firstNonEmpty(s.Project, "—")
			row := fmt.Sprintf("%-*s %-6s %-10s %s", nameW, truncate(s.Name, nameW), ssl, truncate(proj, 10), truncate(strings.Join(s.ServerNames, " "), 24))
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
	title := fmt.Sprintf("ROTAS (%d)", n)
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
		raw = []string{StyleMuted.Render("(selecione uma rota na lista)")}
	} else {
		raw = append(raw,
			StyleNormal.Bold(true).Render(truncate(s.Name, innerW)),
			"",
			tunnelDetailKV("Arquivo", s.File),
			tunnelDetailKV("Projeto", firstNonEmpty(s.Project, "(este)")),
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
	if a.nginxCursor < 0 || a.nginxCursor >= len(a.nginxSites) {
		return nginxutil.Site{}, false
	}
	return a.nginxSites[a.nginxCursor], true
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

	lines := tunnelModalChrome("NGINX", accent, "Nova rota", "server_name + proxy_pass ou root estático", proj, innerW)
	lines = append(lines, "")

	nameBox := renderApiTitledBox("nome (arquivo)", []string{a.renderNginxWizardFieldValue(a.nginxNewName, nginxWizName)}, innerW, 3, a.nginxWizardField == nginxWizName)
	snBox := renderApiTitledBox("server_name", []string{a.renderNginxWizardFieldValue(a.nginxNewServerName, nginxWizServerName)}, innerW, 3, a.nginxWizardField == nginxWizServerName)
	targetBox := renderApiTitledBox("proxy_pass (destino)", []string{a.renderNginxWizardFieldValue(a.nginxNewTarget, nginxWizTarget)}, innerW, 3, a.nginxWizardField == nginxWizTarget)
	rootBox := renderApiTitledBox("root (se estático — deixe proxy_pass vazio)", []string{a.renderNginxWizardFieldValue(a.nginxNewRoot, nginxWizRoot)}, innerW, 3, a.nginxWizardField == nginxWizRoot)
	portBox := renderApiTitledBox("porta", []string{a.renderNginxWizardFieldValue(a.nginxNewPortStr, nginxWizPort)}, innerW, 3, a.nginxWizardField == nginxWizPort)
	sslShown := boolLabel(a.nginxNewSSL)
	if a.nginxWizardField == nginxWizSSL {
		sslShown += "  ⟨space⟩"
	}
	sslBox := renderApiTitledBox("ssl", []string{a.renderNginxWizardFieldValue(sslShown, nginxWizSSL)}, innerW, 3, a.nginxWizardField == nginxWizSSL)

	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(snBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(targetBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(rootBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(portBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(sslBox, "\n")...)
	lines = append(lines, "",
		StyleMuted.Render("preencha proxy_pass OU root — o outro fica vazio"),
		StyleMuted.Render("tab campo  ·  space toggle ssl  ·  enter salva  ·  esc"),
	)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) renderNginxWizardFieldValue(value string, field int) string {
	focused := a.nginxWizardField == field
	if !focused {
		return StyleNormal.Render(value)
	}
	if field == nginxWizSSL {
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

func (a *App) beginNginxWizard() {
	a.nginxNewName = ""
	a.nginxNewServerName = ""
	a.nginxNewTarget = ""
	a.nginxNewRoot = ""
	if a.nginxNewPortStr == "" {
		a.nginxNewPortStr = "80"
	}
	a.nginxNewSSL = false
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
	}
}

func (a *App) nginxWizardFocusField(field int) {
	if field < nginxWizName {
		field = nginxWizSSL
	}
	if field > nginxWizSSL {
		field = nginxWizName
	}
	a.nginxWizardField = field
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
		return a, a.leaveNginxTab()
	case "tab":
		a.nginxFocus = (a.nginxFocus + 1) % 2
	case "up", "k":
		return a, a.nginxMove(-1)
	case "down", "j":
		return a, a.nginxMove(1)
	case "n":
		a.beginNginxWizard()
	case "d":
		if s, ok := a.nginxSelected(); ok {
			if s.Project != "" {
				a.nginxStatus = "rota de outro projeto — abra " + s.Project + " para editar"
				return a, nil
			}
			a.nginxConfirmDelete = true
			a.nginxStatus = "delete rota?"
		}
	case "A", "shift+a", "shift+A":
		a.nginxShowAll = !a.nginxShowAll
		if a.nginxShowAll {
			a.nginxStatus = "mostrando rotas de todos os projetos"
		} else {
			a.nginxStatus = "filtrando rotas do projeto"
		}
		return a, a.refreshNginx(p)
	case "R", "ctrl+r":
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
	prev := a.nginxCursor
	a.nginxCursor += delta
	if a.nginxCursor < 0 {
		a.nginxCursor = 0
	}
	if a.nginxCursor > len(a.nginxSites)-1 {
		a.nginxCursor = maxInt(0, len(a.nginxSites)-1)
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
		return a, a.nginxCreateSite(p)
	case "tab", "down":
		a.nginxWizardFocusField(a.nginxWizardField + 1)
		return a, nil
	case "shift+tab", "up":
		a.nginxWizardFocusField(a.nginxWizardField - 1)
		return a, nil
	case " ":
		if a.nginxWizardField == nginxWizSSL {
			a.nginxNewSSL = !a.nginxNewSSL
			return a, nil
		}
	}
	if a.nginxWizardField == nginxWizSSL {
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

func (a *App) nginxCreateSite(p *core.Project) tea.Cmd {
	if p == nil {
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
	path := p.Path
	a.nginxWizard = false
	a.nginxLoading = true
	return func() tea.Msg {
		site, err := nginxutil.CreateSite(path, n)
		if err != nil {
			return nginxActionMsg{err: err.Error()}
		}
		return nginxActionMsg{out: "criado " + site.File}
	}
}

func (a *App) nginxDeleteSelected(p *core.Project) tea.Cmd {
	a.nginxConfirmDelete = false
	s, ok := a.nginxSelected()
	if !ok || p == nil || s.Project != "" {
		return nil
	}
	path := p.Path
	a.nginxLoading = true
	return func() tea.Msg {
		if err := nginxutil.DeleteSite(path, s.File); err != nil {
			return nginxActionMsg{err: err.Error()}
		}
		return nginxActionMsg{out: "removido " + s.File}
	}
}
