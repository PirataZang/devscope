package ui

import (
	"math"
	"math/rand"
)

// ── Ampulheta cósmica ─────────────────────────────────────────────────────────
//
// A areia é estrela. No bulbo de cima elas descem em vórtice encostadas no
// vidro, passam uma a uma pelo gargalo e, embaixo, caem em órbita: o monte que
// se forma é um disco espiral. Quando o de cima esvazia a ampulheta vira, e a
// galáxia inteira volta a ser areia.
//
// Tudo mora em coordenada local: x em -1..1, y de -1 (tampa de cima) a 1 (a de
// baixo), gargalo em 0. A virada é uma rotação de π no desenho; quando ela
// fecha, as estrelas são espelhadas e o quadro fica idêntico ao de antes — é
// isso que faz o ciclo não ter emenda.

const (
	relaxHgDisc   = 0.60 // altura do centro do disco no bulbo de baixo
	relaxHgFlat   = 0.46 // achatamento do disco: ele é visto de viés
	relaxHgTwist  = 3.4  // quanto a elipse gira por unidade de raio — dá o braço
	relaxHgSquash = 0.21 // achatamento dos aros: a ampulheta é vista de pouco acima
	relaxHgRim    = 0.488
	relaxHgNeck   = 0.043
	relaxHgN      = 430

	relaxHgFlipDur  = 24 // passos da virada (~2,4s)
	relaxHgFlipWait = 26 // parado, olhando a galáxia pronta, antes de virar
)

// relaxHgProfile é o contorno do vidro: |y| (0 no gargalo, 1 na tampa) → raio.
// Tabela, e não fórmula, porque é ela que dá a barriga do bojo e o ombro que
// fecha na boca — dois senos fazem cone, não ampulheta.
var relaxHgProfile = [][2]float64{
	{0.000, 0.043}, {0.028, 0.065}, {0.062, 0.107}, {0.105, 0.162},
	{0.160, 0.223}, {0.225, 0.286}, {0.300, 0.343}, {0.385, 0.393},
	{0.480, 0.433}, {0.580, 0.464}, {0.680, 0.484}, {0.780, 0.494},
	{0.860, 0.489}, {0.930, 0.466}, {0.975, 0.429}, {1.000, 0.393},
}

// relaxHgHalf é a meia-largura do vidro na altura y.
func relaxHgHalf(y float64) float64 {
	a := math.Abs(y)
	last := relaxHgProfile[len(relaxHgProfile)-1]
	if a >= last[0] {
		return last[1]
	}
	for i := 1; i < len(relaxHgProfile); i++ {
		if p := relaxHgProfile[i]; a <= p[0] {
			q := relaxHgProfile[i-1]
			return lerp(q[1], p[1], (a-q[0])/(p[0]-q[0]))
		}
	}
	return last[1]
}

// relaxHgSpan é o inverso: entre que alturas o vidro comporta o raio r. Devolve
// as duas bordas porque o bojo fecha em cima — sem o teto, a estrela do vórtice
// atravessaria o ombro.
func relaxHgSpan(r float64) (float64, float64) {
	lo, hi := 0.0, 1.0
	for i := 1; i < len(relaxHgProfile); i++ {
		p, q := relaxHgProfile[i-1], relaxHgProfile[i]
		if p[1] == q[1] || (p[1]-r)*(q[1]-r) > 0 {
			continue
		}
		y := lerp(p[0], q[0], (r-p[1])/(q[1]-p[1]))
		if q[1] > p[1] {
			lo = y
		} else {
			hi = y
		}
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// ── Paleta ────────────────────────────────────────────────────────────────────

const (
	relaxHgStarN   = 6
	relaxHgTintN   = 2
	relaxHgGlassN  = 6
	relaxHgBronzeN = 6
	relaxHgCoreN   = 5
)

const (
	relaxHgStar   = 0
	relaxHgGlass  = relaxHgStar + relaxHgStarN*relaxHgTintN
	relaxHgBronze = relaxHgGlass + relaxHgGlassN
	relaxHgCore   = relaxHgBronze + relaxHgBronzeN
	relaxHgFlash  = relaxHgCore + relaxHgCoreN
)

var relaxHgPal = func() []relaxColor {
	out := make([]relaxColor, relaxHgFlash+1)
	// Duas temperaturas de estrela: a maioria branco-azulada, algumas âmbar.
	copy(out[relaxHgStar:], relaxRamp([]string{"#1B2440", "#2E4472", "#4E76B4", "#84AEE2", "#C2DCF6", "#FFFFFF"}, relaxHgStarN))
	copy(out[relaxHgStar+relaxHgStarN:], relaxRamp([]string{"#2A1E12", "#573A17", "#946123", "#CE9445", "#EFC98A", "#FFF0CE"}, relaxHgStarN))
	copy(out[relaxHgGlass:], relaxRamp([]string{"#0C1E26", "#153C48", "#22626F", "#3A94A4", "#79C9D8", "#E4FBFF"}, relaxHgGlassN))
	copy(out[relaxHgBronze:], relaxRamp([]string{"#180E04", "#33200C", "#573A14", "#8A6224", "#BE8F3E", "#F0CE86"}, relaxHgBronzeN))
	copy(out[relaxHgCore:], relaxRamp([]string{"#2A1C2E", "#5C3A46", "#9A6C58", "#D6AC7C", "#FFEFC8"}, relaxHgCoreN))
	out[relaxHgFlash] = "#FFFFFF"
	return out
}()

// ── Estado ────────────────────────────────────────────────────────────────────

const (
	relaxHgVortex = iota // descendo em espiral pelo bulbo de cima
	relaxHgFall          // atravessando o gargalo
	relaxHgOrbit         // em órbita no disco de baixo
)

type relaxHgStar2 struct {
	phase int8
	tint  int8
	bri   float64

	r, th, d, drain        float64 // vórtice: raio, ângulo em torno do eixo, altura sobre a parede
	x, y, vy               float64 // queda pelo gargalo
	a, bfac, orb, om, grow float64 // órbita no disco
}

type relaxHourglassState struct {
	inited  bool
	tick    int
	stars   []relaxHgStar2
	pattern float64 // fase do padrão espiral do disco
	flipT   int     // >0 durante a virada
	rest    int     // passos parados depois que o de cima esvaziou
	fell    int     // quantas passaram pelo gargalo neste ciclo
	neck    float64 // brilho do gargalo, decai a cada passo
}

func relaxHgNewStar() relaxHgStar2 {
	s := relaxHgStar2{
		// Raio com raiz: área igual por faixa, senão o vórtice fica oco no meio.
		r: relaxHgNeck + (relaxHgRim-relaxHgNeck)*math.Sqrt(rand.Float64()),
		// nasce em qualquer altura que o vidro comporte naquele raio
		th:    rand.Float64() * 2 * math.Pi,
		d:     rand.Float64(),
		drain: 0.0024 + rand.Float64()*0.0020,
		bri:   0.45 + rand.Float64()*0.55,
	}
	if rand.Intn(6) == 0 {
		s.tint = 1
	}
	return s
}

func stepRelaxHourglass(st *relaxHourglassState) {
	if !st.inited {
		st.inited = true
		st.stars = make([]relaxHgStar2, relaxHgN)
		for i := range st.stars {
			st.stars[i] = relaxHgNewStar()
		}
	}
	st.tick++
	st.pattern += 0.009
	st.neck *= 0.86

	if st.flipT > 0 {
		if st.flipT++; st.flipT > relaxHgFlipDur {
			st.flipT = 0
			relaxHgTurn(st)
		}
	}

	top := 0
	for i := range st.stars {
		s := &st.stars[i]
		switch s.phase {
		case relaxHgVortex:
			top++
			// Vórtice: gira mais rápido quanto mais perto do eixo, e desce
			// junto com o raio — a estrela acompanha a parede pra dentro.
			s.th += 0.028 + 0.052/(s.r+0.16)
			s.r -= s.drain * (0.34 + 0.66*(1-s.r))
			if s.d -= s.drain * 0.9; s.d < 0 {
				s.d = 0
			}
			if s.r <= relaxHgNeck {
				s.phase, s.x, s.y, s.vy = relaxHgFall, (rand.Float64()-0.5)*relaxHgNeck, 0, 0.006
			}
		case relaxHgFall:
			top++
			s.vy += 0.0021
			s.y += s.vy
			if math.Abs(s.y) < 0.05 {
				st.neck = 1
			}
			if s.y >= relaxHgDisc-0.16 {
				st.fell++
				s.phase = relaxHgOrbit
				// Chega no miolo e é jogada pra fora: o semieixo cresce de 0
				// até o alvo em ~2s, que é o disco se montando em vez de
				// aparecer pronto.
				s.a = 0.06 + 0.29*math.Sqrt(rand.Float64())
				s.bfac = 0.52 + 0.30*rand.Float64()
				s.orb = rand.Float64() * 2 * math.Pi
				s.om = 0.052 / (0.34 + s.a*1.7)
				s.grow = 0
			}
		default:
			s.orb += s.om
			if s.grow < 1 {
				s.grow = math.Min(1, s.grow+0.017)
			}
		}
	}

	// Bulbo de cima vazio: um tempo olhando a galáxia pronta e vira.
	if top == 0 && st.flipT == 0 {
		if st.rest++; st.rest > relaxHgFlipWait {
			st.rest, st.flipT = 0, 1
		}
	}
}

// relaxHgTurn é o instante em que a virada fecha: as estrelas são espelhadas e
// o disco vira vórtice de novo. Como o desenho já estava rodado de π, espelhar
// devolve exatamente o mesmo quadro — a troca não aparece.
func relaxHgTurn(st *relaxHourglassState) {
	st.fell = 0
	for i := range st.stars {
		s := &st.stars[i]
		x, y := relaxHgPos(st, *s)
		x, y = -x, -y
		r := math.Max(relaxHgNeck*1.05, math.Min(relaxHgRim*0.99, math.Abs(x)))
		th := 0.0
		if x < 0 {
			th = math.Pi
		}
		lo, hi := relaxHgSpan(r)
		*s = relaxHgStar2{
			phase: relaxHgVortex,
			tint:  s.tint,
			bri:   s.bri,
			r:     r,
			th:    th,
			d:     clamp01((math.Abs(y) - lo) / math.Max(0.05, hi-lo)),
			drain: 0.0024 + rand.Float64()*0.0020,
		}
	}
}

// relaxHgPos devolve a posição da estrela em coordenada local, antes da virada.
func relaxHgPos(st *relaxHourglassState, s relaxHgStar2) (float64, float64) {
	switch s.phase {
	case relaxHgVortex:
		lo, hi := relaxHgSpan(s.r)
		return s.r * math.Cos(s.th), -(lo + s.d*(hi-lo))
	case relaxHgFall:
		return s.x, s.y
	}
	// Disco: elipse cuja orientação gira com o raio. É a precessão que junta as
	// órbitas em braço; sem ela o disco é um borrão redondo.
	ang := s.a*relaxHgTwist + st.pattern
	ca, sa := math.Cos(ang), math.Sin(ang)
	ex, ey := s.a*math.Cos(s.orb), s.a*s.bfac*math.Sin(s.orb)
	return (ex*ca - ey*sa) * s.grow, relaxHgDisc + (ex*sa+ey*ca)*relaxHgFlat*s.grow
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxHourglassFrames(st *relaxHourglassState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxHourglass(st)
	}
	w := maxInt(20, minInt(width, 120))
	h := maxInt(8, minInt(height, 34))
	b := newRelaxBrailleVote(w, h)
	relaxHgDraw(st, b)

	status := "a areia é feita de estrela"
	switch {
	case st.flipT > 0:
		status = "o tempo vira"
	case st.rest > 0:
		status = "a galáxia está pronta"
	case st.fell > relaxHgN/2:
		status = "o disco já tem braço"
	case st.fell > 12:
		status = "uma galáxia se forma embaixo"
	}
	return b.lines(relaxStyles(relaxHgPal, fade)), StyleMuted.Render(status)
}

func relaxHgDraw(st *relaxHourglassState, b *relaxBraille) {
	sw, sh := b.w*2, b.h*4
	scale := math.Min(float64(sw)/1.34, float64(sh)/2.32)
	cx, cy := float64(sw)/2, float64(sh)/2

	// A virada roda o desenho inteiro; π no fim deixa a ampulheta igual, e é
	// por isso que o espelhamento das estrelas passa despercebido.
	ang := 0.0
	if st.flipT > 0 {
		ang = math.Pi * easeInOut(float64(st.flipT)/relaxHgFlipDur)
	}
	sn, cs := math.Sin(ang), math.Cos(ang)
	put := func(x, y float64, lvl int) {
		b.set(int(cx+(x*cs-y*sn)*scale), int(cy+(x*sn+y*cs)*scale), lvl)
	}

	relaxHgFrame(put, scale)
	relaxHgGlassDraw(put, scale)

	// Bulbo do disco: o brilho difuso cresce com o que já caiu. Sem ele o
	// disco é um punhado de pontos; com ele vira galáxia.
	if load := float64(st.fell) / float64(relaxHgN); load > 0.02 {
		rad := 0.17 * math.Min(1, 0.45+load)
		for dy := -rad; dy <= rad; dy += 0.5 / scale {
			for dx := -rad * 2.2; dx <= rad*2.2; dx += 0.5 / scale {
				q := (dx*dx)/(rad*rad*4.8) + (dy*dy)/(rad*rad)
				if q >= 1 {
					continue
				}
				v := (1 - q) * (1 - q) * (1 - q) * load * 0.95
				px, py := cx+(dx*cs-(relaxHgDisc+dy)*sn)*scale, cy+(dx*sn+(relaxHgDisc+dy)*cs)*scale
				if v < 0.06 || relaxHalftone(int(px), int(py)) > v {
					continue
				}
				put(dx, relaxHgDisc+dy, relaxHgCore+minInt(int(v*float64(relaxHgCoreN)*1.4), relaxHgCoreN-1))
			}
		}
	}

	// Gargalo: acende quando alguém passa.
	if st.neck > 0.08 {
		for i := 0; i < 26; i++ {
			a := float64(i) / 26 * 2 * math.Pi
			r := relaxHgNeck * (1.2 + 1.6*st.neck)
			put(math.Cos(a)*r, math.Sin(a)*r*0.6, relaxHgCore+minInt(int(st.neck*5), relaxHgCoreN-1))
		}
	}

	for _, s := range st.stars {
		x, y := relaxHgPos(st, s)
		lum := s.bri
		switch s.phase {
		case relaxHgVortex:
			// Quem está na frente do eixo brilha mais: é o único jeito de o
			// vórtice ter volume num desenho sem profundidade.
			lum *= 0.62 + 0.38*math.Cos(s.th)
		case relaxHgOrbit:
			// Braço: as órbitas se acotovelam nas pontas do eixo maior.
			arm := math.Cos(s.orb)
			lum *= (0.52 + 0.62*arm*arm) * (0.35 + 0.65*s.grow)
		default:
			lum *= 1.15
		}
		lvl := int(s.tint)*relaxHgStarN + minInt(maxInt(int(clamp01(lum)*float64(relaxHgStarN)), 0), relaxHgStarN-1)
		put(x, y, relaxHgStar+lvl)
		if lum > 0.86 {
			put(x+0.5/scale, y, relaxHgStar+lvl)
			put(x-0.5/scale, y, relaxHgStar+lvl)
		}
	}

}

func relaxHgLvl(lum float64, n int) int {
	return minInt(maxInt(int(clamp01(lum)*float64(n)), 0), n-1)
}

// relaxHgRing desenha um aro elíptico: é o único lugar em que a peça mostra
// que é redonda e não recortada em papel.
func relaxHgRing(put func(float64, float64, int), y, r, scale float64, lvl int) {
	n := maxInt(16, int(2*math.Pi*r*scale))
	for i := 0; i <= n; i++ {
		a := float64(i) / float64(n) * 2 * math.Pi
		put(r*math.Cos(a), y+r*relaxHgSquash*math.Sin(a), lvl)
	}
}

// relaxHgGlassDraw desenha o vidro: parede com espessura, realce especular e os
// aros da boca. Contorno de um ponto não lê como vidro — lê como arame. O que
// faz o material aparecer é a faixa de realce no terço iluminado, que acende
// onde a parede está de pé e apaga onde ela deita.
func relaxHgGlassDraw(put func(float64, float64, int), scale float64) {
	step := 0.5 / scale
	for y := -1.0; y <= 1.0; y += step {
		r := relaxHgHalf(y)
		face := 1 / math.Hypot(1, (relaxHgHalf(y+0.03)-relaxHgHalf(y-0.03))/0.06)
		hot := 0.34 + 0.66*face
		edge := relaxHgGlass + relaxHgLvl(hot, relaxHgGlassN)
		inner := relaxHgGlass + maxInt(0, relaxHgLvl(hot, relaxHgGlassN)-2)
		for _, sgn := range [2]float64{-1, 1} {
			put(sgn*r, y, edge)
			put(sgn*(r-1.3*step), y, inner)
		}
		// Dois realces: a faixa larga do lado da luz e o fio do lado oposto,
		// que é o que a luz devolve depois de atravessar a peça.
		for _, h := range [3]float64{-0.55, 0.72, 0} {
			if h == 0 {
				continue
			}
			hw := 0.075 * r
			gain := 1.0
			if h > 0 {
				hw, gain = 0.030*r, 0.55
			}
			for dx := -hw; dx <= hw; dx += step {
				x := h*r + dx
				v := (1 - math.Abs(dx)/math.Max(hw, 1e-9)) * face * gain
				if v < 0.18 || relaxHalftone(int(x*scale), int(y*scale)) > v*0.92 {
					continue
				}
				put(x, y, relaxHgGlass+relaxHgLvl(v*1.15, relaxHgGlassN))
			}
		}
	}
	for _, sgn := range [2]float64{-1, 1} {
		relaxHgRing(put, sgn*0.995, relaxHgHalf(1), scale, relaxHgGlass+relaxHgGlassN-2)
	}
}

// relaxHgFrame são as duas bases, o colar do gargalo e os montantes da frente.
// A base é um disco visto de pouco acima — elipse cheia com a face clara e a
// espessura escura na borda —, e o montante é torneado, com barriga no meio e
// colar nas pontas. Chapa reta e barra reta é o que dava cara de mesa.
func relaxHgFrame(put func(float64, float64, int), scale float64) {
	step := 0.5 / scale
	const plate = 0.60
	for _, sgn := range [2]float64{-1, 1} {
		fy := sgn * 1.03
		for dy := -plate * relaxHgSquash; dy <= plate*relaxHgSquash; dy += step {
			for dx := -plate; dx <= plate; dx += step {
				q := (dx*dx)/(plate*plate) + (dy*dy)/(plate*plate*relaxHgSquash*relaxHgSquash)
				if q > 1 {
					continue
				}
				// Luz de cima à esquerda; a borda do disco escurece, o que
				// arredonda a chapa em vez de deixá-la de papel.
				lum := 0.60 - 0.26*dx/plate - 0.20*sgn*dy/(plate*relaxHgSquash) - 0.30*q*q
				put(dx, fy+dy, relaxHgBronze+relaxHgLvl(lum, relaxHgBronzeN))
			}
		}
		for dx := -plate; dx <= plate; dx += step {
			k := 1 - (dx*dx)/(plate*plate)
			if k < 0 {
				continue
			}
			e := plate * relaxHgSquash * math.Sqrt(k)
			for t := 0.0; t <= 0.075; t += step {
				put(dx, fy+sgn*(e+t), relaxHgBronze+relaxHgLvl(0.34-0.22*dx/plate-t*3.4, relaxHgBronzeN))
			}
		}
	}
	// Colar do gargalo: aro de bronze na cintura, com o furo por onde a areia
	// passa. É ele que resolve o encontro dos dois bojos.
	for y := -0.055; y <= 0.055; y += step {
		w := 0.098 * math.Sqrt(math.Max(0, 1-(y/0.062)*(y/0.062)))
		for x := relaxHgHalf(y); x <= w; x += step {
			for _, sgn := range [2]float64{-1, 1} {
				put(sgn*x, y, relaxHgBronze+relaxHgLvl(0.80-0.55*sgn-0.9*math.Abs(y)/0.055, relaxHgBronzeN))
			}
		}
	}
	for _, sgn := range [2]float64{-1, 1} {
		relaxHgPost(put, sgn*0.560, scale, 1)
	}
}

// relaxHgPost é um montante torneado: barriga no meio, colar nas pontas. O
// lado esquerdo pega a luz — é o gradiente na largura de três subpontos que
// faz a barra parecer cilindro em vez de risco.
func relaxHgPost(put func(float64, float64, int), cx, scale, lit float64) {
	step := 0.5 / scale
	for y := -1.06; y <= 1.06; y += step {
		w := 0.019 + 0.013*math.Cos(y*math.Pi*0.5)
		if a := math.Abs(y); a > 0.86 {
			w += 0.017 * math.Sin((a-0.86)/0.20*math.Pi)
		}
		for dx := -w; dx <= w; dx += step {
			lum := (0.74 - 0.58*(dx/w) - 0.14*math.Abs(y)) * lit
			put(cx+dx, y, relaxHgBronze+relaxHgLvl(lum, relaxHgBronzeN))
		}
	}
}
