package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
)

func dockerAddStepBar(active dockerAddStep) string {
	parts := []struct {
		step  dockerAddStep
		label string
	}{
		{dockerAddStepSearch, "Busca"},
		{dockerAddStepResults, "Imagem"},
		{dockerAddStepEdit, "Compose"},
	}
	var out []string
	for i, p := range parts {
		label := fmt.Sprintf("%d %s", i+1, p.label)
		var piece string
		switch {
		case p.step == active:
			piece = StyleSelected.Render(" " + label + " ")
		case p.step < active:
			piece = StyleHealthy.Render("✓ " + label)
		default:
			piece = StyleMuted.Render(" " + label + " ")
		}
		out = append(out, piece)
		if i < len(parts)-1 {
			out = append(out, StyleMuted.Render(" › "))
		}
	}
	return strings.Join(out, "")
}

func dockerAddModalChrome(subtitle string, step dockerAddStep, ruleW int) []string {
	brand := lipgloss.NewStyle().Bold(true).Foreground(ColorDocker).Render("DOCKER HUB")
	if ruleW < 44 {
		ruleW = 44
	}
	return []string{
		brand + StyleMuted.Render("  ·  ") + StyleNormal.Render(subtitle),
		dockerAddStepBar(step),
		StyleMuted.Render(strings.Repeat("─", ruleW)),
	}
}

func dockerAddHelpLine(parts ...string) string {
	return StyleMuted.Render(strings.Join(parts, "  ·  "))
}

func dockerAddSourceBadge(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	switch source {
	case collectors.ComposeSourcePreset:
		return StyleHealthy.Render(" preset ")
	case collectors.ComposeSourceManifest:
		return StyleAccent.Render(" manifesto ")
	case collectors.ComposeSourceMinimal:
		return StyleWarning.Render(" mínimo ")
	case collectors.ComposeSourceManual:
		return StyleMuted.Render(" manual ")
	default:
		return StyleMuted.Render(" " + source + " ")
	}
}

func (a *App) renderDockerAddSearchBox() string {
	boxW := minInt(a.width-2, maxInt(56, a.width*88/100))
	boxH := minInt(a.height-2, maxInt(18, a.height*70/100))
	innerW := maxInt(24, boxW-6)

	input := a.dockerAddQuery
	if a.dockerAddLoading {
		input += " …"
	} else if a.dockerAddSearchFocus == dockerAddSearchQuery {
		input += "█"
	}
	placeholder := strings.TrimSpace(a.dockerAddQuery) == "" && a.dockerAddSearchFocus != dockerAddSearchQuery
	var fieldBody string
	switch {
	case placeholder:
		fieldBody = StyleMuted.Render("postgres  ·  redis  ·  nginx  ·  node…")
	case a.dockerAddSearchFocus == dockerAddSearchQuery:
		fieldBody = StyleSelected.Render(truncate(input, innerW-2))
	default:
		fieldBody = StyleNormal.Render(truncate(input, innerW-2))
	}
	fieldBorder := ColorBorder
	if a.dockerAddSearchFocus == dockerAddSearchQuery {
		fieldBorder = ColorDocker
	}
	field := StyleInnerPanel.Width(innerW).BorderForeground(fieldBorder).Render(fieldBody)

	chips := StyleMuted.Render("popular  ") +
		StyleNormal.Render("postgres") + StyleMuted.Render("  ") +
		StyleNormal.Render("mysql") + StyleMuted.Render("  ") +
		StyleNormal.Render("redis") + StyleMuted.Render("  ") +
		StyleNormal.Render("nginx") + StyleMuted.Render("  ") +
		StyleNormal.Render("node")

	var manual string
	if a.dockerAddSearchFocus == dockerAddSearchRefuse {
		manual = StyleSelected.Render("▸ Escrever YAML manualmente (sem Hub)")
	} else {
		manual = StyleInnerPanel.Width(innerW).Render(StyleMuted.Render("Escrever YAML manualmente (sem Hub)"))
	}

	status := StyleMuted.Render("hub.docker.com  ·  busca pública, sem login")
	if a.dockerAddLoading {
		status = StyleAccent.Render(a.spinner() + " buscando no Docker Hub…")
	}

	lines := dockerAddModalChrome("adicionar serviço ao compose", dockerAddStepSearch, innerW)
	lines = append(lines,
		"",
		StyleSection.Render("Buscar imagem"),
		status,
		field,
		chips,
		"",
		StyleMuted.Render("atalho"),
		manual,
		"",
		dockerAddHelpLine("enter busca", "tab alterna", "esc cancela"),
	)
	return StylePanel.Width(boxW).BorderForeground(ColorDocker).Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}

func (a *App) renderDockerAddResultsBox() string {
	boxW := minInt(a.width-2, maxInt(76, a.width-4))
	boxH := minInt(a.height-2, maxInt(28, a.height*92/100))
	innerW := maxInt(40, boxW-6)
	q := strings.TrimSpace(a.dockerAddQuery)
	repo, hasRepo := a.selectedDockerHubRepo()

	moreHint := ""
	if a.dockerAddLoading {
		moreHint = "  ·  " + a.spinner() + " carregando…"
	} else if a.dockerAddHasMore {
		moreHint = "  ·  ↓ carrega mais"
	}

	lines := dockerAddModalChrome("escolha a imagem do serviço", dockerAddStepResults, innerW)
	lines = append(lines, "",
		StyleMuted.Render(fmt.Sprintf("“%s”", truncate(q, 40)))+
			StyleMuted.Render(fmt.Sprintf("  ·  %d resultados%s", len(a.dockerAddResults), moreHint)),
	)

	// chrome(~5) + blank + body + blank + help
	bodyH := maxInt(16, boxH-7)
	gap := 1
	leftW := maxInt(28, minInt(40, innerW*32/100))
	rightW := innerW - leftW - gap
	if rightW < 40 {
		rightW = maxInt(36, innerW*55/100)
		leftW = maxInt(24, innerW-rightW-gap)
	}

	listLines := a.renderDockerAddResultList(leftW-2, bodyH-2)
	leftBox := renderApiTitledBox("Resultados", listLines, leftW, bodyH, a.dockerAddResultsFocus == dockerAddResultsList)
	rightCol := a.renderDockerAddRightColumn(repo, hasRepo, rightW, bodyH)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, strings.Repeat(" ", gap), rightCol)
	lines = append(lines, "")
	lines = append(lines, strings.Split(joined, "\n")...)
	lines = append(lines, "",
		dockerAddHelpLine("enter → compose", "tab troca painel", "↑↓ navega no painel", "o link", "esc volta"),
	)
	return StylePanel.Width(boxW).BorderForeground(ColorDocker).Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}

// renderDockerAddRightColumn: tag no topo, métricas no meio, detalhes com scroll embaixo.
func (a *App) renderDockerAddRightColumn(repo collectors.DockerHubRepo, hasRepo bool, width, height int) string {
	metricsH := 5
	imageH := maxInt(8, height*38/100)
	if height < 18 {
		imageH = 7
		metricsH = 4
	}
	detailsH := height - imageH - metricsH
	if detailsH < 5 {
		detailsH = 5
		imageH = maxInt(6, height-metricsH-detailsH)
	}

	imageBox := a.renderDockerAddImageBox(width, imageH)
	metricsBox := a.renderDockerAddMetricsBoxes(repo, hasRepo, width, metricsH)
	detailsBox := a.renderDockerAddDetailsBox(repo, hasRepo, width, detailsH)
	return lipgloss.JoinVertical(lipgloss.Left, imageBox, metricsBox, detailsBox)
}

func (a *App) renderDockerAddImageBox(width, height int) string {
	innerW := maxInt(8, width-2)
	innerH := maxInt(1, height-2)
	focused := a.dockerAddResultsFocus == dockerAddResultsImage

	img := a.dockerAddImage
	if focused {
		img += "█"
		img = StyleSelected.Render(truncate(img, innerW))
	} else if strings.TrimSpace(img) == "" {
		img = StyleMuted.Render("nome:tag  (ex: postgres:16)")
	} else {
		img = StyleNormal.Render(truncate(img, innerW))
	}

	lines := []string{img, StyleMuted.Render("tags recentes")}
	tagListH := maxInt(1, innerH-2)
	lines = append(lines, a.renderDockerAddTagList(innerW, tagListH, focused)...)

	title := "Imagem (ajuste tag se quiser)"
	if a.dockerAddTagsLoading && a.dockerAddTagsRepo != "" {
		title = a.spinner() + " Imagem · carregando tags…"
	} else if n := len(a.dockerAddTags); n > 0 {
		title = fmt.Sprintf("Imagem · %d tags", n)
	}
	return renderApiTitledBox(title, fitExactLines(lines, innerH), width, height, focused)
}

func (a *App) renderDockerAddTagList(width, height int, focused bool) []string {
	if a.dockerAddTagsLoading && len(a.dockerAddTags) == 0 {
		return fitExactLines([]string{StyleAccent.Render(a.spinner() + " buscando tags no Hub…")}, height)
	}
	if len(a.dockerAddTags) == 0 {
		return fitExactLines([]string{StyleMuted.Render("(sem tags · digite manualmente)")}, height)
	}

	a.dockerAddTagsScroll = ensureVisible(a.dockerAddTagCursor, a.dockerAddTagsScroll, height, len(a.dockerAddTags))
	start := a.dockerAddTagsScroll
	end := minInt(start+height, len(a.dockerAddTags))
	nameW := maxInt(10, width-12)

	var lines []string
	for i := start; i < end; i++ {
		t := a.dockerAddTags[i]
		size := collectors.FormatHubBytes(t.Size)
		row := fmt.Sprintf("%-*s  %s", nameW, truncate(t.Name, nameW), size)
		selected := focused && i == a.dockerAddTagCursor
		if selected {
			lines = append(lines, StyleSelected.Render("▸ "+truncate(row, width-2)))
		} else if i == a.dockerAddTagCursor {
			lines = append(lines, StyleNormal.Render("· "+truncate(row, width-2)))
		} else {
			lines = append(lines, StyleMuted.Render("  "+truncate(t.Name, nameW))+StyleMuted.Render("  "+size))
		}
	}
	return fitExactLines(lines, height)
}

func (a *App) renderDockerAddMetricsBoxes(repo collectors.DockerHubRepo, hasRepo bool, width, height int) string {
	stars, pulls, size, updated, tag := "—", "—", "—", "—", "—"
	if hasRepo {
		stars = collectors.FormatHubCount(int64(repo.Stars))
		pulls = collectors.FormatHubCount(repo.Pulls)
		if d, ok := a.dockerAddDetailsCache[repo.Name]; ok {
			if d.Stars > 0 {
				stars = collectors.FormatHubCount(int64(d.Stars))
			}
			if d.Pulls > 0 {
				pulls = collectors.FormatHubCount(d.Pulls)
			}
			size = collectors.FormatHubBytes(d.TagSize)
			updated = collectors.FormatHubRelative(d.LastUpdated)
			if d.Tag != "" {
				tag = d.Tag
			}
		} else if a.dockerAddDetailsLoading && a.dockerAddDetailsName == repo.Name {
			size, updated, tag = "…", "…", "…"
		}
		// Tag selecionada na lista prevalece no card SIZE/UPDATE.
		if a.dockerAddTagCursor >= 0 && a.dockerAddTagCursor < len(a.dockerAddTags) && a.dockerAddTagsRepo == repo.Name {
			t := a.dockerAddTags[a.dockerAddTagCursor]
			tag = t.Name
			if t.Size > 0 {
				size = collectors.FormatHubBytes(t.Size)
			}
			if !t.LastUpdated.IsZero() {
				updated = collectors.FormatHubRelative(t.LastUpdated)
			}
		}
	}

	gap := 1
	n := 4
	boxW := (width - gap*(n-1)) / n
	if boxW < 10 {
		// Telas estreitas: uma linha só.
		line := StyleMuted.Render(truncate(
			fmt.Sprintf("★ %s  ↓ %s  %s  %s", stars, pulls, size, updated), width-2,
		))
		return renderApiTitledBox("Métricas", []string{line}, width, height, false)
	}
	rest := width - (boxW+gap)*(n-1)
	boxes := []string{
		renderApiTitledBox("STARS", []string{StyleAccent.Bold(true).Render(truncate(stars, boxW-2)), StyleMuted.Render("favoritos")}, boxW, height, false),
		renderApiTitledBox("PULLS", []string{StyleAccent.Bold(true).Render(truncate(pulls, boxW-2)), StyleMuted.Render("downloads")}, boxW, height, false),
		renderApiTitledBox("SIZE", []string{StyleAccent.Bold(true).Render(truncate(size, boxW-2)), StyleMuted.Render(truncate("tag "+tag, boxW-2))}, boxW, height, false),
		renderApiTitledBox("UPDATE", []string{StyleAccent.Bold(true).Render(truncate(updated, rest-2)), StyleMuted.Render("último push")}, rest, height, false),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		boxes[0], strings.Repeat(" ", gap),
		boxes[1], strings.Repeat(" ", gap),
		boxes[2], strings.Repeat(" ", gap),
		boxes[3],
	)
}

func (a *App) renderDockerAddDetailsBox(repo collectors.DockerHubRepo, hasRepo bool, width, height int) string {
	focused := a.dockerAddResultsFocus == dockerAddResultsDetails
	innerH := maxInt(1, height-2)
	innerW := maxInt(8, width-2)
	content := a.dockerAddDetailsContent(repo, hasRepo, innerW)

	maxScroll := maxInt(0, len(content)-innerH)
	if a.dockerAddDetailsScroll > maxScroll {
		a.dockerAddDetailsScroll = maxScroll
	}
	if a.dockerAddDetailsScroll < 0 {
		a.dockerAddDetailsScroll = 0
	}
	start := a.dockerAddDetailsScroll
	end := minInt(start+innerH, len(content))
	view := append([]string{}, content[start:end]...)

	title := "Detalhes"
	switch {
	case maxScroll == 0:
		title = "Detalhes"
	case start == 0:
		title = fmt.Sprintf("Detalhes ↓ %d", len(content)-end)
	case end >= len(content):
		title = fmt.Sprintf("Detalhes ↑ %d", start)
	default:
		title = fmt.Sprintf("Detalhes ↑%d ↓%d", start, len(content)-end)
	}
	if focused {
		title += " · ↑↓"
	}
	return renderApiTitledBox(title, fitExactLines(view, innerH), width, height, focused)
}

func (a *App) dockerAddDetailsContent(repo collectors.DockerHubRepo, hasRepo bool, innerW int) []string {
	if !hasRepo {
		return []string{StyleMuted.Render("(selecione uma imagem na lista)")}
	}
	var lines []string
	title := StyleNormal.Bold(true).Render(truncate(repo.Name, maxInt(8, innerW-16)))
	badges := ""
	if repo.Official {
		badges += StyleHealthy.Render(" official")
	} else if repo.Automated {
		badges += StyleMuted.Render(" automated")
	}
	details, hasDetails := a.dockerAddDetailsCache[repo.Name]
	if hasDetails && len(details.Categories) > 0 {
		badges += StyleMuted.Render(" · " + truncate(details.Categories[0], maxInt(12, innerW/3)))
	}
	lines = append(lines, title+badges)

	if hasDetails {
		metaParts := []string{}
		if len(details.Architectures) > 0 {
			metaParts = append(metaParts, strings.Join(details.Architectures, " · "))
		}
		if len(details.OS) > 0 {
			metaParts = append(metaParts, strings.Join(details.OS, "/"))
		}
		if !details.DateRegistered.IsZero() {
			metaParts = append(metaParts, "desde "+details.DateRegistered.Format("2006"))
		}
		if details.Status != "" {
			metaParts = append(metaParts, details.Status)
		}
		for _, w := range wrapText(strings.Join(metaParts, "  ·  "), innerW) {
			lines = append(lines, StyleMuted.Render(w))
		}
	} else if a.dockerAddDetailsLoading && a.dockerAddDetailsName == repo.Name {
		lines = append(lines, StyleAccent.Render(a.spinner()+" carregando detalhes…"))
	}

	desc := strings.TrimSpace(repo.Description)
	if hasDetails {
		if d := strings.TrimSpace(details.Description); d != "" {
			desc = d
		}
	}
	if desc == "" {
		lines = append(lines, StyleMuted.Render("(sem descrição no Hub)"))
	} else {
		lines = append(lines, "")
		for _, w := range wrapText(desc, innerW) {
			lines = append(lines, StyleNormal.Render(w))
		}
	}
	if hasDetails {
		overview := strings.TrimSpace(details.Overview)
		if overview != "" && !strings.EqualFold(overview, desc) && !strings.HasPrefix(strings.ToLower(overview), strings.ToLower(desc)) {
			lines = append(lines, "", StyleMuted.Render("overview"))
			for _, w := range wrapText(overview, innerW) {
				lines = append(lines, StyleMuted.Render(w))
			}
		}
	}
	hubURL := collectors.DockerHubURL(repo.Name)
	if hubURL != "" {
		lines = append(lines, "", StyleAccent.Underline(true).Render(truncate(hubURL, innerW)))
		lines = append(lines, StyleMuted.Render("o  abre no browser"))
	}
	return lines
}

func (a *App) renderDockerAddResultList(width, height int) []string {
	if height < 3 {
		height = 3
	}
	listFocus := a.dockerAddResultsFocus == dockerAddResultsList
	start := 0
	if a.dockerAddCursor >= height-1 {
		start = a.dockerAddCursor - (height - 2)
	}
	end := minInt(start+height-1, len(a.dockerAddResults))
	if end < start {
		end = start
	}
	nameW := maxInt(12, width-18)

	var lines []string
	lines = append(lines, StyleMuted.Render(fmt.Sprintf("%-*s  %5s  %6s", nameW, "IMAGEM", "STARS", "PULLS")))
	for i := start; i < end; i++ {
		r := a.dockerAddResults[i]
		name := truncate(r.Name, nameW)
		stars := collectors.FormatHubCount(int64(r.Stars))
		pulls := collectors.FormatHubCount(r.Pulls)
		badge := ""
		if r.Official {
			badge = StyleHealthy.Render(" ●")
		}
		selected := listFocus && i == a.dockerAddCursor
		rowPlain := fmt.Sprintf("%-*s  %5s  %6s", nameW, name, stars, pulls)
		if selected {
			lines = append(lines, StyleSelected.Render("▸ "+truncate(stripANSI(rowPlain), maxInt(8, width-2)))+badge)
		} else {
			line := StyleNormal.Render("  "+truncate(name, nameW)) +
				StyleMuted.Render(fmt.Sprintf("  %5s  %6s", stars, pulls)) + badge
			lines = append(lines, line)
		}
	}
	if start > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
	}
	remain := len(a.dockerAddResults) - end
	if remain > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", remain)))
	} else if a.dockerAddHasMore {
		lines = append(lines, StyleAccent.Render("  ↓ carregar mais"))
	}
	return fitExactLines(lines, height)
}

func (a *App) renderDockerAddEditBox() string {
	boxW := minInt(a.width-2, maxInt(70, a.width-4))
	boxH := minInt(a.height-2, maxInt(22, a.height*90/100))
	innerW := maxInt(32, boxW-6)

	summary := dockerAddComposeSummary(a.dockerAddEdit)
	badge := dockerAddSourceBadge(a.dockerAddComposeSource)
	meta := StyleMuted.Render("mescla no compose do projeto")
	if summary.image != "" {
		meta = StyleMuted.Render("imagem  ") + StyleNormal.Render(truncate(summary.image, 40)) +
			StyleMuted.Render("  ·  mescla no projeto")
	}
	if badge != "" {
		meta += StyleMuted.Render("  ·") + badge
	}

	editorH := maxInt(8, boxH-11)
	editing := a.dockerAddFocus == dockerAddFocusEditor
	ed := a.dockerAddEditState
	body := renderEditorLines(a.dockerAddEdit, &ed, maxInt(20, innerW-2), maxInt(4, editorH-2), editing, false)
	a.dockerAddEditState = ed
	editorBox := renderApiTitledBox("docker-compose YAML", body, innerW, editorH, editing)

	saveBtn := StyleMuted.Render("  Salvar no compose  ")
	cancelBtn := StyleMuted.Render("  Cancelar  ")
	switch a.dockerAddFocus {
	case dockerAddFocusSave:
		saveBtn = StyleSelected.Render("▸ Salvar no compose ◂")
	case dockerAddFocusCancel:
		cancelBtn = StyleSelected.Render("▸ Cancelar ◂")
	}

	stats := StyleMuted.Render(fmt.Sprintf(
		"serviço %s  ·  %d ports  ·  %d env  ·  %d volumes",
		firstNonEmpty(summary.service, "—"), summary.ports, summary.env, summary.volumes,
	))

	lines := dockerAddModalChrome("revisar e salvar o serviço", dockerAddStepEdit, innerW)
	lines = append(lines, "", meta, stats, "")
	lines = append(lines, strings.Split(editorBox, "\n")...)
	lines = append(lines, "",
		saveBtn+StyleMuted.Render("    ")+cancelBtn,
		dockerAddHelpLine("tab troca foco", "enter no botão confirma", "esc sai"),
	)
	return StylePanel.Width(boxW).BorderForeground(ColorDocker).Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}

type dockerAddComposeSummaryInfo struct {
	service string
	image   string
	ports   int
	env     int
	volumes int
}

func dockerAddComposeSummary(yamlText string) dockerAddComposeSummaryInfo {
	info := dockerAddComposeSummaryInfo{image: dockerAddImageFromYAML(yamlText)}
	inPorts, inEnv, inVols := false, false, false
	for _, line := range strings.Split(yamlText, "\n") {
		trim := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 2 && strings.HasSuffix(trim, ":") && trim != "services:" {
			name := strings.TrimSuffix(trim, ":")
			switch name {
			case "volumes", "networks", "configs", "secrets":
			default:
				if info.service == "" && name != "" && !strings.Contains(name, " ") {
					info.service = name
				}
			}
		}

		switch trim {
		case "ports:":
			inPorts, inEnv, inVols = true, false, false
			continue
		case "environment:":
			inPorts, inEnv, inVols = false, true, false
			continue
		case "volumes:":
			inPorts, inEnv, inVols = false, false, indent >= 4
			if indent == 0 {
				inPorts, inEnv, inVols = false, false, false
			}
			continue
		}

		if indent <= 4 && strings.HasSuffix(trim, ":") &&
			trim != "ports:" && trim != "environment:" && trim != "volumes:" &&
			!strings.HasPrefix(trim, "-") {
			if indent <= 4 {
				inPorts, inEnv, inVols = false, false, false
			}
		}

		if strings.HasPrefix(trim, "- ") {
			if inPorts {
				info.ports++
			}
			if inVols && indent >= 4 {
				info.volumes++
			}
		}
		if inEnv && strings.Contains(trim, ":") && !strings.HasPrefix(trim, "-") && trim != "environment:" {
			info.env++
		}
	}
	return info
}

func dockerAddImageFromYAML(yamlText string) string {
	for _, line := range strings.Split(yamlText, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "image:") {
			return strings.TrimSpace(strings.TrimPrefix(trim, "image:"))
		}
	}
	return ""
}
