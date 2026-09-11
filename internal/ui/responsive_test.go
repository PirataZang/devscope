package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// ─── bancada de responsividade ──────────────────────────────────────────────
//
// Os cinco tamanhos que o DevScope precisa servir. Não são arbitrários:
//
//	80×24    o mínimo do POSIX — e o terminal de split do VS Code
//	100×30   metade de um monitor 1080p
//	120×40   a janela típica em tela cheia
//	160×50   ultrawide / fonte pequena
//	200×60   duas telas ou fonte muito pequena
//
// A regra que rege as cinco: uma tela grande NÃO é a pequena esticada. Cada
// faixa decide O QUE cabe, não só o quanto estica (docs/DESIGN.md §1.1).

var responsiveSizes = [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}}

func responsiveBranches(n int) []core.GitBranch {
	out := []core.GitBranch{{Name: "feature/checkout-v2-pagamento-recorrente", Current: true}}
	for i := 1; i < n; i++ {
		out = append(out, core.GitBranch{
			Name:   fmt.Sprintf("feature/tarefa-%02d-descricao-longa", i),
			Remote: i%5 == 0,
		})
	}
	return out
}

func responsiveCommits(n int) []core.GitCommit {
	msgs := []string{
		"corrige o cálculo de saldo no fechamento do mês",
		"Merge branch 'main' into feature/checkout-v2",
		"sobe o timeout do gateway de pagamento para 30s",
	}
	out := make([]core.GitCommit, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, core.GitCommit{
			Hash: fmt.Sprintf("a1b2c3%02d", i), Author: []string{"Igor", "Ana", "Bruno"}[i%3],
			Message: msgs[i%len(msgs)], Date: fmt.Sprintf("2026-09-%02d 08:12", 1+i%28),
		})
	}
	return out
}

func responsiveProject() core.Project {
	now := time.Now()
	ctr := func(n, img, state, ports string) core.Container {
		return core.Container{ID: n, Name: n, Image: img, State: state, Status: "Up 2 hours",
			Ports: ports, CPU: 12.5, Memory: 400 << 20}
	}
	return core.Project{
		Name: "digiliza-checkout-web", Path: "/home/igor/Área de trabalho/digiliza/digiliza-checkout-web",
		Status: core.StatusDegraded, Health: core.HealthUnhealthy, Uptime: 53 * time.Hour,
		Framework:  core.FrameworkInfo{Name: "Laravel", Version: "11.2"},
		Frameworks: []core.FrameworkInfo{{Name: "Laravel", Version: "11.2"}, {Name: "Vue", Version: "3.4"}},
		Ports:      []int{8080, 3306, 6379, 5173}, HasDockerCompose: true, ContainerCount: 4,
		Containers: []core.Container{
			ctr("digiliza-checkout-web-app-1", "registry.digiliza.com.br/php:8.3-fpm-alpine", "running", "0.0.0.0:8080->80/tcp, 0.0.0.0:8443->443/tcp"),
			ctr("digiliza-checkout-web-nginx-1", "nginx:1.27-alpine", "running", "0.0.0.0:80->80/tcp"),
			ctr("digiliza-checkout-web-mysql-1", "mysql:8.0", "restarting", "0.0.0.0:3306->3306/tcp"),
			ctr("digiliza-checkout-web-redis-1", "redis:7-alpine", "exited", ""),
		},
		Domains: []core.Domain{{Host: "checkout.digiliza.local"}},
		Git: &core.GitInfo{
			IsRepo: true, Branch: "feature/checkout-v2-pagamento-recorrente",
			Remote: "git@github.com:digiliza/digiliza-checkout-web.git",
			Ahead:  3, Behind: 1, Modified: 4, Staged: 2, Untracked: 1, StashCount: 2,
			LastCommit: "a1b2c3d4", LastCommitMsg: "corrige o cálculo de saldo no fechamento do mês",
			LastCommitDate: now.Add(-3 * time.Hour),
			Branches:       responsiveBranches(24),
			Commits:        responsiveCommits(40),
			Files: []core.GitFileStatus{
				{Staging: "M", Worktree: " ", Path: "app/Services/Checkout/RecurringPaymentService.php"},
				{Staging: " ", Worktree: "M", Path: "resources/js/pages/Checkout.vue"},
				{Staging: "?", Worktree: "?", Path: "docs/notas.md"},
			},
			Stashes: []core.GitStash{{Ref: "stash@{0}", Message: "WIP on feature: experimento"}},
			Remotes: []core.GitRemote{{Name: "origin", URL: "git@github.com:digiliza/digiliza-checkout-web.git"}},
		},
	}
}

// responsiveApp monta o app num tamanho, já com dados realistas: caminho longo,
// nome de container longo, branch longa. Conteúdo curto esconde estouro.
func responsiveApp(w, h int) *App {
	p := responsiveProject()
	a := &App{
		width: w, height: h, now: time.Now(), view: ViewDashboard,
		selectedProject: &p, gitSubview: gitSubviewMain, gitFocus: gitFocusBranches,
		gitViewBranch: p.Git.Branch, gitBranches: p.Git.Branches, gitBranchCommits: p.Git.Commits,
		containerSubview:       containerSubviewList,
		containerPreviewID:     p.Containers[0].ID,
		containerPreviewLogs:   "2026/09/10 10:22:01 [notice] 1#1: start worker processes\n172.18.0.1 - - [10/Sep/2026:10:22:14] \"GET /api/health HTTP/1.1\" 200 15",
		containerPreviewStats:  "CPU (%): 12.6%\nMemory: 400MiB / 16GiB (2.4%)\nNet I/O: 1.2MB / 800KB",
		containerCPUHistory:    []float64{4, 8, 12, 10, 14, 9},
		containerMemHistory:    []float64{10, 12, 11, 13, 12, 12},
		containerNetHistory:    []float64{100, 120, 90, 140, 110, 130},
		containerDetailContent: strings.Join(responsiveLogLines(60), "\n"),
		snapshot: core.Snapshot{
			Projects: responsiveProjects(p, 12),
			HostMetrics: core.HostMetrics{
				CPUPercent: 34, MemoryPercent: 61, DiskPercent: 88, OSInfo: "Ubuntu 24.04",
				Uptime: 52 * time.Hour, LoadAvg: "1.42 0.98 0.71", DockerRunning: 11,
				MemoryUsedMB: 9800, MemoryTotalMB: 16000,
			},
			ScannedAt: time.Now().Add(-12 * time.Second),
		},
	}
	return a
}

// responsiveProjects clona o projeto base n vezes: uma lista com dois itens não
// tem como provar que a tela cresce.
func responsiveProjects(base core.Project, n int) []core.Project {
	out := make([]core.Project, 0, n)
	for i := 0; i < n; i++ {
		c := base
		c.Name = fmt.Sprintf("%s-%02d", base.Name, i)
		c.Path = fmt.Sprintf("%s-%02d", base.Path, i)
		switch i % 4 {
		case 1:
			c.Status = core.StatusRunning
		case 2:
			c.Status = core.StatusStopped
		case 3:
			c.Status = core.StatusUnknown
		}
		out = append(out, c)
	}
	return out
}

func responsiveLogLines(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("2026/09/10 10:%02d:01 [notice] worker %d handled request /api/v1/checkout", i%60, i))
	}
	return out
}

// responsiveScenes é toda tela que o app sabe desenhar, cada uma com o estado
// mínimo que a coloca no ar. Tela que não estiver aqui não é auditada.
func responsiveScenes() []struct {
	name string
	prep func(*App)
} {
	type scene = struct {
		name string
		prep func(*App)
	}
	scenes := []scene{
		{"dashboard", nil},
		{"dashboard/filtro", func(a *App) { a.filterOn, a.filterInput = true, "digi" }},
		{"dashboard/vazio", func(a *App) { a.snapshot.Projects = nil }},
		{"dashboard/fuzzy", func(a *App) { a.fuzzyOn, a.fuzzyInput = true, "check" }},
		{"ajuda", func(a *App) { a.helpOn = true }},
		{"tema", func(a *App) { a.themeOn = true }},
		{"preferências", func(a *App) { a.userCfgOn = true }},
		{"relax", func(a *App) { a.view = ViewRelax }},
	}
	for _, tab := range AllTabs {
		t := tab
		scenes = append(scenes, scene{"módulo/" + t.String(), func(a *App) { a.view, a.tab = ViewProject, t }})
	}
	scenes = append(scenes,
		scene{"containers/detalhe", func(a *App) {
			a.view, a.tab, a.containerSubview = ViewProject, TabContainers, containerSubviewDetail
		}},
		scene{"containers/portas", func(a *App) {
			a.view, a.tab, a.containerSubview = ViewProject, TabContainers, containerSubviewPorts
		}},
		scene{"containers/imagens", func(a *App) {
			a.view, a.tab, a.containerSubview = ViewProject, TabContainers, containerSubviewImages
		}},
		scene{"containers/deps", func(a *App) {
			a.view, a.tab, a.containerSubview = ViewProject, TabContainers, containerSubviewDeps
		}},
		scene{"git/gaveta-log", func(a *App) {
			a.view, a.tab = ViewProject, TabGit
			a.gitDrawer, a.gitFocus = gitDrawerLog, gitFocusCmdLog
		}},
		scene{"git/gaveta-stash", func(a *App) {
			a.view, a.tab, a.gitDrawer = ViewProject, TabGit, gitDrawerStash
		}},
		scene{"git/branch", func(a *App) {
			a.view, a.tab, a.gitSubview = ViewProject, TabGit, gitSubviewBranch
		}},
		scene{"git/commit", func(a *App) {
			a.view, a.tab, a.gitSubview = ViewProject, TabGit, gitSubviewCommit
			a.gitSelectedCommit = a.selectedProject.Git.Commits[0]
		}},
		scene{"git/grafo", func(a *App) {
			a.view, a.tab, a.gitSubview = ViewProject, TabGit, gitSubviewGraph
		}},
		scene{"git/filtro-branch", func(a *App) {
			a.view, a.tab = ViewProject, TabGit
			a.gitBranchFilterOn, a.gitBranchFilterInput = true, "feat"
		}},
	)
	return scenes
}

// TestResponsiveNoOverflow é a invariante que não pode cair em tela nenhuma:
// nada mais largo nem mais alto que o terminal. Linha que passa da largura é
// quebrada pelo lipgloss e empurra o rodapé para fora; altura a mais rola o
// topo da UI para fora da tela.
func TestResponsiveNoOverflow(t *testing.T) {
	for _, sc := range responsiveScenes() {
		for _, sz := range responsiveSizes {
			a := responsiveApp(sz[0], sz[1])
			if sc.prep != nil {
				sc.prep(a)
			}
			out := a.View()
			lines := strings.Split(out, "\n")
			if len(lines) != sz[1] {
				t.Errorf("%s @ %dx%d: %d linhas (esperado %d)", sc.name, sz[0], sz[1], len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > sz[0] {
					t.Errorf("%s @ %dx%d: linha %d mede %d colunas: %q",
						sc.name, sz[0], sz[1], i, w, truncate(stripANSI(l), 60))
					break
				}
			}
		}
	}
}

// contentStats mede o que a tela realmente usa: linhas com conteúdo e células
// pintadas. É como se distingue "cresceu" de "esticou".
func contentStats(out string) (rows, cells int) {
	for _, l := range strings.Split(out, "\n") {
		plain := strings.TrimRight(stripANSI(l), " ")
		body := strings.TrimSpace(strings.Trim(plain, "│─┌┐└┘├┤╭╮╰╯▌ "))
		if body != "" {
			rows++
		}
		cells += len([]rune(body))
	}
	return rows, cells
}

// ─── as três classes de tela ────────────────────────────────────────────────
//
// A regra da sessão 7 — "uma tela grande não é a pequena esticada" — não vale
// igual para toda tela, e fingir que vale produz preenchimento inventado, que é
// justamente o que o §1 proíbe. Cada tela cai numa destas classes:
//
//	ROLÁVEL  tem mais conteúdo do que cabe. A altura extra TEM de virar linha:
//	         dashboard, listas de container, branches, commits, logs, diff.
//
//	PORTÃO   diz o estado e a tecla, e acabou. A abertura de módulo é isto: ela
//	         cresce com a lista de preview até onde o projeto tem itens, e para.
//	         Encher o resto seria folheto.
//
//	PROMPT   uma pergunta e um campo. Modal de tema, busca fuzzy, confirmação.
//	         Altura extra vira moldura, e é assim que tem de ser.
//
// Só a primeira classe é cobrada por crescimento.

// boundedScenes são as telas PORTÃO e PROMPT: conteúdo limitado por natureza.
// Estar nesta lista é uma decisão de design, não uma isenção de teste — elas
// continuam cobradas por não estourar e por não mentir.
var boundedScenes = map[string]string{
	"dashboard/vazio": "estado vazio: diz o motivo e a saída, e nada mais",
	"dashboard/fuzzy": "prompt de busca — uma linha de campo",
	"tema":            "modal: lista fixa de temas",
	"preferências":    "modal: formulário de tamanho fixo",
	"git/commit":      "cresce com arquivos e diff; sem commit carregado é o estado vazio",
	"containers/deps": "árvore do compose: tem o tamanho do compose",
	"git/grafo":       "cresce com os commits do grafo",
}

func isBounded(name string) bool {
	if _, ok := boundedScenes[name]; ok {
		return true
	}
	// Toda abertura de módulo é portão (docs/DESIGN.md §1.5).
	return strings.HasPrefix(name, "módulo/")
}

// TestResponsiveScrollableGrowsWithHeight: numa tela ROLÁVEL, mais altura tem
// de virar mais linha de conteúdo. Se não vira, o que cresceu foi o vazio.
func TestResponsiveScrollableGrowsWithHeight(t *testing.T) {
	for _, sc := range responsiveScenes() {
		if isBounded(sc.name) || strings.HasPrefix(sc.name, "relax") {
			continue
		}
		small := responsiveApp(80, 24)
		big := responsiveApp(200, 60)
		if sc.prep != nil {
			sc.prep(small)
			sc.prep(big)
		}
		sRows, sCells := contentStats(small.View())
		bRows, bCells := contentStats(big.View())
		if bCells <= sCells {
			t.Errorf("%s: 200×60 mostra %d células contra %d de 80×24 — esticou, não cresceu",
				sc.name, bCells, sCells)
		}
		if bRows < sRows*14/10 {
			t.Errorf("%s: %d linhas de conteúdo em 60 contra %d em 24 — a altura virou vazio",
				sc.name, bRows, sRows)
		}
	}
}

// TestResponsiveBoundedScenesStayHonest: a tela PORTÃO não é obrigada a crescer,
// mas é obrigada a não inventar. Nada de linha de preenchimento com conteúdo
// fabricado só para tapar o vão.
func TestResponsiveBoundedScenesStayHonest(t *testing.T) {
	for name, why := range boundedScenes {
		if why == "" {
			t.Errorf("%s: toda tela limitada precisa dizer POR QUE é limitada", name)
		}
	}
	// Uma tela limitada mostra o MESMO conteúdo em qualquer altura — o que
	// muda é a moldura em volta, não o que se lê.
	for _, sc := range responsiveScenes() {
		if !isBounded(sc.name) || strings.HasPrefix(sc.name, "relax") {
			continue
		}
		var seen []string
		for _, sz := range responsiveSizes {
			a := responsiveApp(sz[0], sz[1])
			if sc.prep != nil {
				sc.prep(a)
			}
			body := strings.Join(strings.Fields(stripANSI(a.View())), " ")
			seen = append(seen, body)
		}
		// A menor tela não pode ter conteúdo que a maior não tem: cortar por
		// falta de espaço é degradação; sumir em tela grande é bug.
		for _, want := range strings.Fields(seen[0]) {
			if len(want) < 6 || strings.ContainsAny(want, "─│┌┐└┘⣀⣤⣦⣶⣷⣿·…") {
				continue
			}
			if !strings.Contains(seen[len(seen)-1], want) {
				t.Errorf("%s: %q aparece em 80×24 e some em 200×60", sc.name, want)
				break
			}
		}
	}
}

// TestResponsiveVoidStaysAtTheEnd: o vazio pode existir — a tela nem sempre tem
// o que dizer —, mas ele fica NUM lugar só, no fim.
//
// Vazio no meio lê como layout quebrado; o mesmo vazio embaixo lê como página
// que terminou. Foi o defeito da Dashboard em 200×60: o painel do selecionado
// ficava ancorado no rodapé e abria vinte linhas de buraco entre ele e a lista.
//
// Densidade pura não serve de métrica: ela mede quantos projetos o usuário tem,
// não a qualidade do layout. Contiguidade mede o layout.
// A visão de projeto compõe sidebar + conteúdo lado a lado, e a sidebar tem
// nav no topo e rodapé no pé por desenho próprio — o vão entre os dois não é
// buraco do layout do módulo. Essas telas são medidas no painel isolado, pelo
// teste da §1.5, não aqui.
func hasSidebar(name string) bool {
	return strings.HasPrefix(name, "módulo/") || strings.HasPrefix(name, "git/") || strings.HasPrefix(name, "containers/")
}

func TestResponsiveVoidStaysAtTheEnd(t *testing.T) {
	for _, sc := range responsiveScenes() {
		// Prompt e estado vazio não têm o que mostrar; o vão é a moldura deles.
		if strings.HasPrefix(sc.name, "relax") || hasSidebar(sc.name) || isBounded(sc.name) {
			continue
		}
		for _, sz := range responsiveSizes {
			a := responsiveApp(sz[0], sz[1])
			if sc.prep != nil {
				sc.prep(a)
			}
			// Mede os buracos: sequências de linhas vazias SEGUIDAS de conteúdo.
			var holes []int
			run := 0
			for _, l := range strings.Split(a.View(), "\n") {
				if strings.TrimSpace(strings.Trim(stripANSI(l), "│─┌┐└┘├┤╭╮╰╯▌ ")) == "" {
					run++
					continue
				}
				if run > 0 {
					holes = append(holes, run)
				}
				run = 0
			}
			// O ÚLTIMO buraco é legítimo: a barra de comandos é ancorada no pé
			// da tela por design (§2), e o vão antes dela é a página acabando.
			// Qualquer outro buraco grande é layout desenhado para outro tamanho.
			if len(holes) > 0 {
				holes = holes[:len(holes)-1]
			}
			for _, h := range holes {
				if h > 4 && h > sz[1]/5 {
					t.Errorf("%s @ %dx%d: buraco de %d linhas no MEIO da tela",
						sc.name, sz[0], sz[1], h)
					break
				}
			}
		}
	}
}

// TestResponsiveColumnsDegradeInOrder: quando falta largura, as colunas saem
// numa ordem — nunca a crítica antes da terciária.
//
// A ordem é a mesma nas duas tabelas do app, e é a regra da sessão 4:
// COMPARAR antes de DESCREVER. O que se varre com o olho (estado, nome, cpu,
// tempo) sobrevive; o que se lê num item só (imagem, portas) cede primeiro,
// porque já está no detalhe do selecionado logo abaixo.
func TestResponsiveColumnsDegradeInOrder(t *testing.T) {
	// Projetos: glifo, NOME, BRANCH e CAMINHO entram sempre.
	for _, termW := range []int{60, 80, 100, 120, 160, 200} {
		c := tableColumns(safeTableWidth(termW))
		if c.dot == 0 || c.name < 12 || c.branch < 9 || c.path < 14 {
			t.Errorf("projetos @ %d col: coluna crítica caiu: %+v", termW, c)
		}
	}
	// Containers: CPU/MEM/TEMPO antes de IMAGEM/PORTAS.
	var lastImage, lastPorts int
	for _, tw := range []int{52, 60, 73, 90, 117, 160, 200} {
		a := &App{width: 200, containerTableWidth: tw}
		c := a.containerColumns()
		if c.cpu == 0 || c.mem == 0 || c.uptime == 0 || c.name < 12 {
			t.Errorf("containers @ %d col: coluna de comparação caiu: %+v", tw, c)
		}
		// Monotônica: uma coluna que entrou numa largura não some numa maior.
		if lastImage > 0 && c.image == 0 {
			t.Errorf("containers @ %d col: IMAGEM sumiu numa largura MAIOR", tw)
		}
		if lastPorts > 0 && c.ports == 0 {
			t.Errorf("containers @ %d col: PORTAS sumiu numa largura MAIOR", tw)
		}
		lastImage, lastPorts = c.image, c.ports
	}
}

// TestResponsiveKeyHintsDegradeByDropping: a barra de status ENCURTA a lista,
// não reescreve as teclas. Antes, trocar de terminal trocava o nome do atalho:
// "pgup/pgdown rolar" virava "↑↓/pg scroll" e "q sair" virava "? help".
func TestResponsiveKeyHintsDegradeByDropping(t *testing.T) {
	for _, tab := range AllTabs {
		var prev []string
		for _, sz := range [][2]int{{200, 60}, {160, 50}, {120, 40}, {100, 30}, {80, 24}} {
			a := responsiveApp(sz[0], sz[1])
			a.view, a.tab = ViewProject, tab
			words := strings.Fields(stripANSI(a.projectHints(a.projectCompact())))
			if prev != nil {
				// Cada palavra da tela menor tem de existir na maior, na mesma
				// forma: encurtar é descartar do fim, não trocar de vocabulário.
				for _, w := range words {
					found := false
					for _, p := range prev {
						if p == w {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("%v @ %dx%d: %q não existe na barra da tela maior — a dica foi reescrita, não encurtada",
							tab, sz[0], sz[1], w)
						break
					}
				}
			}
			prev = words
		}
	}
}
