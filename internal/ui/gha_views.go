package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderGHALanding(p *core.Project) string {
	w, h := a.moduleSize()
	info := a.landingGHA
	procs := a.landingGHAProcs

	status := "…"
	if a.landingGHAOK {
		status = "offline"
		switch {
		case !info.Available:
			status = "no-gh"
		case info.Error != "" && !info.Authed:
			status = "auth"
		case info.Authed:
			status = "ready"
		}
	}

	ctx := a.renderModuleContext(p, w, "ACTIONS", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)
	openH := maxInt(7, bodyH*42/100)
	featH := maxInt(6, bodyH-openH)

	openLines := []string{
		StyleMuted.Render("processes · runs · workflows · logs"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingGHAOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando ambiente…"))
	case !info.Available:
		openLines = append(openLines, "",
			StyleUnhealthy.Render("⚠ GitHub CLI (gh) não instalado"),
			StyleMuted.Render("sudo apt install gh  ·  depois L login"),
		)
	case !info.Authed:
		openLines = append(openLines, "",
			StyleWarning.Render("⚠ gh sem autenticação"),
			StyleMuted.Render("pressione L para gh auth login"),
		)
	default:
		openLines = append(openLines, "",
			StyleHealthy.Render(a.pulse()+" READY")+
				StyleMuted.Render(fmt.Sprintf("  %s/%s  ·  %d processos", info.Owner, info.Repo, procs)))
	}

	featLines := []string{
		StyleMuted.Render("1 arquivo por processo em .github/workflows/"),
		StyleMuted.Render("catálogo central: .devscope/actions.yaml"),
		StyleMuted.Render("c criar  ·  d deletar  ·  t trigger  ·  L login"),
	}

	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("GITHUB ACTIONS", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("POR PROJETO", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	cliLabel, authLabel, procsLabel := "…", "…", "…"
	if a.landingGHAOK {
		cliLabel, authLabel = boolLabel(info.Available), boolLabel(info.Authed)
		procsLabel = fmt.Sprintf("%d", procs)
	}
	details := []string{
		StyleMuted.Render("CLI     ") + StyleNormal.Render(cliLabel),
		StyleMuted.Render("Auth    ") + StyleMuted.Render(authLabel),
		StyleMuted.Render("Procs   ") + StyleNormal.Render(procsLabel),
	}
	if info.Owner != "" {
		details = append(details, StyleMuted.Render("Repo    ")+StyleMuted.Render(truncate(info.Owner+"/"+info.Repo, 22)))
	}
	actions := moduleActionLines(
		[2]string{"enter", "control center"},
		[2]string{"L", "login gh"},
		[2]string{"!", "aviso setup"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderGHATab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	showProcess := a.ghaScreen == ghaScrProcess ||
		(a.ghaForm == ghaFormTrigger && a.ghaTriggerReturn == ghaScrProcess)
	var view string
	if showProcess {
		view = a.renderGHAProcessDetail(p)
	} else {
		view = a.renderGHACluster(p, w, h)
	}
	switch {
	case a.ghaForm == ghaFormTrigger:
		view = overlayCentered(view, a.renderGHATriggerBox(), w, h)
	case a.ghaScreen == ghaScrForm && a.ghaForm == ghaFormSetup:
		view = overlayCentered(view, a.renderGHASetupBox(), w, h)
	case a.ghaScreen == ghaScrForm && a.ghaForm == ghaFormCreate:
		view = overlayCentered(view, a.renderGHACreateBox(), w, h)
	case a.ghaScreen == ghaScrForm && a.ghaForm == ghaFormViewYAML:
		view = overlayCentered(view, a.renderGHAYAMLBox(w, h), w, h)
	case a.ghaScreen == ghaScrLogs:
		view = overlayCentered(view, a.renderGHALogsBox(w, h), w, h)
	case a.ghaScreen == ghaScrDetail:
		view = overlayCentered(view, a.renderGHADetailBox(w, h), w, h)
	}
	if a.ghaConfirm {
		box := renderDeleteConfirmBox(a.ghaConfirmOpts(), w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) ghaConfirmOpts() deleteConfirmOpts {
	act := a.ghaConfirmAction
	opts := deleteConfirmOpts{
		Brand: "ACTIONS",
		Color: tabAccentColor(TabActions),
	}
	switch {
	case strings.HasPrefix(act, "rm-process:"):
		opts.Title = "Excluir processo"
		opts.Subtitle = "remove do catálogo e o workflow local"
		opts.Label = "processo"
		opts.Target = strings.TrimPrefix(act, "rm-process:")
		if a.ghaCursor < len(a.ghaProcesses) {
			opts.Detail = a.ghaProcesses[a.ghaCursor].File
		}
	case strings.HasPrefix(act, "stop-run:"), strings.HasPrefix(act, "cancel-run:"):
		id := strings.TrimPrefix(act, "stop-run:")
		id = strings.TrimPrefix(id, "cancel-run:")
		proc := ""
		if i := strings.Index(id, ":"); i >= 0 {
			proc = id[i+1:]
			id = id[:i]
		}
		opts.Title = "Parar job"
		opts.Subtitle = "cancela o run no GitHub Actions"
		opts.Label = "run"
		opts.Target = "#" + id
		opts.Detail = firstNonEmpty(proc, a.ghaStatus)
	case strings.HasPrefix(act, "stop-all:"):
		rest := strings.TrimPrefix(act, "stop-all:")
		opts.Title = "Parar todos ativos"
		opts.Subtitle = "cancela jobs ativos do processo"
		opts.Label = "processo"
		opts.Target = rest
		if i := strings.LastIndex(rest, ":"); i >= 0 {
			opts.Target = rest[:i]
			opts.Detail = rest[i+1:] + " job(s)"
		}
	case strings.HasPrefix(act, "stop-marked:"):
		opts.Title = "Parar selecionados"
		opts.Subtitle = "cancela runs marcados"
		opts.Label = "runs"
		ids := strings.TrimPrefix(act, "stop-marked:")
		n := 0
		if ids != "" {
			n = strings.Count(ids, ",") + 1
		}
		opts.Target = fmt.Sprintf("%d run(s)", n)
	default:
		opts.Title = "Confirmar"
		opts.Subtitle = "ação no GitHub Actions"
		opts.Label = "ação"
		opts.Target = firstNonEmpty(act, "—")
	}
	return opts
}

func (a *App) renderGHACluster(p *core.Project, w, h int) string {
	header := a.renderGHAHeader(w, p)
	status := a.renderGHAStatusRow(w)
	cards := a.renderGHACards(w)
	tabs := a.renderGHAKindTabs(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(status) + lipgloss.Height(cards) + lipgloss.Height(tabs) + 2
	bodyH := maxInt(8, h-chromeH-2)

	rightW := maxInt(22, w*24/100)
	if rightW > 34 {
		rightW = 34
	}
	mainW := maxInt(40, w-rightW)
	// RESUMO ganha mais espaço quando está em foco (leitura do YAML).
	tablePct := 50
	if a.ghaFocus == ghaFocusResumo {
		tablePct = 38
	}
	tableH := maxInt(6, bodyH*tablePct/100)
	detailH := maxInt(5, bodyH-tableH)

	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderGHATable(mainW, tableH),
		a.renderGHASummary(mainW, detailH),
	)
	right := a.renderGHARightRail(rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, header, status, cards, tabs, body, a.renderStatusBar(a.ghaHints()))
}

func (a *App) renderGHAHeader(width int, p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabActions)).Bold(true)
	proj := "project"
	if p != nil && p.Name != "" {
		proj = p.Name
	}
	left := accent.Render("GITHUB ACTIONS") + StyleMuted.Render(" › PROJECT") +
		StyleMuted.Render("  ·  ") + StyleNormal.Render(truncate(proj, 20))
	right := StyleMuted.Render(time.Now().Format("15:04:05"))
	if a.ghaLoading {
		right = a.loadingMuted("Loading…")
	} else if a.ghaOpen {
		right = StyleMuted.Render(fmt.Sprintf("auto %ds  %s", int(a.ghaTickInterval()/time.Second), time.Now().Format("15:04:05")))
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderGHAStatusRow(width int) string {
	owner, repoName := a.ghaResolveOwnerRepo()
	var badge string
	switch {
	case !a.ghaInfo.Available && owner != "":
		badge = StyleWarning.Render("⚠ NO GH CLI")
	case !a.ghaInfo.Available:
		badge = StyleUnhealthy.Render("✕ NO GH CLI")
	case !a.ghaInfo.Authed:
		badge = StyleWarning.Render("⚠ AUTH REQUIRED")
	default:
		badge = a.livePulse("READY")
	}
	repo := "—"
	if owner != "" {
		repo = owner + "/" + repoName
	}
	meta := StyleMuted.Render("  "+truncate(repo, 28)) +
		StyleMuted.Render(fmt.Sprintf("  %d processes  %d runs", len(a.ghaProcesses), len(a.ghaRuns)))
	line := badge + meta
	if a.ghaErr != "" && owner == "" {
		line += StyleMuted.Render("  ") + StyleUnhealthy.Render(truncate(a.ghaErr, 24))
	} else if a.ghaErr != "" && !a.ghaInfo.Available {
		line += StyleMuted.Render("  ") + StyleMuted.Render("o abre no browser")
	}
	return truncate(line, width)
}

func (a *App) renderGHACards(width int) string {
	n := 6
	boxW := maxInt(12, width/n)
	success, fail, running := 0, 0, 0
	for _, r := range a.ghaRuns {
		switch {
		case r.Status == "in_progress" || r.Status == "queued":
			running++
		case r.Conclusion == "success":
			success++
		case r.Conclusion == "failure" || r.Conclusion == "cancelled":
			fail++
		}
	}
	heat := collectors.FormatGHAFailHeatmap(collectors.GHAFailHeatmap(a.ghaRuns, 20), boxW)
	usageTitle, usageVal, usageStyle := a.ghaUsageCardBits(boxW - 4)
	cards := []struct {
		title, value string
		style        lipgloss.Style
	}{
		{"PROCESSES", fmt.Sprintf("%d", len(a.ghaProcesses)), StyleAccent},
		{"RUNS", fmt.Sprintf("%d", len(a.ghaRuns)), StyleMuted},
		{"OK/FAIL", fmt.Sprintf("%d / %d", success, fail), StyleHealthy},
		{"ACTIVE", fmt.Sprintf("%d", running), StyleWarning},
		{"FAIL/WEEK", heat, StyleUnhealthy},
		{usageTitle, usageVal, usageStyle},
	}
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		val := c.style.Render(truncate(c.value, boxW-4))
		parts = append(parts, renderApiTitledBox(c.title, fitExactLines([]string{val}, 1), boxW, 3, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) ghaUsageCardBits(maxW int) (title, value string, style lipgloss.Style) {
	style = StyleHealthy
	proj := collectors.GHAMinutesFromRuns(a.ghaRuns)
	if a.ghaBilling.OK {
		title = "MIN LEFT"
		rem := a.ghaBilling.Remaining
		if rem < a.ghaBilling.Included*0.15 {
			style = StyleUnhealthy
		} else if rem < a.ghaBilling.Included*0.35 {
			style = StyleWarning
		}
		barW := maxInt(6, minInt(10, maxW-8))
		value = fmt.Sprintf("%s %s",
			meterBar(a.ghaBilling.Used*100/a.ghaBilling.Included, barW),
			fmt.Sprintf("%.0fm", rem),
		)
		return title, value, style
	}
	title = "USO ~"
	style = StyleNormal
	value = collectors.FormatGHAMinutes(proj) + " proj"
	return title, value, style
}

func (a *App) renderGHAKindTabs(width int) string {
	kinds := []ghaKind{ghaKindProcesses, ghaKindRuns, ghaKindWorkflows}
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		label := " " + strings.ToUpper(k.String()) + " "
		if k == a.ghaKind {
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

func (a *App) renderGHATable(width, height int) string {
	title := strings.ToUpper(a.ghaKind.String())
	n := a.ghaRowCount()
	if a.ghaKind == ghaKindRuns {
		total := len(a.ghaRuns)
		if n != total || a.ghaRunScope != ghaRunScopeAll || a.ghaRunProcFilter != "" {
			title = fmt.Sprintf("RUNS (%d/%d) · %s", n, total, a.ghaRunsFilterLabel())
		} else if n > 0 {
			title = fmt.Sprintf("RUNS (%d)", n)
		}
	} else if n > 0 {
		title = fmt.Sprintf("%s (%d)", title, n)
	}
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	lines := []string{
		StyleTableHeader.Render(truncate(a.ghaTableHeader(), width-4)),
		StyleMuted.Render(strings.Repeat("─", maxInt(8, width-6))),
	}
	if n == 0 {
		msg := "nenhum item"
		switch a.ghaKind {
		case ghaKindProcesses:
			msg = "nenhum processo — pressione c para criar"
		case ghaKindRuns:
			msg = "nenhum run com estes filtros — f status · p processo · 0 limpar"
		}
		lines = append(lines, StyleMuted.Render("  "+msg))
		return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusTable)
	}
	a.ghaScroll = ensureVisible(a.ghaCursor, a.ghaScroll, viewport, n)
	for i := a.ghaScroll; i < minInt(a.ghaScroll+viewport, n); i++ {
		lines = append(lines, a.renderGHARow(i, width-4, i == a.ghaCursor && a.ghaFocus == ghaFocusTable))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusTable)
}

func (a *App) ghaTableHeader() string {
	switch a.ghaKind {
	case ghaKindProcesses:
		return "STATUS      NAME              FILE                         EVENT"
	case ghaKindRuns:
		return "M  STATUS     CONCLUSION  WORKFLOW        BRANCH      TITLE"
	case ghaKindWorkflows:
		return "NAME                      STATE      PATH"
	}
	return ""
}

func (a *App) renderGHARow(i, width int, selected bool) string {
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	switch a.ghaKind {
	case ghaKindProcesses:
		p := a.ghaProcesses[i]
		live := a.ghaStatusForProcess(p.Name, p.File)
		rest := fmt.Sprintf(" %-16s  %-26s  %s",
			truncate(p.Name, 16), truncate(p.File, 26), truncate(firstNonEmpty(live.Event, "-"), 16))
		if selected {
			text := fmt.Sprintf("%-10s %-16s  %-26s  %s",
				truncate(live.Label, 10), truncate(p.Name, 16), truncate(p.File, 26), truncate(firstNonEmpty(live.Event, "-"), 16))
			return style.Width(width).MaxWidth(width).Render(truncate(text, width))
		}
		return padRight(ghaLiveBadge(live.Label, a.animFrame)+StyleNormal.Render(rest), width)
	case ghaKindRuns:
		runs := a.ghaFilteredRuns()
		if i < 0 || i >= len(runs) {
			return ""
		}
		r := runs[i]
		mark := " "
		if a.ghaRunMarked != nil && a.ghaRunMarked[r.ID] {
			mark = "✓"
		}
		note := ""
		if a.ghaNotes != nil && a.ghaNotes[r.ID] != "" {
			note = "!"
		}
		text := fmt.Sprintf("%s%s %-9s  %-10s  %-14s  %-10s  %s",
			mark, note, truncate(r.Status, 9), truncate(firstNonEmpty(r.Conclusion, "-"), 10),
			truncate(r.Workflow, 14), truncate(r.Branch, 10), truncate(firstNonEmpty(r.DisplayTitle, r.Name), 24))
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	case ghaKindWorkflows:
		w := a.ghaWorkflows[i]
		text := fmt.Sprintf("%-24s  %-8s  %s", truncate(w.Name, 24), truncate(w.State, 8), truncate(w.Path, 36))
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	}
	return ""
}

func (a *App) renderGHASummary(width, height int) string {
	inner := maxInt(2, height-2)
	body := a.ghaDetail
	title := "RESUMO"
	if a.ghaFocus == ghaFocusResumo {
		title = "RESUMO  ↑↓ scroll"
	}
	if a.ghaStatus != "" && strings.TrimSpace(body) == "" {
		title = "STATUS"
		body = a.ghaStatus
	}
	if strings.TrimSpace(body) == "" {
		body = "tab → resumo  ·  enter foca aqui  ·  ↑↓ scroll no YAML"
	}
	raw := strings.Split(body, "\n")
	viewport := maxInt(1, inner)
	a.ghaDetailScroll = clampScroll(a.ghaDetailScroll, viewport, len(raw))
	end := minInt(a.ghaDetailScroll+viewport, len(raw))
	total := len(raw)
	if total > viewport {
		pos := a.ghaDetailScroll + 1
		maxPos := total - viewport + 1
		title = fmt.Sprintf("%s  %d/%d", title, pos, maxPos)
	}
	lines := make([]string, 0, viewport)
	focused := a.ghaFocus == ghaFocusResumo
	for i := a.ghaDetailScroll; i < end; i++ {
		ln := truncate(sanitizeTerminalLine(raw[i]), width-4)
		if focused {
			lines = append(lines, StyleNormal.Render(ln))
		} else {
			lines = append(lines, StyleMuted.Render(ln))
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, focused)
}

func (a *App) renderGHARightRail(width, height int) string {
	repoH := maxInt(5, height*22/100)
	usageH := maxInt(8, height*34/100)
	runsH := maxInt(4, height*22/100)
	actH := maxInt(5, height-repoH-usageH-runsH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderGHARepoPanel(width, repoH),
		a.renderGHAUsagePanel(width, usageH),
		a.renderGHARecentRunsPanel(width, runsH),
		a.renderGHAActionsPanel(width, actH),
	)
}

func (a *App) renderGHAUsagePanel(width, height int) string {
	inner := maxInt(3, height-2)
	barW := maxInt(8, minInt(18, width-8))
	projMin := collectors.GHAMinutesFromRuns(a.ghaRuns)
	lines := []string{}

	if a.ghaBilling.OK {
		pct := 0.0
		if a.ghaBilling.Included > 0 {
			pct = a.ghaBilling.Used * 100 / a.ghaBilling.Included
		}
		lines = append(lines,
			StyleMuted.Render("Conta ")+StyleNormal.Render(a.ghaBilling.Source),
			meterBar(pct, barW)+StyleMuted.Render(fmt.Sprintf(" %.0f%%", pct)),
			StyleMuted.Render("Restante ")+StyleHealthy.Render(fmt.Sprintf("%.0fm", a.ghaBilling.Remaining))+
				StyleMuted.Render(fmt.Sprintf(" / %.0fm", a.ghaBilling.Included)),
			StyleMuted.Render("Usado   ")+StyleWarning.Render(fmt.Sprintf("%.0fm", a.ghaBilling.Used)),
		)
		if a.ghaBilling.DaysLeft > 0 {
			lines = append(lines, StyleMuted.Render("Ciclo    ")+StyleNormal.Render(fmt.Sprintf("%dd", a.ghaBilling.DaysLeft)))
		}
	} else {
		lines = append(lines,
			StyleMuted.Render("Cota conta indisponível"),
			StyleMuted.Render("(billing API / permissão)"),
		)
		if a.ghaBilling.Error != "" {
			lines = append(lines, StyleMuted.Render(truncate(a.ghaBilling.Error, width-4)))
		}
	}

	lines = append(lines, "",
		StyleMuted.Render("Projeto  ")+StyleAccent.Render(collectors.FormatGHAMinutes(projMin))+
			StyleMuted.Render("  (runs)"),
	)

	// Per-process breakdown from catalog + run estimates.
	type row struct {
		name string
		min  float64
	}
	rows := make([]row, 0, len(a.ghaProcesses))
	if len(a.ghaProcesses) > 0 {
		for _, p := range a.ghaProcesses {
			m := collectors.GHAMinutesFromRuns(a.ghaRunsForProcess(p.Name, p.File))
			rows = append(rows, row{name: p.Name, min: m})
		}
	} else {
		for _, b := range collectors.GHABillingEstimate(a.ghaRuns, 40) {
			rows = append(rows, row{name: b.Workflow, min: b.Minutes})
		}
	}
	if len(rows) == 0 {
		lines = append(lines, StyleMuted.Render("Processos (sem dados)"))
	} else {
		lines = append(lines, StyleMuted.Render("Por processo"))
		maxShow := minInt(len(rows), maxInt(2, inner-len(lines)-1))
		for i := 0; i < maxShow; i++ {
			r := rows[i]
			lines = append(lines, StyleMuted.Render("· ")+
				StyleNormal.Render(truncate(r.name, maxInt(6, width-12)))+
				StyleMuted.Render(" ")+
				StyleWarning.Render(collectors.FormatGHAMinutes(r.min)))
		}
	}
	return renderApiTitledBox("USO", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderGHARepoPanel(width, height int) string {
	inner := maxInt(2, height-2)
	lines := []string{
		StyleMuted.Render("Catalog"),
		StyleNormal.Render(truncate(".devscope/actions.yaml", width-4)),
		StyleMuted.Render("Workflows dir"),
		StyleNormal.Render(truncate(".github/workflows/", width-4)),
	}
	if a.ghaInfo.Owner != "" {
		lines = append(lines,
			StyleMuted.Render("Repository"),
			StyleNormal.Render(truncate(a.ghaInfo.Owner+"/"+a.ghaInfo.Repo, width-4)),
		)
	}
	return renderApiTitledBox("PROJETO", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderGHARecentRunsPanel(width, height int) string {
	inner := maxInt(2, height-2)
	lines := []string{}
	limit := minInt(10, len(a.ghaRuns))
	for i := 0; i < limit; i++ {
		r := a.ghaRuns[i]
		dot := StyleMuted.Render("●")
		switch {
		case r.Conclusion == "success":
			dot = a.livePulse("")
		case r.Conclusion == "failure":
			dot = StyleUnhealthy.Render("●")
		case r.Status == "in_progress" || r.Status == "queued":
			dot = StyleWarning.Render("●")
		}
		lines = append(lines, dot+" "+StyleMuted.Render(truncate(r.Workflow+"  "+firstNonEmpty(r.Conclusion, r.Status), width-6)))
	}
	if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("sem runs recentes"))
	}
	return renderApiTitledBox("RUNS RECENTES", fitExactLines(lines, inner), width, height, false)
}

func (a *App) renderGHAActionsPanel(width, height int) string {
	items := a.ghaQuickActionItems()
	if a.ghaActionIdx >= len(items) {
		a.ghaActionIdx = maxInt(0, len(items)-1)
	}
	inner := maxInt(2, height-2)
	lines := make([]string, 0, len(items))
	for i, it := range items {
		prefix := "  "
		style := StyleMuted
		if i == a.ghaActionIdx && a.ghaFocus == ghaFocusActions {
			prefix = StyleAccent.Render("› ")
			style = StyleNormal
		}
		lines = append(lines, prefix+StyleKey.Render(it[0])+" "+style.Render(it[1]))
	}
	return renderApiTitledBox("AÇÕES RÁPIDAS", fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusActions)
}

func (a *App) ghaConfirmHint() string {
	return "modal  y confirma  n/esc cancela"
}

func (a *App) ghaHints() string {
	if a.ghaConfirm {
		return a.ghaConfirmHint()
	}
	if a.ghaScreen == ghaScrForm || a.ghaForm == ghaFormTrigger {
		switch a.ghaForm {
		case ghaFormTrigger:
			return "TRIGGER  ↑↓ branch  tab inputs  P push  enter/y dispara  esc"
		case ghaFormViewYAML:
			return "yaml  ↑↓ scroll  esc fechar"
		case ghaFormSetup:
			if !a.ghaInfo.Available {
				return "SETUP  o docs  enter continuar sem gh  esc fechar"
			}
			return "SETUP  L login  enter continuar  esc fechar"
		default:
			return "criar  tab campo  [] template  enter salva  esc cancela"
		}
	}
	if a.ghaScreen == ghaScrLogs {
		return "logs  f refresh  ↑↓ scroll  esc voltar"
	}
	if a.ghaScreen == ghaScrDetail {
		return "detalhe  l logs  y yaml  esc voltar"
	}
	auto := fmt.Sprintf("auto %ds", int(a.ghaTickInterval()/time.Second))
	base := "tab painel  [] lista  enter detalhe  ↑↓  t trigger  s parar  ·  " + auto + "  esc"
	if a.ghaNoteEditing {
		return "NOTA  " + a.ghaNoteInput + "█  enter salva  esc"
	}
	if a.ghaKind == ghaKindRuns {
		base = "RUNS  f/p filtro  space marca  S bulk  i nota  F failed  ·  " + auto
	}
	switch a.ghaFocus {
	case ghaFocusResumo:
		base = "RESUMO  ↑↓/pg scroll  tab painel  enter detalhe  ·  " + auto + "  esc"
	case ghaFocusActions:
		base = "AÇÕES  ↑↓  enter executa  tab painel  ·  " + auto + "  esc"
	}
	if a.ghaNeedsSetup() {
		base = "⚠ L login  ! setup  ·  " + base
	}
	if a.ghaStatus != "" {
		return truncate(a.ghaStatus+"  ·  "+base, maxInt(40, a.width-4))
	}
	return base
}

func (a *App) renderGHASetupBox() string {
	w := 62
	var lines []string
	title := "GITHUB ACTIONS · SETUP"
	if !a.ghaInfo.Available {
		title = "⚠ GH CLI NÃO INSTALADO"
		lines = []string{
			StyleUnhealthy.Render("O GitHub CLI (gh) não está no PATH."),
			"",
			StyleMuted.Render("Sem o gh você ainda pode:"),
			StyleNormal.Render("  · ver / criar / deletar workflows locais"),
			StyleNormal.Render("  · abrir o YAML (enter)"),
			StyleNormal.Render("  · abrir o GitHub no browser (o)"),
			"",
			StyleMuted.Render("Para trigger, runs e logs remotos, instale:"),
			StyleAccent.Render("  sudo apt install gh"),
			StyleMuted.Render("  # ou:  sudo snap install gh"),
			"",
			StyleMuted.Render("Depois:  L  →  gh auth login"),
			"",
			StyleKey.Render("o") + StyleMuted.Render("  abrir docs  cli.github.com"),
			StyleKey.Render("enter") + StyleMuted.Render("  continuar só com arquivos locais"),
			StyleKey.Render("esc") + StyleMuted.Render("  fechar aviso"),
		}
	} else {
		title = "⚠ GH SEM AUTENTICAÇÃO"
		lines = []string{
			StyleWarning.Render("gh encontrado, mas não autenticado."),
			"",
			StyleMuted.Render("Trigger, runs e logs remotos precisam de login."),
			"",
			StyleAccent.Render("L") + StyleNormal.Render("  inicia  gh auth login  (browser)"),
			"",
			StyleMuted.Render("Fluxo: escolha GitHub.com → HTTPS → Login with browser"),
			"",
			StyleMuted.Render("Arquivos locais (.github/workflows) continuam ok."),
			"",
			StyleKey.Render("L") + StyleMuted.Render("  login agora"),
			StyleKey.Render("r") + StyleMuted.Render("  rechecar status"),
			StyleKey.Render("enter") + StyleMuted.Render("  continuar sem auth"),
			StyleKey.Render("esc") + StyleMuted.Render("  fechar aviso"),
		}
	}
	inner := fitExactLines(lines, len(lines))
	return renderApiTitledBox(title, inner, w, len(inner)+2, true)
}

func (a *App) renderGHACreateBox() string {
	w := 56
	name, desc, tpl := a.ghaFormName, a.ghaFormDesc, a.ghaFormTemplate
	switch a.ghaFormField {
	case 0:
		name = a.ghaFormInput
	case 1:
		desc = a.ghaFormInput
	case 2:
		tpl = a.ghaFormInput
	}
	lines := []string{
		StyleMuted.Render("Cria .github/workflows/<name>.yml"),
		StyleMuted.Render("e registra em .devscope/actions.yaml"),
		"",
		swarmFormFieldLine(0, a.ghaFormField, "Name", name),
		swarmFormFieldLine(1, a.ghaFormField, "Desc", desc),
		swarmFormFieldLine(2, a.ghaFormField, "Template", tpl+"  ([] ci|deploy|manual)"),
		"",
		StyleMuted.Render("enter cria  ·  esc cancela"),
	}
	inner := fitExactLines(lines, len(lines))
	return renderApiTitledBox("CREATE PROCESS", inner, w, len(inner)+2, true)
}

func (a *App) renderGHATriggerBox() string {
	w := 62
	viewport := 8
	n := len(a.ghaTriggerBranches)
	if a.ghaTriggerCursor < 0 {
		a.ghaTriggerCursor = 0
	}
	if n > 0 && a.ghaTriggerCursor >= n {
		a.ghaTriggerCursor = n - 1
	}
	a.ghaTriggerScroll = ensureVisible(a.ghaTriggerCursor, a.ghaTriggerScroll, viewport, n)

	proc := firstNonEmpty(a.ghaTriggerProc, a.ghaProcName, "workflow")
	wf := firstNonEmpty(a.ghaTriggerWF, "—")
	lines := []string{
		StyleMuted.Render("Processo  ") + StyleNormal.Render(truncate(proc, 40)),
		StyleMuted.Render("Workflow  ") + StyleNormal.Render(truncate(wf, 40)),
		StyleMuted.Render("Branches no origin (pushed)"),
		"",
		StyleTableHeader.Render(truncate("  BRANCH", w-6)),
		StyleMuted.Render(strings.Repeat("─", maxInt(8, w-8))),
	}
	if n == 0 {
		lines = append(lines,
			StyleWarning.Render("  nenhuma branch remote"),
			StyleMuted.Render("  git push -u origin <branch>"),
		)
	} else {
		current := collectors.GitCurrentBranchName(a.ghaPath)
		end := minInt(a.ghaTriggerScroll+viewport, n)
		for i := a.ghaTriggerScroll; i < end; i++ {
			b := a.ghaTriggerBranches[i]
			mark := "  "
			if b == current {
				mark = "● "
			}
			plain := mark + truncate(b, w-10)
			style := StyleNormal
			if i == a.ghaTriggerCursor && a.ghaTriggerInputIdx < 0 {
				style = StyleSelected
				plain = "› " + strings.TrimPrefix(plain, "  ")
			}
			lines = append(lines, style.Width(w-4).MaxWidth(w-4).Render(truncate(plain, w-4)))
		}
	}
	if a.ghaTriggerAhead > 0 {
		lines = append(lines, "",
			StyleWarning.Render(fmt.Sprintf("⚠ PUSH NEEDED · %d commit(s) local ahead of origin", a.ghaTriggerAhead)),
			StyleMuted.Render("  P push agora  ·  y dispara mesmo assim"),
		)
	}
	if len(a.ghaTriggerInputs) > 0 {
		lines = append(lines, "",
			StyleTableHeader.Render("INPUTS (tab)"),
			StyleMuted.Render(strings.Repeat("─", maxInt(8, w-8))),
		)
		for i, in := range a.ghaTriggerInputs {
			val := ""
			if i < len(a.ghaTriggerInputVals) {
				val = a.ghaTriggerInputVals[i]
			}
			req := ""
			if in.Required {
				req = "*"
			}
			hint := ""
			if len(in.Options) > 0 {
				hint = " []"
			} else if in.Type == "boolean" {
				hint = " [] bool"
			}
			plain := fmt.Sprintf("  %s%s = %s%s", in.Name, req, val, hint)
			style := StyleNormal
			if i == a.ghaTriggerInputIdx {
				style = StyleSelected
				plain = "› " + strings.TrimSpace(plain)
			}
			lines = append(lines, style.Width(w-4).MaxWidth(w-4).Render(truncate(plain, w-4)))
		}
	}
	lines = append(lines, "",
		StyleMuted.Render("↑↓ branch  tab inputs  enter dispara  P push  r  esc"),
	)
	inner := fitExactLines(lines, len(lines))
	return renderApiTitledBox("TRIGGER · BRANCH + INPUTS", inner, w, len(inner)+2, true)
}

func (a *App) renderGHAYAMLBox(termW, termH int) string {
	w := minInt(termW-4, 88)
	h := minInt(termH-4, 30)
	inner := maxInt(8, h-2)
	raw := strings.Split(a.ghaDetail, "\n")
	if strings.TrimSpace(a.ghaDetail) == "" {
		raw = []string{a.loadingMuted("carregando arquivo…")}
	}
	a.ghaDetailScroll = clampScroll(a.ghaDetailScroll, inner, len(raw))
	end := minInt(a.ghaDetailScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaDetailScroll; i < end; i++ {
		lines = append(lines, StyleNormal.Render(truncate(raw[i], w-4)))
	}
	title := "WORKFLOW YAML"
	if a.ghaFormName != "" {
		title = filepath.Base(a.ghaFormName)
	}
	footer := StyleMuted.Render("↑↓ scroll  ·  esc fechar  ·  o github")
	body := append(fitExactLines(lines, maxInt(1, inner-1)), footer)
	return renderApiTitledBox(title, body, w, h, true)
}

func (a *App) renderGHALogsBox(termW, termH int) string {
	w := minInt(termW-4, 84)
	h := minInt(termH-4, 26)
	inner := maxInt(6, h-2)
	raw := strings.Split(a.ghaDetail, "\n")
	a.ghaDetailScroll = clampScroll(a.ghaDetailScroll, inner, len(raw))
	end := minInt(a.ghaDetailScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaDetailScroll; i < end; i++ {
		ln := sanitizeTerminalLine(raw[i])
		low := strings.ToLower(ln)
		switch {
		case strings.Contains(low, "error"), strings.Contains(low, "##[error]"):
			lines = append(lines, StyleUnhealthy.Render(truncate(ln, w-4)))
		case strings.Contains(low, "warning"), strings.Contains(low, "##[warning]"):
			lines = append(lines, StyleWarning.Render(truncate(ln, w-4)))
		default:
			lines = append(lines, StyleMuted.Render(truncate(ln, w-4)))
		}
	}
	title := "RUN LOGS"
	if a.ghaKind == ghaKindRuns {
		if r, ok := a.ghaSelectedRun(); ok {
			title += " · #" + r.ID
			if note := a.ghaNotes[r.ID]; note != "" {
				title += " · !" + truncate(note, 20)
			}
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), w, h, true)
}

func (a *App) renderGHADetailBox(termW, termH int) string {
	w := minInt(termW-4, 84)
	h := minInt(termH-4, 28)
	inner := maxInt(6, h-2)
	raw := strings.Split(a.ghaDetail, "\n")
	a.ghaDetailScroll = clampScroll(a.ghaDetailScroll, inner, len(raw))
	end := minInt(a.ghaDetailScroll+inner, len(raw))
	lines := make([]string, 0, inner)
	for i := a.ghaDetailScroll; i < end; i++ {
		lines = append(lines, StyleMuted.Render(truncate(sanitizeTerminalLine(raw[i]), w-4)))
	}
	return renderApiTitledBox("DETAILS", fitExactLines(lines, inner), w, h, true)
}
