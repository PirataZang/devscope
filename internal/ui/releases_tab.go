package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Releases: lançar uma versão sem esquecer passo.
//
// À mão, release é uma lista que se esquece pela metade: conferir se tudo foi
// commitado e enviado, achar o próximo número, escrever o CHANGELOG, criar a
// tag anotada, enviar a tag, olhar o workflow publicar. Aqui a lista vira
// VERIFICAÇÕES — o que bloqueia não deixa lançar, o que só preocupa avisa — e
// o resto é uma tecla.
//
// A tag sai com a seção do CHANGELOG da versão como mensagem; sem seção, com a
// lista de commits. Depois do push a tela acompanha o workflow disparado pela
// tag até ele terminar.

type relLoadedMsg struct {
	path  string
	state collectors.ReleaseState
	runs  map[string]collectors.GHARun
}

type relTaggedMsg struct {
	path string
	tag  string
	err  error
}

type relRunsMsg struct {
	path string
	runs map[string]collectors.GHARun
}

type relPollMsg struct{ path string }

// relCheck é uma linha das VERIFICAÇÕES. block: falhar impede o lançamento.
type relCheck struct {
	ok, block bool
	text      string
}

func (a *App) loadRelease(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	if a.relPath != path {
		a.relPath, a.relState, a.relRuns, a.relWatch, a.relBumpSet = path, collectors.ReleaseState{}, nil, "", false
	}
	a.relLoading = true
	return func() tea.Msg {
		st := collectors.CollectRelease(context.Background(), path)
		return relLoadedMsg{path: path, state: st, runs: collectors.ReleaseRuns(path, releaseTagNames(st))}
	}
}

func releaseTagNames(st collectors.ReleaseState) []string {
	var names []string
	if st.Last.Name != "" {
		names = append(names, st.Last.Name)
	}
	for _, t := range st.Previous {
		names = append(names, t.Name)
	}
	return names
}

func (a *App) handleReleaseLoaded(msg relLoadedMsg) {
	if msg.path != a.relPath {
		return
	}
	a.relLoading = false
	a.relState = msg.state
	a.relRuns = msg.runs
	if !a.relBumpSet {
		a.relBump = suggestedBump(msg.state)
		a.relBumpSet = true
	}
}

// suggestedBump: se o CHANGELOG já tem a seção de uma das próximas versões, é
// ela — foi o autor quem decidiu. Senão, patch, o passo mais cauteloso.
func suggestedBump(st collectors.ReleaseState) int {
	if st.Changelog != "" {
		next := collectors.NextVersions(st.Last.Name)
		for i := len(next) - 1; i >= 0; i-- {
			if collectors.ChangelogSection(st.Changelog, next[i]) != "" {
				return i
			}
		}
	}
	return 0
}

func (a *App) relNext() string {
	return collectors.NextVersions(a.relState.Last.Name)[a.relBump]
}

func releaseChecks(st collectors.ReleaseState, next string) []relCheck {
	var out []relCheck
	pend := st.Pending
	if pend == nil {
		pend = &core.GitPending{}
	}
	if pend.Changes == 0 {
		out = append(out, relCheck{ok: true, text: "árvore limpa"})
	} else {
		out = append(out, relCheck{block: true, text: countOf(pend.Changes, "alteração", "alterações") + " sem commit — commite ou guarde antes"})
	}
	switch {
	case !pend.Upstream:
		out = append(out, relCheck{block: true, text: "branch sem upstream — envie a branch antes (shift+P no Git)"})
	case pend.Ahead > 0:
		out = append(out, relCheck{block: true, text: countOf(pend.Ahead, "commit não enviado", "commits não enviados") + " — a tag apontaria para algo que o remoto não tem"})
	default:
		out = append(out, relCheck{ok: true, text: "tudo enviado"})
	}
	if pend.Behind > 0 {
		out = append(out, relCheck{text: countOf(pend.Behind, "commit", "commits") + " no remoto que você não tem — faça pull antes"})
	}
	if st.Default != "" && st.Branch != st.Default {
		out = append(out, relCheck{text: "na branch " + st.Branch + ", não na " + st.Default})
	} else if st.Branch != "" {
		out = append(out, relCheck{ok: true, text: "na branch " + st.Branch})
	}
	// Quantos commits entram já está na linha "última"; aqui só o caso que
	// impede lançar.
	if st.Last.Name != "" && st.Count == 0 {
		out = append(out, relCheck{block: true, text: "nada novo desde " + st.Last.Name})
	}
	if st.Changelog != "" {
		if collectors.ChangelogSection(st.Changelog, next) != "" {
			out = append(out, relCheck{ok: true, text: "CHANGELOG tem a seção " + strings.TrimPrefix(next, "v")})
		} else {
			out = append(out, relCheck{text: "CHANGELOG sem a seção " + strings.TrimPrefix(next, "v") + " — a tag vai com a lista de commits"})
		}
	}
	if st.Workflow != "" {
		out = append(out, relCheck{ok: true, text: st.Workflow + " publica a release quando a tag chega"})
	} else {
		out = append(out, relCheck{text: "nenhum workflow roda em tag — a tag só marca o commit"})
	}
	return out
}

func firstBlock(checks []relCheck) string {
	for _, c := range checks {
		if c.block {
			return c.text
		}
	}
	return ""
}

func (a *App) handleReleaseKeys(msg tea.KeyMsg, p *core.Project) (tea.Cmd, bool) {
	switch msg.String() {
	case "left", "h":
		a.relBump = maxInt(0, a.relBump-1)
	case "right", "l":
		a.relBump = minInt(2, a.relBump+1)
	case "r":
		a.relStatus = ""
		return a.loadRelease(p), true
	case "enter":
		if a.relLoading {
			return nil, true
		}
		if b := firstBlock(releaseChecks(a.relState, a.relNext())); b != "" {
			a.relStatus = "não dá para lançar: " + b
			return nil, true
		}
		a.gitConfirmOn = true
		a.gitConfirmAction = "release"
		a.gitConfirmBranch = a.relNext()
	case "o":
		a.openReleasePage(p)
	default:
		return nil, false
	}
	return nil, true
}

// openReleasePage abre o run da release mais recente; sem run, a página de
// releases do repositório.
func (a *App) openReleasePage(p *core.Project) {
	tag := firstNonEmpty(a.relWatch, a.relState.Last.Name)
	if r, ok := a.relRuns[tag]; ok && r.URL != "" {
		openBrowser(r.URL)
		a.relStatus = "abrindo o workflow de " + tag
		return
	}
	remote := ""
	if p.Git != nil {
		remote = p.Git.Remote
	}
	if owner, repo, ok := collectors.ParseGitHubRepo(firstNonEmpty(remote, collectors.GitRemoteOrigin(p.Path))); ok {
		openBrowser("https://github.com/" + owner + "/" + repo + "/releases")
		a.relStatus = "abrindo as releases no GitHub"
		return
	}
	a.relStatus = "remoto GitHub não detectado"
}

// releaseTag cria e envia a tag. A mensagem é a seção do CHANGELOG; sem ela,
// a lista de commits — nunca uma tag anotada vazia.
func (a *App) releaseTag(p *core.Project, tag string) tea.Cmd {
	st, path := a.relState, p.Path
	msg := ""
	if st.Changelog != "" {
		msg = collectors.ChangelogSection(st.Changelog, tag)
	}
	if msg == "" {
		msg = "Release " + tag + "\n\n" + strings.Join(st.Commits, "\n")
	}
	a.relStatus = "criando e enviando " + tag + "…"
	a.relLoading = true
	return func() tea.Msg {
		_, err := collectors.CreateReleaseTag(path, tag, msg)
		return relTaggedMsg{path: path, tag: tag, err: err}
	}
}

func (a *App) handleReleaseTagged(msg relTaggedMsg) tea.Cmd {
	a.relLoading = false
	if msg.err != nil {
		a.relStatus = "erro: " + lastLine(msg.err.Error())
		return nil
	}
	a.relStatus = msg.tag + " enviada"
	a.relBumpSet = false
	p := a.currentProject()
	if p == nil || !pathsMatch(p.Path, msg.path) {
		return nil
	}
	if a.relState.Workflow != "" && collectors.GHAAvailable() {
		a.relWatch = msg.tag
		a.relStatus = msg.tag + " enviada — acompanhando " + a.relState.Workflow
		return tea.Batch(a.loadRelease(p), relPoll(msg.path))
	}
	return a.loadRelease(p)
}

// relPoll: o workflow leva minutos; olhar a cada 5s é fresco o bastante e não
// estoura o limite da API do gh.
func relPoll(path string) tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return relPollMsg{path: path} })
}

func (a *App) handleReleasePoll(msg relPollMsg) tea.Cmd {
	if a.relWatch == "" || msg.path != a.relPath || a.tab != TabReleases {
		return nil // saiu da tela: para de acompanhar, `r` retoma o estado
	}
	tags := append([]string{a.relWatch}, releaseTagNames(a.relState)...)
	path := msg.path
	return func() tea.Msg { return relRunsMsg{path: path, runs: collectors.ReleaseRuns(path, tags)} }
}

func (a *App) handleReleaseRuns(msg relRunsMsg) tea.Cmd {
	if msg.path != a.relPath {
		return nil
	}
	if msg.runs != nil {
		a.relRuns = msg.runs
	}
	if r, ok := a.relRuns[a.relWatch]; ok && r.Status == "completed" {
		if r.Conclusion == "success" {
			a.relStatus = a.relWatch + " publicada"
		} else {
			a.relStatus = a.relWatch + ": workflow terminou com " + r.Conclusion + " — o abre o log"
		}
		a.relWatch = ""
		return nil
	}
	return relPoll(msg.path)
}

// ─── tela ────────────────────────────────────────────────────────────────────

func (a *App) renderReleasesTab(p *core.Project) string {
	w, h := a.moduleSize()
	st := a.relState
	next := a.relNext()
	checks := releaseChecks(st, next)
	head := []string{a.renderModuleContext(p, w, "Releases", a.relStatusText())}

	launch := [2]string{"enter", "lançar " + next}
	if firstBlock(checks) != "" {
		launch = [2]string{"enter", "ver o que falta"}
	}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		launch,
		[2]string{"←→", "versão"},
		[2]string{"o", "abrir no GitHub"},
		[2]string{"r", "atualizar"},
	))

	if a.relLoading && st.Branch == "" {
		body := []string{"", "  " + a.loadingText("lendo tags e commits…")}
		return a.fillReleaseScreen(append(head, body...), cmdBar, h)
	}

	lines := append(head, "")
	lines = append(lines, factLine("última", a.releaseLastFacts(), w))
	lines = append(lines, factLine("próxima", a.releaseBumpFacts(), w))
	lines = append(lines, "", sectionHeader("VERIFICAÇÕES", w, tabAccentColor(TabReleases)))
	for _, c := range checks {
		lines = append(lines, "  "+releaseCheckLine(c, w-2))
	}

	history := a.releaseHistoryLines(w)
	room := h - len(lines) - lipgloss.Height(cmdBar) - 1
	if len(history) > 0 {
		room -= len(history) + 2
	}
	if room >= 3 && len(st.Commits) > 0 {
		title := panelTitle("COMMITS DESDE "+firstNonEmpty(st.Last.Name, "O INÍCIO"), strconv.Itoa(st.Count))
		lines = append(lines, "", sectionHeader(title, w, ColorMuted))
		for i, c := range st.Commits {
			if i >= room-1 {
				lines = append(lines, StyleMuted.Render("  +"+strconv.Itoa(st.Count-i)))
				break
			}
			hash, subject, _ := strings.Cut(c, " ")
			lines = append(lines, "  "+StyleMuted.Render(hash)+" "+StyleNormal.Render(truncate(subject, maxInt(10, w-12))))
		}
	}
	if len(history) > 0 && h-len(lines)-lipgloss.Height(cmdBar) >= len(history)+2 {
		lines = append(lines, "", sectionHeader("RELEASES", w, ColorMuted))
		lines = append(lines, history...)
	}
	return a.fillReleaseScreen(lines, cmdBar, h)
}

func (a *App) fillReleaseScreen(lines []string, cmdBar string, h int) string {
	stack := strings.Join(lines, "\n")
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)
}

func (a *App) relStatusText() string {
	if a.relStatus == "" {
		return ""
	}
	if strings.HasPrefix(a.relStatus, "erro") || strings.HasPrefix(a.relStatus, "não dá") {
		return StyleUnhealthy.Render(a.relStatus)
	}
	return StyleWarning.Render(a.relStatus)
}

func (a *App) releaseLastFacts() []string {
	st := a.relState
	if st.Last.Name == "" {
		return []string{StyleMuted.Render("nenhuma release ainda — a primeira sai de " + countOf(st.Count, "commit", "commits"))}
	}
	out := []string{StyleNormal.Bold(true).Render(st.Last.Name)}
	if !st.Last.Date.IsZero() {
		out = append(out, StyleMuted.Render("há "+relTime(st.Last.Date)))
	}
	if st.Count > 0 {
		out = append(out, StyleWarning.Render(countOf(st.Count, "commit", "commits")+" desde então"))
	} else {
		out = append(out, StyleHealthy.Render("nada novo desde então"))
	}
	return out
}

// releaseBumpFacts: as três opções lado a lado, a escolhida em destaque — o
// número que vai virar tag é o que o olho tem de achar primeiro.
func (a *App) releaseBumpFacts() []string {
	next := collectors.NextVersions(a.relState.Last.Name)
	labels := [3]string{"patch", "minor", "major"}
	out := make([]string, 3)
	for i := range next {
		text := labels[i] + " " + next[i]
		if i == a.relBump {
			out[i] = StyleSelected.Bold(true).Render(" " + text + " ")
		} else {
			out[i] = StyleMuted.Render(text)
		}
	}
	return out
}

func releaseCheckLine(c relCheck, width int) string {
	switch {
	case c.ok:
		return StyleHealthy.Render("◆ ") + StyleNormal.Render(truncate(c.text, width-2))
	case c.block:
		return StyleUnhealthy.Bold(true).Render("⚠ " + truncate(c.text, width-2))
	}
	return StyleWarning.Render("◌ " + truncate(c.text, width-2))
}

// releaseHistoryLines: a release em andamento (se houver) e as anteriores,
// cada uma com o estado do workflow que a publicou.
func (a *App) releaseHistoryLines(width int) []string {
	type item struct {
		name string
		date time.Time
	}
	var items []item
	if a.relWatch != "" && a.relWatch != a.relState.Last.Name {
		items = append(items, item{name: a.relWatch})
	}
	if a.relState.Last.Name != "" {
		items = append(items, item{a.relState.Last.Name, a.relState.Last.Date})
	}
	for _, t := range a.relState.Previous {
		items = append(items, item{t.Name, t.Date})
	}
	if len(items) <= 1 && len(a.relRuns) == 0 {
		return nil // só a última, sem workflow: já está na linha "última"
	}
	var out []string
	for _, it := range items {
		glyph, st, label := "·", StyleMuted, emDash
		if r, ok := a.relRuns[it.name]; ok {
			glyph, st, label = a.runVisual(r)
		} else if it.name == a.relWatch {
			glyph, st, label = a.spinner(), StyleWarning, "esperando o workflow começar…"
		}
		when := emDash
		if !it.date.IsZero() {
			when = "há " + relTime(it.date)
		}
		out = append(out, "  "+st.Render(glyph)+" "+
			StyleNormal.Bold(true).Render(padRight(it.name, 10))+" "+
			StyleMuted.Render(padRight(when, 9))+" "+st.Render(truncate(label, maxInt(10, width-26))))
	}
	return out
}

// runVisual: publicada, falhou e rodando com glifos distintos (§1).
func (a *App) runVisual(r collectors.GHARun) (string, lipgloss.Style, string) {
	switch {
	case r.Status != "completed":
		return a.spinner(), StyleWarning, r.Workflow + " rodando…"
	case r.Conclusion == "success":
		return "◆", StyleHealthy, "publicada · " + r.Workflow
	}
	return "⚠", StyleUnhealthy, r.Workflow + " " + r.Conclusion
}

func (a *App) releaseConfirmOpts(tag string) deleteConfirmOpts {
	st := a.relState
	detail := countOf(st.Count, "commit entra", "commits entram")
	if st.Workflow != "" {
		detail += " · " + st.Workflow + " publica depois do push"
	}
	warn := 0
	for _, c := range releaseChecks(st, tag) {
		if !c.ok && !c.block {
			warn++
		}
	}
	if warn > 0 {
		detail += " · " + countOf(warn, "aviso", "avisos")
	}
	return deleteConfirmOpts{
		Brand:    "RELEASES",
		Color:    tabAccentColor(TabReleases),
		Title:    "Lançar " + tag,
		Subtitle: "tag anotada no HEAD + push para o origin",
		Label:    "tag",
		Target:   tag,
		Detail:   detail,
	}
}
