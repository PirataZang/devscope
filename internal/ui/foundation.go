package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Fundação visual do DevScope.
//
// Este arquivo é a resposta a uma pergunta só: "com o que uma tela é
// desenhada?". Antes as peças estavam espalhadas — o painel morava em
// api_tab.go, o corte com consciência de ANSI em git_tab.go, o espaçador em
// dashboard.go — e cada tela nova copiava a de onde tinha olhado por último.
//
// A ordem de escolha, da mais leve para a mais pesada, é:
//
//	1. espaçamento e alinhamento   (padRightVisible, joinWithSpacer)
//	2. cor e peso                  (StyleSection, StyleMuted, StyleKey)
//	3. cabeçalho de seção          (sectionHeader)
//	4. divisor                     (rule, ruleColored)
//	5. caixa                       (panelBox)
//
// Só desça um degrau quando o de cima não separar. Uma caixa custa 2 colunas,
// 2 linhas e uma moldura competindo com o conteúdo; um cabeçalho custa 1 linha
// e agrupa igual. Ver docs/DESIGN.md §§4, 8 e 11.

// ─── vocabulário de glifos ──────────────────────────────────────────────────

// glyphVocabulary lista TODO glifo não-Braille que o DevScope desenha em linha
// de largura calculada. Todos medem 1 coluna nas duas bibliotecas que o app usa
// (go-runewidth e lipgloss) — glifo onde elas discordam desalinha a linha
// inteira num terminal e não no outro. foundation_test.go trava isso.
var glyphVocabulary = []string{
	"⌂", "⑂", "▣", "⎈", "⬡", "≡", "↯", "▤", "{", "⚿", "⇄", "⇅", "⇪", "☁", "⇌",
	"⚙", "▶", "◈", "·", "◆", "▌", "│", "─", "┌", "┐", "└", "┘", "├", "╭", "╰",
	"█", "░", "▸", "◌", "⧗", "⚠", "⊕", "↑", "↓", "←", "→", "…", "—",
}

// glyphBanned são os glifos que JÁ quebraram o alinhamento e não podem voltar:
// medem 2 colunas (ou 1 numa lib e 2 na outra) e são desenhados como 1 pela
// maioria dos terminais. A coluna da direita sai fora de lugar na linha toda.
var glyphBanned = []string{"⚡", "☰", "🔀", "🔒", "📄", "🍒", "🚀", "✅", "❌", "📦", "🔥"}

// ─── largura: cortar e preencher sem quebrar ANSI ───────────────────────────

// padRightVisible ajusta a string para EXATAMENTE `width` colunas visíveis,
// cortando com "…" ou completando com espaço. Conta colunas, não bytes nem
// runas: cortar por byte uma string já estilizada parte o escape ANSI no meio
// e derrama a cor pelo resto da tela.
func padRightVisible(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		s = ansi.Truncate(s, width, "…")
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncateVisible corta no teto de largura mas NÃO preenche — para quando a
// linha continua com outro conteúdo depois.
func truncateVisible(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return padRightVisible(s, width)
}

// joinWithSpacer empurra `right` para a borda direita de `totalWidth`. É como
// se escreve "título … contexto" numa linha só, sem inventar uma coluna.
func joinWithSpacer(left, right string, totalWidth int) string {
	spacer := totalWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if spacer < 1 {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", spacer), right)
}

// stripANSI devolve só o texto visível. Use para MEDIR e COMPARAR conteúdo —
// nunca len() numa string estilizada.
func stripANSI(s string) string {
	var b strings.Builder
	inESC := false
	for _, r := range s {
		if r == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inESC = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ─── altura: encher a caixa até a última linha ──────────────────────────────

// fitExactLines devolve exatamente `height` linhas, cortando o excesso e
// completando com vazias. Sem isso o conteúdo curto deixa a moldura aberta.
func fitExactLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// clampRenderedHeight é o fitExactLines de um bloco já renderizado. É o último
// passo de toda tela cheia: nada pode passar da altura do terminal.
func clampRenderedHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ─── divisores ──────────────────────────────────────────────────────────────

// rule é o divisor padrão: um filete apagado, largura cheia. Separa conteúdo
// de MESMO nível — cabeçalho de tabela e linhas, bloco e bloco. Quando o que
// se quer é separar CONTEXTOS diferentes, aí sim é caixa (panelBox).
func rule(width int) string {
	if width <= 0 {
		return ""
	}
	return StyleMuted.Render(strings.Repeat("─", width))
}

// ruleColored é o filete que pertence a um módulo/grupo: mesma espessura, cor
// do contexto e apagado, para marcar o dono da seção sem virar uma segunda
// borda competindo com o texto.
func ruleColored(width int, color lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(color).Faint(true).Render(strings.Repeat("─", width))
}

// ─── cabeçalho de seção ─────────────────────────────────────────────────────

// sectionHeader agrupa SEM caixa: nome em negrito na cor do contexto e o filete
// ocupando o resto da linha.
//
//	BRANCHES ────────────────────────────────
//
// Custa 1 linha onde a caixa custa 2 linhas + 2 colunas, e agrupa igual. É a
// primeira coisa a tentar quando duas informações precisam ficar separadas.
// Em largura apertada o filete some antes do nome — o nome é o dado.
func sectionHeader(title string, width int, color lipgloss.Color) string {
	title = strings.TrimSpace(stripANSI(title))
	if width <= 0 {
		return ""
	}
	head := lipgloss.NewStyle().Foreground(color).Bold(true).Render(truncate(title, width))
	if gap := width - lipgloss.Width(stripANSI(head)) - 1; gap > 0 {
		head += " " + ruleColored(gap, color)
	}
	return head
}

// panelTitle monta o título de painel no padrão do projeto: NOME seguido dos
// contadores, separados por " · ", pulando o que vier vazio.
//
//	panelTitle("LISTA", "12")               → "LISTA · 12"
//	panelTitle("RUNS", "8/40", "main")      → "RUNS · 8/40 · main"
//
// Substitui os vinte "NOME (%d)" escritos à mão, que divergiram em três
// formatos diferentes de tela para tela. Ver docs/DESIGN.md §4.3.
func panelTitle(name string, parts ...string) string {
	out := strings.TrimSpace(name)
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out += " · " + p
		}
	}
	return out
}

// ─── fatos rotulados ────────────────────────────────────────────────────────

// factLabelW é a coluna do rótulo. Fixa: é ela que alinha as linhas umas com as
// outras e faz o bloco ler como tabela sem precisar de tabela.
const factLabelW = 8

// factLine desenha "  rótulo   fato · fato · fato" — o jeito do DevScope de
// mostrar o detalhe de um item selecionado sem abrir uma segunda tabela nem uma
// caixa por assunto. Nasceu no painel do projeto (dashboard) e é o mesmo do
// container selecionado: quem aprendeu a ler um lê o outro.
//
// Sem fatos vira "—": o bloco tem altura fixa, e linha que some conforme o
// cursor anda faz a tela piscar.
func factLine(label string, facts []string, width int) string {
	if len(facts) == 0 {
		facts = []string{StyleMuted.Render(emDash)}
	}
	// O espaço vai FORA do padRight: rótulo que ocupa a coluna inteira
	// ("recursos", 8) colava no primeiro fato.
	head := "  " + StyleMuted.Render(padRight(label, factLabelW)) + " "
	return truncateVisible(head+strings.Join(facts, StyleMuted.Render(" · ")), width)
}

// ─── dicas de teclado ───────────────────────────────────────────────────────

// keyHint é o par tecla+efeito, com UMA aparência no app inteiro: a tecla puxa
// a atenção, a descrição fica apagada. Antes o rodapé do dashboard usava um
// tom e o trilho dos módulos outro, e a barra inteira parecia um parágrafo.
func keyHint(key, desc string) string {
	if desc == "" {
		return StyleKey.Render(key)
	}
	return StyleKey.Render(key) + " " + StyleMuted.Render(desc)
}

// ─── caixa ──────────────────────────────────────────────────────────────────

// panelBox é a caixa com título embutido na borda de cima. É o ÚLTIMO recurso
// da lista lá de cima: use quando a área é interativa, quando o foco precisa
// ser visível (focused) ou quando o conteúdo é complexo o bastante para ter um
// dentro e um fora. Duas informações serem diferentes não é motivo.
//
// Devolve exatamente width × height, com o conteúdo cortado com consciência de
// ANSI. O título é texto puro de propósito: com escape no meio a conta da
// borda erra e a caixa sai torta.
func panelBox(title string, lines []string, width, height int, focused bool) string {
	if height < 3 {
		height = 3
	}
	innerW := maxInt(4, width-2)
	innerH := maxInt(1, height-2)

	body := make([]string, innerH)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if lipgloss.Width(line) > innerW {
			line = ansi.Truncate(line, innerW, "…")
		}
		body[i] = padRightVisible(line, innerW)
	}

	borderColor := ColorBorder
	if focused {
		borderColor = ColorAccent
	}

	title = stripANSI(title)
	title = truncate(title, maxInt(1, innerW-2))
	titleW := lipgloss.Width(title)
	pad := innerW - 1 - titleW
	if pad < 0 {
		pad = 0
	}
	topPlain := "─" + title + strings.Repeat("─", pad)
	if lipgloss.Width(topPlain) > innerW {
		topPlain = truncate(topPlain, innerW)
	}
	topPlain = padRightVisible(topPlain, innerW)

	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(borderColor)
	if focused {
		titleStyle = titleStyle.Bold(true)
	}
	b.WriteString(titleStyle.Render("┌" + topPlain + "┐"))
	b.WriteByte('\n')
	side := lipgloss.NewStyle().Foreground(borderColor)
	for _, line := range body {
		b.WriteString(side.Render("│"))
		b.WriteString(line)
		b.WriteString(side.Render("│"))
		b.WriteByte('\n')
	}
	b.WriteString(side.Render("└" + strings.Repeat("─", innerW) + "┘"))
	return b.String()
}

// ─── densidade ──────────────────────────────────────────────────────────────
//
// Três faixas, medidas em células do terminal. Uma tela grande NÃO é a pequena
// esticada: cada faixa decide o que cabe, não só o quanto estica.
//
//	faixa      quando                        o que muda
//	─────────  ────────────────────────────  ──────────────────────────────────
//	tiny       altura < 22                   VS Code / split curto: sem espaçador,
//	                                         sidebar de 16 col, sem linha de branch
//	compact    altura < 34 ou largura < 110  medidores voltam para o canto,
//	                                         sidebar de 20 col, dicas encurtadas
//	full       o resto                       faixa de host própria, sidebar de 26
//
// dashboardCompact é a mesma ideia na tela inicial, com o corte na altura em
// que a faixa de medidores deixa de pagar a linha que ocupa.

func (a *App) projectCompact() bool {
	return (a.height > 0 && a.height < 34) || (a.width > 0 && a.width < 110)
}

// projectTiny é o terminal do VS Code / split curto: muito pouca altura.
func (a *App) projectTiny() bool {
	return a.height > 0 && a.height < 22
}

func (a *App) dashboardCompact() bool {
	return a.height > 0 && a.height < 28
}
