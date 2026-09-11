package ui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/pkg/version"
)

// Premium left rail — brand, grouped nav, live badges, footer meters.

func (a *App) renderProjectSidebar() string {
	return a.renderProjectSidebarH(a.projectPanelHeight())
}

func (a *App) renderProjectSidebarH(height int) string {
	width := 26
	if a.projectCompact() {
		width = 20
	}
	if a.projectTiny() {
		width = 16
	}
	inner := maxInt(12, width-2)
	p := a.currentProject()
	accent := tabAccentColor(a.tab)
	// lipgloss Height is content rows; border adds ±2 to the measured box.
	// Keep outer lipgloss.Height(sidebar) == height so it fits the panel.
	contentH := maxInt(1, height-2)

	top := make([]string, 0, 24)
	top = append(top, a.sidebarBrandBlock(p, inner)...)
	top = append(top, ruleColored(inner, accent))
	top = append(top, a.sidebarNavBlock(p, inner)...)

	foot := a.sidebarFooterLines(p, inner)
	// Prefer nav over meters when vertical space is scarce (VS Code terminal).
	if len(top)+1+len(foot) > contentH {
		foot = []string{StyleMuted.Render(truncate("tab · esc", inner))}
	}
	if len(top)+1+len(foot) > contentH {
		foot = nil
	}
	// Still too tall: drop blank group separators, then trim brand.
	if len(top)+len(foot) > contentH {
		top = a.sidebarBrandBlock(p, inner)
		if !a.projectTiny() {
			top = append(top, ruleColored(inner, accent))
		}
		top = append(top, a.sidebarNavBlockDense(p, inner)...)
	}
	if len(top)+len(foot) > contentH && len(foot) > 0 {
		foot = nil
	}

	blank := 0
	if foot != nil {
		blank = maxInt(0, contentH-len(top)-1-len(foot))
	} else {
		blank = maxInt(0, contentH-len(top))
	}

	rows := make([]string, 0, contentH)
	rows = append(rows, top...)
	for i := 0; i < blank; i++ {
		rows = append(rows, "")
	}
	if foot != nil {
		rows = append(rows, ruleColored(inner, ColorBorder))
		rows = append(rows, foot...)
	}
	if len(rows) > contentH {
		rows = sidebarWindow(rows, slices.Index(rows, a.renderProjectSidebarRow(a.tab, inner, p)), contentH)
	}

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(width).
		Height(contentH).
		Align(lipgloss.Left, lipgloss.Top).
		Render(body)
}

// sidebarWindow scrolls the nav so the active row stays visible when the rail
// is taller than the terminal.
func sidebarWindow(rows []string, focus, height int) []string {
	if height <= 0 || len(rows) <= height {
		return rows
	}
	start := focus - height/2
	if start > len(rows)-height {
		start = len(rows) - height
	}
	if start < 0 {
		start = 0
	}
	return rows[start : start+height]
}

func (a *App) sidebarBrandBlock(p *core.Project, width int) []string {
	accent := tabAccentColor(a.tab)
	mark := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("◆")
	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("devscope")
	if a.projectTiny() {
		name := "project"
		if p != nil {
			name = p.Name
		}
		return []string{
			mark + " " + StyleNormal.Render(truncate(name, maxInt(6, width-2))),
		}
	}
	ver := StyleMuted.Render("v" + version.Version)
	rows := []string{mark + " " + title + " " + ver}
	if p == nil {
		return rows
	}
	// Estado + saúde numa linha só — truncada na largura do trilho. Sem o
	// corte, lipgloss QUEBRA a linha em duas e o rodapé cai para fora da caixa.
	state := projectStatusStyle(p.Status).Render(statusLabel(p.Status, a.animFrame))
	if chip := healthChip(p.Health, a.animFrame); lipgloss.Width(state)+lipgloss.Width(chip)+2 <= width {
		state += StyleMuted.Render("  ") + chip
	}
	rows = append(rows,
		StyleMuted.Render(truncate(p.Name, width)),
		truncateVisible(state, width),
	)
	if !a.projectCompact() {
		if branch := sidebarBranchLine(p, width); branch != "" {
			rows = append(rows, branch)
		}
	}
	return rows
}

// healthChip: glifo + rótulo numa cor só. Glifos distintos por estado — cor
// sozinha não resolve em terminal sem cor nem para quem não distingue.
func healthChip(h core.HealthStatus, frame int) string {
	glyph, st := healthGlyph(h, frame)
	switch h {
	case core.HealthHealthy:
		return st.Render(glyph + " ok")
	case core.HealthUnhealthy:
		return st.Render(glyph + " falha")
	default:
		return st.Render(glyph + " n/a")
	}
}

func sidebarBranchLine(p *core.Project, width int) string {
	if p.Git == nil || !p.Git.IsRepo || p.Git.Branch == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(ColorWarning).Render("⑂ " + truncate(p.Git.Branch, maxInt(8, width-3)))
}

func (a *App) sidebarNavBlock(p *core.Project, width int) []string {
	return a.sidebarNav(p, width, !a.projectTiny())
}

// sidebarNavBlockDense drops blank separators between groups (short terminals).
func (a *App) sidebarNavBlockDense(p *core.Project, width int) []string {
	return a.sidebarNav(p, width, false)
}

// sidebarNav monta a navegação com só os módulos que fazem sentido para este
// projeto. Grupo que ficou inteiro de fora não vira um rótulo órfão: some
// junto — era o "MANAGER" vazio embaixo de projeto sem Docker.
func (a *App) sidebarNav(p *core.Project, width int, spaced bool) []string {
	var rows []string
	for _, g := range sidebarGroups() {
		visible := make([]Tab, 0, len(g.tabs))
		for _, t := range g.tabs {
			if a.tabVisible(t) {
				visible = append(visible, t)
			}
		}
		if len(visible) == 0 {
			continue
		}
		if len(rows) > 0 && spaced {
			rows = append(rows, "")
		}
		rows = append(rows, sidebarGroupLabel(g.title, width, g.color))
		for _, t := range visible {
			rows = append(rows, a.renderProjectSidebarRow(t, width, p))
		}
	}
	if hint := a.sidebarHiddenHint(width, spaced); hint != nil {
		rows = append(rows, hint...)
	}
	return rows
}

// sidebarHiddenHint é a promessa de que nada sumiu: uma linha dizendo quantos
// módulos estão fora e qual tecla os traz de volta. Sem ela, esconder módulo
// seria perder funcionalidade — com ela é hierarquia.
func (a *App) sidebarHiddenHint(width int, spaced bool) []string {
	var line string
	if a.showAllModules {
		line = sidebarHintLine("todos os módulos", "t", width)
	} else {
		n := a.hiddenTabCount()
		if n == 0 {
			return nil
		}
		label := "⋯ " + strconv.Itoa(n) + " módulos ocultos"
		if lipgloss.Width(label) > width-3 {
			label = "⋯ " + strconv.Itoa(n) + " ocultos"
		}
		line = sidebarHintLine(label, "t", width)
	}
	if spaced && !a.projectTiny() {
		return []string{"", line}
	}
	return []string{line}
}

// sidebarHintLine encaixa "rótulo … tecla" na largura do trilho. A conta é
// obrigatória: lipgloss QUEBRA a linha que passa da caixa em vez de cortar, e
// uma linha a mais empurra o rodapé para fora do painel — foi assim que a
// sidebar estourou em 100×30 com o modo "todos" ligado.
func sidebarHintLine(label, key string, width int) string {
	if width < 6 {
		return StyleKey.Render(key)
	}
	room := width - lipgloss.Width(key) - 2 // " " antes do rótulo + " " antes da tecla
	return " " + StyleMuted.Render(padRightVisible(truncate(label, room), room)) + StyleKey.Render(key)
}

type sidebarGroup struct {
	title string
	color lipgloss.Color
	tabs  []Tab
}

// sidebarGroups são as cinco categorias da navegação. Os nomes antigos —
// SCOPE, AUTOMATION, MANAGER, TUNNEL, TOOLS — não diziam o que havia dentro:
// "MANAGER" era Swarm e Kubernetes, e "TOOLS" era um saco com cinco coisas sem
// relação. Agora cada rótulo responde a uma pergunta do usuário:
//
//	PROJETO   o que é isto aqui?
//	CÓDIGO    o que mudou e o que roda em cima do código?
//	EXECUÇÃO  o que está no ar?
//	REDE      por onde se chega?
//	DADOS     com o que eu falo?
//
// A cor do grupo é a cor de destaque do módulo no app inteiro
// (tabAccentColor): cabeçalho do módulo, borda da sidebar, foco do painel. São
// cinco, uma por categoria — não uma por módulo.
func sidebarGroups() []sidebarGroup {
	return []sidebarGroup{
		{"PROJETO", ColorAccent, []Tab{TabOverview}},
		{"CÓDIGO", ColorWarning, []Tab{TabGit, TabActions, TabJenkins}},
		{"EXECUÇÃO", ColorDocker, []Tab{TabContainers, TabSwarm, TabKubernetes}},
		{"REDE", ColorPrimary, []Tab{TabNginx, TabRoutes, TabNgrok, TabSSH, TabCFTunnel}},
		{"DADOS", ColorPink, []Tab{TabAPI, TabDatabase, TabWebSocket}},
	}
}

func sidebarGroupColorForTab(t Tab) lipgloss.Color {
	for _, g := range sidebarGroups() {
		for _, tab := range g.tabs {
			if tab == t {
				return g.color
			}
		}
	}
	return ColorHighlight
}

// sidebarFooterLines fecha o trilho: onde estou (servidor) e como saio daqui.
//
// A largura é obrigatória. Antes as duas linhas eram fixas — hostname cortado
// em 22 colunas e "tab · shift+tab · esc" com 21 — dentro de um trilho que em
// modo compacto tem 18: as duas quebravam em quatro linhas e empurravam o
// rodapé para fora da caixa.
func (a *App) sidebarFooterLines(p *core.Project, width int) []string {
	_ = p
	if a.projectTiny() {
		return []string{StyleMuted.Render(truncate("tab · esc", width))}
	}
	keys := "tab · shift+tab · esc"
	if lipgloss.Width(keys) > width {
		keys = "tab · esc"
	}
	// Servidor mora aqui — saiu da barra de topo, onde repetia a cada módulo.
	return []string{
		StyleMuted.Render(truncate(moduleHostname(), width)),
		StyleMuted.Render(truncate(keys, width)),
	}
}

func meterBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	bar := collectors.BrailleBar(pct/100.0, width)
	st := StyleMetricCPU
	switch {
	case pct >= 80:
		st = StyleUnhealthy
	case pct >= 50:
		st = StyleMetricRAM
	}
	return st.Render(bar)
}

func sidebarGroupLabel(title string, width int, accent lipgloss.Color) string {
	label := lipgloss.NewStyle().Foreground(accent).Faint(true).Bold(true).Render(title)
	gap := width - lipgloss.Width(title) - 1
	if gap < 1 {
		gap = 1
	}
	return label + " " + StyleMuted.Render(strings.Repeat("·", gap))
}

func tabAccentColor(t Tab) lipgloss.Color {
	// Abas ocultas do menu (Logs/JSON/JWT) mantêm cor própria.
	switch t {
	case TabLogs:
		return ColorAccent
	case TabJSON:
		return ColorWarning
	case TabJWT:
		return ColorSuccess
	}
	return sidebarGroupColorForTab(t)
}

func tabGlyph(t Tab) string {
	switch t {
	case TabOverview:
		return "⌂"
	case TabGit:
		return "⑂"
	case TabContainers:
		return "▣"
	case TabKubernetes:
		return "⎈"
	case TabSwarm:
		return "⬡"
	case TabLogs:
		return "≡"
	case TabAPI:
		return "↯"
	case TabDatabase:
		return "▤"
	case TabJSON:
		return "{"
	case TabJWT:
		return "⚿"
	case TabRoutes:
		return "⇄"
	case TabWebSocket:
		// ⚡ e ☰ medem 2 colunas nas libs e 1 na maioria dos terminais — a linha
		// da sidebar saía 1 coluna curta.
		return "⇅"
	case TabNgrok:
		return "⇪"
	case TabCFTunnel:
		return "☁"
	case TabSSH:
		return "⇌"
	case TabJenkins:
		return "⚙"
	case TabActions:
		return "▶"
	case TabNginx:
		return "◈"
	default:
		return "·"
	}
}

func tabActiveBg(_ Tab) lipgloss.Color {
	// Theme-driven so light/dracula don't keep dark-only tints.
	return ColorSelBg
}

func (a *App) renderProjectSidebarRow(t Tab, width int, _ *core.Project) string {
	accentCol := tabAccentColor(t)
	accent := lipgloss.NewStyle().Foreground(accentCol).Bold(true)
	name := t.String()
	if a.projectTiny() {
		name = truncate(name, maxInt(6, width-4))
	}

	if t == a.tab {
		left := "▌" + tabGlyph(t) + " " + name
		pad := width - lipgloss.Width(left)
		if pad < 0 {
			pad = 0
		}
		return lipgloss.NewStyle().
			Foreground(ColorText).
			Background(tabActiveBg(t)).
			Bold(true).
			Render(left + strings.Repeat(" ", pad))
	}
	left := " " + tabGlyph(t) + " " + name
	pad := width - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	nameStyle := StyleMuted
	// Módulo que só está na tela porque o `t` está ligado aparece apagado: é o
	// que ensina, sem legenda, por que ele não estava ali antes.
	if a.showAllModules && !a.moduleCaps.relevant(t) && t != a.tab {
		accent = accent.Faint(true)
		nameStyle = nameStyle.Faint(true)
	}
	return " " + accent.Render(tabGlyph(t)) + " " + nameStyle.Render(name) + strings.Repeat(" ", pad)
}
