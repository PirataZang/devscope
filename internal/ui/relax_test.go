package ui

import (
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

func TestRelaxCatCycleVisitsAllPhases(t *testing.T) {
	st := relaxCatState{}
	seen := map[relaxCatPhase]int{}
	sleepDur, zWhileAwake := 0, 0
	for i := 0; i < 1200; i++ {
		stepRelaxCat(&st)
		seen[st.phase]++
		if st.phase == catSleeping && st.t == 1 && st.dur > 0 {
			sleepDur = st.dur
		}
		if st.phase == catGrooming && st.t > 30 {
			for _, z := range st.zzz {
				if z.glyph != "♪" { // ♪ é o ronrom, esse é pra acontecer acordado
					zWhileAwake++ // Z sobrando muito depois de acordar
				}
			}
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
	for _, p := range []relaxCatPhase{catSleeping, catWaking, catStretching, catYawning, catGrooming, catSettling} {
		if seen[p] == 0 {
			t.Fatalf("fase %d nunca aconteceu no ciclo", p)
		}
	}
	if sleepDur < 250 || sleepDur > 350 {
		t.Fatalf("sono fora de 25–35s: %d frames", sleepDur)
	}
	if zWhileAwake > 0 {
		t.Fatalf("Zzz continuou nascendo com o gato acordado (%d frames)", zWhileAwake)
	}
	// Dormindo tem de ser a maior parte do ciclo.
	if seen[catSleeping] < 600 {
		t.Fatalf("gato dormiu de menos: %d frames de 1200", seen[catSleeping])
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

func TestRelaxFoxStillAndFliesMove(t *testing.T) {
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
			if strings.Contains(status, "rabo") {
				t.Fatalf("rabo ainda anima: %q", status)
			}
		}
	}
	if !moved || !lit {
		t.Fatalf("vaga-lumes parados: moved=%v lit=%v", moved, lit)
	}
}

func TestRelaxInvadersPlaysWaves(t *testing.T) {
	st := relaxInvadersState{}
	for i := 0; i < 4000; i++ {
		stepRelaxInvaders(&st)
		if st.shipX < 0 || st.shipX > float64(st.sw) {
			t.Fatalf("nave fora do campo: %.1f de %d", st.shipX, st.sw)
		}
		if len(st.shots) > 12 || len(st.parts) > 120 || len(st.aliens) > 40 {
			t.Fatalf("entidades acumulando: shots=%d parts=%d aliens=%d", len(st.shots), len(st.parts), len(st.aliens))
		}
		relaxInvadersFrames(&st, 80, 22, 1)
	}
	// Os escudos têm de sofrer: se nada os corrói, a colisão com bomba parou.
	solid := 0
	for _, bk := range st.bunkers {
		for _, r := range bk.rows {
			solid += bitsOn(r)
		}
	}
	full := 0
	for _, r := range relaxInvBunkerArt.rows {
		full += bitsOn(r) * len(st.bunkers)
	}
	if solid >= full {
		t.Fatalf("escudos intactos depois de 4000 passos: %d de %d pixels", solid, full)
	}
	if st.wave < 2 {
		t.Fatalf("nenhuma onda nova em ~6min: wave=%d", st.wave)
	}
	if st.score == 0 {
		t.Fatal("a nave nunca acertou um alien")
	}
}

func bitsOn(v uint32) int {
	n := 0
	for ; v != 0; v &= v - 1 {
		n++
	}
	return n
}

func TestRelaxExitStopsLoopAndFreesState(t *testing.T) {
	a := &App{width: 120, height: 40, view: ViewDashboard}
	a.openRelax()
	a.relaxGame = relaxGameInvaders
	for i := 0; i < 200; i++ {
		a.stepRelax()
	}
	if len(a.relaxInv.aliens) == 0 {
		t.Fatal("cena não chegou a rodar")
	}
	if cmd := a.kickAnim(); cmd == nil {
		t.Fatal("Relax deveria ligar o loop")
	}

	a.closeRelax()
	if a.relaxInv.inited || a.relaxCat.inited || a.relaxSky.inited || a.relaxGalaxy.inited ||
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
