package ui

import (
	"math"
	"math/rand"
)

// ── Lua ───────────────────────────────────────────────────────────────────────
//
// Uma lua grande em Braille (2×4 subpixels por célula). Não é um disco pintado:
// para cada subpixel dentro do círculo se reconstrói a normal da esfera, e daí
// saem terminador, mares e crateras por iluminação de verdade. Por isso o
// terminador é um degradê de densidade de pontos, e não uma linha serrilhada.
//
// O que se move: a fase (ciclo de ~55s), a libração (a lua balança de leve, como
// a de verdade) e nuvens finas atravessando na frente. Tudo contínuo.

var relaxMoonStops = []string{"#0B0D14", "#2B3040", "#5C6272", "#9AA0AC", "#D8D6CE", "#F6F2E6"}

const relaxMoonLevels = 18

const (
	relaxMoonStar  = relaxMoonLevels + iota // estrela de fundo
	relaxMoonCloud                          // nuvem na frente
)

var relaxMoonRamp = func() []relaxColor {
	out := make([]relaxColor, relaxMoonCloud+1)
	copy(out, relaxRamp(relaxMoonStops, relaxMoonLevels))
	out[relaxMoonStar] = "#8C93A8"
	out[relaxMoonCloud] = "#20242F"
	return out
}()

// relaxMoonSpot é um acidente do relevo em coordenadas de superfície: um vetor
// unitário na esfera, um raio angular e quanto ele escurece ou clareia.
type relaxMoonSpot struct {
	x, y, z float64
	r       float64
	lim     float64 // (1,35·r)², o corte barato antes da raiz
	albedo  float64 // negativo = mar; positivo = ejeção clara
	rim     float64 // >0 desenha borda de cratera
}

type relaxMoonState struct {
	inited bool
	tick   int
	phase  float64 // 0 = nova, π = cheia
	spots  []relaxMoonSpot
	stars  []relaxSkyPt
}

func relaxMoonInit(st *relaxMoonState) {
	st.inited = true
	st.phase = rand.Float64() * 2 * math.Pi
	st.spots = st.spots[:0]

	// Mares: manchas grandes e escuras, concentradas num hemisfério — a lua
	// real é bem assimétrica, e um disco de manchas espalhadas parece bolinha
	// de gude.
	bias := rand.Float64() * 2 * math.Pi
	for i, n := 0, 5+rand.Intn(3); i < n; i++ {
		lon := bias + (rand.Float64()-0.5)*2.2
		lat := (rand.Float64() - 0.5) * 1.7
		st.spots = append(st.spots, relaxMoonSpot{
			x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
			r: 0.22 + rand.Float64()*0.24, albedo: -0.30 - rand.Float64()*0.16,
		})
	}
	// Crateras: menores, espalhadas, fundo escuro e borda clara.
	for i, n := 0, 16+rand.Intn(10); i < n; i++ {
		lon := rand.Float64() * 2 * math.Pi
		lat := math.Asin(rand.Float64()*2 - 1)
		st.spots = append(st.spots, relaxMoonSpot{
			x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
			r:      0.045 + rand.Float64()*0.075,
			albedo: -0.14 - rand.Float64()*0.12,
			rim:    0.22 + rand.Float64()*0.20,
		})
	}

	for i := range st.spots {
		sp := &st.spots[i]
		sp.lim = (1.35 * sp.r) * (1.35 * sp.r)
	}

	st.stars = st.stars[:0]
	for i, n := 0, 48+rand.Intn(26); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64()})
	}
}

func stepRelaxMoon(st *relaxMoonState) {
	if !st.inited {
		relaxMoonInit(st)
	}
	st.tick++
	st.phase += 2 * math.Pi / 550 // volta completa em ~55s
	if st.phase > 2*math.Pi {
		st.phase -= 2 * math.Pi
	}
}

var relaxMoonNames = []string{
	"lua nova", "crescente fina", "quarto crescente", "gibosa crescente",
	"lua cheia", "gibosa minguante", "quarto minguante", "minguante fina",
}

func relaxMoonFrames(st *relaxMoonState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxMoon(st)
	}
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBraille(w, h)
	sw, sh := w*2, h*4
	t := float64(st.tick) * 0.1

	cx, cy := float64(sw-1)/2, float64(sh-1)*0.46
	rad := math.Min(float64(sw)*0.34, float64(sh)*0.44)

	// Libração: a lua balança devagar em torno dos dois eixos, então as
	// crateras da borda somem e reaparecem em vez de ficarem cravadas.
	librX := 0.13 * math.Sin(t*0.037)
	librY := 0.10 * math.Sin(t*0.029+1.1)
	sx, cxr := math.Sin(librX), math.Cos(librX)
	sy, cyr := math.Sin(librY), math.Cos(librY)

	// Luz: percorre o equador, e é o que varre o terminador pela face.
	lx, lz := math.Cos(st.phase), math.Sin(st.phase)

	// Nuvem fina atravessando na frente. As três ondas planas são separáveis
	// em x e y, então o laço interno não chama math.Sin nenhuma vez — são
	// dezenas de milhares de subpixels por frame.
	s1x, c1x, s1y, c1y := relaxTreePlane(0.031, 0, -t*0.30, sw, sh)
	s2x, c2x, s2y, c2y := relaxTreePlane(0.017, 0.09, t*0.19, sw, sh)
	s3x, c3x, s3y, c3y := relaxTreePlane(0.058, -0.04, -t*0.41, sw, sh)
	cloud := func(x, y int) float64 {
		v := s1x[x]*c1y[y] + c1x[x]*s1y[y] +
			0.7*(s2x[x]*c2y[y]+c2x[x]*s2y[y]) +
			0.5*(s3x[x]*c3y[y]+c3x[x]*s3y[y])
		return clamp01((v/2.2)*1.9 - 0.62)
	}

	for _, p := range st.stars {
		x, y := p.x*float64(sw-1), p.y*float64(sh-1)
		if math.Hypot(x-cx, y-cy) < rad+1 {
			continue // atrás da lua
		}
		if cloud(int(x), int(y)) > 0.45 {
			continue // encoberta
		}
		if relaxHalftone(int(x), int(y)) < 0.62 {
			b.set(int(x), int(y), relaxMoonStar)
		}
	}

	x0, x1 := maxInt(0, int(cx-rad)-1), minInt(sw-1, int(cx+rad)+1)
	y0, y1 := maxInt(0, int(cy-rad)-1), minInt(sh-1, int(cy+rad)+1)

	// Mancha por ladrilho. Sem isso cada subpixel do disco testa as ~30 manchas,
	// e é aí que ia quase todo o tempo do frame. A posição de tela da mancha sai
	// da transposta da rotação da libração (rotação é ortonormal, então a
	// transposta é a inversa).
	const tiles = 8
	tw := float64(x1-x0+1)/tiles + 0.001
	th := float64(y1-y0+1)/tiles + 0.001
	buckets := make([][]int16, tiles*tiles)
	for i := range st.spots {
		sp := st.spots[i]
		nzs := sy*sp.x - sx*cyr*sp.y + cxr*cyr*sp.z
		if nzs < -0.45 {
			continue // do outro lado
		}
		nxs := cyr*sp.x + sx*sy*sp.y - cxr*sy*sp.z
		nys := cxr*sp.y + sx*sp.z
		px, py := cx+nxs*rad, cy+nys*rad
		pr := 1.35*sp.r*rad + 2
		for ty := int((py - pr - float64(y0)) / th); ty <= int((py+pr-float64(y0))/th); ty++ {
			if ty < 0 || ty >= tiles {
				continue
			}
			for tx := int((px - pr - float64(x0)) / tw); tx <= int((px+pr-float64(x0))/tw); tx++ {
				if tx < 0 || tx >= tiles {
					continue
				}
				buckets[ty*tiles+tx] = append(buckets[ty*tiles+tx], int16(i))
			}
		}
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			nx, ny := (float64(x)-cx)/rad, (float64(y)-cy)/rad
			d2 := nx*nx + ny*ny
			if d2 > 1 {
				continue
			}
			nz := math.Sqrt(1 - d2)

			// Coordenada de superfície: desfaz a libração pra que a textura
			// ande com a lua em vez de ficar colada na tela.
			ux := nx*cyr + nz*sy
			uz := -nx*sy + nz*cyr
			uy := ny*cxr - uz*sx
			uz = ny*sx + uz*cxr

			albedo := 0.80
			ti := minInt(tiles-1, int(float64(y-y0)/th))*tiles + minInt(tiles-1, int(float64(x-x0)/tw))
			for _, si := range buckets[ti] {
				s := st.spots[si]
				// Corda ao quadrado primeiro: quase todo par (subpixel, mancha)
				// morre aqui, e sem raiz quadrada.
				dx, dy, dz := ux-s.x, uy-s.y, uz-s.z
				d2s := dx*dx + dy*dy + dz*dz
				if d2s > s.lim {
					continue
				}
				if ux*s.x+uy*s.y+uz*s.z <= 0 {
					continue // do outro lado da esfera
				}
				dist := math.Sqrt(d2s) / s.r
				if dist < 1 {
					albedo += s.albedo * (1 - dist*dist*dist)
				} else if s.rim > 0 {
					albedo += s.rim * (1 - (dist-1)/0.35) // borda clara
				}
			}

			// Lambert com um pouco de retroespalhamento: a lua cheia é chapada,
			// não uma bola com highlight no meio.
			lam := nx*lx + nz*lz
			// Luz cinzenta: o lado escuro não é preto, é a Terra iluminando de
			// volta. Sai como uma poeira rala de pontos e é o que faz a
			// crescente ainda ter disco em vez de virar uma foice solta.
			lum := 0.12
			if lam > 0 {
				lum += clamp01(albedo) * (0.18 + 0.86*math.Pow(lam, 0.55))
			}
			lum *= 1 - 0.72*cloud(x, y)

			if lum <= relaxHalftone(x, y)*0.92 {
				continue
			}
			b.set(x, y, int(clamp01(lum)*float64(relaxMoonLevels-1)+0.5))
		}
	}

	// A nuvem em si: um véu tênue por cima de tudo.
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			if c := cloud(x, y); c > 0.30 && relaxHalftone(x, y) < (c-0.30)*0.85 {
				b.set(x, y, relaxMoonCloud)
			}
		}
	}

	// Cheia quando a luz aponta pro observador (fase π/2), daí o +2.
	name := relaxMoonNames[int(math.Mod(st.phase/(2*math.Pi)*8+2.5, 8))]
	return b.lines(relaxStyles(relaxMoonRamp, fade)), StyleMuted.Render(name)
}
