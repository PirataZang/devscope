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

func (a *App) enterDbTab(_ *core.Project) {
	a.tab = TabDatabase
	a.tabCursor = 0
	a.dbOpen = false
	a.dbEditing = false
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
	a.dbErr = ""
	a.dbLoading = false
	a.dbTargets = collectors.DetectProjectDatabases(p)
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
	a.tab = TabDatabase
	a.tabCursor = 0
	return nil
}

func (a *App) renderDbLanding(p *core.Project) string {
	w, h := a.moduleSize()
	targets := collectors.DetectProjectDatabases(p)
	status := fmt.Sprintf("%d detectado(s)", len(targets))
	ctx := a.renderModuleContext(p, w, "Database", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(5, bodyH*28/100)
	listH := maxInt(6, bodyH-openH)
	openLines := append([]string{StyleMuted.Render("tabelas e SQL no contexto do projeto")}, moduleOpenHint()...)
	listLines := make([]string, 0, listH-2)
	if len(targets) == 0 {
		listLines = append(listLines,
			StyleMuted.Render("nenhum Postgres/MySQL nos containers"),
			StyleMuted.Render("suba o compose com um serviço db"),
		)
	} else {
		for _, t := range targets {
			listLines = append(listLines, fmt.Sprintf("%s %s  %s",
				StyleIconDocker.Render("●"),
				StyleNormal.Render(t.Label),
				StyleMuted.Render(string(t.Engine)+" · "+t.Database)))
		}
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DATABASE", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("DETECTADOS", fitExactLines(listLines, listH-2), centerW, listH, false),
	)
	details := []string{
		StyleMuted.Render("Targets ") + StyleNormal.Render(fmt.Sprintf("%d", len(targets))),
	}
	if len(targets) > 0 {
		t := targets[0]
		details = append(details,
			StyleMuted.Render("Engine  ") + StyleNormal.Render(string(t.Engine)),
			StyleMuted.Render("DB      ") + StyleNormal.Render(truncate(t.Database, rightW-10)),
			StyleMuted.Render("Label   ") + StyleMuted.Render(truncate(t.Label, rightW-10)),
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
		if a.dbTableCursor >= len(a.dbTables) {
			a.dbTableCursor = maxInt(0, len(a.dbTables)-1)
		}
	case dbQueryMsg:
		a.dbLoading = false
		a.dbResultScroll = 0
		a.dbResultHScroll = 0
		if m.err != "" {
			a.dbErr = m.err
			a.dbResult = m.out
			return a, nil
		}
		a.dbErr = ""
		a.dbResult = m.out
	}
	return a, nil
}

func (a *App) renderDbTab(p *core.Project) string {
	w := maxInt(60, a.width)
	h := maxInt(18, a.height-2)
	leftW := maxInt(22, w/3)
	rightW := maxInt(30, w-leftW-1)
	queryH := maxInt(6, h/3)
	resultH := maxInt(6, h-queryH-4)

	header := a.renderDbHeader()
	left := a.renderDbLeft(leftW, h-lipgloss.Height(header)-2)
	query := a.renderDbQueryPane(rightW, queryH)
	result := a.renderDbResultPane(rightW, resultH)
	right := lipgloss.JoinVertical(lipgloss.Left, query, result)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	hints := "tab painel  ↑↓ tabelas  enter preview  e SQL  ctrl+enter run  [] banco  ←→ scroll  esc abas"
	if a.dbPane == dbPaneResult && !a.dbEditing {
		hints = "↑↓ scroll  ←→ lateral  pgup/pgdn  tab painel  esc abas"
	}
	if a.dbEditing {
		hints = "editando SQL  ctrl+enter run  esc sair"
	}
	if a.dbLoading {
		hints = "carregando...  " + hints
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		a.renderStatusBar(hints),
	)
}

func (a *App) renderDbHeader() string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabDatabase)).Bold(true)
	t, ok := a.currentDbTarget()
	meta := StyleMuted.Render("nenhum target")
	if ok {
		meta = accent.Render(string(t.Engine)) + StyleMuted.Render(" · "+t.Label+" · "+t.User+"@"+t.Database)
		if len(a.dbTargets) > 1 {
			meta += StyleMuted.Render(fmt.Sprintf("  [%d/%d]", a.dbTargetIdx+1, len(a.dbTargets)))
		}
	}
	err := ""
	if a.dbErr != "" {
		err = "  " + StyleUnhealthy.Render(truncate(a.dbErr, 40))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		accent.Render("Database"),
		"  ",
		meta,
		err,
	)
}

func (a *App) renderDbLeft(width, height int) string {
	focus := a.dbPane == dbPaneTables && !a.dbEditing
	title := "[tables]"
	lines := make([]string, 0, height)
	if len(a.dbTables) == 0 {
		if a.dbLoading {
			lines = append(lines, StyleMuted.Render("carregando..."))
		} else {
			lines = append(lines, StyleMuted.Render("(sem tabelas)"))
		}
	} else {
		a.dbTablesScroll = ensureVisible(a.dbTableCursor, a.dbTablesScroll, height-2, len(a.dbTables))
		start := a.dbTablesScroll
		end := minInt(start+height-2, len(a.dbTables))
		for i := start; i < end; i++ {
			name := a.dbTables[i]
			prefix := "  "
			style := StyleMuted
			if i == a.dbTableCursor {
				prefix = "> "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			lines = append(lines, style.Render(truncate(prefix+name, width-2)))
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
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
	lines := make([]string, 0, height-2)
	for _, line := range raw {
		if len(lines) >= height-2 {
			break
		}
		style := StyleNormal
		if focus {
			style = StyleSelected
		}
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), width-2)))
	}
	return renderApiTitledBox("[sql]", fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderDbResultPane(width, height int) string {
	focus := a.dbPane == dbPaneResult && !a.dbEditing
	body := a.dbResult
	if a.dbLoading && body == "" {
		body = "executando..."
	}
	if a.dbErr != "" && body == "" {
		body = a.dbErr
	}
	if strings.TrimSpace(body) == "" {
		body = "enter numa tabela ou ctrl+enter no SQL"
	}
	raw := strings.Split(body, "\n")
	innerW := maxInt(8, width-2)
	if a.dbResultHScroll < 0 {
		a.dbResultHScroll = 0
	}
	a.dbResultScroll = clampScroll(a.dbResultScroll, height-2, len(raw))
	start := a.dbResultScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		style := StyleMuted
		if focus {
			style = StyleNormal
		}
		display := sliceColumns(sanitizeTerminalLine(line), a.dbResultHScroll, innerW)
		lines = append(lines, style.Render(display))
	}
	title := "[result]"
	if a.dbResultHScroll > 0 {
		title = fmt.Sprintf("[result · ←%d]", a.dbResultHScroll)
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) handleDbKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.dbEditing {
		return a.updateDbEdit(msg, p)
	}
	switch msg.String() {
	case "esc":
		return a, a.leaveDbTab()
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
		if a.dbPane == dbPaneTables && a.dbTableCursor < len(a.dbTables) {
			table := a.dbTables[a.dbTableCursor]
			a.dbSQL = fmt.Sprintf("SELECT * FROM %s LIMIT 50;", quoteSQLIdent(table))
			a.dbEditorCursor = len([]rune(a.dbSQL))
			return a, a.runDbQuery(p)
		}
		if a.dbPane == dbPaneQuery {
			return a, a.runDbQuery(p)
		}
	case "up", "k":
		switch a.dbPane {
		case dbPaneTables:
			if a.dbTableCursor > 0 {
				a.dbTableCursor--
			}
		case dbPaneResult:
			if a.dbResultScroll > 0 {
				a.dbResultScroll--
			}
		}
	case "down", "j":
		switch a.dbPane {
		case dbPaneTables:
			if a.dbTableCursor < len(a.dbTables)-1 {
				a.dbTableCursor++
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
