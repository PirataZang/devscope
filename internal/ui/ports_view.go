package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Portas: "quem está usando a porta 3000?" para a máquina inteira.
//
// O coletor diz porta → processo (e a pasta onde ele roda). Esta tela casa
// isso com os projetos, nesta ordem de certeza:
//
//  1. container publicado na porta — o processo dono é o docker-proxy, que
//     não diz nada; quem diz é o mapeamento do container. Container de compose
//     fora da lista de projetos ainda traz a pasta do compose: vira projeto
//     pelo nome dela;
//  2. pasta de trabalho do processo dentro da pasta do projeto;
//  3. porta declarada no compose do projeto, mesmo sem nada rodando lá.
//
// A lista é AGRUPADA por dono — a pergunta da tela é "de quem é", e um
// cabeçalho por projeto responde sem ler coluna. Encerrar faz a coisa certa
// para cada dono: `docker stop` no container, SIGTERM no processo.

type portOwnerKind int

const (
	portOwnerProject portOwnerKind = iota // projeto que o DevScope escaneia
	portOwnerCompose                      // compose de uma pasta fora da lista de projetos
	portOwnerLoose                        // container sem compose
	portOwnerProgram                      // processo seu, fora de projeto
	portOwnerSystem                       // processo de outro usuário
)

type portRow struct {
	collectors.Listener
	kind      portOwnerKind
	project   string // nome do projeto dono, "" = ninguém conhecido
	path      string
	container string   // container que publica a porta, quando é o caso
	conflict  []string // outros projetos que declaram a mesma porta
}

type portsLoadedMsg struct{ list []collectors.Listener }

type portStoppedMsg struct {
	note string
	err  error
}

func (a *App) openPortsView() tea.Cmd {
	a.dashboardSubview = dashboardSubviewPorts
	a.portsCursor, a.portsScroll, a.portsStatus, a.portsConfirm = 0, 0, "", false
	return a.loadPorts()
}

func (a *App) loadPorts() tea.Cmd {
	a.portsLoading = true
	return func() tea.Msg { return portsLoadedMsg{list: collectors.ListListeners()} }
}

func (a *App) handlePortsLoaded(msg portsLoadedMsg) {
	a.portsLoading = false
	a.portsList = matchPortOwners(msg.list, a.snapshot)
	a.portsCursor = clampCursor(a.portsCursor, len(a.portsList))
}

// matchPortOwners aplica as três regras do topo do arquivo e devolve as linhas
// já na ordem dos grupos: projetos, composes de fora, containers soltos,
// programas, sistema.
func matchPortOwners(list []collectors.Listener, snap core.Snapshot) []portRow {
	type owner struct {
		kind                     portOwnerKind
		project, path, container string
	}
	byContainerPort := map[int]owner{}
	declared := map[int]owner{}
	byPath := map[string]core.Project{}
	for _, p := range snap.Projects {
		byPath[p.Path] = p
		for _, c := range p.Containers {
			for _, m := range collectors.ParseContainerPortMappings(c.Ports) {
				byContainerPort[m.HostPort] = owner{portOwnerProject, p.Name, p.Path, c.Name}
			}
		}
		for _, port := range p.Ports {
			if _, ok := declared[port]; !ok {
				declared[port] = owner{portOwnerProject, p.Name, p.Path, ""}
			}
		}
	}
	for _, c := range snap.OrphanContainers {
		o := owner{kind: portOwnerLoose, container: c.Name}
		if c.ProjectPath != "" {
			o = owner{portOwnerCompose, filepath.Base(c.ProjectPath), c.ProjectPath, c.Name}
		}
		for _, m := range collectors.ParseContainerPortMappings(c.Ports) {
			if _, ok := byContainerPort[m.HostPort]; !ok {
				byContainerPort[m.HostPort] = o
			}
		}
	}

	out := make([]portRow, 0, len(list))
	for _, l := range list {
		row := portRow{Listener: l, kind: portOwnerProgram}
		if l.PID == 0 {
			row.kind = portOwnerSystem
		}
		switch o, ok := byContainerPort[l.Port]; {
		case ok:
			row.kind, row.project, row.path, row.container = o.kind, o.project, o.path, o.container
		case l.Cwd != "":
			if p := projectContaining(snap.Projects, l.Cwd); p != nil {
				row.kind, row.project, row.path = portOwnerProject, p.Name, p.Path
			}
		}
		if row.project == "" && row.container == "" {
			if o, ok := declared[l.Port]; ok {
				row.kind, row.project, row.path = o.kind, o.project, o.path
			}
		}
		if p, ok := byPath[row.path]; ok {
			row.conflict = p.PortConflicts[l.Port]
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.project != b.project {
			return a.project < b.project
		}
		return a.Port < b.Port
	})
	return out
}

// projectContaining devolve o projeto mais específico que contém a pasta —
// monorepo com `app/` e `app/web` escolhe `app/web`.
func projectContaining(projects []core.Project, dir string) *core.Project {
	var best *core.Project
	for i := range projects {
		pp := filepath.Clean(projects[i].Path)
		if dir == pp || strings.HasPrefix(dir, pp+string(filepath.Separator)) {
			if best == nil || len(pp) > len(best.Path) {
				best = &projects[i]
			}
		}
	}
	return best
}

func (a *App) handlePortsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.portsConfirm {
		switch msg.String() {
		case "y", "Y":
			a.portsConfirm = false
			return a, a.stopSelectedPort()
		case "n", "N", "esc":
			a.portsConfirm = false
		}
		return a, nil
	}
	row, ok := a.selectedPort()
	switch msg.String() {
	case "esc":
		a.dashboardSubview = dashboardSubviewList
	case "up", "k":
		a.portsCursor = clampCursor(a.portsCursor-1, len(a.portsList))
	case "down", "j":
		a.portsCursor = clampCursor(a.portsCursor+1, len(a.portsList))
	case "r":
		a.portsStatus = ""
		return a, a.loadPorts()
	case "x":
		switch {
		case !ok:
		case row.container == "" && row.PID == 0:
			a.portsStatus = "processo de outro usuário — o DevScope não pode encerrar"
		default:
			a.portsConfirm = true
		}
	case "o":
		if ok {
			openBrowser(fmt.Sprintf("http://localhost:%d", row.Port))
			a.portsStatus = fmt.Sprintf("abrindo http://localhost:%d", row.Port)
		}
	case "enter":
		switch {
		case !ok:
		case row.kind == portOwnerProject:
			if p := projectByPath(a.snapshot.Projects, row.path); p != nil {
				a.dashboardSubview = dashboardSubviewList
				return a, a.openProject(*p, TabOverview)
			}
		case row.kind == portOwnerCompose:
			a.portsStatus = row.project + " não está na lista de projetos do DevScope"
		default:
			a.portsStatus = "nenhum projeto nesta porta"
		}
	}
	return a, nil
}

func (a *App) selectedPort() (portRow, bool) {
	if a.portsCursor < 0 || a.portsCursor >= len(a.portsList) {
		return portRow{}, false
	}
	return a.portsList[a.portsCursor], true
}

func (a *App) stopSelectedPort() tea.Cmd {
	row, ok := a.selectedPort()
	if !ok {
		return nil
	}
	a.portsStatus = "encerrando :" + strconv.Itoa(row.Port) + "…"
	return func() tea.Msg {
		if row.container != "" {
			return portStoppedMsg{note: "container " + row.container + " parado", err: collectors.DockerStop(row.container)}
		}
		return portStoppedMsg{note: fmt.Sprintf("%s (%d) encerrado", row.Process, row.PID), err: collectors.StopListener(row.PID)}
	}
}

func (a *App) handlePortStopped(msg portStoppedMsg) tea.Cmd {
	a.portsStatus = msg.note
	if msg.err != nil {
		a.portsStatus = "erro: " + lastLine(msg.err.Error())
		return nil
	}
	return a.loadPorts()
}

// ─── tela ────────────────────────────────────────────────────────────────────

func (a *App) renderPortsView() string {
	w := maxInt(40, a.width)
	h := maxInt(12, a.height)

	head := a.renderPortsHeader(w)
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"enter", "abrir projeto"},
		[2]string{"x", "encerrar"},
		[2]string{"o", "browser"},
		[2]string{"r", "atualizar"},
		[2]string{"esc", "voltar"},
	))
	detail := a.portsDetail(w)
	lines, cursorLine, headerLine := a.portsLines(maxInt(20, w-4))
	room := maxInt(3, h-1-lipgloss.Height(cmdBar)-len(detail))
	listH := maxInt(3, minInt(room, len(lines)+2))
	stack := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{head, a.renderPortsBox(lines, cursorLine, headerLine, w, listH)}, detail...)...)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	out := lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar)
	if a.portsConfirm {
		if row, ok := a.selectedPort(); ok {
			out = overlayCentered(out, renderDeleteConfirmBox(portStopOpts(row), w, h), w, h)
		}
	}
	return clampRenderedHeight(out, h)
}

// renderPortsHeader: os números que importam, cada um na cor do que ele diz —
// exposto na rede é o que pede atenção, disputa entre projetos é o que trava.
func (a *App) renderPortsHeader(w int) string {
	left := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("⇄ PORTAS")
	open, inProject, exposed, disputed := len(a.portsList), 0, 0, 0
	for _, r := range a.portsList {
		if r.kind == portOwnerProject || r.kind == portOwnerCompose {
			inProject++
		}
		if r.Addr == "*" {
			exposed++
		}
		if len(r.conflict) > 0 {
			disputed++
		}
	}
	sep := StyleMuted.Render(" · ")
	var facts []string
	switch {
	case a.portsLoading && open == 0:
		facts = append(facts, StyleMuted.Render("lendo…")) // zero antes de medir é mentira
	default:
		facts = append(facts, StyleNormal.Bold(true).Render(countOf(open, "aberta", "abertas")))
		if inProject > 0 {
			facts = append(facts, lipgloss.NewStyle().Foreground(ColorAccent).Render(strconv.Itoa(inProject)+" em projetos"))
		}
		if exposed > 0 {
			facts = append(facts, StyleWarning.Render(strconv.Itoa(exposed)+" na rede"))
		}
		if disputed > 0 {
			facts = append(facts, StyleUnhealthy.Bold(true).Render("⚠ "+countOf(disputed, "disputada", "disputadas")))
		}
	}
	left += "   " + strings.Join(facts, sep)
	if a.portsStatus == "" {
		return truncateVisible(left, w)
	}
	return joinWithSpacer(truncateVisible(left, w), StyleWarning.Render(truncate(a.portsStatus, maxInt(10, w/2))), w)
}

// portsLines monta a lista agrupada: um cabeçalho por dono e as portas dele
// embaixo. Devolve também a linha do cursor e a do cabeçalho do grupo dele —
// o cabeçalho não é selecionável, mas conta para a rolagem.
func (a *App) portsLines(inner int) (lines []string, cursorLine, headerLine int) {
	if a.portsLoading && len(a.portsList) == 0 {
		return []string{a.loadingText("lendo portas…")}, 0, 0
	}
	if len(a.portsList) == 0 {
		return []string{StyleMuted.Render("nenhuma porta TCP aberta para conexão")}, 0, 0
	}
	nameW := 8
	for _, r := range a.portsList {
		nameW = maxInt(nameW, lipgloss.Width(portProcessLabel(r)))
	}
	nameW = minInt(nameW, 34)
	const portW, scopeW = 6, 5
	tailW := maxInt(0, inner-2-1-portW-nameW-scopeW-4)

	group := 0
	for i, r := range a.portsList {
		if i == 0 || portGroupKey(a.portsList[i-1]) != portGroupKey(r) {
			group = len(lines)
			lines = append(lines, portGroupHeader(a.portsList, i, inner))
		}
		if i == a.portsCursor {
			cursorLine, headerLine = len(lines), group
		}
		glyph, glyphSt := portOwnerGlyph(r)
		nameSt := StyleNormal
		switch {
		case r.container != "":
			nameSt = lipgloss.NewStyle().Foreground(ColorDocker)
		case r.kind == portOwnerSystem:
			nameSt = StyleMuted
		}
		scope, scopeSt := "rede", StyleWarning
		if r.Addr != "*" {
			scope, scopeSt = "local", StyleHealthy
		}
		tail, tailSt := r.Cmdline, StyleMuted
		if r.container != "" {
			tail = "" // o comando do docker-proxy não diz nada
		}
		if len(r.conflict) > 0 {
			tail, tailSt = "⚠ também em "+strings.Join(r.conflict, ", "), StyleUnhealthy.Bold(true)
		}
		portSt := StyleNormal.Bold(true)
		if len(r.conflict) > 0 {
			portSt = StyleUnhealthy.Bold(true)
		}
		selected := i == a.portsCursor
		row := renderCells(selected, []dashCell{
			{text: glyph, width: 1, style: glyphSt},
			{text: ":" + strconv.Itoa(r.Port), width: portW, style: portSt},
			{text: portProcessLabel(r), width: nameW, style: nameSt},
			{text: scope, width: scopeW, style: scopeSt},
			{text: tail, width: tailW, style: tailSt},
		})
		if selected {
			row = StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	return lines, cursorLine, headerLine
}

func (a *App) renderPortsBox(lines []string, cursorLine, headerLine, width, height int) string {
	viewport := maxInt(1, height-2)
	// Subindo, o cabeçalho do grupo entra junto quando cabe: porta sem o nome
	// do dono é a pergunta da tela sem a resposta.
	if headerLine < a.portsScroll && cursorLine-headerLine < viewport {
		a.portsScroll = headerLine
	}
	a.portsScroll = ensureVisible(cursorLine, a.portsScroll, viewport, len(lines))
	end := minInt(len(lines), a.portsScroll+viewport)
	title := panelTitle("PORTAS", countIf(len(a.portsList)))
	if below := len(lines) - end; below > 0 {
		title = panelTitle("PORTAS", countIf(len(a.portsList)), "↓"+strconv.Itoa(below))
	}
	return panelBox(title, fitExactLines(lines[a.portsScroll:end], viewport), width, height, true)
}

// portGroupKey separa os grupos: mesmo tipo de dono e mesmo projeto.
func portGroupKey(r portRow) string {
	return strconv.Itoa(int(r.kind)) + "\x00" + r.project
}

// portGroupHeader: nome do dono na cor do tipo, quantas portas ele tem e, para
// projeto, onde ele mora.
func portGroupHeader(rows []portRow, i, width int) string {
	r := rows[i]
	n := 0
	for j := i; j < len(rows) && portGroupKey(rows[j]) == portGroupKey(r); j++ {
		n++
	}
	var title, tag string
	color := ColorMuted
	switch r.kind {
	case portOwnerProject:
		title, color = r.project, ColorAccent
	case portOwnerCompose:
		title, tag, color = r.project, "compose fora da lista de projetos", ColorDocker
	case portOwnerLoose:
		title = "CONTAINERS SEM PROJETO"
	case portOwnerProgram:
		title = "PROGRAMAS FORA DE PROJETOS"
	default:
		title = "SISTEMA · outro usuário"
	}
	head := lipgloss.NewStyle().Foreground(color).Bold(true).Render(title)
	meta := []string{countOf(n, "porta", "portas")}
	if tag != "" {
		meta = append(meta, tag)
	}
	if r.path != "" {
		meta = append(meta, shortenPath(r.path))
	}
	head += StyleMuted.Render("  " + strings.Join(meta, " · "))
	head = truncateVisible(head, width)
	if gap := width - lipgloss.Width(head) - 1; gap > 0 {
		head += " " + ruleColored(gap, color)
	}
	return head
}

func portOwnerGlyph(r portRow) (string, lipgloss.Style) {
	switch {
	case r.container != "":
		return "▣", lipgloss.NewStyle().Foreground(ColorDocker)
	case r.kind == portOwnerSystem:
		return "·", StyleMuted
	}
	return "⚙", lipgloss.NewStyle().Foreground(ColorAccent)
}

func (a *App) portsDetail(width int) []string {
	r, ok := a.selectedPort()
	if !ok || a.height < 20 {
		return nil
	}
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	project := []string{StyleMuted.Render("nenhum projeto conhecido")}
	switch r.kind {
	case portOwnerProject:
		project = []string{accent.Render(r.project), StyleMuted.Render(shortenPath(r.path))}
	case portOwnerCompose:
		project = []string{lipgloss.NewStyle().Foreground(ColorDocker).Bold(true).Render(r.project),
			StyleMuted.Render(shortenPath(r.path) + " — fora da lista de projetos")}
	}
	owner := []string{StyleNormal.Render(firstNonEmpty(r.Process, emDash))}
	switch {
	case r.container != "":
		owner = []string{lipgloss.NewStyle().Foreground(ColorDocker).Render("▣ " + r.container),
			StyleMuted.Render("x faz docker stop")}
	case r.PID > 0:
		owner = []string{StyleNormal.Render(fmt.Sprintf("%s %d", r.Process, r.PID)),
			StyleMuted.Render(firstNonEmpty(shortenPath(r.Cwd), emDash))}
	case r.kind == portOwnerSystem:
		owner = []string{StyleMuted.Render("processo de outro usuário — só com sudo")}
	}
	where := StyleWarning.Render("todas as interfaces — acessível por outras máquinas da rede")
	if r.Addr != "*" {
		where = StyleHealthy.Render("só " + r.Addr + " — acessível só desta máquina")
	}
	out := []string{
		rule(width),
		factLine("projeto", project, width),
		factLine("dono", owner, width),
		factLine("escuta", []string{where}, width),
	}
	if len(r.conflict) > 0 {
		out = append(out, factLine("disputa", []string{StyleUnhealthy.Bold(true).Render(
			"⚠ " + strings.Join(r.conflict, ", ") + " também usa esta porta — só um sobe por vez")}, width))
	} else if r.Cmdline != "" && r.container == "" {
		out = append(out, factLine("comando", []string{StyleMuted.Render(r.Cmdline)}, width))
	}
	return out
}

func portStopOpts(r portRow) deleteConfirmOpts {
	opts := deleteConfirmOpts{
		Brand: "PORTAS", Color: ColorPrimary, Title: "Encerrar :" + strconv.Itoa(r.Port),
		Label: "processo", Target: fmt.Sprintf("%s (%d)", r.Process, r.PID),
		Subtitle: "SIGTERM — o servidor fecha como num ctrl+c",
		Detail:   firstNonEmpty(r.project, "sem projeto conhecido"),
	}
	if r.container != "" {
		opts.Label, opts.Target, opts.Subtitle = "container", r.container, "docker stop — o container pode subir de novo depois"
	}
	return opts
}

func portProcessLabel(r portRow) string {
	switch {
	case r.container != "":
		return r.container
	case r.PID == 0:
		return "outro usuário"
	}
	return fmt.Sprintf("%s %d", r.Process, r.PID)
}
