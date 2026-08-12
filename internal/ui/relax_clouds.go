package ui

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

// ── Nuvens ────────────────────────────────────────────────────────────────────
//
// Só o céu. O que segura a cena é a luz: o dia inteiro passa em ~4 minutos, e a
// paleta caminha entre manhã, tarde, poente e noite — nuvem branca de meio-dia e
// nuvem cor de brasa no fim da tarde são a mesma nuvem com outra luz.
//
// A iluminação não é um degradê vertical: para cada ponto se compara a densidade
// com a densidade um pouco na direção do sol. Onde ela cai, é borda virada pra
// luz, e acende. É o que dá volume ao cúmulo em vez de um borrão branco — e é o
// mesmo cálculo que produz o fio de prata quando a nuvem passa na frente do sol.

// relaxDayStop é um instante do dia: três paradas de céu, três de nuvem e a cor
// do sol. O render interpola entre dois instantes vizinhos.
type relaxDayStop struct {
	sky   [3][3]float64
	cloud [3][3]float64
	sun   [3]float64
	name  string
}

var relaxDayCycle = []relaxDayStop{
	{name: "manhã",
		sky:   [3][3]float64{{0.16, 0.19, 0.34}, {0.45, 0.42, 0.55}, {0.86, 0.68, 0.55}},
		cloud: [3][3]float64{{0.22, 0.18, 0.28}, {0.74, 0.56, 0.54}, {1.00, 0.92, 0.84}},
		sun:   [3]float64{1.00, 0.88, 0.70}},
	{name: "meio-dia",
		sky:   [3][3]float64{{0.10, 0.28, 0.52}, {0.36, 0.58, 0.79}, {0.70, 0.84, 0.94}},
		cloud: [3][3]float64{{0.30, 0.35, 0.44}, {0.76, 0.81, 0.87}, {1.00, 1.00, 1.00}},
		sun:   [3]float64{1.00, 0.99, 0.92}},
	{name: "fim de tarde",
		sky:   [3][3]float64{{0.14, 0.11, 0.24}, {0.50, 0.24, 0.34}, {0.92, 0.56, 0.32}},
		cloud: [3][3]float64{{0.18, 0.13, 0.22}, {0.68, 0.37, 0.42}, {1.00, 0.80, 0.58}},
		sun:   [3]float64{1.00, 0.72, 0.40}},
	{name: "noite",
		sky:   [3][3]float64{{0.03, 0.04, 0.09}, {0.07, 0.10, 0.20}, {0.15, 0.20, 0.34}},
		cloud: [3][3]float64{{0.05, 0.06, 0.11}, {0.14, 0.17, 0.26}, {0.33, 0.38, 0.50}},
		sun:   [3]float64{0.82, 0.84, 0.90}},
}

const (
	relaxCldSkyN   = 16 // degraus generosos: com poucos, o halo do sol vira anel
	relaxCldCloudN = 13
)

const (
	relaxCldSky   = 0
	relaxCldCloud = relaxCldSkyN
	relaxCldSun   = relaxCldCloud + relaxCldCloudN
	relaxCldBird  = relaxCldSun + 1
)

type relaxCloudBird struct {
	x, y, vx, ph float64
}

type relaxCloudState struct {
	inited bool
	tick   int
	ph     [8]float64
	sunX   float64
	birds  []relaxCloudBird
	nextB  int
}

func stepRelaxClouds(st *relaxCloudState) {
	if !st.inited {
		st.inited = true
		for i := range st.ph {
			st.ph[i] = rand.Float64() * 2 * math.Pi
		}
		st.sunX = 0.28 + rand.Float64()*0.44
		st.nextB = 40
	}
	st.tick++

	if st.nextB--; st.nextB <= 0 {
		// Bando pequeno, de um lado ou do outro.
		dir := 1.0
		x := -0.08
		if rand.Intn(2) == 0 {
			dir, x = -1, 1.08
		}
		for i, n := 0, 3+rand.Intn(4); i < n; i++ {
			st.birds = append(st.birds, relaxCloudBird{
				x:  x - dir*float64(i)*0.035,
				y:  0.24 + rand.Float64()*0.34 + float64(i)*0.012,
				vx: dir * (0.0022 + rand.Float64()*0.0012),
				ph: rand.Float64() * 6.28,
			})
		}
		st.nextB = 260 + rand.Intn(420)
	}
	keep := st.birds[:0]
	for _, bd := range st.birds {
		bd.x += bd.vx
		bd.y += 0.0006 * math.Sin(float64(st.tick)*0.06+bd.ph)
		bd.ph += 0.34
		if bd.x > -0.12 && bd.x < 1.12 {
			keep = append(keep, bd)
		}
	}
	st.birds = keep
}

func relaxCldRGB(c [3]float64) lipgloss.Color {
	f := func(v float64) int { return int(clamp01(v)*255 + 0.5) }
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", f(c[0]), f(c[1]), f(c[2])))
}

// relaxCldRamp monta uma rampa de n degraus a partir de três paradas RGB.
func relaxCldRamp(stops [3][3]float64, n int, out []lipgloss.Color) {
	for i := 0; i < n; i++ {
		p := float64(i) / float64(n-1) * 2
		k := minInt(int(p), 1)
		f := p - float64(k)
		var c [3]float64
		for j := 0; j < 3; j++ {
			c[j] = lerp(stops[k][j], stops[k+1][j], f)
		}
		out[i] = relaxCldRGB(c)
	}
}

func relaxCloudsFrames(st *relaxCloudState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxClouds(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBrailleVote(w, h)
	pal, name := relaxCloudsDraw(st, b, w, h)
	return b.lines(relaxStyles(pal, fade)), StyleMuted.Render(name)
}

func relaxCloudsDraw(st *relaxCloudState, b *relaxBraille, w, h int) ([]lipgloss.Color, string) {
	sw, sh := w*2, h*4
	t := float64(st.tick) * 0.1

	// Relógio do dia: uma volta completa em ~4 minutos.
	day := math.Mod(t/240, 1) * float64(len(relaxDayCycle))
	di := int(day)
	dm := day - float64(di)
	a, bq := relaxDayCycle[di], relaxDayCycle[(di+1)%len(relaxDayCycle)]
	mix3 := func(x, y [3][3]float64) [3][3]float64 {
		var o [3][3]float64
		for i := range o {
			for j := range o[i] {
				o[i][j] = lerp(x[i][j], y[i][j], dm)
			}
		}
		return o
	}
	pal := make([]lipgloss.Color, relaxCldBird+1)
	relaxCldRamp(mix3(a.sky, bq.sky), relaxCldSkyN, pal[relaxCldSky:])
	relaxCldRamp(mix3(a.cloud, bq.cloud), relaxCldCloudN, pal[relaxCldCloud:])
	var sc [3]float64
	for j := range sc {
		sc[j] = lerp(a.sun[j], bq.sun[j], dm)
	}
	pal[relaxCldSun] = relaxCldRGB(sc)
	pal[relaxCldBird] = relaxCldRGB([3]float64{0.10, 0.10, 0.14})

	// O sol sobe e desce com o mesmo relógio.
	sunX := st.sunX * float64(sw)
	sunY := float64(sh) * (0.16 + 0.52*clamp01(math.Sin(math.Mod(t/240, 1)*math.Pi*2-0.6)*-0.5+0.5))
	sunR := float64(minInt(sw, sh)) * 0.085

	// ── Pássaros ── na frente de tudo.
	for _, bd := range st.birds {
		bx, by := bd.x*float64(sw), bd.y*float64(sh)
		flap := math.Sin(bd.ph) * 1.6
		for k := 0.0; k <= 2.6; k += 0.5 {
			b.set(int(bx-k), int(by-k*0.35-flap*k/2.6), relaxCldBird)
			b.set(int(bx+k), int(by-k*0.35-flap*k/2.6), relaxCldBird)
		}
	}

	// ── Cúmulos ── campo em buffer: a iluminação precisa olhar o vizinho na
	// direção do sol, e pra isso o valor tem de estar guardado.
	den := make([]float32, sw*sh)
	for li, l := range [2]struct{ alt, spread, drift, amp float64 }{
		{0.46, 0.30, 1.00, 1.00}, // banco principal
		{0.66, 0.20, 1.85, 0.62}, // camada baixa, mais rápida (paralaxe)
	} {
		for k, f := range [3]float64{1, 2.3, 4.4} {
			sx, cx2, sy, cy2 := relaxTreePlane(0.016*f, 0.052*f, st.ph[li*3+k]+t*0.011*f, sw, sh)
			off := int(t * l.drift * 1.5)
			wt := float32(l.amp / (1 + f*0.9))
			for y := 0; y < sh; y++ {
				cyv, syv := cy2[y], sy[y]
				row := y * sw
				for x := 0; x < sw; x++ {
					i := x + off
					for i >= sw {
						i -= sw
					}
					den[row+x] += wt * float32(sx[i]*cyv+cx2[i]*syv)
				}
			}
		}
		// Banda de altitude: o cúmulo tem base chata e topo fofo, então o corte
		// por baixo é bem mais duro que o de cima.
		altY := l.alt * float64(sh)
		for y := 0; y < sh; y++ {
			d := (float64(y) - altY) / (l.spread * float64(sh))
			band := 1 - d*d
			if d > 0 {
				band = 1 - d*d*3.2 // base chata
			}
			if band < 0 {
				band = 0
			}
			row := y * sw
			for x := 0; x < sw; x++ {
				den[row+x] *= float32(band)
			}
		}
	}

	// Deslocamento da amostra na direção do sol, em subpixels.
	lx, ly := 5, 4
	if sunX < float64(sw)/2 {
		lx = -5
	}
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			d := float64(den[y*sw+x])
			if d <= 0.02 {
				continue
			}
			// Densidade um passo na direção do sol: se lá é menos densa, este
			// ponto está na borda virada pra luz.
			nx, ny := x+lx, y-ly
			toward := 0.0
			if nx >= 0 && nx < sw && ny >= 0 {
				toward = float64(den[ny*sw+nx])
			}
			lit := clamp01((d - toward) * 2.6)
			lum := 0.20 + 0.52*clamp01(d*1.7) + 0.46*lit

			// Fio de prata: nuvem fina bem na frente do sol fica incandescente.
			ds := math.Hypot(float64(x)-sunX, (float64(y)-sunY)*2) / (sunR * 3.2)
			if ds < 1 {
				lum += (1 - ds) * (1 - ds) * (1.5 - clamp01(d*2.2)) * 0.9
			}
			if lum <= relaxHalftone(x, y)*0.34 {
				continue
			}
			b.set(x, y, relaxCldCloud+minInt(int(lum*float64(relaxCldCloudN)), relaxCldCloudN-1))
		}
	}

	// ── Cirros ── altos e esgarçados: ruído em crista, esticado na horizontal.
	s1x, c1x, s1y, c1y := relaxTreePlane(0.010, 0.170, st.ph[6]+t*0.02, sw, sh)
	s2x, c2x, s2y, c2y := relaxTreePlane(0.031, 0.330, st.ph[7]-t*0.03, sw, sh)
	for y := 0; y < int(float64(sh)*0.42); y++ {
		fade := clamp01(1 - float64(y)/(float64(sh)*0.42))
		for x := 0; x < sw; x++ {
			v := s1x[x]*c1y[y] + c1x[x]*s1y[y] + 0.5*(s2x[x]*c2y[y]+c2x[x]*s2y[y])
			r := 1 - math.Abs(v/1.5)
			lum := (r*r*r*1.5 - 0.72) * fade
			if lum <= relaxHalftone(x, y)*0.5 {
				continue
			}
			b.set(x, y, relaxCldCloud+minInt(int(lum*float64(relaxCldCloudN)*0.8), relaxCldCloudN-1))
		}
	}

	// ── Sol ── e o céu por último, preenchendo o que sobrou.
	for y := int(sunY - sunR/2); y <= int(sunY+sunR/2); y++ {
		for x := int(sunX - sunR); x <= int(sunX+sunR); x++ {
			nx, ny := (float64(x)-sunX)/sunR, (float64(y)-sunY)/(sunR/2)
			if nx*nx+ny*ny <= 1 {
				b.set(x, y, relaxCldSun)
			}
		}
	}
	for y := 0; y < sh; y++ {
		fy := float64(y) / float64(sh)
		for x := 0; x < sw; x++ {
			v := 0.12 + 0.80*fy*fy
			d := math.Hypot(float64(x)-sunX, (float64(y)-sunY)*2.0) / (sunR * 5.0)
			v += 0.50 / (1 + d*d*5)
			// Céu sem meio-tom: liso, em faixas de cor.
			b.set(x, y, relaxCldSky+minInt(int(v*float64(relaxCldSkyN)), relaxCldSkyN-1))
		}
	}

	return pal, relaxDayCycle[di].name
}
