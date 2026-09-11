package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tela de ajuda no padrão de docs/DESIGN.md. Antes era uma string única de 200
// linhas dentro de uma caixa de 76 colunas: sem cor, sem separação, e com
// atalhos que já não existiam mais. Agora os comandos são dados, agrupados
// pelos mesmos grupos da barra lateral e com as mesmas cores — quem procura o
// comando do Git procura no amarelo, onde o módulo mora.

type helpEntry struct{ keys, desc string }

// helpBlock é uma tela do DevScope. Um grupo tem várias.
type helpBlock struct {
	title   string
	entries []helpEntry
}

type helpGroup struct {
	title  string
	color  lipgloss.Color
	blocks []helpBlock
}

func helpGroups() []helpGroup {
	return []helpGroup{
		{"GERAL", ColorAccent, []helpBlock{
			{"AS TRÊS CAMADAS", []helpEntry{
				{"GLOBAL", "vale em qualquer tela: ? ajuda · q sair · esc voltar"},
				{"CONTEXTUAL", "vale nesta tela: t módulos · / filtrar · r atualizar"},
				{"LOCAL", "vale no módulo aberto: s é stash no Git e parar no ngrok"},
				{"quem vence", "LOCAL antes de CONTEXTUAL antes de GLOBAL"},
			}},
			{"NAVEGAÇÃO", []helpEntry{
				{"ctrl+O", "agente de IA na pasta do projeto"},
				{"↑↓ · k j", "navegar na lista"},
				{"enter", "abrir projeto / ver detalhe"},
				{"esc", "voltar um nível"},
				{"tab · shift+tab", "próximo / anterior módulo"},
				{"t", "mostrar todos os módulos, não só os do projeto"},
				{"pgup · pgdown", "rolar uma página"},
				{"?", "abrir e fechar esta ajuda"},
				{"q", "sair do DevScope"},
			}},
			{"TELA INICIAL", []helpEntry{
				{"/", "filtrar projetos ao vivo"},
				{"ctrl+p", "busca fuzzy de projetos"},
				{"g · c", "abrir direto em Git / Containers"},
				{"shift+E", "terminal no diretório do projeto"},
				{"ctrl+O · shift+O", "agente de IA no projeto"},
				{"shift+C", "preferências (edita aqui dentro)"},
				{"shift+T", "escolher tema"},
				{"ctrl+t", "relax — animações de terminal"},
				{"r", "atualização rápida"},
			}},
			{"APPS EXTERNOS", []helpEntry{
				{"1-9 · 0", "abrir o programa cadastrado na tecla"},
				{"Commands", "o bloco JSON do user_config.txt (10 slots)"},
				{"name", "o texto que aparece na tela inicial"},
				{"command", "vai para o shell; \"&\" no fim abre solto"},
			}},
			{"PREFERÊNCIAS (shift+C)", []helpEntry{
				{"tab · ↑↓", "próximo / anterior campo"},
				{"←→", "trocar o tema (aplica na hora)"},
				{"digitar", "editar nome e comando do atalho, ou a IA"},
				{"enter", "salvar e aplicar sem reiniciar"},
				{"ctrl+r", "voltar tudo ao padrão (pede confirmação)"},
				{"esc", "fechar sem salvar"},
				{"arquivo", "~/.config/devscope/user_config.txt"},
				{"cli", "devscope scan --json · devscope watch"},
			}},
		}},

		{"CÓDIGO", ColorWarning, []helpBlock{
			{"GIT · NAVEGAR", []helpEntry{
				{"←→ · h l", "alternar Branches / Commits / Arquivos"},
				{"s", "gaveta de stashes (abre e fecha)"},
				{"ctrl+l", "gaveta do log de comandos (abre sozinha após pull/push)"},
				{"↑↓ · k j", "mover o cursor na coluna"},
				{"enter", "detalhe da branch/commit ou diff do arquivo"},
				{"b", "filtrar a lista de branches"},
				{"r", "atualizar"},
				{"ctrl+g", "abrir o Git Graph"},
			}},
			{"GIT · BRANCHES", []helpEntry{
				{"space", "checkout da branch selecionada"},
				{"n", "criar branch"},
				{"d", "apagar branch (confirma)"},
				{"shift+R", "renomear branch"},
				{"shift+D", "marcar a branch de origem"},
				{"shift+M", "mesclar a branch na atual (confirma)"},
				{"p", "pull da branch pai"},
				{"shift+P", "push"},
				{"o", "abrir Pull Request no GitHub"},
			}},
			{"GIT · ALTERAÇÕES", []helpEntry{
				{"a", "stage / unstage do arquivo"},
				{"shift+A", "stage / unstage de todos"},
				{"c", "novo commit"},
			}},
			{"GIT GRAPH", []helpEntry{
				{"tab · shift+tab", "commits / detalhe / arquivos"},
				{"↑↓ · j k", "mover o cursor ou rolar o painel"},
				{"←→ · h l", "rolar lateral (shift+H shift+L = 10 colunas)"},
				{"shift+B", "filtrar por branch"},
				{"enter", "detalhe completo do commit"},
				{"r · esc", "recarregar / voltar"},
			}},
			{"GIT · CHERRY-PICK", []helpEntry{
				{"x", "marcar um commit"},
				{"shift+↑↓", "marcar um intervalo"},
				{"shift+C", "copiar os commits marcados"},
				{"shift+V", "colar na branch atual"},
			}},
			{"GITHUB ACTIONS", []helpEntry{
				{"1-3", "processos / runs / workflows"},
				{"enter", "abrir o centro de controle ou o detalhe"},
				{"tab", "lista → resumo → ações"},
				{"t · shift+R", "disparar / re-executar"},
				{"l · o", "logs / abrir no GitHub"},
				{"c · d", "criar / apagar processo"},
				{"shift+L", "login com o gh"},
				{"!", "aviso de setup do GitHub CLI"},
				{"r · esc", "atualizar (auto 8s) / voltar"},
			}},
			{"JENKINS", []helpEntry{
				{"0-3", "overview / pipelines / builds / settings"},
				{"tab", "trocar de painel"},
				{"b · x", "build / parar"},
				{"enter", "logs do build"},
				{"r · esc", "atualizar / voltar"},
			}},
		}},

		{"EXECUÇÃO", ColorDocker, []helpBlock{
			{"CONTAINERS", []helpEntry{
				{"enter", "portas do container"},
				{"m", "detalhe: logs, métricas, env, config"},
				{"n", "novo serviço (Docker Hub ou YAML)"},
				{"s · r", "parar / iniciar e reiniciar"},
				{"p · d", "pausar / remover (confirma)"},
				{"shift+R", "alternar restart=always"},
				{"shift+E", "shell dentro do container"},
				{"shift+U · shift+D", "compose up / compose down"},
				{"shift+A", "incluir containers de outros projetos"},
				{"v", "só os que existem no docker"},
				{"i", "imagens do container"},
			}},
			{"DETALHE DO CONTAINER", []helpEntry{
				{"1-7", "trocar de aba"},
				{"↑↓", "rolar o conteúdo"},
				{"←→ · 0", "rolar lateral / voltar ao início"},
				{"/ · n · shift+N", "buscar / ocorrência seguinte e anterior"},
				{"f · p", "acompanhar ao vivo / pausar (logs)"},
				{"r · esc", "recarregar / voltar"},
			}},
			{"PORTAS E IMAGENS", []helpEntry{
				{"enter", "preview HTTP da porta"},
				{"o · x", "abrir no navegador / fechar a porta"},
				{"shift+A", "escopo das imagens: container→projeto→todas"},
				{"shift+D", "remover imagem (com ou sem force)"},
			}},
			{"SWARM", []helpEntry{
				{"1-8", "services · nodes · tasks · stacks · redes…"},
				{"enter", "centro de controle / detalhe do recurso"},
				{"tab", "tabela → nodes → ações"},
				{"s · u · c", "escalar / atualizar / criar service"},
				{"d · l", "deploy da stack / logs do service"},
				{"t · shift+T", "token de worker / manager"},
				{"i", "swarm init"},
				{"p · m · a", "promover / rebaixar / disponibilidade"},
				{"shift+R · b", "force update / rollback"},
				{"shift+D · shift+P", "remover / podar redes (confirma)"},
				{"r · esc", "atualizar (auto 5s) / voltar"},
			}},
			{"KUBERNETES", []helpEntry{
				{"0-4", "visão geral · workloads · rede · config…"},
				{"←→ · [ ]", "trocar o tipo do recurso"},
				{"n · p", "namespace seguinte / anterior"},
				{"b", "filtrar"},
				{"enter · y", "detalhe / ver o yaml"},
				{"c · e · a", "criar / editar / aplicar"},
				{"ctrl+s", "aplicar o yaml em edição"},
				{"l · d", "logs do pod / excluir (confirma)"},
				{"+ -", "escalar o deployment"},
				{"r · esc", "atualizar / voltar"},
			}},
		}},

		{"REDE", ColorPrimary, []helpBlock{
			{"NGINX", []helpEntry{
				{"1-2", "rotas / arquivo"},
				{"↑↓", "navegar nas rotas"},
				{"enter", "abrir o hub e ver as .inc dele"},
				{"n · d", "nova rota / apagar (confirma)"},
				{"←→ · 0", "rolar o arquivo de lado / início"},
				{"shift+A · shift+R", "confs de todos os projetos / rescan"},
				{"esc", "voltar um nível"},
			}},
			{"ROTAS", []helpEntry{
				{"enter", "detectar a stack e escanear as rotas"},
				{"↑↓", "navegar"},
				{"b", "filtrar por palavra no path"},
				{"enter", "abrir na aba API com método e URL"},
				{"r · esc", "reescanear / voltar"},
			}},
			{"NGROK", []helpEntry{
				{"1-3", "túneis / requests / config"},
				{"tab", "trocar de painel"},
				{"n · e · d", "novo / editar / apagar túnel"},
				{"s · x · r", "subir / parar / reiniciar"},
				{"c · o", "copiar URL / abrir no navegador"},
				{"shift+A", "túneis de todos os projetos"},
				{"esc", "voltar"},
			}},
			{"SSH TUNNEL", []helpEntry{
				{"1-2", "túneis / config"},
				{"tab", "lista → detalhes → logs"},
				{"n · d", "novo / apagar túnel"},
				{"s · x · r", "subir / parar / reiniciar"},
				{"c", "copiar o comando"},
				{"shift+A", "túneis de todos os projetos"},
				{"esc", "voltar"},
			}},
			{"CLOUDFLARE TUNNEL", []helpEntry{
				{"1-3", "túneis / conta / config"},
				{"tab", "lista → detalhes → logs"},
				{"n · d", "novo / apagar túnel"},
				{"s · x", "subir / parar"},
				{"shift+C · shift+R", "criar túnel / rota"},
				{"shift+I · shift+L", "instalar o cloudflared / login"},
				{"shift+K", "matar processos órfãos"},
				{"c · o", "copiar / abrir no navegador"},
				{"shift+A", "túneis de todos os projetos"},
				{"esc", "voltar"},
			}},
		}},

		{"DADOS", ColorPink, []helpBlock{
			{"API", []helpEntry{
				{"tab", "request → URL → headers → auth"},
				{"[ ]", "body / response"},
				{"↑↓", "método no request, senão rola"},
				{"enter", "enviar o request"},
				{"u · a", "porta do projeto / tipo de auth"},
				{"e", "editar o campo em foco"},
				{", · .", "requisição anterior / próxima no histórico"},
				{"/", "buscar no body ou na resposta"},
			}},
			{"DATABASE", []helpEntry{
				{"tab", "tabelas / SQL / resultado"},
				{"enter", "preview com SELECT * LIMIT 50"},
				{"e", "editar o SQL"},
				{"ctrl+enter", "executar"},
				{"[ ]", "trocar o banco detectado"},
				{"←→ · h l", "rolar o resultado de lado"},
				{"r · esc", "recarregar / voltar"},
			}},
			{"JSON", []helpEntry{
				{"p · m", "formatar / minificar"},
				{"v · s", "validar / ordenar chaves"},
				{"w · t · x", "converter para YAML / TOML / XML"},
				{"d", "diff lado a lado"},
				{"n", "remover chaves nulas"},
				{"e · enter", "editar o painel"},
				{"c", "copiar a saída"},
				{"/", "buscar"},
				{"tab · ←→", "entrada ↔ saída"},
			}},
			{"JWT", []helpEntry{
				{"d · v", "decodificar / verificar assinatura"},
				{"g · s", "gerar / assinar"},
				{"c", "claims legíveis (iat/nbf/exp)"},
				{"[ · ]", "algoritmo anterior / próximo"},
				{"y · ctrl+y", "copiar o token"},
				{"shift+Y", "copiar o resultado"},
				{"x", "exportar JSON"},
				{"e · enter", "editar o painel"},
				{"tab · shift+tab", "token ↔ segredo ↔ saída"},
			}},
			{"EDIÇÃO DE TEXTO", []helpEntry{
				{"ctrl+a", "selecionar tudo"},
				{"ctrl+c", "copiar a seleção"},
				{"ctrl+x · ctrl+v", "recortar / colar"},
				{"home · end", "início / fim da linha"},
				{"ctrl+home", "início do texto"},
				{"ctrl+end", "fim do texto"},
				{"ctrl+←→", "palavra anterior / próxima"},
				{"shift+←→", "estender a seleção"},
				{"shift+home · shift+end", "estender até a borda da linha"},
				{"ctrl+shift+←→", "estender por palavra"},
				{"ctrl+shift+home", "estender até o início"},
				{"ctrl+shift+end", "estender até o fim"},
				{"onde", "API · JSON · JWT · Kubernetes · WebSocket"},
			}},
			{"WEBSOCKET", []helpEntry{
				{"enter", "abrir a conversa / trocar de servidor"},
				{"tab", "servidores ↔ conversa"},
				{"c · d · r", "conectar / desligar / reconectar"},
				{"n · e", "novo / editar servidor"},
				{"m", "nova mensagem"},
				{"/ · f", "buscar / filtrar"},
				{"shift+enter", "quebra de linha na mensagem"},
				{"shift+A · esc", "todos os projetos / voltar"},
			}},
		}},
	}
}

func helpGroupCount() int { return len(helpGroups()) }

// helpCommandCount alimenta o cabeçalho: dá a dimensão do que existe sem
// obrigar a rolar as seis abas.
func helpCommandCount() int {
	n := 0
	for _, g := range helpGroups() {
		for _, b := range g.blocks {
			n += len(b.entries)
		}
	}
	return n
}

func (a *App) renderHelpScreen(background string) string {
	groups := helpGroups()
	if a.helpTab < 0 || a.helpTab >= len(groups) {
		a.helpTab = 0
	}
	g := groups[a.helpTab]

	w := maxInt(48, minInt(a.width-4, 150))
	hMax := maxInt(14, minInt(a.height-2, 44))

	header := a.renderHelpHeader(w)
	ruler := a.renderHelpRuler(groups, w)
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"1-" + fmt.Sprint(len(groups)), "grupos"},
		[2]string{"←→", "grupo"},
		[2]string{"↑↓", "rolar"},
		[2]string{"?", "fechar"},
	))
	chrome := lipgloss.Height(header) + lipgloss.Height(ruler) + lipgloss.Height(cmdBar)
	body := a.renderHelpBody(g, w, maxInt(6, hMax-chrome))

	// A ajuda é um overlay: ela tem a altura do que mostra, e o resto da tela
	// fica em volta como moldura. Fixá-la no teto da tela enchia de linha em
	// branco entre o conteúdo e a barra de comandos — em 160×50 eram dezenove.
	h := minInt(hMax, chrome+lipgloss.Height(body))
	stack := lipgloss.JoinVertical(lipgloss.Left, header, ruler, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	box := lipgloss.NewStyle().Background(ColorBgPanel).
		Render(clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h))
	return overlayCentered(background, box, a.width, a.height)
}

func (a *App) renderHelpHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	left := accent.Render("? AJUDA") + StyleMuted.Render("   ") +
		StyleNormal.Bold(true).Render("atalhos do DevScope")
	right := StyleMuted.Render(fmt.Sprintf("%d comandos", helpCommandCount()))
	return joinWithSpacer(truncateVisible(left, width), right, width)
}

// renderHelpRuler: um número por grupo, cada um na cor que o grupo tem na
// barra lateral — quem procura o comando do Git procura no amarelo.
func (a *App) renderHelpRuler(groups []helpGroup, width int) string {
	parts := make([]string, 0, len(groups))
	for i, g := range groups {
		label := fmt.Sprintf(" %d %s ", i+1, g.title)
		if i == a.helpTab {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ColorBg).Background(g.color).Bold(true).Render(label))
			continue
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(g.color).Render(label))
	}
	return padRightVisible(strings.Join(parts, StyleMuted.Render("│")), width)
}

func (a *App) renderHelpBody(g helpGroup, width, height int) string {
	// Duas colunas a partir de 100 colunas: os grupos grandes (SCOPE tem seis
	// telas) cabem sem rolar, que é o ponto de uma tela de ajuda.
	// A caixa come 2 colunas de moldura: as colunas do conteúdo saem do que
	// sobra, senão a régua de cada tela estoura e vira "…".
	inner := maxInt(20, width-2)
	cols := 1
	if inner >= 98 {
		cols = 2
	}
	colW := (inner - (cols-1)*2) / cols
	lines := helpBlockLines(g, colW, cols)

	// A caixa tem a altura do que há para mostrar, com teto na tela. Esticá-la
	// até o rodapé enchia de linha em branco DENTRO da moldura — em 160×50 o
	// grupo GERAL cabia inteiro e sobravam dezenove linhas vazias emolduradas.
	viewport := maxInt(1, minInt(height-2, len(lines)))
	a.helpScroll = clampScroll(a.helpScroll, viewport, len(lines))
	start := a.helpScroll
	end := minInt(start+viewport, len(lines))

	title := g.title
	if len(lines) > viewport {
		title = panelTitle(g.title, fmt.Sprintf("%d-%d/%d", start+1, end, len(lines)))
	}
	box := panelBox(title, fitExactLines(lines[start:end], viewport), width, viewport+2, true)
	return lipgloss.NewStyle().Foreground(g.color).Render(box)
}

// helpBlockLines distribui as telas do grupo em colunas, equilibrando pela
// altura: a coluna da esquerda recebe blocos até passar da metade.
func helpBlockLines(g helpGroup, colW, cols int) []string {
	rendered := make([][]string, len(g.blocks))
	total := 0
	for i, b := range g.blocks {
		rendered[i] = helpOneBlock(b, g.color, colW)
		total += len(rendered[i])
	}
	if cols == 1 {
		var out []string
		for _, block := range rendered {
			out = append(out, block...)
		}
		return out
	}

	var left, right []string
	half := (total + 1) / 2
	for i, block := range rendered {
		if len(left) < half || i == 0 {
			left = append(left, block...)
			continue
		}
		right = append(right, block...)
	}
	for len(left) < len(right) {
		left = append(left, "")
	}
	for len(right) < len(left) {
		right = append(right, "")
	}

	out := make([]string, len(left))
	for i := range left {
		out[i] = padRightVisible(left[i], colW) + "  " + right[i]
	}
	return out
}

func helpOneBlock(b helpBlock, color lipgloss.Color, width int) []string {
	keyW := 0
	for _, e := range b.entries {
		keyW = maxInt(keyW, lipgloss.Width(e.keys))
	}
	keyW = minInt(keyW, maxInt(8, width/2-2))

	// A régua depois do nome é o que separa uma tela da outra: sem ela
	// "CONTAINERS" parecia mais um comando do Git.
	lines := []string{sectionHeader(b.title, width, color)}
	for _, e := range b.entries {
		key := StyleKey.Render(padRight(truncate(e.keys, keyW), keyW))
		lines = append(lines, "  "+key+"  "+StyleMuted.Render(truncate(e.desc, maxInt(6, width-keyW-4))))
	}
	return append(lines, "")
}
