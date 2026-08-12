package ui

import (
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

// ── Cat ───────────────────────────────────────────────────────────────────────
//
// Um gatinho tirando um cochilo: ~30s dormindo (respirando, com Zzz e algum
// movimento involuntário), ~15s acordado (acorda devagar, espreguiça, boceja,
// se lambe, se ajeita) e volta a dormir. Ciclo de ~45–60s.
//
// Máquina de estados avançada no tick de 10fps do App (stepRelaxCat), render
// puro a partir do estado (relaxCatFrames). Sem timer próprio, sem goroutine,
// sem estado global: sair do Relax zera o estado e o loop de anim morre sozinho
// via needsAnim().

type relaxCatPhase int

const (
	catSleeping relaxCatPhase = iota
	catWaking
	catStretching
	catYawning
	catGrooming
	catSettling
)

// relaxZzz é um "z" (ou um ♪ de ronrom) flutuando: sobe devagar, deriva pro
// lado e some. Posição em décimos de unidade do espaço de projeto do gato, pra
// subida ficar lenta o bastante no passo de 100ms.
type relaxZzz struct {
	x10, y10 int
	rise     int
	drift    int
	age, ttl int
	glyph    string
}

type relaxCatState struct {
	inited bool
	tick   int
	phase  relaxCatPhase
	t      int // frames dentro da fase
	dur    int // duração sorteada da fase

	breath     float64 // fase da respiração (radianos)
	breathRate float64

	zzz   []relaxZzz
	nextZ int

	nextTwitch int
	twitch     int // frames restantes do movimento involuntário
	twitchKind int
	restDx     int // posição em que ele se acomodou pra dormir
	licks      int

	blink     int // frames restantes da piscada
	nextBlink int
}

// relaxCatPhaseDur sorteia a duração de cada fase (em frames de 100ms).
// Sono ~25–35s; acordado somado dá ~14–20s.
func relaxCatPhaseDur(p relaxCatPhase) int {
	switch p {
	case catWaking:
		return 22 + rand.Intn(10)
	case catStretching:
		return 30 + rand.Intn(13)
	case catYawning:
		return 12 + rand.Intn(10)
	case catGrooming:
		return 60 + rand.Intn(22)
	case catSettling:
		return 20 + rand.Intn(12)
	default:
		return 250 + rand.Intn(101)
	}
}

func relaxCatNextPhase(p relaxCatPhase) relaxCatPhase {
	if p == catSettling {
		return catSleeping
	}
	return p + 1
}

// respiração: ~4s por ciclo dormindo, um pouco mais curta acordado, com
// variação pequena pra não bater sempre no mesmo compasso.
func relaxCatBreathRate(p relaxCatPhase) float64 {
	period := 38.0 + float64(rand.Intn(9))
	if p != catSleeping {
		period = 26.0 + float64(rand.Intn(7))
	}
	return 2 * math.Pi / period
}

// relaxCatSpawnZ solta um Z acima da cabeça (unidades do espaço de projeto).
func relaxCatSpawnZ(glyph string) relaxZzz {
	if glyph == "" {
		glyph = "z"
		if rand.Intn(3) == 0 {
			glyph = "Z"
		}
	}
	return relaxZzz{
		x10:   210 + rand.Intn(60),
		y10:   80 + rand.Intn(25),
		rise:  2 + rand.Intn(2),
		drift: rand.Intn(3),
		ttl:   26 + rand.Intn(16),
		glyph: glyph,
	}
}

func stepRelaxCat(st *relaxCatState) {
	if !st.inited {
		st.inited = true
		st.phase = catSleeping
		st.dur = relaxCatPhaseDur(catSleeping)
		st.breathRate = relaxCatBreathRate(catSleeping)
		st.nextZ = 5 + rand.Intn(12)
		st.nextTwitch = 40 + rand.Intn(60)
		st.nextBlink = 30 + rand.Intn(40)
	}
	st.tick++
	st.t++
	if st.breath += st.breathRate; st.breath > 2*math.Pi {
		st.breath -= 2 * math.Pi
	}
	if st.twitch > 0 {
		st.twitch--
	}
	// Piscada só acontece de olho aberto — dormindo não teria o que piscar.
	if st.blink > 0 {
		st.blink--
	} else if st.phase != catSleeping {
		if st.nextBlink--; st.nextBlink <= 0 {
			st.blink = 3
			st.nextBlink = 25 + rand.Intn(45)
		}
	}

	if st.phase == catSleeping {
		if st.nextTwitch--; st.nextTwitch <= 0 {
			st.twitchKind = rand.Intn(4)
			st.twitch = 5 + rand.Intn(9)
			st.nextTwitch = 45 + rand.Intn(75)
			if st.twitchKind == 3 { // se acomodou sem acordar
				st.restDx = rand.Intn(3) - 1
			}
		}
		if st.nextZ--; st.nextZ <= 0 {
			st.zzz = append(st.zzz, relaxCatSpawnZ(""))
			st.nextZ = 8 + rand.Intn(13)
		}
	}
	// Se limpando ele ronrona — mesma física dos Z, outra nota.
	if st.phase == catGrooming {
		if st.nextZ--; st.nextZ <= 0 {
			st.zzz = append(st.zzz, relaxCatSpawnZ("♪"))
			st.nextZ = 14 + rand.Intn(16)
		}
	}

	// Os Z já no ar terminam de subir mesmo depois que ele acorda.
	live := st.zzz[:0]
	for _, z := range st.zzz {
		z.age++
		z.y10 -= z.rise
		z.x10 += z.drift
		if z.age < z.ttl && z.y10 > 0 {
			live = append(live, z)
		}
	}
	st.zzz = live

	if st.t >= st.dur {
		st.phase = relaxCatNextPhase(st.phase)
		st.t = 0
		st.dur = relaxCatPhaseDur(st.phase)
		st.breathRate = relaxCatBreathRate(st.phase)
		st.twitch = 0
		switch st.phase {
		case catSleeping:
			st.nextTwitch = 40 + rand.Intn(60)
			st.nextZ = 12 + rand.Intn(20)
		case catGrooming:
			st.licks = 3 + rand.Intn(3)
			st.nextZ = 8 + rand.Intn(12)
		}
	}
}

// ── Desenho ───────────────────────────────────────────────────────────────────
//
// Desenhado em Braille (2×4 subpixels por célula) sobre um espaço de projeto de
// 62×32 unidades que é escalado pro palco — por isso o gato cresce no terminal
// grande sem perder proporção. As formas são elipses e triângulos sombreados,
// não arte ASCII fixa: a pose é um punhado de números contínuos, então respirar,
// espreguiçar e bocejar são interpolações e não troca de quadro.
//
// A ordem de desenho é da frente pra trás: relaxBraille não sobrescreve ponto
// aceso, então quem chega primeiro fica por cima.

var relaxCatFurStops = []string{"#2B1E15", "#5A3A20", "#96612F", "#CE8F49", "#F2C782"}

// Níveis 0..9 são o pelo; daí pra frente cada detalhe tem cor própria e é
// fixado na célula com paint(), senão a média o dissolveria no pelo.
const relaxCatFurLevels = 10

const (
	relaxCatEye = relaxCatFurLevels + iota
	relaxCatPink
	relaxCatWhisker
	relaxCatZzz
)

var relaxCatRamp = func() []lipgloss.Color {
	out := make([]lipgloss.Color, relaxCatZzz+1)
	copy(out, relaxRamp(relaxCatFurStops, relaxCatFurLevels))
	out[relaxCatEye] = lipgloss.Color("#8FD86A")
	out[relaxCatPink] = lipgloss.Color("#E79AA6")
	out[relaxCatWhisker] = lipgloss.Color("#EDE6D6")
	out[relaxCatZzz] = lipgloss.Color("#7C88A0")
	return out
}()

// relaxCatPose é a pose em números contínuos. Toda fase escreve aqui, e o
// desenho só lê — é o que permite misturar respiração com espreguiçar sem que
// cada fase precise saber desenhar.
type relaxCatPose struct {
	breath  float64 // -1..1
	headDX  float64
	headDY  float64 // >0 = cabeça baixa, apoiada
	eye     float64 // 0 fechado .. 1 arregalado
	mouth   float64 // 0 fechado .. 1 bocejo
	reach   float64 // patas dianteiras esticadas
	stretch float64 // corpo alongado
	arch    float64 // costas arqueadas: peito no chão, anca no alto
	tailUp  float64
	earL    float64 // 0 em pé .. 1 virada
	earR    float64
	pawUp   float64 // pata levantada pra lamber
	tail    float64 // amplitude do rabo
}

func relaxCatPoseOf(st *relaxCatState) relaxCatPose {
	p := 0.0
	if st.dur > 0 {
		p = float64(st.t) / float64(st.dur)
	}
	po := relaxCatPose{breath: math.Sin(st.breath), tail: 0.30}

	switch st.phase {
	case catSleeping:
		po.headDY, po.tail = 2.6, 0.12
		if st.twitch > 0 {
			// O movimento involuntário sobe e desce, não liga e desliga.
			g := math.Sin(clamp01(float64(st.twitch)/9) * math.Pi)
			switch st.twitchKind {
			case 0:
				po.earL = g
			case 1:
				po.earR = g
			case 2:
				po.headDX = g * 1.4
			default:
				po.reach = g * 0.35
			}
		}

	case catWaking:
		// Acordar em tempos, não numa rampa só: a cabeça sobe, as orelhas
		// giram uma de cada vez, os olhos abrem em fenda antes de abrir.
		g := easeInOut(p)
		po.headDY = 2.6 * (1 - easeInOut(clamp01((p-0.10)/0.55)))
		po.eye = 0.75 * easeInOut(clamp01((p-0.30)/0.55))
		po.headDX = 1.8 * math.Sin(clamp01((p-0.22)/0.50)*math.Pi)
		po.earL = math.Sin(clamp01((p-0.16)/0.20) * math.Pi)
		po.earR = math.Sin(clamp01((p-0.34)/0.20) * math.Pi)
		po.tail = 0.12 + 0.25*g

	case catStretching:
		var g float64
		switch {
		case p < 0.45:
			g = easeInOut(p / 0.45)
		case p < 0.70:
			g = 1 // segura o alongamento
		default:
			g = 1 - easeInOut((p-0.70)/0.30)
		}
		// Reverência de brincar: patas longe à frente, peito no chão, anca
		// no alto, cabeça entre os ombros e rabo levantado. Só alongar a
		// elipse do tronco fazia o gato ficar comprido, não espreguiçado.
		po.reach, po.stretch, po.arch, po.tailUp = g, g, g, g
		po.headDY = 5.2 * g    // cabeça no chão, entre as patas
		po.eye = 0.85 - 0.78*g // aperta os olhos no esforço
		po.tail = 0.30 + 0.55*g

	case catYawning:
		w := math.Sin(clamp01(p) * math.Pi)
		po.mouth, po.eye, po.tail = w, 0.85-0.70*w, 0.25

	case catGrooming:
		// Três lambidas: a pata sobe, encosta e volta.
		seg := math.Mod(p*float64(maxInt(1, st.licks)), 1)
		po.pawUp = math.Sin(clamp01(seg) * math.Pi)
		po.eye = 0.85 - 0.45*po.pawUp
		po.headDY = po.pawUp * 1.2
		po.tail = 0.45

	case catSettling:
		g := easeInOut(p)
		po.headDY, po.eye, po.tail = 2.6*g, 0.8*(1-g), 0.30-0.18*g
	}

	// Piscada: independente da fase, e nunca abaixo de zero.
	if st.blink > 0 {
		po.eye *= 1 - math.Sin(clamp01(float64(st.blink)/3)*math.Pi)
	}
	return po
}

// relaxCatEllipse pinta uma elipse cheia com sombreado: a luz vem de cima e um
// pouco da esquerda, e o tigrado entra como listra escura sobre isso.
func relaxCatEllipse(b *relaxBraille, cx, cy, rx, ry, tone float64, stripes bool) {
	for y := int(cy - ry); y <= int(cy+ry)+1; y++ {
		for x := int(cx - rx); x <= int(cx+rx)+1; x++ {
			nx, ny := (float64(x)-cx)/rx, (float64(y)-cy)/ry
			if nx*nx+ny*ny > 1 {
				continue
			}
			l := tone - 0.32*ny - 0.10*nx
			if stripes {
				l -= 0.13 * math.Max(0, math.Sin(float64(x)*0.55+float64(y)*0.11))
			}
			b.set(x, y, int(clamp01(l)*float64(relaxCatFurLevels-1)+0.5))
		}
	}
}

// relaxCatSpine desenha o tronco como uma corrente de discos sobre uma bezier
// quadrática (ombro · meio · anca). Os discos ficam a pouco mais de uma unidade
// um do outro, então o sombreado de cada um bate com o do vizinho e o tubo sai
// contínuo em vez de listrado.
func relaxCatSpine(b *relaxBraille, p0, p1, p2 [3]float64) {
	for i := 0; i <= 28; i++ {
		t := float64(i) / 28
		u := 1 - t
		wa, wb, wc := u*u, 2*u*t, t*t
		r := wa*p0[2] + wb*p1[2] + wc*p2[2]
		relaxCatEllipse(b, wa*p0[0]+wb*p1[0]+wc*p2[0], wa*p0[1]+wb*p1[1]+wc*p2[1],
			r*1.04, r, 0.52, true)
	}
}

func (p relaxCatPhase) status() string {
	switch p {
	case catWaking:
		return "acordando…"
	case catStretching:
		return "se espreguiçando"
	case catYawning:
		return "bocejando"
	case catGrooming:
		return "se limpando, ronronando"
	case catSettling:
		return "se ajeitando"
	default:
		return "dormindo…"
	}
}

func relaxCatFrames(st *relaxCatState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxCat(st)
	}
	w := maxInt(26, minInt(width, 80))
	h := maxInt(7, minInt(height, 22))
	b := newRelaxBraille(w, h)
	relaxCatDraw(st, b, w, h)

	return b.lines(relaxStyles(relaxCatRamp, fade)), StyleMuted.Render(st.phase.status())
}

func relaxCatDraw(st *relaxCatState, b *relaxBraille, w, h int) {
	sw, sh := float64(w*2), float64(h*4)
	// Espaço de projeto de 62×32 unidades, escalado e centralizado no palco.
	sc := math.Min(sw/62, sh/32)
	ox, oy := (sw-62*sc)/2, (sh-32*sc)/2
	X := func(v float64) float64 { return ox + v*sc }
	Y := func(v float64) float64 { return oy + v*sc }
	L := func(v float64) float64 { return v * sc }
	cell := func(x, y float64) (int, int) { return int(x) / 2, int(y) / 4 }

	po := relaxCatPoseOf(st)
	t := float64(st.tick) * 0.1

	// Âncoras. Tudo pendura na cabeça e no corpo; a pose só empurra esses dois
	// e o resto acompanha.
	headX := X(15.5 - po.reach*2.5 + po.headDX)
	headY := Y(12.6 + po.headDY*1.6 - po.breath*0.15)
	pawY := Y(27.0 + po.arch*1.4)
	// Coluna: ombro, meio e anca. Arquear move esses três, e o tronco inteiro
	// acompanha — é o que separa "espreguiçando" de "mais comprido".
	breathe := po.breath * 0.35
	shoulder := [3]float64{X(21.5 - po.reach*1.2), Y(21.4 + po.arch*3.6 - breathe), L(5.4)}
	midBack := [3]float64{X(33.5 + po.stretch*1.0), Y(20.0 - po.arch*2.2 - breathe), L(6.3 + po.breath*0.35)}
	rump := [3]float64{X(45.5 + po.stretch*2.2), Y(21.8 - po.arch*3.4), L(5.8)}

	// ── Frente ──
	// Bigodes primeiro: fora da silhueta ficam com a cor própria; sobre o
	// focinho se misturam ao pelo, que é como se vê de verdade.
	for i, dy := range []float64{-1.2, 0.0, 1.2} {
		for _, dir := range []float64{-1, 1} {
			x0, y0 := headX+dir*L(2.0), headY+L(2.4)
			b.line(x0, y0, x0+dir*L(6.8), y0+L(dy)-L(float64(i)*0.1), relaxCatWhisker)
		}
	}

	// Olhos: fenda quando fechados, disco com pupila em fenda quando abertos.
	for _, dir := range []float64{-1, 1} {
		ex, ey := headX+dir*L(2.7), headY-L(0.6)
		if po.eye < 0.12 {
			b.line(ex-L(1.5), ey, ex+L(1.5), ey, 1)
			continue
		}
		ry := L(1.3) * (0.4 + 0.6*po.eye)
		rx := L(1.6)
		relaxCatEllipse(b, ex, ey, rx, ry, 0.95, false)
		// Nesta escala o olho tem ~3 subpixels: pupila não caberia. Fixar a
		// cor das células que ele ocupa é o que faz o verde aparecer inteiro.
		for cy := int(ey-ry) / 4; cy <= int(ey+ry)/4; cy++ {
			for cx := int(ex-rx) / 2; cx <= int(ex+rx)/2; cx++ {
				b.paint(cx, cy, relaxCatEye)
			}
		}
	}

	// Nariz, boca e focinho.
	nx, ny := headX, headY+L(2.2)
	b.tri(nx-L(0.9), ny-L(0.5), nx+L(0.9), ny-L(0.5), nx, ny+L(0.7), relaxCatPink)
	pcx, pcy := cell(nx, ny)
	b.paint(pcx, pcy, relaxCatPink)
	if po.mouth > 0.05 {
		relaxCatEllipse(b, nx, ny+L(1.2+1.1*po.mouth), L(1.2+0.4*po.mouth), L(0.5+1.7*po.mouth), 0.05, false)
	}
	relaxCatEllipse(b, headX-L(1.4), headY+L(2.9), L(2.0), L(1.4), 0.88, false)
	relaxCatEllipse(b, headX+L(1.4), headY+L(2.9), L(2.0), L(1.4), 0.88, false)

	// Orelhas: interior rosa desenhado antes, casco por cima.
	for _, e := range []struct{ dir, fold float64 }{{-1, po.earL}, {1, po.earR}} {
		bx, by := headX+e.dir*L(3.8), headY-L(3.8)
		tipX, tipY := bx+e.dir*L(1.0+1.8*e.fold), by-L(4.6-2.8*e.fold)
		b.tri(bx-L(1.1), by, bx+L(1.1), by, tipX, tipY+L(1.3), relaxCatPink)
		b.tri(bx-L(2.2), by+L(1.0), bx+L(2.2), by+L(1.0), tipX, tipY, 3)
	}

	// Cabeça — um tom mais clara que o corpo, senão as duas massas se fundem.
	relaxCatEllipse(b, headX, headY, L(6.2), L(5.8), 0.70, true)

	// Patas dianteiras. A perna é uma corrente de discos do ombro até a pata:
	// com elipse fixa a pata se soltava do corpo no espreguiçar.
	for i, dir := range []float64{-1, 1} {
		px := headX + dir*L(2.6) - L(po.reach*5.5)
		py := pawY - L(po.pawUp*6.5*float64(i))
		if i == 1 {
			px += L(po.pawUp * 2.2)
		}
		sx, sy := headX+dir*L(2.2)+L(po.reach*1.0), Y(21.0+po.arch*3.2)
		for k := 0; k <= 8; k++ {
			f := float64(k) / 8
			relaxCatEllipse(b, lerp(sx, px+L(0.8), f), lerp(sy, py, f), L(1.9-0.2*f), L(1.9-0.4*f), 0.62, false)
		}
		relaxCatEllipse(b, px, py, L(2.6), L(1.3), 0.80, false)
	}

	// Tronco.
	relaxCatSpine(b, shoulder, midBack, rump)

	// Rabo: arco de discos que afina na ponta e balança devagar. Na
	// reverência ele sobe junto com a anca.
	tbx, tby := rump[0]+L(3.0), rump[1]+L(2.2)
	for i := 0; i <= 30; i++ {
		f := float64(i) / 30
		a := -0.30 + f*2.2 + po.tail*0.26*math.Sin(t*1.1+f*3.4)
		tx := tbx + L(6.5-po.tailUp*2.6)*math.Sin(a)
		ty := tby - L(6.5+po.tailUp*3.2)*(1-math.Cos(a))
		relaxCatEllipse(b, tx, ty, L(1.5-0.6*f), L(1.4-0.6*f), 0.46, false)
	}

	// Tapete: bem apagado, só pro gato não flutuar.
	for y := int(Y(27.6)); y <= int(Y(30)); y++ {
		for x := int(X(5)); x <= int(X(57)); x++ {
			nx := (float64(x) - X(31)) / L(25)
			ny := (float64(y) - Y(28.8)) / L(1.3)
			if nx*nx+ny*ny <= 1 {
				// Nível 0 (o pelo mais escuro) em vez de cor própria: o tapete
				// divide célula com as patas, e a média de duas paletas daria
				// uma cor que não é nem uma nem outra.
				b.set(x, y, 0)
			}
		}
	}

	// Zzz e ronrom, por cima de tudo (texto, não Braille).
	for _, z := range st.zzz {
		cx, cy := cell(X(float64(z.x10)/10), Y(float64(z.y10)/10))
		b.text(cx, cy, z.glyph, relaxCatZzz)
	}
}
