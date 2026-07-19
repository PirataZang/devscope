package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/core"
)

// Focus panes — Tab cycles Conn → Tables → Query → Result.
type dbBlock int

const (
	dbBlockInfo   dbBlock = iota // Conn / credentials
	dbBlockSchema                // tables + columns
	dbBlockQuery                 // SQL editor
	dbBlockResult                // query output
)

// Kept for compatibility with older right-tab checks in tests/state.
type dbRightTab int

const (
	dbRightQuery dbRightTab = iota
	dbRightResult
)

// Conn field rows (↑↓ inside Conn pane).
const (
	dbFieldTarget = iota
	dbFieldHost
	dbFieldPort
	dbFieldDatabase
	dbFieldUser
	dbFieldPass
)

type dbHistoryItem struct {
	Engine string
	Query  string
}

func (a *App) enterDbTab(_ *core.Project) {
	a.tab = TabDB
	a.tabCursor = 0
	a.dbOpen = false
	a.dbEditing = false
}

func (a *App) openDbClient(p *core.Project) tea.Cmd {
	a.dbOpen = true
	if p != nil {
		a.initDbTab(p)
		return a.refreshDbSchema()
	}
	return nil
}

func (a *App) leaveDbTab() tea.Cmd {
	a.dbOpen = false
	a.dbEditing = false
	a.dbAuthEditPass = false
	a.dbClearSel()
	a.dbSearchOn = false
	a.tab = TabDB
	a.tabCursor = 0
	return nil
}

func (a *App) initDbTab(p *core.Project) {
	a.dbTargets = discoverDBTargets(p)
	if a.dbTargetCursor < 0 || a.dbTargetCursor >= len(a.dbTargets) {
		a.dbTargetCursor = 0
	}
	a.dbConnField = dbFieldTarget
	a.dbEditorCursor = 0
	a.dbEditorAnchor = -1
	a.dbEditorScroll = 0
	a.dbResultScroll = 0
	a.dbHScroll = 0
	a.dbColHScroll = 0
	a.dbSearchOn = false
	a.dbSearchQuery = ""
	a.dbSearchIdx = 0
	a.resetDbSchema()
	a.applyDbTarget(a.dbTargetCursor, p, false)
	if strings.TrimSpace(a.dbQuery) == "" && len(a.dbTargets) > 0 {
		a.dbQuery = a.dbTargets[a.dbTargetCursor].Engine.DefaultQuery()
	}
	// Start ready to write SQL.
	a.focusDbQuery()
}

// syncDbRightTab keeps legacy dbRightTab in sync with dbBlock.
func (a *App) syncDbRightTab() {
	switch a.dbBlock {
	case dbBlockQuery:
		a.dbRightTab = dbRightQuery
	case dbBlockResult:
		a.dbRightTab = dbRightResult
	}
}

func (a *App) resetDbSchema() {
	a.dbTables = nil
	a.dbTableCursor = 0
	a.dbTableScroll = 0
	a.dbColumns = nil
	a.dbColumnScroll = 0
	a.dbColHScroll = 0
	a.dbSchemaLoading = false
	a.dbSchemaErr = ""
	a.dbSchemaTable = ""
	a.dbColumnsLoading = false
}

func (a *App) applyDbTarget(idx int, p *core.Project, forceDefaults bool) {
	if idx < 0 || idx >= len(a.dbTargets) {
		return
	}
	t := a.dbTargets[idx]
	a.dbTargetCursor = idx
	a.dbEngine = t.Engine
	a.dbContainer = t.Container
	if forceDefaults || strings.TrimSpace(a.dbHost) == "" {
		a.dbHost = t.Host
	}
	if forceDefaults || a.dbPort == 0 {
		a.dbPort = t.Port
	}
	if forceDefaults || strings.TrimSpace(a.dbUser) == "" {
		a.dbUser = t.User
	}
	if forceDefaults || strings.TrimSpace(a.dbDatabase) == "" {
		a.dbDatabase = t.Database
	}
	if forceDefaults {
		a.dbPassword = t.Password
	}

	// Prefer container env (POSTGRES_*, MYSQL_*, …) then project .env files.
	if t.Container != "" {
		user, pass, database, host, port := dbSuggestFromContainerEnv(t.Container)
		if forceDefaults || strings.TrimSpace(a.dbUser) == "" || a.dbUser == t.Engine.DefaultUser() {
			if user != "" {
				a.dbUser = user
			}
		}
		if forceDefaults || strings.TrimSpace(a.dbPassword) == "" {
			if pass != "" {
				a.dbPassword = pass
			}
		}
		if forceDefaults || strings.TrimSpace(a.dbDatabase) == "" || a.dbDatabase == t.Engine.DefaultDatabase() {
			if database != "" {
				a.dbDatabase = database
			}
		}
		if host != "" && (forceDefaults || strings.TrimSpace(a.dbHost) == "" || a.dbHost == "127.0.0.1") {
			// docker exec ignores host; keep 127.0.0.1 for display
			_ = host
		}
		if port > 0 && (forceDefaults || a.dbPort == 0 || a.dbPort == t.Engine.DefaultPort()) {
			a.dbPort = port
		}
	}
	if p != nil {
		user, pass, database, host, port := dbSuggestFromEnvFiles(p.Path)
		if strings.TrimSpace(a.dbUser) == "" && user != "" {
			a.dbUser = user
		}
		if strings.TrimSpace(a.dbPassword) == "" && pass != "" {
			a.dbPassword = pass
		}
		if strings.TrimSpace(a.dbDatabase) == "" && database != "" {
			a.dbDatabase = database
		}
		if t.Container == "" {
			if strings.TrimSpace(a.dbHost) == "" && host != "" {
				a.dbHost = host
			}
			if a.dbPort == 0 && port > 0 {
				a.dbPort = port
			}
		}
	}
	if a.dbPort <= 0 {
		a.dbPort = t.Engine.DefaultPort()
	}
	if strings.TrimSpace(a.dbHost) == "" {
		a.dbHost = "127.0.0.1"
	}
	a.resetDbSchema()
}

func (a *App) dbRequest() dbRequest {
	return dbRequest{
		Engine:    a.dbEngine,
		Host:      a.dbHost,
		Port:      a.dbPort,
		User:      a.dbUser,
		Password:  a.dbPassword,
		Database:  a.dbDatabase,
		Container: a.dbContainer,
		Query:     a.dbQuery,
	}
}

func (a *App) refreshDbSchema() tea.Cmd {
	if a.dbSchemaLoading {
		return nil
	}
	a.dbSchemaLoading = true
	a.dbSchemaErr = ""
	a.dbTables = nil
	a.dbColumns = nil
	a.dbSchemaTable = ""
	return sendDBSchema(a.dbRequest())
}

func (a *App) loadDbColumnsForCursor() tea.Cmd {
	if a.dbTableCursor < 0 || a.dbTableCursor >= len(a.dbTables) {
		a.dbColumns = nil
		a.dbSchemaTable = ""
		return nil
	}
	table := a.dbTables[a.dbTableCursor]
	if table == a.dbSchemaTable && len(a.dbColumns) > 0 && !a.dbColumnsLoading {
		return nil
	}
	a.dbColumnsLoading = true
	a.dbSchemaTable = table
	a.dbColumns = nil
	a.dbColumnScroll = 0
	a.dbColHScroll = 0
	return sendDBColumns(a.dbRequest(), table)
}

func (a *App) handleDbSchema(msg dbSchemaMsg) tea.Cmd {
	a.dbSchemaLoading = false
	if msg.err != nil {
		a.dbSchemaErr = msg.err.Error()
		a.dbTables = nil
		a.dbColumns = nil
		return nil
	}
	a.dbSchemaErr = ""
	a.dbTables = msg.tables
	if a.dbTableCursor >= len(a.dbTables) {
		a.dbTableCursor = 0
	}
	if len(a.dbTables) == 0 {
		a.dbColumns = nil
		a.dbSchemaTable = ""
		return nil
	}
	return a.loadDbColumnsForCursor()
}

func (a *App) handleDbColumns(msg dbColumnsMsg) {
	a.dbColumnsLoading = false
	if msg.table != "" && msg.table != a.dbSchemaTable {
		return
	}
	if msg.err != nil {
		a.dbColumns = nil
		if a.dbSchemaErr == "" {
			a.dbSchemaErr = msg.err.Error()
		}
		return
	}
	a.dbColumns = msg.columns
}

func (a *App) renderDbLanding(p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabDB)).Bold(true)
	targets := discoverDBTargets(p)
	lines := []string{
		accent.Render("⬡  Database"),
		StyleMuted.Render("cliente SQL/NoSQL no contexto do projeto"),
		"",
		StyleSection.Render("ABRIR"),
		StyleNormal.Render("  pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para entrar"),
		StyleMuted.Render("  esc no client volta para esta aba"),
	}
	if len(targets) > 0 {
		lines = append(lines, "", StyleSection.Render("TARGETS"))
		n := minInt(6, len(targets))
		for i := 0; i < n; i++ {
			t := targets[i]
			mode := "host"
			if t.Container != "" {
				mode = "docker"
			}
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  %-10s %-8s %s", t.Engine.String(), mode, truncate(t.Label, 28))))
		}
	}
	if len(a.dbHistory) > 0 {
		lines = append(lines, "", StyleSection.Render("HISTÓRICO"))
		n := minInt(4, len(a.dbHistory))
		for i := 0; i < n; i++ {
			h := a.dbHistory[i]
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  %-8s %s", h.Engine, truncate(h.Query, 36))))
		}
	}
	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) dbSidebarWidth() int {
	if a.width <= 0 {
		return 28
	}
	w := a.width * 32 / 100
	if w < 22 {
		return 22
	}
	if w > 40 {
		return 40
	}
	return w
}

func (a *App) renderDbTab(p *core.Project) string {
	height := maxInt(14, a.height-2)
	panelW := maxInt(20, a.width)
	innerW := maxInt(16, panelW-2)
	chrome := a.renderDbChrome(innerW)
	chromeH := lipgloss.Height(chrome)
	bodyHeight := maxInt(8, height-chromeH-2)
	sideW := a.dbSidebarWidth()
	if sideW+28 > innerW {
		sideW = maxInt(20, innerW/3)
	}
	mainW := maxInt(24, innerW-sideW)

	// Query smaller on top; result gets most of the space for reading data.
	queryH := maxInt(6, bodyHeight*35/100)
	if queryH > bodyHeight-6 {
		queryH = maxInt(5, bodyHeight-6)
	}
	resultH := maxInt(5, bodyHeight-queryH)

	left := a.renderDbLeftColumn(p, sideW, bodyHeight)
	queryBox := a.renderDbQueryBox(mainW, queryH)
	resultBox := a.renderDbResultBox(mainW, resultH)
	right := lipgloss.JoinVertical(lipgloss.Left, queryBox, resultBox)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	footer := a.renderDbFooterLine(innerW)

	content := chrome + "\n" + body + "\n" + footer
	panel := clampRenderedHeight(content, height)

	engine := a.dbEngine.String()
	return lipgloss.JoinVertical(lipgloss.Left,
		panel,
		a.renderStatusBar("db · "+engine),
	)
}

func (a *App) renderDbChrome(width int) string {
	engine := a.dbEngine.String()
	brand := StyleSection.Render("⬡ DB")
	engineBadge := lipgloss.NewStyle().Foreground(ColorDocker).Bold(true).Render(" " + engine + " ")

	meta := StyleMuted.Render("pronto")
	if a.dbLoading {
		meta = StyleWarning.Render("● rodando…")
	} else if a.dbResultErr != "" {
		meta = StyleUnhealthy.Render("● erro")
	} else if a.dbResultBody != "" {
		hint := a.dbResultHint
		if hint == "" {
			hint = "ok"
		}
		meta = StyleHealthy.Render("● "+hint) + "  " +
			StyleMuted.Render(a.dbResultTime.Round(time.Millisecond).String())
	}

	conn := a.dbConnSummary()
	// Focus breadcrumb so the user always knows where they are.
	focus := a.dbFocusLabel()
	line1 := lipgloss.JoinHorizontal(lipgloss.Top, brand, "  ", engineBadge, "  ", meta, "  ", StyleMuted.Render("· "+focus))
	line2 := StyleMuted.Render("↗ ") + StyleNormal.Render(truncate(conn, maxInt(10, width-4)))
	sep := StyleMuted.Render(strings.Repeat("─", maxInt(8, width)))
	return truncate(line1, width) + "\n" + truncate(line2, width) + "\n" + sep
}

func (a *App) dbFocusLabel() string {
	switch a.dbBlock {
	case dbBlockInfo:
		return "Conn"
	case dbBlockSchema:
		return "Tables"
	case dbBlockQuery:
		if a.dbEditing {
			return "Query*"
		}
		return "Query"
	case dbBlockResult:
		return "Result"
	default:
		return "Query"
	}
}

func (a *App) dbConnSummary() string {
	if a.dbContainer != "" {
		return fmt.Sprintf("docker://%s  %s@%s", truncate(a.dbContainer, 24), a.dbUser, a.dbDatabase)
	}
	return fmt.Sprintf("%s:%d  %s@%s", a.dbHost, a.dbPort, a.dbUser, a.dbDatabase)
}

func (a *App) renderDbFooterLine(width int) string {
	return StyleMuted.Render(truncate(a.dbFooter(), width))
}

func (a *App) renderDbLeftColumn(p *core.Project, width, height int) string {
	_ = p
	infoH := maxInt(7, height*40/100)
	if infoH > height-5 {
		infoH = maxInt(6, height-5)
	}
	schemaH := maxInt(5, height-infoH)

	info := renderApiTitledBox("Conn", a.renderDbInfoBlockLines(width-2, infoH-2), width, infoH, a.dbBlock == dbBlockInfo)
	schema := renderApiTitledBox("Tables", a.renderDbSchemaBlockLines(width-2, schemaH-2), width, schemaH, a.dbBlock == dbBlockSchema)
	return lipgloss.JoinVertical(lipgloss.Left, info, schema)
}

func (a *App) renderDbQueryBox(width, height int) string {
	title := "Query"
	if a.dbBlock == dbBlockQuery {
		if a.dbEditing {
			title = "Query  · editando"
		} else {
			title = "Query  · foco"
		}
	}
	title += "  (ctrl+enter roda)"
	lines := a.renderDbQueryContent(height-2, width-2)
	return renderApiTitledBox(title, lines, width, height, a.dbBlock == dbBlockQuery)
}

func (a *App) renderDbResultBox(width, height int) string {
	title := "Result"
	if a.dbBlock == dbBlockResult {
		title = "Result  · foco"
	}
	if a.dbLoading {
		title += "  · rodando…"
	} else if a.dbResultErr != "" {
		title += "  · erro"
	} else if a.dbResultHint != "" {
		title += "  · " + a.dbResultHint
	}
	if a.dbHScroll > 0 {
		title += fmt.Sprintf("  →%d", a.dbHScroll)
	}
	lines := a.renderDbResultPanel(height-2, width-2)
	return renderApiTitledBox(title, lines, width, height, a.dbBlock == dbBlockResult)
}

func (a *App) dbInfoRows() []int {
	if a.dbContainer != "" {
		return []int{dbFieldTarget, dbFieldDatabase, dbFieldUser, dbFieldPass}
	}
	return []int{dbFieldTarget, dbFieldHost, dbFieldPort, dbFieldDatabase, dbFieldUser, dbFieldPass}
}

func (a *App) dbInfoRowIndex(field int) int {
	rows := a.dbInfoRows()
	for i, r := range rows {
		if r == field {
			return i
		}
	}
	return 0
}

func (a *App) dbClampInfoField() {
	rows := a.dbInfoRows()
	if len(rows) == 0 {
		a.dbConnField = dbFieldTarget
		return
	}
	for _, r := range rows {
		if r == a.dbConnField {
			return
		}
	}
	a.dbConnField = rows[0]
}

func (a *App) dbMoveInfoField(delta int) {
	rows := a.dbInfoRows()
	if len(rows) == 0 {
		return
	}
	idx := a.dbInfoRowIndex(a.dbConnField)
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	a.dbConnField = rows[idx]
	a.dbEditing = false
	a.dbClearSel()
}

func (a *App) renderDbInfoBlockLines(width, height int) []string {
	a.dbClampInfoField()
	editing := a.dbEditing && a.dbBlock == dbBlockInfo
	field := a.dbConnField
	focusedBlock := a.dbBlock == dbBlockInfo
	lines := make([]string, 0, height)

	// Target row
	if len(a.dbTargets) == 0 {
		line := StyleMuted.Render("tgt  nenhum")
		if focusedBlock && field == dbFieldTarget {
			line = StyleSelected.Render(truncate("▶ tgt  nenhum", width))
		}
		lines = append(lines, line)
	} else {
		t := a.dbTargets[a.dbTargetCursor]
		mode := "H"
		if t.Container != "" {
			mode = "D"
		}
		label := fmt.Sprintf("tgt  %s [%s] %s", t.Engine.String(), mode, t.Label)
		if len(a.dbTargets) > 1 {
			label = fmt.Sprintf("tgt  %s [%s] %s  (%d/%d ←→)", t.Engine.String(), mode, t.Label, a.dbTargetCursor+1, len(a.dbTargets))
		}
		switch {
		case focusedBlock && field == dbFieldTarget:
			lines = append(lines, StyleSelected.Render(truncate("▶ "+label, width)))
		case focusedBlock:
			lines = append(lines, StyleTabActive.Render(truncate("● "+label, width)))
		default:
			lines = append(lines, StyleMuted.Render(truncate("  "+label, width)))
		}
	}

	hostLine := a.dbHost
	portLine := strconv.Itoa(a.dbPort)
	dbLine := a.dbDatabase
	userLine := a.dbUser
	passShow := apiMaskSecret(a.dbPassword, maxInt(4, width-6))
	if a.dbContainer != "" {
		// show container as read-only hint under target when docker
		if height > 6 {
			lines = append(lines, StyleMuted.Render(truncate("ctr  "+a.dbContainer, width)))
		}
	}
	if strings.TrimSpace(userLine) == "" {
		userLine = "(vazio)"
	}
	if strings.TrimSpace(a.dbPassword) == "" {
		passShow = "(vazio)"
	}

	renderField := func(name, value string, idx int) string {
		onRow := focusedBlock && field == idx
		if editing && field == idx {
			editVal := value
			if idx == dbFieldPass {
				editVal = strings.Repeat("•", len([]rune(a.dbPassword)))
			}
			// empty placeholder while editing blank password/user
			if idx == dbFieldPass && a.dbPassword == "" {
				editVal = ""
			}
			if idx == dbFieldUser && a.dbUser == "" {
				editVal = ""
			}
			selLo, selHi, _ := a.dbSelRange()
			view := fitApiFieldWindowSel(editVal, a.dbEditorCursor, maxInt(4, width-len(name)-2), true, selLo, selHi)
			return StyleMuted.Render(name+" ") + StyleSelected.Render(padRightVisible(view, maxInt(4, width-len(name)-1)))
		}
		if onRow {
			return StyleSelected.Render(truncate("▶ "+name+" "+value, width))
		}
		if focusedBlock {
			return StyleNormal.Render(truncate("  "+name+" "+value, width))
		}
		return StyleMuted.Render(truncate("  "+name+" "+value, width))
	}

	if a.dbContainer != "" {
		lines = append(lines,
			renderField("db  ", dbLine, dbFieldDatabase),
			renderField("user", userLine, dbFieldUser),
			renderField("pass", passShow, dbFieldPass),
		)
	} else {
		lines = append(lines,
			renderField("host", hostLine, dbFieldHost),
			renderField("port", portLine, dbFieldPort),
			renderField("db  ", dbLine, dbFieldDatabase),
			renderField("user", userLine, dbFieldUser),
			renderField("pass", passShow, dbFieldPass),
		)
	}
	if height >= len(lines)+1 {
		lines = append(lines, StyleMuted.Render("↑↓  ←→ target  enter edita"))
	}
	return fitExactLines(lines, height)
}

func (a *App) renderDbSchemaBlockLines(width, height int) []string {
	if a.dbSchemaLoading {
		return fitExactLines([]string{StyleMuted.Render("carregando…")}, height)
	}
	if a.dbSchemaErr != "" && len(a.dbTables) == 0 {
		errLines := wrapAPIErrorLines(a.dbSchemaErr, width)
		hint := StyleMuted.Render("s recarregar")
		return fitExactLines(append(errLines, "", hint), height)
	}
	if len(a.dbTables) == 0 {
		return fitExactLines([]string{
			StyleMuted.Render("sem tabelas"),
			StyleMuted.Render("s carregar"),
			StyleMuted.Render("enter = SELECT *"),
		}, height)
	}

	// Tables top, columns bottom.
	tableH := maxInt(2, height*45/100)
	if tableH > height-3 {
		tableH = maxInt(2, height-3)
	}
	colH := maxInt(2, height-tableH)

	a.dbTableScroll = clampScroll(a.dbTableScroll, tableH, len(a.dbTables))
	a.dbTableScroll = ensureVisible(a.dbTableCursor, a.dbTableScroll, tableH, len(a.dbTables))

	lines := make([]string, 0, height)
	start := a.dbTableScroll
	end := minInt(start+tableH, len(a.dbTables))
	for i := start; i < end; i++ {
		name := a.dbTables[i]
		selected := i == a.dbTableCursor
		switch {
		case selected && a.dbBlock == dbBlockSchema:
			lines = append(lines, StyleSelected.Render(truncate("▶ "+name, width)))
		case selected:
			lines = append(lines, StyleTabActive.Render(truncate("● "+name, width)))
		default:
			lines = append(lines, StyleMuted.Render(truncate("  "+name, width)))
		}
	}
	for len(lines) < tableH {
		lines = append(lines, "")
	}

	colTitle := "cols"
	if a.dbSchemaTable != "" {
		colTitle = a.dbSchemaTable
	}
	if a.dbColHScroll > 0 {
		colTitle += fmt.Sprintf(" →%d", a.dbColHScroll)
	}
	lines = append(lines, StyleSection.Render(truncate(colTitle, width)))

	avail := maxInt(1, colH-1)
	if a.dbColumnsLoading {
		lines = append(lines, StyleMuted.Render("  …"))
	} else if len(a.dbColumns) == 0 {
		lines = append(lines, StyleMuted.Render("  —"))
	} else {
		a.dbColumnScroll = clampScroll(a.dbColumnScroll, avail, len(a.dbColumns))
		cStart := a.dbColumnScroll
		cEnd := minInt(cStart+avail, len(a.dbColumns))
		for _, col := range a.dbColumns[cStart:cEnd] {
			raw := col.Name
			if col.Type != "" {
				raw = fmt.Sprintf("%s  %s", col.Name, col.Type)
			}
			display := sliceColumns(raw, a.dbColHScroll, maxInt(6, width-2))
			lines = append(lines, StyleNormal.Render("  "+display))
		}
	}
	return fitExactLines(lines, height)
}

func (a *App) renderDbQueryContent(height, width int) []string {
	innerW := maxInt(8, width-2)
	// Always show editor when Query is focused.
	if a.dbBlock == dbBlockQuery {
		if !a.dbEditing {
			// auto-arm editor so cursor is visible
			a.dbEditing = true
			if a.dbEditorCursor < 0 || a.dbEditorCursor > len([]rune(a.dbQuery)) {
				a.dbEditorCursor = len([]rune(a.dbQuery))
			}
		}
		return a.renderDbMultilineEdit(a.dbQuery, innerW, height)
	}
	if strings.TrimSpace(a.dbQuery) == "" {
		return fitExactLines([]string{StyleMuted.Render("SQL…  tab aqui  ctrl+enter roda")}, height)
	}
	raw := strings.Split(a.dbQuery, "\n")
	a.dbEditorScroll = clampScroll(a.dbEditorScroll, height, len(raw))
	start := a.dbEditorScroll
	end := minInt(start+height, len(raw))
	lines := make([]string, 0, height)
	for _, line := range raw[start:end] {
		display := sliceColumns(sanitizeTerminalLine(line), 0, innerW)
		lines = append(lines, StyleMuted.Render(display))
	}
	return fitExactLines(lines, height)
}

func (a *App) renderDbMultilineEdit(content string, width, height int) []string {
	// Reuse API multiline editor path by temporarily mapping fields.
	// Standalone copy to avoid coupling edit state names.
	runes := []rune(content)
	cursor := a.dbEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	selLo, selHi, hasSel := a.dbSelRange()

	type lineSpan struct{ start, end int }
	var spans []lineSpan
	start := 0
	for i, r := range runes {
		if r == '\n' {
			spans = append(spans, lineSpan{start, i})
			start = i + 1
		}
	}
	spans = append(spans, lineSpan{start, len(runes)})

	cursorLine := 0
	for i, sp := range spans {
		if cursor <= sp.end {
			cursorLine = i
			break
		}
		if i == len(spans)-1 {
			cursorLine = i
		}
	}
	a.dbEditorScroll = ensureVisible(cursorLine, a.dbEditorScroll, height, len(spans))
	from := a.dbEditorScroll
	to := minInt(from+height, len(spans))

	out := make([]string, 0, height)
	for _, sp := range spans[from:to] {
		var b strings.Builder
		if cursor == sp.start && cursor == sp.end {
			b.WriteRune('█')
		}
		for i := sp.start; i < sp.end; i++ {
			if i == cursor {
				b.WriteRune('█')
			}
			s := string(runes[i])
			if hasSel && i >= selLo && i < selHi {
				s = StyleApiSel.Render(s)
			}
			b.WriteString(s)
		}
		if cursor == sp.end && (sp.end == len(runes) || (sp.end < len(runes) && runes[sp.end] == '\n')) {
			if sp.start != sp.end {
				b.WriteRune('█')
			}
		}
		out = append(out, ansi.Truncate(b.String(), width, "…"))
	}
	return fitExactLines(out, height)
}

func (a *App) renderDbResultPanel(viewport, width int) []string {
	if a.dbLoading {
		return fitExactLines([]string{StyleMuted.Render("Executando query...")}, viewport)
	}
	if a.dbResultErr != "" {
		return fitExactLines(wrapAPIErrorLines(a.dbResultErr, width), viewport)
	}
	if a.dbResultBody == "" {
		return fitExactLines([]string{
			StyleSection.Render("Sem resultado ainda"),
			StyleMuted.Render("1. confira Conn (user/pass)"),
			StyleMuted.Render("2. escreva SQL em Query"),
			StyleMuted.Render("3. ctrl+enter para rodar"),
			"",
			StyleMuted.Render("Tables: enter = SELECT * LIMIT 100"),
		}, viewport)
	}

	header := []string{
		StyleHealthy.Render("ok") + "  " + StyleMuted.Render(a.dbResultTime.Round(time.Millisecond).String()),
	}
	if a.dbResultHint != "" {
		header[0] += "  " + StyleMuted.Render(a.dbResultHint)
	}
	if a.dbSearchQuery != "" {
		matches := a.dbSearchMatches()
		if len(matches) == 0 {
			header = append(header, StyleMuted.Render("/0"))
		} else {
			header = append(header, StyleAccent.Render(fmt.Sprintf("/%d/%d", a.dbSearchIdx+1, len(matches))))
		}
	}

	bodyLines := strings.Split(a.dbResultBody, "\n")
	avail := maxInt(1, viewport-len(header))
	a.dbResultScroll = clampScroll(a.dbResultScroll, avail, len(bodyLines))
	start := a.dbResultScroll
	end := minInt(start+avail, len(bodyLines))
	matchSet := a.dbMatchLineSet()
	current := -1
	if matches := a.dbSearchMatches(); len(matches) > 0 {
		current = matches[a.dbSearchIdx]
	}

	out := append([]string{}, header...)
	for i := start; i < end; i++ {
		display := sliceColumns(sanitizeTerminalLine(bodyLines[i]), a.dbHScroll, maxInt(8, width-2))
		switch {
		case i == current:
			out = append(out, StyleDiffMatch.Render(display))
		case matchSet[i]:
			out = append(out, StyleWarning.Render(display))
		default:
			out = append(out, StyleNormal.Render(display))
		}
	}
	return fitExactLines(out, viewport)
}

func (a *App) dbFooter() string {
	// One clear line per mode — keep short.
	if a.dbEditing && a.dbBlock == dbBlockInfo {
		return "editando Conn  enter/esc ok  tab próximo campo"
	}
	switch a.dbBlock {
	case dbBlockInfo:
		return "Conn  ↑↓ campo  ←→ target  enter edita  tab Tables  s schema  esc sair"
	case dbBlockSchema:
		return "Tables  ↑↓  enter SELECT*  s reload  tab Query  ,/. scroll  esc sair"
	case dbBlockQuery:
		return "Query  digite SQL  ctrl+enter roda  tab Result  esc Conn"
	case dbBlockResult:
		base := "Result  ↑↓ scroll  ,/. lado  / busca  tab Conn  q Query  esc sair"
		if a.dbSearchQuery != "" {
			base = "Result  n/p match  esc limpa busca"
		}
		return base
	default:
		return "tab navega  ctrl+enter roda  esc sair"
	}
}

func (a *App) dbCyclePane(forward bool) {
	order := []dbBlock{dbBlockInfo, dbBlockSchema, dbBlockQuery, dbBlockResult}
	idx := 0
	for i, b := range order {
		if b == a.dbBlock {
			idx = i
			break
		}
	}
	if forward {
		a.dbBlock = order[(idx+1)%len(order)]
	} else {
		a.dbBlock = order[(idx-1+len(order))%len(order)]
	}
	a.dbEditing = false
	a.dbClearSel()
	a.syncDbRightTab()
	if a.dbBlock == dbBlockQuery {
		a.focusDbQuery()
	}
	if a.dbBlock == dbBlockInfo {
		a.dbClampInfoField()
	}
}

func (a *App) dbClearSel() {
	a.dbEditorAnchor = -1
}

func (a *App) dbSelRange() (lo, hi int, ok bool) {
	if !a.dbEditing || a.dbEditorAnchor < 0 {
		return 0, 0, false
	}
	lo, hi = a.dbEditorAnchor, a.dbEditorCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return 0, 0, false
	}
	return lo, hi, true
}

func (a *App) dbDeleteSelection() bool {
	lo, hi, ok := a.dbSelRange()
	if !ok {
		return false
	}
	runes := []rune(a.dbCurrentEditText())
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	runes = append(runes[:lo], runes[hi:]...)
	a.dbSetCurrentEditText(string(runes))
	a.dbEditorCursor = lo
	a.dbClearSel()
	return true
}

func (a *App) dbCurrentEditText() string {
	switch a.dbBlock {
	case dbBlockInfo:
		switch a.dbConnField {
		case dbFieldHost:
			return a.dbHost
		case dbFieldPort:
			return strconv.Itoa(a.dbPort)
		case dbFieldDatabase:
			return a.dbDatabase
		case dbFieldUser:
			return a.dbUser
		case dbFieldPass:
			return a.dbPassword
		}
	case dbBlockQuery:
		return a.dbQuery
	}
	return ""
}

func (a *App) dbSetCurrentEditText(text string) {
	switch a.dbBlock {
	case dbBlockInfo:
		switch a.dbConnField {
		case dbFieldHost:
			a.dbHost = text
		case dbFieldPort:
			text = strings.TrimSpace(text)
			if text == "" {
				a.dbPort = 0
				return
			}
			if n, err := strconv.Atoi(text); err == nil {
				a.dbPort = n
			}
		case dbFieldDatabase:
			a.dbDatabase = text
		case dbFieldUser:
			a.dbUser = text
		case dbFieldPass:
			a.dbPassword = text
		}
	case dbBlockQuery:
		a.dbQuery = text
	}
}

func (a *App) beginDbEdit() {
	a.dbClearSel()
	switch a.dbBlock {
	case dbBlockInfo:
		a.dbClampInfoField()
		if a.dbConnField == dbFieldTarget {
			return
		}
		a.dbEditing = true
		a.dbAuthEditPass = a.dbConnField == dbFieldPass
		a.dbEditorCursor = len([]rune(a.dbCurrentEditText()))
	case dbBlockQuery:
		a.focusDbQuery()
	}
}

// focusDbQuery puts the cursor in the SQL editor ready to type.
func (a *App) focusDbQuery() {
	a.dbBlock = dbBlockQuery
	a.dbRightTab = dbRightQuery
	a.dbEditing = true
	a.dbHScroll = 0
	a.dbEditorScroll = 0
	a.dbClearSel()
	a.dbEditorCursor = len([]rune(a.dbQuery))
}

func (a *App) focusDbResult() {
	a.dbBlock = dbBlockResult
	a.dbRightTab = dbRightResult
	a.dbEditing = false
	a.dbClearSel()
}

func (a *App) pushDbHistory(engine, query string) {
	item := dbHistoryItem{Engine: engine, Query: strings.TrimSpace(query)}
	if item.Query == "" {
		return
	}
	out := []dbHistoryItem{item}
	for _, h := range a.dbHistory {
		if h.Engine == item.Engine && h.Query == item.Query {
			continue
		}
		out = append(out, h)
		if len(out) >= 10 {
			break
		}
	}
	a.dbHistory = out
}

func (a *App) sendDbQuery() tea.Cmd {
	if a.dbLoading {
		return nil
	}
	a.dbLoading = true
	a.dbResultErr = ""
	a.focusDbResult()
	a.dbResultScroll = 0
	a.dbHScroll = 0
	a.pushDbHistory(a.dbEngine.String(), a.dbQuery)
	return sendDBQuery(a.dbRequest())
}

func (a *App) handleDbResult(msg dbResultMsg) tea.Cmd {
	a.dbLoading = false
	if msg.err != nil {
		a.dbResultErr = msg.err.Error()
		a.dbResultBody = ""
		a.dbResultHint = ""
		a.dbResultTime = msg.duration
		return nil
	}
	a.dbResultErr = ""
	a.dbResultBody = msg.body
	a.dbResultHint = msg.rowsHint
	a.dbResultTime = msg.duration
	a.dbResultScroll = 0
	a.dbHScroll = 0
	// Auto-load schema after first successful query if still empty.
	if len(a.dbTables) == 0 && !a.dbSchemaLoading {
		return a.refreshDbSchema()
	}
	return nil
}

func (a *App) dbSearchMatches() []int {
	q := strings.ToLower(strings.TrimSpace(a.dbSearchQuery))
	if q == "" {
		return nil
	}
	var matches []int
	for i, line := range strings.Split(a.dbResultBody, "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (a *App) dbMatchLineSet() map[int]bool {
	matches := a.dbSearchMatches()
	if len(matches) == 0 {
		return nil
	}
	set := make(map[int]bool, len(matches))
	for _, i := range matches {
		set[i] = true
	}
	return set
}

func (a *App) jumpDbSearch(delta int) {
	matches := a.dbSearchMatches()
	if len(matches) == 0 {
		return
	}
	a.dbSearchIdx = (a.dbSearchIdx + delta) % len(matches)
	if a.dbSearchIdx < 0 {
		a.dbSearchIdx += len(matches)
	}
	a.focusDbResult()
	a.dbResultScroll = ensureVisible(matches[a.dbSearchIdx], a.dbResultScroll, maxInt(1, a.dbViewport()-3), len(strings.Split(a.dbResultBody, "\n")))
}

func (a *App) dbViewport() int {
	return maxInt(4, maxInt(14, a.height-2)-5)
}

func (a *App) updateDbEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	text := a.dbCurrentEditText()
	runes := []rune(text)
	cursor := a.dbEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	editingQuery := a.dbBlock == dbBlockQuery
	editingInfo := a.dbBlock == dbBlockInfo
	multiline := editingQuery
	key := msg.String()

	move := func(next int, extend bool) {
		if next < 0 {
			next = 0
		}
		if next > len(runes) {
			next = len(runes)
		}
		if extend {
			if a.dbEditorAnchor < 0 {
				a.dbEditorAnchor = cursor
			}
		} else {
			a.dbClearSel()
		}
		cursor = next
	}

	switch key {
	case "esc":
		if editingInfo {
			a.dbEditing = false
			a.dbAuthEditPass = false
			a.dbClearSel()
			return a, nil
		}
		// Query (or anything else): leave client.
		a.dbEditing = false
		a.dbClearSel()
		return a, a.leaveDbTab()
	case "ctrl+a":
		if len(runes) == 0 {
			a.dbClearSel()
			return a, nil
		}
		a.dbEditorAnchor = 0
		cursor = len(runes)
		a.dbEditorCursor = cursor
		return a, nil
	case "enter":
		if editingInfo {
			a.dbEditing = false
			a.dbAuthEditPass = false
			a.dbClearSel()
			return a, nil
		}
		// SQL: Enter = nova linha (natural). Rodar = ctrl+enter.
		if editingQuery {
			if a.dbDeleteSelection() {
				text = a.dbCurrentEditText()
				runes = []rune(text)
				cursor = a.dbEditorCursor
			}
			indent := apiLineIndent(runes, cursor)
			insert := []rune("\n" + indent)
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			cursor += len(insert)
			a.dbSetCurrentEditText(string(runes))
			a.dbEditorCursor = cursor
			a.dbClearSel()
			return a, nil
		}
		return a, nil
	case "ctrl+enter":
		return a, a.sendDbQuery()
	case "left":
		if lo, _, ok := a.dbSelRange(); ok {
			move(lo, false)
		} else {
			move(cursor-1, false)
		}
	case "right":
		if _, hi, ok := a.dbSelRange(); ok {
			move(hi, false)
		} else {
			move(cursor+1, false)
		}
	case "shift+left":
		move(cursor-1, true)
	case "shift+right":
		move(cursor+1, true)
	case "up":
		if multiline {
			move(apiMoveLine(runes, cursor, -1), false)
		} else if editingInfo {
			a.dbEditing = false
			a.dbClearSel()
			a.dbMoveInfoField(-1)
			return a, nil
		}
	case "down":
		if multiline {
			move(apiMoveLine(runes, cursor, 1), false)
		} else if editingInfo {
			a.dbEditing = false
			a.dbClearSel()
			a.dbMoveInfoField(1)
			return a, nil
		}
	case "shift+up":
		if multiline {
			move(apiMoveLine(runes, cursor, -1), true)
		}
	case "shift+down":
		if multiline {
			move(apiMoveLine(runes, cursor, 1), true)
		}
	case "home":
		move(apiLineStart(runes, cursor), false)
	case "end":
		move(apiLineEnd(runes, cursor), false)
	case "shift+home":
		move(apiLineStart(runes, cursor), true)
	case "shift+end":
		move(apiLineEnd(runes, cursor), true)
	case "backspace":
		if a.dbDeleteSelection() {
			return a, nil
		}
		if cursor > 0 {
			runes = append(runes[:cursor-1], runes[cursor:]...)
			cursor--
			a.dbSetCurrentEditText(string(runes))
		}
	case "delete":
		if a.dbDeleteSelection() {
			return a, nil
		}
		if cursor < len(runes) {
			runes = append(runes[:cursor], runes[cursor+1:]...)
			a.dbSetCurrentEditText(string(runes))
		}
	case "tab":
		// Always cycle panes — never indent with tab (use 2 spaces via typing).
		a.dbEditing = false
		a.dbClearSel()
		a.dbCyclePane(true)
		return a, nil
	case "shift+tab":
		a.dbEditing = false
		a.dbClearSel()
		a.dbCyclePane(false)
		return a, nil
	default:
		var inserted []rune
		if len(msg.Runes) > 0 {
			inserted = msg.Runes
		} else if s := key; len(s) == 1 {
			inserted = []rune(s)
		}
		if len(inserted) > 0 {
			if a.dbDeleteSelection() {
				text = a.dbCurrentEditText()
				runes = []rune(text)
				cursor = a.dbEditorCursor
			}
			if a.dbBlock == dbBlockInfo && a.dbConnField == dbFieldPort {
				filtered := make([]rune, 0, len(inserted))
				for _, r := range inserted {
					if r >= '0' && r <= '9' {
						filtered = append(filtered, r)
					}
				}
				inserted = filtered
			}
			if len(inserted) > 0 {
				runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
				cursor += len(inserted)
				a.dbSetCurrentEditText(string(runes))
				a.dbClearSel()
			}
		}
	}
	a.dbEditorCursor = cursor
	return a, nil
}

func (a *App) updateDbSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.dbSearchOn = false
		return a, nil
	case tea.KeyEnter:
		a.dbSearchQuery = strings.TrimSpace(a.dbSearchInput)
		a.dbSearchIdx = 0
		a.dbSearchOn = false
		if a.dbSearchQuery != "" {
			a.jumpDbSearch(0)
		}
		return a, nil
	case tea.KeyBackspace:
		if a.dbSearchInput != "" {
			r := []rune(a.dbSearchInput)
			a.dbSearchInput = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		a.dbSearchInput += string(msg.Runes)
	}
	return a, nil
}

func (a *App) renderDbSearchPrompt() string {
	content := a.renderDbTab(a.currentProject())
	prompt := StylePanel.Render("Buscar no result: " + a.dbSearchInput + "█")
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		prompt,
		a.renderStatusBar("enter buscar | esc cancelar"),
	)
}

func (a *App) handleDbKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.dbSearchOn {
		return a.updateDbSearch(msg)
	}
	// Query pane is always in edit mode when focused.
	if a.dbBlock == dbBlockQuery {
		if !a.dbEditing {
			a.focusDbQuery()
		}
		return a.updateDbEdit(msg)
	}
	if a.dbEditing {
		return a.updateDbEdit(msg)
	}

	key := msg.String()

	// Conn field: typing starts edit.
	if a.dbBlock == dbBlockInfo && a.dbConnField != dbFieldTarget {
		if a.dbShouldStartFieldEdit(msg) {
			a.beginDbEdit()
			return a.updateDbEdit(msg)
		}
	}

	switch key {
	case "esc":
		if a.dbSearchQuery != "" {
			a.dbSearchQuery = ""
			a.dbSearchIdx = 0
			return a, nil
		}
		return a, a.leaveDbTab()
	case "tab":
		a.dbCyclePane(true)
	case "shift+tab":
		a.dbCyclePane(false)
	case "1":
		a.dbEditing = false
		a.dbBlock = dbBlockInfo
		a.dbClampInfoField()
	case "2":
		a.dbEditing = false
		a.dbBlock = dbBlockSchema
	case "3", "q":
		a.focusDbQuery()
	case "4":
		a.focusDbResult()
	case "s":
		return a, a.refreshDbSchema()
	case "left":
		if a.dbBlock == dbBlockInfo && a.dbConnField == dbFieldTarget {
			return a, a.dbCycleTarget(p, -1)
		}
		if a.dbBlock == dbBlockResult && a.dbHScroll > 0 {
			a.dbHScroll -= 8
			if a.dbHScroll < 0 {
				a.dbHScroll = 0
			}
			return a, nil
		}
		if a.dbBlock == dbBlockSchema && a.dbColHScroll > 0 {
			a.dbColHScroll -= 8
			if a.dbColHScroll < 0 {
				a.dbColHScroll = 0
			}
			return a, nil
		}
	case "right":
		if a.dbBlock == dbBlockInfo && a.dbConnField == dbFieldTarget {
			return a, a.dbCycleTarget(p, 1)
		}
		if a.dbBlock == dbBlockResult {
			a.dbHScroll += 8
			return a, nil
		}
		if a.dbBlock == dbBlockSchema {
			a.dbColHScroll += 8
			return a, nil
		}
	case "enter":
		switch a.dbBlock {
		case dbBlockInfo:
			if a.dbConnField == dbFieldTarget {
				return a, a.refreshDbSchema()
			}
			a.beginDbEdit()
			return a, nil
		case dbBlockSchema:
			if a.dbTableCursor >= 0 && a.dbTableCursor < len(a.dbTables) {
				a.dbQuery = a.dbSelectAllQuery(a.dbTables[a.dbTableCursor])
				return a, a.sendDbQuery()
			}
		case dbBlockResult:
			return a, a.sendDbQuery()
		}
	case "r":
		return a, a.sendDbQuery()
	case "up":
		switch a.dbBlock {
		case dbBlockInfo:
			a.dbMoveInfoField(-1)
		case dbBlockSchema:
			if a.dbTableCursor > 0 {
				a.dbTableCursor--
				return a, a.loadDbColumnsForCursor()
			}
		case dbBlockResult:
			if a.dbResultScroll > 0 {
				a.dbResultScroll--
			}
		}
	case "down":
		switch a.dbBlock {
		case dbBlockInfo:
			a.dbMoveInfoField(1)
		case dbBlockSchema:
			if a.dbTableCursor < len(a.dbTables)-1 {
				a.dbTableCursor++
				return a, a.loadDbColumnsForCursor()
			}
		case dbBlockResult:
			a.dbResultScroll++
		}
	case "pgup":
		if a.dbBlock == dbBlockResult {
			a.dbResultScroll -= a.dbViewport()
			if a.dbResultScroll < 0 {
				a.dbResultScroll = 0
			}
		} else if a.dbBlock == dbBlockSchema {
			a.dbColumnScroll -= 5
			if a.dbColumnScroll < 0 {
				a.dbColumnScroll = 0
			}
		}
	case "pgdown":
		if a.dbBlock == dbBlockResult {
			a.dbResultScroll += a.dbViewport()
		} else if a.dbBlock == dbBlockSchema {
			a.dbColumnScroll += 5
		}
	case ",":
		if a.dbBlock == dbBlockResult && a.dbHScroll > 0 {
			a.dbHScroll -= 8
			if a.dbHScroll < 0 {
				a.dbHScroll = 0
			}
		} else if a.dbBlock == dbBlockSchema && a.dbColHScroll > 0 {
			a.dbColHScroll -= 8
			if a.dbColHScroll < 0 {
				a.dbColHScroll = 0
			}
		}
	case ".":
		if a.dbBlock == dbBlockResult {
			a.dbHScroll += 8
		} else if a.dbBlock == dbBlockSchema {
			a.dbColHScroll += 8
		}
	case "/":
		if a.dbBlock == dbBlockResult {
			a.dbSearchOn = true
			a.dbSearchInput = a.dbSearchQuery
		}
	case "n", "N":
		if a.dbBlock == dbBlockResult && a.dbSearchQuery != "" {
			a.jumpDbSearch(1)
		}
	case "p", "P":
		if a.dbBlock == dbBlockResult && a.dbSearchQuery != "" {
			a.jumpDbSearch(-1)
		}
	}
	return a, nil
}

func (a *App) dbCycleTarget(p *core.Project, delta int) tea.Cmd {
	if len(a.dbTargets) == 0 {
		return nil
	}
	next := a.dbTargetCursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(a.dbTargets) {
		next = len(a.dbTargets) - 1
	}
	if next == a.dbTargetCursor {
		return nil
	}
	a.dbTargetCursor = next
	a.applyDbTarget(a.dbTargetCursor, p, true)
	a.dbQuery = a.dbEngine.DefaultQuery()
	return a.refreshDbSchema()
}

func (a *App) dbShouldStartQueryEdit(msg tea.KeyMsg) bool {
	// On Query, almost everything printable starts free SQL edit.
	// Keep only structural navigation out of the editor.
	switch msg.String() {
	case "esc", "tab", "shift+tab", "ctrl+enter", "enter",
		"up", "down", "left", "right", "pgup", "pgdown",
		"[", "]", "ctrl+c", "ctrl+d":
		return false
	}
	if len(msg.Runes) > 0 {
		return true
	}
	s := msg.String()
	if len(s) == 1 {
		return true
	}
	if s == "backspace" || s == "delete" || s == "space" {
		return true
	}
	return false
}

func (a *App) dbShouldStartFieldEdit(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "tab", "shift+tab", "enter", "up", "down", "left", "right",
		"pgup", "pgdown", "1", "2", "3", "s", "r", "e", "h", "j", "k", "l",
		"ctrl+c", "ctrl+d", "ctrl+enter":
		return false
	}
	if len(msg.Runes) > 0 {
		return true
	}
	s := msg.String()
	if len(s) == 1 {
		return true
	}
	if s == "backspace" || s == "delete" {
		return true
	}
	return false
}

func (a *App) dbSelectAllQuery(table string) string {
	safe := strings.ReplaceAll(table, `"`, `""`)
	switch a.dbEngine {
	case dbEnginePostgres:
		return fmt.Sprintf(`SELECT * FROM "%s" LIMIT 100;`, safe)
	case dbEngineMySQL:
		safe = strings.ReplaceAll(table, "`", "``")
		return fmt.Sprintf("SELECT * FROM `%s` LIMIT 100;", safe)
	case dbEngineMongo:
		return fmt.Sprintf("db.getCollection('%s').find().limit(20).toArray()", strings.ReplaceAll(table, "'", "\\'"))
	case dbEngineRedis:
		return "GET " + table
	default:
		return "SELECT 1;"
	}
}

func isDefaultDbQuery(q string) bool {
	q = strings.TrimSpace(q)
	for _, e := range dbEngines {
		if q == e.DefaultQuery() {
			return true
		}
	}
	return false
}
