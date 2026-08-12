package ui

import (
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

// ── Copa ao vento ─────────────────────────────────────────────────────────────
//
// Desenhada em Braille (2×4 subpixels por célula), do jeito que uma imagem vira
// ASCII: quem decide o caractere é a densidade de pontos acesos, e a densidade
// vem da luz. Folha no sol acende quase todos os pontos (⣿), sombra acende
// poucos (⣤⣀), a franja acende um ou dois (⠂⠄). Por isso não há área chapada —
// a textura é o próprio meio-tom.
//
// Separação exigida pela cena (e pelo custo): a ESTRUTURA — silhueta, cachos de
// folhas, luz base, rigidez e o ruído do meio-tom — é calculada uma vez em
// grades de subpixel. Por frame só se avaliam vento e sombra, que deformam a
// amostragem dessas grades. Deformar a amostragem (domain warp) em vez de mover
// os cachos é o que faz a folhagem mexer por dentro com a silhueta parada.

var relaxTreeStops = []string{"#173D2C", "#286335", "#3D9638", "#65C43A"}

const (
	relaxTreeLevels = 16              // degraus de luz na copa
	relaxTreeTrunk  = relaxTreeLevels // nível extra: madeira
	relaxTreeWood   = "#3A2E20"
)

type relaxTreeState struct {
	inited bool
	tick   int
	w, h   int
	sw, sh int
	x0, x1 int // bbox da copa, em subpixels
	y0, y1 int

	dens  []float32 // 0 fora da copa, 1 folhagem cheia
	base  []float32 // luminância estática: luz, camada de profundidade, miolo
	stiff []float32 // quanto o vento mexe naquela região
	dith  []float32 // limiar fixo do meio-tom (fixo, senão a copa cintila)
	lo    []int16   // primeira e última coluna com folhagem em cada linha
	hi    []int16
	trunk [][2]int16

	clusters int
}

// relaxTreeRamp interpola a paleta em degraus, mais o tom da madeira no fim.
// Cor não muda com o tempo — só qual degrau cada célula usa.
var relaxTreeRamp = func() []lipgloss.Color {
	out := make([]lipgloss.Color, relaxTreeLevels+1)
	copy(out, relaxRamp(relaxTreeStops, relaxTreeLevels))
	out[relaxTreeTrunk] = lipgloss.Color(relaxTreeWood)
	return out
}()

func stepRelaxTree(st *relaxTreeState) { st.tick++ }

func relaxTreeBuild(st *relaxTreeState, w, h int) {
	tick := st.tick
	*st = relaxTreeState{inited: true, tick: tick, w: w, h: h, sw: w * 2, sh: h * 4}
	sw, sh := float64(st.sw), float64(st.sh)
	n := st.sw * st.sh
	st.dens, st.base, st.stiff, st.dith = make([]float32, n), make([]float32, n), make([]float32, n), make([]float32, n)
	wsum := make([]float32, n)
	// Meio-tom ordenado (Bayer 4×4) com uma pitada de ruído: só ruído branco
	// vira chuvisco, só Bayer vira trama de tecido. A mistura dá folhagem.
	// 8×8 em vez de 4×4: a trama fica pequena demais pra ler como padrão.
	var bayer [64]float64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v, mask := 0, 4
			for b := 0; b < 3; b++ {
				bx, by := (x/mask)&1, (y/mask)&1
				v = v<<2 | (bx^by)<<1 | bx
				mask >>= 1
			}
			bayer[y*8+x] = (float64(v) + 0.5) / 64
		}
	}
	for y := 0; y < st.sh; y++ {
		for x := 0; x < st.sw; x++ {
			// Bayer só encosta: sozinho ele desenha trama regular na folhagem.
			st.dith[y*st.sw+x] = float32(0.18*bayer[(y%8)*8+x%8] + 0.82*rand.Float64())
		}
	}

	cy, cry := sh*0.41, sh*0.39
	// A largura é limitada pela altura: em terminal largo e baixo uma copa
	// esticada viraria uma faixa, não uma árvore.
	crx := math.Min(sw*0.45, cry*1.7)
	cx := sw * 0.5

	// Silhueta: raio modulado por três harmônicas de fase sorteada — dá
	// saliências e reentrâncias sem virar círculo, estrela nem triângulo.
	a1, a2, a3 := rand.Float64()*2*math.Pi, rand.Float64()*2*math.Pi, rand.Float64()*2*math.Pi
	k1, k2, k3 := 2.0+float64(rand.Intn(2)), 4.0+float64(rand.Intn(2)), 7.0+float64(rand.Intn(3))
	// inside devolve 0 fora e ~1 no miolo da copa.
	inside := func(x, y float64) float64 {
		nx, ny := (x-cx)/crx, (y-cy)/cry
		d := math.Hypot(nx, ny)
		th := math.Atan2(-ny, nx)
		r := 1 + 0.13*math.Sin(k1*th+a1) + 0.09*math.Sin(k2*th+a2) + 0.05*math.Sin(k3*th+a3)
		if d >= r {
			return 0
		}
		return 1 - d/r
	}

	st.x0, st.x1 = maxInt(0, int(cx-crx*1.3)), minInt(st.sw-1, int(cx+crx*1.3))
	st.y0, st.y1 = maxInt(0, int(cy-cry*1.3)), minInt(st.sh-1, int(cy+cry*1.3))

	// Cachos: a unidade orgânica da copa. Cada um carrega direção, densidade,
	// iluminação e rigidez próprias, e é a soma deles que dá o volume irregular.
	target := int(math.Pi * crx * cry / 17)
	for tries := 0; st.clusters < target && tries < target*40; tries++ {
		x := cx + (rand.Float64()*2-1)*crx*1.15
		y := cy + (rand.Float64()*2-1)*cry*1.15
		f := inside(x, y)
		if f <= 0 || rand.Float64() > 0.45+0.55*math.Sqrt(f) {
			continue
		}
		st.clusters++
		r := 3.5 + rand.Float64()*5.0
		// Três camadas: fundo escuro e difuso, meio, frente clara e definida.
		layer := 1
		switch p := rand.Float64(); {
		case p < 0.30:
			layer = 0
		case p > 0.68:
			layer = 2
		}
		// Luz suave de cima e um pouco da esquerda; o miolo da copa é sombra.
		top := clamp01(1 - (y-(cy-cry))/(2*cry))
		left := clamp01(0.5 - (x-cx)/(2.4*crx))
		lum := 0.42 + 0.30*top + 0.10*left - 0.20*f +
			[]float64{-0.15, 0, 0.13}[layer] + (rand.Float64()-0.5)*0.09
		// Ponta de galho balança; cacho de dentro quase não. Fundo é mais preso.
		sway := (0.20 + 0.80*(1-f)) * (0.5 + 0.5*float64(layer)/2)
		// Elipse girada: cada cacho tem direção, senão a copa vira bolinhas.
		ang := rand.Float64() * math.Pi
		sn, cs := math.Sin(ang), math.Cos(ang)
		ry := r * (0.55 + rand.Float64()*0.25)

		for dy := -int(r) - 1; dy <= int(r)+1; dy++ {
			for dx := -int(r) - 1; dx <= int(r)+1; dx++ {
				px, py := int(x)+dx, int(y)+dy
				if px < 0 || py < 0 || px >= st.sw || py >= st.sh {
					continue
				}
				fx, fy := float64(dx), float64(dy)
				ex, ey := (fx*cs+fy*sn)/r, (-fx*sn+fy*cs)/ry
				d2 := ex*ex + ey*ey
				if d2 > 1 {
					continue
				}
				g := float32(math.Exp(-1.7 * d2))
				i := py*st.sw + px
				st.dens[i] += g
				st.base[i] += g * float32(lum)
				st.stiff[i] += g * float32(sway)
				wsum[i] += g
			}
		}
	}

	for i := range st.dens {
		if wsum[i] <= 0 {
			continue
		}
		st.base[i] /= wsum[i]
		st.stiff[i] /= wsum[i]
		// A soma dos cachos satura: o miolo fica cheio e só a franja fica raleada.
		if st.dens[i] = st.dens[i] * 0.85; st.dens[i] > 1 {
			st.dens[i] = 1
		}
	}

	// Vão de folhagem por linha (com folga pro deslocamento do vento): o laço
	// de render pula o vazio em volta da copa em vez de varrer o retângulo.
	st.lo, st.hi = make([]int16, st.sh), make([]int16, st.sh)
	for y := 0; y < st.sh; y++ {
		lo, hi := -1, -1
		for x := 0; x < st.sw; x++ {
			if st.dens[y*st.sw+x] > 0.02 {
				if lo < 0 {
					lo = x
				}
				hi = x
			}
		}
		st.lo[y], st.hi[y] = int16(maxInt(0, lo-5)), int16(minInt(st.sw-1, hi+5))
		if lo < 0 {
			st.lo[y], st.hi[y] = 0, -1
		}
	}

	// Tronco e dois galhos. Entram depois da folhagem no desenho, então só
	// aparecem nas frestas — é o que mantém a madeira discreta.
	tx := cx + (rand.Float64()-0.5)*sw*0.03
	for y := int(cy + cry*0.5); y < st.sh; y++ {
		fy := float64(y)
		half := lerp(0.7, 2.2, clamp01((fy-cy)/(sh-cy)))
		bend := 2.0 * math.Sin(fy*0.030+a1)
		for x := -int(half); x <= int(half); x++ {
			st.trunk = append(st.trunk, [2]int16{int16(tx + bend + float64(x)), int16(y)})
		}
	}
	for _, dir := range []float64{-1, 1} {
		bx, by := tx, cy+cry*0.62
		for s := 0.0; s < cry*0.85; s += 0.55 {
			bx += dir * (0.60 + 0.25*math.Sin(s*0.09))
			by -= 0.70
			st.trunk = append(st.trunk, [2]int16{int16(bx), int16(by)})
		}
	}
}

// relaxTreePlane separa a onda plana sin(ax+by+c) em fatores de x e de y:
// sin(ax+by+c) = sin(ax)·cos(by+c) + cos(ax)·sin(by+c). O laço interno vira
// duas multiplicações em vez de um math.Sin por subpixel — a copa inteira são
// dezenas de milhares de subpixels por frame, e é aí que estava o custo todo.
func relaxTreePlane(a, b, c float64, w, h int) (sx, cx, sy, cy []float64) {
	sx, cx, sy, cy = make([]float64, w), make([]float64, w), make([]float64, h), make([]float64, h)
	for x := 0; x < w; x++ {
		sx[x], cx[x] = math.Sincos(a * float64(x))
	}
	for y := 0; y < h; y++ {
		sy[y], cy[y] = math.Sincos(b*float64(y) + c)
	}
	return
}

func relaxTreeFrames(st *relaxTreeState, width, height int, fade float64) ([]string, string) {
	w := maxInt(28, minInt(width, 110))
	h := maxInt(7, minInt(height, 32))
	if !st.inited || st.w != w || st.h != h {
		relaxTreeBuild(st, w, h)
	}
	b := newRelaxBraille(w, h)
	t := float64(st.tick) * 0.1
	// Os primeiros segundos são quase parados: a brisa entra depois.
	amp := easeOutCubic(t / 7)
	// Lufada longa (~55s) por cima da onda que atravessa a copa (~9s da
	// esquerda pra direita). Só senos: o loop não tem início nem emenda.
	gust := 0.55 + 0.45*math.Sin(t*0.113)
	fall := 0.80 * float64(st.sh)

	// Onda de vento: depende só da coluna, então uma por coluna basta.
	wcol := make([]float64, st.sw)
	for x := 0; x < st.sw; x++ {
		tr := t*0.30 - float64(x)*0.026
		wcol[x] = gust * amp * (math.Sin(tr) + 0.42*math.Sin(tr*2.3+1.7)) / 1.42
	}
	// Sombras orgânicas: três ondas planas em direções diferentes, derivando
	// com o vento. Senos não alinhados não desenham grade.
	s1x, c1x, s1y, c1y := relaxTreePlane(0.041, 0.062, t*0.28, st.sw, st.sh)
	s2x, c2x, s2y, c2y := relaxTreePlane(0.083, -0.037, -t*0.17, st.sw, st.sh)
	s3x, c3x, s3y, c3y := relaxTreePlane(0.027, 0.110, t*0.41, st.sw, st.sh)

	for y := st.y0; y <= st.y1; y++ {
		// O alto da copa sente mais vento que o interior baixo.
		hgt := 0.35 + 0.65*clamp01(1-float64(y)/fall)
		row := y * st.sw
		for x := int(st.lo[y]); x <= int(st.hi[y]); x++ {
			si := row + x
			wind := wcol[x] * hgt * float64(st.stiff[si])

			// Domain warp: amostra a folhagem alguns subpixels atrás do vento.
			// A silhueta fica de pé porque o miolo do campo é constante; só a
			// franja e as manchas de luz é que caminham.
			sx := x - int(math.Round(wind*3.4))
			sy := y + int(math.Round(math.Abs(wind)*1.2))
			if sx < 0 || sy < 0 || sx >= st.sw || sy >= st.sh {
				continue
			}
			d := st.dens[sy*st.sw+sx]
			if d <= 0.02 {
				continue
			}

			shade := (s1x[x]*c1y[y] + c1x[x]*s1y[y] +
				0.70*(s2x[x]*c2y[y]+c2x[x]*s2y[y]) +
				0.50*(s3x[x]*c3y[y]+c3x[x]*s3y[y])) / 2.2
			lum := clamp01(float64(st.base[sy*st.sw+sx]) + 0.22*shade + 0.18*wind)

			// Meio-tom: o ponto acende conforme a luz. É daqui que sai o ⣿ no
			// claro e o ⣤⣀⠂ na sombra, em vez de bloco chapado.
			if float64(d)*(0.34+0.86*lum) <= float64(st.dith[si]) {
				continue
			}
			b.set(x, y, int(lum*float64(relaxTreeLevels-1)+0.5))
		}
	}
	for _, p := range st.trunk {
		b.set(int(p[0]), int(p[1]), relaxTreeTrunk)
	}

	pal := relaxStyles(relaxTreeRamp, fade)
	status := "a brisa atravessa a copa"
	if gust < 0.35 {
		status = "o vento quase parou"
	}
	return b.lines(pal), StyleMuted.Render(status)
}
