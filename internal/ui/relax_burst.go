package ui

import (
	"math"
	"math/rand"
)

// ── Supernova ─────────────────────────────────────────────────────────────────
//
// O ciclo é a cena: a matéria chega em bolinhas, cada uma caindo em espiral pro
// centro. Quanto mais tempo passa, mais bolinhas vêm e mais rápido elas caem,
// até a estrela estourar. Sobra uma estrela azul iluminando, e ela apaga
// devagar até não haver mais nada. Aí recomeça.
//
// As bolinhas são partículas de verdade, não uma espiral desenhada. É isso que
// faz o estouro parecer consequência e não interrupção: dá pra ver a estrela
// engolindo o que chega, e chegando mais.
//
// Quatro famílias de cor (rosa, azul, dourado, violeta) convivem porque cada
// coisa que cai é de um elemento diferente. Na explosão isso acaba: o que sai é
// só azul, azul claro e branco — a estrela de nêutrons é o que ficou mais quente.

var relaxNovaFamilies = [4][]string{
	{"#22071A", "#5E1240", "#A02476", "#D854A4", "#F79AD0", "#FFE0F2"}, // rosa
	{"#04102A", "#0E3070", "#2168BC", "#54A8EC", "#9AD6FA", "#E4F6FF"}, // azul
	{"#241704", "#6B420A", "#B4841A", "#E8C048", "#F8E68E", "#FFFAD8"}, // dourado
	{"#160828", "#3E1466", "#7430AE", "#A96EE0", "#D4AEF4", "#F2E2FF"}, // violeta
}

const (
	relaxNovaFamN  = 8 // degraus por família
	relaxNovaBlueF = 1 // índice da família azul
)

const (
	relaxNovaFlash = 4 * relaxNovaFamN
	relaxNovaDark  = relaxNovaFlash + 1
)

var relaxNovaRamp = func() []relaxColor {
	out := make([]relaxColor, relaxNovaDark+1)
	for i, fam := range relaxNovaFamilies {
		copy(out[i*relaxNovaFamN:], relaxRamp(fam, relaxNovaFamN))
	}
	out[relaxNovaFlash] = "#FFFFFF"
	out[relaxNovaDark] = "#141726"
	return out
}()

func relaxNovaLvl(fam int, v float64) int {
	return fam*relaxNovaFamN + minInt(maxInt(int(v*float64(relaxNovaFamN)), 0), relaxNovaFamN-1)
}

// relaxNovaBlue é o nível dos detritos: só azul, e branco no que está mais
// quente. Depois do estouro não sobra mais nenhuma outra cor.
func relaxNovaBlue(heat float64) int {
	if heat > 0.86 {
		return relaxNovaFlash
	}
	return relaxNovaLvl(relaxNovaBlueF, heat)
}

type relaxNovaPhase int

const (
	novaIdle relaxNovaPhase = iota
	novaSpiral
	novaFlash
	novaStar // a estrela azul iluminando
	novaFade
)

type relaxNovaPart struct {
	x, y, vx, vy float64
	heat, cool   float64
}

// relaxNovaFall é uma bolinha caindo: raio, ângulo e o momento angular que faz
// ela girar mais rápido conforme aperta. pr/pa é onde ela estava, pro rastro.
type relaxNovaFall struct {
	r, a, L, vr float64
	pr, pa      float64
	fam         int
}

type relaxBurstState struct {
	inited bool
	tick   int
	phase  relaxNovaPhase
	t      int
	dur    int

	energy float64 // 0–1 durante a queda
	flash  float64
	star   float64 // brilho da estrela azul depois do estouro
	shock  float64
	parts  []relaxNovaPart
	fall   []relaxNovaFall
}

func relaxNovaDur(p relaxNovaPhase) int {
	switch p {
	case novaSpiral:
		return 135
	case novaFlash:
		return 5
	case novaStar:
		return 85
	case novaFade:
		return 110
	default:
		return 20
	}
}

func stepRelaxBurst(st *relaxBurstState) {
	if !st.inited {
		st.inited = true
		st.dur = relaxNovaDur(novaIdle)
		st.shock = -1
	}
	st.tick++
	st.t++
	p := float64(st.t) / float64(maxInt(1, st.dur))
	st.flash *= 0.66

	switch st.phase {
	case novaIdle:
		st.energy, st.star = 0, 0
	case novaSpiral:
		st.energy = easeInOut(p)
		// Começa com uma bolinha ou outra e termina com um enxame: é a
		// aceleração da chegada que diz que a estrela não vai aguentar.
		if st.tick%2 == 0 {
			for i, n := 0, 1+rand.Intn(1+int(5*st.energy)); i < n; i++ {
				r := 0.62 + rand.Float64()*0.48
				st.fall = append(st.fall, relaxNovaFall{
					r: r, a: rand.Float64() * 2 * math.Pi,
					L: (0.048 + rand.Float64()*0.038) * (r + 0.08),
					// Velocidade de queda bem espalhada de propósito: com
					// todas iguais elas viram anéis concêntricos em vez de
					// bolinhas soltas.
					vr:  0.006 + rand.Float64()*0.022,
					fam: rand.Intn(4),
				})
			}
		}
	case novaFlash:
		if st.t == 1 {
			st.flash, st.shock, st.star = 1, 0, 1
			st.fall = st.fall[:0] // tudo que estava caindo virou explosão
			// Casca de velocidade: sem ela o resultado é um disco cheio.
			for i, n := 0, 2400+rand.Intn(1200); i < n; i++ {
				a := rand.Float64() * 2 * math.Pi
				v := (0.007 + rand.Float64()*0.032) * (0.55 + 0.45*rand.Float64())
				st.parts = append(st.parts, relaxNovaPart{
					vx: math.Cos(a) * v, vy: math.Sin(a) * v * 0.62,
					heat: 0.72 + rand.Float64()*0.28,
					cool: 0.005 + rand.Float64()*0.009,
				})
			}
		}
	case novaStar:
		st.star = 1
	case novaFade:
		// A estrela azul apaga devagar; é o rescaldo mais longo que a explosão.
		st.star = 1 - easeInOut(p)
	}
	if st.t >= st.dur && !(st.phase == novaFade && len(st.parts) > 0) {
		st.phase = (st.phase + 1) % 5
		st.t = 0
		st.dur = relaxNovaDur(st.phase)
	}

	// As bolinhas espiralam: o ângulo anda mais rápido perto do centro, e o
	// raio despenca mais rápido ainda. É o funil, não uma reta.
	kf := st.fall[:0]
	for _, q := range st.fall {
		q.pr, q.pa = q.r, q.a
		q.a += q.L / (q.r + 0.08)
		q.r -= q.vr * (1 + 0.9*(1-q.r))
		if q.r > 0.05 {
			kf = append(kf, q)
		}
	}
	st.fall = kf

	if st.shock >= 0 {
		if st.shock += 0.032; st.shock > 1.6 {
			st.shock = -1
		}
	}
	kp := st.parts[:0]
	for _, q := range st.parts {
		q.x += q.vx
		q.y += q.vy
		q.vx *= 0.983
		q.vy *= 0.983
		if q.heat -= q.cool; q.heat > 0.02 {
			kp = append(kp, q)
		}
	}
	st.parts = kp
}

func relaxBurstFrames(st *relaxBurstState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxBurst(st)
	}
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBrailleVote(w, h)
	sw, sh := float64(w*2), float64(h*4)
	cx, cy := sw/2, sh/2
	rad := math.Min(sw/2, sh)
	t := float64(st.tick) * 0.1

	put := func(fx, fy, v float64, fam int) {
		x, y := int(cx+fx*rad), int(cy+fy*rad*0.62)
		if v <= relaxHalftone(x, y)*0.42 {
			return
		}
		b.set(x, y, relaxNovaLvl(fam, v))
		b.paint(x/2, y/4, relaxNovaLvl(fam, v))
	}

	// O núcleo vem primeiro: relaxBraille não sobrescreve, e a estrela tem de
	// ficar na frente de tudo — inclusive dos próprios detritos.
	// ── Núcleo ── carregando, depois a estrela azul com suas pontas.
	core := 0.045 + 0.11*st.energy + 0.55*st.flash
	glow := st.energy*0.8 + st.star + st.flash*2
	if glow > 0.02 {
		pulse := 1 + 0.10*math.Sin(t*(5+16*st.energy))
		rx := core * pulse * rad
		for dy := -int(rx*0.62) - 2; dy <= int(rx*0.62)+2; dy++ {
			for dx := -int(rx) - 2; dx <= int(rx)+2; dx++ {
				d := math.Hypot(float64(dx)/rx, float64(dy)/(rx*0.62))
				if d > 1.5 {
					continue
				}
				v := glow * math.Exp(-1.6*d*d)
				x, y := int(cx)+dx, int(cy)+dy
				if v <= relaxHalftone(x, y)*0.4 {
					continue
				}
				lvl := relaxNovaLvl(relaxNovaBlueF, v)
				if st.flash > 0.45 {
					lvl = relaxNovaFlash
				}
				b.set(x, y, lvl)
				b.paint(x/2, y/4, lvl)
			}
		}
		// Pontas de luz: só depois do estouro, quando sobrou a estrela azul.
		if st.star > 0.05 {
			for _, d := range [4][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				for k := 0.0; k < 0.55*st.star; k += 0.004 {
					put(d[0]*k, d[1]*k*1.6, st.star*(1-k/(0.55*st.star))*0.85, relaxNovaBlueF)
				}
			}
		}
	}

	// ── Detritos da explosão ── azul e branco, nada mais.
	for _, q := range st.parts {
		lvl := relaxNovaBlue(q.heat)
		x, y := cx+q.x*rad, cy+q.y*rad*0.62
		if q.heat > 0.55 {
			b.line(x-q.vx*rad*1.5, y-q.vy*rad*0.9, x, y, lvl)
			continue
		}
		b.set(int(x), int(y), lvl)
	}

	// ── Bolinhas caindo ── mais claras conforme se aproximam do centro.
	for _, q := range st.fall {
		v := 0.42 + 0.58*clamp01(1-q.r)
		lvl := relaxNovaLvl(q.fam, v)
		sa, ca := math.Sincos(q.a)
		x, y := cx+ca*q.r*rad, cy+sa*q.r*rad*0.62
		sp, cp := math.Sincos(q.pa)
		px, py := cx+cp*q.pr*rad, cy+sp*q.pr*rad*0.62
		// Rastro só quando a bolinha andou mais que o próprio corpo — perto do
		// centro, onde ela corre. Longe, rastro em tudo vira teia de linhas
		// em vez de bolinhas soltas, que é o oposto da leitura.
		if q.pr > 0 && math.Hypot(x-px, y-py) > 3.5 {
			b.line(px, py, x, y, relaxNovaLvl(q.fam, v*0.45))
		}
		// Bolinha de 2×2 subpixels: um ponto só some no meio da tela.
		ix, iy := int(x), int(y)
		b.set(ix, iy, lvl)
		b.set(ix+1, iy, lvl)
		b.set(ix, iy+1, lvl)
		b.set(ix+1, iy+1, lvl)
	}

	// ── Onda de choque ──
	if st.shock >= 0 {
		amp := clamp01(1 - st.shock/1.6)
		for a := 0.0; a < 6.29; a += 0.004 {
			sa, ca := math.Sincos(a)
			put(ca*st.shock, sa*st.shock, amp*0.9, relaxNovaBlueF)
		}
	}

	status := "silêncio antes"
	switch {
	case st.flash > 0.3:
		status = "AGORA"
	case st.phase == novaSpiral:
		status = "a matéria cai em espiral"
	case st.phase == novaStar:
		status = "uma estrela azul no lugar dela"
	case st.phase == novaFade && st.star > 0.05:
		status = "e vai apagando"
	case len(st.parts) > 0:
		status = "os restos se espalham"
	}
	return b.lines(relaxStyles(relaxNovaRamp, fade)), StyleMuted.Render(status)
}
