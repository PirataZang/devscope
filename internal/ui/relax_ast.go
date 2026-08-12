package ui

import (
	"math"
	"math/rand"
)

// ── Asteroids Idle ────────────────────────────────────────────────────────────
//
// Não é um jogo: ninguém controla a nave. Uma cena espacial de ~28–35s em que
// uma nave em piloto automático vagueia com inércia, cruza com asteroides,
// ocasionalmente mira e atira, e na maior parte do tempo simplesmente viaja.
// No fim do ciclo a cena escurece e outra começa, com nave/rochas/eventos novos.
//
// Física em float por frame no tick de 10fps do App; render puro num grid de
// células. A célula do terminal é ~2:1, então todo ângulo é convertido pra
// velocidade dividindo o eixo Y — senão tudo parece achatado.

const (
	relaxAstW      = 46
	relaxAstH      = 14
	relaxAstAspect = 2.0
	relaxAstMaxV   = 0.55 // células por frame
	relaxAstFade   = 16   // frames de fade na virada de ciclo
)

type relaxIdlePhase int

const (
	astCruising relaxIdlePhase = iota
	astObserving
	astAiming
	astShooting
)

type relaxRock struct {
	x, y     float64
	vx, vy   float64
	spin     float64
	rot      float64
	size     int // 0 pequeno · 1 médio · 2 grande · 3 gigante (raro)
	shape    int
	backdrop bool // rocha de fundo: passa longe, a nave ignora
}

type relaxShot struct {
	x, y, vx, vy float64
	px, py       float64 // posição anterior, pra trilha curta
	life         int
}

type relaxSpark struct {
	x, y, vx, vy float64
	life, ttl    int
}

type relaxAsteroidState struct {
	inited bool
	tick   int
	dur    int
	phase  relaxIdlePhase

	// nave
	shipX, shipY   float64
	shipVX, shipVY float64
	rot            float64 // radianos, 0 = direita
	heading        float64 // rumo geral do piloto automático
	thrust         int
	nextNudge      int

	target   int // índice da rocha mirada (-1 = nenhuma)
	aimHold  int
	cooldown int

	rocks []relaxRock
	shots []relaxShot
	parts []relaxSpark
	stars []relaxSkyPt // poucos pontos de fundo, só pra dar profundidade

	destroyed int
}

func relaxAstNewScene(st *relaxAsteroidState) {
	st.tick = 0
	st.dur = 280 + rand.Intn(71)
	st.phase = astCruising
	st.target = -1
	st.aimHold = 0
	st.cooldown = 30 + rand.Intn(60)
	st.shots = st.shots[:0]
	st.parts = st.parts[:0]

	st.shipX = 8 + rand.Float64()*float64(relaxAstW-16)
	st.shipY = 3 + rand.Float64()*float64(relaxAstH-6)
	st.heading = rand.Float64() * 2 * math.Pi
	st.rot = st.heading
	st.shipVX = math.Cos(st.heading) * 0.18
	st.shipVY = math.Sin(st.heading) * 0.18 / relaxAstAspect
	st.thrust = 0
	st.nextNudge = 25 + rand.Intn(45)

	st.rocks = st.rocks[:0]
	for i, n := 0, 3+rand.Intn(3); i < n; i++ {
		st.rocks = append(st.rocks, relaxAstRock(1+rand.Intn(2), false))
	}
	if rand.Intn(12) == 0 { // asteroide gigante: evento raro
		st.rocks = append(st.rocks, relaxAstRock(3, false))
	}
	if rand.Intn(8) == 0 { // campo de asteroides ao fundo, só de passagem
		for i, n := 0, 4+rand.Intn(4); i < n; i++ {
			st.rocks = append(st.rocks, relaxAstRock(0, true))
		}
	}

	st.stars = st.stars[:0]
	for i, n := 0, 10+rand.Intn(8); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{
			x: rand.Float64() * float64(relaxAstW),
			y: rand.Float64() * float64(relaxAstH),
		})
	}
}

func relaxAstRock(size int, backdrop bool) relaxRock {
	ang := rand.Float64() * 2 * math.Pi
	speed := 0.34 - 0.07*float64(size) + rand.Float64()*0.1 // grandes são lentos
	if size == 3 {
		speed = 0.08 + rand.Float64()*0.05
	}
	if backdrop {
		speed *= 1.4
	}
	// Nasce numa borda, atravessa a cena.
	x, y := rand.Float64()*float64(relaxAstW), rand.Float64()*float64(relaxAstH)
	if rand.Intn(2) == 0 {
		x = 0
		if math.Cos(ang) < 0 {
			x = float64(relaxAstW - 1)
		}
	} else {
		y = 0
		if math.Sin(ang) < 0 {
			y = float64(relaxAstH - 1)
		}
	}
	return relaxRock{
		x:        x,
		y:        y,
		vx:       math.Cos(ang) * speed,
		vy:       math.Sin(ang) * speed / relaxAstAspect,
		spin:     (rand.Float64() - 0.5) * 0.06, // rotação bem lenta
		rot:      rand.Float64() * 2 * math.Pi,
		size:     size,
		shape:    rand.Intn(3),
		backdrop: backdrop,
	}
}

func relaxAstWrap(v, span float64) float64 {
	for v < 0 {
		v += span
	}
	for v >= span {
		v -= span
	}
	return v
}

// angDiff normaliza pra [-π, π] — sem isso a nave gira o caminho longo.
func angDiff(a, b float64) float64 {
	d := math.Mod(a-b+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d - math.Pi
}

// visual: distância/ângulo com o eixo Y corrigido pelo aspecto da célula.
func relaxAstAngleTo(fromX, fromY, toX, toY float64) (float64, float64) {
	dx, dy := toX-fromX, (toY-fromY)*relaxAstAspect
	return math.Atan2(dy, dx), math.Hypot(dx, dy)
}

func stepRelaxAsteroid(st *relaxAsteroidState) {
	if !st.inited {
		st.inited = true
		relaxAstNewScene(st)
	}
	st.tick++

	relaxAstPilot(st)

	// Nave: inércia + wrap-around.
	st.shipX = relaxAstWrap(st.shipX+st.shipVX, relaxAstW)
	st.shipY = relaxAstWrap(st.shipY+st.shipVY, relaxAstH)

	for i := range st.rocks {
		r := &st.rocks[i]
		r.x = relaxAstWrap(r.x+r.vx, relaxAstW)
		r.y = relaxAstWrap(r.y+r.vy, relaxAstH)
		r.rot += r.spin
		if !r.backdrop && r.size >= 2 && rand.Intn(70) == 0 { // poeira ocasional
			st.parts = append(st.parts, relaxSpark{x: r.x, y: r.y, ttl: 6 + rand.Intn(6)})
		}
	}

	shots := st.shots[:0]
	for _, s := range st.shots {
		s.px, s.py = s.x, s.y
		s.x = relaxAstWrap(s.x+s.vx, relaxAstW)
		s.y = relaxAstWrap(s.y+s.vy, relaxAstH)
		if s.life--; s.life > 0 {
			shots = append(shots, s)
		}
	}
	st.shots = shots
	relaxAstCollide(st)

	parts := st.parts[:0]
	for _, p := range st.parts {
		p.x += p.vx
		p.y += p.vy
		p.vx *= 0.86 // some expandindo e desacelerando, sem estouro
		p.vy *= 0.86
		if p.life++; p.life < p.ttl {
			parts = append(parts, p)
		}
	}
	st.parts = parts

	// Repõe rochas devagar pra cena nunca ficar vazia nem cheia demais.
	if live := relaxAstLiveRocks(st); live < 3 && st.tick%25 == 0 {
		st.rocks = append(st.rocks, relaxAstRock(1+rand.Intn(2), false))
	}

	if st.tick >= st.dur {
		relaxAstNewScene(st)
	}
}

func relaxAstLiveRocks(st *relaxAsteroidState) int {
	n := 0
	for _, r := range st.rocks {
		if !r.backdrop {
			n++
		}
	}
	return n
}

// relaxAstPilot é o "piloto automático": cruza a maior parte do tempo, às vezes
// repara numa rocha, gira devagar até ela, espera uma janela e atira. Ignorar
// asteroides é o comportamento normal — a cena só é calma porque quase nunca
// acontece alguma coisa.
func relaxAstPilot(st *relaxAsteroidState) {
	if st.cooldown > 0 {
		st.cooldown--
	}
	if st.thrust > 0 {
		st.thrust--
	}

	// Desvio cinematográfico: rocha muito perto, correção suave de rumo.
	if i, d := relaxAstNearest(st, 4.5); i >= 0 && d < 3.2 {
		ang, _ := relaxAstAngleTo(st.shipX, st.shipY, st.rocks[i].x, st.rocks[i].y)
		st.heading = ang + math.Pi + (rand.Float64()-0.5)*0.4
		st.thrust = maxInt(st.thrust, 8)
		st.phase = astCruising
		st.target = -1
	}

	switch st.phase {
	case astCruising:
		if st.nextNudge--; st.nextNudge <= 0 {
			st.heading += (rand.Float64() - 0.5) * 0.9 // pequena curva
			st.nextNudge = 30 + rand.Intn(60)
			if rand.Intn(3) > 0 {
				st.thrust = 8 + rand.Intn(14)
			}
		}
		if st.cooldown == 0 {
			if i, _ := relaxAstNearest(st, 16); i >= 0 && rand.Intn(60) == 0 {
				st.target = i
				st.phase = astObserving
				st.aimHold = 4 + rand.Intn(8) // repara nela antes de girar
			}
		}

	case astObserving:
		if !relaxAstTargetOK(st) {
			st.phase, st.target = astCruising, -1
			break
		}
		if st.aimHold--; st.aimHold <= 0 {
			st.phase = astAiming
			st.aimHold = 0
		}

	case astAiming:
		if !relaxAstTargetOK(st) {
			st.phase, st.target = astCruising, -1
			break
		}
		r := st.rocks[st.target]
		_, dist := relaxAstAngleTo(st.shipX, st.shipY, r.x, r.y)
		lead := dist / 1.6 // antecipa o movimento da rocha
		want, _ := relaxAstAngleTo(st.shipX, st.shipY, r.x+r.vx*lead, r.y+r.vy*lead)
		d := angDiff(want, st.rot)
		st.rot += math.Max(-0.09, math.Min(0.09, d)) // gira devagar, com limite
		if math.Abs(d) < 0.12 {
			if st.aimHold++; st.aimHold > 3+rand.Intn(5) { // janela antes do tiro
				st.phase = astShooting
			}
		}

	case astShooting:
		st.shots = append(st.shots, relaxShot{
			x: st.shipX, y: st.shipY,
			vx:   math.Cos(st.rot) * 1.6,
			vy:   math.Sin(st.rot) * 1.6 / relaxAstAspect,
			life: 22 + rand.Intn(10),
		})
		st.phase, st.target = astCruising, -1
		st.aimHold = 0
		st.cooldown = 45 + rand.Intn(90) // longos trechos sem tiro nenhum
		st.heading = st.rot
	}

	// Rotação para o rumo quando não está mirando + inércia.
	if st.phase == astCruising || st.phase == astObserving {
		st.rot += math.Max(-0.05, math.Min(0.05, angDiff(st.heading, st.rot)))
	}
	if st.thrust > 0 {
		st.shipVX += math.Cos(st.rot) * 0.035
		st.shipVY += math.Sin(st.rot) * 0.035 / relaxAstAspect
	}
	st.shipVX *= 0.985 // desaceleração muito suave
	st.shipVY *= 0.985
	if sp := math.Hypot(st.shipVX, st.shipVY*relaxAstAspect); sp > relaxAstMaxV {
		k := relaxAstMaxV / sp
		st.shipVX *= k
		st.shipVY *= k
	}
}

func relaxAstTargetOK(st *relaxAsteroidState) bool {
	return st.target >= 0 && st.target < len(st.rocks) && !st.rocks[st.target].backdrop
}

// relaxAstNearest devolve a rocha "detectada" mais próxima dentro do raio.
func relaxAstNearest(st *relaxAsteroidState, radius float64) (int, float64) {
	best, bestD := -1, radius
	for i, r := range st.rocks {
		if r.backdrop {
			continue
		}
		if _, d := relaxAstAngleTo(st.shipX, st.shipY, r.x, r.y); d < bestD {
			best, bestD = i, d
		}
	}
	return best, bestD
}

func relaxAstCollide(st *relaxAsteroidState) {
	for si := len(st.shots) - 1; si >= 0; si-- {
		s := st.shots[si]
		for ri := len(st.rocks) - 1; ri >= 0; ri-- {
			r := st.rocks[ri]
			if r.backdrop {
				continue
			}
			_, d := relaxAstAngleTo(s.x, s.y, r.x, r.y)
			if d > 1.1+float64(r.size)*0.9 {
				continue
			}
			relaxAstBreak(st, ri)
			st.shots = append(st.shots[:si], st.shots[si+1:]...)
			break
		}
	}
}

// relaxAstBreak: grande vira dois médios em direções diferentes, médio vira
// dois pequenos, pequeno vira só partículas.
func relaxAstBreak(st *relaxAsteroidState, idx int) {
	r := st.rocks[idx]
	st.rocks = append(st.rocks[:idx], st.rocks[idx+1:]...)
	st.destroyed++
	if st.target == idx {
		st.target = -1
	}

	n := 5 + r.size*3
	for i := 0; i < n; i++ {
		a := rand.Float64() * 2 * math.Pi
		sp := 0.15 + rand.Float64()*0.35
		st.parts = append(st.parts, relaxSpark{
			x: r.x, y: r.y,
			vx:  math.Cos(a) * sp,
			vy:  math.Sin(a) * sp / relaxAstAspect,
			ttl: 5 + rand.Intn(7),
		})
	}
	if r.size == 0 {
		return
	}
	for i := 0; i < 2; i++ {
		a := math.Atan2(r.vy*relaxAstAspect, r.vx) + (float64(i)*2-1)*(0.5+rand.Float64()*0.5)
		sp := 0.28 + rand.Float64()*0.18
		st.rocks = append(st.rocks, relaxRock{
			x: r.x, y: r.y,
			vx:    math.Cos(a) * sp,
			vy:    math.Sin(a) * sp / relaxAstAspect,
			spin:  (rand.Float64() - 0.5) * 0.08,
			rot:   rand.Float64() * 2 * math.Pi,
			size:  minInt(r.size-1, 1),
			shape: rand.Intn(3),
		})
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

// Nave em 8 direções — o giro lento passa por todas.
func (st *relaxAsteroidState) fade() float64 {
	if st.tick < relaxAstFade {
		return easeInOut(float64(st.tick) / relaxAstFade)
	}
	if left := st.dur - st.tick; left < relaxAstFade {
		return easeInOut(float64(left) / relaxAstFade)
	}
	return 1
}

// ── Render ────────────────────────────────────────────────────────────────────
//
// Vetorial em Braille, como no fliperama: as rochas são polígonos irregulares
// girando (contorno, não bloco) e a nave é um triângulo. A simulação continua
// em células; o render converte pra subpixel, que é onde as linhas cabem.

const (
	relaxAstLvlStar = iota
	relaxAstLvlBack
	relaxAstLvlRock
	relaxAstLvlShip
	relaxAstLvlFlame
	relaxAstLvlShot
	relaxAstLvlSpark
)

var relaxAstRamp = []relaxColor{
	"#38415A", "#394150", "#828C9E", "#EAEFF7", "#F0A63C", "#6FD3E8", "#F2C14A",
}

// relaxAstPoly devolve o contorno de uma rocha: raios sorteados de forma
// determinística pelo shape, então cada pedra tem silhueta própria e estável.
func relaxAstPoly(shape, size int, rot, cx, cy, rad float64, out [][2]float64) [][2]float64 {
	n := 9 + size
	out = out[:0]
	for i := 0; i < n; i++ {
		a := rot + float64(i)*2*math.Pi/float64(n)
		k := 0.70 + 0.30*float64(relaxHash(shape, i)%1000)/1000
		out = append(out, [2]float64{cx + math.Cos(a)*rad*k, cy + math.Sin(a)*rad*k})
	}
	return out
}

var relaxAstRadius = [4]float64{0.85, 1.5, 2.3, 3.5} // em unidades de simulação

func relaxAsteroidFrames(st *relaxAsteroidState, width, height int, gfade float64) ([]string, string) {
	if !st.inited {
		stepRelaxAsteroid(st)
	}
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBrailleVote(w, h)
	sw, sh := float64(w*2), float64(h*4)

	// Uma unidade de simulação em subpixels. O eixo Y já vem compensado pela
	// física (relaxAstAspect), então usar a escala de X nos dois lados é o que
	// mantém as rochas redondas.
	ux := sw / relaxAstW
	uy := sh / relaxAstH
	px := func(x float64) float64 { return x * ux }
	py := func(y float64) float64 { return y * uy }

	fade := st.fade() * gfade
	if fade < 0.12 {
		return newRelaxBraille(w, h).lines(nil), StyleMuted.Render(st.phase.status(st))
	}

	for _, p := range st.stars {
		b.set(int(px(p.x)), int(py(p.y)), relaxAstLvlStar)
	}

	poly := make([][2]float64, 0, 13)
	for _, r := range st.rocks {
		lvl := relaxAstLvlRock
		if r.backdrop {
			lvl = relaxAstLvlBack
		}
		poly = relaxAstPoly(r.shape, r.size, r.rot, px(r.x), py(r.y), relaxAstRadius[r.size]*ux, poly)
		for i := range poly {
			q := poly[(i+1)%len(poly)]
			b.line(poly[i][0], poly[i][1], q[0], q[1], lvl)
		}
	}

	for _, s := range st.shots {
		b.line(px(s.px), py(s.py), px(s.x), py(s.y), relaxAstLvlShot)
	}
	for _, p := range st.parts {
		b.set(int(px(p.x)), int(py(p.y)), relaxAstLvlSpark)
	}

	// Chama: pisca e encolhe, saindo pela traseira.
	if st.thrust > 0 {
		back := st.rot + math.Pi
		fl := (0.9 + 0.5*float64(st.tick%3)) * ux
		b.line(px(st.shipX)+math.Cos(back)*ux*0.7, py(st.shipY)+math.Sin(back)*ux*0.7,
			px(st.shipX)+math.Cos(back)*fl, py(st.shipY)+math.Sin(back)*fl, relaxAstLvlFlame)
	}
	// Nave: triângulo com a proa no rumo.
	nose := 2.1 * ux
	tail := 1.4 * ux
	var tri [3][2]float64
	for i, off := range [3]float64{0, 2.5, -2.5} {
		d, l := st.rot+off, nose
		if off != 0 {
			l = tail
		}
		tri[i] = [2]float64{px(st.shipX) + math.Cos(d)*l, py(st.shipY) + math.Sin(d)*l}
	}
	for i := 0; i < 3; i++ {
		q := tri[(i+1)%3]
		b.line(tri[i][0], tri[i][1], q[0], q[1], relaxAstLvlShip)
	}

	return b.lines(relaxStyles(relaxAstRamp, fade)), StyleMuted.Render(st.phase.status(st))
}

func (p relaxIdlePhase) status(st *relaxAsteroidState) string {
	if len(st.parts) > 6 {
		return "destroços se dispersando"
	}
	switch p {
	case astObserving:
		return "algo se aproxima…"
	case astAiming, astShooting:
		return "ajustando a mira"
	default:
		return "à deriva"
	}
}
