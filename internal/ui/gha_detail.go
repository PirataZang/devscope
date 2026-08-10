package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) ghaOpenProcessDetail() tea.Cmd {
	return a.ghaOpenProcessDetailAt(ghaProcTabOverview)
}

func (a *App) ghaOpenProcessDetailAt(tab ghaProcTab) tea.Cmd {
	name, file := a.ghaSelectedProcessRef()
	if name == "" {
		a.ghaStatus = "selecione um processo / run / workflow"
		return nil
	}
	a.ghaScreen = ghaScrProcess
	a.ghaProcTab = tab
	a.ghaProcName = name
	a.ghaProcFile = file
	a.ghaProcScroll = 0
	a.ghaProcRunCursor = 0
	a.ghaProcJobs = ""
	a.ghaProcLogs = ""
	a.ghaProcYAML = ""
	live := a.ghaStatusForProcess(name, file)
	a.ghaProcRunID = live.RunID
	return a.ghaLoadProcessDetail()
}

func (a *App) ghaSelectedProcessRef() (name, file string) {
	switch a.ghaKind {
	case ghaKindProcesses:
		if a.ghaCursor < len(a.ghaProcesses) {
			p := a.ghaProcesses[a.ghaCursor]
			return p.Name, p.File
		}
	case ghaKindWorkflows:
		if a.ghaCursor < len(a.ghaWorkflows) {
			w := a.ghaWorkflows[a.ghaCursor]
			return sanitizeGHAName(w.Name), w.Path
		}
	case ghaKindRuns:
		if r, ok := a.ghaSelectedRun(); ok {
			return sanitizeGHAName(r.Workflow), ""
		}
	}
	return "", ""
}

func sanitizeGHAName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func (a *App) ghaMatchRun(processName, file string, r collectors.GHARun) bool {
	pn := strings.ToLower(processName)
	wf := strings.ToLower(r.Workflow)
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	if pn == "" {
		return false
	}
	if strings.EqualFold(r.Workflow, processName) {
		return true
	}
	if strings.Contains(wf, pn) || strings.Contains(pn, wf) {
		return true
	}
	if base != "" && (strings.Contains(wf, base) || strings.Contains(base, strings.ReplaceAll(wf, " ", "-"))) {
		return true
	}
	return false
}

func (a *App) ghaRunsForProcess(name, file string) []collectors.GHARun {
	var out []collectors.GHARun
	for _, r := range a.ghaRuns {
		if a.ghaMatchRun(name, file, r) {
			out = append(out, r)
		}
	}
	return out
}

func (a *App) ghaStatusForProcess(name, file string) ghaProcLive {
	live := ghaProcLive{Label: "idle"}
	if a.ghaTriggered != nil {
		if t, ok := a.ghaTriggered[name]; ok && time.Since(t) < 2*time.Minute {
			live.Label = "triggered"
			live.Event = "workflow_dispatch"
		}
	}
	runs := a.ghaRunsForProcess(name, file)
	if len(runs) == 0 {
		return live
	}
	r := runs[0] // newest first from gh
	live.RunID = r.ID
	live.Event = r.Event
	live.Status = r.Status
	live.Conclusion = r.Conclusion
	live.Title = firstNonEmpty(r.DisplayTitle, r.Name)
	live.CreatedAt = r.CreatedAt
	switch {
	case r.Status == "queued":
		live.Label = "queued"
	case r.Status == "in_progress" || r.Status == "waiting" || r.Status == "requested":
		live.Label = "running"
	case r.Conclusion == "success":
		if live.Label != "triggered" {
			live.Label = "success"
		}
	case r.Conclusion == "failure":
		if live.Label != "triggered" {
			live.Label = "failure"
		}
	case r.Conclusion == "cancelled" || r.Conclusion == "startup_failure":
		if live.Label != "triggered" {
			live.Label = "cancelled"
		}
	case r.Status == "completed" && r.Conclusion == "":
		live.Label = "idle"
	}
	// recent manual trigger wins briefly
	if a.ghaTriggered != nil {
		if t, ok := a.ghaTriggered[name]; ok && time.Since(t) < 45*time.Second {
			if live.Label == "idle" || live.Label == "success" || live.Label == "failure" {
				live.Label = "triggered"
				live.Event = "workflow_dispatch"
			}
		}
	}
	return live
}

func ghaLiveBadge(label string, frame int) string {
	switch label {
	case "running":
		return StyleWarning.Render(animSpinner(frame) + " running")
	case "queued":
		return StyleWarning.Render(animArc(frame) + " queued")
	case "triggered":
		return StyleAccent.Render(animSpinner(frame) + " triggered")
	case "success":
		return StyleHealthy.Render(animPulse(frame) + " success")
	case "failure":
		return StyleUnhealthy.Render("● failure")
	case "cancelled":
		return StyleMuted.Render("○ stopped")
	default:
		return StyleMuted.Render("○ idle")
	}
}

func (a *App) ghaLoadProcessDetail() tea.Cmd {
	gen := a.ghaGen
	path := a.ghaPath
	name := a.ghaProcName
	file := a.ghaProcFile
	runID := a.ghaProcRunID
	tab := a.ghaProcTab
	return func() tea.Msg {
		switch tab {
		case ghaProcTabYAML:
			body, err := collectors.GHAReadProcessFile(path, firstNonEmpty(file, name))
			e := ""
			if err != nil {
				e = err.Error()
			}
			return ghaDetailMsg{gen: gen, body: body, err: e}
		case ghaProcTabJobs:
			if runID == "" {
				return ghaDetailMsg{gen: gen, body: "nenhum run — selecione em Runs ou dispare com t"}
			}
			jobs, err := collectors.GHAListRunJobs(path, runID)
			if err != nil {
				return ghaDetailMsg{gen: gen, body: "", err: err.Error()}
			}
			return ghaDetailMsg{gen: gen, body: collectors.FormatGHAJobsText(jobs)}
		case ghaProcTabLogs:
			if runID == "" {
				return ghaDetailMsg{gen: gen, body: "nenhum run para este processo — use t para trigger"}
			}
			body, err := collectors.GHARunLogs(path, runID)
			e := ""
			if err != nil {
				e = err.Error()
			}
			return ghaDetailMsg{gen: gen, body: body, err: e}
		default:
			_ = name
			return ghaDetailMsg{gen: gen, body: ""}
		}
	}
}

func (a *App) handleGHAProcessKeys(msg tea.KeyMsg, _ *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.ghaScreen = ghaScrCluster
		a.ghaProcLogs = ""
		a.ghaProcYAML = ""
		return a, nil
	case "tab", "right", "]":
		a.ghaProcTab = ghaProcTab((int(a.ghaProcTab) + 1) % int(ghaProcTabCount))
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "shift+tab", "left", "[":
		a.ghaProcTab = ghaProcTab((int(a.ghaProcTab) + int(ghaProcTabCount) - 1) % int(ghaProcTabCount))
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "1":
		a.ghaProcTab = ghaProcTabOverview
		a.ghaProcScroll = 0
		return a, nil
	case "2":
		a.ghaProcTab = ghaProcTabRuns
		a.ghaProcScroll = 0
		return a, nil
	case "3":
		a.ghaProcTab = ghaProcTabJobs
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "4":
		a.ghaProcTab = ghaProcTabLogs
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "5":
		a.ghaProcTab = ghaProcTabYAML
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "up", "k":
		if a.ghaProcTab == ghaProcTabRuns {
			if a.ghaProcRunCursor > 0 {
				a.ghaProcRunCursor--
				runs := a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile)
				if a.ghaProcRunCursor < len(runs) {
					a.ghaProcRunID = runs[a.ghaProcRunCursor].ID
				}
			}
			return a, nil
		}
		a.ghaProcScroll = maxInt(0, a.ghaProcScroll-1)
	case "down", "j":
		if a.ghaProcTab == ghaProcTabRuns {
			runs := a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile)
			if a.ghaProcRunCursor < len(runs)-1 {
				a.ghaProcRunCursor++
				a.ghaProcRunID = runs[a.ghaProcRunCursor].ID
			}
			return a, nil
		}
		a.ghaProcScroll++
	case "pgup":
		a.ghaProcScroll = maxInt(0, a.ghaProcScroll-10)
	case "pgdown":
		a.ghaProcScroll += 10
	case "enter":
		if a.ghaProcTab == ghaProcTabRuns {
			a.ghaSyncSelectedProcRun()
			a.ghaProcTab = ghaProcTabJobs
			a.ghaProcScroll = 0
			return a, a.ghaLoadProcessDetail()
		}
	case "l":
		a.ghaSyncSelectedProcRun()
		a.ghaProcTab = ghaProcTabLogs
		a.ghaProcScroll = 0
		return a, a.ghaLoadProcessDetail()
	case "F":
		a.ghaSyncSelectedProcRun()
		return a, a.ghaShowFailedLogs()
	case "i":
		a.ghaSyncSelectedProcRun()
		return a, a.ghaBeginNoteEdit()
	case "t":
		if a.ghaNeedsSetup() {
			a.ghaShowSetup()
			return a, nil
		}
		return a, a.ghaTriggerCurrentProcess()
	case "R":
		a.ghaSyncSelectedProcRun()
		if a.ghaProcRunID == "" {
			a.ghaStatus = "nenhum run para re-run"
			return a, nil
		}
		return a, a.ghaRerunID(a.ghaProcRunID)
	case "s", "x":
		a.ghaSyncSelectedProcRun()
		return a, a.ghaAskStopJob()
	case "S":
		return a, a.ghaAskStopAllJobs()
	case "o", "O":
		return a, a.ghaOpenBrowserForProcess()
	case "r":
		return a, tea.Batch(a.refreshGHA(), a.ghaLoadProcessDetail())
	case "f":
		// refresh jobs/logs
		if a.ghaProcTab == ghaProcTabLogs || a.ghaProcTab == ghaProcTabJobs {
			return a, a.ghaLoadProcessDetail()
		}
	}
	return a, nil
}

// ghaSyncSelectedProcRun pins ghaProcRunID to the highlighted row on the Runs tab.
func (a *App) ghaSyncSelectedProcRun() {
	runs := a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile)
	if len(runs) == 0 {
		return
	}
	if a.ghaProcRunCursor < 0 {
		a.ghaProcRunCursor = 0
	}
	if a.ghaProcRunCursor >= len(runs) {
		a.ghaProcRunCursor = len(runs) - 1
	}
	a.ghaProcRunID = runs[a.ghaProcRunCursor].ID
}

func (a *App) ghaTriggerCurrentProcess() tea.Cmd {
	name := a.ghaProcName
	file := a.ghaProcFile
	wf := filepath.Base(file)
	if wf == "" || wf == "." {
		wf = name + ".yml"
	}
	return a.ghaBeginTrigger(name, file, wf, ghaScrProcess)
}

func (a *App) ghaRerunID(id string) tea.Cmd {
	gen := a.ghaGen
	path := a.ghaPath
	return func() tea.Msg {
		out, err := collectors.GHARerun(path, id)
		if err != nil {
			return ghaActionMsg{gen: gen, err: err.Error()}
		}
		return ghaActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "rerun "+id)}
	}
}

func (a *App) ghaOpenBrowserForProcess() tea.Cmd {
	a.ghaSyncSelectedProcRun()
	owner, repo := a.ghaResolveOwnerRepo()
	url := ""
	runID := a.ghaProcRunID
	if runID != "" {
		for _, r := range a.ghaRuns {
			if r.ID == runID {
				url = r.URL
				break
			}
		}
		// Prefer explicit run URL — workflow page often lands on an older completed run.
		if url == "" {
			url = collectors.GHARunURL(owner, repo, runID)
		}
	}
	if url == "" {
		url = collectors.GHAActionsURL(owner, repo, a.ghaProcFile)
	}
	if url == "" {
		a.ghaStatus = "sem URL"
		return nil
	}
	if err := collectors.GHAOpenRunURL(url); err != nil {
		a.ghaStatus = err.Error()
		return nil
	}
	a.ghaStatus = "browser · run #" + firstNonEmpty(runID, "?") + " · " + url
	return nil
}

func (a *App) renderGHAProcessDetail(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	live := a.ghaStatusForProcess(a.ghaProcName, a.ghaProcFile)
	if a.ghaProcTab == ghaProcTabRuns {
		a.ghaSyncSelectedProcRun()
	} else if a.ghaProcRunID == "" {
		a.ghaProcRunID = live.RunID
	}

	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabActions)).Bold(true)
	proj := "project"
	if p != nil && p.Name != "" {
		proj = p.Name
	}
	left := accent.Render("ACTION") + StyleMuted.Render(" › ") + StyleNormal.Render(truncate(a.ghaProcName, 28)) +
		StyleMuted.Render("  ·  ") + StyleMuted.Render(truncate(proj, 16))
	right := ghaLiveBadge(live.Label, a.animFrame)
	pad := w - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	header := left + strings.Repeat(" ", pad) + right

	meta := StyleMuted.Render(fmt.Sprintf("file %s", firstNonEmpty(a.ghaProcFile, "—")))
	if live.Event != "" {
		meta += StyleMuted.Render("  ·  event ") + StyleNormal.Render(live.Event)
	}
	if live.RunID != "" {
		meta += StyleMuted.Render("  ·  run #") + StyleNormal.Render(live.RunID)
	}

	tabs := a.renderGHAProcTabs(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(meta) + lipgloss.Height(tabs) + 2
	bodyH := maxInt(8, h-chromeH-2)
	cmdW := actionsCmdWidth(w)
	mainW := maxInt(36, w-cmdW)

	body := a.renderGHAProcTabBody(mainW, bodyH, live)
	side := renderActionsBox(cmdW, bodyH,
		[2]string{"[]", "abas"},
		[2]string{"t", "trigger"},
		[2]string{"3", "timeline"},
		[2]string{"s", "parar job"},
		[2]string{"S", "parar todos"},
		[2]string{"l", "logs"},
		[2]string{"F", "failed logs"},
		[2]string{"i", "incidente"},
		[2]string{"R", "re-run"},
		[2]string{"o", "github"},
		[2]string{"r", "refresh"},
		[2]string{"esc", "voltar"},
	)
	main := body
	if side != "" {
		main = lipgloss.JoinHorizontal(lipgloss.Top, body, side)
	}
	auto := fmt.Sprintf("auto %ds", int(a.ghaTickInterval()/time.Second))
	hints := "ACTION  [] abas  3 jobs  t trigger  s parar  l logs  ·  " + auto + "  esc"
	if a.ghaConfirm {
		hints = a.ghaConfirmHint()
	}
	if a.ghaStatus != "" {
		hints = truncate(a.ghaStatus+"  ·  "+hints, maxInt(40, w-4))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, meta, tabs, main, a.renderStatusBar(hints))
}

func (a *App) renderGHAProcTabs(width int) string {
	parts := make([]string, 0, int(ghaProcTabCount))
	for i := 0; i < int(ghaProcTabCount); i++ {
		t := ghaProcTab(i)
		label := fmt.Sprintf(" %d:%s ", i+1, t.String())
		if t == a.ghaProcTab {
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

func (a *App) renderGHAProcTabBody(width, height int, live ghaProcLive) string {
	switch a.ghaProcTab {
	case ghaProcTabOverview:
		return a.renderGHAProcOverview(width, height, live)
	case ghaProcTabRuns:
		return a.renderGHAProcRuns(width, height)
	case ghaProcTabJobs:
		return a.renderGHAProcJobs(width, height)
	case ghaProcTabLogs:
		return a.renderGHAProcLogs(width, height)
	case ghaProcTabYAML:
		return a.renderGHAProcYAML(width, height)
	}
	return ""
}

func (a *App) renderGHAProcJobs(width, height int) string {
	inner := maxInt(4, height-2)
	body := a.ghaProcJobs
	if body == "" {
		body = a.ghaDetail
	}
	if strings.TrimSpace(body) == "" {
		body = "carregando jobs…\n(selecione um run na aba Runs se estiver vazio)"
	}
	raw := strings.Split(body, "\n")
	a.ghaProcScroll = clampScroll(a.ghaProcScroll, inner, len(raw))
	end := minInt(a.ghaProcScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaProcScroll; i < end; i++ {
		ln := sanitizeTerminalLine(raw[i])
		trimmed := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(trimmed, "✗"):
			lines = append(lines, StyleUnhealthy.Render(truncate(ln, width-4)))
		case strings.HasPrefix(trimmed, "✓"):
			lines = append(lines, StyleHealthy.Render(truncate(ln, width-4)))
		case strings.HasPrefix(trimmed, "●"):
			lines = append(lines, StyleWarning.Render(truncate(ln, width-4)))
		default:
			lines = append(lines, StyleMuted.Render(truncate(ln, width-4)))
		}
	}
	title := "TIMELINE"
	if a.ghaProcRunID != "" {
		title += " · #" + a.ghaProcRunID
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderGHAProcOverview(width, height int, live ghaProcLive) string {
	inner := maxInt(4, height-2)
	procMin := collectors.GHAMinutesFromRuns(a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile))
	projMin := collectors.GHAMinutesFromRuns(a.ghaRuns)
	lines := []string{
		StyleMuted.Render("Process   ") + StyleNormal.Render(a.ghaProcName),
		StyleMuted.Render("File      ") + StyleNormal.Render(firstNonEmpty(a.ghaProcFile, "—")),
		StyleMuted.Render("Status    ") + ghaLiveBadge(live.Label, a.animFrame),
		StyleMuted.Render("Event     ") + StyleNormal.Render(firstNonEmpty(live.Event, "—")),
		StyleMuted.Render("Run       ") + StyleNormal.Render(firstNonEmpty(live.RunID, "—")),
		StyleMuted.Render("Title     ") + StyleNormal.Render(truncate(firstNonEmpty(live.Title, "—"), width-16)),
		StyleMuted.Render("When      ") + StyleNormal.Render(firstNonEmpty(live.CreatedAt, "—")),
		"",
	}
	lines = append(lines, a.ghaUsageOverviewLines(width, procMin, projMin)...)
	lines = append(lines, "",
		StyleMuted.Render("Ações:  t trigger  ·  3 timeline  ·  F failed logs  ·  i incidente"),
		StyleMuted.Render("Abas:   1 Overview  2 Runs  3 Timeline  4 Logs  5 YAML"),
	)
	if live.Label == "idle" {
		lines = append(lines, "", StyleMuted.Render("Parado — nenhum run recente. Use t para disparar manualmente."))
	}
	if live.Label == "triggered" {
		lines = append(lines, "", StyleAccent.Render("Trigger manual enviado — aguardando aparecer em Runs…"))
	}
	return renderApiTitledBox("OVERVIEW", fitExactLines(lines, inner), width, height, true)
}

func (a *App) ghaUsageOverviewLines(width int, procMin, projMin float64) []string {
	barW := maxInt(10, minInt(24, width-18))
	lines := []string{
		StyleMuted.Render("── uso Actions ──"),
	}
	if a.ghaBilling.OK {
		pct := 0.0
		if a.ghaBilling.Included > 0 {
			pct = a.ghaBilling.Used * 100 / a.ghaBilling.Included
		}
		lines = append(lines,
			StyleMuted.Render("Conta     ")+meterBar(pct, barW)+
				StyleMuted.Render(fmt.Sprintf("  %.0fm / %.0fm", a.ghaBilling.Used, a.ghaBilling.Included)),
			StyleMuted.Render("Restante  ")+StyleHealthy.Render(fmt.Sprintf("%.0fm", a.ghaBilling.Remaining)),
		)
	} else {
		lines = append(lines, StyleMuted.Render("Conta     ")+StyleMuted.Render("cota indisponível"))
	}
	share := "—"
	if projMin > 0 {
		share = fmt.Sprintf("%.0f%% do projeto", procMin*100/projMin)
	}
	lines = append(lines,
		StyleMuted.Render("Projeto   ")+StyleAccent.Render(collectors.FormatGHAMinutes(projMin))+
			StyleMuted.Render("  (estimado nos runs)"),
		StyleMuted.Render("Processo  ")+StyleWarning.Render(collectors.FormatGHAMinutes(procMin))+
			StyleMuted.Render("  ·  ")+StyleMuted.Render(share),
	)
	return lines
}

func (a *App) renderGHAProcRuns(width, height int) string {
	inner := maxInt(3, height-2)
	runs := a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile)
	title := fmt.Sprintf("RUNS (%d)", len(runs))
	lines := []string{
		StyleTableHeader.Render(truncate("STATUS     CONCLUSION  EVENT              TITLE", width-4)),
		StyleMuted.Render(strings.Repeat("─", maxInt(8, width-6))),
	}
	if len(runs) == 0 {
		lines = append(lines, StyleMuted.Render("  nenhum run deste processo"))
		return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
	}
	if a.ghaProcRunCursor >= len(runs) {
		a.ghaProcRunCursor = len(runs) - 1
	}
	viewport := maxInt(1, inner-2)
	start := 0
	if a.ghaProcRunCursor >= viewport {
		start = a.ghaProcRunCursor - viewport + 1
	}
	for i := start; i < minInt(start+viewport, len(runs)); i++ {
		r := runs[i]
		st := StyleMuted.Render(truncate(r.Status, 9))
		conc := firstNonEmpty(r.Conclusion, "-")
		switch conc {
		case "success":
			conc = StyleHealthy.Render(truncate(conc, 10))
		case "failure":
			conc = StyleUnhealthy.Render(truncate(conc, 10))
		default:
			conc = StyleMuted.Render(truncate(conc, 10))
		}
		text := fmt.Sprintf("%s  %s  %-16s  %s",
			st, conc, truncate(r.Event, 16), truncate(firstNonEmpty(r.DisplayTitle, r.Name), 28))
		// rebuild without nested styles width issues — simple
		plain := fmt.Sprintf("%-9s  %-10s  %-16s  %s",
			truncate(r.Status, 9), truncate(firstNonEmpty(r.Conclusion, "-"), 10),
			truncate(r.Event, 16), truncate(firstNonEmpty(r.DisplayTitle, r.Name), 28))
		style := StyleNormal
		if i == a.ghaProcRunCursor {
			style = StyleSelected
		}
		_ = text
		lines = append(lines, style.Width(width-4).MaxWidth(width-4).Render(truncate(plain, width-4)))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderGHAProcLogs(width, height int) string {
	inner := maxInt(4, height-2)
	body := a.ghaProcLogs
	if body == "" {
		body = a.ghaDetail
	}
	if strings.TrimSpace(body) == "" {
		body = "carregando logs…\n(se vazio: selecione um run na aba Runs ou dispare com t)"
	}
	raw := strings.Split(body, "\n")
	a.ghaProcScroll = clampScroll(a.ghaProcScroll, inner, len(raw))
	end := minInt(a.ghaProcScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaProcScroll; i < end; i++ {
		ln := sanitizeTerminalLine(raw[i])
		low := strings.ToLower(ln)
		switch {
		case strings.Contains(low, "error"), strings.Contains(low, "##[error]"):
			lines = append(lines, StyleUnhealthy.Render(truncate(ln, width-4)))
		case strings.Contains(low, "warning"), strings.Contains(low, "##[warning]"):
			lines = append(lines, StyleWarning.Render(truncate(ln, width-4)))
		default:
			lines = append(lines, StyleMuted.Render(truncate(ln, width-4)))
		}
	}
	title := "LOGS"
	if a.ghaProcRunID != "" {
		title += " · #" + a.ghaProcRunID
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderGHAProcYAML(width, height int) string {
	inner := maxInt(4, height-2)
	body := a.ghaProcYAML
	if body == "" {
		body = a.ghaDetail
	}
	if strings.TrimSpace(body) == "" {
		body = "carregando YAML…"
	}
	raw := strings.Split(body, "\n")
	a.ghaProcScroll = clampScroll(a.ghaProcScroll, inner, len(raw))
	end := minInt(a.ghaProcScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaProcScroll; i < end; i++ {
		lines = append(lines, StyleNormal.Render(truncate(raw[i], width-4)))
	}
	title := "YAML"
	if a.ghaProcFile != "" {
		title += " · " + filepath.Base(a.ghaProcFile)
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, true)
}
