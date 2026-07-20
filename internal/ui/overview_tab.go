package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderOverviewTab(p *core.Project) string {
	w := maxInt(60, a.width)
	h := maxInt(16, a.projectPanelHeight())
	return a.renderOverviewDashboard(p, w, h)
}

func (a *App) renderOverviewDashboard(p *core.Project, width, height int) string {
	ctx := a.renderOverviewContext(p, width)
	ctxH := lipgloss.Height(ctx)
	bodyH := maxInt(12, height-ctxH)

	rightW := maxInt(24, width*28/100)
	if rightW > 38 {
		rightW = 38
	}
	centerW := maxInt(36, width-rightW-1)

	center := a.renderOverviewCenter(p, centerW, bodyH)
	right := a.renderOverviewRight(p, rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, body)
}

func (a *App) renderOverviewContext(p *core.Project, width int) string {
	env := projectEnvLabel(p)
	host, _ := os.Hostname()
	if host == "" {
		host = "—"
	}
	up := formatUptime(p.Uptime)
	if p.Uptime <= 0 {
		up = formatUptime(a.snapshot.HostMetrics.Uptime)
	}
	online := StyleHealthy.Render("● Online")
	switch {
	case p.Health == core.HealthUnhealthy:
		online = StyleUnhealthy.Render("● Offline")
	case p.Status == core.StatusDegraded:
		online = StyleWarning.Render("● Degraded")
	case p.Status == core.StatusStopped:
		online = StyleMuted.Render("○ Stopped")
	}

	left := StyleMuted.Render("Projeto ") + StyleNormal.Render(p.Name) +
		StyleMuted.Render("  Ambiente ") + StyleWarning.Render(env) +
		StyleMuted.Render("  Servidor ") + StyleNormal.Render(truncate(host, 18)) +
		StyleMuted.Render("  Uptime ") + StyleMuted.Render(up)
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(online)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + online
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

func (a *App) renderOverviewCenter(p *core.Project, width, height int) string {
	row1H := maxInt(5, height*18/100)
	row2H := maxInt(7, height*28/100)
	row3H := maxInt(4, height*14/100)
	row4H := maxInt(5, height*16/100)
	row5H := maxInt(5, height-row1H-row2H-row3H-row4H)

	projAlertW := maxInt(14, width*28/100)
	projMainW := width - projAlertW
	stackW := width / 2
	runtimeW := width - stackW
	actW := width / 2
	healthW := width - actW

	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderOverviewProjectBox(p, projMainW, row1H),
		a.renderOverviewAlertBox(p, projAlertW, row1H),
	)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderOverviewStackBox(p, stackW, row2H),
		a.renderOverviewRuntimeBox(p, runtimeW, row2H),
	)
	row3 := a.renderOverviewModulesBox(p, width, row3H)
	row4 := a.renderOverviewGitBox(p, width, row4H)
	row5 := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderOverviewActivityBox(p, actW, row5H),
		a.renderOverviewHealthBox(p, healthW, row5H),
	)
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3, row4, row5)
}

func (a *App) renderOverviewProjectBox(p *core.Project, width, height int) string {
	lines := []string{
		StyleMuted.Render("Path     ") + StyleNormal.Render(truncate(p.Path, maxInt(12, width-14))),
		StyleMuted.Render("Status   ") + projectStatusStyle(p.Status).Render(statusText(p.Status)),
		StyleMuted.Render("Health   ") + healthLabel(p.Health),
	}
	return renderApiTitledBox("PROJETO", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewAlertBox(p *core.Project, width, height int) string {
	bad := 0
	for _, c := range p.HealthChecks {
		if c.Status == core.HealthUnhealthy {
			bad++
		}
	}
	if bad == 0 && p.Health == core.HealthUnhealthy {
		bad = 1
	}
	for _, c := range p.Containers {
		st := strings.ToLower(c.State + " " + c.Status)
		if strings.Contains(st, "exited") || strings.Contains(st, "dead") || strings.Contains(st, "restarting") {
			bad++
		}
	}
	var lines []string
	if bad == 0 {
		lines = []string{StyleHealthy.Render("tudo certo"), StyleMuted.Render("sem alertas")}
	} else {
		lines = []string{
			StyleWarning.Render(fmt.Sprintf("%d problema(s)", bad)),
			StyleMuted.Render("ver Health (5)"),
		}
	}
	return renderApiTitledBox("Atenção", fitExactLines(lines, height-2), width, height, bad > 0)
}

func (a *App) renderOverviewStackBox(p *core.Project, width, height int) string {
	frameworks := p.Frameworks
	if len(frameworks) == 0 && p.Framework.Name != "" && p.Framework.Name != "Unknown" {
		frameworks = []core.FrameworkInfo{p.Framework}
	}
	lines := make([]string, 0, height-2)
	if len(frameworks) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum detectado)"))
	} else {
		for i, fw := range frameworks {
			prefix := "├ "
			if i == len(frameworks)-1 {
				prefix = "└ "
			}
			ver := ""
			if fw.Version != "" {
				ver = " v" + fw.Version
			}
			lines = append(lines, fmt.Sprintf("%s%s %s%s",
				StyleMuted.Render(prefix),
				frameworkIcon(fw.Name),
				StyleNormal.Render(fw.Name),
				StyleMuted.Render(" ("+fw.Language+")"+ver),
			))
		}
	}
	return renderApiTitledBox("STACK", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewRuntimeBox(p *core.Project, width, height int) string {
	cpu, ram := projectRuntimeMetrics(p)
	hostRAM := a.snapshot.HostMetrics.MemoryTotalMB
	if hostRAM <= 0 {
		hostRAM = 8192
	}
	lines := make([]string, 0, height-2)
	if p.HasDockerCompose {
		lines = append(lines, StyleIconDocker.Render("Docker")+" "+StyleMuted.Render("compose detectado"))
	} else if p.HasDockerfile {
		lines = append(lines, StyleIconDocker.Render("Docker")+" "+StyleMuted.Render("Dockerfile"))
	} else {
		lines = append(lines, StyleMuted.Render("Docker  —"))
	}
	if p.ContainerCount > 0 {
		lines = append(lines, StyleNormal.Render(fmt.Sprintf("Containers  %d vinculados", p.ContainerCount)))
	}
	lines = append(lines,
		StyleMuted.Render("CPU ")+meterBar(cpu, 8)+StyleMuted.Render(fmt.Sprintf(" %.1f%%", cpu)),
		StyleMuted.Render("RAM ")+meterBar(float64(ram)*100/float64(hostRAM), 8)+
			StyleMuted.Render(fmt.Sprintf(" %d / %d MB", ram, hostRAM)),
	)
	if len(p.Ports) > 0 {
		lines = append(lines, StyleAccent.Render(collectors.FormatPortsShort(p.Ports, 6)))
	}
	return renderApiTitledBox("RUNTIME", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewModulesBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.Modules) == 0 {
		if p.WorkerCount > 0 {
			for _, w := range p.Workers {
				st := StyleMuted.Render(w.Status)
				if strings.EqualFold(w.Status, "online") {
					st = StyleHealthy.Render("Online")
				}
				lines = append(lines, fmt.Sprintf("%s  %s  %s",
					StyleNormal.Render(truncate(w.Name, 18)),
					StyleMuted.Render("worker"),
					st,
				))
			}
		} else {
			lines = append(lines, StyleMuted.Render("(nenhum módulo detectado)"))
		}
	} else {
		for _, m := range p.Modules {
			lines = append(lines, fmt.Sprintf("%s  %s  %s",
				StyleNormal.Render(truncate(m.Name, 16)),
				StyleMuted.Render(truncate(m.Path, maxInt(8, width/3))),
				StyleHealthy.Render("Online"),
			))
		}
	}
	return renderApiTitledBox("MÓDULOS ATIVOS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewGitBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if p.Git == nil || !p.Git.IsRepo {
		lines = append(lines, StyleMuted.Render("não é um repositório git"))
	} else {
		g := p.Git
		lines = append(lines,
			StyleMuted.Render("Branch   ")+StyleWarning.Render(g.Branch),
			StyleMuted.Render("Commit   ")+StyleMuted.Render(truncate(g.LastCommit, 8))+" "+
				StyleNormal.Render(truncate(g.LastCommitMsg, maxInt(12, width-28))),
			StyleMuted.Render("Sync     ")+StyleNormal.Render(fmt.Sprintf("+%d / -%d", g.Ahead, g.Behind))+
				StyleMuted.Render(fmt.Sprintf("  ·  %d modified", g.Modified)),
		)
	}
	return renderApiTitledBox("GIT", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewActivityBox(p *core.Project, width, height int) string {
	items := overviewActivity(p, a.snapshot.ScannedAt)
	lines := make([]string, 0, height-2)
	if len(items) == 0 {
		lines = append(lines, StyleMuted.Render("(sem atividade recente)"))
	} else {
		for _, it := range items {
			lines = append(lines, it)
		}
	}
	return renderApiTitledBox("ATIVIDADE RECENTE", fitExactLines(lines, height-2), width, height, false)
}

func overviewActivity(p *core.Project, scanned time.Time) []string {
	var out []string
	if p.Git != nil && p.Git.IsRepo && p.Git.LastCommitMsg != "" {
		when := "agora"
		if !p.Git.LastCommitDate.IsZero() {
			when = relTime(p.Git.LastCommitDate)
		}
		out = append(out, StyleHealthy.Render("✓")+" "+StyleMuted.Render(when)+" "+
			StyleNormal.Render(truncate(p.Git.LastCommitMsg, 36)))
	}
	for _, c := range p.Containers {
		st := strings.ToLower(c.Status + " " + c.State)
		if strings.Contains(st, "restart") {
			out = append(out, StyleWarning.Render("△")+" "+StyleMuted.Render("recente")+" "+
				StyleNormal.Render("restart "+truncate(c.Name, 24)))
		}
	}
	for _, hc := range p.HealthChecks {
		if hc.Status == core.HealthUnhealthy {
			out = append(out, StyleUnhealthy.Render("✗")+" "+StyleMuted.Render("health")+" "+
				StyleNormal.Render(truncate(hc.URL, 32)))
		}
	}
	if !scanned.IsZero() && len(out) < 3 {
		out = append(out, StyleMuted.Render("·")+" "+StyleMuted.Render(relTime(scanned))+" "+
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
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func (a *App) renderOverviewHealthBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.HealthChecks) == 0 {
		lines = append(lines,
			healthRow("App", p.Health),
			healthRow("Git", healthFromBool(p.Git != nil && p.Git.IsRepo)),
			healthRow("Docker", healthFromBool(p.HasDockerCompose || p.ContainerCount > 0)),
		)
	} else {
		for _, hc := range p.HealthChecks {
			label := truncate(hc.URL, maxInt(10, width-16))
			lines = append(lines, healthRow(label, hc.Status))
		}
	}
	return renderApiTitledBox("HEALTH CHECK", fitExactLines(lines, height-2), width, height, false)
}

func healthFromBool(ok bool) core.HealthStatus {
	if ok {
		return core.HealthHealthy
	}
	return core.HealthUnknown
}

func healthRow(label string, h core.HealthStatus) string {
	return StyleMuted.Render(fmt.Sprintf("%-12s", truncate(label, 12))) + " " + healthLabel(h)
}

func (a *App) renderOverviewRight(p *core.Project, width, height int) string {
	detH := maxInt(8, height*36/100)
	actH := maxInt(8, height*36/100)
	noteH := maxInt(4, height-detH-actH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderOverviewDetailsBox(p, width, detH),
		a.renderOverviewActionsBox(width, actH),
		a.renderOverviewNotesBox(width, noteH),
	)
}

func (a *App) renderOverviewDetailsBox(p *core.Project, width, height int) string {
	env := projectEnvLabel(p)
	host, _ := os.Hostname()
	if host == "" {
		host = "—"
	}
	scan := "—"
	if !a.snapshot.ScannedAt.IsZero() {
		scan = a.snapshot.ScannedAt.Format("15:04:05")
	}
	lines := []string{
		StyleMuted.Render("Name     ") + StyleNormal.Render(truncate(p.Name, width-12)),
		StyleMuted.Render("Ambiente ") + StyleWarning.Render(env),
		StyleMuted.Render("Servidor ") + StyleNormal.Render(truncate(host, width-12)),
		StyleMuted.Render("Path     ") + StyleMuted.Render(truncate(p.Path, width-12)),
		StyleMuted.Render("Status   ") + projectStatusStyle(p.Status).Render(statusText(p.Status)),
		StyleMuted.Render("Health   ") + healthLabel(p.Health),
		StyleMuted.Render("Scan     ") + StyleMuted.Render(scan),
	}
	return renderApiTitledBox("DETALHES", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewActionsBox(width, height int) string {
	actions := []struct{ key, desc string }{
		{"a", "analisar projeto"},
		{"2", "ver git"},
		{"5", "health check"},
		{"6", "ver logs"},
		{"o", "abrir no browser"},
		{"E", "shell no projeto"},
		{"3", "containers"},
		{"7", "métricas"},
	}
	lines := make([]string, 0, height-2)
	for _, ac := range actions {
		lines = append(lines, StyleKey.Render(fmt.Sprintf("%-3s", ac.key))+StyleMuted.Render(ac.desc))
	}
	return renderApiTitledBox("AÇÕES RÁPIDAS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderOverviewNotesBox(width, height int) string {
	// ponytail: display-only until notes persist somewhere real
	lines := []string{
		StyleMuted.Render("(vazio)"),
		StyleMuted.Render("notas locais em breve"),
	}
	return renderApiTitledBox("NOTAS", fitExactLines(lines, height-2), width, height, false)
}
