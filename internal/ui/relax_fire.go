package ui

import (
	"math"
	"math/rand"
)

// ── Fogueira ──────────────────────────────────────────────────────────────────
//
// O autômato de fogo do Doom, em subpixels: cada célula puxa o calor da célula
// de baixo, perde um tanto sorteado e desliza de lado. É bobo e é o que existe
// de mais convincente em fogo 2D — a chama sobe, ondula e some sozinha, sem
// nenhuma partícula.
//
// A grade é de subpixel Braille, então a chama tem quatro vezes mais resolução
// vertical do que a linha do terminal, que é justamente onde o fogo precisa.

var relaxFireStops = []string{
	"#000000", "#1F0700", "#571C00", "#8F3200", "#C75200",
	"#E87B08", "#F5A623", "#FBD24B", "#FEF08A", "#FFFBE8",
}

const relaxFireLevels = 24

const (
	relaxFireLog = relaxFireLevels + iota
	relaxFireLogLit
	relaxFireSpark
)

var relaxFireRamp = func() []relaxColor {
	out := make([]relaxColor, relaxFireSpark+1)
	copy(out, relaxRamp(relaxFireStops, relaxFireLevels))
	out[relaxFireLog] = "#2B1C12"
	out[relaxFireLogLit] = "#6B3B1C"
	out[relaxFireSpark] = "#FFD98A"
	return out
}()

// relaxFireSparkP é uma brasa solta. Ela não é só um ponto que sobe: enquanto
// está quente o ar quente a empurra pra cima, e conforme esfria a gravidade
// ganha e ela cai girando. A cor sai da própria temperatura, na mesma rampa da
// chama — branco, laranja, vermelho, nada.
type relaxFireSparkP struct {
	x, y   float64
	vx, vy float64
	px, py float64 // posição anterior, pro rastro
	heat   float64
	spin   float64
}

type relaxFireState struct {
	inited bool
	tick   int
	w, h   int // em subpixels
	heat   []uint8
	sparks []relaxFireSparkP
	// A base respira: a fogueira não queima sempre com a mesma força.
	breath float64
	logs   [][4]float64 // toras: x0,y0,x1,y1 em subpixels
}

func relaxFireBuild(st *relaxFireState, w, h int) {
	tick := st.tick
	*st = relaxFireState{inited: true, tick: tick, w: w, h: h, heat: make([]uint8, w*h)}
	// Três toras cruzadas na base, com inclinações sorteadas.
	base := float64(h) - 5
	cx := float64(w) / 2
	for i, n := 0, 3; i < n; i++ {
		ang := -0.55 + float64(i)*0.55 + (rand.Float64()-0.5)*0.25
		half := float64(w) * (0.16 + rand.Float64()*0.07)
		dy := float64(i) * 1.6
		st.logs = append(st.logs, [4]float64{
			cx - half*math.Cos(ang), base + half*math.Sin(ang) - dy,
			cx + half*math.Cos(ang), base - half*math.Sin(ang) - dy,
		})
	}
}

func stepRelaxFire(st *relaxFireState) {
	if !st.inited {
		relaxFireBuild(st, 160, 88)
	}
	st.tick++
	w, h := st.w, st.h
	t := float64(st.tick) * 0.1
	// Sopro lento mais um tremor curto: a labareda cresce e recua.
	// O piso importa: abaixo dele a base não chega nem ao limiar de soltar
	// brasa, e a fogueira apagava sozinha em meio minuto.
	st.breath = 0.84 + 0.14*math.Sin(t*0.23) + 0.06*math.Sin(t*1.7)

	// Fonte: a base pega fogo com força variável ao longo da largura, mais
	// quente no meio das toras.
	src := h - 1
	for x := 0; x < w; x++ {
		f := 1 - math.Abs(float64(x)-float64(w)/2)/(float64(w)*0.26)
		if f <= 0 {
			st.heat[src*w+x] = 0
			continue
		}
		v := st.breath * f * (0.86 + 0.14*relaxNoise(float64(x)*0.06+t*0.9, 17))
		st.heat[src*w+x] = uint8(clamp01(v) * 255)
	}

	// Propagação: cada célula puxa da de baixo, perde calor e desvia de lado.
	for y := 0; y < h-1; y++ {
		for x := 0; x < w; x++ {
			below := st.heat[(y+1)*w+x]
			if below == 0 {
				st.heat[y*w+x] = 0
				continue
			}
			r := relaxHash(x, y, st.tick)
			// O decaimento define a altura da chama: 255 de calor perdendo ~3
			// por linha sobe umas 80 linhas de subpixel.
			decay := uint8(r % 7)
			if below <= decay {
				st.heat[y*w+x] = 0
				continue
			}
			// O deslocamento lateral é o que faz a chama serpentear em vez de
			// subir reta.
			dx := (r/7)%3 - 1
			nx := x + dx
			if nx < 0 || nx >= w {
				nx = x
			}
			st.heat[y*w+nx] = below - decay
		}
	}

	// Brasas: saem de onde a chama está mais viva, quanto mais forte o sopro
	// mais delas. Um estouro ocasional solta um punhado de uma vez.
	burst := 1
	if rand.Intn(70) == 0 {
		burst = 5 + rand.Intn(6) // uma tora estala
	}
	for k := 0; k < burst; k++ {
		if burst == 1 && rand.Float64() > st.breath*0.75 {
			break
		}
		x := w/2 + rand.Intn(w/2) - w/4
		if x < 0 || x >= w {
			continue
		}
		for y := 0; y < h; y++ {
			if st.heat[y*w+x] > 72 {
				st.sparks = append(st.sparks, relaxFireSparkP{
					x: float64(x), y: float64(y),
					px: float64(x), py: float64(y),
					vx:   (rand.Float64() - 0.5) * 1.4,
					vy:   -0.9 - rand.Float64()*1.1,
					heat: 0.80 + rand.Float64()*0.20,
					spin: rand.Float64() * 6.28,
				})
				break
			}
		}
	}

	keep := st.sparks[:0]
	for _, p := range st.sparks {
		p.px, p.py = p.x, p.y
		// Vento: ruído contínuo, então a brasa passeia em vez de tremer.
		p.vx += relaxNoise(p.y*0.035+t*0.7, 7) * 0.11
		p.vx *= 0.985
		// Empuxo enquanto quente; esfriando, a gravidade assume e ela cai.
		p.vy += 0.075 - 0.13*p.heat
		p.x += p.vx + 0.25*math.Sin(p.spin)
		p.y += p.vy
		p.spin += 0.22
		// Esfria mais devagar quando ainda está subindo depressa.
		p.heat -= 0.016 + 0.014*clamp01(p.vy+0.4)
		if p.heat > 0.04 && p.y > 0 && p.y < float64(h) && p.x > -2 && p.x < float64(w)+2 {
			keep = append(keep, p)
		}
	}
	st.sparks = keep
}

func relaxFireFrames(st *relaxFireState, width, height int, fade float64) ([]string, string) {
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	if !st.inited || st.w != w*2 || st.h != h*4 {
		relaxFireBuild(st, w*2, h*4)
		for i := 0; i < 60; i++ {
			stepRelaxFire(st) // acende antes de aparecer, sem partir do vazio
		}
	}
	b := newRelaxBraille(w, h)

	for i, v := range st.heat {
		if v < 8 {
			continue
		}
		x, y := i%st.w, i/st.w
		lum := float64(v) / 255
		if lum <= relaxHalftone(x, y)*0.22 {
			continue
		}
		b.set(x, y, minInt(int(lum*float64(relaxFireLevels-1)+0.5), relaxFireLevels-1))
	}

	// Brasa desenhada como rastro curto: um ponto sozinho não dá a ideia de
	// que está viajando.
	for _, p := range st.sparks {
		lvl := relaxFireLevels - 1 - int((1-p.heat)*float64(relaxFireLevels-8))
		b.line(p.px, p.py, p.x, p.y, minInt(maxInt(lvl, 6), relaxFireLevels-1))
		if p.heat > 0.7 {
			b.set(int(p.x), int(p.y), relaxFireSpark)
		}
	}

	// Toras por último: ficam nas frestas da chama, como brasa entre o fogo.
	for _, l := range st.logs {
		n := int(math.Hypot(l[2]-l[0], l[3]-l[1]))
		for i := 0; i <= n; i++ {
			f := float64(i) / float64(maxInt(1, n))
			x, y := lerp(l[0], l[2], f), lerp(l[1], l[3], f)
			// A ponta encostada no fogo fica em brasa.
			lvl := relaxFireLog
			if math.Abs(x-float64(st.w)/2) < float64(st.w)*0.13 {
				lvl = relaxFireLogLit
			}
			for dy := 0; dy < 3; dy++ {
				b.set(int(x), int(y)+dy, lvl)
			}
		}
	}

	status := "o fogo respira"
	switch {
	case st.breath > 0.95:
		status = "a labareda cresce"
	case len(st.sparks) > 22:
		status = "as brasas sobem"
	case st.breath < 0.76:
		status = "o fogo abaixa"
	}
	return b.lines(relaxStyles(relaxFireRamp, fade)), StyleMuted.Render(status)
}
