package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type dbPane int

const (
	dbPaneTables dbPane = iota
	dbPaneQuery
	dbPaneResult
)

type dbTablesMsg struct {
	tables []string
	err    string
}

type dbQueryMsg struct {
	out string
	err string
}

type dbSchemaMsg struct {
	table string
	info  collectors.DBTableInfo
	err   string
}

func (a *App) enterDbTab(_ *core.Project) {
	a.tab = TabDatabase
	a.tabCursor = 0
	a.dbOpen = false
	a.dbEditing = false
	a.dbFilterOn = false
}

func (a *App) openDbClient(p *core.Project) tea.Cmd {
	a.dbOpen = true
	a.dbEditing = false
	a.dbPane = dbPaneTables
	a.dbTableCursor = 0
	a.dbTablesScroll = 0
	a.dbResultScroll = 0
	a.dbResultHScroll = 0
	a.dbResult = ""
	a.dbResultRows = 0
	a.dbErr = ""
	a.dbLoading = false
	a.dbSchemaLoading = false
	a.dbSchema = collectors.DBTableInfo{}
	a.dbSchemaErr = ""
	a.dbFilterOn = false
	a.dbFilter = ""
	a.dbFilterInput = ""
	// Lite on open (no docker exec). Env from container is resolved when querying.
	a.dbTargets = collectors.DetectProjectDatabasesLite(p)
	a.dbTargetIdx = 0
	if a.dbSQL == "" {
		a.dbSQL = "SELECT 1;"
	}
	a.dbEditorCursor = len([]rune(a.dbSQL))
	return a.refreshDbTables(p)
}

func (a *App) leaveDbTab() tea.Cmd {
	a.dbOpen = false
	a.dbEditing = false
	a.dbLoading = false
	a.dbFilterOn = false
	a.tab = TabDatabase
	a.tabCursor = 0
	return nil
}

func (a *App) renderDbLanding(p *core.Project) string {
	w, h := a.moduleSize()
	// Lite: never docker exec in View (full detect runs on open client).
	targets := collectors.DetectProjectDatabasesLite(p)
	status := fmt.Sprintf("%d detectado(s)", len(targets))
	ctx := a.renderModuleContext(p, w, "Database", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(5, bodyH*26/100)
	listH := maxInt(6, bodyH*40/100)
	keysH := maxInt(5, bodyH-openH-listH)
	openLines := append([]string{StyleMuted.Render("tabelas, schema e SQL no projeto")}, moduleOpenHint()...)
	listLines := make([]string, 0, listH-2)
	if len(targets) == 0 {
		listLines = append(listLines,
			StyleMuted.Render("nenhum Postgres/MySQL nos containers"),
			StyleMuted.Render("suba o compose com um serviço db"),
			StyleMuted.Render("aceita postgres · timescale · mysql · mariadb"),
			StyleMuted.Render("ou conceda acesso em .devscope/database.json"),
		)
	} else {
		for _, t := range targets {
			ports := t.Ports
			if ports == "" {
				ports = "—"
			}
			listLines = append(listLines, fmt.Sprintf("%s %s",
				StyleIconDocker.Render("●"),
				StyleNormal.Render(t.Label)))
			listLines = append(listLines, StyleMuted.Render(fmt.Sprintf("  %s · %s@%s · %s",
				t.Engine, t.User, t.Database, truncate(ports, centerW-8))))
		}
	}
	keyLines := []string{
		StyleMuted.Render("↑↓ / j k   tabelas"),
		StyleMuted.Render("enter      preview LIMIT 50"),
		StyleMuted.Render("d          schema da tabela"),
		StyleMuted.Render("e / ctrl+enter  editar / run SQL"),
		StyleMuted.Render("b          filtrar tabelas"),
		StyleMuted.Render("config em .devscope/database.json"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DATABASE", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("DETECTADOS", fitExactLines(listLines, listH-2), centerW, listH, false),
		renderApiTitledBox("ATALHOS NO CLIENTE", fitExactLines(keyLines, keysH-2), centerW, keysH, false),
	)
	details := []string{
		StyleMuted.Render("Targets ") + StyleNormal.Render(fmt.Sprintf("%d", len(targets))),
	}
	if len(targets) > 0 {
		t := targets[0]
		details = append(details,
			StyleMuted.Render("Engine  ")+StyleNormal.Render(string(t.Engine)),
			StyleMuted.Render("DB      ")+StyleNormal.Render(truncate(t.Database, rightW-10)),
			StyleMuted.Render("User    ")+StyleMuted.Render(truncate(t.User, rightW-10)),
			StyleMuted.Render("Label   ")+StyleMuted.Render(truncate(t.Label, rightW-10)),
		)
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir cliente"},
		[2]string{"r", "atualizar scan"},
		[2]string{"3", "containers"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) currentDbTarget() (collectors.DBTarget, bool) {
	if a.dbTargetIdx < 0 || a.dbTargetIdx >= len(a.dbTargets) {
		return collectors.DBTarget{}, false
	}
	return a.dbTargets[a.dbTargetIdx], true
}

func (a *App) filteredDbTables() []string {
	q := strings.ToLower(strings.TrimSpace(a.dbFilter))
	if q == "" {
		return a.dbTables
	}
	out := make([]string, 0, len(a.dbTables))
	for _, t := range a.dbTables {
		if strings.Contains(strings.ToLower(t), q) {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) syncDbTableCursor() {
	n := len(a.filteredDbTables())
	if n == 0 {
		a.dbTableCursor = 0
		a.dbTablesScroll = 0
		return
	}
	if a.dbTableCursor >= n {
		a.dbTableCursor = n - 1
	}
	if a.dbTableCursor < 0 {
		a.dbTableCursor = 0
	}
}

func (a *App) selectedDbTable() (string, bool) {
	vis := a.filteredDbTables()
	if a.dbTableCursor < 0 || a.dbTableCursor >= len(vis) {
		return "", false
	}
	return vis[a.dbTableCursor], true
}

func (a *App) refreshDbTables(p *core.Project) tea.Cmd {
	t, ok := a.currentDbTarget()
	if !ok || p == nil {
		a.dbTables = nil
		a.dbErr = "nenhum banco detectado"
		return nil
	}
	a.dbLoading = true
	a.dbErr = ""
	path := p.Path
	return func() tea.Msg {
		tables, err := collectors.DBListTables(t, path)
		if err != nil {
			return dbTablesMsg{err: err.Error()}
		}
		return dbTablesMsg{tables: tables}
	}
}

func (a *App) refreshDbSchema(p *core.Project) tea.Cmd {
	table, ok := a.selectedDbTable()
	if !ok || p == nil {
		a.dbSchema = collectors.DBTableInfo{}
		a.dbSchemaErr = ""
		return nil
	}
	t, tok := a.currentDbTarget()
	if !tok {
		return nil
	}
	a.dbSchemaLoading = true
	path := p.Path
	return func() tea.Msg {
		info, err := collectors.DBDescribeTable(t, path, table)
		if err != nil {
			return dbSchemaMsg{table: table, err: err.Error()}
		}
		return dbSchemaMsg{table: table, info: info}
	}
}

func (a *App) runDbQuery(p *core.Project) tea.Cmd {
	t, ok := a.currentDbTarget()
	if !ok || p == nil {
		a.dbErr = "nenhum banco detectado"
		return nil
	}
	sql := strings.TrimSpace(a.dbSQL)
	if sql == "" {
		a.dbErr = "SQL vazio"
		return nil
	}
	a.dbLoading = true
	a.dbErr = ""
	a.dbPane = dbPaneResult
	path := p.Path
	return func() tea.Msg {
		out, err := collectors.DBQuery(t, path, sql)
		if err != nil {
			return dbQueryMsg{out: out, err: err.Error()}
		}
		return dbQueryMsg{out: out}
	}
}

func (a *App) handleDbMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case dbTablesMsg:
		a.dbLoading = false
		if m.err != "" {
			a.dbErr = m.err
			a.dbTables = nil
			return a, nil
		}
		a.dbTables = m.tables
		a.dbErr = ""
		a.syncDbTableCursor()
		p := a.selectedProject
		return a, a.refreshDbSchema(p)
	case dbQueryMsg:
		a.dbLoading = false
		a.dbResultScroll = 0
		a.dbResultHScroll = 0
		a.dbResultRows = parseDBResultRows(m.out)
		if m.err != "" {
			a.dbErr = m.err
			a.dbResult = m.out
			return a, nil
		}
		a.dbErr = ""
		a.dbResult = m.out
	case dbSchemaMsg:
		a.dbSchemaLoading = false
		cur, _ := a.selectedDbTable()
		if m.table != "" && cur != "" && m.table != cur {
			return a, nil // stale
		}
		if m.err != "" {
			a.dbSchemaErr = m.err
			a.dbSchema = collectors.DBTableInfo{Table: m.table}
			return a, nil
		}
		a.dbSchemaErr = ""
		a.dbSchema = m.info
	}
	return a, nil
}

func parseDBResultRows(out string) int {
	low := strings.ToLower(out)
	// postgres: (12 rows) / (1 row)
	for _, line := range strings.Split(low, "\n") {
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "(%d rows)", &n); err == nil {
			return n
		}
		if _, err := fmt.Sscanf(line, "(%d row)", &n); err == nil {
			return n
		}
		if strings.Contains(line, "rows in set") {
			var m int
			if _, err := fmt.Sscanf(line, "%d rows in set", &m); err == nil {
				return m
			}
		}
	}
	lines := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines > 1 {
		return lines - 1 // header guess
	}
	return 0
}

// renderDbTab: sem sub-abas — tabelas, schema, SQL e resultado convivem na
// mesma tela — então a régua numerada sai; no lugar fica a régua de
// contadores (§5), que substitui os cinco cards de um número cada e a linha
// de filtro que existiam antes. A coluna vertical "AÇÕES" também saiu: os
// atalhos vão na barra de comandos, presa embaixo (§2.3).
func (a *App) renderDbTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	a.syncDbTableCursor()

	header := a.renderDbHeader(p, w)
	status := a.renderDbStatusLine(w)
	cmdBar := a.renderDbCommandBar(w)
	bodyH := maxInt(10, h-lipgloss.Height(header)-lipgloss.Height(status)-lipgloss.Height(cmdBar))

	topH := maxInt(6, bodyH*42/100)
	sqlH := maxInt(4, bodyH*22/100)
	resultH := maxInt(5, bodyH-topH-sqlH)

	leftW := maxInt(22, w*28/100)
	if leftW > 36 {
		leftW = 36
	}
	schemaW := maxInt(24, w-leftW-1)
	tables := a.renderDbTablesPane(leftW, topH)
	schema := a.renderDbSchemaPane(schemaW, topH)
	top := lipgloss.JoinHorizontal(lipgloss.Top, tables, schema)
	query := a.renderDbQueryPane(w, sqlH)
	result := a.renderDbResultPane(w, resultH)
	body := lipgloss.JoinVertical(lipgloss.Left, top, query, result)

	stack := lipgloss.JoinVertical(lipgloss.Left, header, status, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)
}

func (a *App) renderDbHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabDatabase)).Bold(true)
	left := accent.Render("▦ DATABASE")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   ") + StyleNormal.Bold(true).Render(truncate(p.Name, 24))
	}
	t, ok := a.currentDbTarget()
	var right []string
	if ok {
		target := accent.Render(string(t.Engine)) + StyleMuted.Render(" · "+truncate(t.User+"@"+t.Database, 28))
		if len(a.dbTargets) > 1 {
			target += StyleMuted.Render(fmt.Sprintf("  [%d/%d]", a.dbTargetIdx+1, len(a.dbTargets)))
		}
		right = append(right, target)
	} else {
		right = append(right, StyleMuted.Render("nenhum target"))
	}
	if a.dbLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// renderDbStatusLine substitui os cards e a linha de filtro por uma régua só.
func (a *App) renderDbStatusLine(width int) string {
	if a.dbFilterOn {
		left := StyleKey.Render("filtrar ") + StyleSelected.Render(a.dbFilterInput+"▌")
		return joinWithSpacer(left, StyleMuted.Render("nome da tabela"), width)
	}
	if a.dbErr != "" {
		return padRightVisible(truncateVisible(StyleUnhealthy.Render(a.dbErr), width), width)
	}
	vis := a.filteredDbTables()
	countLbl := fmt.Sprintf("%d tabelas", len(a.dbTables))
	if q := strings.TrimSpace(a.dbFilter); q != "" {
		countLbl = fmt.Sprintf("%d/%d tabelas · \"%s\"", len(vis), len(a.dbTables), q)
	}
	parts := []string{StyleHealthy.Render(countLbl)}
	if sel, ok := a.selectedDbTable(); ok {
		parts = append(parts, StyleNormal.Render(sel))
		if n := len(a.dbSchema.Columns); n > 0 {
			parts = append(parts, StyleMuted.Render(fmt.Sprintf("%d cols", n)))
		}
		if a.dbSchema.Rows >= 0 {
			parts = append(parts, StyleMuted.Render(fmt.Sprintf("~%d rows", a.dbSchema.Rows)))
		}
	}
	if a.dbResultRows > 0 {
		parts = append(parts, StyleMuted.Render(fmt.Sprintf("resultado: %d linhas", a.dbResultRows)))
	}
	left := strings.Join(parts, StyleMuted.Render("  ·  "))
	return padRightVisible(truncateVisible(left, width), width)
}

func (a *App) renderDbCommandBar(width int) string {
	if a.dbFilterOn {
		return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2,
			[2]string{"enter", "aplicar"}, [2]string{"esc", "limpar"}))
	}
	if a.dbEditing {
		return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2,
			[2]string{"ctrl+enter", "rodar sql"}, [2]string{"esc", "sair do editor"}))
	}
	items := [][2]string{{"↑↓", "navegar"}, {"tab", "painel"}}
	switch a.dbPane {
	case dbPaneResult:
		items = append(items, [2]string{"←→", "lateral"})
	default:
		items = append(items,
			[2]string{"enter", "preview"},
			[2]string{"d", "schema"},
			[2]string{"e", "editar sql"},
			[2]string{"ctrl+enter", "rodar"},
			[2]string{"b", "filtrar"},
		)
	}
	if len(a.dbTargets) > 1 {
		items = append(items, [2]string{"[]", "trocar banco"})
	}
	items = append(items, [2]string{"r", "atualizar"}, [2]string{"esc", "voltar"})
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
}

func (a *App) renderDbTablesPane(width, height int) string {
	focus := a.dbPane == dbPaneTables && !a.dbEditing && !a.dbFilterOn
	vis := a.filteredDbTables()
	viewport := maxInt(1, height-2)
	lines := make([]string, 0, viewport)
	if a.dbLoading && len(a.dbTables) == 0 {
		lines = append(lines, a.loadingMuted("  carregando…"))
	} else if len(a.dbTables) == 0 {
		lines = append(lines, StyleMuted.Render("  (sem tabelas)"))
	} else if len(vis) == 0 {
		lines = append(lines, StyleMuted.Render("  (filtro vazio)"))
	} else {
		a.dbTablesScroll = ensureVisible(a.dbTableCursor, a.dbTablesScroll, viewport, len(vis))
		start := a.dbTablesScroll
		end := minInt(start+viewport, len(vis))
		for i := start; i < end; i++ {
			name := vis[i]
			marker := "  "
			if i == a.dbTableCursor {
				marker = "▶ "
			}
			line := truncate(marker+name, width-2)
			if i == a.dbTableCursor && focus {
				lines = append(lines, StyleSelected.Render(line))
			} else if i == a.dbTableCursor {
				lines = append(lines, StyleNormal.Render(line))
			} else {
				lines = append(lines, StyleMuted.Render(line))
			}
		}
	}
	title := fmt.Sprintf("TABELAS (%d)", len(vis))
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) renderDbSchemaPane(width, height int) string {
	viewport := maxInt(1, height-2)
	lines := make([]string, 0, viewport)
	table, ok := a.selectedDbTable()
	if !ok {
		lines = append(lines, StyleMuted.Render("  selecione uma tabela"))
	} else if a.dbSchemaLoading {
		lines = append(lines, StyleMuted.Render("  lendo schema…"))
	} else if a.dbSchemaErr != "" {
		lines = append(lines,
			StyleMuted.Render("  "+truncate(table, width-4)),
			StyleUnhealthy.Render("  "+truncate(a.dbSchemaErr, width-4)),
		)
	} else {
		rows := "—"
		if a.dbSchema.Rows >= 0 {
			rows = fmt.Sprintf("~%d rows", a.dbSchema.Rows)
		}
		lines = append(lines,
			StyleNormal.Render("  "+truncate(table, width-4)),
			StyleMuted.Render(fmt.Sprintf("  %d cols · %s", len(a.dbSchema.Columns), rows)),
			StyleMuted.Render(truncate(fmt.Sprintf("  %-16s %-18s %s", "COLUMN", "TYPE", "NULL"), width-2)),
		)
		for _, c := range a.dbSchema.Columns {
			key := ""
			switch strings.ToUpper(c.Key) {
			case "PK", "PRI":
				key = " PK"
			case "UNI":
				key = " UQ"
			case "MUL":
				key = " IX"
			}
			null := c.Nullable
			if null == "" {
				null = "—"
			}
			line := fmt.Sprintf("  %-16s %-18s %s%s",
				truncate(c.Name, 16),
				truncate(c.Type, 18),
				truncate(null, 3),
				key,
			)
			st := StyleMuted
			if key != "" {
				st = StyleWarning
			}
			lines = append(lines, st.Render(truncate(line, width-2)))
		}
		if len(a.dbSchema.Columns) == 0 {
			lines = append(lines, StyleMuted.Render("  (sem colunas / d para recarregar)"))
		}
	}
	actions := []string{
		"",
		StyleMuted.Render("  enter preview · d refresh"),
	}
	for _, l := range actions {
		if len(lines) < viewport {
			lines = append(lines, l)
		}
	}
	return renderApiTitledBox("SCHEMA", fitExactLines(lines, viewport), width, height, false)
}

func (a *App) renderDbQueryPane(width, height int) string {
	focus := a.dbPane == dbPaneQuery || a.dbEditing
	content := a.dbSQL
	if a.dbEditing {
		content = renderApiCursor(a.dbSQL, a.dbEditorCursor)
	} else if strings.TrimSpace(content) == "" {
		content = "e  editar SQL"
	}
	raw := strings.Split(content, "\n")
	viewport := maxInt(1, height-2)
	lines := make([]string, 0, viewport)
	for _, line := range raw {
		if len(lines) >= viewport {
			break
		}
		style := StyleNormal
		if focus {
			style = StyleSelected
		}
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	return renderApiTitledBox("SQL", fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) renderDbResultPane(width, height int) string {
	focus := a.dbPane == dbPaneResult && !a.dbEditing
	body := a.dbResult
	if a.dbLoading && body == "" {
		body = "executando…"
	}
	if a.dbErr != "" && body == "" {
		body = a.dbErr
	}
	if strings.TrimSpace(body) == "" {
		body = "enter numa tabela ou ctrl+enter no SQL"
	}
	raw := strings.Split(body, "\n")
	innerW := maxInt(8, width-2)
	viewport := maxInt(1, height-2)
	if a.dbResultHScroll < 0 {
		a.dbResultHScroll = 0
	}
	a.dbResultScroll = clampScroll(a.dbResultScroll, viewport, len(raw))
	start := a.dbResultScroll
	end := minInt(start+viewport, len(raw))
	lines := make([]string, 0, viewport)
	for i, line := range raw[start:end] {
		abs := start + i
		display := sliceColumns(sanitizeTerminalLine(line), a.dbResultHScroll, innerW)
		style := StyleMuted
		if focus {
			style = StyleNormal
		}
		if abs == 0 || (abs > 0 && isDBResultSeparator(line)) {
			style = StyleMuted
		}
		if abs == 0 && focus {
			style = StyleHealthy
		}
		lines = append(lines, style.Render(display))
	}
	title := "RESULT"
	if a.dbResultRows > 0 {
		title = fmt.Sprintf("RESULT (%d)", a.dbResultRows)
	}
	if a.dbResultHScroll > 0 {
		title += fmt.Sprintf(" · ←%d", a.dbResultHScroll)
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func isDBResultSeparator(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '-' && r != '+' && r != '|' && r != '=' && r != ' ' {
			return false
		}
	}
	return true
}

func (a *App) updateDbFilter(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.dbFilterOn = false
		a.dbFilterInput = ""
		a.dbFilter = ""
		a.syncDbTableCursor()
		return a, a.refreshDbSchema(p)
	case "enter":
		a.dbFilterOn = false
		a.dbFilter = strings.TrimSpace(a.dbFilterInput)
		a.dbFilterInput = ""
		a.dbTableCursor = 0
		a.syncDbTableCursor()
		return a, a.refreshDbSchema(p)
	case "backspace":
		if len(a.dbFilterInput) > 0 {
			r := []rune(a.dbFilterInput)
			a.dbFilterInput = string(r[:len(r)-1])
		}
		a.dbFilter = strings.TrimSpace(a.dbFilterInput)
		a.dbTableCursor = 0
		a.syncDbTableCursor()
	default:
		if len(msg.String()) == 1 {
			a.dbFilterInput += msg.String()
			a.dbFilter = strings.TrimSpace(a.dbFilterInput)
			a.dbTableCursor = 0
			a.syncDbTableCursor()
		}
	}
	return a, nil
}

func (a *App) handleDbKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.dbFilterOn {
		return a.updateDbFilter(msg, p)
	}
	if a.dbEditing {
		return a.updateDbEdit(msg, p)
	}
	switch msg.String() {
	case "esc":
		if a.dbFilter != "" {
			a.dbFilter = ""
			a.dbFilterInput = ""
			a.syncDbTableCursor()
			return a, a.refreshDbSchema(p)
		}
		return a, a.leaveDbTab()
	case "b":
		a.dbFilterOn = true
		a.dbFilterInput = a.dbFilter
		return a, nil
	case "d":
		return a, a.refreshDbSchema(p)
	case "tab":
		a.dbPane = dbPane((int(a.dbPane) + 1) % 3)
	case "shift+tab":
		i := int(a.dbPane) - 1
		if i < 0 {
			i = 2
		}
		a.dbPane = dbPane(i)
	case "[":
		if len(a.dbTargets) > 1 {
			a.dbTargetIdx = (a.dbTargetIdx - 1 + len(a.dbTargets)) % len(a.dbTargets)
			return a, a.refreshDbTables(p)
		}
	case "]":
		if len(a.dbTargets) > 1 {
			a.dbTargetIdx = (a.dbTargetIdx + 1) % len(a.dbTargets)
			return a, a.refreshDbTables(p)
		}
	case "left", "h":
		if a.dbPane == dbPaneResult {
			a.dbResultHScroll -= 8
			if a.dbResultHScroll < 0 {
				a.dbResultHScroll = 0
			}
			return a, nil
		}
		if len(a.dbTargets) > 1 {
			a.dbTargetIdx = (a.dbTargetIdx - 1 + len(a.dbTargets)) % len(a.dbTargets)
			return a, a.refreshDbTables(p)
		}
	case "right", "l":
		if a.dbPane == dbPaneResult {
			a.dbResultHScroll += 8
			return a, nil
		}
		if len(a.dbTargets) > 1 {
			a.dbTargetIdx = (a.dbTargetIdx + 1) % len(a.dbTargets)
			return a, a.refreshDbTables(p)
		}
	case "r":
		return a, a.refreshDbTables(p)
	case "e":
		a.dbPane = dbPaneQuery
		a.dbEditing = true
		a.dbEditorCursor = len([]rune(a.dbSQL))
	case "ctrl+enter":
		return a, a.runDbQuery(p)
	case "enter":
		if a.dbPane == dbPaneTables {
			if table, ok := a.selectedDbTable(); ok {
				a.dbSQL = fmt.Sprintf("SELECT * FROM %s LIMIT 50;", quoteSQLIdent(table))
				a.dbEditorCursor = len([]rune(a.dbSQL))
				return a, tea.Batch(a.runDbQuery(p), a.refreshDbSchema(p))
			}
		}
		if a.dbPane == dbPaneQuery {
			return a, a.runDbQuery(p)
		}
	case "up", "k":
		switch a.dbPane {
		case dbPaneTables:
			if a.dbTableCursor > 0 {
				a.dbTableCursor--
				return a, a.refreshDbSchema(p)
			}
		case dbPaneResult:
			if a.dbResultScroll > 0 {
				a.dbResultScroll--
			}
		}
	case "down", "j":
		switch a.dbPane {
		case dbPaneTables:
			vis := a.filteredDbTables()
			if a.dbTableCursor < len(vis)-1 {
				a.dbTableCursor++
				return a, a.refreshDbSchema(p)
			}
		case dbPaneResult:
			a.dbResultScroll++
		}
	case "pgup":
		a.dbResultScroll -= 10
		if a.dbResultScroll < 0 {
			a.dbResultScroll = 0
		}
	case "pgdown":
		a.dbResultScroll += 10
	}
	return a, nil
}

func (a *App) updateDbEdit(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	runes := []rune(a.dbSQL)
	cursor := a.dbEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	switch msg.String() {
	case "esc":
		a.dbEditing = false
		return a, nil
	case "ctrl+enter":
		a.dbEditing = false
		a.dbSQL = string(runes)
		return a, a.runDbQuery(p)
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
	a.dbSQL = string(runes)
	a.dbEditorCursor = cursor
	return a, nil
}

func quoteSQLIdent(name string) string {
	if strings.ContainsAny(name, " \"'`") {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return name
}
