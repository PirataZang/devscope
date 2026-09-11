package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// landingRenderers é a lista de TODAS as landings. Um módulo novo que não
// entrar aqui não é coberto pelas invariantes abaixo — inclua junto.
func landingRenderers(a *App) map[Tab]func(*core.Project) string {
	return map[Tab]func(*core.Project) string{
		TabAPI:        a.renderApiLanding,
		TabDatabase:   a.renderDbLanding,
		TabWebSocket:  a.renderWsLanding,
		TabSwarm:      a.renderSwarmLanding,
		TabKubernetes: a.renderK8sLanding,
		TabActions:    a.renderGHALanding,
		TabJenkins:    a.renderJenkinsLanding,
		TabNginx:      a.renderNginxLanding,
		TabRoutes:     a.renderRoutesLanding,
		TabNgrok:      a.renderNgrokLanding,
		TabSSH:        a.renderSSHLanding,
		TabCFTunnel:   a.renderCFLanding,
		TabJSON:       a.renderJsonLanding,
		TabJWT:        a.renderJwtLanding,
	}
}

func landingTestApp(w, h int, tab Tab) (*App, *core.Project) {
	p := &core.Project{
		Name: "app", Path: "/p", Status: core.StatusRunning, Health: core.HealthHealthy,
		Framework: core.FrameworkInfo{Name: "Laravel"}, Ports: []int{8080},
		Git: &core.GitInfo{IsRepo: true, Branch: "main"},
	}
	a := &App{
		width: w, height: h, view: ViewProject, tab: tab, selectedProject: p,
		snapshot: core.Snapshot{Projects: []core.Project{*p}},
	}
	return a, p
}

// A landing não tem caixa. Ela era quatro molduras esticadas até o pé da tela
// para mostrar doze linhas — em 120×40, vinte e quatro das trinta e sete linhas
// eram vazio dentro de borda.
func TestLandingsHaveNoBoxes(t *testing.T) {
	for tab := range landingRenderers(nil) {
		a, p := landingTestApp(120, 40, tab)
		got := stripANSI(landingRenderers(a)[tab](p))
		for _, glyph := range []string{"┌", "└", "┐", "┘"} {
			if strings.Contains(got, glyph) {
				t.Errorf("%v: a landing voltou a usar caixa (%q):\n%s", tab, glyph, got)
			}
		}
	}
}

// Toda landing responde às três perguntas: o que é, dá para usar, como entro.
func TestLandingsAnswerTheThreeQuestions(t *testing.T) {
	for tab := range landingRenderers(nil) {
		a, p := landingTestApp(120, 40, tab)
		got := stripANSI(landingRenderers(a)[tab](p))
		// "como entro" — toda landing tem enter e esc.
		for _, want := range []string{"enter", "esc"} {
			if !strings.Contains(got, want) {
				t.Errorf("%v: a landing não diz %q:\n%s", tab, want, got)
			}
		}
		// "o que é" — a identidade vem logo depois da régua de abertura, com o
		// glifo do módulo, e a linha seguinte diz o que ele faz.
		lines := strings.Split(got, "\n")
		open := landingBandOpen(lines)
		if open < 0 || open+2 >= len(lines) {
			t.Fatalf("%v: faixa não encontrada:\n%s", tab, got)
		}
		if !strings.Contains(lines[open+1], tabGlyph(tab)) {
			t.Errorf("%v: a identidade não traz o glifo: %q", tab, lines[open+1])
		}
		if len(strings.Fields(lines[open+2])) < 4 {
			t.Errorf("%v: não diz o que o módulo faz: %q", tab, lines[open+2])
		}
	}
}

// Nada estoura, e o conteúdo fica no topo — landing que estica até o rodapé é
// caixa vazia com outro nome.
func TestLandingsFitEveryTerminal(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}} {
		for tab := range landingRenderers(nil) {
			a, p := landingTestApp(sz[0], sz[1], tab)
			out := landingRenderers(a)[tab](p)
			lines := strings.Split(out, "\n")
			for i, l := range lines {
				if lipgloss.Width(l) > sz[0] {
					t.Fatalf("%v %dx%d: linha %d mede %d", tab, sz[0], sz[1], i, lipgloss.Width(l))
				}
			}
			// Conteúdo no topo: a última linha não-vazia vem antes da metade.
			last := 0
			for i, l := range lines {
				if strings.TrimSpace(stripANSI(l)) != "" {
					last = i
				}
			}
			if _, h := a.moduleSize(); last > h {
				t.Fatalf("%v %dx%d: conteúdo passa da altura do módulo (%d > %d)", tab, sz[0], sz[1], last, h)
			}
		}
	}
}

// O estado mora no corpo, não repetido no canto do cabeçalho (§1: uma
// informação, um lugar).
func TestLandingStateIsNotDuplicated(t *testing.T) {
	a, p := landingTestApp(140, 40, TabNgrok)
	a.landingNgrokOK, a.landingNgrokAvail = true, false
	got := stripANSI(a.renderNgrokLanding(p))
	if n := strings.Count(got, "não encontrado no PATH"); n != 1 {
		t.Fatalf("o estado aparece %d vezes, devia aparecer 1:\n%s", n, got)
	}
	head := strings.SplitN(got, "\n", 2)[0]
	if strings.Contains(head, "não encontrado") {
		t.Fatalf("o estado não deve ficar no cabeçalho:\n%s", head)
	}
}

// A sondagem que ainda não voltou diz "…", não mente com "offline".
func TestLandingProbePendingIsHonest(t *testing.T) {
	for _, tab := range []Tab{TabNgrok, TabSSH, TabCFTunnel, TabKubernetes, TabSwarm, TabActions, TabJenkins, TabNginx} {
		a, p := landingTestApp(140, 40, tab)
		got := stripANSI(landingRenderers(a)[tab](p))
		if !strings.Contains(got, "sondando") {
			t.Errorf("%v: antes da sondagem a landing devia dizer que está sondando:\n%s", tab, got)
		}
		for _, lie := range []string{"não encontrado", "offline"} {
			if strings.Contains(got, lie) {
				t.Errorf("%v: %q antes de medir é mentira:\n%s", tab, lie, got)
			}
		}
	}
}

// A primeira ação é a razão de a tela existir e vai em destaque; o resto fica
// apagado.
func TestLandingPrimaryActionStandsOut(t *testing.T) {
	a, _ := landingTestApp(120, 40, TabAPI)
	line := a.renderLandingActions([][2]string{{"enter", "abrir"}, {"esc", "voltar"}}, 80)
	// A principal é estilizada diferente das demais — sem depender de qual
	// escape o perfil de cor do terminal emite.
	first := a.renderLandingActions([][2]string{{"enter", "abrir"}}, 80)
	rest := a.renderLandingActions([][2]string{{"esc", "abrir"}}, 80)
	if first == rest {
		t.Fatalf("a ação principal não se distingue das demais: %q vs %q", first, rest)
	}
	plain := stripANSI(line)
	if !strings.Contains(plain, "enter abrir") || !strings.Contains(plain, "esc voltar") {
		t.Fatalf("a linha de ações perdeu conteúdo: %q", plain)
	}
}

// landingBandOpen acha a régua que abre a faixa.
func landingBandOpen(lines []string) int {
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), strings.Repeat("─", 20)) {
			return i
		}
	}
	return -1
}

// A faixa é fechada por DUAS réguas de largura cheia — é o fechamento que faz o
// que vem depois ler como "a página acaba aqui" em vez de "está faltando algo".
func TestLandingBandIsClosed(t *testing.T) {
	for tab := range landingRenderers(nil) {
		a, p := landingTestApp(120, 44, tab)
		lines := strings.Split(stripANSI(landingRenderers(a)[tab](p)), "\n")
		var rules []int
		for i, l := range lines {
			if strings.Count(l, "─") >= 100 {
				rules = append(rules, i)
			}
		}
		if len(rules) != 2 {
			t.Fatalf("%v: a faixa devia ter 2 réguas, tem %d:\n%s", tab, len(rules), strings.Join(lines, "\n"))
		}
		// Uma linha de respiro entre o cabeçalho e a faixa — nem grudada nem
		// boiando no meio da tela.
		if rules[0] != 2 {
			t.Errorf("%v: a faixa abre na linha %d, esperado 2", tab, rules[0])
		}
		if h := rules[1] - rules[0]; h < 4 {
			t.Errorf("%v: faixa com %d linhas — curta demais", tab, h)
		}
		// Nada de conteúdo solto fora da faixa.
		for i, l := range lines {
			if (i < rules[0] || i > rules[1]) && i > 0 && strings.TrimSpace(l) != "" {
				t.Errorf("%v: linha %d fora da faixa: %q", tab, i, l)
			}
		}
	}
}

// Fato que só repete a linha de estado é ruído: sem a ferramenta no PATH,
// "cli não" já foi dito no aviso logo acima.
func TestLandingDropsFactsThatRepeatTheState(t *testing.T) {
	cases := []struct {
		tab   Tab
		prep  func(*App)
		state string
	}{
		{TabNgrok, func(a *App) { a.landingNgrokOK, a.landingNgrokAvail = true, false }, "ngrok"},
		{TabSSH, func(a *App) { a.landingSSHOK, a.landingSSHAvail = true, false }, "ssh"},
		{TabActions, func(a *App) { a.landingGHAOK = true }, "gh"},
		{TabKubernetes, func(a *App) { a.landingK8sOK = true }, "kubectl"},
	}
	for _, c := range cases {
		a, p := landingTestApp(120, 44, c.tab)
		c.prep(a)
		got := stripANSI(landingRenderers(a)[c.tab](p))
		if !strings.Contains(got, "não encontrado no PATH") {
			t.Fatalf("%v: devia avisar que a ferramenta falta:\n%s", c.tab, got)
		}
		if strings.Contains(got, "  não\n") || strings.Contains(got, " não  ") {
			t.Errorf("%v: fato repetindo o estado:\n%s", c.tab, got)
		}
	}
}

// Rótulo curto não abre buraco: a coluna acompanha os rótulos que existem, não
// o teto global.
func TestLandingFactColumnFollowsItsLabels(t *testing.T) {
	// Cabem: deitam numa linha só.
	if got := landingFactRows([][2]string{{"lê", "x"}, {"ok", "y"}}, 60); len(got) != 1 {
		t.Fatalf("fatos curtos deviam caber numa linha, vieram %d: %q", len(got), got)
	}
	// Não cabem: empilham alinhados pelos rótulos que existem.
	rows := landingFactRows([][2]string{{"lê", "x"}, {"contexto", "y"}}, 14)
	if len(rows) != 2 {
		t.Fatalf("sem largura os fatos deviam empilhar, vieram %d: %q", len(rows), rows)
	}
	// Coluna, não byte: "lê" tem 3 bytes e 2 colunas (§8.2).
	col := func(s, needle string) int {
		i := strings.Index(stripANSI(s), needle)
		if i < 0 {
			return -1
		}
		return lipgloss.Width(stripANSI(s)[:i])
	}
	a := col(rows[0], "x")
	b := col(rows[1], "y")
	if a != b || a <= 0 {
		t.Fatalf("os valores começam em colunas diferentes (%d vs %d):\n%q\n%q", a, b, stripANSI(rows[0]), stripANSI(rows[1]))
	}
	// Valor vazio vira "—" (§10).
	if got := stripANSI(strings.Join(landingFactRows([][2]string{{"x", ""}}, 60), "")); !strings.Contains(got, emDash) {
		t.Fatalf("valor ausente devia ser %q: %q", emDash, got)
	}
}

// A abertura mostra o que ESTE projeto tem para o módulo operar. Sem isso
// sobram cinco linhas num painel de quarenta e cinco, e nenhuma delas responde
// à única pergunta antes do enter: "o que eu vou encontrar lá dentro?".
func TestLandingShowsWhatTheProjectHas(t *testing.T) {
	p := &core.Project{
		Name: "app", Path: "/p", Status: core.StatusRunning, Ports: []int{4322, 5432},
		Containers: []core.Container{
			{ID: "w", Name: "app-web", Image: "node:20", State: "running", Ports: "0.0.0.0:4322->4322/tcp"},
			{ID: "d", Name: "app-db", Image: "postgres:16", State: "running", Ports: "0.0.0.0:5432->5432/tcp"},
		},
	}
	cases := []struct {
		tab   Tab
		prep  func(*App)
		wants []string
	}{
		{TabSwarm, func(a *App) {
			a.landingSwarmOK, a.landingSwarmAvail = true, true
			a.landingSwarmCompose = "/p/docker-compose.yml"
		}, []string{"SERVICES DESTE PROJETO", "app-web", "postgres:16", "2 containers"}},
		{TabNgrok, func(a *App) { a.landingNgrokOK, a.landingNgrokAvail = true, true },
			[]string{"O QUE ESTE PROJETO EXPÕE", "localhost:4322", "2 portas"}},
		{TabActions, func(a *App) {
			a.landingGHAOK, a.landingGHAProcs = true, 2
			a.landingGHA.Available, a.landingGHA.Authed = true, true
			a.landingGHANames = []string{"CI", "Deploy"}
		}, []string{"WORKFLOWS DESTE REPOSITÓRIO", "CI", "Deploy", "2 workflows"}},
	}
	for _, c := range cases {
		a := &App{width: 110, height: 44, view: ViewProject, tab: c.tab, selectedProject: p,
			snapshot: core.Snapshot{Projects: []core.Project{*p}}}
		c.prep(a)
		got := stripANSI(landingRenderers(a)[c.tab](p))
		for _, want := range c.wants {
			if !strings.Contains(got, want) {
				t.Errorf("%v: falta %q na abertura:\n%s", c.tab, want, got)
			}
		}
	}
}

// Sem nada para listar, a lista diz o motivo E a saída (§9) — não some nem
// mostra uma seção vazia.
func TestLandingPreviewEmptyStateSpeaks(t *testing.T) {
	p := &core.Project{Name: "app", Path: "/p", Status: core.StatusRunning}
	a := &App{width: 110, height: 44, view: ViewProject, tab: TabNgrok, selectedProject: p,
		snapshot:       core.Snapshot{Projects: []core.Project{*p}},
		landingNgrokOK: true, landingNgrokAvail: true}
	got := stripANSI(a.renderNgrokLanding(p))
	if !strings.Contains(got, "nenhuma porta publicada") {
		t.Fatalf("o estado vazio devia dizer o motivo:\n%s", got)
	}
	if !strings.Contains(got, "suba o projeto") {
		t.Fatalf("o estado vazio devia dizer a saída:\n%s", got)
	}
}

// A lista corta com "+N" em vez de estourar o painel.
func TestLandingPreviewTruncatesWithCount(t *testing.T) {
	names := make([]string, 40)
	for i := range names {
		names[i] = fmt.Sprintf("workflow-%02d", i)
	}
	p := &core.Project{Name: "app", Path: "/p", Status: core.StatusRunning}
	a := &App{width: 110, height: 26, view: ViewProject, tab: TabActions, selectedProject: p,
		snapshot: core.Snapshot{Projects: []core.Project{*p}}, landingGHAOK: true,
		landingGHAProcs: len(names), landingGHANames: names}
	a.landingGHA.Available, a.landingGHA.Authed = true, true
	out := a.renderGHALanding(p)
	if !strings.Contains(stripANSI(out), "+") {
		t.Fatalf("a lista longa devia cortar com +N:\n%s", stripANSI(out))
	}
	if n := len(strings.Split(out, "\n")); n > 26 {
		t.Fatalf("a abertura estourou o painel: %d linhas", n)
	}
}

// "1 containers" é o erro de microcópia mais comum desta tela.
func TestLandingCountsAreSingularWhenOne(t *testing.T) {
	if got := countOf(1, "container", "containers"); got != "1 container" {
		t.Fatalf("got %q", got)
	}
	if got := countOf(3, "porta", "portas"); got != "3 portas" {
		t.Fatalf("got %q", got)
	}
}

// O pulso de "saudável" não pode usar o mesmo glifo de "parado" em quadro
// nenhum — em screenshot parado e em terminal sem cor os dois viravam a mesma
// coisa (§6).
func TestOkPulseNeverLooksStopped(t *testing.T) {
	a := &App{}
	for f := 0; f < 120; f++ {
		a.animFrame = f
		if got := a.okPulse(); got == animStoppedGlyph {
			t.Fatalf("quadro %d: o pulso saudável virou o glifo de parado (%q)", f, got)
		}
	}
}
