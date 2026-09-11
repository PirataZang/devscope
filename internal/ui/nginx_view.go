package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/nginxutil"
)

// Console do Nginx no padrão de docs/DESIGN.md: cabeçalho de identificação,
// régua numerada, corpo e barra de comandos. Antes eram duas colunas de mesmo
// peso — a da direita repetia nome/tipo/ssl que a tabela já mostrava e só então
// despejava o arquivo em cinza, sem numeração e sem realce.

type nginxView int

const (
	nginxViewRoutes nginxView = iota
	nginxViewFile
)

const nginxViewTotal = int(nginxViewFile) + 1

func (v nginxView) label() string {
	if v == nginxViewFile {
		return "ARQUIVO"
	}
	return "ROTAS"
}

func (a *App) renderNginxTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()

	header := a.renderNginxHeader(p, w)
	ruler := a.renderNginxRuler(w)
	cmdBar := a.renderNginxCommandBar(w)
	bodyH := maxInt(6, h-lipgloss.Height(header)-lipgloss.Height(ruler)-lipgloss.Height(cmdBar))

	var body string
	if a.nginxView == nginxViewFile {
		body = a.renderNginxFilePanel(w, bodyH, true)
	} else {
		body = a.renderNginxRoutesView(w, bodyH)
	}

	stack := lipgloss.JoinVertical(lipgloss.Left, header, ruler, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	view := clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)

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

// renderNginxHeader: qual projeto, dentro de qual hub, e de onde as configs
// estão sendo lidas. O breadcrumb "devscope › nginx" saiu — a barra lateral já
// diz em que módulo você está.
func (a *App) renderNginxHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabNginx)).Bold(true)
	left := accent.Render("▤ NGINX")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   ") + StyleNormal.Bold(true).Render(truncate(p.Name, 24))
	}
	if a.nginxHub != nil {
		left += StyleMuted.Render("  ›  ") + StyleAccent.Render(truncate(a.nginxHub.Name, 22)) +
			StyleMuted.Render("  ·  hub")
	}

	var right []string
	if dir := a.nginxLayout.SitesDir; dir != "" {
		right = append(right, StyleMuted.Render(elideLeft(dir, 34)))
	}
	if a.nginxLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// renderNginxRuler junta as abas numeradas com os contadores. Eram três lugares
// dizendo quantas confs existem: o cabeçalho, o título da caixa e nada mais.
func (a *App) renderNginxRuler(width int) string {
	parts := make([]string, 0, nginxViewTotal)
	for i := 0; i < nginxViewTotal; i++ {
		v := nginxView(i)
		label := fmt.Sprintf(" %d %s ", i+1, v.label())
		if v == nginxViewRoutes {
			label = fmt.Sprintf(" %d %s %d ", i+1, v.label(), len(a.nginxCurrentList()))
		}
		if v == a.nginxView {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	right := a.nginxRulerStatus()
	if right == "" || lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, right+" ", width)
}

// nginxRulerStatus: recado transitório na frente, senão os contadores.
func (a *App) nginxRulerStatus() string {
	if a.nginxErr != "" {
		return StyleUnhealthy.Render(truncate(a.nginxErr, 52))
	}
	if a.nginxStatus != "" {
		return StyleWarning.Render(truncate(a.nginxStatus, 52))
	}
	if a.nginxView == nginxViewFile {
		if pos := a.nginxFilePosition(); pos != "" {
			return StyleMuted.Render(pos)
		}
		return ""
	}

	var chips []string
	hubs, singles, ssl := a.nginxCounts()
	if a.nginxHub == nil {
		if hubs > 0 {
			chips = append(chips, StyleAccent.Render(strconv.Itoa(hubs))+StyleMuted.Render(plural(hubs, " hub", " hubs")))
		}
		if singles > 0 {
			chips = append(chips, StyleNormal.Render(strconv.Itoa(singles))+StyleMuted.Render(" single"))
		}
	}
	if ssl > 0 {
		chips = append(chips, StyleHealthy.Render(strconv.Itoa(ssl))+StyleMuted.Render(" com ssl"))
	}
	if a.nginxHub == nil {
		if a.nginxShowAll {
			chips = append(chips, StyleAccent.Render("A todos os projetos"))
		} else if a.nginxForeign > 0 {
			chips = append(chips, StyleMuted.Render(fmt.Sprintf("+%d de outros projetos · A", a.nginxForeign)))
		}
	}
	return strings.Join(chips, StyleMuted.Render("  ·  "))
}

func (a *App) nginxCounts() (hubs, singles, ssl int) {
	for _, s := range a.nginxCurrentList() {
		switch s.Kind {
		case nginxutil.KindHub:
			hubs++
		case nginxutil.KindSingle:
			singles++
		}
		if s.SSL {
			ssl++
		}
	}
	return hubs, singles, ssl
}

func (a *App) renderNginxCommandBar(width int) string {
	items := [][2]string{{"1-2", "abas"}}
	if a.nginxView == nginxViewFile {
		items = append(items, [2]string{"↑↓", "rolar"}, [2]string{"←→", "lateral"})
		if a.nginxFileHScroll > 0 {
			items = append(items, [2]string{"0", "voltar ao início"})
		}
	} else {
		items = append(items, [2]string{"↑↓", "navegar"})
		if s, ok := a.nginxSelected(); ok && a.nginxHub == nil && s.Kind == nginxutil.KindHub {
			items = append(items, [2]string{"enter", "abrir hub"})
		}
	}
	if a.nginxHub != nil {
		items = append(items, [2]string{"n", "nova rota"})
	} else {
		items = append(items, [2]string{"n", "novo .conf"})
	}
	items = append(items, [2]string{"d", "apagar"})
	if a.nginxHub == nil {
		scope := "ver todos os projetos"
		if a.nginxShowAll {
			scope = "ver só este projeto"
		}
		items = append(items, [2]string{"A", scope})
	}
	items = append(items, [2]string{"R", "reescanear"}, [2]string{"esc", "voltar"})
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
}

// renderNginxRoutesView: lista à esquerda, arquivo selecionado à direita. Em
// terminal estreito o arquivo sai — a lista é que responde "o que existe aqui".
func (a *App) renderNginxRoutesView(width, height int) string {
	if width < 96 {
		return a.renderNginxTable(width, height)
	}
	leftW := maxInt(52, width*58/100)
	rightW := width - leftW - 1
	return lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderNginxTable(leftW, height), " ",
		a.renderNginxFilePanel(rightW, height, false))
}

// nginxCurrentList é a lista relevante pro nível atual: as .conf de nível 1,
// ou as .inc do hub aberto.
func (a *App) nginxCurrentList() []nginxutil.Site {
	if a.nginxHub != nil {
		return a.nginxIncs
	}
	return a.nginxSites
}

type nginxCols struct{ kind, name, server, target, ssl, project int }

// nginxColumns dimensiona pela largura do painel, não pela do terminal: a
// tabela vive dentro de uma caixa e mais estreita que a tela.
func nginxColumns(width int, topLevel, showProject bool) nginxCols {
	var c nginxCols
	if topLevel {
		c = nginxCols{kind: 6, name: 12, server: 14, target: 16, ssl: 3}
		if showProject {
			c.project = 10
		}
	} else {
		c = nginxCols{name: 10, server: 12, target: 18}
	}

	// Estreitou: as colunas opcionais caem antes de espremer nome e destino.
	for _, drop := range []*int{&c.project, &c.ssl, &c.kind} {
		if nginxColsWidth(c) <= width {
			break
		}
		*drop = 0
	}
	for nginxColsWidth(c) > width && (c.server > 8 || c.target > 10) {
		if c.server > 8 {
			c.server--
		}
		if nginxColsWidth(c) > width && c.target > 10 {
			c.target--
		}
	}

	surplus := width - nginxColsWidth(c)
	for _, g := range []struct {
		col *int
		max int
	}{{&c.server, 32}, {&c.target, 46}, {&c.name, 26}, {&c.project, 16}} {
		if surplus <= 0 {
			break
		}
		if *g.col == 0 {
			continue
		}
		if add := minInt(surplus, g.max-*g.col); add > 0 {
			*g.col += add
			surplus -= add
		}
	}
	return c
}

// nginxColsWidth: 2 do prefixo do cursor mais, por coluna visível, o separador
// de 1 que renderCells insere entre todas as células.
func nginxColsWidth(c nginxCols) int {
	w := 2
	for _, col := range []int{c.kind, c.name, c.server, c.target, c.ssl, c.project} {
		if col > 0 {
			w += 1 + col
		}
	}
	return w
}

func (a *App) renderNginxTable(width, height int) string {
	topLevel := a.nginxHub == nil
	list := a.nginxCurrentList()
	viewport := maxInt(1, height-3) // moldura + cabeçalho de colunas
	a.nginxScroll = ensureVisible(a.nginxCursor, a.nginxScroll, viewport, len(list))

	inner := maxInt(20, width-2)
	cols := nginxColumns(inner, topLevel, a.nginxShowAll || a.nginxForeign > 0)

	lines := []string{a.renderNginxTableHeader(cols, topLevel)}
	if len(list) == 0 {
		lines = append(lines, "", StyleMuted.Render(a.nginxEmptyHint()))
	}
	start := a.nginxScroll
	end := minInt(start+viewport, len(list))
	for i := start; i < end; i++ {
		lines = append(lines, a.renderNginxRow(cols, list[i], topLevel, i == a.nginxCursor))
	}

	title := "ROTAS"
	if !topLevel && a.nginxHub != nil {
		title = "ROTAS DE " + strings.ToUpper(a.nginxHub.Name)
	}
	return panelBox(title, fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderNginxTableHeader(c nginxCols, topLevel bool) string {
	server, target := "SERVER_NAME", "DESTINO"
	if !topLevel {
		server, target = "LOCATION", "DESTINO"
	}
	return renderCells(false, []dashCell{
		{text: "", width: 2},
		{text: "TIPO", width: c.kind, style: StyleTableHeader},
		{text: "ARQUIVO", width: c.name, style: StyleTableHeader},
		{text: server, width: c.server, style: StyleTableHeader},
		{text: target, width: c.target, style: StyleTableHeader},
		{text: "SSL", width: c.ssl, style: StyleTableHeader},
		{text: "PROJETO", width: c.project, style: StyleTableHeader},
	})
}

func (a *App) renderNginxRow(c nginxCols, s nginxutil.Site, topLevel, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	kind, kindStyle := nginxKindCell(s)
	ssl, sslStyle := "—", StyleMuted
	if s.SSL {
		ssl, sslStyle = "ssl", StyleHealthy
	}
	second := strings.Join(s.ServerNames, " ")
	if !topLevel {
		second = firstNonEmpty(s.Location, emDash)
	}
	return renderCells(selected, []dashCell{
		{text: prefix, width: 2, style: StyleAccent},
		{text: kind, width: c.kind, style: kindStyle},
		{text: s.Name, width: c.name, style: a.nginxNameStyle(s)},
		{text: firstNonEmpty(second, emDash), width: c.server, style: StyleMuted},
		{text: nginxTargetLabel(s), width: c.target, style: StyleNormal},
		{text: ssl, width: c.ssl, style: sslStyle},
		{text: firstNonEmpty(s.Project, emDash), width: c.project, style: a.nginxProjectStyle(s)},
	})
}

// Item de outro projeto sai em amarelo, como na lista de containers — é o aviso
// de que ele não pode ser editado daqui. Vai no nome também porque a coluna
// PROJETO é a primeira a cair quando o painel estreita.
func (a *App) nginxProjectStyle(s nginxutil.Site) lipgloss.Style {
	if s.Project == "" {
		return StyleMuted
	}
	return StyleWarning
}

func (a *App) nginxNameStyle(s nginxutil.Site) lipgloss.Style {
	if s.Project != "" {
		return StyleWarning
	}
	return StyleNormal
}

func nginxKindCell(s nginxutil.Site) (string, lipgloss.Style) {
	switch s.Kind {
	case nginxutil.KindHub:
		return "hub", StyleAccent
	case nginxutil.KindSingle:
		return "single", StyleNormal
	default:
		return "inc", StyleMuted
	}
}

// nginxTargetLabel: para onde a rota aponta. O hub não tem destino próprio —
// aponta para a pasta que guarda as .inc dele.
func nginxTargetLabel(s nginxutil.Site) string {
	if s.Kind == nginxutil.KindHub {
		if s.HubDir != "" {
			return filepath.Base(s.HubDir) + "/"
		}
		return "(sem pasta)"
	}
	if s.ProxyPass != "" {
		return strings.TrimPrefix(strings.TrimPrefix(s.ProxyPass, "http://"), "https://")
	}
	if s.Root != "" {
		return s.Root
	}
	return emDash
}

func (a *App) nginxEmptyHint() string {
	if a.nginxErr != "" {
		return a.nginxErr
	}
	if a.nginxLoading {
		return ""
	}
	if a.nginxHub != nil {
		return "hub sem rotas ainda — n cria a primeira .inc"
	}
	if a.nginxForeign > 0 {
		return fmt.Sprintf("nenhuma conf deste projeto — A mostra as %d de outros, n cria uma", a.nginxForeign)
	}
	return "nenhuma conf encontrada — n cria a primeira"
}

// renderNginxFilePanel mostra o arquivo da rota selecionada com numeração e
// realce. Antes ele vinha inteiro em cinza dentro da caixa DETALHES, depois de
// oito linhas que repetiam a tabela.
func (a *App) renderNginxFilePanel(width, height int, full bool) string {
	s, ok := a.nginxSelected()
	if !ok {
		return panelBox("ARQUIVO",
			fitExactLines([]string{StyleMuted.Render("selecione uma rota na lista")}, height-2),
			width, height, full)
	}

	all := nginxFileLines(s)
	title := panelTitle(truncate(s.File, maxInt(12, width-24)), fmt.Sprintf("%d linhas", len(all)))

	viewport := maxInt(1, height-2)
	a.nginxFileScroll = clampScroll(a.nginxFileScroll, viewport, len(all))
	start := a.nginxFileScroll
	end := minInt(start+viewport, len(all))

	gutter := maxInt(3, len(strconv.Itoa(len(all))))
	textW := maxInt(8, maxInt(20, width-2)-gutter-3)
	a.nginxFileHScroll = clampScroll(a.nginxFileHScroll, textW, nginxMaxLineWidth(all))

	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		lines = append(lines,
			StyleMuted.Render(padLeft(strconv.Itoa(i+1), gutter))+StyleMuted.Render(" │ ")+
				highlightNginxLine(sliceColumns(sanitizeTerminalLine(all[i]), a.nginxFileHScroll, textW)))
	}
	return panelBox(title, fitExactLines(lines, viewport), width, height, full)
}

func nginxFileLines(s nginxutil.Site) []string {
	raw := strings.TrimRight(s.Raw, "\n")
	if strings.TrimSpace(raw) == "" {
		return []string{"(arquivo vazio)"}
	}
	return strings.Split(raw, "\n")
}

func nginxMaxLineWidth(lines []string) int {
	w := 0
	for _, line := range lines {
		w = maxInt(w, lipgloss.Width(line))
	}
	return w
}

func (a *App) nginxFilePosition() string {
	s, ok := a.nginxSelected()
	if !ok {
		return ""
	}
	n := len(nginxFileLines(s))
	viewport := maxInt(1, a.nginxFileViewport())
	start := a.nginxFileScroll
	end := minInt(start+viewport, n)
	pos := fmt.Sprintf("%d-%d/%d", start+1, end, n)
	if a.nginxFileHScroll > 0 {
		pos = fmt.Sprintf("↔ %d  ·  %s", a.nginxFileHScroll, pos)
	}
	return pos
}

// nginxFileViewport é a mesma conta do corpo, para a posição na régua bater com
// o que a caixa mostra e a rolagem parar onde o texto acaba.
func (a *App) nginxFileViewport() int {
	w := a.screenWidth()
	bodyH := maxInt(6, a.screenHeight()-2-lipgloss.Height(a.renderNginxCommandBar(w)))
	return maxInt(1, bodyH-2)
}

// highlightNginxLine: diretiva em destaque, bloco em negrito, comentário e
// pontuação apagados. Só pinta — o texto continua idêntico ao do arquivo.
func highlightNginxLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return StyleMuted.Render(line)
	}
	if strings.HasPrefix(trimmed, "}") {
		return indent + StyleMuted.Render(trimmed)
	}

	word := trimmed
	rest := ""
	if i := strings.IndexAny(trimmed, " \t"); i > 0 {
		word, rest = trimmed[:i], trimmed[i:]
	}
	style := lipgloss.NewStyle().Foreground(ColorAccent)
	if strings.HasSuffix(strings.TrimRight(trimmed, " \t"), "{") {
		style = StyleKey // abertura de bloco: server, location, upstream
	}
	return indent + style.Render(word) + nginxHighlightArgs(rest)
}

// nginxHighlightArgs deixa ";" "{" "}" apagados e o argumento em texto normal.
func nginxHighlightArgs(rest string) string {
	var b strings.Builder
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			b.WriteString(StyleNormal.Render(buf.String()))
			buf.Reset()
		}
	}
	for _, r := range rest {
		switch r {
		case ';', '{', '}':
			flush()
			b.WriteString(StyleMuted.Render(string(r)))
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
