package ui

import (
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestCtrlTTogglesRelax(t *testing.T) {
	a := &App{width: 120, height: 40, view: ViewDashboard}

	if _, _ = a.updateKey(tea.KeyMsg{Type: tea.KeyCtrlT}); a.view != ViewRelax {
		t.Fatalf("ctrl+t deveria abrir Relax, view=%d", a.view)
	}
	if _, _ = a.updateKey(tea.KeyMsg{Type: tea.KeyCtrlT}); a.view != ViewDashboard {
		t.Fatalf("ctrl+t deveria voltar para o dashboard, view=%d", a.view)
	}
}

func TestRelaxEscReturnsToPreviousView(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/tmp/demo"}
	a := &App{width: 120, height: 40, view: ViewProject, selectedProject: &p}
	a.openRelax()
	a.updateRelax(tea.KeyMsg{Type: tea.KeyEsc})
	if a.view != ViewProject {
		t.Fatalf("esc deveria voltar para o projeto, view=%d", a.view)
	}
}

func TestRelaxAnimLoopIsSingle(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	if a.kickAnim() == nil {
		t.Fatal("Relax deveria ligar o loop de animação")
	}
	if a.kickAnim() != nil {
		t.Fatal("kickAnim duplicou o loop de animação")
	}
}

func TestRelaxGameNavigation(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	// A cena troca no meio do crossfade, mas a seleção (e o destaque na lista)
	// responde na hora.
	a.updateRelax(tea.KeyMsg{Type: tea.KeyDown})
	if a.relaxSelectedGame() != relaxGameAsteroid {
		t.Fatalf("esperava Asteroids, got %v", a.relaxSelectedGame())
	}
	relaxSettle(a)
	a.updateRelax(tea.KeyMsg{Type: tea.KeyUp})
	relaxSettle(a)
	if a.relaxGame != relaxGameCube {
		t.Fatalf("esperava Magic Cube, got %v", a.relaxGame)
	}
	a.updateRelax(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	relaxSettle(a)
	if a.relaxGame != relaxGameCat {
		t.Fatalf("esperava Cat, got %v", a.relaxGame)
	}
	// As teclas 1–9 e 0 cobrem as dez primeiras cenas; o resto é ↑↓.
	a.updateRelax(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	relaxSettle(a)
	if a.relaxGame != relaxGames[9] {
		t.Fatalf("tecla 0 deveria ir na décima cena, got %v", a.relaxGame)
	}
}

// relaxSettle roda a transição até o fim, como o loop de render faria.
func relaxSettle(a *App) {
	now := time.Unix(0, 0)
	for i := 0; i < 60 && a.relaxPendingOn; i++ {
		now = now.Add(relaxRenderInterval)
		a.relaxEng.advance(now, func() { a.stepRelax() })
		a.applyRelaxPending()
	}
}

func TestRelaxRenderHasBannerAndGames(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	out := a.renderRelax()
	for _, want := range []string{"ANIMATIONS", "Magic Cube", "Asteroid", "Cat", "Starfield", "MAGIC CUBE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render sem %q:\n%s", want, out)
		}
	}
	if !strings.Contains(stripANSI(out), "RELAX!") {
		t.Fatalf("render sem o título RELAX!:\n%s", out)
	}
}

func TestRelaxGamesRenderEveryFrame(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	for _, g := range relaxGames {
		a.selectRelaxGame(g)
		for i := 0; i < 200; i++ {
			a.animFrame++
			a.stepRelax()
			if out := a.renderRelaxStage(70, 22); strings.TrimSpace(out) == "" {
				t.Fatalf("%v: stage vazio no frame %d", g, i)
			}
		}
	}
}

func TestRelaxAsteroidIdleStaysBoundedAndCalm(t *testing.T) {
	st := relaxAsteroidState{}
	calm, busy := 0, 0
	for i := 0; i < 1500; i++ {
		stepRelaxAsteroid(&st)
		if st.shipX < 0 || st.shipX >= relaxAstW || st.shipY < 0 || st.shipY >= relaxAstH {
			t.Fatalf("nave saiu do campo sem dar wrap: %.1f,%.1f", st.shipX, st.shipY)
		}
		if len(st.rocks) > 30 || len(st.shots) > 8 || len(st.parts) > 120 {
			t.Fatalf("entidades acumulando: rocks=%d shots=%d parts=%d", len(st.rocks), len(st.shots), len(st.parts))
		}
		// Poeira ambiente não conta como ação; tiro e explosão contam.
		if len(st.shots) > 0 || len(st.parts) > 4 {
			busy++
		} else {
			calm++
		}
	}
	if st.destroyed == 0 {
		t.Fatal("a nave nunca acertou um asteroide em 150s")
	}
	// A cena é contemplativa: quase todo frame tem de ser silêncio.
	if calm < busy*3 {
		t.Fatalf("cena agitada demais: calmos=%d agitados=%d", calm, busy)
	}
	if relaxAstLiveRocks(&st) == 0 {
		t.Fatal("cena ficou vazia de asteroides")
	}
}

func TestRelaxCubeScramblesAndSolvesForReal(t *testing.T) {
	st := relaxCubeState{}
	scrambled, messy, solvedAgain := false, false, false
	for i := 0; i < 6000 && !solvedAgain; i++ {
		stepRelaxCube(&st)
		if _, status := relaxCubeFrames(&st, 60, 18, 1); strings.Contains(status, "embaralhando") {
			scrambled = true
		}
		if !relaxCubeSolved(&st) {
			messy = true
		} else if messy {
			// Desfazer o histórico tem de devolver o cubo de verdade, não só
			// trocar o texto do rodapé.
			solvedAgain = true
		}
	}
	if !scrambled || !messy || !solvedAgain {
		t.Fatalf("ciclo do cubo incompleto: embaralhou=%v bagunçou=%v resolveu=%v", scrambled, messy, solvedAgain)
	}
}

func TestRelaxCatSleepsAndFlicksTail(t *testing.T) {
	st := relaxCatState{}
	flicks := 0
	for i := 0; i < 800; i++ {
		was := st.tailFlick
		stepRelaxCat(&st)
		if st.phase != catSleeping {
			t.Fatalf("fase=%d, só dorme", st.phase)
		}
		if was == 0 && st.tailFlick > 0 {
			flicks++
		}
		if len(st.zzz) > 8 {
			t.Fatalf("Zzz acumulando: %d", len(st.zzz))
		}
		const stageW, stageH = 60, 16
		art, status := relaxCatFrames(&st, stageW, stageH, 1)
		if len(art) != stageH || status == "" {
			t.Fatalf("frame inválido: %d linhas", len(art))
		}
		for _, l := range art {
			if lipgloss.Width(l) > stageW {
				t.Fatalf("sprite estourou o palco (%d): %q", lipgloss.Width(l), stripANSI(l))
			}
		}
	}
	if flicks < 8 {
		t.Fatalf("cauda quase não mexeu: %d flicks em 80s", flicks)
	}
}

func TestRelaxFoxTailBendsAndPlays(t *testing.T) {
	st := relaxFoxState{}
	stepRelaxFox(&st)
	const w, h = 90, 26
	// Primeira célula de bicho varrendo de cima: é a ponta da cauda, que é a
	// única parte do sprite que sai do lugar.
	tip := func() int {
		b := newRelaxBrailleVote(w, h)
		relaxFoxScene(&st, b)
		for i := 0; i < w*h; i++ {
			if l := int(b.dom[i]); b.cnt[i] > 0 && l >= relaxFxFur && l < relaxFxFly {
				return i
			}
		}
		return -1
	}
	seen := map[int]bool{}
	plays := 0
	for i := 0; i < 900; i++ {
		was := st.play
		stepRelaxFox(&st)
		if was == 0 && st.play > 0 {
			plays++
		}
		if i%7 == 0 {
			seen[tip()] = true
		}
	}
	if len(seen) < 3 {
		t.Fatalf("a cauda não dobrou: %d posições de ponta", len(seen))
	}
	if plays < 2 {
		t.Fatalf("ela não brincou com o capim: %d tapas em 90s", plays)
	}
}

func TestRelaxCatBreathesOnlyOnTheFlank(t *testing.T) {
	const w, h = 80, 22
	st := relaxCatState{}
	stepRelaxCat(&st)
	top := func(phase float64) []int {
		st.breath = phase
		b := newRelaxBrailleVote(w, h)
		relaxCatDraw(&st, b, w, h)
		out := make([]int, w)
		for x := 0; x < w; x++ {
			out[x] = -1
			for y := 0; y < h; y++ {
				if b.cnt[y*w+x] > 0 && b.over[y*w+x] == "" {
					out[x] = y
					break
				}
			}
		}
		return out
	}
	in, ex := top(math.Pi/2), top(3*math.Pi/2)
	// Cabeça e orelhas ficam onde estão: o gato inteiro subindo lê como falha
	// de render, não como respiração.
	for x := 0; x < w/4; x++ {
		if in[x] != ex[x] {
			t.Fatalf("a cabeça mexeu na coluna %d: %d contra %d", x, in[x], ex[x])
		}
	}
	moved := 0
	for x := w / 2; x < w; x++ {
		if in[x] >= 0 && ex[x] >= 0 && in[x] != ex[x] {
			moved++
		}
	}
	if moved < 6 {
		t.Fatalf("o lombo não respirou: %d colunas mexeram", moved)
	}
}

func TestRelaxCatTailSeamSeparatesWithoutEatingTheBody(t *testing.T) {
	// A curva sai do fim da pata e chega na cauda: nas duas pontas ela existe.
	for _, x := range []float64{46, 66, 87} {
		hit := false
		for y := 55.0; y < 80; y += 0.25 {
			if relaxCatTailSeam(x, y) < relaxCatTailSeamW {
				hit = true
				break
			}
		}
		if !hit {
			t.Fatalf("a curva não passa por x=%.0f", x)
		}
	}
	// E é só uma curva: fora da faixa dela o gato fica inteiro. Cabeça, dorso,
	// peito e anca não podem ter buraco.
	for _, p := range [][2]float64{{40, 62}, {60, 30}, {115, 45}, {25, 74}, {100, 70}} {
		if d := relaxCatTailSeam(p[0], p[1]); d < relaxCatTailSeamW {
			t.Fatalf("a curva comeu (%.0f,%.0f): distância %.2f", p[0], p[1], d)
		}
	}
	// Fina: numa coluna ela apaga um par de pontos, não uma faixa.
	n := 0
	for y := 55.0; y < 80; y += 0.5 {
		if relaxCatTailSeam(66, y) < relaxCatTailSeamW {
			n++
		}
	}
	if n > 6 {
		t.Fatalf("curva grossa demais: %d amostras de meio ponto na coluna", n)
	}
}

func TestRelaxSkyCycleAndFade(t *testing.T) {
	st := relaxSkyState{}
	stepRelaxSky(&st)
	first := len(st.stars)
	cycles, minFade := 0, 1.0
	for i := 0; i < 1200; i++ {
		prevT := st.t
		stepRelaxSky(&st)
		if st.t < prevT {
			cycles++
			if st.dur < 280 || st.dur > 350 {
				t.Fatalf("ciclo fora de 28–35s: %d frames", st.dur)
			}
		}
		if f := st.fade(); f < minFade {
			minFade = f
		}
		if len(st.comets) > 6 {
			t.Fatalf("cometas acumulando: %d", len(st.comets))
		}
		if lines, _ := relaxSkyFrames(&st, 60, 14, 1); len(lines) != 14 {
			t.Fatalf("céu com %d linhas", len(lines))
		}
	}
	if cycles < 2 {
		t.Fatalf("o céu não reiniciou: %d ciclos", cycles)
	}
	if first == 0 || len(st.stars) == 0 {
		t.Fatal("céu sem estrelas")
	}
	// O reset é um crossfade: escurece, mas nunca apaga a tela.
	if minFade > 0.6 || minFade < relaxSkyDimmest-0.01 {
		t.Fatalf("crossfade fora do esperado: min=%.2f", minFade)
	}
}

func TestRelaxEngineFixedStepCatchesUp(t *testing.T) {
	e := relaxEngine{}
	base := time.Unix(0, 0)
	e.advance(base, func() { t.Fatal("primeiro frame não pode simular") })

	steps := 0
	bump := func() { steps++ }
	// Frame na hora certa: um passo.
	e.advance(base.Add(100*time.Millisecond), bump)
	if steps != 1 {
		t.Fatalf("esperava 1 passo, deu %d", steps)
	}
	// Frame de render mais rápido que o passo: desenha sem simular de novo.
	steps = 0
	e.advance(base.Add(133*time.Millisecond), bump)
	if steps != 0 {
		t.Fatalf("tick adiantado não deveria simular, deu %d", steps)
	}
	// Travou 300ms: recupera os passos perdidos em vez de andar em câmera lenta.
	steps = 0
	e.advance(base.Add(433*time.Millisecond), bump)
	if steps != 3 {
		t.Fatalf("esperava recuperar 3 passos, deu %d", steps)
	}
	// Pausa longa (terminal suspenso): não despeja minutos de simulação.
	steps = 0
	e.advance(base.Add(60*time.Second), bump)
	if steps > 1 {
		t.Fatalf("volta de pausa longa simulou demais: %d passos", steps)
	}
	if e.elapsed <= 0 || e.fps <= 0 {
		t.Fatalf("engine sem relógio: elapsed=%.2f fps=%.1f", e.elapsed, e.fps)
	}
}

func TestRelaxSceneSwitchCrossfades(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	a.relaxEng.advance(time.Unix(0, 0), func() {})
	a.selectRelaxGame(relaxGameGalaxy)
	if a.relaxGame == relaxGameGalaxy {
		t.Fatal("a cena trocou na hora, sem transição")
	}
	now := time.Unix(0, 0)
	dark := 1.0
	for i := 0; i < 40 && a.relaxGame != relaxGameGalaxy; i++ {
		now = now.Add(relaxRenderInterval)
		a.relaxEng.advance(now, func() { a.stepRelax() })
		if f := a.relaxEng.fade(); f < dark {
			dark = f
		}
		a.applyRelaxPending()
	}
	if a.relaxGame != relaxGameGalaxy {
		t.Fatal("a transição nunca terminou")
	}
	if dark > 0.2 {
		t.Fatalf("crossfade não apagou a cena (mínimo %.2f)", dark)
	}
	if a.relaxEng.fade() >= 1 {
		t.Fatal("cena nova deveria entrar acendendo, não cheia")
	}
}

func TestRelaxEveryScenePaintsAndFades(t *testing.T) {
	a := &App{width: 120, height: 40}
	a.openRelax()
	for _, g := range relaxGames {
		a.relaxGame = g
		a.resetRelaxScenes()
		for i := 0; i < 120; i++ {
			a.animFrame++
			a.stepRelax()
		}
		art, status := g.scene().frames(a, 70, 16, 1)
		if strings.TrimSpace(stripANSI(strings.Join(art, "\n"))) == "" || status == "" {
			t.Fatalf("%s: cena vazia", g)
		}
		// Apagada de vez: mesmos glifos, sem cor acesa sobrando.
		off, _ := g.scene().frames(a, 70, 16, 0)
		if len(off) == 0 {
			t.Fatalf("%s: fade 0 não devolveu quadro", g)
		}
	}
}

func TestRelaxRainbowFlowsSmoothly(t *testing.T) {
	rgb := func(c lipgloss.Color) (int, int, int) {
		r, g, b, ok := relaxHexRGB(string(c))
		if !ok {
			t.Fatalf("cor inválida: %q", string(c))
		}
		return r, g, b
	}
	// Cada letra pega um ponto diferente do espectro.
	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		seen[string(relaxRainbowColor(i, 0))] = true
	}
	if len(seen) != 6 {
		t.Fatalf("letras repetiram cor no quadro inicial: %v", seen)
	}
	// A onda anda continuamente, inclusive na volta 360°→0°: nenhum passo de
	// frame pode dar um salto grande de cor.
	worst, prevR, prevG, prevB := 0, 0, 0, 0
	for f := 0; f <= 300; f++ {
		secs := float64(f) / 30 // 10s no ritmo de render
		r, g, b := rgb(relaxRainbowColor(0, secs))
		if f > 0 {
			d := absInt(r-prevR) + absInt(g-prevG) + absInt(b-prevB)
			if d > worst {
				worst = d
			}
		}
		prevR, prevG, prevB = r, g, b
	}
	if worst > 24 {
		t.Fatalf("salto de cor entre frames: %d (deveria ser um fluxo)", worst)
	}
}

func TestRelaxTetrisClearsLines(t *testing.T) {
	st := relaxTetrisState{}
	best := 0
	for i := 0; i < 6000; i++ {
		stepRelaxTetris(&st)
		best = maxInt(best, st.lines) // encher o tabuleiro zera o placar
		// Linha completa nunca pode ficar parada no tabuleiro sem estar saindo.
		if len(st.clearing) == 0 && st.wipe == 0 {
			for y := 0; y < st.h; y++ {
				full := true
				for x := 0; x < relaxTetW; x++ {
					if st.grid[y][x] == 0 {
						full = false
						break
					}
				}
				if full {
					t.Fatalf("linha %d completa não foi removida (passo %d)", y, i)
				}
			}
		}
		if lines, _ := relaxTetrisFrames(&st, 40, 16, 1); len(lines) != st.h+2 {
			t.Fatalf("quadro com %d linhas, tabuleiro tem %d+2", len(lines), st.h)
		}
	}
	if best == 0 {
		t.Fatalf("a IA não limpou nenhuma linha em 10min: linhas=%d pontos=%d", st.lines, st.score)
	}
}

func TestRelaxCoffeeFillsAndEmpties(t *testing.T) {
	st := relaxCoffeeState{}
	full, poured, drank := false, false, false
	for i := 0; i < 4000; i++ {
		stepRelaxCoffee(&st)
		if st.phase == coffeePour {
			poured = true
		}
		if st.level > 0.92 {
			full = true
		}
		// Esvaziar só conta depois de ter enchido: senão o estado inicial
		// (xícara vazia) passaria no teste sem nada ter acontecido.
		if full && st.level < 0.05 {
			drank = true
			break
		}
		if st.level < -0.01 || st.level > 1.05 {
			t.Fatalf("nível fora de 0–1: %.2f na fase %d", st.level, st.phase)
		}
		relaxCoffeeFrames(&st, 70, 20, 1)
	}
	if !poured || !full || !drank {
		t.Fatalf("ciclo do café incompleto: serviu=%v encheu=%v bebeu=%v", poured, full, drank)
	}
}

func TestRelaxSeaWavesAndMoonPath(t *testing.T) {
	st := relaxSeaState{}
	lo, hi := 1.0, 0.0
	for i := 0; i < 1400; i++ {
		stepRelaxSea(&st)
		if st.swell < lo {
			lo = st.swell
		}
		if st.swell > hi {
			hi = st.swell
		}
		if i%40 == 0 {
			art, status := relaxSeaFrames(&st, 70, 18, 1)
			if strings.TrimSpace(stripANSI(strings.Join(art, "\n"))) == "" || status == "" {
				t.Fatalf("mar vazio no passo %d", i)
			}
		}
	}
	if hi-lo < 0.3 {
		t.Fatalf("swell não respirou: %.2f..%.2f", lo, hi)
	}
	if len(st.stars) == 0 || st.moonX < 0.6 || len(st.puffs) < 6 {
		t.Fatalf("cena incompleta: moonX=%.2f stars=%d puffs=%d", st.moonX, len(st.stars), len(st.puffs))
	}
}

func TestRelaxFoxSpriteCapsAndShrinks(t *testing.T) {
	big := relaxFoxAt(relaxFxMaxW*3, relaxFxMaxH*3)
	if big.w > relaxFxMaxW || big.h > relaxFxMaxH {
		t.Fatalf("raposa passou do teto: %dx%d", big.w, big.h)
	}
	small := relaxFoxAt(relaxFxMaxW/2, relaxFxMaxH/2)
	if small.w >= big.w || small.h >= big.h {
		t.Fatalf("raposa não encolheu com o palco: %dx%d contra %dx%d", small.w, small.h, big.w, big.h)
	}
	// Proporção mantida nos dois: raposa esticada é outro bicho.
	for _, sp := range []*relaxFoxSprite{big, small} {
		want := float64(relaxFoxDotW) / float64(relaxFoxDotH) * 2
		if got := float64(sp.w) / float64(sp.h); math.Abs(got-want) > 0.15 {
			t.Fatalf("proporção torta em %dx%d: %.2f contra %.2f", sp.w, sp.h, got, want)
		}
	}
}

func TestRelaxFoxBodyStillAndFliesMove(t *testing.T) {
	st := relaxFoxState{}
	stepRelaxFox(&st)
	if len(st.flies) == 0 {
		t.Fatal("cena sem vaga-lumes")
	}
	x0 := st.flies[0].x
	moved, lit := false, false
	for i := 0; i < 400; i++ {
		stepRelaxFox(&st)
		if st.flies[0].x != x0 {
			moved = true
		}
		for _, f := range st.flies {
			if f.glow > 0.5 {
				lit = true
			}
		}
		if i%40 == 0 {
			art, status := relaxFoxFrames(&st, 80, 40, 1)
			plain := stripANSI(strings.Join(art, "\n"))
			if !strings.Contains(plain, "⣿") || status == "" {
				t.Fatalf("raposa vazia no passo %d", i)
			}
		}
	}
	if !moved || !lit {
		t.Fatalf("vaga-lumes parados: moved=%v lit=%v", moved, lit)
	}
}

func TestRelaxExitStopsLoopAndFreesState(t *testing.T) {
	a := &App{width: 120, height: 40, view: ViewDashboard}
	a.openRelax()
	a.relaxGame = relaxGameFox
	for i := 0; i < 200; i++ {
		a.stepRelax()
	}
	if len(a.relaxFox.flies) == 0 {
		t.Fatal("cena não chegou a rodar")
	}
	if cmd := a.kickAnim(); cmd == nil {
		t.Fatal("Relax deveria ligar o loop")
	}

	a.closeRelax()
	if a.relaxFox.inited || a.relaxCat.inited || a.relaxSky.inited || a.relaxGalaxy.inited ||
		a.relaxLeaves.inited || a.relaxTetris.inited {
		t.Fatal("estado das cenas continuou vivo depois de sair")
	}
	if a.relaxEng.elapsed != 0 || !a.relaxEng.last.IsZero() {
		t.Fatal("engine continuou com relógio andando")
	}
	// O tick pendente encontra a view trocada e não reagenda.
	m, cmd := a.Update(animTickMsg{})
	if app := m.(*App); app.animOn {
		t.Fatal("loop de animação continuou ligado fora do Relax")
	} else if cmd != nil {
		t.Fatal("tick fora do Relax reagendou o loop sem necessidade")
	}
}

// A rajada é a cena inteira: o capim anda junto, para, e fica ~3s parado sem
// sair do lugar — folha que não volta ao ponto de origem faz o campo derivar.
func TestRelaxSwordGustsThenRests(t *testing.T) {
	st := relaxSwordState{}
	stepRelaxSword(&st)
	frame := func() string {
		art, _ := relaxSwordFrames(&st, 60, 20, 1)
		return strings.Join(art, "\n")
	}
	for st.phase != relaxSwGustTicks+5 { // bem no meio do silêncio
		stepRelaxSword(&st)
	}
	rest, moved := frame(), false
	for i := 0; i < relaxSwCycle; i++ {
		stepRelaxSword(&st)
		if frame() != rest {
			moved = true
		}
	}
	if !moved {
		t.Fatal("o capim não se mexeu em um ciclo inteiro")
	}
	if frame() != rest {
		t.Fatal("o campo não voltou ao mesmo lugar depois da rajada")
	}

	// O silêncio entre rajadas: ~3s no passo de 100ms da simulação.
	streak, best := 0, 0
	for i := 0; i < relaxSwCycle*2; i++ {
		stepRelaxSword(&st)
		if st.gust != 0 {
			streak = 0
			continue
		}
		if streak++; streak > best {
			best = streak
		}
	}
	if best < relaxSwRestTicks || best > relaxSwRestTicks+2 {
		t.Fatalf("silêncio de %d passos, esperava ~%d", best, relaxSwRestTicks)
	}
}

// A ampulheta não pode vazar: em qualquer fase a estrela tem de estar dentro do
// vidro. O disco de baixo é o caso apertado — ele cresce até quase encostar na
// parede, e um raio a mais o faz atravessar o desenho.
func TestRelaxHourglassKeepsStarsInsideTheGlass(t *testing.T) {
	st := relaxHourglassState{}
	stepRelaxHourglass(&st)
	seen, flipped := [3]bool{}, false
	for i := 0; i < 900; i++ {
		stepRelaxHourglass(&st)
		if st.flipT > 0 {
			flipped = true
		}
		for _, s := range st.stars {
			x, y := relaxHgPos(&st, s)
			if math.Abs(y) > 1.001 {
				t.Fatalf("estrela passou da tampa: y=%.3f fase=%d", y, s.phase)
			}
			if lim := relaxHgHalf(y); math.Abs(x) > lim+0.02 {
				t.Fatalf("estrela atravessou o vidro: |x|=%.3f > %.3f em y=%.3f fase=%d",
					math.Abs(x), lim, y, s.phase)
			}
			seen[s.phase] = true
		}
	}
	for p, ok := range seen {
		if !ok {
			t.Fatalf("fase %d nunca aconteceu em 90s de simulação", p)
		}
	}
	if !flipped {
		t.Fatal("a ampulheta não virou depois de esvaziar")
	}
}

// A dama é sólido de revolução: girar no próprio eixo só muda o desenho por
// causa da canelura e das contas. Se este teste passar a falhar, é sinal de que
// as duas sumiram e a peça virou um poste parado.
func TestRelaxChessSpinIsVisible(t *testing.T) {
	draw := func(spin float64) string {
		st := relaxChessState{inited: true, spin: spin, nod: 0.30}
		art, status := relaxChessFrames(&st, 70, 24, 1)
		if status == "" {
			t.Fatal("quadro sem legenda")
		}
		for _, l := range art {
			if lipgloss.Width(l) > 70 {
				t.Fatalf("a peça estourou o palco: %d colunas", lipgloss.Width(l))
			}
		}
		return stripANSI(strings.Join(art, "\n"))
	}
	// Meia canelura de diferença: o suficiente pra trocar vale por crista.
	a, b := draw(0.4), draw(0.4+math.Pi/relaxChFlutes)
	if a == b {
		t.Fatal("meia canelura de giro não mudou nada no desenho")
	}
	if strings.TrimSpace(a) == "" {
		t.Fatal("a peça não apareceu")
	}
}

// A tabela de probabilidade do Jackpoint tem de valer de verdade: trinca perto
// de 1/10 e J J J em 1/30. Se alguém mexer nos pesos sem refazer a conta, é
// aqui que aparece — e não na sensação de quem está olhando a cena.
func TestRelaxJackpotOddsAreReal(t *testing.T) {
	rng := rand.New(rand.NewSource(20260814))
	const n = 300000
	wantTriple := 0.0
	for _, p := range relaxJpTriple {
		wantTriple += p
	}
	triple, jack := 0, 0
	for i := 0; i < n; i++ {
		r := relaxJpSpin(rng)
		if r[0] != r[1] || r[1] != r[2] {
			continue
		}
		triple++
		if r[0] == jpJack {
			jack++
		}
	}
	if got := float64(triple) / n; math.Abs(got-wantTriple) > 0.006 {
		t.Fatalf("trinca em %.4f, tabela promete %.4f", got, wantTriple)
	}
	if got := float64(jack) / n; math.Abs(got-1.0/30) > 0.004 {
		t.Fatalf("J J J em %.4f, esperado %.4f", got, 1.0/30)
	}
}

// O rolo não pode mentir: ele para exatamente no símbolo que o sorteio deu, os
// três param em momentos diferentes, e a máquina passa por todas as fases sem
// acumular partícula.
func TestRelaxJackpotReelsLandOnTheDraw(t *testing.T) {
	st := relaxJackpotState{}
	stepRelaxJackpot(&st)
	seen, spins := map[int8]bool{}, 0
	for i := 0; i < 4000; i++ {
		was := st.phase
		stepRelaxJackpot(&st)
		seen[st.phase] = true
		if len(st.parts) > 900 {
			t.Fatalf("partículas acumulando: %d", len(st.parts))
		}
		if was != jpPhaseSpin || st.phase != jpPhaseHold {
			continue
		}
		spins++
		stops := map[int]bool{}
		for k := range st.reel {
			pos := int(math.Round(st.reel[k].pos))
			if sym := int8((pos%jpSymbols + jpSymbols) % jpSymbols); sym != st.result[k] {
				t.Fatalf("rolo %d parou em %d, sorteio deu %d", k, sym, st.result[k])
			}
			stops[st.reel[k].dur] = true
		}
		if len(stops) != 3 {
			t.Fatalf("os três rolos pararam juntos: %v", stops)
		}
	}
	if spins < 5 {
		t.Fatalf("só %d sorteios em 400s", spins)
	}
	for _, p := range []int8{jpPhaseSpin, jpPhaseHold, jpPhaseShow, jpPhaseRest} {
		if !seen[p] {
			t.Fatalf("fase %d nunca aconteceu", p)
		}
	}
}

// O V4 tem de ser motor, não animação de motor: o pistão nunca sai do curso que
// a biela permite, os oito se movem, e cada cilindro queima uma vez a cada DUAS
// voltas — quatro tempos. Se alguém trocar a conta da biela-manivela por um
// seno, o curso deixa de bater com os limites e este teste cai.
func TestRelaxV4IsAFourStrokeEngine(t *testing.T) {
	st := relaxV4State{}
	stepRelaxV4(&st)
	lo, hi := relaxV4Rod-relaxV4Crank, relaxV4Rod+relaxV4Crank
	var fires [len(st.cyl)]int
	var minD, maxD [len(st.cyl)]float64
	for i := range minD {
		minD[i], maxD[i] = 9, -9
	}
	start, stages := st.ang, map[int8]bool{}
	for i := 0; i < 1200; i++ {
		before := st.cyl
		stepRelaxV4(&st)
		stages[st.stage] = true
		for k := range st.cyl {
			d, _, _ := relaxV4Piston(st.cyl[k], st.ang)
			if d < lo-1e-9 || d > hi+1e-9 {
				t.Fatalf("pistão %d fora do curso: %.3f não está em [%.3f %.3f]", k, d, lo, hi)
			}
			minD[k], maxD[k] = math.Min(minD[k], d), math.Max(maxD[k], d)
			if st.cyl[k].fire > before[k].fire {
				fires[k]++
			}
		}
	}
	turns := (st.ang - start) / (2 * math.Pi)
	for k := range st.cyl {
		if maxD[k]-minD[k] < 0.9*(hi-lo) {
			t.Fatalf("pistão %d mal se mexeu: curso de %.3f", k, maxD[k]-minD[k])
		}
		// Uma queima a cada duas voltas, com folga pros passos de arranque em
		// que ainda não há ignição.
		if want := turns / 2; float64(fires[k]) < want*0.7 || float64(fires[k]) > want*1.1 {
			t.Fatalf("cilindro %d queimou %d vezes em %.1f voltas, esperado ~%.1f", k, fires[k], turns, want)
		}
	}
	for _, s := range []int8{v4Crank, v4Catch, v4Idle} {
		if !stages[s] {
			t.Fatalf("estágio %d nunca aconteceu", s)
		}
	}
	if len(st.puffs) > 40 {
		t.Fatalf("escape acumulando: %d baforadas", len(st.puffs))
	}
}
