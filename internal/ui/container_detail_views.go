package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) renderContainerDetailChrome(body string, bodyH int) string {
	height := maxInt(12, a.height-2)
	panelW := maxInt(40, a.width)
	innerW := maxInt(36, panelW-2)

	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabContainers)).Bold(true)
	left := accent.Render("devscope") + StyleMuted.Render(" › docker › ") +
		StyleNormal.Render(truncate(a.containerDetailName, maxInt(10, innerW/3)))
	right := a.containerDetailStatusBadge()
	pad := innerW - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	header := left + strings.Repeat(" ", pad) + right
	tabs := a.renderContainerDetailTabBar(innerW)

	posViewport := maxInt(1, bodyH-4)
	position := a.containerDetailPosition(posViewport)
	footer := StyleMuted.Render(truncate(a.containerDetailFooter(position), innerW))

	content := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tabAccentColor(TabContainers)).
		Width(panelW).
		MaxWidth(panelW).
		Render(content)
	panel = clampRenderedHeight(panel, height)
	statusBar := truncate(a.renderStatusBar("container · "+a.containerDetailTab.shortLabel()), panelW)
	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

func (a *App) renderContainerDetailRichBody(width, height int) string {
	if a.containerDetailLoading {
		return renderApiTitledBox(
			strings.ToUpper(a.containerDetailTab.shortLabel()),
			fitExactLines([]string{StyleMuted.Render("Carregando…")}, height-2),
			width, height, true,
		)
	}
	switch a.containerDetailTab {
	case containerDetailTabLogs:
		return a.renderContainerLogsView(width, height)
	case containerDetailTabEnv:
		return a.renderContainerEnvView(width, height)
	case containerDetailTabConfig:
		return a.renderContainerConfigView(width, height)
	case containerDetailTabTop:
		return a.renderContainerTopView(width, height)
	case containerDetailTabCompose, containerDetailTabFile:
		return a.renderContainerCodeView(width, height)
	default:
		lines := a.renderContainerDetailBodyLines(maxInt(1, height-2), width)
		return renderApiTitledBox(a.containerDetailTab.shortLabel(), fitExactLines(lines, height-2), width, height, true)
	}
}

func (a *App) renderContainerLogsView(width, height int) string {
	all := a.containerDetailLines()
	errN, warnN, infoN := countLogLevels(all)
	cardH := 3
	boxW := maxInt(10, width/5)
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("LINES", fmt.Sprintf("%d", len(all)), StyleMuted.Render("total"), StyleNormal, boxW, cardH),
		" ",
		renderStatsCard("ERROR", fmt.Sprintf("%d", errN), StyleMuted.Render("match"), StyleUnhealthy, boxW, cardH),
		" ",
		renderStatsCard("WARN", fmt.Sprintf("%d", warnN), StyleMuted.Render("match"), StyleWarning, boxW, cardH),
		" ",
		renderStatsCard("INFO", fmt.Sprintf("%d", infoN), StyleMuted.Render("match"), StyleAccent, boxW, cardH),
		" ",
		renderStatsCard("FOLLOW", logFollowLabel(a), StyleMuted.Render("f / p"), StyleHealthy, boxW, cardH),
	)

	restH := maxInt(6, height-cardH)
	rightW := maxInt(22, width*26/100)
	if rightW > 34 {
		rightW = 34
	}
	leftW := width - rightW
	viewport := maxInt(1, restH-2)

	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(all))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(all))
	matchSet := a.containerDetailMatchLineSet()
	current := -1
	if matches := a.containerDetailSearchMatches(); len(matches) > 0 && a.containerDetailSearchIdx < len(matches) {
		current = matches[a.containerDetailSearchIdx]
	}
	textW := maxInt(8, leftW-4)
	logLines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		logLines = append(logLines, a.renderContainerLogRichLine(all[i], textW, matchSet[i], i == current))
	}
	stream := renderApiTitledBox("LOG STREAM", fitExactLines(logLines, viewport), leftW, restH, true)

	recent := recentInterestingLogs(all, 8)
	sideLines := []string{
		StyleMuted.Render("atalhos"),
		StyleKey.Render("f") + StyleMuted.Render(" follow"),
		StyleKey.Render("p") + StyleMuted.Render(" pausa"),
		StyleKey.Render("/") + StyleMuted.Render(" buscar"),
		StyleKey.Render("r") + StyleMuted.Render(" reload"),
		"",
		StyleMuted.Render("recent err/warn"),
	}
	if len(recent) == 0 {
		sideLines = append(sideLines, StyleMuted.Render("(nenhum)"))
	} else {
		for _, line := range recent {
			sideLines = append(sideLines, colorLogLine(truncate(line, rightW-4), false, false))
		}
	}
	side := renderApiTitledBox("INSPECT", fitExactLines(sideLines, viewport), rightW, restH, false)
	return lipgloss.JoinVertical(lipgloss.Left, cards, lipgloss.JoinHorizontal(lipgloss.Top, stream, side))
}

func logFollowLabel(a *App) string {
	switch {
	case a.containerDetailFollow && a.containerDetailFollowPaused:
		return "PAUSED"
	case a.containerDetailFollow:
		return "LIVE"
	default:
		return "off"
	}
}

func (a *App) renderContainerLogRichLine(line string, width int, matched, current bool) string {
	line = sanitizeTerminalLine(line)
	display := sliceColumns(line, a.containerDetailHScroll, width)
	return colorLogLine(display, matched, current)
}

func colorLogLine(display string, matched, current bool) string {
	if current {
		return StyleDiffMatch.Render(display)
	}
	if matched {
		return StyleWarning.Render(display)
	}
	low := strings.ToLower(display)
	switch {
	case strings.Contains(low, "error") || strings.Contains(low, "fatal") || strings.Contains(low, "panic"):
		return StyleUnhealthy.Render(display)
	case strings.Contains(low, "warn"):
		return StyleWarning.Render(display)
	case strings.Contains(low, "info") || strings.Contains(low, " cron["):
		return StyleAccent.Render(display)
	default:
		return StyleMuted.Render(display)
	}
}

func countLogLevels(lines []string) (errN, warnN, infoN int) {
	for _, line := range lines {
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "error") || strings.Contains(low, "fatal") || strings.Contains(low, "panic"):
			errN++
		case strings.Contains(low, "warn"):
			warnN++
		case strings.Contains(low, "info"):
			infoN++
		}
	}
	return
}

func recentInterestingLogs(lines []string, n int) []string {
	var out []string
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		low := strings.ToLower(lines[i])
		if strings.Contains(low, "error") || strings.Contains(low, "warn") || strings.Contains(low, "fatal") {
			out = append(out, strings.TrimSpace(lines[i]))
		}
	}
	// reverse to chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (a *App) renderContainerEnvView(width, height int) string {
	pairs := parseEnvPairs(a.containerDetailContent)
	groups := groupEnvPairs(pairs)
	cardH := 3
	boxW := maxInt(10, width/5)
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("VARS", fmt.Sprintf("%d", len(pairs)), StyleMuted.Render("total"), StyleNormal, boxW, cardH),
		" ",
		renderStatsCard("GROUPS", fmt.Sprintf("%d", len(groups)), StyleMuted.Render("categorias"), StyleAccent, boxW, cardH),
		" ",
		renderStatsCard("PATH", envHasKey(pairs, "PATH"), StyleMuted.Render("system"), StyleHealthy, boxW, cardH),
		" ",
		renderStatsCard("NODE", envHasKey(pairs, "NODE_VERSION"), StyleMuted.Render("runtime"), StyleWarning, boxW, cardH),
		" ",
		renderStatsCard("PHP", envHasKey(pairs, "PHP_VERSION"), StyleMuted.Render("runtime"), StyleAccent, boxW, cardH),
	)

	restH := maxInt(6, height-cardH)
	rightW := maxInt(24, width*30/100)
	leftW := width - rightW
	viewport := maxInt(1, restH-2)

	flat := flattenEnvGroups(groups)
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(flat))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(flat))
	textW := maxInt(8, leftW-4)
	listLines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		listLines = append(listLines, renderEnvFlatLine(flat[i], textW, a.containerDetailHScroll))
	}
	list := renderApiTitledBox("ENVIRONMENT", fitExactLines(listLines, viewport), leftW, restH, true)

	side := []string{StyleMuted.Render("por grupo")}
	for _, g := range groups {
		side = append(side, StyleMuted.Render(fmt.Sprintf("%-10s ", truncate(g.name, 10)))+
			StyleNormal.Render(fmt.Sprintf("%d", len(g.pairs))))
	}
	side = append(side, "", StyleMuted.Render("/ busca key"), StyleMuted.Render("↑↓ scroll"))
	rail := renderApiTitledBox("RESUMO", fitExactLines(side, viewport), rightW, restH, false)
	return lipgloss.JoinVertical(lipgloss.Left, cards, lipgloss.JoinHorizontal(lipgloss.Top, list, rail))
}

type envPair struct{ key, val string }
type envGroup struct {
	name  string
	pairs []envPair
}

func parseEnvPairs(content string) []envPair {
	var out []envPair
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out = append(out, envPair{key: k, val: v})
	}
	return out
}

func envHasKey(pairs []envPair, key string) string {
	for _, p := range pairs {
		if p.key == key {
			return truncate(p.val, 12)
		}
	}
	return "—"
}

func groupEnvPairs(pairs []envPair) []envGroup {
	order := []string{"PHP", "NODE", "COMPOSER", "DOCKER", "DB", "SYSTEM", "OTHER"}
	buckets := map[string][]envPair{}
	for _, p := range pairs {
		g := envGroupName(p.key)
		buckets[g] = append(buckets[g], p)
	}
	var out []envGroup
	for _, name := range order {
		if len(buckets[name]) == 0 {
			continue
		}
		out = append(out, envGroup{name: name, pairs: buckets[name]})
	}
	return out
}

func envGroupName(key string) string {
	k := strings.ToUpper(key)
	switch {
	case strings.Contains(k, "PHP"):
		return "PHP"
	case strings.Contains(k, "NODE") || strings.Contains(k, "NPM") || strings.Contains(k, "YARN"):
		return "NODE"
	case strings.Contains(k, "COMPOSER"):
		return "COMPOSER"
	case strings.Contains(k, "DOCKER"):
		return "DOCKER"
	case strings.Contains(k, "MYSQL") || strings.Contains(k, "POSTGRES") || strings.Contains(k, "REDIS") || strings.Contains(k, "DB_"):
		return "DB"
	case k == "PATH" || k == "HOME" || k == "USER" || k == "LANG" || k == "TZ" || k == "TERM" || k == "SHELL" || k == "PWD":
		return "SYSTEM"
	default:
		return "OTHER"
	}
}

type envFlatLine struct {
	section bool
	text    string
	pair    envPair
}

func flattenEnvGroups(groups []envGroup) []envFlatLine {
	var out []envFlatLine
	for _, g := range groups {
		out = append(out, envFlatLine{section: true, text: "▸ " + g.name + fmt.Sprintf(" (%d)", len(g.pairs))})
		for _, p := range g.pairs {
			out = append(out, envFlatLine{pair: p})
		}
	}
	return out
}

func renderEnvFlatLine(line envFlatLine, width, hScroll int) string {
	if line.section {
		return StyleAccent.Bold(true).Render(truncate(line.text, width))
	}
	raw := line.pair.key + "=" + line.pair.val
	visible := sliceColumns(raw, hScroll, width)
	if eq := strings.IndexByte(visible, '='); eq > 0 && hScroll == 0 {
		return StyleWarning.Render(visible[:eq]) + StyleNormal.Render(visible[eq:])
	}
	return StyleNormal.Render(visible)
}

func (a *App) renderContainerConfigView(width, height int) string {
	cfg := parseContainerConfig(a.containerDetailContent)
	cardH := 3
	boxW := maxInt(10, width/5)
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("IMAGE", truncate(cfg.image, boxW-4), StyleMuted.Render("runtime"), StyleAccent, boxW, cardH),
		" ",
		renderStatsCard("RESTART", firstNonEmpty(cfg.restart, "—"), StyleMuted.Render("policy"), StyleWarning, boxW, cardH),
		" ",
		renderStatsCard("MOUNTS", fmt.Sprintf("%d", len(cfg.mounts)), StyleMuted.Render("volumes"), StyleHealthy, boxW, cardH),
		" ",
		renderStatsCard("PORTS", fmt.Sprintf("%d", len(cfg.ports)), StyleMuted.Render("publish"), StyleNormal, boxW, cardH),
		" ",
		renderStatsCard("LABELS", fmt.Sprintf("%d", len(cfg.labels)), StyleMuted.Render("meta"), StyleMuted, boxW, cardH),
	)

	restH := maxInt(6, height-cardH)
	leftW := width * 55 / 100
	rightW := width - leftW

	ident := []string{
		StyleMuted.Render("Name  ") + StyleNormal.Render(truncate(cfg.name, leftW-10)),
		StyleMuted.Render("ID    ") + StyleMuted.Render(truncate(cfg.id, 18)),
		StyleMuted.Render("Image ") + StyleAccent.Render(truncate(cfg.image, leftW-10)),
		StyleMuted.Render("Cmd   ") + StyleMuted.Render(truncate(cfg.command, leftW-10)),
		StyleMuted.Render("Rest  ") + StyleWarning.Render(firstNonEmpty(cfg.restart, "—")),
	}
	mountLines := []string{StyleMuted.Render("(none)")}
	if len(cfg.mounts) > 0 {
		mountLines = mountLines[:0]
		for _, m := range cfg.mounts {
			mountLines = append(mountLines, StyleHealthy.Render("● ")+StyleMuted.Render(truncate(m, leftW-4)))
		}
	}
	portLines := []string{StyleMuted.Render("(none)")}
	if len(cfg.ports) > 0 {
		portLines = portLines[:0]
		for _, p := range cfg.ports {
			portLines = append(portLines, StyleAccent.Render("⇄ ")+StyleNormal.Render(truncate(p, rightW-4)))
		}
	}
	labelLines := make([]string, 0, len(cfg.labels))
	for _, l := range cfg.labels {
		labelLines = append(labelLines, StyleMuted.Render(truncate(l, rightW-2)))
	}
	if len(labelLines) == 0 {
		labelLines = []string{StyleMuted.Render("(none)")}
	}

	// scroll applies to combined right labels mostly; keep mounts/ports visible
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, maxInt(1, restH/2), len(labelLines))
	labStart := a.containerDetailScroll
	labView := labelLines[labStart:minInt(labStart+maxInt(3, restH/2-1), len(labelLines))]

	identH := maxInt(6, restH*40/100)
	mountH := restH - identH
	left := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("IDENTITY", fitExactLines(ident, identH-2), leftW, identH, false),
		renderApiTitledBox("MOUNTS", fitExactLines(mountLines, mountH-2), leftW, mountH, false),
	)
	portH := maxInt(4, restH*30/100)
	labH := restH - portH
	right := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("PORTS", fitExactLines(portLines, portH-2), rightW, portH, false),
		renderApiTitledBox("LABELS", fitExactLines(labView, labH-2), rightW, labH, true),
	)
	return lipgloss.JoinVertical(lipgloss.Left, cards, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
}

type containerConfigView struct {
	id, name, image, command, restart string
	mounts, ports, labels             []string
}

func parseContainerConfig(content string) containerConfigView {
	var c containerConfigView
	section := ""
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "ID:"):
			c.id = strings.TrimSpace(strings.TrimPrefix(trim, "ID:"))
			section = ""
		case strings.HasPrefix(trim, "Name:"):
			c.name = strings.TrimSpace(strings.TrimPrefix(trim, "Name:"))
		case strings.HasPrefix(trim, "Image:"):
			c.image = strings.TrimSpace(strings.TrimPrefix(trim, "Image:"))
		case strings.HasPrefix(trim, "Command:"):
			c.command = strings.TrimSpace(strings.TrimPrefix(trim, "Command:"))
		case strings.HasPrefix(trim, "Restart:"):
			c.restart = strings.TrimSpace(strings.TrimPrefix(trim, "Restart:"))
		case trim == "Labels:":
			section = "labels"
		case trim == "Mounts:" || strings.HasPrefix(trim, "Mounts:"):
			section = "mounts"
			if strings.Contains(trim, "none") {
				section = ""
			}
		case trim == "Ports:" || strings.HasPrefix(trim, "Ports:"):
			section = "ports"
			if strings.Contains(trim, "none") {
				section = ""
			}
		case strings.HasPrefix(line, "  ") && section != "":
			item := strings.TrimSpace(line)
			switch section {
			case "labels":
				c.labels = append(c.labels, item)
			case "mounts":
				c.mounts = append(c.mounts, item)
			case "ports":
				c.ports = append(c.ports, item)
			}
		}
	}
	return c
}

func (a *App) renderContainerTopView(width, height int) string {
	content := strings.TrimSpace(a.containerDetailContent)
	low := strings.ToLower(content)
	if strings.Contains(low, "not running") || strings.Contains(low, "is not running") || strings.Contains(low, "erro:") {
		msg := []string{
			StyleUnhealthy.Render("● container não está running"),
			"",
			StyleMuted.Render(truncate(content, width-4)),
			"",
			StyleMuted.Render("volte para Stats/Logs ou inicie o container"),
			StyleKey.Render("esc") + StyleMuted.Render(" lista  ") + StyleKey.Render("←→") + StyleMuted.Render(" abas"),
		}
		return renderApiTitledBox("TOP · PROCESSOS", fitExactLines(msg, height-2), width, height, true)
	}

	lines := strings.Split(content, "\n")
	procs := maxInt(0, len(lines)-1)
	cardH := 3
	boxW := maxInt(12, width/4)
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("PROCS", fmt.Sprintf("%d", procs), StyleMuted.Render("docker top"), StyleAccent, boxW, cardH),
		" ",
		renderStatsCard("STATUS", "running", StyleMuted.Render("daemon"), StyleHealthy, boxW, cardH),
		" ",
		renderStatsCard("REFRESH", "r / ↔", StyleMuted.Render("atalho"), StyleMuted, boxW, cardH),
	)
	restH := maxInt(6, height-cardH)
	viewport := maxInt(1, restH-2)
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(lines))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(lines))
	textW := maxInt(8, width-4)
	body := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		line := sliceColumns(sanitizeTerminalLine(lines[i]), a.containerDetailHScroll, textW)
		if i == 0 || (start == 0 && i == start) {
			body = append(body, StyleMuted.Render(line))
		} else {
			body = append(body, StyleNormal.Render(line))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		cards,
		renderApiTitledBox("PROCESS TABLE", fitExactLines(body, viewport), width, restH, true),
	)
}

func (a *App) renderContainerCodeView(width, height int) string {
	raw := a.containerDetailContent
	path := ""
	body := raw
	if strings.HasPrefix(strings.TrimSpace(raw), "#") {
		lines := strings.Split(raw, "\n")
		path = strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
		body = strings.Join(lines[1:], "\n")
		body = strings.TrimLeft(body, "\n")
	}
	lines := strings.Split(body, "\n")
	cardH := 3
	boxW := maxInt(14, width/3)
	title := "COMPOSE"
	if a.containerDetailTab == containerDetailTabFile {
		title = "DOCKERFILE"
	}
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("FILE", truncate(firstNonEmpty(path, "—"), boxW-4), StyleMuted.Render("path"), StyleAccent, boxW, cardH),
		" ",
		renderStatsCard("LINES", fmt.Sprintf("%d", len(lines)), StyleMuted.Render("conteúdo"), StyleNormal, boxW, cardH),
		" ",
		renderStatsCard("TAB", title, StyleMuted.Render("fonte"), StyleWarning, boxW, cardH),
	)
	restH := maxInt(6, height-cardH)
	viewport := maxInt(1, restH-2)
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(lines))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(lines))
	textW := maxInt(8, width-4)
	out := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		num := StyleMuted.Render(fmt.Sprintf("%4d ", i+1))
		out = append(out, num+StyleNormal.Render(sliceColumns(lines[i], a.containerDetailHScroll, textW-5)))
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		cards,
		renderApiTitledBox(title, fitExactLines(out, viewport), width, restH, true),
	)
}
