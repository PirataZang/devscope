package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/nginxutil"
)

// "Publicar projeto" (p na aba Nginx): uma tela só — o que foi detectado, as
// checagens, a .inc pronta — e enter publica. Sem wizard: o que dá para
// descobrir não é perguntado; só a ambiguidade de verdade pede escolha.

type nginxPublish struct {
	hub      nginxutil.Hub
	plan     nginxutil.Plan
	checks   []nginxutil.Check
	existing string // .inc publicada hoje; "" = nova
	loading  bool

	editing bool
	field   int // índice em nginxutil.PlanFields
	text    string
	cursor  int
	editErr string

	confirm bool // sobrescrever a .inc existente

	running bool
	steps   []nginxutil.Step
	result  *nginxutil.Result
	events  chan tea.Msg

	conflict     string // .inc em conflito aberta no lugar da prévia
	conflictText string
	scroll       int
	status       string

	picker    bool // aberto no próprio nginx-main: qual projeto publicar?
	choices   []core.Project
	published map[string]bool // caminho → já tem sites/<slug>.inc
	pick      int
}

type nginxPubLoadedMsg struct {
	plan     nginxutil.Plan
	checks   []nginxutil.Check
	existing string
}

type nginxPubStepMsg struct{ step nginxutil.Step }

type nginxPubDoneMsg struct{ res nginxutil.Result }

var nginxFieldLabels = map[string]string{
	"slug": "slug", "perfil": "perfil", "porta": "porta", "dist": "dist",
	"body": "body", "timeout": "timeout", "extras": "extras",
}

// nginxPubInfo é o que a tela de abertura e a barra lateral precisam saber do
// "Publicar projeto" — medido fora do render.
type nginxPubInfo struct {
	isHub       bool // o projeto É o nginx-main
	publishable bool
	published   bool
	url         string
}

func (i nginxPubInfo) relevant() bool { return i.isHub || i.publishable }

func probeNginxPublish(path string, prefs config.NginxPrefs) nginxPubInfo {
	hub := nginxutil.LoadHub(prefs)
	switch {
	case path == "" || !hub.Available():
		return nginxPubInfo{}
	case pathsMatch(path, hub.Dir):
		return nginxPubInfo{isHub: true}
	}
	if _, ok := hub.ContainerPath(path); !ok {
		return nginxPubInfo{}
	}
	slug := nginxutil.Slug(path)
	return nginxPubInfo{publishable: true, published: fileExists(hub.IncPath(slug)), url: hub.PublicURL + "/" + slug + "/"}
}

func (a *App) nginxPrefs() config.NginxPrefs {
	if a.cfg == nil {
		return config.NginxPrefs{}
	}
	return a.cfg.Nginx
}

// startNginxPublish: no projeto, revisa a publicação dele; no próprio
// nginx-main, pergunta qual projeto.
func (a *App) startNginxPublish(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	hub := nginxutil.LoadHub(a.nginxPrefs())
	if !pathsMatch(p.Path, hub.Dir) {
		return a.beginNginxPublish(hub, p.Path)
	}
	var choices []core.Project
	published := map[string]bool{}
	for _, sp := range a.snapshot.Projects {
		if _, ok := hub.ContainerPath(sp.Path); ok && !pathsMatch(sp.Path, hub.Dir) {
			choices = append(choices, sp)
			published[sp.Path] = fileExists(hub.IncPath(nginxutil.Slug(sp.Path)))
		}
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })
	a.nginxPub = &nginxPublish{hub: hub, picker: true, choices: choices, published: published}
	return nil
}

func (a *App) beginNginxPublish(hub nginxutil.Hub, dir string) tea.Cmd {
	a.nginxPub = &nginxPublish{hub: hub, loading: true}
	return func() tea.Msg {
		plan, checks := nginxutil.DetectProject(dir, hub)
		existing, _ := os.ReadFile(hub.IncPath(plan.Slug))
		return nginxPubLoadedMsg{plan: plan, checks: checks, existing: string(existing)}
	}
}

func waitNginxPub(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (a *App) handleNginxPublishMsg(msg tea.Msg) tea.Cmd {
	pub := a.nginxPub
	if pub == nil {
		return nil
	}
	switch m := msg.(type) {
	case nginxPubLoadedMsg:
		pub.loading = false
		pub.plan, pub.checks, pub.existing = m.plan, m.checks, m.existing
	case nginxPubStepMsg:
		if m.step.Index < len(pub.steps) {
			pub.steps[m.step.Index] = m.step
		}
		return waitNginxPub(pub.events)
	case nginxPubDoneMsg:
		pub.running = false
		pub.result = &m.res
		b, _ := os.ReadFile(pub.hub.IncPath(pub.plan.Slug))
		pub.existing = string(b)
		if m.res.TestLine > 0 {
			pub.scroll = maxInt(0, m.res.TestLine-3)
		}
	}
	return nil
}

func (a *App) runNginxPublish() tea.Cmd {
	pub := a.nginxPub
	pub.confirm = false
	pub.running = true
	pub.result = nil
	pub.status = ""
	pub.steps = nil
	for i, name := range nginxutil.PublishSteps(pub.plan) {
		pub.steps = append(pub.steps, nginxutil.Step{Index: i, Name: name, State: -1})
	}
	ch := make(chan tea.Msg, len(pub.steps)*2+1)
	pub.events = ch
	hub, plan := pub.hub, pub.plan
	go func() {
		res := nginxutil.Publish(hub, plan, func(s nginxutil.Step) { ch <- nginxPubStepMsg{s} })
		ch <- nginxPubDoneMsg{res}
	}()
	return waitNginxPub(ch)
}

func (a *App) recheckNginxPublish() {
	pub := a.nginxPub
	pub.checks = nginxutil.Checks(pub.plan, pub.hub)
	b, _ := os.ReadFile(pub.hub.IncPath(pub.plan.Slug))
	pub.existing = string(b)
	pub.result = nil
}

// ─── teclas ─────────────────────────────────────────────────────────────────

func (a *App) handleNginxPublishKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	pub := a.nginxPub
	key := msg.String()
	switch {
	case pub.picker:
		return a, a.handleNginxPickerKey(key)
	case pub.running:
		return a, nil
	case pub.confirm:
		switch key {
		case "y", "Y":
			return a, a.runNginxPublish()
		case "n", "N", "esc":
			pub.confirm = false
		}
		return a, nil
	case pub.editing:
		a.handleNginxEditKey(msg)
		return a, nil
	}

	switch key {
	case "esc":
		if pub.conflict != "" {
			pub.conflict = ""
			return a, nil
		}
		a.nginxPub = nil
	case "up", "k":
		pub.scroll = maxInt(0, pub.scroll-1)
	case "down", "j":
		pub.scroll++
	case "e":
		a.beginNginxEdit(a.firstNginxProblemField())
	case "+":
		a.beginNginxEdit(indexOf(nginxutil.PlanFields, "extras"))
		if pub.text != "" {
			pub.text += " "
		}
		pub.text += "/"
		pub.cursor = len([]rune(pub.text))
	case "r":
		if p != nil {
			dir := pub.plan.Dir
			return a, a.beginNginxPublish(nginxutil.LoadHub(a.nginxPrefs()), firstNonEmpty(dir, p.Path))
		}
	case "o":
		if pub.result != nil && pub.result.Err == nil {
			openBrowser(pub.result.URL)
			return a, nil
		}
		for _, c := range pub.checks {
			if c.File != "" {
				if pub.conflict == c.File {
					pub.conflict = ""
				} else {
					b, _ := os.ReadFile(c.File)
					pub.conflict, pub.conflictText, pub.scroll = c.File, string(b), 0
				}
				return a, nil
			}
		}
	case "c":
		if pub.result != nil && pub.result.URL != "" {
			// Na VPS, via SSH, não há xclip: o OSC52 pede ao terminal local.
			if err := copyToClipboard(pub.result.URL); err != nil {
				_, _ = osc52.New(pub.result.URL).WriteTo(os.Stderr)
			}
			pub.status = "URL copiada"
		}
	case "enter":
		if pub.loading {
			return a, nil
		}
		if c, blocked := nginxutil.Blocked(pub.checks); blocked {
			pub.status = c.Text
			return a, nil
		}
		if pub.existing != "" && pub.existing != nginxutil.Render(pub.plan, pub.hub) {
			pub.confirm = true
			return a, nil
		}
		return a, a.runNginxPublish()
	}
	return a, nil
}

func (a *App) handleNginxPickerKey(key string) tea.Cmd {
	pub := a.nginxPub
	switch key {
	case "esc":
		a.nginxPub = nil
	case "up", "k":
		pub.pick = maxInt(0, pub.pick-1)
	case "down", "j":
		pub.pick = minInt(maxInt(0, len(pub.choices)-1), pub.pick+1)
	case "enter":
		if pub.pick < len(pub.choices) {
			return a.beginNginxPublish(pub.hub, pub.choices[pub.pick].Path)
		}
	}
	return nil
}

// firstNginxProblemField: o e cai direto no campo que está pedindo escolha.
func (a *App) firstNginxProblemField() int {
	plan := a.nginxPub.plan
	switch {
	case plan.Profile.HasAPI() && plan.Port == 0:
		return indexOf(nginxutil.PlanFields, "porta")
	case plan.Profile.HasFront() && plan.Dist == "":
		return indexOf(nginxutil.PlanFields, "dist")
	}
	return 0
}

func (a *App) beginNginxEdit(field int) {
	pub := a.nginxPub
	pub.editing, pub.field, pub.editErr = true, field, ""
	pub.text = pub.plan.Get(nginxutil.PlanFields[field])
	pub.cursor = len([]rune(pub.text))
}

// handleNginxEditKey: tab troca de campo (aplicando o atual), ↑↓ percorre as
// opções do campo (perfis, portas e builds candidatos), enter aplica e sai.
func (a *App) handleNginxEditKey(msg tea.KeyMsg) {
	pub := a.nginxPub
	field := nginxutil.PlanFields[pub.field]
	switch msg.String() {
	case "esc":
		pub.editing, pub.editErr = false, ""
		return
	case "enter", "tab", "shift+tab":
		if err := pub.plan.Set(field, pub.text); err != nil {
			pub.editErr = err.Error()
			return
		}
		a.recheckNginxPublish()
		switch msg.String() {
		case "enter":
			pub.editing, pub.editErr = false, ""
		case "tab":
			a.beginNginxEdit((pub.field + 1) % len(nginxutil.PlanFields))
		default:
			a.beginNginxEdit((pub.field + len(nginxutil.PlanFields) - 1) % len(nginxutil.PlanFields))
		}
		return
	case "up", "down":
		opts := nginxFieldOptions(pub.plan, field)
		if len(opts) == 0 {
			return
		}
		i := indexOf(opts, pub.text)
		if msg.String() == "down" {
			i = (i + 1) % len(opts)
		} else {
			i = (i - 1 + len(opts)) % len(opts)
		}
		pub.text = opts[i]
		pub.cursor = len([]rune(pub.text))
		return
	}
	state := editorState{Cursor: pub.cursor, Anchor: -1}
	if text, handled := editorApplyKey(msg, pub.text, &state, false); handled {
		pub.text, pub.cursor, pub.editErr = text, state.Cursor, ""
	}
}

func nginxFieldOptions(plan nginxutil.Plan, field string) []string {
	var out []string
	switch field {
	case "perfil":
		for _, pr := range nginxutil.Profiles {
			out = append(out, string(pr))
		}
	case "porta":
		for _, c := range plan.PortChoices {
			out = append(out, c.Value)
		}
	case "dist":
		for _, c := range plan.DistChoices {
			out = append(out, c.Value)
		}
	}
	return out
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// ─── tela ───────────────────────────────────────────────────────────────────

func (a *App) renderNginxPublish(p *core.Project) string {
	w, h := a.moduleSize()
	pub := a.nginxPub
	accent := tabAccentColor(TabNginx)
	head := []string{a.renderModuleContext(p, w, "Nginx", a.nginxPubStatus()), ""}

	if pub.picker {
		return a.renderNginxPicker(head, w, h)
	}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2, a.nginxPubKeys()...))
	if pub.loading {
		return a.fillReleaseScreen(append(head, "  "+a.loadingText("detectando porta, build e conflitos…")), cmdBar, h)
	}

	plan := pub.plan
	lines := append(head, sectionHeader(panelTitle("PUBLICAR", plan.Slug), w, accent))
	if pub.editing {
		lines = append(lines, a.nginxEditLines(w)...)
	} else {
		lines = append(lines, a.nginxPlanFacts(w)...)
	}

	lines = append(lines, "", sectionHeader("CHECAGENS", w, ColorMuted))
	for _, c := range pub.checks {
		lines = append(lines, "  "+nginxCheckLine(c, w-2))
	}

	if len(pub.steps) > 0 {
		lines = append(lines, "", sectionHeader("PUBLICAÇÃO", w, ColorMuted))
		for _, s := range pub.steps {
			lines = append(lines, "  "+a.nginxStepLine(s, w-2))
		}
		lines = append(lines, a.nginxResultLines(w)...)
	}

	room := h - len(lines) - lipgloss.Height(cmdBar) - 2
	if room >= 3 {
		lines = append(lines, "", a.nginxIncTitle(w))
		lines = append(lines, a.nginxIncLines(w, room)...)
	}
	view := a.fillReleaseScreen(lines, cmdBar, h)
	if pub.confirm {
		box := renderDeleteConfirmBox(deleteConfirmOpts{
			Brand:    "NGINX",
			Color:    accent,
			Title:    "Republicar " + plan.Slug,
			Subtitle: "nginx -t antes do reload; recusou, desfaz",
			Label:    "sobrescreve",
			Target:   "sites/" + plan.Slug + ".inc",
			Detail:   "só ela — a anterior fica em .inc.bak",
		}, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) nginxPubStatus() string {
	pub := a.nginxPub
	switch {
	case pub.status != "":
		return pub.status
	case pub.running:
		return "publicando…"
	}
	return ""
}

func (a *App) nginxPubKeys() [][2]string {
	pub := a.nginxPub
	switch {
	case pub.running:
		return [][2]string{{"", "publicando…"}}
	case pub.editing:
		return [][2]string{{"tab", "campo"}, {"↑↓", "opções"}, {"enter", "aplica"}, {"esc", "cancela"}}
	case pub.result != nil && pub.result.Err == nil:
		return [][2]string{{"c", "copiar URL"}, {"o", "abrir"}, {"r", "detectar de novo"}, {"esc", "voltar"}}
	}
	keys := [][2]string{{"enter", "publicar"}, {"e", "editar"}, {"+", "rota extra"}}
	for _, c := range pub.checks {
		if c.File != "" {
			keys = append(keys, [2]string{"o", "abrir a existente"})
			break
		}
	}
	return append(keys, [2]string{"↑↓", "rolar .inc"}, [2]string{"r", "detectar de novo"}, [2]string{"esc", "voltar"})
}

func (a *App) nginxPlanFacts(w int) []string {
	pub := a.nginxPub
	plan := pub.plan
	lines := []string{factLine("url", []string{StyleAccent.Render(plan.URL(pub.hub))}, w)}
	profile := map[nginxutil.Profile]string{
		nginxutil.ProfileSaaS:  "back-end + front",
		nginxutil.ProfileFront: "só front",
		nginxutil.ProfileAPI:   "só back-end",
	}[plan.Profile]
	lines = append(lines, factLine("perfil", []string{StyleNormal.Render(string(plan.Profile)), StyleMuted.Render(profile)}, w))
	if plan.Profile.HasAPI() {
		lines = append(lines, factLine("porta", sourcedFact(plan.Get("porta"), plan.PortFrom, len(plan.PortChoices) > 1), w))
	}
	if plan.Profile.HasFront() {
		lines = append(lines, factLine("dist", sourcedFact(plan.Dist, plan.DistFrom, len(plan.DistChoices) > 1), w))
	}
	file := []string{StyleNormal.Render("sites/" + plan.Slug + ".inc")}
	switch {
	case pub.existing == "":
		file = append(file, StyleMuted.Render("nova"))
	case pub.existing == nginxutil.Render(plan, pub.hub):
		file = append(file, StyleMuted.Render("já publicada, igual"))
	default:
		file = append(file, StyleWarning.Render("já existe — enter sobrescreve só ela"))
	}
	return append(lines, factLine("arquivo", file, w))
}

func sourcedFact(value, from string, ambiguous bool) []string {
	if value == "" {
		if ambiguous {
			return []string{StyleUnhealthy.Render("escolha com e")}
		}
		return []string{StyleUnhealthy.Render("não detectada")}
	}
	style := StyleMuted
	if from == "build pendente" {
		style = StyleWarning
	}
	return []string{StyleNormal.Bold(true).Render(value), style.Render(firstNonEmpty(from, "editado"))}
}

func (a *App) nginxEditLines(w int) []string {
	pub := a.nginxPub
	var lines []string
	for i, field := range nginxutil.PlanFields {
		value := pub.plan.Get(field)
		if i == pub.field {
			r := []rune(pub.text)
			cur := clampInt(pub.cursor, 0, len(r))
			shown := StyleSelected.Render(string(r[:cur]) + "█" + string(r[cur:]))
			hint := ""
			if opts := nginxFieldOptions(pub.plan, field); len(opts) > 1 {
				hint = StyleMuted.Render("  ↑↓ " + strings.Join(opts, " · "))
			} else if field == "extras" {
				hint = StyleMuted.Render("  fora do prefixo, redireciona para dentro: /portal/")
			}
			lines = append(lines, truncateVisible(StyleAccent.Render("▸ ")+StyleMuted.Render(padRight(nginxFieldLabels[field], factLabelW))+" "+shown+hint, w))
			continue
		}
		lines = append(lines, factLine(nginxFieldLabels[field], []string{StyleNormal.Render(firstNonEmpty(value, emDash))}, w))
	}
	if pub.editErr != "" {
		lines = append(lines, "  "+StyleUnhealthy.Render(pub.editErr))
	}
	return lines
}

func nginxCheckLine(c nginxutil.Check, width int) string {
	switch c.Level {
	case nginxutil.CheckFail:
		return StyleUnhealthy.Bold(true).Render("✗ " + truncate(c.Text, width-2))
	case nginxutil.CheckWarn:
		return StyleWarning.Render("⚠ " + truncate(c.Text, width-2))
	}
	return StyleHealthy.Render("✓ ") + StyleNormal.Render(truncate(c.Text, width-2))
}

func (a *App) nginxStepLine(s nginxutil.Step, width int) string {
	switch s.State {
	case nginxutil.StepOK:
		return StyleHealthy.Render("✓ ") + StyleNormal.Render(s.Name)
	case nginxutil.StepFail:
		return StyleUnhealthy.Render("✗ "+s.Name) + StyleMuted.Render("  "+truncate(s.Detail, maxInt(10, width-lipgloss.Width(s.Name)-4)))
	case nginxutil.StepRunning:
		return a.loadingText(s.Name)
	}
	return StyleMuted.Render("◌ " + s.Name)
}

// nginxResultLines é o card final: onde está no ar, ou o que deu errado.
func (a *App) nginxResultLines(w int) []string {
	res := a.nginxPub.result
	if res == nil {
		return nil
	}
	if res.Err == nil {
		return []string{"", factLine("no ar", []string{StyleHealthy.Bold(true).Render(res.URL), StyleMuted.Render("c copia · o abre")}, w)}
	}
	lines := []string{"", "  " + StyleUnhealthy.Bold(true).Render(truncate(res.Err.Error(), w-2))}
	if res.TestLine > 0 {
		lines = append(lines, "  "+StyleMuted.Render("nginx -t apontou a linha "+strconv.Itoa(res.TestLine)+" — destacada abaixo"))
	}
	return lines
}

func (a *App) nginxIncTitle(w int) string {
	pub := a.nginxPub
	if pub.conflict != "" {
		return sectionHeader(panelTitle("sites/"+filepath.Base(pub.conflict), "em conflito · o volta"), w, ColorWarning)
	}
	tag := "nova"
	if pub.existing != "" && pub.existing != nginxutil.Render(pub.plan, pub.hub) {
		tag = "diff com a publicada"
	}
	return sectionHeader(panelTitle(".INC", tag), w, ColorMuted)
}

// nginxIncLines: a .inc com numeração e realce; republicando, o diff com a
// que está no ar (+ entra, - sai).
func (a *App) nginxIncLines(w, room int) []string {
	pub := a.nginxPub
	var diff []nginxutil.DiffLine
	switch {
	case pub.conflict != "":
		diff = nginxutil.LineDiff(pub.conflictText, pub.conflictText)
	case pub.existing != "":
		diff = nginxutil.LineDiff(pub.existing, nginxutil.Render(pub.plan, pub.hub))
	default:
		diff = nginxutil.LineDiff("", nginxutil.Render(pub.plan, pub.hub))
		for i := range diff {
			diff[i].Op = ' '
		}
	}
	pub.scroll = clampScroll(pub.scroll, room, len(diff))
	bad := 0
	if pub.result != nil && pub.conflict == "" {
		bad = pub.result.TestLine
	}
	var lines []string
	newLine := 0
	for i, d := range diff {
		if d.Op != '-' {
			newLine++
		}
		if i < pub.scroll || len(lines) >= room {
			continue
		}
		num := padLeft(strconv.Itoa(newLine), 3)
		if d.Op == '-' {
			num = "   "
		}
		mark, text := StyleMuted.Render(" "), highlightNginxLine(sanitizeTerminalLine(d.Text))
		switch {
		case d.Op == '+':
			mark, text = StyleHealthy.Render("+"), StyleHealthy.Render(sanitizeTerminalLine(d.Text))
		case d.Op == '-':
			mark, text = StyleUnhealthy.Render("-"), StyleUnhealthy.Render(sanitizeTerminalLine(d.Text))
		}
		if bad > 0 && d.Op != '-' && newLine == bad {
			text = StyleUnhealthy.Bold(true).Render(sanitizeTerminalLine(d.Text) + "   ← nginx -t")
		}
		lines = append(lines, truncateVisible("  "+StyleMuted.Render(num)+" "+mark+" "+text, w))
	}
	return lines
}

func (a *App) renderNginxPicker(head []string, w, h int) string {
	pub := a.nginxPub
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"↑↓", "projeto"}, [2]string{"enter", "revisar"}, [2]string{"esc", "voltar"}))
	lines := append(head, sectionHeader(panelTitle("PUBLICAR QUAL PROJETO?", strconv.Itoa(len(pub.choices))), w, tabAccentColor(TabNginx)))
	if len(pub.choices) == 0 {
		lines = append(lines, "  "+StyleMuted.Render("nenhum projeto em "+pub.hub.HostRoot+" — o nginx só enxerga essa pasta"))
	}
	for i, c := range pub.choices {
		prefix := "  "
		if i == pub.pick {
			prefix = StyleAccent.Render("▸ ")
		}
		state := StyleMuted.Render("não publicado")
		if pub.published[c.Path] {
			state = StyleHealthy.Render("◆ publicado em /" + nginxutil.Slug(c.Path) + "/")
		}
		name := StyleNormal.Render(padRight(truncate(c.Name, 28), 28))
		if i == pub.pick {
			name = StyleSelected.Render(padRight(truncate(c.Name, 28), 28))
		}
		lines = append(lines, truncateVisible(prefix+name+"  "+state, w))
	}
	return a.fillReleaseScreen(lines, cmdBar, h)
}
