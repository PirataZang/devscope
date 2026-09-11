package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderOverviewTab(p *core.Project) string {
	return a.renderOverviewDashboard(p, maxInt(40, a.width), maxInt(6, a.projectPanelHeight()))
}

func (a *App) renderOverviewDashboard(p *core.Project, width, height int) string {
	head := []string{a.renderModuleContext(p, width, "Visão Geral", "")}
	// A faixa de alerta só existe quando há problema — a caixa "Atenção" antiga
	// gastava um terço da largura para dizer "tudo certo".
	if alert := a.renderOverviewAlert(p, width); alert != "" {
		head = append(head, alert)
	}
	if !a.projectTiny() {
		head = append(head, "")
	}
	bodyH := maxInt(4, height-len(head))

	var body string
	if a.projectTiny() || width < 80 {
		body = a.renderOverviewCenter(p, width, bodyH, true)
	} else {
		railW := minInt(38, maxInt(28, width*28/100))
		centerW := maxInt(40, width-railW-1)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderOverviewCenter(p, centerW, bodyH, false),
			a.renderOverviewRail(p, railW, bodyH),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, append(head, body)...)
}

// ─── faixa de alerta ────────────────────────────────────────────────────────

func (a *App) renderOverviewAlert(p *core.Project, width int) string {
	reasons := overviewProblems(p)
	if len(reasons) == 0 {
		return ""
	}
	word := "problemas"
	if len(reasons) == 1 {
		word = "problema"
	}
	head := StyleWarning.Bold(true).Render(fmt.Sprintf("⚠ %d %s", len(reasons), word))
	if len(reasons) > 3 {
		reasons = append(reasons[:3], fmt.Sprintf("+%d", len(reasons)-3))
	}
	tail := StyleKey.Render("l") + StyleMuted.Render(" abre os containers")
	left := head + StyleMuted.Render("   "+strings.Join(reasons, " · "))
	return joinWithSpacer(
		truncateVisible(left, maxInt(10, width-lipgloss.Width(tail)-3)),
		tail, width)
}

func overviewProblems(p *core.Project) []string {
	var out []string
	for _, hc := range p.HealthChecks {
		if hc.Status == core.HealthUnhealthy {
			out = append(out, "probe "+truncate(hc.URL, 28)+" falhando")
		}
	}
	for _, c := range p.Containers {
		switch st := strings.ToLower(c.State + " " + c.Status + " " + c.Health); {
		case strings.Contains(st, "restart"):
			out = append(out, c.Name+" reiniciando")
		case strings.Contains(st, "exited"), strings.Contains(st, "dead"):
			out = append(out, c.Name+" parado")
		case strings.Contains(st, "unhealthy"):
			out = append(out, c.Name+" unhealthy")
		}
	}
	if len(out) == 0 && p.Health == core.HealthUnhealthy {
		out = append(out, "health check falhando")
	}
	return out
}

// ─── coluna principal ───────────────────────────────────────────────────────

type overviewSection struct {
	height int
	render func(width int) string
}

func overviewBox(title string, lines []string) overviewSection {
	h := len(lines) + 2
	return overviewSection{h, func(w int) string {
		return panelBox(title, lines, w, h, false)
	}}
}

// overviewPair põe duas caixas curtas lado a lado — separadas, cada uma gastava
// a largura inteira do painel para mostrar duas linhas.
func overviewPair(t1 string, l1 []string, t2 string, l2 []string) overviewSection {
	h := maxInt(len(l1), len(l2)) + 2
	return overviewSection{h, func(w int) string {
		left := w / 2
		return lipgloss.JoinHorizontal(lipgloss.Top,
			panelBox(t1, l1, left, h, false),
			panelBox(t2, l2, w-left, h, false))
	}}
}

// renderOverviewCenter dá a cada caixa a altura do seu conteúdo e entrega a
// sobra para CONTAINERS — a única lista que cresce de verdade. Antes as alturas
// eram percentuais fixos, e caixas de 2 linhas viravam torres de 13.
func (a *App) renderOverviewCenter(p *core.Project, width, height int, solo bool) string {
	inner := maxInt(10, width-2)
	half := maxInt(10, width/2-2)
	stack := a.overviewStackRuntimeLines(p, inner)

	var tail []overviewSection
	if solo {
		tail = append(tail, overviewBox("GIT", a.overviewGitLines(p, inner)))
	}
	if mods := overviewModuleLines(p, inner); len(mods) > 0 {
		tail = append(tail, overviewBox("MÓDULOS", mods))
	}

	activity := overviewActivity(p, a.snapshot.ScannedAt)
	if pair := !solo && width >= 90; pair {
		tail = append(tail, overviewPair(
			"SAÚDE", overviewHealthLines(p, half, a.animFrame),
			"ATIVIDADE", activity))
	} else {
		tail = append(tail, overviewBox("SAÚDE", overviewHealthLines(p, inner, a.animFrame)))
		if len(activity) > 0 {
			tail = append(tail, overviewBox("ATIVIDADE", activity))
		}
	}

	room := height - (len(stack) + 2) - 2 // 2 = borda da caixa CONTAINERS
	for _, s := range tail {
		room -= s.height
	}
	// Corta as caixas de baixo antes de espremer a lista de containers.
	for len(tail) > 0 && room < 3 {
		room += tail[len(tail)-1].height
		tail = tail[:len(tail)-1]
	}

	ctrs := overviewContainerLines(p, inner, a.animFrame)
	if room = maxInt(1, room); len(ctrs) > room {
		ctrs = append(ctrs[:room-1],
			StyleMuted.Render(fmt.Sprintf("+%d containers", len(ctrs)-room+1)))
	}

	boxes := []string{
		panelBox("STACK & RUNTIME", stack, width, len(stack)+2, false),
		panelBox(panelTitle("CONTAINERS", fmt.Sprint(len(p.Containers))), ctrs, width, len(ctrs)+2, false),
	}
	for _, s := range tail {
		boxes = append(boxes, s.render(width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, boxes...)
}

func (a *App) overviewStackRuntimeLines(p *core.Project, width int) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 9)) }
	lines := make([]string, 0, 8)

	frameworks := projectFrameworks(*p)
	if len(frameworks) == 0 {
		lines = append(lines, label("Stack")+StyleMuted.Render("(nenhum detectado)"))
	} else {
		parts := make([]string, 0, len(frameworks))
		for _, fw := range frameworks {
			name := fw.Name
			if fw.Version != "" {
				name += " " + fw.Version // era "Laravel () v11.2"
			}
			parts = append(parts, stackStyle(fw.Name).Render(name))
		}
		lines = append(lines, label("Stack")+strings.Join(parts, StyleMuted.Render("  ·  ")))
	}

	var runtime []string
	switch {
	case p.HasDockerCompose:
		runtime = append(runtime, StyleNormal.Render("compose"))
	case p.HasDockerfile:
		runtime = append(runtime, StyleNormal.Render("Dockerfile"))
	}
	if p.ContainerCount > 0 {
		runtime = append(runtime, StyleNormal.Render(fmt.Sprintf("%d containers", p.ContainerCount)))
	}
	if p.WorkerCount > 0 {
		runtime = append(runtime, StyleNormal.Render(fmt.Sprintf("%d workers", p.WorkerCount)))
	}
	if len(runtime) == 0 {
		runtime = append(runtime, StyleMuted.Render(emDash))
	}
	lines = append(lines, label("Docker")+strings.Join(runtime, StyleMuted.Render("  ·  ")))

	cpu, ram := projectRuntimeMetrics(p)
	bar := minInt(16, maxInt(6, width/4))
	ramPct := 0.0
	if total := a.snapshot.HostMetrics.MemoryTotalMB; total > 0 {
		ramPct = float64(ram) * 100 / float64(total)
	}
	lines = append(lines,
		label("CPU")+barSolid(cpu, bar)+StyleMuted.Render(fmt.Sprintf("  %.1f%%", cpu)),
		label("RAM")+barSolid(ramPct, bar)+StyleMuted.Render(fmt.Sprintf("  %d MB", ram)),
	)

	if len(p.Ports) > 0 {
		lines = append(lines, label("Portas")+
			lipgloss.NewStyle().Foreground(ColorAccent).Render(collectors.FormatPortsShort(p.Ports, 6)))
	}
	for i, d := range p.Domains {
		if i >= 2 {
			lines = append(lines, label("")+StyleMuted.Render(fmt.Sprintf("+%d domínios", len(p.Domains)-2)))
			break
		}
		key := "Domínio"
		if i > 0 {
			key = ""
		}
		row := label(key) + StyleNormal.Render(truncate(d.Host, maxInt(10, width-24)))
		if days, ok := sslDaysFor(p, d.Host); ok {
			st := StyleHealthy
			switch {
			case days < 15:
				st = StyleUnhealthy
			case days < 30:
				st = StyleWarning
			}
			row += StyleMuted.Render("   SSL ") + st.Render(fmt.Sprintf("%d dias", days))
		}
		lines = append(lines, row)
	}
	return lines
}

func sslDaysFor(p *core.Project, host string) (int, bool) {
	for _, c := range p.SSL {
		if strings.EqualFold(c.Domain, host) {
			return c.DaysLeft, true
		}
	}
	return 0, false
}

func overviewContainerLines(p *core.Project, width, frame int) []string {
	if len(p.Containers) == 0 {
		if p.ContainerCount > 0 {
			return []string{StyleMuted.Render(fmt.Sprintf("%d vinculados · abra a aba Containers", p.ContainerCount))}
		}
		return []string{StyleMuted.Render("(nenhum container)")}
	}
	nameW := minInt(22, maxInt(10, width*24/100))
	imgW := minInt(28, maxInt(8, width*26/100))
	// Colunas que não cabem inteiras somem — ":80…" não informa nada.
	statusW := minInt(14, width*14/100)
	if statusW < 8 {
		statusW = 0
	}
	portW := width - nameW - imgW - statusW - 20
	if statusW > 0 {
		portW--
	}
	if portW < 6 {
		portW = 0
	}

	out := make([]string, 0, len(p.Containers))
	for _, c := range p.Containers {
		glyph, st := containerDot(c, frame)
		row := st.Render(glyph) + " " +
			StyleNormal.Render(padRight(truncate(c.Name, nameW), nameW)) + " " +
			StyleMuted.Render(padRight(truncate(c.Image, imgW), imgW))
		if statusW > 0 {
			row += " " + StyleMuted.Render(padRight(truncate(c.Status, statusW), statusW))
		}
		if portW > 0 {
			row += " " + lipgloss.NewStyle().Foreground(ColorAccent).
				Render(padRight(truncate(containerPortsShort(c), portW), portW))
		}
		out = append(out, row+" "+
			StyleMuted.Render(fmt.Sprintf("%5.1f%%", c.CPU))+" "+
			StyleMuted.Render(fmt.Sprintf("%7s", formatMiB(c.Memory))))
	}
	return out
}

func containerPortsShort(c core.Container) string {
	maps := collectors.ParseContainerPortMappings(c.Ports)
	if len(maps) == 0 {
		return ""
	}
	seen := make(map[int]bool, len(maps))
	parts := make([]string, 0, 3)
	for _, m := range maps {
		if seen[m.HostPort] {
			continue
		}
		seen[m.HostPort] = true
		if len(parts) == 3 {
			parts = append(parts, fmt.Sprintf("+%d", len(maps)-3))
			break
		}
		parts = append(parts, fmt.Sprintf(":%d", m.HostPort))
	}
	return strings.Join(parts, " ")
}

// containerDot: "unhealthy" contém "healthy", então testa primeiro.
func containerDot(c core.Container, frame int) (string, lipgloss.Style) {
	st := strings.ToLower(c.State + " " + c.Status + " " + c.Health)
	switch {
	case strings.Contains(st, "unhealthy"), strings.Contains(st, "restart"):
		return pulseGlyph(pulseWarn, frame), StyleWarning
	case strings.Contains(st, "exited"), strings.Contains(st, "dead"), strings.Contains(st, "created"):
		return pulseGlyph(pulseBad, frame), StyleStopped
	case strings.Contains(st, "running"), strings.Contains(st, "up "):
		return pulseGlyph(pulseOK, frame), StyleRunning
	default:
		return pulseGlyph(pulseIdle, frame), StyleMuted
	}
}

func formatMiB(bytes int64) string {
	return fmt.Sprintf("%d MB", bytes/(1<<20))
}

func overviewModuleLines(p *core.Project, width int) []string {
	if len(p.Modules) == 0 {
		return nil // workers viram "N workers" na linha do Docker
	}
	out := make([]string, 0, len(p.Modules))
	for _, m := range p.Modules {
		out = append(out, StyleNormal.Render(padRight(truncate(m.Name, 18), 18))+" "+
			StyleMuted.Render(truncate(m.Path, maxInt(8, width-20))))
	}
	return out
}

func overviewHealthLines(p *core.Project, width, frame int) []string {
	if len(p.HealthChecks) == 0 {
		return []string{
			healthRow("App", p.Health, frame),
			healthRow("Git", healthFromBool(p.Git != nil && p.Git.IsRepo), frame),
			healthRow("Docker", healthFromBool(p.HasDockerCompose || p.ContainerCount > 0), frame),
		}
	}
	urlW := maxInt(16, width-12)
	out := make([]string, 0, len(p.HealthChecks))
	for _, hc := range p.HealthChecks {
		glyph, st := healthGlyph(hc.Status, frame)
		lat := emDash
		if hc.LatencyMS > 0 {
			lat = fmt.Sprintf("%d ms", hc.LatencyMS)
		}
		out = append(out, st.Render(glyph)+" "+
			StyleNormal.Render(padRight(truncate(hc.URL, urlW), urlW))+
			StyleMuted.Render(fmt.Sprintf("%8s", lat)))
	}
	return out
}

func healthGlyph(h core.HealthStatus, frame int) (string, lipgloss.Style) {
	switch h {
	case core.HealthHealthy:
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case core.HealthUnhealthy:
		return pulseGlyph(pulseBad, frame), StyleUnhealthy
	default:
		return pulseGlyph(pulseIdle, frame), StyleMuted
	}
}

// projectRuntimeMetrics e healthPlain vieram de metrics_tab.go / health_tab.go,
// apagados com as abas Metrics e Status — a Visão Geral é a única consumidora.
func projectRuntimeMetrics(p *core.Project) (float64, int64) {
	var cpu float64
	var memory int64
	for _, c := range p.Containers {
		cpu += c.CPU
		memory += c.Memory
	}
	for _, w := range p.Workers {
		if strings.EqualFold(w.Status, "online") {
			cpu += w.CPU
			memory += w.Memory
		}
	}
	return cpu, memory / (1024 * 1024)
}

func healthPlain(h core.HealthStatus) string {
	if h == "" {
		return "Unknown"
	}
	return string(h)
}

func healthFromBool(ok bool) core.HealthStatus {
	if ok {
		return core.HealthHealthy
	}
	return core.HealthUnknown
}

func healthRow(label string, h core.HealthStatus, frame int) string {
	glyph, st := healthGlyph(h, frame)
	return st.Render(glyph) + " " + StyleNormal.Render(padRight(truncate(label, 12), 12)) +
		StyleMuted.Render(healthPlain(h))
}

func overviewActivity(p *core.Project, scanned time.Time) []string {
	var out []string
	if p.Git != nil && p.Git.IsRepo && p.Git.LastCommitMsg != "" {
		when := "agora"
		if !p.Git.LastCommitDate.IsZero() {
			when = relTime(p.Git.LastCommitDate)
		}
		out = append(out, StyleHealthy.Render("✓")+" "+StyleMuted.Render(padRight(when, 5))+" "+
			StyleNormal.Render(truncate(p.Git.LastCommitMsg, 52)))
	}
	for _, c := range p.Containers {
		if strings.Contains(strings.ToLower(c.Status+" "+c.State), "restart") {
			out = append(out, StyleWarning.Render("△")+" "+StyleMuted.Render(padRight("agora", 5))+" "+
				StyleNormal.Render("restart "+truncate(c.Name, 24)))
		}
	}
	for _, hc := range p.HealthChecks {
		if hc.Status == core.HealthUnhealthy {
			out = append(out, StyleUnhealthy.Render("✗")+" "+StyleMuted.Render(padRight("agora", 5))+" "+
				StyleNormal.Render(truncate(hc.URL, 46)))
		}
	}
	if !scanned.IsZero() && len(out) < 3 {
		out = append(out, StyleMuted.Render("·")+" "+StyleMuted.Render(padRight(relTime(scanned), 5))+" "+
			StyleMuted.Render("último scan"))
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmes", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%da", int(d.Hours()/(24*365)))
	}
}

// ─── rail direito ───────────────────────────────────────────────────────────

func (a *App) renderOverviewRail(p *core.Project, width, height int) string {
	git := a.overviewGitLines(p, maxInt(10, width-2))
	return lipgloss.JoinVertical(lipgloss.Left,
		panelBox("GIT", git, width, minInt(height, len(git)+2), false),
		renderActionsBox(width, maxInt(3, height-len(git)-2),
			[2]string{"a", "analisar"},
			[2]string{"l", "containers"},
			[2]string{"g", "git"},
			[2]string{"o", "browser"},
			[2]string{"E", "shell"},
			[2]string{"r", "atualizar"},
			[2]string{"tab", "próximo módulo"},
		),
	)
}

func (a *App) overviewGitLines(p *core.Project, width int) []string {
	if p.Git == nil || !p.Git.IsRepo {
		return []string{StyleMuted.Render("não é um repositório git")}
	}
	g := p.Git
	sync := ""
	if g.Ahead > 0 {
		sync += StyleHealthy.Render(fmt.Sprintf("  ↑%d", g.Ahead))
	}
	if g.Behind > 0 {
		sync += StyleWarning.Render(fmt.Sprintf("  ↓%d", g.Behind))
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(ColorAccent).Render(
			"⑂ "+truncate(g.Branch, maxInt(8, width-lipgloss.Width(sync)-2))) + sync,
	}
	if g.LastCommit != "" {
		lines = append(lines, StyleMuted.Render(truncate(g.LastCommit, 7)+"  ")+
			StyleNormal.Render(truncate(g.LastCommitMsg, maxInt(8, width-10))))
	}
	var who []string
	if g.Author != "" {
		who = append(who, g.Author)
	}
	if !g.LastCommitDate.IsZero() {
		who = append(who, "há "+relTime(g.LastCommitDate))
	}
	if len(who) > 0 {
		lines = append(lines, StyleMuted.Render(truncate(strings.Join(who, "  ·  "), width)))
	}

	var dirty []string
	if g.Modified > 0 {
		dirty = append(dirty, fmt.Sprintf("%d modificados", g.Modified))
	}
	if g.Staged > 0 {
		dirty = append(dirty, fmt.Sprintf("%d staged", g.Staged))
	}
	if g.Untracked > 0 {
		dirty = append(dirty, fmt.Sprintf("%d novos", g.Untracked))
	}
	if len(dirty) == 0 {
		lines = append(lines, StyleHealthy.Render("árvore limpa"))
	} else {
		lines = append(lines, StyleWarning.Render(truncate(strings.Join(dirty, " · "), width)))
	}
	if g.StashCount > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("%d stash", g.StashCount)))
	}
	return lines
}

func projectEnvLabel(p *core.Project) string {
	if p.Git == nil || p.Git.Branch == "" {
		return "local"
	}
	b := strings.ToLower(p.Git.Branch)
	switch {
	case b == "main" || b == "master" || strings.Contains(b, "prod"):
		return "Prod"
	case strings.HasPrefix(b, "des-") || strings.Contains(b, "dev") || b == "develop" || b == "development":
		return "Dev"
	case strings.Contains(b, "stag") || strings.Contains(b, "homolog"):
		return "Stage"
	default:
		return truncate(p.Git.Branch, 12)
	}
}
