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

// renderGHALanding: uma caixa com dado real no lugar das duas de documentação
// ("POR PROJETO") e do rail DETALHES que só mostrava "CLI …  Auth …  Procs …".
func (a *App) renderGHALanding(p *core.Project) string {
	info := a.landingGHA
	state, note := landingProbing(), ""
	if a.landingGHAOK {
		switch {
		case !info.Available:
			state = StyleUnhealthy.Render("⚠ gh não encontrado no PATH")
			note = StyleMuted.Render("instale o GitHub CLI para ver runs e workflows")
		case !info.Authed:
			state = StyleWarning.Render("○ gh sem login")
			note = StyleMuted.Render("L faz o login sem sair daqui")
		default:
			state = StyleHealthy.Render(a.okPulse() + " conectado ao GitHub")
		}
	}
	// Fato que só repete o aviso é ruído: sem o gh no PATH, "gh não" já foi
	// dito na linha de estado.
	var facts [][2]string
	if a.landingGHAOK && info.Available {
		facts = [][2]string{
			{"repo", StyleNormal.Render(firstNonEmpty(info.Repo, emDash))},
			{"login", StyleNormal.Render(boolLabel(info.Authed))},
			{"fluxos", StyleNormal.Render(probedCount(a.landingGHAOK, a.landingGHAProcs))},
		}
	}
	return a.renderModuleLanding(p, moduleLanding{
		title:        "GITHUB ACTIONS",
		tagline:      "runs, workflows e logs do repositório — trigger e re-run",
		state:        state,
		note:         note,
		facts:        facts,
		previewTitle: "WORKFLOWS DESTE REPOSITÓRIO",
		preview:      a.landingFileRows(a.landingGHANames),
		previewEmpty: "nenhum arquivo em .github/workflows",
		previewFoot:  ghaWorkflowFoot(a.landingGHAProcs),
		actions: [][2]string{
			{"enter", "control center"},
			{"L", "login gh"},
			{"!", "aviso setup"},
			{"esc", "voltar"},
		},
	})
}

func (a *App) ghaLandingLines(width int, info collectors.GHAInfo) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 13)) }
	if !a.landingGHAOK {
		return []string{label("Ambiente") + a.loadingText("detectando gh e repositório…")}
	}
	if !info.Available {
		return []string{
			StyleUnhealthy.Render("✕ GitHub CLI (gh) não encontrado"),
			"",
			StyleMuted.Render("sudo apt install gh"),
			StyleMuted.Render("depois ") + StyleKey.Render("L") + StyleMuted.Render(" para gh auth login"),
		}
	}

	lines := []string{
		label("Repositório") + StyleNormal.Render(firstNonEmpty(truncate(info.Owner+"/"+info.Repo, width-13), emDash)),
		label("CLI") + StyleHealthy.Render("● gh instalado"),
	}
	if info.Authed {
		lines = append(lines, label("Conta")+StyleHealthy.Render("● autenticado"))
	} else {
		lines = append(lines,
			label("Conta")+StyleWarning.Render("⚠ sem login"),
			label("")+StyleMuted.Render("pressione ")+StyleKey.Render("L")+StyleMuted.Render(" para gh auth login"))
	}
	lines = append(lines,
		label("Processos")+StyleNormal.Render(fmt.Sprintf("%d", a.landingGHAProcs))+
			StyleMuted.Render("  em .devscope/actions.yaml"),
		label("Workflows")+StyleMuted.Render(".github/workflows/"),
	)
	if info.Error != "" && !info.Authed {
		lines = append(lines, "", StyleMuted.Render(truncate(info.Error, width)))
	}
	return lines
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

// ─── vocabulário de resultado ───────────────────────────────────────────────

// ghaResult traduz o par (status, conclusion) da API num chip só. A tabela
// tinha duas colunas — STATUS e CONCLUSION — em string crua: "in_progr…" + "-".
func ghaResult(status, conclusion string) (glyph, label string, st lipgloss.Style) {
	switch {
	case status == "in_progress":
		return "●", "rodando", StyleWarning
	case status == "queued", status == "waiting", status == "pending":
		return "◌", "na fila", StyleMuted
	}
	switch conclusion {
	case "success":
		return "✓", "sucesso", StyleHealthy
	case "failure", "startup_failure":
		return "✕", "falha", StyleUnhealthy
	case "timed_out":
		return "✕", "timeout", StyleUnhealthy
	case "cancelled":
		return "⊘", "cancelado", StyleMuted
	case "skipped":
		return "⊘", "pulado", StyleMuted
	case "action_required":
		return "⚠", "ação req.", StyleWarning
	case "neutral":
		return "·", "neutro", StyleMuted
	case "":
		return "◌", firstNonEmpty(status, "—"), StyleMuted
	default:
		return "·", conclusion, StyleMuted
	}
}

// ghaResultCell devolve o chip pintado, com largura visual exata — o badge
// antigo tinha comprimento variável e desalinhava a tabela de processos.
func ghaResultCell(status, conclusion string, width int) string {
	glyph, label, st := ghaResult(status, conclusion)
	return st.Render(padRight(truncate(glyph+" "+label, width), width))
}

func ghaParseTime(vals ...string) time.Time {
	for _, v := range vals {
		if v == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ghaRunWhen: há quanto tempo o run começou. A tabela não tinha nenhuma coluna
// de tempo — é a primeira coisa que se procura num CI.
func ghaRunWhen(r collectors.GHARun) string {
	t := ghaParseTime(r.StartedAt, r.CreatedAt)
	if t.IsZero() {
		return emDash
	}
	return relTime(t)
}

// ghaRunDuration: quanto o run levou (ou leva, se ainda está rodando).
func ghaRunDuration(r collectors.GHARun) string {
	start := ghaParseTime(r.StartedAt, r.CreatedAt)
	if start.IsZero() {
		return emDash
	}
	end := ghaParseTime(r.UpdatedAt)
	if ghaRunIsActive(r) || end.IsZero() || end.Before(start) {
		end = time.Now()
	}
	d := end.Sub(start)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02d", int(d.Hours()), int(d.Minutes())%60)
	}
}

func (a *App) ghaCounters() (running, ok, fail, other int) {
	for _, r := range a.ghaRuns {
		switch {
		case ghaRunIsActive(r):
			running++
		case r.Conclusion == "success":
			ok++
		case r.Conclusion == "failure" || r.Conclusion == "timed_out" || r.Conclusion == "startup_failure":
			fail++
		case r.Conclusion == "cancelled" || r.Conclusion == "skipped":
			other++
		}
	}
	return
}

func (a *App) renderGHACluster(p *core.Project, w, h int) string {
	header := a.renderGHAHeader(w, p)
	tabs := a.renderGHAKindTabs(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(tabs) + 2
	bodyH := maxInt(8, h-chromeH-2)

	rightW := maxInt(22, w*24/100)
	if rightW > 34 {
		rightW = 34
	}
	mainW := maxInt(40, w-rightW)
	// RESUMO só toma espaço quando tem YAML para mostrar; em foco, toma mais.
	detailH := 3
	if strings.TrimSpace(a.ghaDetail) != "" || strings.TrimSpace(a.ghaStatus) != "" {
		pct := 50
		if a.ghaFocus == ghaFocusResumo {
			pct = 62
		}
		detailH = maxInt(5, bodyH*pct/100)
	}
	tableH := maxInt(6, bodyH-detailH)

	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderGHATable(mainW, tableH),
		a.renderGHASummary(mainW, detailH),
	)
	right := a.renderGHARightRail(rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, a.renderStatusBar(a.ghaHints()))
}

// renderGHAHeader agora carrega tudo que ficava espalhado entre o header, a
// linha de status e o card MIN LEFT — os três repetiam repo e contagens.
func (a *App) renderGHAHeader(width int, p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabActions)).Bold(true)
	left := accent.Render("▶ GITHUB ACTIONS")

	owner, repoName := a.ghaResolveOwnerRepo()
	if owner != "" {
		left += StyleMuted.Render("   " + truncate(owner+"/"+repoName, 30))
	} else if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 24))
	}
	left += "   " + a.ghaConnChip()

	var right []string
	if a.ghaLoading {
		right = append(right, a.loadingMuted("carregando…"))
	} else if a.ghaOpen {
		right = append(right, StyleMuted.Render(fmt.Sprintf("⟳ auto %ds", int(a.ghaTickInterval()/time.Second))))
	}
	if chip := a.ghaUsageChip(); chip != "" {
		right = append(right, chip)
	}
	right = append(right, StyleMuted.Render(time.Now().Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("   ")), width)
}

func (a *App) ghaConnChip() string {
	owner, _ := a.ghaResolveOwnerRepo()
	switch {
	case !a.ghaInfo.Available && owner != "":
		return StyleWarning.Render("⚠ sem gh cli")
	case !a.ghaInfo.Available:
		return StyleUnhealthy.Render("✕ sem gh cli")
	case !a.ghaInfo.Authed:
		return StyleWarning.Render("⚠ falta login")
	default:
		return StyleHealthy.Render("● pronto")
	}
}

// ghaUsageChip resume a cota da conta numa expressão só, no lugar do card
// MIN LEFT que gastava 3 linhas × 24 colunas para exibir "580m".
func (a *App) ghaUsageChip() string {
	if !a.ghaBilling.OK || a.ghaBilling.Included <= 0 {
		return ""
	}
	st := StyleHealthy
	switch rem := a.ghaBilling.Remaining; {
	case rem < a.ghaBilling.Included*0.10:
		st = StyleUnhealthy
	case rem < a.ghaBilling.Included*0.25:
		st = StyleWarning
	}
	chip := st.Render(fmt.Sprintf("%.0f", a.ghaBilling.Used)) +
		StyleMuted.Render(fmt.Sprintf("/%.0f min", a.ghaBilling.Included))
	if a.ghaBilling.DaysLeft > 0 {
		chip += StyleMuted.Render(fmt.Sprintf(" · %dd", a.ghaBilling.DaysLeft))
	}
	return chip
}

// renderGHAKindTabs mostra as teclas que trocam de aba (1/2/3 existem no
// handler) e à direita os contadores que antes eram seis caixas de 3 linhas.
func (a *App) renderGHAKindTabs(width int) string {
	counts := map[ghaKind]int{
		ghaKindProcesses: len(a.ghaProcesses),
		ghaKindRuns:      len(a.ghaRuns),
		ghaKindWorkflows: len(a.ghaWorkflows),
	}
	keys := map[ghaKind]string{ghaKindProcesses: "1", ghaKindRuns: "2", ghaKindWorkflows: "3"}

	parts := make([]string, 0, 3)
	for _, k := range []ghaKind{ghaKindProcesses, ghaKindRuns, ghaKindWorkflows} {
		label := fmt.Sprintf(" %s %s %d ", keys[k], strings.ToUpper(k.String()), counts[k])
		if k == a.ghaKind {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	running, ok, fail, other := a.ghaCounters()
	var chips []string
	add := func(st lipgloss.Style, glyph, label string, n int) {
		if n > 0 {
			chips = append(chips, st.Render(fmt.Sprintf("%s %d", glyph, n))+StyleMuted.Render(" "+label))
		}
	}
	add(StyleWarning, "●", "rodando", running)
	add(StyleHealthy, "✓", "ok", ok)
	add(StyleUnhealthy, "✕", "falha", fail)
	if other == 1 {
		add(StyleMuted, "⊘", "cancelado", other)
	} else {
		add(StyleMuted, "⊘", "cancelados", other)
	}
	if len(chips) == 0 {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, strings.Join(chips, "  ")+" ", width)
}

func (a *App) renderGHATable(width, height int) string {
	title := strings.ToUpper(a.ghaKind.String())
	n := a.ghaRowCount()
	if a.ghaKind == ghaKindRuns {
		total := len(a.ghaRuns)
		if n != total || a.ghaRunScope != ghaRunScopeAll || a.ghaRunProcFilter != "" {
			title = panelTitle("RUNS", fmt.Sprintf("%d/%d", n, total), a.ghaRunsFilterLabel())
		} else if n > 0 {
			title = panelTitle("RUNS", fmt.Sprint(n))
		}
	} else if n > 0 {
		title = panelTitle(title, fmt.Sprint(n))
	}
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	inner4 := maxInt(8, width-2)
	lines := []string{
		a.ghaTableHeader(inner4),
		rule(inner4),
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
		return panelBox(title, fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusTable)
	}
	a.ghaScroll = ensureVisible(a.ghaCursor, a.ghaScroll, viewport, n)
	for i := a.ghaScroll; i < minInt(a.ghaScroll+viewport, n); i++ {
		lines = append(lines, a.renderGHARow(i, inner4, i == a.ghaCursor && a.ghaFocus == ghaFocusTable))
	}
	return panelBox(title, fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusTable)
}

// ghaCols distribui as colunas da tabela pela largura útil. Antes o cabeçalho
// era uma string fixa e as linhas usavam %-Ns próprios — os dois discordavam.
type ghaCols struct {
	mark, result, name, file, event, branch, when, dur, path, title int
}

func (a *App) ghaColumns(width int) ghaCols {
	w := maxInt(30, width)
	switch a.ghaKind {
	case ghaKindProcesses:
		c := ghaCols{result: 11, name: minInt(22, maxInt(10, w*22/100)),
			file: minInt(26, maxInt(10, w*20/100)), event: 14}
		c.title = maxInt(0, w-c.result-c.name-c.file-c.event-4)
		if c.title < 12 {
			c.title = 0 // não cabe descrição legível: devolve o espaço ao arquivo
			c.file = maxInt(10, w-c.result-c.name-c.event-3)
		}
		return c
	case ghaKindWorkflows:
		c := ghaCols{name: minInt(28, maxInt(12, w*26/100)), event: 12}
		c.path = maxInt(12, w-c.name-c.event-2)
		return c
	default: // runs
		c := ghaCols{mark: 2, result: 11, name: minInt(16, maxInt(8, w*13/100)),
			branch: minInt(20, maxInt(8, w*15/100)), when: 6, dur: 6}
		c.title = maxInt(10, w-c.mark-c.result-c.name-c.branch-c.when-c.dur-6)
		return c
	}
}

func (a *App) ghaTableHeader(width int) string {
	c := a.ghaColumns(width)
	head := StyleMuted.Bold(true)
	cell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return head.Render(padRight(truncate(t, n), n))
	}
	switch a.ghaKind {
	case ghaKindProcesses:
		return joinNonEmpty(" ", cell("RESULTADO", c.result), cell("NOME", c.name),
			cell("ARQUIVO", c.file), cell("DESCRIÇÃO", c.title), cell("EVENTO", c.event))
	case ghaKindWorkflows:
		return joinNonEmpty(" ", cell("NOME", c.name), cell("ESTADO", c.event),
			cell("CAMINHO", c.path))
	default:
		return joinNonEmpty(" ", cell("", c.mark), cell("RESULTADO", c.result),
			cell("WORKFLOW", c.name), cell("BRANCH", c.branch), cell("TÍTULO", c.title),
			head.Render(padLeft("QUANDO", c.when)), head.Render(padLeft("DUR", c.dur)))
	}
}

func (a *App) renderGHARow(i, width int, selected bool) string {
	c := a.ghaColumns(width)
	plain := func(t string, n int) string { return padRight(truncate(t, n), n) }

	var cells []string
	var resultCell string
	switch a.ghaKind {
	case ghaKindProcesses:
		p := a.ghaProcesses[i]
		live := a.ghaStatusForProcess(p.Name, p.File)
		glyph, label, st := ghaProcResult(live)
		resultCell = st.Render(padRight(truncate(glyph+" "+label, c.result), c.result))
		cells = []string{plain(p.Name, c.name), plain(p.File, c.file),
			plain(firstNonEmpty(p.Description, live.Title, emDash), c.title),
			plain(firstNonEmpty(live.Event, emDash), c.event)}
	case ghaKindWorkflows:
		w := a.ghaWorkflows[i]
		cells = []string{plain(w.Name, c.name), plain(ghaWorkflowState(w.State), c.event),
			plain(w.Path, c.path)}
	default:
		runs := a.ghaFilteredRuns()
		if i < 0 || i >= len(runs) {
			return ""
		}
		r := runs[i]
		mark := " "
		if a.ghaRunMarked != nil && a.ghaRunMarked[r.ID] {
			mark = "✓"
		}
		if a.ghaNotes != nil && a.ghaNotes[r.ID] != "" {
			mark += "!"
		}
		resultCell = ghaResultCell(r.Status, r.Conclusion, c.result)
		cells = []string{plain(mark, c.mark), "", plain(r.Workflow, c.name),
			plain(r.Branch, c.branch), plain(firstNonEmpty(r.DisplayTitle, r.Name), c.title),
			padLeft(ghaRunWhen(r), c.when), padLeft(ghaRunDuration(r), c.dur)}
		cells[1] = "\x00" // marca a posição do chip de resultado
	}

	if a.ghaKind == ghaKindProcesses {
		cells = append([]string{"\x00"}, cells...)
	}

	// Selecionada: um fundo só na linha inteira, senão o realce fica serrilhado.
	out := make([]string, 0, len(cells))
	for _, cell := range cells {
		if cell == "\x00" {
			if selected {
				glyph, label, _ := ghaResultFor(a, i)
				out = append(out, StyleSelected.Render(padRight(truncate(glyph+" "+label, c.result), c.result)))
			} else {
				out = append(out, resultCell)
			}
			continue
		}
		if selected {
			out = append(out, StyleSelected.Render(cell))
		} else {
			out = append(out, StyleNormal.Render(cell))
		}
	}
	sep := " "
	if selected {
		sep = StyleSelected.Render(" ")
	}
	row := strings.Join(out, sep)
	if selected {
		return row + StyleSelected.Render(strings.Repeat(" ", maxInt(0, width-lipgloss.Width(row))))
	}
	return padRightVisible(row, width)
}

// ghaResultFor devolve o par (status, conclusion) da linha i já traduzido.
func ghaResultFor(a *App, i int) (string, string, lipgloss.Style) {
	if a.ghaKind == ghaKindProcesses {
		if i < len(a.ghaProcesses) {
			p := a.ghaProcesses[i]
			return ghaProcResult(a.ghaStatusForProcess(p.Name, p.File))
		}
		return ghaResult("", "")
	}
	runs := a.ghaFilteredRuns()
	if i < len(runs) {
		return ghaResult(runs[i].Status, runs[i].Conclusion)
	}
	return ghaResult("", "")
}

// ghaProcResult traduz o rótulo já resolvido do processo. "idle" virava
// "◌ —" ao passar pelo caminho genérico.
func ghaProcResult(live ghaProcLive) (string, string, lipgloss.Style) {
	switch live.Label {
	case "idle", "":
		return "◌", "ocioso", StyleMuted
	case "triggered":
		return "●", "disparado", StyleAccent
	case "running":
		return ghaResult("in_progress", "")
	case "queued":
		return ghaResult("queued", "")
	case "success", "failure", "cancelled":
		return ghaResult("completed", live.Label)
	}
	return ghaResult(live.Status, ghaProcConclusion(live))
}

// ghaProcConclusion: o rótulo do processo já vem resolvido; converte de volta
// para o par que ghaResult entende.
func ghaProcConclusion(live ghaProcLive) string {
	if live.Conclusion != "" {
		return live.Conclusion
	}
	switch live.Label {
	case "success", "failure", "cancelled":
		return live.Label
	}
	return ""
}

func ghaWorkflowState(state string) string {
	switch state {
	case "active":
		return "ativo"
	case "disabled_manually":
		return "desativado"
	case "disabled_inactivity":
		return "inativo"
	case "disabled_fork":
		return "fork"
	case "":
		return emDash
	default:
		return state
	}
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
		// Sem YAML a caixa encolhe para uma linha em vez de ficar oca.
		return panelBox(title,
			[]string{StyleMuted.Render("enter foca aqui  ·  ↑↓ rola o YAML do processo")},
			width, 3, a.ghaFocus == ghaFocusResumo)
	}
	raw := strings.Split(body, "\n")
	viewport := maxInt(1, inner)
	a.ghaDetailScroll = clampScroll(a.ghaDetailScroll, viewport, len(raw))
	end := minInt(a.ghaDetailScroll+viewport, len(raw))
	total := len(raw)
	if total > viewport {
		pos := a.ghaDetailScroll + 1
		maxPos := total - viewport + 1
		title = panelTitle(title, fmt.Sprintf("%d/%d", pos, maxPos))
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
	return panelBox(title, fitExactLines(lines, inner), width, height, focused)
}

// renderGHARightRail: CONTEXTO + USO + AÇÕES. A caixa RUNS RECENTES saiu —
// repetia, coluna ao lado, a mesma lista da tabela principal.
func (a *App) renderGHARightRail(width, height int) string {
	ctx := a.ghaContextLines(width - 2)
	ctxH := len(ctx) + 2
	usageH := maxInt(8, (height-ctxH)*55/100)
	actH := maxInt(5, height-ctxH-usageH)
	return lipgloss.JoinVertical(lipgloss.Left,
		panelBox("CONTEXTO", ctx, width, ctxH, false),
		a.renderGHAUsagePanel(width, usageH),
		a.renderGHAActionsPanel(width, actH),
	)
}

// ghaContextLines põe rótulo e valor na mesma linha — antes cada par gastava
// duas, e "Repository" aparecia sem valor.
func (a *App) ghaContextLines(width int) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 10)) }
	valW := maxInt(8, width-10)
	lines := []string{
		label("Catálogo") + StyleNormal.Render(elideLeft(".devscope/actions.yaml", valW)),
		label("Workflows") + StyleNormal.Render(elideLeft(".github/workflows/", valW)),
	}
	if a.ghaInfo.Owner != "" {
		lines = append(lines, label("Repo")+StyleNormal.Render(elideLeft(a.ghaInfo.Owner+"/"+a.ghaInfo.Repo, valW)))
	}
	return lines
}

func (a *App) renderGHAUsagePanel(width, height int) string {
	inner := maxInt(3, height-2)
	barW := maxInt(8, minInt(18, width-8))
	label := func(k string) string { return StyleMuted.Render(padRight(k, 10)) }
	var lines []string

	if a.ghaBilling.OK {
		pct := 0.0
		if a.ghaBilling.Included > 0 {
			pct = a.ghaBilling.Used * 100 / a.ghaBilling.Included
		}
		lines = append(lines,
			barSolid(pct, barW)+StyleMuted.Render(fmt.Sprintf("  %.0f%%", pct)),
			label("Restante")+StyleHealthy.Render(fmt.Sprintf("%.0fm", a.ghaBilling.Remaining))+
				StyleMuted.Render(fmt.Sprintf(" de %.0fm", a.ghaBilling.Included)),
			label("Usado")+StyleWarning.Render(fmt.Sprintf("%.0fm", a.ghaBilling.Used)),
			label("Conta")+StyleMuted.Render(a.ghaBilling.Source),
		)
		if a.ghaBilling.DaysLeft > 0 {
			lines = append(lines, label("Ciclo")+StyleNormal.Render(fmt.Sprintf("fecha em %dd", a.ghaBilling.DaysLeft)))
		}
	} else {
		lines = append(lines, StyleMuted.Render("cota da conta indisponível"))
		if a.ghaBilling.Error != "" {
			lines = append(lines, StyleMuted.Render(truncate(a.ghaBilling.Error, width-4)))
		}
	}

	projMin := collectors.GHAMinutesFromRuns(a.ghaRuns)
	lines = append(lines, "",
		label("Projeto")+StyleAccent.Render(collectors.FormatGHAMinutes(projMin)))

	// Minutos e falhas por processo na mesma linha — o card FAIL/WEEK mostrava
	// isso como barra Braille ilegível.
	fails := map[string]int{}
	for _, b := range collectors.GHAFailHeatmap(a.ghaRuns, 40) {
		fails[b.Process] = b.Fails
	}
	type row struct {
		name  string
		min   float64
		fails int
	}
	var rows []row
	if len(a.ghaProcesses) > 0 {
		for _, p := range a.ghaProcesses {
			rows = append(rows, row{p.Name, collectors.GHAMinutesFromRuns(a.ghaRunsForProcess(p.Name, p.File)), fails[p.Name]})
		}
	} else {
		for _, b := range collectors.GHABillingEstimate(a.ghaRuns, 40) {
			rows = append(rows, row{b.Workflow, b.Minutes, fails[b.Workflow]})
		}
	}
	if len(rows) == 0 {
		lines = append(lines, StyleMuted.Render("Processos (sem dados)"))
	} else {
		lines = append(lines, StyleMuted.Render("Por processo"))
		nameW := maxInt(6, width-18)
		for i := 0; i < minInt(len(rows), maxInt(2, inner-len(lines))); i++ {
			r := rows[i]
			line := StyleMuted.Render("· ") + StyleNormal.Render(padRight(truncate(r.name, nameW), nameW)) +
				StyleWarning.Render(padLeft(collectors.FormatGHAMinutes(r.min), 7))
			if r.fails > 0 {
				line += StyleUnhealthy.Render(fmt.Sprintf(" ✕%d", r.fails))
			}
			lines = append(lines, line)
		}
	}
	return panelBox("USO", fitExactLines(lines, inner), width, height, false)
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
	return panelBox("AÇÕES RÁPIDAS", fitExactLines(lines, inner), width, height, a.ghaFocus == ghaFocusActions)
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
	return panelBox(title, inner, w, len(inner)+2, true)
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
	return panelBox("CREATE PROCESS", inner, w, len(inner)+2, true)
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
		rule(maxInt(8, w-8)),
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
			rule(maxInt(8, w-8)),
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
	return panelBox("TRIGGER · BRANCH + INPUTS", inner, w, len(inner)+2, true)
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
	return panelBox(title, body, w, h, true)
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
	return panelBox(title, fitExactLines(lines, inner), w, h, true)
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
	return panelBox("DETAILS", fitExactLines(lines, inner), w, h, true)
}
