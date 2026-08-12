package ui

import (
	"math"
	"math/rand"
)

// ── Nebulosa ──────────────────────────────────────────────────────────────────
//
// Duas nuvens de emissão sobrepostas: hidrogênio no rosa e oxigênio no azul —
// que é de onde vêm as cores das fotos de verdade, não de escolha estética. Uma
// terceira camada é poeira, e ela subtrai: as faixas escuras são o que dá
// profundidade à nuvem em vez de um borrão colorido.
//
// Cada camada é soma de ondas planas separáveis em x e y, então o laço interno
// não chama trigonometria nenhuma — são dezenas de milhares de subpixels.

var (
	relaxNebHaStops  = []string{"#160A14", "#3C1030", "#78204E", "#B23A66", "#DE6E85", "#F5AEB2", "#FFE6E2"}
	relaxNebO3Stops  = []string{"#07131C", "#0E3040", "#155C6B", "#2A93A0", "#5FC6C8", "#A8E8E4", "#E6FBF8"}
	relaxNebStarStop = []string{"#3A4560", "#8FA2C4", "#FFFFFF"}
)

const relaxNebLevels = 11

const (
	relaxNebHa   = 0
	relaxNebO3   = relaxNebLevels
	relaxNebStar = 2 * relaxNebLevels
)

var relaxNebRamp = func() []relaxColor {
	out := make([]relaxColor, relaxNebStar+3)
	copy(out[relaxNebHa:], relaxRamp(relaxNebHaStops, relaxNebLevels))
	copy(out[relaxNebO3:], relaxRamp(relaxNebO3Stops, relaxNebLevels))
	copy(out[relaxNebStar:], relaxRamp(relaxNebStarStop, 3))
	return out
}()

type relaxNebulaState struct {
	inited bool
	tick   int
	// Fases sorteadas por cena: a nuvem nunca é a mesma duas vezes.
	ph    [8]float64
	tiltA float64
	env   [3]float64 // harmônicas do contorno
	stars []relaxSkyPt
	young []relaxSkyPt // estrelas jovens embutidas, com brilho próprio
	globs []relaxSkyPt // glóbulos de Bok: nós escuros e redondos de poeira

	// Campo estático: onde as estrelas jovens ionizam o gás (positivo) e onde
	// os glóbulos o escondem (negativo). Não muda com o tempo, então é
	// calculado uma vez por tamanho em vez de por frame.
	w, h int
	mod  []float32
	envf []float32 // envelope já com o contorno irregular
}

func relaxNebulaInit(st *relaxNebulaState) {
	st.inited = true
	for i := range st.ph {
		st.ph[i] = rand.Float64() * 2 * math.Pi
	}
	st.tiltA = (rand.Float64() - 0.5) * 0.9
	st.stars = st.stars[:0]
	for i, n := 0, 150+rand.Intn(80); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64()})
	}
	st.young = st.young[:0]
	for i, n := 0, 6+rand.Intn(6); i < n; i++ {
		st.young = append(st.young, relaxSkyPt{x: 0.25 + rand.Float64()*0.5, y: 0.25 + rand.Float64()*0.5})
	}
	st.globs = st.globs[:0]
	for i, n := 0, 3+rand.Intn(4); i < n; i++ {
		st.globs = append(st.globs, relaxSkyPt{x: 0.2 + rand.Float64()*0.6, y: 0.2 + rand.Float64()*0.6})
	}
	for i := range st.env {
		st.env[i] = rand.Float64() * 2 * math.Pi
	}
	st.w, st.h = 0, 0
}

// relaxNebulaField monta o campo estático de ionização e poeira. A estrela
// jovem não é só um ponto brilhante: ela acende o gás à volta, e é essa auréola
// que faz a nebulosa parecer iluminada por dentro.
func relaxNebulaField(st *relaxNebulaState, sw, sh int) {
	st.w, st.h = sw, sh
	st.mod = make([]float32, sw*sh)

	// O contorno também entra aqui: ele não muda com o tempo, e calcular
	// atan2 mais três senos por subpixel a cada frame custava mais que a
	// nebulosa inteira.
	st.envf = make([]float32, sw*sh)
	ecx, ecy := float64(sw)/2, float64(sh)/2
	rx, ry := float64(sw)*0.44, float64(sh)*0.42
	sa, ca := math.Sin(st.tiltA), math.Cos(st.tiltA)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			dx, dy := float64(x)-ecx, float64(y)-ecy
			ex, ey := (dx*ca+dy*sa)/rx, (-dx*sa+dy*ca)/ry
			d2 := ex*ex + ey*ey
			if d2 > 2.2 {
				continue
			}
			th := math.Atan2(ey, ex)
			wob := 1 + 0.20*math.Sin(2*th+st.env[0]) + 0.13*math.Sin(3*th+st.env[1]) +
				0.07*math.Sin(5*th+st.env[2])
			if v := 1.25 - d2/(wob*wob); v > 0 {
				st.envf[y*sw+x] = float32(math.Min(v, 1))
			}
		}
	}

	stamp := func(p relaxSkyPt, rad, amp float64) {
		cx, cy := p.x*float64(sw-1), p.y*float64(sh-1)
		for y := maxInt(0, int(cy-rad)); y <= minInt(sh-1, int(cy+rad)); y++ {
			for x := maxInt(0, int(cx-rad)); x <= minInt(sw-1, int(cx+rad)); x++ {
				d := math.Hypot(float64(x)-cx, float64(y)-cy) / rad
				if d > 1 {
					continue
				}
				st.mod[y*sw+x] += float32(amp * (1 - d) * (1 - d))
			}
		}
	}
	r := float64(minInt(sw, sh))
	for _, p := range st.young {
		stamp(p, r*0.22, 0.55)
	}
	for _, p := range st.globs {
		stamp(p, r*0.10, -1.6)
	}
}

func stepRelaxNebula(st *relaxNebulaState) {
	if !st.inited {
		relaxNebulaInit(st)
	}
	st.tick++
}

func relaxNebulaFrames(st *relaxNebulaState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxNebula(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBrailleVote(w, h)
	sw, sh := w*2, h*4
	t := float64(st.tick) * 0.1

	// Seis ondas planas: três pro hidrogênio, três pro oxigênio, e duas pra
	// poeira. Deriva lenta — a nuvem se remexe em minutos, não em segundos.
	type plane struct{ sx, cx, sy, cy []float64 }
	mk := func(a, bq, c float64) plane {
		sx, cx, sy, cy := relaxTreePlane(a, bq, c, sw, sh)
		return plane{sx, cx, sy, cy}
	}
	// Ponteiro, não valor: a struct tem quatro slices e copiá-la a cada uma das
	// ~170 mil chamadas por frame custava mais que a conta em si.
	at := func(p *plane, x, y int) float64 { return p.sx[x]*p.cy[y] + p.cx[x]*p.sy[y] }

	ha := [3]plane{
		mk(0.021, 0.038, st.ph[0]+t*0.021),
		mk(0.047, -0.029, st.ph[1]-t*0.014),
		mk(0.093, 0.071, st.ph[2]+t*0.031),
	}
	o3 := [2]plane{
		mk(0.026, -0.031, st.ph[3]-t*0.017),
		mk(0.055, 0.043, st.ph[4]+t*0.011),
	}
	dust := [2]plane{
		mk(0.018, 0.024, st.ph[6]+t*0.009),
		mk(0.061, -0.037, st.ph[7]-t*0.019),
	}

	if st.w != sw || st.h != sh {
		relaxNebulaField(st, sw, sh)
	}

	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			env := float64(st.envf[y*sw+x])
			if env <= 0 {
				continue
			}
			// Ruído em crista: o valor alto fica onde a soma cruza o zero, e
			// isso desenha filamento fino em vez de mancha redonda — é a
			// diferença entre nuvem de verdade e borrão de ruído.
			raw := (at(&ha[0], x, y) + 0.62*at(&ha[1], x, y) + 0.34*at(&ha[2], x, y)) / 1.96
			ridge := 1 - math.Abs(raw)
			fh := ridge*ridge*ridge*2.1 - 0.55
			fo := (at(&o3[0], x, y) + 0.62*at(&o3[1], x, y)) / 1.62
			fd := (at(&dust[0], x, y) + 0.55*at(&dust[1], x, y)) / 1.55

			m := float64(st.mod[y*sw+x])
			gh := clamp01(0.34+0.66*fh+m) * env
			go3 := clamp01(0.30+0.70*fo+m*0.5) * env * 0.85
			// Poeira: onde ela é densa a nuvem apaga quase toda.
			if fd > 0.10 {
				k := 1 - clamp01((fd-0.10)/0.40)*0.96
				gh, go3 = gh*k, go3*k
			}

			lum, ramp := gh, relaxNebHa
			if go3 > gh {
				lum, ramp = go3, relaxNebO3
			}
			if lum <= relaxHalftone(x, y)*0.95 {
				continue
			}
			b.set(x, y, ramp+minInt(int(lum*float64(relaxNebLevels-1)+0.5), relaxNebLevels-1))
		}
	}

	// Estrelas de campo, na frente da nuvem.
	for i, p := range st.stars {
		x, y := int(p.x*float64(sw-1)), int(p.y*float64(sh-1))
		if relaxHalftone(x, y) > 0.36+0.14*math.Sin(t*0.5+float64(i)) {
			continue
		}
		b.set(x, y, relaxNebStar)
	}
	// Estrelas jovens embutidas: são elas que acendem o gás em volta.
	for i, p := range st.young {
		x, y := int(p.x*float64(sw-1)), int(p.y*float64(sh-1))
		g := 0.75 + 0.25*math.Sin(t*0.8+float64(i)*1.7)
		b.set(x, y, relaxNebStar+2)
		if g > 0.85 {
			b.set(x-1, y, relaxNebStar+1)
			b.set(x+1, y, relaxNebStar+1)
			b.set(x, y-1, relaxNebStar+1)
			b.set(x, y+1, relaxNebStar+1)
		}
	}

	return b.lines(relaxStyles(relaxNebRamp, fade)), StyleMuted.Render("gás e poeira, devagar")
}
