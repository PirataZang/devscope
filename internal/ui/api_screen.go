package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// ── Tela do API ───────────────────────────────────────────────────────────────
//
// Reescrita no vocabulário de Postman/Thunder Client, e no padrão de telas do
// DevScope (docs/DESIGN.md §2): cabeçalho, régua de abas NUMERADA, corpo em
// painéis e barra de comandos larga no rodapé.
//
// O QUE ESTAVA ERRADO ANTES:
//
//   - a tela era uma pilha lazydocker: [1]-Request (uma caixa de sete linhas só
//     pra listar cinco métodos), [2]-URL, [3]-Headers e [4]-Auth empilhados numa
//     coluna estreita, enquanto Body e Response DISPUTAVAM o painel da direita
//     por `[` e `]`. Ver o corpo que você enviou e a resposta ao mesmo tempo era
//     impossível — e é justamente isso que se faz o dia inteiro;
//   - a URL aparecia duas vezes (no cabeçalho e na caixa [2]) e em nenhuma das
//     duas dava pra ver a query inteira;
//   - a coluna vertical AÇÕES roubava 17 colunas do conteúdo — padrão que o
//     DESIGN.md §2.3 já tinha substituído pela barra de comandos no resto do app;
//   - a query era editada à mão dentro da URL: sem tabela de params, sem
//     desligar um parâmetro sem apagá-lo.
//
// O QUE A TELA NOVA FAZ:
//
//	◆ API  projeto        GET · 200 OK · 142ms · 1.2 kB · 09:34:12   <- cabeçalho
//	 1 PARAMS 2 │ 2 HEADERS 3 │ 3 AUTH • │ 4 BODY │ 5 RESPOSTA       <- abas numeradas
//	┌─ REQUISIÇÃO ────────────────────────────────────────────────┐
//	│  GET ▾  http://localhost:8080/users?page=1      ⏎ ENVIAR     │  <- a linha do Postman
//	└─────────────────────────────────────────────────────────────┘
//	┌─ PARAMS · 2 de 3 ativos ────────────────────────────────────┐
//	│  ✓ page                  1                                  │
//	└─────────────────────────────────────────────────────────────┘
//	┌─ RESPOSTA · 200 OK · 142ms · corpo ─────────────────────────┐
//	│   1 │ {                                                     │
//	└─────────────────────────────────────────────────────────────┘
//	 1-5 abas · tab foco · enter enviar · m método · u url · ...    <- barra de comandos
//
// A requisição e a resposta ficam VISÍVEIS JUNTAS, que é o ponto do Postman. Em
// terminal curto (corpo < 20 linhas) a divisão não paga a borda: aí a tela mostra
// só a faixa em foco, e a aba 5 abre a resposta inteira.

// apiSection é a aba da requisição. O número é a única forma de trocar (§3).
type apiSection int

const (
	apiSecParams apiSection = iota
	apiSecHeaders
	apiSecAuth
	apiSecBody
	apiSecResponse
)

var apiSectionNames = [5]string{"PARAMS", "HEADERS", "AUTH", "BODY", "RESPOSTA"}

// apiFocus é a faixa que responde ao teclado. Três, não cinco: a linha da
// requisição, o painel da aba e a resposta.
type apiFocus int

const (
	apiFocusURL apiFocus = iota
	apiFocusSection
	apiFocusResponse
)

// apiRespView escolhe o que a resposta mostra.
type apiRespView int

const (
	apiRespBody apiRespView = iota
	apiRespHeaders
)

func (a *App) renderApiTab(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(12, a.height-1)

	header := a.renderApiHeader(p, w)
	tabs := a.renderApiTabBar(w)
	cmdBar := a.renderApiCommandBar(w)
	bodyH := maxInt(6, h-lipgloss.Height(header)-lipgloss.Height(tabs)-lipgloss.Height(cmdBar))
	body := a.renderApiBody(w, bodyH)

	stack := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)
}

// ── Cabeçalho ─────────────────────────────────────────────────────────────────

// renderApiHeader responde as três perguntas de quem chega: em que projeto estou,
// o que vou disparar e como foi a última resposta.
func (a *App) renderApiHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabAPI)).Bold(true)
	left := accent.Render(tabGlyph(TabAPI) + " API")
	if p != nil {
		left += StyleMuted.Render("   ") + StyleNormal.Bold(true).Render(truncate(p.Name, 24))
	}
	method := a.apiMethodOrGet()
	left += "   " + apiMethodStyle(method).Render(method)

	var right []string
	switch {
	case a.apiLoading:
		right = append(right, StyleWarning.Render(a.spinner()+" enviando…"))
	case a.apiResponseErr != "":
		right = append(right, StyleUnhealthy.Render("● falhou"))
	case a.apiResponseStatus != "":
		right = append(right,
			a.apiStatusStyle().Render("● "+a.apiResponseStatus),
			StyleMuted.Render(a.apiResponseTime.Round(time.Millisecond).String()),
			StyleMuted.Render(apiHumanSize(len(a.apiResponseBody))),
		)
	default:
		right = append(right, StyleMuted.Render("nada enviado ainda"))
	}
	if n := len(a.apiHistory); n > 0 {
		right = append(right, StyleMuted.Render(fmt.Sprintf("%d no histórico", n)))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

// renderApiTabBar é a régua numerada. O número de cada aba mostra quantas linhas
// ATIVAS ela tem (como o Postman): dá pra ver que há header ligado sem abrir.
func (a *App) renderApiTabBar(width int) string {
	compact := width < 82
	var b strings.Builder
	for i, name := range apiSectionNames {
		if i > 0 {
			b.WriteString(StyleMuted.Render("│"))
		}
		sec := apiSection(i)
		badge := a.apiSectionBadge(sec)
		label := fmt.Sprintf(" %d %s%s ", i+1, name, badge)
		if compact && sec != a.apiSection {
			label = fmt.Sprintf(" %d ", i+1)
		}
		if sec == a.apiSection {
			b.WriteString(StyleSelected.Render(label))
			continue
		}
		b.WriteString(StyleMuted.Render(label))
	}
	right := ""
	switch {
	case a.apiEditing:
		right = StyleWarning.Render("editando · esc conclui")
	case a.apiSearchQuery != "":
		matches := a.apiSearchMatches()
		if len(matches) == 0 {
			right = StyleMuted.Render("busca: 0 de 0")
		} else {
			right = StyleAccent.Render(fmt.Sprintf("busca: %d de %d", a.apiSearchIdx+1, len(matches)))
		}
	default:
		right = StyleMuted.Render(a.apiFocusLabel())
	}
	return joinWithSpacer(truncateVisible(b.String(), width), right, width)
}

// apiSectionBadge é o contador que vai no rótulo da aba.
func (a *App) apiSectionBadge(sec apiSection) string {
	switch sec {
	case apiSecParams:
		if n := apiEnabledCount(a.apiParams); n > 0 {
			return "·" + strconv.Itoa(n)
		}
	case apiSecHeaders:
		if n := apiEnabledCount(apiParseHeaderRows(a.apiHeaders)); n > 0 {
			return "·" + strconv.Itoa(n)
		}
	case apiSecAuth:
		if a.apiAuthType != apiAuthNone {
			return "·•"
		}
	case apiSecBody:
		if strings.TrimSpace(a.apiBody) != "" {
			return "·•"
		}
	case apiSecResponse:
		if a.apiResponseCode > 0 {
			return "·" + strconv.Itoa(a.apiResponseCode)
		}
	}
	return ""
}

func (a *App) apiFocusLabel() string {
	switch a.apiFocus {
	case apiFocusURL:
		return "foco: url"
	case apiFocusResponse:
		return "foco: resposta"
	default:
		return "foco: " + strings.ToLower(apiSectionNames[a.apiSection])
	}
}

// ── Corpo ─────────────────────────────────────────────────────────────────────

// renderApiBody empilha a linha da requisição, o painel da aba e a resposta. A
// resposta só divide a tela quando há altura que pague a borda; senão quem manda
// é o foco, e a aba 5 abre a resposta inteira.
func (a *App) renderApiBody(width, height int) string {
	reqLine := a.renderApiRequestLine(width)
	rest := maxInt(3, height-lipgloss.Height(reqLine))

	if a.apiSection == apiSecResponse {
		return lipgloss.JoinVertical(lipgloss.Left, reqLine, a.renderApiResponse(width, rest))
	}
	if rest < 20 {
		// Terminal curto: uma faixa só. Duas caixas de 4 linhas não mostram nada.
		if a.apiFocus == apiFocusResponse {
			return lipgloss.JoinVertical(lipgloss.Left, reqLine, a.renderApiResponse(width, rest))
		}
		return lipgloss.JoinVertical(lipgloss.Left, reqLine, a.renderApiSection(width, rest))
	}
	// A aba ocupa o que o conteúdo dela pede, e não uma fração fixa: tabela de
	// três linhas não precisa de dez, e a sobra vale mais na resposta — que é o
	// painel que se lê rolando.
	secH := a.apiSectionHeight() + 2
	if secH < 6 {
		secH = 6
	}
	if max := rest * 55 / 100; secH > max {
		secH = max
	}
	if max := rest - 6; secH > max {
		secH = max
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		reqLine,
		a.renderApiSection(width, secH),
		a.renderApiResponse(width, rest-secH),
	)
}

// apiSectionHeight é quantas linhas o conteúdo da aba ativa pede.
func (a *App) apiSectionHeight() int {
	switch a.apiSection {
	case apiSecParams:
		return len(a.apiParams) + 1 // +1 = a linha de criar
	case apiSecHeaders:
		return len(apiParseHeaderRows(a.apiHeaders)) + 1
	case apiSecAuth:
		if a.apiAuthType == apiAuthBasic {
			return 6
		}
		return 5
	default: // corpo
		if strings.TrimSpace(a.apiBody) == "" {
			return 3
		}
		return len(strings.Split(a.apiBody, "\n"))
	}
}

// renderApiRequestLine é a assinatura do Postman: método, URL e o botão de
// enviar numa linha só, do tamanho da tela.
func (a *App) renderApiRequestLine(width int) string {
	inner := maxInt(20, width-4)
	method := a.apiMethodOrGet()
	badge := apiMethodStyle(method).Render(method + " ▾")
	send := " ⏎ ENVIAR "
	switch {
	case a.apiLoading:
		send = " " + a.spinner() + " ENVIANDO "
	case a.apiFocus == apiFocusURL:
		send = " ⏎ ENVIAR "
	}
	sendStyle := StyleMuted
	if a.apiFocus == apiFocusURL && !a.apiLoading {
		sendStyle = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorSuccess).Bold(true)
	} else if a.apiLoading {
		sendStyle = StyleWarning
	}
	sendBox := sendStyle.Render(send)

	urlW := maxInt(8, inner-lipgloss.Width(badge)-lipgloss.Width(sendBox)-5)
	editing := a.apiEditing && a.apiFocus == apiFocusURL
	selLo, selHi, _ := a.apiSelRange()
	shown := fitApiFieldWindowSel(a.apiURL, a.apiEditorCursor, urlW, editing, selLo, selHi)
	if strings.TrimSpace(a.apiURL) == "" && !editing {
		shown = StyleMuted.Render("cole ou digite a URL — u edita")
	}
	urlStyle := StyleNormal
	if a.apiFocus == apiFocusURL {
		urlStyle = StyleSelected
	}
	line := " " + badge + "  " + urlStyle.Render(padRightVisible(shown, urlW)) + "  " + sendBox
	return panelBox(a.apiRequestTitle(), []string{line}, width, 3, a.apiFocus == apiFocusURL)
}

func (a *App) apiRequestTitle() string {
	title := "REQUISIÇÃO"
	if a.apiFocus == apiFocusURL && a.apiEditing {
		return panelTitle(title, "editando a URL")
	}
	if n := apiEnabledCount(a.apiParams); n > 0 {
		return panelTitle(title, fmt.Sprintf("%d params na query", n))
	}
	return title
}

// renderApiSection desenha o painel da aba ativa.
func (a *App) renderApiSection(width, height int) string {
	focused := a.apiFocus == apiFocusSection
	innerW, innerH := maxInt(8, width-2), maxInt(1, height-2)
	var title string
	var lines []string

	switch a.apiSection {
	case apiSecParams:
		rows := a.apiParams
		title = panelTitle("PARAMS", apiRowsSummary(rows, "parâmetro"))
		lines = a.renderApiRowTable(rows, innerW, innerH, focused, "nenhum parâmetro — n cria a primeira linha")
	case apiSecHeaders:
		rows := apiParseHeaderRows(a.apiHeaders)
		title = panelTitle("HEADERS", apiRowsSummary(rows, "header"))
		lines = a.renderApiRowTable(rows, innerW, innerH, focused, "nenhum header — n cria a primeira linha")
	case apiSecAuth:
		title = panelTitle("AUTH", apiAuthLabel(a.apiAuthType))
		lines = a.renderApiAuthLines(innerW, innerH)
	default: // apiSecBody
		title = panelTitle("BODY", apiBodySummary(a.apiBody, a.apiMethodOrGet()))
		lines = a.renderApiBodyLines(innerW, innerH, focused)
	}
	return panelBox(title, fitExactLines(lines, innerH), width, height, focused)
}

func apiRowsSummary(rows []apiKV, noun string) string {
	on := apiEnabledCount(rows)
	switch {
	case len(rows) == 0:
		return "vazio"
	case on == len(rows):
		return fmt.Sprintf("%d %s", len(rows), pluralPT(noun, len(rows)))
	default:
		return fmt.Sprintf("%d de %d ativos", on, len(rows))
	}
}

func pluralPT(noun string, n int) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

func apiBodySummary(body, method string) string {
	if !methodAllowsBody(method) {
		return method + " não leva corpo"
	}
	if strings.TrimSpace(body) == "" {
		return "vazio · e edita"
	}
	return fmt.Sprintf("%d linhas · %s", len(strings.Split(body, "\n")), apiHumanSize(len(body)))
}

// renderApiRowTable é a tabela de params/headers: caixinha, chave e valor. A
// linha extra no fim é o "nova linha" do Postman — ela existe pra você não ter
// de saber que a tecla é `n`.
func (a *App) renderApiRowTable(rows []apiKV, width, height int, focused bool, empty string) []string {
	keyW := clampInt(width*36/100, 10, 26)
	valW := maxInt(6, width-keyW-6)

	total := len(rows) + 1 // +1 = a linha de criar
	a.apiRowScroll = clampScroll(a.apiRowScroll, height, total)
	if focused {
		// Mantém o cursor à vista sem deixar a rolagem "pular" quando ele anda.
		if a.apiRowCursor < a.apiRowScroll {
			a.apiRowScroll = a.apiRowCursor
		}
		if a.apiRowCursor >= a.apiRowScroll+height {
			a.apiRowScroll = a.apiRowCursor - height + 1
		}
	}
	if len(rows) == 0 && !focused {
		return []string{StyleMuted.Render(truncate(empty, width))}
	}

	out := make([]string, 0, height)
	for i := a.apiRowScroll; i < total && len(out) < height; i++ {
		cursor := focused && i == a.apiRowCursor
		if i == len(rows) { // linha de criar
			line := "  " + StyleMuted.Render("+ nova linha")
			if cursor {
				line = StyleSelected.Render(padRightVisible("  + nova linha  (enter)", width))
			}
			out = append(out, line)
			continue
		}
		r := rows[i]
		box := StyleMuted.Render("○")
		if r.On {
			box = StyleHealthy.Render("✓")
		}
		key, val := r.Key, r.Value
		editing := cursor && a.apiEditing
		selLo, selHi, _ := a.apiSelRange()
		if editing && a.apiRowField == 0 {
			key = fitApiFieldWindowSel(key, a.apiEditorCursor, keyW, true, selLo, selHi)
		} else {
			key = truncate(key, keyW)
		}
		if editing && a.apiRowField == 1 {
			val = fitApiFieldWindowSel(val, a.apiEditorCursor, valW, true, selLo, selHi)
		} else {
			val = truncate(val, valW)
		}

		keyStyle, valStyle := StyleNormal, StyleMuted
		if !r.On {
			keyStyle, valStyle = StyleMuted, StyleMuted
		}
		if cursor && a.apiRowField == 0 {
			keyStyle = StyleSelected
		}
		if cursor && a.apiRowField == 1 {
			valStyle = StyleSelected
		}
		line := " " + box + " " + keyStyle.Render(padRightVisible(key, keyW)) + "  " + valStyle.Render(padRightVisible(val, valW))
		if cursor && !a.apiEditing {
			line = StyleMuted.Render("▌") + line[len(" "):]
		}
		out = append(out, line)
	}
	return out
}

// renderApiAuthLines é o seletor de tipo + os campos, no formato de formulário do
// Thunder Client: rótulo à esquerda, valor à direita, segredo mascarado.
func (a *App) renderApiAuthLines(width, height int) []string {
	focused := a.apiFocus == apiFocusSection
	seg := a.renderApiAuthSegments(width, focused)
	lines := []string{seg, ""}

	label := func(s string) string { return StyleMuted.Render(padRightVisible(s, 8)) }
	fieldW := maxInt(8, width-10)
	selLo, selHi, _ := a.apiSelRange()

	switch a.apiAuthType {
	case apiAuthBearer:
		val := ""
		switch {
		case a.apiEditing && focused:
			val = StyleSelected.Render(padRightVisible(
				fitApiFieldWindowSel(a.apiAuthToken, a.apiEditorCursor, fieldW, true, selLo, selHi), fieldW))
		case strings.TrimSpace(a.apiAuthToken) == "":
			val = StyleMuted.Render("(vazio) · e edita")
		default:
			val = StyleNormal.Render(apiMaskSecret(a.apiAuthToken, fieldW))
		}
		lines = append(lines, label("token")+val)
		lines = append(lines, "", StyleMuted.Render(truncate("vai como Authorization: Bearer …", width)))
	case apiAuthBasic:
		user, pass := a.apiAuthUser, apiMaskSecret(a.apiAuthPass, fieldW)
		if strings.TrimSpace(user) == "" {
			user = StyleMuted.Render("(vazio)")
		} else {
			user = StyleNormal.Render(truncate(user, fieldW))
		}
		if strings.TrimSpace(a.apiAuthPass) == "" {
			pass = StyleMuted.Render("(vazio)")
		} else {
			pass = StyleNormal.Render(pass)
		}
		if a.apiEditing && focused {
			cur := fitApiFieldWindowSel(a.apiAuthUser, a.apiEditorCursor, fieldW, true, selLo, selHi)
			if a.apiAuthEditPass {
				cur = fitApiFieldWindowSel(strings.Repeat("•", len([]rune(a.apiAuthPass))),
					a.apiEditorCursor, fieldW, true, selLo, selHi)
				pass = StyleSelected.Render(padRightVisible(cur, fieldW))
			} else {
				user = StyleSelected.Render(padRightVisible(cur, fieldW))
			}
		}
		lines = append(lines, label("usuário")+user, label("senha")+pass)
		hint := "e edita · tab troca de campo"
		if !a.apiEditing {
			hint = "e edita o usuário · tab a senha"
		}
		lines = append(lines, "", StyleMuted.Render(truncate(hint, width)))
	default:
		lines = append(lines, StyleMuted.Render(truncate("a requisição vai sem cabeçalho de autenticação", width)))
		lines = append(lines, "", StyleMuted.Render(truncate("a alterna o tipo · bearer ou basic", width)))
	}
	return fitExactLines(lines, height)
}

// renderApiAuthSegments é o controle segmentado: o tipo ativo em destaque, os
// outros apagados. Vale mais que uma lista vertical — ocupa uma linha e mostra
// as três opções ao mesmo tempo.
func (a *App) renderApiAuthSegments(width int, focused bool) string {
	names := [3]string{"nenhuma", "bearer", "basic"}
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteString(" ")
		}
		if apiAuthType(i) == a.apiAuthType {
			style := StyleTabActive
			if focused {
				style = StyleSelected
			}
			b.WriteString(style.Render(" " + n + " "))
			continue
		}
		b.WriteString(StyleMuted.Render(" " + n + " "))
	}
	return truncate(b.String(), width)
}

// renderApiBodyLines: o editor de corpo, com numeração de linha e realce de JSON.
func (a *App) renderApiBodyLines(width, height int, focused bool) []string {
	if a.apiEditing && focused {
		return a.renderApiMultilineEdit(a.apiBody, width, height, true)
	}
	if strings.TrimSpace(a.apiBody) == "" {
		hint := []string{StyleMuted.Render("corpo vazio")}
		if methodAllowsBody(a.apiMethodOrGet()) {
			hint = append(hint, "", StyleMuted.Render("e abre o editor · o JSON ganha realce e indentação"))
		} else {
			hint = append(hint, "", StyleMuted.Render(a.apiMethodOrGet()+" normalmente não leva corpo — m troca o método"))
		}
		return hint
	}
	raw := strings.Split(a.apiBody, "\n")
	a.apiEditorScroll = clampScroll(a.apiEditorScroll, height, len(raw))
	gutter := len(strconv.Itoa(len(raw)))
	out := make([]string, 0, height)
	for i := a.apiEditorScroll; i < len(raw) && len(out) < height; i++ {
		num := StyleMuted.Render(padLeft(strconv.Itoa(i+1), gutter) + " │ ")
		text := renderJSONColumns(sanitizeTerminalLine(raw[i]), a.apiHScroll, maxInt(4, width-gutter-3))
		out = append(out, num+text)
	}
	return out
}

// ── Resposta ──────────────────────────────────────────────────────────────────

// renderApiResponse é o painel de baixo: status, tempo, tamanho e a chave do
// Postman — corpo e cabeçalhos como VISTAS da resposta, não como abas que
// disputam espaço com a requisição.
func (a *App) renderApiResponse(width, height int) string {
	focused := a.apiFocus == apiFocusResponse || a.apiSection == apiSecResponse
	innerW, innerH := maxInt(8, width-2), maxInt(1, height-2)

	parts := []string{}
	switch {
	case a.apiLoading:
		parts = append(parts, "enviando…")
	case a.apiResponseErr != "":
		parts = append(parts, "falhou")
	case a.apiResponseStatus != "":
		parts = append(parts,
			a.apiResponseStatus,
			a.apiResponseTime.Round(time.Millisecond).String(),
			apiHumanSize(len(a.apiResponseBody)))
	}
	view := "corpo"
	if a.apiRespView == apiRespHeaders {
		view = "cabeçalhos"
	}
	if a.apiResponseStatus != "" || a.apiResponseErr != "" {
		parts = append(parts, view)
	}
	title := panelTitle("RESPOSTA", parts...)

	var lines []string
	switch {
	case a.apiLoading:
		lines = []string{a.loadingText("esperando o servidor…")}
	case a.apiResponseErr != "":
		lines = wrapAPIErrorLines(a.apiResponseErr, innerW)
	case a.apiResponseStatus == "" && a.apiResponseBody == "":
		lines = []string{
			StyleSection.Render("pronto pra enviar"),
			"",
			StyleMuted.Render("monte a requisição acima e aperte enter"),
			StyleMuted.Render("1-5 troca de aba · m o método · u a URL"),
		}
	default:
		lines = a.renderApiResponseLines(innerW, innerH)
	}
	return panelBox(title, fitExactLines(lines, innerH), width, height, focused)
}

func (a *App) renderApiResponseLines(width, height int) []string {
	body := a.apiResponseBody
	if a.apiRespView == apiRespHeaders {
		body = a.apiResponseHeaders
		if strings.TrimSpace(body) == "" {
			return []string{StyleMuted.Render("a resposta não trouxe cabeçalhos")}
		}
	}
	raw := strings.Split(body, "\n")
	a.apiResponseScroll = clampScroll(a.apiResponseScroll, height, len(raw))
	matchSet := a.apiMatchLineSet()
	current := -1
	if matches := a.apiSearchMatches(); len(matches) > 0 {
		current = matches[a.apiSearchIdx]
	}
	gutter := len(strconv.Itoa(len(raw)))
	out := make([]string, 0, height)
	for i := a.apiResponseScroll; i < len(raw) && len(out) < height; i++ {
		num := StyleMuted.Render(padLeft(strconv.Itoa(i+1), gutter) + " │ ")
		text := sliceColumns(sanitizeTerminalLine(raw[i]), a.apiHScroll, maxInt(4, width-gutter-3))
		switch {
		case i == current:
			out = append(out, num+StyleDiffMatch.Render(text))
		case matchSet[i]:
			out = append(out, num+StyleWarning.Render(text))
		case a.apiRespView == apiRespBody:
			out = append(out, num+renderJSONColumns(sanitizeTerminalLine(raw[i]), a.apiHScroll, maxInt(4, width-gutter-3)))
		default:
			out = append(out, num+StyleNormal.Render(text))
		}
	}
	return out
}

// ── Barra de comandos ─────────────────────────────────────────────────────────

// renderApiCommandBar troca a coluna vertical AÇÕES pela barra larga do padrão
// (§2.3): comandos por extenso, em até duas linhas, e o que aparece depende do
// foco — anunciar `n nova linha` na aba de auth seria mentira.
func (a *App) renderApiCommandBar(width int) string {
	items := [][2]string{}
	if a.apiEditing {
		items = append(items,
			[2]string{"esc", "concluir"},
			[2]string{"ctrl+enter", "enviar"},
			[2]string{"ctrl+a", "tudo"},
			[2]string{"shift+←→", "selecionar"},
		)
		if a.apiFocus == apiFocusSection && a.apiSection == apiSecBody {
			items = append(items, [2]string{"tab", "indentar"})
		}
		if a.apiFocus == apiFocusSection && (a.apiSection == apiSecParams || a.apiSection == apiSecHeaders) {
			items = append(items, [2]string{"tab", "chave/valor"})
		}
		return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
	}

	items = append(items,
		[2]string{"1-5", "abas"},
		[2]string{"tab", "foco"},
		[2]string{"enter", "enviar"},
	)
	switch {
	case a.apiFocus == apiFocusURL:
		items = append(items,
			[2]string{"u", "editar a URL"},
			[2]string{"m", "método"},
			[2]string{"p", "porta do projeto"},
		)
	case a.apiFocus == apiFocusResponse || a.apiSection == apiSecResponse:
		items = append(items,
			[2]string{"↑↓", "rolar"},
			[2]string{"←→", "lateral"},
			[2]string{"h", "corpo/cabeçalhos"},
			[2]string{"/", "buscar"},
		)
		if a.apiSearchQuery != "" {
			items = append(items, [2]string{"n/N", "ocorrência"})
		}
		if a.apiHScroll > 0 {
			items = append(items, [2]string{"0", "voltar ao início"})
		}
	default:
		switch a.apiSection {
		case apiSecParams, apiSecHeaders:
			items = append(items,
				[2]string{"↑↓", "linha"},
				[2]string{"enter", "editar"},
				[2]string{"space", "ligar/desligar"},
				[2]string{"n", "nova"},
				[2]string{"d", "apagar"},
			)
		case apiSecAuth:
			items = append(items, [2]string{"a", "tipo"}, [2]string{"e", "editar"})
		case apiSecBody:
			items = append(items, [2]string{"e", "editar"}, [2]string{"↑↓", "rolar"})
		}
	}
	items = append(items,
		[2]string{"y", "copiar curl"},
		[2]string{",/.", "histórico"},
		[2]string{"esc", "voltar"},
	)
	return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
}

// ── Utilidades ────────────────────────────────────────────────────────────────

func (a *App) apiMethodOrGet() string {
	if m := strings.ToUpper(strings.TrimSpace(a.apiMethod)); m != "" {
		return m
	}
	return "GET"
}

// apiHumanSize é o tamanho da resposta — número que o Postman mostra e que aqui
// faltava: 1,2 kB explica um scroll infinito melhor que qualquer aviso.
func apiHumanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f kB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func apiAuthLabel(t apiAuthType) string {
	switch t {
	case apiAuthBearer:
		return "bearer token"
	case apiAuthBasic:
		return "basic"
	default:
		return "sem autenticação"
	}
}
