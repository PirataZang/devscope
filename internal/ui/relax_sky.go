package ui

import (
	"math"
	"math/rand"
)

// ── Starfield ─────────────────────────────────────────────────────────────────
//
// Céu noturno contemplativo: ciclo de ~28–35s em que quase nada acontece.
// Estrelas em três camadas piscando por seno (cada uma com fase/ritmo próprio),
// deriva quase imperceptível, microeventos raros, cometas ocasionais (0–2 por
// ciclo), estrela cadente e — muito raramente — uma constelação.
//
// Estado em per-mille (0–1000) e render mapeado pro tamanho do palco: o céu se
// adapta ao terminal sem recalcular nada. Sem timer próprio: anda no tick de
// 10fps do App e é zerado ao sair do Relax.

const (
	relaxSkyFadeIn  = 14 // frames de crossfade na virada de ciclo
	relaxSkyDimmest = 0.45
	// Célula de terminal é ~2:1: o mesmo per-mille anda muito mais em coluna
	// do que em linha. Sem corrigir, todo cometa sai quase horizontal.
	relaxSkyAspect = 4.0
	// Deriva das camadas: ~1 célula ao longo do ciclo inteiro na camada mais
	// próxima. É pra não parecer imagem estática, não pra ser percebido.
	relaxSkyDrift = 0.035
)

type relaxStar struct {
	x, y   int     // per-mille
	layer  int     // 0 distante · 1 média · 2 próxima
	tint   int     // 0 branco-azulado · 1 âmbar · 2 azul
	base   float64 // brilho base
	amp    float64 // amplitude do piscar (0 = quase estática)
	speed  float64 // rad por frame
	phase  float64
	evKind int // 0 nenhum · 1 brilho crescente · 2 sumindo
	evT    int
	evDur  int
}

type relaxComet struct {
	x, y     float64 // per-mille
	vx, vy   float64
	age, ttl int
	tail     int
	bright   bool // cometa raro: maior, cauda longa, atravessa mais céu
	trail    []relaxSkyPt
}

type relaxSkyPt struct{ x, y float64 }

type relaxConstellation struct {
	idx    []int
	t, dur int
}

type relaxSkyState struct {
	inited    bool
	t         int // frame dentro do ciclo
	dur       int
	stars     []relaxStar
	comets    []relaxComet
	nextComet int // frames até o próximo cometa (-1 = nenhum neste ciclo)
	nextShoot int
	constel   *relaxConstellation

	// Via Láctea: a faixa é uma reta em per-mille com espalhamento gaussiano;
	// dust são os grãos fracos demais pra virar estrela.
	bandSlope float64
	bandY     float64
	dust      []relaxSkyPt

	// Horizonte: três harmônicas de fase sorteada e algumas coníferas na
	// crista. É o que faz a cena virar céu noturno em vez de espaço aberto.
	ridge  [3]float64
	ridgeH float64
	trees  []relaxSkyPt

	aurora  bool
	auroraP [3]float64
}

// relaxSkyNewCycle sorteia um céu novo: agrupamentos naturais, algumas regiões
// vazias, e o roteiro de eventos deste ciclo.
func relaxSkyNewCycle(st *relaxSkyState) {
	st.t = 0
	st.dur = 280 + rand.Intn(71) // 28–35s
	st.stars = st.stars[:0]
	st.constel = nil

	// A inclinação é em per-mille, mas quem decide o ângulo na tela é a razão
	// h/w: 0,5–1,2 aqui dá uma faixa em diagonal suave; acima disso ela sobe
	// quase na vertical e sai do quadro em três colunas.
	st.bandSlope = (0.5 + rand.Float64()*0.7) * float64(1-2*rand.Intn(2))
	st.bandY = 330 + rand.Float64()*300
	st.dust = st.dust[:0]
	rift := -10 + rand.Float64()*20 // a fenda escura não fica no meio da faixa
	for i := 0; i < 460; i++ {
		x := rand.Float64() * 1000
		off := rand.NormFloat64() * 95
		if off > rift-9 && off < rift+22 {
			continue // fenda de poeira: a faixa é partida ao meio, não maciça
		}
		y := st.bandY + st.bandSlope*(x-500) + off
		if y < 0 || y > 1000 {
			continue
		}
		st.dust = append(st.dust, relaxSkyPt{x: x, y: y})
	}

	st.ridge = [3]float64{rand.Float64() * 6.28, rand.Float64() * 6.28, rand.Float64() * 6.28}
	st.ridgeH = 892 + rand.Float64()*40
	// Aurora: evento de alguns ciclos, não de todos — se aparecesse sempre
	// deixaria de ser um acontecimento.
	st.aurora = rand.Intn(3) == 0
	st.auroraP = [3]float64{rand.Float64() * 6.28, rand.Float64() * 6.28, rand.Float64() * 6.28}

	st.trees = st.trees[:0]
	for i, n := 0, 2+rand.Intn(3); i < n; i++ {
		st.trees = append(st.trees, relaxSkyPt{x: 80 + rand.Float64()*840, y: 4 + rand.Float64()*3.4})
	}

	// Aglomerados: a maioria das estrelas nasce perto de um punhado de centros,
	// o resto espalhado — céu uniforme demais não parece céu.
	clusters := 3 + rand.Intn(3)
	centers := make([]relaxSkyPt, clusters)
	for i := range centers {
		centers[i] = relaxSkyPt{x: float64(60 + rand.Intn(880)), y: float64(80 + rand.Intn(840))}
	}
	total := 34 + rand.Intn(16)
	for i := 0; i < total; i++ {
		var x, y int
		if rand.Intn(10) < 4 {
			// Boa parte das estrelas mora na faixa: é ali que o céu é denso.
			fx := rand.Float64() * 1000
			x = int(fx)
			y = int(st.bandY + st.bandSlope*(fx-500) + rand.NormFloat64()*80)
		} else if rand.Intn(10) < 7 {
			c := centers[rand.Intn(len(centers))]
			// spread maior em y: a célula do terminal é ~2:1, então o mesmo
			// per-mille rende muito mais coluna do que linha — sem isso o
			// aglomerado vira uma faixa horizontal.
			x = int(c.x) + rand.Intn(240) - 120
			y = int(c.y) + rand.Intn(500) - 250
		} else {
			x, y = rand.Intn(1000), rand.Intn(1000)
		}
		if x < 0 || x > 999 || y < 0 || y > 999 {
			continue
		}
		layer := 0
		switch r := rand.Intn(10); {
		case r < 5:
			layer = 0
		case r < 8:
			layer = 1
		default:
			layer = 2
		}
		// Quase toda estrela é branco-azulada; umas poucas âmbar dão a
		// sensação de temperatura sem virar céu colorido.
		tint := 0
		switch r := rand.Intn(20); {
		case r < 3:
			tint = 1
		case r < 5:
			tint = 2
		}
		s := relaxStar{
			x:     x,
			y:     y,
			layer: layer,
			tint:  tint,
			base:  0.16 + 0.16*float64(layer) + rand.Float64()*0.22,
			phase: rand.Float64() * 2 * math.Pi,
			speed: (0.012 + rand.Float64()*0.05) / float64(layer+1),
		}
		// Nem toda estrela pisca: algumas ficam quase estáticas o ciclo inteiro.
		switch r := rand.Intn(10); {
		case r < 3:
			s.amp = 0.02 // quase estática
		case r < 7:
			s.amp = 0.08 + rand.Float64()*0.07
		default:
			s.amp = 0.16 + rand.Float64()*0.12
		}
		st.stars = append(st.stars, s)
	}

	// Cometas: às vezes nenhum, normalmente um, raramente dois.
	st.comets = st.comets[:0]
	switch r := rand.Intn(10); {
	case r < 3:
		st.nextComet = -1
	default:
		st.nextComet = 50 + rand.Intn(200) // 5–25s
	}
	st.nextShoot = 60 + rand.Intn(240)
	if rand.Intn(14) == 0 { // constelação: evento raro
		st.constel = &relaxConstellation{dur: 90 + rand.Intn(60)}
	}
}

func relaxSkySpawnComet(st *relaxSkyState) {
	big := rand.Intn(9) == 0 // cometa raro
	speed := 9.0 + rand.Float64()*11
	if big {
		speed = 7 + rand.Float64()*6
	}
	// Diagonal descendente pra um lado ou pro outro, com ângulo variável.
	dir := 1.0
	x := 40.0 + rand.Float64()*260
	if rand.Intn(2) == 0 {
		dir, x = -1, 700+rand.Float64()*260
	}
	ang := 0.45 + rand.Float64()*0.5 // rad abaixo da horizontal
	c := relaxComet{
		x:      x,
		y:      float64(30 + rand.Intn(300)),
		vx:     dir * speed * math.Cos(ang),
		vy:     speed * math.Sin(ang) * relaxSkyAspect,
		ttl:    38 + rand.Intn(26),
		tail:   6 + rand.Intn(4),
		bright: big,
	}
	if big {
		c.ttl += 30
		c.tail += 6
	}
	st.comets = append(st.comets, c)
}

// relaxSkyShootingStar: bem mais discreta que um cometa — atravessa um pedaço
// pequeno do céu em ~0,5–1,5s e some.
func relaxSkyShootingStar(st *relaxSkyState) {
	dir := 1.0
	if rand.Intn(2) == 0 {
		dir = -1
	}
	st.comets = append(st.comets, relaxComet{
		x:    100 + rand.Float64()*800,
		y:    float64(40 + rand.Intn(500)),
		vx:   dir * (16 + rand.Float64()*14),
		vy:   (14 + rand.Float64()*12) * relaxSkyAspect,
		ttl:  5 + rand.Intn(11),
		tail: 3 + rand.Intn(2),
	})
}

func stepRelaxSky(st *relaxSkyState) {
	if !st.inited {
		st.inited = true
		relaxSkyNewCycle(st)
	}
	st.t++

	for i := range st.stars {
		s := &st.stars[i]
		s.phase += s.speed
		if s.evDur > 0 {
			if s.evT++; s.evT >= s.evDur {
				s.evDur, s.evT, s.evKind = 0, 0, 0
			}
		} else if rand.Intn(2200) == 0 { // microevento: raro por estrela
			s.evKind = 1 + rand.Intn(2)
			s.evDur = 30 + rand.Intn(50) // alguns segundos
			s.evT = 0
		}
	}

	if st.nextComet > 0 {
		if st.nextComet--; st.nextComet == 0 {
			relaxSkySpawnComet(st)
			if rand.Intn(5) == 0 { // ocasionalmente um segundo cometa no ciclo
				st.nextComet = 80 + rand.Intn(120)
			} else {
				st.nextComet = -1
			}
		}
	}
	if st.nextShoot > 0 {
		if st.nextShoot--; st.nextShoot == 0 {
			relaxSkyShootingStar(st)
			st.nextShoot = 120 + rand.Intn(200)
		}
	}

	live := st.comets[:0]
	for _, c := range st.comets {
		c.trail = append(c.trail, relaxSkyPt{x: c.x, y: c.y})
		if len(c.trail) > c.tail {
			c.trail = c.trail[len(c.trail)-c.tail:]
		}
		c.x += c.vx
		c.y += c.vy
		if c.age++; c.age < c.ttl && c.x > -80 && c.x < 1080 && c.y < 1080 {
			live = append(live, c)
		}
	}
	st.comets = live

	if st.constel != nil && len(st.stars) > 6 {
		k := st.constel
		if k.idx == nil && st.t > 40 {
			k.idx = relaxSkyPickConstellation(st.stars)
		}
		if k.idx != nil {
			if k.t++; k.t >= k.dur {
				st.constel = nil
			}
		}
	}

	if st.t >= st.dur {
		relaxSkyNewCycle(st) // crossfade: o brilho já está no mínimo aqui
	}
}

// relaxSkyPickConstellation escolhe estrelas vizinhas pra ligar — linhas entre
// pontos distantes não parecem constelação, parecem rabisco.
func relaxSkyPickConstellation(stars []relaxStar) []int {
	start := rand.Intn(len(stars))
	idx := []int{start}
	used := map[int]bool{start: true}
	for len(idx) < 4 {
		cur := stars[idx[len(idx)-1]]
		best, bestD := -1, 300*300
		for i, s := range stars {
			if used[i] {
				continue
			}
			dx, dy := s.x-cur.x, s.y-cur.y
			if d := dx*dx + dy*dy; d < bestD {
				best, bestD = i, d
			}
		}
		if best < 0 {
			break
		}
		used[best] = true
		idx = append(idx, best)
	}
	if len(idx) < 3 {
		return nil
	}
	return idx
}

// ── Render ────────────────────────────────────────────────────────────────────

// brightness junta brilho base, piscar e microevento.
func (s relaxStar) brightness() float64 {
	b := s.base + s.amp*math.Sin(s.phase)
	if s.evDur > 0 {
		p := float64(s.evT) / float64(s.evDur)
		w := math.Sin(p * math.Pi) // sobe e volta, sem degrau
		switch s.evKind {
		case 1:
			b += 0.5 * w
		default:
			b *= 1 - 0.92*w
		}
	}
	return math.Max(0, math.Min(1, b))
}

// relaxSkyFade dá o crossfade da virada: escurece no fim do ciclo e volta no
// começo do seguinte, então o reset parece outro momento do céu, não um reload.
func (st *relaxSkyState) fade() float64 {
	if st.t < relaxSkyFadeIn {
		return relaxSkyDimmest + (1-relaxSkyDimmest)*easeInOut(float64(st.t)/relaxSkyFadeIn)
	}
	if left := st.dur - st.t; left < relaxSkyFadeIn {
		return relaxSkyDimmest + (1-relaxSkyDimmest)*easeInOut(float64(left)/relaxSkyFadeIn)
	}
	return 1
}

// ── Render ────────────────────────────────────────────────────────────────────
//
// Em Braille: a estrela ocupa de um a cinco subpixels conforme o brilho, a
// Via Láctea vira poeira em resolução de subpixel e o horizonte ganha o recorte
// que a grade de células não permitia — encosta e conífera com contorno macio.

const (
	relaxSkyLvlDust = iota * 5
	relaxSkyLvlWarm
	relaxSkyLvlBlue
)

var relaxSkyStops = [3][]string{
	{"#232A3C", "#37415C", "#5A6B8C", "#8FA2C4", "#DCE6F7"}, // neutra
	{"#2F2A21", "#48402E", "#726246", "#AD946A", "#F2DEB4"}, // âmbar
	{"#232C45", "#33436B", "#4E6AA0", "#7C9BD4", "#CFE2FF"}, // azul
}

const (
	relaxSkyNStar   = 15 // 3 rampas × 5 degraus
	relaxSkyAurora  = relaxSkyNStar
	relaxSkyGroundL = relaxSkyAurora + 5
	relaxSkyCrestL  = relaxSkyGroundL + 1
	relaxSkyCometL  = relaxSkyCrestL + 1
	relaxSkyLineL   = relaxSkyCometL + 1
)

var relaxSkyRamp2 = func() []relaxColor {
	out := make([]relaxColor, relaxSkyLineL+1)
	for i, stops := range relaxSkyStops {
		copy(out[i*5:], relaxRamp(stops, 5))
	}
	copy(out[relaxSkyAurora:], relaxRamp([]string{"#16342A", "#256B45", "#43A86A", "#7ADFA0", "#9E8BE8"}, 5))
	out[relaxSkyGroundL] = "#0F131C"
	out[relaxSkyCrestL] = "#1C2430"
	out[relaxSkyCometL] = "#F2F6FF"
	out[relaxSkyLineL] = "#2E3A55"
	return out
}()

func relaxSkyLevel(b float64, tint int) int {
	i := minInt(maxInt(int(b*5), 0), 4)
	return tint*5 + i
}

func relaxSkyFrames(st *relaxSkyState, width, height int, gfade float64) ([]string, string) {
	if !st.inited {
		stepRelaxSky(st)
	}
	w := maxInt(24, minInt(width, 120))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBrailleVote(w, h)
	sw, sh := w*2, h*4
	fade := st.fade() * gfade

	// Horizonte primeiro: relaxBraille não sobrescreve, então o morro apaga
	// naturalmente tudo o que ficaria atrás dele.
	top := relaxSkyRidge(st, sw, sh)
	for x := 0; x < sw; x++ {
		for y := top[x]; y < sh; y++ {
			lvl := relaxSkyGroundL
			if y < top[x]+2 {
				lvl = relaxSkyCrestL
			}
			b.set(x, y, lvl)
		}
	}

	if st.aurora {
		relaxSkyAuroraDraw(st, b, top, sw, sh, fade)
	}

	// Poeira da Via Láctea.
	for i, p := range st.dust {
		x, y := int(p.x*float64(sw-1)/1000), int(p.y*float64(sh-1)/1000)
		lum := (0.16 + 0.10*math.Sin(float64(st.t)*0.02+float64(i))) * fade
		if lum <= relaxHalftone(x, y)*0.42 {
			continue
		}
		b.set(x, y, relaxSkyLevel(lum, 0))
	}

	if k := st.constel; k != nil && k.idx != nil {
		w2 := math.Sin(float64(k.t)/float64(k.dur)*math.Pi) * fade
		if w2 > 0.10 {
			for i := 0; i+1 < len(k.idx); i++ {
				p, q := st.stars[k.idx[i]], st.stars[k.idx[i+1]]
				relaxSkyDash(b, float64(p.x)*float64(sw-1)/1000, float64(p.y)*float64(sh-1)/1000,
					float64(q.x)*float64(sw-1)/1000, float64(q.y)*float64(sh-1)/1000)
			}
		}
	}

	// Estrelas: quanto mais brilhante, mais subpixels — de um ponto solto a uma
	// cruz de cinco. É o que substitui o "·/•/✦" da versão em células.
	for _, s := range st.stars {
		lum := s.brightness() * fade
		if lum < 0.06 {
			continue
		}
		fx := (float64(s.x) + float64(st.t)*relaxSkyDrift*float64(s.layer)) * float64(sw-1) / 1000
		x, y := int(fx), int(float64(s.y)*float64(sh-1)/1000)
		lvl := relaxSkyLevel(lum, s.tint)
		b.set(x, y, lvl)
		if lum > 0.55 {
			b.set(x-1, y, lvl)
			b.set(x+1, y, lvl)
		}
		if lum > 0.80 {
			b.set(x, y-1, lvl)
			b.set(x, y+1, lvl)
		}
	}

	for _, c := range st.comets {
		for i, p := range c.trail {
			f := float64(i+1) / float64(len(c.trail)+1)
			if f < 0.35 && i%2 == 1 {
				continue // a cauda rareia longe do núcleo
			}
			b.set(int(p.x*float64(sw-1)/1000), int(p.y*float64(sh-1)/1000), relaxSkyLevel(0.2+0.6*f, 0))
		}
		cx, cy := int(c.x*float64(sw-1)/1000), int(c.y*float64(sh-1)/1000)
		b.set(cx, cy, relaxSkyCometL)
		if c.bright {
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				b.set(cx+d[0], cy+d[1], relaxSkyCometL)
			}
		}
	}

	return b.lines(relaxStyles(relaxSkyRamp2, gfade)), StyleMuted.Render(relaxSkyStatus(st))
}

// relaxSkyRidge devolve a altura do horizonte por coluna de subpixel: três
// harmônicas de fase sorteada, com as coníferas recortadas por cima.
func relaxSkyRidge(st *relaxSkyState, sw, sh int) []int {
	top := make([]int, sw)
	for x := 0; x < sw; x++ {
		fx := float64(x) / float64(maxInt(1, sw-1)) * 1000
		pm := st.ridgeH + 52*math.Sin(fx*0.0062+st.ridge[0]) +
			30*math.Sin(fx*0.0131+st.ridge[1]) + 14*math.Sin(fx*0.0270+st.ridge[2])
		top[x] = maxInt(2, minInt(sh-1, int(pm*float64(sh-1)/1000+0.5)))
	}
	for _, t := range st.trees {
		cx := int(t.x * float64(sw-1) / 1000)
		if cx < 0 || cx >= sw {
			continue
		}
		hgt := int(t.y * 6.5) // coníferas altas e estreitas, não morrinhos
		base := top[cx]
		for r := 0; r < hgt; r++ {
			half := r/3 + 1
			for dx := -half; dx <= half; dx++ {
				if x := cx + dx; x >= 0 && x < sw {
					top[x] = minInt(top[x], base-(hgt-1-r))
				}
			}
		}
	}
	return top
}

// relaxSkyDash liga duas estrelas com um traço pontilhado, no limite do visível.
func relaxSkyDash(b *relaxBraille, x0, y0, x1, y1 float64) {
	n := int(math.Hypot(x1-x0, y1-y0))
	if n < 6 {
		return
	}
	for i := 3; i < n-2; i++ {
		if i%3 != 0 {
			continue
		}
		f := float64(i) / float64(n)
		b.set(int(lerp(x0, x1, f)), int(lerp(y0, y1, f)), relaxSkyLineL)
	}
}

func relaxSkyAuroraDraw(st *relaxSkyState, b *relaxBraille, top []int, sw, sh int, fade float64) {
	t := float64(st.t) * 0.1
	breath := math.Sin(float64(st.t)/float64(maxInt(1, st.dur))*math.Pi) * fade
	if breath <= 0.05 {
		return
	}
	for x := 0; x < sw; x++ {
		fx := float64(x)
		curtain := 0.55 + 0.45*math.Sin(fx*0.08+t*0.21+st.auroraP[0])
		curtain *= 0.60 + 0.40*math.Sin(fx*0.026-t*0.13+st.auroraP[1])
		curtain += 0.25 * math.Sin(fx*0.17+t*0.37+st.auroraP[2])
		if curtain <= 0.12 {
			continue
		}
		bottom := float64(sh) * (0.22 + 0.16*math.Sin(fx*0.035+t*0.09))
		for y := 0; y < int(bottom) && y < top[x]; y++ {
			v := curtain * breath * (1 - float64(y)/bottom) * 1.05
			if v <= 0.14 || relaxHalftone(x, y) > v {
				continue
			}
			lvl := relaxSkyAurora + minInt(int(v*4), 3)
			if y < 2 && v > 0.8 {
				lvl = relaxSkyAurora + 4 // ponta violeta
			}
			b.set(x, y, lvl)
		}
	}
}

func relaxSkyStatus(st *relaxSkyState) string {
	for _, c := range st.comets {
		if c.bright {
			return "um cometa atravessa o céu"
		}
	}
	if len(st.comets) > 0 {
		return "algo passou lá em cima…"
	}
	if st.constel != nil && st.constel.idx != nil {
		return "uma constelação se desenha"
	}
	return "silêncio"
}
