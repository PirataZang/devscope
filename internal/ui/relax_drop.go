package ui

import (
	"math"
	"math/rand"
)

// ── Gota na poça ──────────────────────────────────────────────────────────────
//
// Uma gota cai, estoura numa coroa e deixa ondas concêntricas. Depois de alguns
// segundos, outra.
//
// As ondas não são anéis desenhados um a um: cada impacto vira um pacote de
// onda (duas ou três cristas dentro de uma janela que anda pra fora), e a altura
// da água em cada ponto é a SOMA dos pacotes vivos. É por isso que duas gotas
// próximas produzem interferência de verdade — cristas que se somam e se
// cancelam — em vez de dois círculos sobrepostos.
//
// A superfície é lisa: a onda aparece pela cor, e o meio-tom fica só pros
// respingos.

var relaxDropStops = []string{
	"#050A12", "#0A1726", "#12293F", "#1C3E5C", "#2A587C",
	"#3F779E", "#5F9CBE", "#8CC2DA", "#C2E4F0", "#F2FBFF",
}

const relaxDropLevels = 16

const (
	relaxDropSpark = relaxDropLevels + iota
	relaxDropBead
)

var relaxDropRamp = func() []relaxColor {
	out := make([]relaxColor, relaxDropBead+1)
	copy(out, relaxRamp(relaxDropStops, relaxDropLevels))
	out[relaxDropSpark] = "#DCF2FF"
	out[relaxDropBead] = "#B4DCEE"
	return out
}()

// relaxWavePack é o pacote de onda tabelado: janela gaussiana vezes as cristas,
// amostrado em u ∈ [-7, 7]. São dezenas de milhares de avaliações por frame, e
// com math.Exp e math.Sin no laço a cena custava cinco vezes mais.
const relaxWaveN = 512

var relaxWavePack = func() [relaxWaveN]float64 {
	var w [relaxWaveN]float64
	for i := range w {
		u := (float64(i)/(relaxWaveN-1))*14 - 7
		w[i] = math.Exp(-u*u*0.06) * math.Sin(u)
	}
	return w
}()

// relaxRipple é um pacote de onda: a janela caminha pra fora com o tempo e o
// conteúdo dela são duas ou três cristas.
type relaxRipple struct {
	x, y float64 // impacto, em normalizado 0–1
	r    float64 // raio da frente
	amp  float64
}

type relaxDropSpray struct {
	x, y, vx, vy float64
	life         float64
}

type relaxDropState struct {
	inited  bool
	tick    int
	ripples []relaxRipple
	spray   []relaxDropSpray

	// Gota em queda: y normalizado, negativo quando não há gota.
	dropX, dropY, dropV float64
	nextDrop            int
	flash               float64
}

func stepRelaxDrop(st *relaxDropState) {
	if !st.inited {
		st.inited = true
		st.dropY = -1
		st.nextDrop = 8
	}
	st.tick++
	st.flash *= 0.82

	if st.dropY < 0 {
		if st.nextDrop--; st.nextDrop <= 0 {
			st.dropX = 0.22 + rand.Float64()*0.56
			st.dropY, st.dropV = 0, 0.008
			st.nextDrop = 22 + rand.Intn(28) // 2,2 a 5 segundos
		}
	} else {
		st.dropV += 0.012
		st.dropY += st.dropV
		if st.dropY >= 0.62 { // altura da superfície
			st.dropY = -1
			st.flash = 1
			st.ripples = append(st.ripples, relaxRipple{x: st.dropX, y: 0.62, amp: 1})
			// Coroa: respingos saindo do ponto de impacto.
			for i, n := 0, 10+rand.Intn(8); i < n; i++ {
				a := rand.Float64() * math.Pi
				v := 0.010 + rand.Float64()*0.016
				st.spray = append(st.spray, relaxDropSpray{
					x: st.dropX, y: 0.62,
					vx: math.Cos(a) * v, vy: -math.Abs(math.Sin(a)) * v * 1.5,
					life: 1,
				})
			}
		}
	}

	keep := st.ripples[:0]
	for _, r := range st.ripples {
		r.r += 0.026   // atravessa a poça em uns 4 segundos
		r.amp *= 0.972 // some por atrito, não por corte
		if r.amp > 0.03 && r.r < 1.8 {
			keep = append(keep, r)
		}
	}
	st.ripples = keep

	ks := st.spray[:0]
	for _, p := range st.spray {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.0022
		if p.life -= 0.05; p.life > 0 && p.y < 0.66 {
			ks = append(ks, p)
		}
	}
	st.spray = ks
}

func relaxDropFrames(st *relaxDropState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxDrop(st)
	}
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBrailleVote(w, h)
	relaxDropDraw(st, b, w, h)
	status := "a poça descansa"
	switch {
	case st.flash > 0.4:
		status = "plic"
	case len(st.ripples) > 0:
		status = "as ondas se abrem"
	}
	return b.lines(relaxStyles(relaxDropRamp, fade)), StyleMuted.Render(status)
}

func relaxDropDraw(st *relaxDropState, b *relaxBraille, w, h int) {
	sw, sh := w*2, h*4
	t := float64(st.tick) * 0.1

	// A poça está no chão, vista de cima em ângulo: o anel tem de sair mais
	// largo que alto. O achatamento vai no eixo Y — no X ele fazia o contrário,
	// e a poça parecia estar em pé.
	const squash = 2.7

	// Respingos e gota, na frente.
	for _, p := range st.spray {
		b.set(int(p.x*float64(sw)), int(p.y*float64(sh)), relaxDropSpark)
	}
	if st.dropY >= 0 {
		dx := st.dropX * float64(sw)
		dy := st.dropY * float64(sh)
		// Estica com a velocidade: gota rápida vira um risco.
		tail := st.dropV * float64(sh) * 2.2
		for k := 0.0; k <= tail+1; k += 0.8 {
			b.set(int(dx), int(dy-k), relaxDropBead)
		}
		b.set(int(dx)-1, int(dy), relaxDropBead)
		b.set(int(dx)+1, int(dy), relaxDropBead)
	}

	// O brilho de fundo depende só de x ou só de y: dois senos por subpixel
	// custavam mais que as ondas inteiras.
	shimX := make([]float64, sw)
	for x := 0; x < sw; x++ {
		shimX[x] = 0.06 * math.Sin(float64(x)/float64(sw)*7+t*0.3)
	}

	// Altura da água: soma dos pacotes. Percorre x em sequência pra tirar a
	// inclinação da diferença com o vizinho, que é o que dá o relevo.
	for y := 0; y < sh; y++ {
		fy := float64(y) / float64(sh)
		shimY := 0.20 + 0.05*math.Sin(fy*11-t*0.2)
		prev := 0.0
		for x := 0; x < sw; x++ {
			fx := float64(x) / float64(sw)
			hgt := 0.0
			for _, r := range st.ripples {
				dx := fx - r.x
				dy := (fy - r.y) * squash
				d2 := dx*dx + dy*dy
				// Corte por distância ao quadrado, antes da raiz: quase todo
				// par (subpixel, onda) morre aqui.
				lo, hi := r.r-0.152, r.r+0.152
				if lo < 0 {
					lo = 0
				}
				if d2 < lo*lo || d2 > hi*hi {
					continue
				}
				d := math.Sqrt(d2)
				u := (d - r.r) * 46
				hgt += r.amp * relaxWavePack[int((u+7)/14*(relaxWaveN-1))] / (1 + d*1.5)
			}
			slope := hgt - prev
			prev = hgt

			// Fundo com um brilho fraco que varia devagar, senão a água parada
			// fica chapada demais.
			lum := shimY + shimX[x] + 0.55*hgt + 9*slope
			if st.flash > 0.02 {
				d := math.Hypot(fx-st.dropX, (fy-0.62)*squash)
				lum += st.flash * clamp01(1-d*9) * 0.9
			}
			b.set(x, y, relaxDropLevels-1-minInt(maxInt(int((1-clamp01(lum))*float64(relaxDropLevels-1)+0.5), 0), relaxDropLevels-1))
		}
	}

}
