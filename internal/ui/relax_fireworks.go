package ui

import (
	"math"
	"math/rand"
)

// ── Fogos de artifício ────────────────────────────────────────────────────────
//
// Foguetes sobem de trás do morro, estouram e caem em cima de um lago que
// devolve tudo tremido. O tempo entre um e outro é parte da cena: se estourasse
// sem parar não teria estouro nenhum, só um fundo aceso.
//
// A conta toda é feita em unidades de LARGURA (x e y na mesma escala), não em
// fração de tela. É o que garante que a explosão saia redonda: em fração de
// tela ela viraria uma elipse achatada, porque o palco não é quadrado.
//
// Cada formato é uma curva fechada amostrada, e a velocidade de cada faísca é o
// ponto da curva. Coração, estrela e anel saem do mesmo laço — o que muda é a
// curva, não o motor.

const (
	relaxFwFamN = 6 // degraus por cor
	relaxFwFams = 6 // quantas cores
)

const (
	relaxFwFlash = relaxFwFamN * relaxFwFams
	relaxFwGnd   = relaxFwFlash + iota
	relaxFwWater
	relaxFwStar
	relaxFwWin
	relaxFwTrail
)

var relaxFwFamilies = [relaxFwFams][]string{
	{"#2A1A02", "#6A4A08", "#B8860C", "#E8B830", "#FFDE78", "#FFF6C8"}, // ouro
	{"#2C0606", "#6E1210", "#B82820", "#E85040", "#FF8C7A", "#FFD4C8"}, // vermelho
	{"#062A10", "#0E6A24", "#1CB03C", "#40E060", "#86F49A", "#D8FFDC"}, // verde
	{"#041A34", "#0C3C78", "#1A70C8", "#40A0EC", "#86CCFA", "#D8EEFF"}, // azul
	{"#1E0630", "#4A126A", "#8024B0", "#B052E0", "#D294F4", "#F0DAFF"}, // violeta
	{"#1C1E24", "#4A4E58", "#808690", "#B4BAC4", "#DEE2EA", "#FFFFFF"}, // prata
}

var relaxFwRamp = func() []relaxColor {
	out := make([]relaxColor, relaxFwTrail+1)
	for i, fam := range relaxFwFamilies {
		copy(out[i*relaxFwFamN:], relaxRamp(fam, relaxFwFamN))
	}
	out[relaxFwFlash] = "#FFFFFF"
	out[relaxFwGnd] = "#080C12"
	out[relaxFwWater] = "#0C1626"
	out[relaxFwStar] = "#8894B0"
	out[relaxFwWin] = "#D8A850"
	out[relaxFwTrail] = "#7A6440"
	return out
}()

func relaxFwLvl(fam int, v float64) int {
	return fam*relaxFwFamN + minInt(maxInt(int(v*float64(relaxFwFamN)), 0), relaxFwFamN-1)
}

const (
	fwShell = iota
	fwRing
	fwWillow
	fwPalm
	fwHeart
	fwStar
	fwShapes
)

type relaxFwPart struct {
	x, y, vx, vy float64
	heat, cool   float64
	drag, grav   float64
	fam          int
	crack        bool // estala de novo quando já está apagando
}

type relaxFwRocket struct {
	x, y, vx, vy float64
	fuse         int
	fam, shape   int
}

type relaxFireworksState struct {
	inited bool
	tick   int

	rockets []relaxFwRocket
	parts   []relaxFwPart
	stars   []relaxSkyPt
	wins    []relaxSkyPt
	skyline []float64

	next   int
	flash  float64 // clarão do estouro, ilumina a água
	last   int     // formato do último estouro, pro status
	aspect float64 // altura/largura do palco, pra saber onde é o horizonte
}

// relaxFwShapePt devolve o ponto (já normalizado) da curva do formato em f∈[0,1).
func relaxFwShapePt(shape int, f float64) (float64, float64) {
	switch shape {
	case fwHeart:
		a := f * 2 * math.Pi
		s := math.Sin(a)
		x := 16 * s * s * s
		y := 13*math.Cos(a) - 5*math.Cos(2*a) - 2*math.Cos(3*a) - math.Cos(4*a)
		return x / 17, -y / 17 // y da tela cresce pra baixo
	case fwStar:
		// Dez vértices alternando raio: a faísca anda pela borda da estrela.
		u := f * 10
		k := int(u)
		g := u - float64(k)
		pt := func(i int) (float64, float64) {
			r := 1.0
			if i%2 == 1 {
				r = 0.44
			}
			a := float64(i)*math.Pi/5 - math.Pi/2
			return r * math.Cos(a), r * math.Sin(a)
		}
		x0, y0 := pt(k % 10)
		x1, y1 := pt((k + 1) % 10)
		return lerp(x0, x1, g), lerp(y0, y1, g)
	default:
		a := f * 2 * math.Pi
		return math.Cos(a), math.Sin(a)
	}
}

func relaxFwBurst(st *relaxFireworksState, r relaxFwRocket) {
	st.flash = 1
	st.last = r.shape
	add := func(vx, vy, heat, cool, drag, grav float64, fam int, crack bool) {
		st.parts = append(st.parts, relaxFwPart{
			x: r.x, y: r.y, vx: vx + r.vx*0.35, vy: vy + r.vy*0.35,
			heat: heat, cool: cool, drag: drag, grav: grav, fam: fam, crack: crack,
		})
	}
	switch r.shape {
	case fwWillow:
		// Salgueiro: sobe pouco, esfria devagar e a gravidade manda. É o único
		// que ainda está caindo quando os outros já acabaram.
		for i, n := 0, 130+rand.Intn(60); i < n; i++ {
			a := rand.Float64() * 2 * math.Pi
			v := 0.0035 + rand.Float64()*0.0045
			add(math.Cos(a)*v, math.Sin(a)*v*0.85-0.001, 0.85+rand.Float64()*0.15,
				0.0035+rand.Float64()*0.003, 0.972, 0.00020, r.fam, false)
		}
	case fwPalm:
		arms := 6 + rand.Intn(3)
		off := rand.Float64() * 2 * math.Pi
		for k := 0; k < arms; k++ {
			a := off + float64(k)*2*math.Pi/float64(arms)
			for i, n := 0, 26; i < n; i++ {
				v := (0.0035 + 0.0075*float64(i)/float64(n)) * (0.9 + rand.Float64()*0.2)
				sa, ca := math.Sincos(a + (rand.Float64()-0.5)*0.10)
				add(ca*v, sa*v, 0.9, 0.008+rand.Float64()*0.005, 0.965, 0.00013, r.fam, i > n-6)
			}
		}
	case fwRing:
		// Anel visto de viés: o mesmo círculo, achatado, e por isso ele lê como
		// um aro no ar em vez de um disco.
		for i, n := 0, 190; i < n; i++ {
			a := float64(i)/float64(n)*2*math.Pi + rand.Float64()*0.02
			v := 0.0105 * (0.94 + rand.Float64()*0.12)
			sa, ca := math.Sincos(a)
			add(ca*v, sa*v*0.34, 0.92, 0.010+rand.Float64()*0.004, 0.960, 0.00011, r.fam, false)
		}
	case fwHeart, fwStar:
		n := 230
		for i := 0; i < n; i++ {
			hx, hy := relaxFwShapePt(r.shape, float64(i)/float64(n))
			v := 0.0115 * (0.95 + rand.Float64()*0.10)
			add(hx*v, hy*v, 0.95, 0.0085+rand.Float64()*0.003, 0.962, 0.00012, r.fam, false)
		}
	default: // fwShell: a casca clássica, com um miolo de outra cor
		in := (r.fam + 2 + rand.Intn(3)) % relaxFwFams
		for i, n := 0, 240+rand.Intn(120); i < n; i++ {
			a := rand.Float64() * 2 * math.Pi
			v := (0.0090 + rand.Float64()*0.0030) * (0.86 + 0.14*rand.Float64())
			sa, ca := math.Sincos(a)
			add(ca*v, sa*v, 0.92+rand.Float64()*0.08, 0.008+rand.Float64()*0.005,
				0.962, 0.00013, r.fam, rand.Intn(6) == 0)
		}
		for i, n := 0, 110; i < n; i++ {
			a := rand.Float64() * 2 * math.Pi
			v := (0.0034 + rand.Float64()*0.0022)
			sa, ca := math.Sincos(a)
			add(ca*v, sa*v, 0.95, 0.011+rand.Float64()*0.006, 0.955, 0.00013, in, false)
		}
	}
}

func stepRelaxFireworks(st *relaxFireworksState) {
	if !st.inited {
		st.inited = true
		st.next, st.aspect = 8, 0.52
		for i, n := 0, 60+rand.Intn(40); i < n; i++ {
			st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.62})
		}
		for i, n := 0, 12+rand.Intn(10); i < n; i++ {
			st.wins = append(st.wins, relaxSkyPt{x: rand.Float64(), y: rand.Float64()})
		}
		for i, n := 0, 9+rand.Intn(6); i < n; i++ {
			st.skyline = append(st.skyline, rand.Float64())
		}
	}
	st.tick++
	st.flash *= 0.80

	if st.next--; st.next <= 0 {
		for i, n := 0, 1+rand.Intn(2); i < n; i++ {
			st.rockets = append(st.rockets, relaxFwRocket{
				x: 0.14 + rand.Float64()*0.72, y: 0.52,
				vx:   (rand.Float64() - 0.5) * 0.0022,
				vy:   -(0.0125 + rand.Float64()*0.0060),
				fuse: 16 + rand.Intn(12), fam: rand.Intn(relaxFwFams),
				shape: rand.Intn(fwShapes),
			})
		}
		// A pausa é a cena tanto quanto o estouro.
		st.next = 26 + rand.Intn(60)
	}

	kr := st.rockets[:0]
	for _, r := range st.rockets {
		r.x += r.vx
		r.y += r.vy
		r.vy += 0.00026
		if r.fuse--; r.fuse > 0 && r.vy < -0.0015 {
			kr = append(kr, r)
			continue
		}
		relaxFwBurst(st, r)
	}
	st.rockets = kr

	kp := st.parts[:0]
	for _, q := range st.parts {
		q.x += q.vx
		q.y += q.vy
		q.vx *= q.drag
		q.vy = q.vy*q.drag + q.grav
		q.heat -= q.cool
		// Estalo: a faísca que "pipoca" acende de novo já no fim. É o barulho
		// que a gente não tem, virado em luz.
		if q.crack && q.heat < 0.42 && rand.Intn(9) == 0 {
			q.heat = math.Min(0.75, q.heat+0.35)
		}
		if q.heat > 0.02 {
			kp = append(kp, q)
		}
	}
	st.parts = kp
}

func relaxFireworksFrames(st *relaxFireworksState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxFireworks(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	st.aspect = float64(h*4) / float64(w*2)
	b := newRelaxBrailleVote(w, h)
	relaxFireworksDraw(st, b, w, h)

	status := "silêncio entre um e outro"
	switch {
	case st.flash > 0.5:
		switch st.last {
		case fwHeart:
			status = "esse abriu em coração"
		case fwStar:
			status = "esse abriu em estrela"
		case fwRing:
			status = "um anel, de lado"
		case fwWillow:
			status = "salgueiro: vai cair devagar"
		default:
			status = "essa foi grande"
		}
	case len(st.rockets) > 0:
		status = "sobe um foguete"
	case len(st.parts) > 40:
		status = "as faíscas caem no lago"
	}
	return b.lines(relaxStyles(relaxFwRamp, fade)), StyleMuted.Render(status)
}

func relaxFireworksDraw(st *relaxFireworksState, b *relaxBraille, w, h int) {
	sw, sh := w*2, h*4
	fw, fh := float64(sw), float64(sh)
	// Tudo em unidades de largura: y de tela = y*fw. Assim círculo é círculo.
	U := func(v float64) float64 { return v * fw }
	hz := fh * 0.76 // linha d'água
	t := float64(st.tick) * 0.1

	// reflexo: a mesma faísca espelhada na água, tremida e mais fraca. É ela
	// que faz a metade de baixo da tela existir.
	refl := func(x, y float64, lvl int, heat float64) {
		ry := 2*hz - y
		if ry <= hz || ry >= fh {
			return
		}
		d := (ry - hz) / math.Max(1, fh-hz)
		rx := x + math.Sin(ry*0.55+t*2.1+x*0.10)*(0.8+2.6*d)
		if heat*(1-d*0.5) <= relaxHalftone(int(rx), int(ry))*1.15 {
			return
		}
		b.set(int(rx), int(ry), lvl)
	}

	// ── Faíscas ──
	for _, q := range st.parts {
		x, y := U(q.x), U(q.y)
		lvl := relaxFwLvl(q.fam, q.heat)
		if q.heat > 0.80 {
			lvl = relaxFwFlash
		}
		if y < hz {
			// Risco no lugar do ponto enquanto ela corre: é o rastro que faz
			// a explosão parecer explosão e não confete parado.
			if sp := math.Hypot(q.vx, q.vy) * fw; sp > 1.6 {
				b.line(x-q.vx*fw*1.6, y-q.vy*fw*1.6, x, y, relaxFwLvl(q.fam, q.heat*0.6))
			}
			b.set(int(x), int(y), lvl)
		}
		refl(x, y, relaxFwLvl(q.fam, q.heat*0.55), q.heat)
	}

	// ── Foguetes ── ponto branco e o rastro de pólvora atrás.
	for _, r := range st.rockets {
		x, y := U(r.x), U(r.y)
		for k := 0.0; k < 9; k++ {
			ty := y - r.vy*fw*k*0.9
			if relaxHalftone(int(x), int(ty)) < 1-k/9 {
				b.set(int(x-r.vx*fw*k), int(ty), relaxFwTrail)
			}
		}
		b.set(int(x), int(y), relaxFwFlash)
		b.set(int(x), int(y)-1, relaxFwFlash)
		refl(x, y, relaxFwTrail, 0.8)
	}

	// ── Água ── faixas horizontais rasas; o estouro clareia ela inteira.
	for y := int(hz); y < sh; y++ {
		d := float64(y-int(hz)) / math.Max(1, fh-hz)
		for x := 0; x < sw; x++ {
			v := 0.26 + 0.22*math.Sin(float64(y)*0.85+t*1.1+math.Sin(float64(x)*0.045)*1.5) + 0.30*st.flash
			if relaxHalftone(x, y) > v*(1-0.4*d) {
				continue
			}
			b.set(x, y, relaxFwWater)
		}
	}

	// ── Cidade na margem ── silhueta baixa com uma ou outra janela acesa.
	base := int(hz)
	for _, bx := range st.skyline {
		x := int(bx * fw)
		bw := 2 + relaxHash(x, 3)%5
		bh := 3 + relaxHash(x, 5)%10
		for dx := -bw; dx <= bw; dx++ {
			for y := base - bh; y < base; y++ {
				b.set(x+dx, y, relaxFwGnd)
			}
		}
	}
	for i, p := range st.wins {
		x := int(p.x * fw)
		y := base - 1 - int(p.y*6)
		if math.Sin(t*0.3+float64(i)*2.3) > -0.4 {
			b.set(x, y, relaxFwWin)
			b.paint(x/2, y/4, relaxFwWin)
		}
	}
	for y := base - 2; y < int(hz); y++ {
		for x := 0; x < sw; x++ {
			b.set(x, y, relaxFwGnd)
		}
	}

	// ── Estrelas ── e o céu fica preto mesmo: aqui o preto é o palco.
	for i, p := range st.stars {
		x, y := int(p.x*fw), int(p.y*fh)
		if relaxHalftone(x, y) < 0.22+0.14*math.Sin(t*0.4+float64(i)) {
			b.set(x, y, relaxFwStar)
		}
	}
}
