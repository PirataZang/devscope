package ui

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Tarefas: os comandos que o projeto já declara — Makefile, justfile,
// Taskfile, scripts do package.json e do composer.json — numa lista só.
//
// A tarefa roda no terminal de verdade (tea.ExecProcess), não numa caixa de
// log: `npm run dev` é servidor que fica no ar, pede ctrl+c, pinta cor e às
// vezes faz pergunta. Quando termina, a tela segura a saída até o enter, e a
// lista guarda o código e o tempo da última execução.
//
// Sem faixa de abertura (§1.5): a lista é o conteúdo, como em Worktrees.

type tasksLoadedMsg struct {
	path string
	list []collectors.ProjectTask
}

type taskDoneMsg struct {
	key  string
	code int
	dur  time.Duration
}

type taskResult struct {
	code int
	dur  time.Duration
}

func (a *App) loadTasks(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	a.taskLoading = true
	return func() tea.Msg {
		return tasksLoadedMsg{path: path, list: collectors.ProjectTasks(path)}
	}
}

func (a *App) handleTasksLoaded(msg tasksLoadedMsg) {
	if p := a.currentProject(); p == nil || !pathsMatch(p.Path, msg.path) {
		return
	}
	if a.taskPath != msg.path {
		a.taskPath, a.taskLast, a.taskCursor = msg.path, nil, 0
	}
	a.taskLoading = false
	a.taskList = msg.list
	a.taskCursor = clampCursor(a.taskCursor, len(a.taskList))
}

func (a *App) handleTaskDone(msg taskDoneMsg) {
	if a.taskLast == nil {
		a.taskLast = map[string]taskResult{}
	}
	a.taskLast[msg.key] = taskResult{code: msg.code, dur: msg.dur}
}

func (a *App) runTask(p *core.Project, t collectors.ProjectTask) tea.Cmd {
	cmd := collectors.TaskCommand(p.Path, t)
	key, start := t.Key(), time.Now()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		code := 0
		switch {
		case cmd.ProcessState != nil:
			code = cmd.ProcessState.ExitCode() // -1 quando morreu por sinal
		case err != nil:
			code = -1
		}
		return taskDoneMsg{key: key, code: code, dur: time.Since(start)}
	})
}

func (a *App) handleTaskKeys(msg tea.KeyMsg, p *core.Project) (tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		a.taskCursor = clampCursor(a.taskCursor-1, len(a.taskList))
	case "down", "j":
		a.taskCursor = clampCursor(a.taskCursor+1, len(a.taskList))
	case "enter":
		if a.taskCursor < len(a.taskList) {
			return a.runTask(p, a.taskList[a.taskCursor]), true
		}
	case "r":
		return a.loadTasks(p), true
	default:
		return nil, false
	}
	return nil, true
}

// ─── tela ────────────────────────────────────────────────────────────────────

func (a *App) renderTasksTab(p *core.Project) string {
	w, h := a.moduleSize()
	head := []string{a.renderModuleContext(p, w, "Tarefas", "")}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"enter", "rodar"},
		[2]string{"r", "atualizar"},
		[2]string{"shift+E", "terminal"},
	))
	room := maxInt(4, h-len(head)-lipgloss.Height(cmdBar))

	detail := a.taskDetail(w)
	if a.projectTiny() || len(detail)+5 > room {
		detail = nil
	}
	listH := maxInt(3, minInt(room-len(detail), len(a.taskList)+2))
	parts := append(head, a.renderTaskList(w, listH))
	parts = append(parts, detail...)
	stack := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar)
}

func (a *App) renderTaskList(width, height int) string {
	viewport := maxInt(1, height-2)
	title := panelTitle("TAREFAS", countIf(len(a.taskList)))
	switch {
	case a.taskLoading && len(a.taskList) == 0:
		return panelBox(title, fitExactLines([]string{a.loadingText("lendo tarefas…")}, viewport), width, height, true)
	case len(a.taskList) == 0:
		msg := "nenhuma tarefa — Makefile, justfile, Taskfile, package.json e composer.json são lidos daqui"
		return panelBox(title, fitExactLines([]string{StyleMuted.Render(truncate(msg, width-4))}, viewport), width, height, true)
	}

	a.taskScroll = ensureVisible(a.taskCursor, a.taskScroll, viewport, len(a.taskList))
	if below := len(a.taskList) - a.taskScroll - viewport; below > 0 {
		title = panelTitle("TAREFAS", countIf(len(a.taskList)), "↓"+strconv.Itoa(below))
	}

	// Colunas pelo conteúdo mais longo, com teto (§11); a descrição leva o
	// resto e é a primeira a encolher — o comando inteiro está no detalhe.
	inner := maxInt(20, width-4)
	srcW, nameW, lastW := 3, 4, 0
	for _, t := range a.taskList {
		srcW = maxInt(srcW, lipgloss.Width(t.Source))
		nameW = maxInt(nameW, lipgloss.Width(t.Name))
		if r, ok := a.taskLast[t.Key()]; ok {
			lastW = maxInt(lastW, lipgloss.Width(taskResultLabel(r).text))
		}
	}
	nameW = minInt(nameW, 28)
	descW := maxInt(0, inner-2-1-srcW-nameW-lastW-4)

	lines := make([]string, 0, viewport)
	end := minInt(len(a.taskList), a.taskScroll+viewport)
	for i := a.taskScroll; i < end; i++ {
		t := a.taskList[i]
		glyph, glyphSt := "·", StyleMuted
		last := stateLabel{}
		if r, ok := a.taskLast[t.Key()]; ok {
			last = taskResultLabel(r)
			glyph, glyphSt = taskResultGlyph(r.code), last.style
		}
		selected := i == a.taskCursor
		row := renderCells(selected, []dashCell{
			{text: glyph, width: 1, style: glyphSt},
			{text: t.Source, width: srcW, style: StyleMuted},
			{text: t.Name, width: nameW, style: lipgloss.NewStyle().Foreground(ColorAccent)},
			{text: t.Desc, width: descW, style: StyleMuted},
			{text: last.text, width: lastW, style: last.style, right: true},
		})
		if selected {
			row = StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	return panelBox(title, fitExactLines(lines, viewport), width, height, true)
}

func (a *App) taskDetail(width int) []string {
	if a.taskCursor >= len(a.taskList) {
		return nil
	}
	t := a.taskList[a.taskCursor]
	out := []string{
		rule(width),
		factLine("roda", []string{StyleNormal.Render(strings.Join(t.Argv, " "))}, width),
	}
	if t.Desc != "" {
		out = append(out, factLine("faz", []string{StyleMuted.Render(t.Desc)}, width))
	}
	if r, ok := a.taskLast[t.Key()]; ok {
		l := taskResultLabel(r)
		out = append(out, factLine("última", []string{l.style.Render(l.text)}, width))
	}
	return out
}

// taskResultLabel: "ok · 3s" ou "saiu 2 · 12s". -1 é a tarefa morta por sinal
// (ctrl+c num servidor) — não é falha do código, é parada.
func taskResultLabel(r taskResult) stateLabel {
	dur := formatTaskDur(r.dur)
	switch r.code {
	case 0:
		return stateLabel{"ok · " + dur, StyleHealthy}
	case -1, 130:
		return stateLabel{"parada · " + dur, StyleWarning}
	}
	return stateLabel{"saiu " + strconv.Itoa(r.code) + " · " + dur, StyleUnhealthy}
}

// taskResultGlyph: ok, parada e falha com glifos distintos — a tela lê sem
// cor (§1).
func taskResultGlyph(code int) string {
	switch code {
	case 0:
		return "◆"
	case -1, 130:
		return "◌"
	}
	return "⚠"
}

func formatTaskDur(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())) + "s"
	}
	return formatUptime(d)
}

func countIf(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}
