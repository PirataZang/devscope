package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// A abertura de um módulo — a primeira tela de treze dos quinze.
//
// Ela era quatro caixas esticadas até o pé da tela para mostrar doze linhas de
// conteúdo: em 120×40, vinte e quatro das trinta e sete linhas eram vazio dentro
// de moldura. E duas dessas caixas ("CAPACIDADES", "ATALHOS NO CLIENTE") eram
// folheto — listavam o que o módulo sabe fazer, que é assunto do DevScope, não
// deste projeto (docs/DESIGN.md §1).
//
// A abertura responde a três perguntas, nesta ordem:
//
//	o que é isto?      o nome e uma linha
//	dá para usar aqui? o estado — a ferramenta existe? o projeto tem isso?
//	como eu entro?     as teclas
//
// Ver docs/DESIGN.md §1.5.

// moduleLanding são os DADOS da abertura. Cada módulo preenche; o desenho é um
// só, aqui — antes cada um montava o próprio layout e os treze divergiram.
type moduleLanding struct {
	// title é o nome do módulo em caixa alta ("NGROK").
	title string
	// tagline: uma linha dizendo o que o módulo faz.
	tagline string
	// state é a linha de estado, já estilizada pelo módulo (é ele que sabe se
	// "offline" é vermelho ou só cinza). Vazio some.
	state string
	// note é um aviso ou erro — some quando vazio.
	note string
	// facts são os pares rótulo→valor do módulo. Valor vazio vira "—".
	facts [][2]string
	// preview é o que ESTE projeto tem para o módulo operar: os containers que
	// virariam services, os workflows que vão rodar, as rotas cadastradas.
	//
	// É o que faz a abertura valer a tela. Sem ele sobram cinco linhas num
	// painel de quarenta e cinco, e nenhuma delas responde à única pergunta que
	// importa antes de apertar enter: "o que eu vou encontrar lá dentro?".
	previewTitle string
	preview      []string
	// previewEmpty diz o motivo E a saída quando não há nada (§9).
	previewEmpty string
	// previewFoot é a consequência numa linha: "3 containers viram services".
	previewFoot string
	// actions são as teclas. A primeira é a principal e ganha destaque.
	actions [][2]string
}

// renderModuleLanding desenha a FAIXA de abertura do módulo.
//
// Duas tentativas anteriores erraram o mesmo alvo. A primeira trocou as quatro
// caixas por cinco parágrafos soltos — tirou a moldura em volta do vazio sem
// tirar o vazio. A segunda prendeu os parágrafos num cartão e o centrou, o que
// só mudou o vazio de lugar: sete linhas de conteúdo boiando numa tela de
// quarenta e quatro continuam parecendo tela quebrada.
//
// O problema é de PROPORÇÃO, não de arranjo. Esta tela tem pouco a dizer e muita
// tela, e o que sobra aqui é LARGURA — noventa colunas para três fatos curtos.
// Então o conteúdo deita em vez de empilhar, e vira uma faixa:
//
//	══════════════════════════════════════════════════════════ ← régua da cor do módulo
//	 ⇪  NGROK                         ○ agente local offline     identidade | estado
//	    expõe o ambiente local — túnel por projeto
//	    versão 3.37.3 · api :4040 · config .devscope/ngrok.json   fatos deitados
//	    ▸ enter abrir console                       esc voltar    ação | saída
//	──────────────────────────────────────────────────────────  ← fecha a faixa
//
// As duas réguas dão FECHAMENTO: o que vem depois lê como "a página acaba aqui",
// não como "está faltando alguma coisa". É a diferença entre uma tela terminada
// e uma tela pela metade.
func (a *App) renderModuleLanding(p *core.Project, l moduleLanding) string {
	w, h := a.moduleSize()
	accent := tabAccentColor(a.tab)

	// A barra de topo não repete o estado: ele é a resposta a "dá para usar isto
	// aqui?", e essa resposta vai na faixa (§1: uma informação, um lugar).
	ctx := a.renderModuleContext(p, w, l.title, "")

	// Uma linha de respiro e a faixa começa. Ela é o conteúdo da tela, não um
	// aviso flutuando: ancorada no topo, o `enter` fica perto do olho e o vão
	// que sobra lê como página que terminou — não como fragmento no ar.
	//
	// O orçamento é a altura inteira: a lista de preview cresce até ocupar o
	// que houver, em vez de mostrar três itens e deixar vinte linhas em branco.
	const top = 1
	band := a.landingBand(l, accent, w, maxInt(6, h-1-top))

	rows := []string{ctx}
	for i := 0; i < top; i++ {
		rows = append(rows, "")
	}
	rows = append(rows, band...)
	for len(rows) < h {
		rows = append(rows, "")
	}
	return strings.Join(rows[:minInt(len(rows), h)], "\n")
}

// landingBand monta a faixa. A régua de cima é da cor do módulo e a de baixo é
// apagada: abre com o dono e fecha em silêncio.
func (a *App) landingBand(l moduleLanding, accent lipgloss.Color, width, budget int) []string {
	const pad = "    "
	inner := maxInt(24, width-len(pad))

	name := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(tabGlyph(a.tab) + "  " + l.title)
	head := name
	if l.state != "" {
		head = joinWithSpacer(name, truncateVisible(l.state, maxInt(10, inner-lipgloss.Width(name)-3)), inner)
	}

	rows := []string{
		ruleColored(width, accent),
		" " + head,
		pad + StyleMuted.Render(truncate(l.tagline, inner-1)),
	}
	if l.note != "" {
		rows = append(rows, pad+truncateVisible(l.note, inner-1))
	}
	if facts := landingFactRows(l.facts, inner-1); len(facts) > 0 {
		rows = append(rows, "")
		for _, f := range facts {
			rows = append(rows, pad+f)
		}
	}
	rows = append(rows, a.landingPreview(l, accent, width, budget-len(rows))...)
	if len(l.actions) > 0 {
		rows = append(rows, "", pad+a.renderLandingActions(l.actions, inner-1))
	}
	return append(rows, rule(width))
}

// landingPreview é a lista do que este projeto tem. Ela usa cabeçalho de seção,
// não caixa — degrau 3 da escada da §1.1, e a razão de a escada existir.
//
// `budget` é o que sobrou da altura do painel: a lista cresce até ocupar o vão
// em vez de deixá-lo vazio, e corta com "+N" quando não cabe.
func (a *App) landingPreview(l moduleLanding, accent lipgloss.Color, width, budget int) []string {
	if l.previewTitle == "" {
		return nil
	}
	const pad = "    "
	inner := maxInt(24, width-len(pad)) - 1

	// 3 = a linha em branco de cima, o cabeçalho, e a linha do rodapé.
	room := budget - 3
	if room < 1 {
		return nil
	}

	body := l.preview
	if len(body) == 0 {
		if l.previewEmpty == "" {
			return nil
		}
		body = []string{StyleMuted.Render(l.previewEmpty)}
	} else if len(body) > room {
		body = append(body[:room-1:room-1],
			StyleMuted.Render(fmt.Sprintf("+%d", len(l.preview)-room+1)))
	}

	rows := []string{"", pad + sectionHeader(l.previewTitle, inner, accent)}
	for _, line := range body {
		rows = append(rows, pad+truncateVisible(line, inner))
	}
	if l.previewFoot != "" && len(rows)+1 <= budget {
		rows = append(rows, pad+StyleMuted.Render(truncate(l.previewFoot, inner)))
	}
	return rows
}

// landingFactRows deita os fatos numa linha só quando eles cabem — três pares
// curtos ("versão 3.37.3", "api :4040") empilhados em três linhas fazem uma
// coluna esquerda esburacada e gastam três linhas para nove palavras.
//
// Quando não cabem, empilha alinhando pela largura dos rótulos que EXISTEM: o
// teto global de factLabelW abria buraco depois de rótulos curtos ("lê", "gh").
func landingFactRows(facts [][2]string, width int) []string {
	if len(facts) == 0 {
		return nil
	}
	inline := make([]string, 0, len(facts))
	for _, f := range facts {
		if f[0] == "" {
			inline = append(inline, landingFactText(f[1]))
			continue
		}
		inline = append(inline, StyleMuted.Render(f[0])+" "+landingFactText(f[1]))
	}
	if line := strings.Join(inline, StyleMuted.Render("   ·   ")); lipgloss.Width(line) <= width {
		return []string{line}
	}

	labelW := 0
	for _, f := range facts {
		if n := lipgloss.Width(f[0]); n > labelW {
			labelW = n
		}
	}
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, truncateVisible(
			StyleMuted.Render(padRight(f[0], labelW))+"   "+landingFactText(f[1]), width))
	}
	return out
}

// landingFactText: ausência é "—" (§10), nunca vazio nem "n/a".
func landingFactText(v string) string {
	if strings.TrimSpace(stripANSI(v)) == "" {
		return StyleMuted.Render(emDash)
	}
	return v
}

// renderLandingActions põe as teclas numa linha. A primeira é a razão de a tela
// existir e leva marcador e negrito; "esc voltar" é saída, não ação, e vai
// encostado na direita, longe das teclas que fazem alguma coisa.
func (a *App) renderLandingActions(actions [][2]string, width int) string {
	if len(actions) == 0 {
		return ""
	}
	accent := tabAccentColor(a.tab)
	first := lipgloss.NewStyle().Foreground(accent).Render("▸ ") +
		StyleKey.Render(actions[0][0]) + " " + StyleNormal.Bold(true).Render(actions[0][1])

	rest := make([]string, 0, len(actions))
	var exit string
	for _, act := range actions[1:] {
		if act[0] == "esc" {
			exit = keyHint(act[0], act[1])
			continue
		}
		rest = append(rest, keyHint(act[0], act[1]))
	}
	left := first
	if len(rest) > 0 {
		left += "     " + strings.Join(rest, StyleMuted.Render("   ·   "))
	}
	if exit == "" {
		return truncateVisible(left, width)
	}
	return joinWithSpacer(truncateVisible(left, maxInt(10, width-lipgloss.Width(exit)-3)), exit, width)
}

// landingProbing é o estado de uma sondagem que ainda não voltou. Todo módulo
// com probe assíncrono passa por ele, e antes cada um escrevia o próprio texto.
func landingProbing() string { return StyleMuted.Render("sondando o ambiente…") }

// landingToolState é a linha de estado de um módulo que depende de um binário no
// PATH. Mesmo vocabulário em ngrok, ssh, cloudflared, kubectl e docker.
func landingToolState(probed, available bool, tool, detail string) string {
	switch {
	case !probed:
		return landingProbing()
	case !available:
		return StyleUnhealthy.Render("⚠ " + tool + " não encontrado no PATH")
	case detail != "":
		return StyleHealthy.Render(detail)
	default:
		return StyleHealthy.Render(tool + " pronto")
	}
}

// probedCount escreve "…" enquanto a sondagem não voltou, em vez de "0" — que
// seria uma medida que ninguém fez.
func probedCount(probed bool, n int) string {
	if !probed {
		return "…"
	}
	return strconv.Itoa(n)
}

// ─── listas prontas para o preview ──────────────────────────────────────────

// landingContainerRows lista os containers do projeto no mesmo vocabulário da
// tela de Containers: faixa Braille do estado, nome, imagem, portas.
//
// É a lista mais reaproveitada das aberturas porque é a resposta certa para a
// maioria delas: são estes containers que virariam services no Swarm, pods no
// Kubernetes, e são as portas deles que o ngrok, o SSH e o Nginx expõem.
func (a *App) landingContainerRows(p *core.Project, width int) []string {
	if p == nil || len(p.Containers) == 0 {
		return nil
	}
	nameW, imgW := 0, 0
	for _, c := range p.Containers {
		nameW = maxInt(nameW, lipgloss.Width(sanitizeTerminalLine(c.Name)))
		imgW = maxInt(imgW, lipgloss.Width(sanitizeTerminalLine(c.Image)))
	}
	nameW = minInt(nameW, maxInt(12, width*32/100))
	imgW = minInt(imgW, maxInt(10, width*28/100))

	rows := make([]string, 0, len(p.Containers))
	for _, c := range p.Containers {
		wave, waveStyle, _, _ := a.containerStateVisual(c)
		ports := emDash
		if pm := collectors.ParseContainerPortMappings(c.Ports); len(pm) > 0 {
			parts := make([]string, 0, len(pm))
			for i, m := range pm {
				if i == 3 {
					parts = append(parts, fmt.Sprintf("+%d", len(pm)-i))
					break
				}
				parts = append(parts, fmt.Sprintf(":%d", m.HostPort))
			}
			ports = strings.Join(parts, " ")
		}
		rows = append(rows, waveStyle.Render(wave)+" "+
			StyleNormal.Render(padRight(truncate(sanitizeTerminalLine(c.Name), nameW), nameW))+"  "+
			StyleMuted.Render(padRight(truncate(sanitizeTerminalLine(c.Image), imgW), imgW))+"  "+
			StyleAccent.Render(ports))
	}
	return rows
}

// landingFileRows lista arquivos achados pela sondagem (workflows, manifests,
// rotas do nginx) com o glifo do próprio módulo — o mesmo que está na sidebar.
func (a *App) landingFileRows(names []string) []string {
	glyph := lipgloss.NewStyle().Foreground(tabAccentColor(a.tab)).Render(tabGlyph(a.tab))
	rows := make([]string, 0, len(names))
	for _, n := range names {
		rows = append(rows, glyph+" "+StyleNormal.Render(sanitizeTerminalLine(n)))
	}
	return rows
}

// landingPortRows lista as portas que o projeto publica — o que um túnel expõe
// e o que uma rota de nginx aponta.
func landingPortRows(p *core.Project) []string {
	if p == nil {
		return nil
	}
	seen := map[int]bool{}
	rows := make([]string, 0, len(p.Ports))
	for _, port := range p.Ports {
		if seen[port] {
			continue
		}
		seen[port] = true
		rows = append(rows, StyleAccent.Render(fmt.Sprintf("localhost:%d", port)))
	}
	for _, d := range p.Domains {
		rows = append(rows, StyleAccent.Render(d.Host)+StyleMuted.Render("   nginx"))
	}
	return rows
}

// countOf evita "1 containers" — o erro de microcópia mais comum da tela.
func countOf(n int, one, many string) string {
	return fmt.Sprintf("%d %s", n, plural(n, one, many))
}

// ─── rodapés: a consequência de apertar enter, numa linha ───────────────────

func swarmDeployFoot(p *core.Project, compose string) string {
	if p == nil || len(p.Containers) == 0 {
		return ""
	}
	if compose == "" {
		return countOf(len(p.Containers), "container", "containers") + " · sem compose de stack neste projeto"
	}
	return countOf(len(p.Containers), "container", "containers") + " · o stack sobe a partir de " + swarmComposeBase(compose)
}

func k8sManifestFoot(n int) string {
	if n == 0 {
		return ""
	}
	return countOf(n, "arquivo", "arquivos") + " · enter abre o console para aplicar"
}

func ghaWorkflowFoot(n int) string {
	if n == 0 {
		return ""
	}
	return countOf(n, "workflow", "workflows") + " · enter mostra os runs de cada um"
}

func nginxRouteFoot(n int) string {
	if n == 0 {
		return ""
	}
	return countOf(n, "entrada", "entradas") + " · enter abre para editar e criar"
}

func tunnelPortFoot(p *core.Project, what string) string {
	if p == nil || (len(p.Ports) == 0 && len(p.Domains) == 0) {
		return ""
	}
	return countOf(len(p.Ports), "porta", "portas") + " · cada uma vira " + what
}
