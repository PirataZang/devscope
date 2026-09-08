package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// renderContainerDetailChrome monta a tela no padrão dos outros módulos:
// barra de identificação, régua de abas numeradas e barra de comandos larga.
// Some a moldura arredondada externa — ela custava 2 colunas e 2 linhas e não
// separava nada, já que a tela ocupa tudo.
func (a *App) renderContainerDetailChrome(body string) string {
	w := maxInt(40, a.width)
	h := maxInt(12, a.height-1)

	header := a.renderContainerDetailHeader(w)
	tabs := a.renderContainerDetailTabBar(w)
	cmdBar := a.renderContainerDetailCommandBar(w)

	stack := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)
}

// renderContainerDetailHeader: quem é o container, como está e onde escuta —
// as três perguntas antes de mexer em qualquer coisa.
func (a *App) renderContainerDetailHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabContainers)).Bold(true)
	left := accent.Render("▣ CONTAINER") +
		StyleMuted.Render("   ") + StyleNormal.Bold(true).Render(truncate(a.containerDetailName, 28))

	var right []string
	if c, ok := a.containerDetailContainer(); ok {
		wave, waveStyle, label, labelStyle := a.containerStateVisual(c)
		left += "   " + waveStyle.Render(wave) + " " + labelStyle.Render(label)
		if c.Image != "" {
			right = append(right, StyleMuted.Render(elideLeft(c.Image, 30)))
		}
		if ports := containerPortsLabel(c); ports != emDash {
			right = append(right, lipgloss.NewStyle().Foreground(ColorAccent).Render(ports))
		}
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// containerDetailContainer acha o container aberto na lista do projeto.
func (a *App) containerDetailContainer() (core.Container, bool) {
	p := a.currentProject()
	if p == nil {
		return core.Container{}, false
	}
	for _, c := range p.Containers {
		if (a.containerDetailID != "" && c.ID == a.containerDetailID) || c.Name == a.containerDetailName {
			return c, true
		}
	}
	return core.Container{}, false
}

// renderContainerDetailCommandBar substitui a coluna AÇÕES: os comandos por
// extenso, em até duas linhas, sem roubar largura do conteúdo.
func (a *App) renderContainerDetailCommandBar(width int) string {
	items := [][2]string{{"1-7", "abas"}}
	if a.containerDetailTab == containerDetailTabStats {
		// Métricas não rola nem busca — oferecer o atalho seria mentira.
		items = append(items,
			[2]string{"r", "recarregar"},
			[2]string{"esc", "voltar"},
		)
		return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
	}
	items = append(items,
		[2]string{"↑↓", "rolar"},
		[2]string{"←→", "lateral"},
	)
	if a.containerDetailHScroll > 0 {
		items = append(items, [2]string{"0", "voltar ao início"})
	}
	items = append(items, [2]string{"/", "buscar"})
	if a.containerDetailSearchQuery != "" {
		items = append(items, [2]string{"n/N", "ocorrência"})
	}
	if a.containerDetailTab == containerDetailTabLogs {
		items = append(items,
			[2]string{"f", "acompanhar"},
			[2]string{"p", "pausar"},
		)
	}
	items = append(items,
		[2]string{"r", "recarregar"},
		[2]string{"esc", "voltar"},
	)
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
}

// renderContainerDetailRichBody: um painel só, largura cheia, com numeração de
// linha e realce conforme o conteúdo. Antes cada aba montava caixas próprias —
// cinco cards de um número, uma coluna AÇÕES e um painel INSPECT que só
// repetia os atalhos — e a leitura sobrava em menos de metade da tela.
func (a *App) renderContainerDetailRichBody(width, height int) string {
	title := strings.ToUpper(a.containerDetailTab.shortLabel())
	if a.containerDetailLoading {
		return renderApiTitledBox(title,
			fitExactLines([]string{a.loadingText("carregando " + strings.ToLower(title) + "…")}, height-2),
			width, height, true)
	}

	all := a.containerDetailLines()
	if len(all) == 0 {
		return renderApiTitledBox(title,
			fitExactLines([]string{StyleMuted.Render(a.containerDetailEmptyHint())}, height-2),
			width, height, true)
	}
	title = fmt.Sprintf("%s · %d linhas", title, len(all))
	if a.containerDetailTab == containerDetailTabLogs {
		errN, warnN := countLogLevels(all)
		if errN > 0 {
			title += fmt.Sprintf(" · %d erros", errN)
		}
		if warnN > 0 {
			title += fmt.Sprintf(" · %d avisos", warnN)
		}
	}

	viewport := maxInt(1, height-2)
	a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(all))
	start := a.containerDetailScroll
	end := minInt(start+viewport, len(all))

	// Gutter: numerar a linha é o que permite falar sobre ela ("olha a 412").
	gutter := len(fmt.Sprintf("%d", len(all)))
	if gutter < 3 {
		gutter = 3
	}
	textW := a.containerDetailTextWidth()
	a.containerDetailHScroll = clampScroll(a.containerDetailHScroll, textW, a.containerDetailMaxLineWidth())

	matchSet := a.containerDetailMatchLineSet()
	current := -1
	if m := a.containerDetailSearchMatches(); len(m) > 0 && a.containerDetailSearchIdx < len(m) {
		current = m[a.containerDetailSearchIdx]
	}

	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		num := StyleMuted.Render(padLeft(fmt.Sprintf("%d", i+1), gutter))
		if i == current {
			num = StyleKey.Render(padLeft(fmt.Sprintf("%d", i+1), gutter))
		}
		lines = append(lines, num+StyleMuted.Render(" │ ")+
			a.renderContainerDetailCodeLine(all[i], textW, matchSet[i], i == current))
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, true)
}

// renderContainerDetailCodeLine aplica o corte lateral e então o realce. A
// ordem importa: realçar antes do corte quebraria os escapes ANSI.
func (a *App) renderContainerDetailCodeLine(line string, width int, matched, current bool) string {
	line = sanitizeTerminalLine(line)
	shown := sliceColumns(line, a.containerDetailHScroll, width)
	switch {
	case current:
		return StyleDiffMatch.Render(shown)
	case matched:
		return StyleSelected.Render(shown)
	default:
		return highlightDetailLine(a.containerDetailTab, shown)
	}
}

func (a *App) containerDetailEmptyHint() string {
	switch a.containerDetailTab {
	case containerDetailTabLogs:
		return "sem saída ainda — r recarrega, f acompanha ao vivo"
	case containerDetailTabCompose:
		return "nenhum docker-compose encontrado para este projeto"
	case containerDetailTabEnv:
		return "o container não expõe variáveis de ambiente"
	default:
		return "sem conteúdo — r recarrega"
	}
}
