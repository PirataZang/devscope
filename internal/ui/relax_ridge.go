package ui

import (
	"math"
	"math/rand"
)

// ── Montanha com nuvens ───────────────────────────────────────────────────────
//
// Fim de tarde numa serra: cinco cadeias de morro, uma atrás da outra, e nuvens
// atravessando o vale muito devagar.
//
// A profundidade sai toda da perspectiva aérea: quanto mais longe, mais CLARA a
// cadeia, porque o ar entre você e ela também tem cor. Por isso a cadeia da
// frente é quase preta e a do fundo quase se confunde com o céu — é o único
// jeito de cinco silhuetas empilhadas não virarem uma mancha só.
//
// As nuvens entram ENTRE as cadeias, não por cima de todas: cada nuvem tem uma
// profundidade e é desenhada depois do morro que está na frente dela. É isso, e
// não o tamanho, que diz ao olho que ela está longe.

const (
	relaxRgSkyN   = 10
	relaxRgCloudN = 4
	relaxRgLayers = 5
)

const (
	relaxRgSky   = 0
	relaxRgCloud = relaxRgSkyN
	relaxRgHill  = relaxRgCloud + relaxRgCloudN // relaxRgLayers níveis, do fundo pra frente
)

const (
	relaxRgSun  = relaxRgHill + relaxRgLayers
	relaxRgMist = relaxRgSun + 1
	relaxRgBird = relaxRgMist + 1
)

var relaxRidgeRamp = func() []relaxColor {
	out := make([]relaxColor, relaxRgBird+1)
	// Céu de fim de tarde: azul lá em cima, quente na linha do horizonte.
	copy(out[relaxRgSky:], relaxRamp([]string{"#16224A", "#2C3A6E", "#5A5688", "#9C6E86", "#D89478", "#F6C48E"}, relaxRgSkyN))
	copy(out[relaxRgCloud:], relaxRamp([]string{"#6E6A96", "#A08EB0", "#DCB4B0", "#FCE0C0"}, relaxRgCloudN))
	// Do fundo pra frente: clara → quase preta.
	copy(out[relaxRgHill:], relaxRamp([]string{"#9A9CC0", "#6C6E9C", "#464C78", "#282E52", "#0E1330"}, relaxRgLayers))
	out[relaxRgSun] = "#FFE8B8"
	out[relaxRgMist] = "#C8BCD0"
	out[relaxRgBird] = "#2A2A3E"
	return out
}()

type relaxRgCloudObj struct {
	x, y  float64 // 0–1 no palco
	w, h  float64
	v     float64
	depth int // 0 = na frente de tudo, relaxRgLayers-1 = no fundo
	seed  int
}

type relaxRidgeState struct {
	inited bool
	tick   int

	ph     [relaxRgLayers * 3]float64
	clouds []relaxRgCloudObj
	birds  []relaxSkyPt
	birdV  float64
	nextB  int
	mist   float64 // fase da névoa do fundo do vale
}

// relaxRgRidge devolve a linha base e a amplitude da cadeia d (0 = a da
// frente). Nuvem e morro leem daqui, senão a nuvem nasce atrás da serra e
// ninguém vê nuvem nenhuma.
func relaxRgRidge(d int) (base, amp float64) {
	near := float64(relaxRgLayers-1-d) / float64(relaxRgLayers-1)
	return 0.40 + 0.42*near, 0.045 + 0.075*near
}

// relaxRgCrest é a altura do pico mais alto daquela cadeia.
func relaxRgCrest(d int) float64 {
	base, amp := relaxRgRidge(d)
	return base - amp*1.5
}

func relaxRgNewCloud(depth int, x float64) relaxRgCloudObj {
	// Longe = menor, mais alta e mais lenta. É a mesma nuvem vista de longe.
	f := 1 - float64(depth)/float64(relaxRgLayers)
	// A nuvem fica logo ABAIXO da crista do morro que está na frente dela: os
	// picos entram na frente e cortam pedaços da nuvem. É esse recorte que diz
	// que ela está dentro do vale, e não colada no céu.
	return relaxRgCloudObj{
		x: x, y: relaxRgCrest(depth) + 0.015 + rand.Float64()*0.035,
		w: (0.10 + rand.Float64()*0.18) * (0.45 + 0.75*f),
		h: (0.030 + rand.Float64()*0.030) * (0.55 + 0.60*f),
		v: (0.00045 + rand.Float64()*0.00075) * (0.35 + 0.9*f),
		// Nuvem sem semente própria respira toda no mesmo compasso, e aí o
		// céu inteiro pulsa junto — que é a coisa mais artificial possível.
		depth: depth, seed: rand.Intn(9999),
	}
}

func stepRelaxRidge(st *relaxRidgeState) {
	if !st.inited {
		st.inited = true
		for i := range st.ph {
			st.ph[i] = rand.Float64() * 2 * math.Pi
		}
		for d := 0; d < relaxRgLayers; d++ {
			for i, n := 0, 2+rand.Intn(2); i < n; i++ {
				st.clouds = append(st.clouds, relaxRgNewCloud(d, rand.Float64()*1.4-0.2))
			}
		}
		st.nextB = 120 + rand.Intn(300)
	}
	st.tick++
	st.mist += 0.004

	for i := range st.clouds {
		c := &st.clouds[i]
		if c.x += c.v; c.x-c.w > 1.15 {
			*c = relaxRgNewCloud(c.depth, -c.w-0.15)
		}
	}

	// Um bando de pássaros de vez em quando: a única coisa rápida da cena, e
	// ela dura poucos segundos justamente por isso.
	if len(st.birds) > 0 {
		for i := range st.birds {
			st.birds[i].x += st.birdV
			st.birds[i].y += math.Sin(float64(st.tick)*0.11+float64(i)) * 0.0012
		}
		if st.birds[0].x < -0.2 || st.birds[0].x > 1.2 {
			st.birds = nil
			st.nextB = 260 + rand.Intn(500)
		}
	} else if st.nextB--; st.nextB <= 0 {
		st.birdV = 0.006 + rand.Float64()*0.004
		x, y := -0.15, 0.14+rand.Float64()*0.22
		if rand.Intn(2) == 0 {
			st.birdV, x = -st.birdV, 1.15
		}
		for i, n := 0, 4+rand.Intn(5); i < n; i++ {
			st.birds = append(st.birds, relaxSkyPt{
				x: x - float64(i)*0.035*math.Copysign(1, st.birdV),
				y: y + float64(i%3)*0.018,
			})
		}
	}
}

func relaxRidgeFrames(st *relaxRidgeState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxRidge(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBrailleVote(w, h)
	relaxRidgeDraw(st, b, w, h)

	status := "as nuvens atravessam o vale"
	switch {
	case len(st.birds) > 0:
		status = "um bando passou"
	case st.tick%500 < 120:
		status = "a serra não vai a lugar nenhum"
	}
	return b.lines(relaxStyles(relaxRidgeRamp, fade)), StyleMuted.Render(status)
}

// relaxRgDrawCloud desenha uma nuvem como um bolo de bolhas com meio-tom: a borda
// esgarça em vez de cortar, que é o que separa nuvem de mancha.
func relaxRgDrawCloud(b *relaxBraille, c relaxRgCloudObj, sw, sh int, t float64, lvl int) {
	fw, fh := float64(sw), float64(sh)
	cx, cy := c.x*fw, c.y*fh
	rx, ry := c.w*fw, c.h*fh
	for i := 0; i < 7; i++ {
		s := float64(relaxHash(c.seed, i)%1000) / 1000
		// Bolhas ao longo do eixo, mais altas no meio: perfil de nuvem.
		bx := cx + (s*2-1)*rx
		lift := 1 - math.Abs(s*2-1)
		br := ry * (0.55 + 1.15*lift)
		by := cy - ry*lift*0.55 + math.Sin(t*0.5+float64(i)+float64(c.seed%7))*ry*0.10
		for dy := -int(br) - 1; dy <= int(br)+1; dy++ {
			for dx := -int(br*1.5) - 1; dx <= int(br*1.5)+1; dx++ {
				d := math.Hypot(float64(dx)/(br*1.5), float64(dy)/br)
				if d > 1 {
					continue
				}
				x, y := int(bx)+dx, int(by)+dy
				// Mais denso em cima: embaixo a nuvem se desfaz na sombra.
				dens := (1 - d*d) * (0.78 + 0.35*float64(-dy)/br)
				if relaxHalftone(x, y) > dens {
					continue
				}
				b.set(x, y, lvl)
			}
		}
	}
}

func relaxRidgeDraw(st *relaxRidgeState, b *relaxBraille, w, h int) {
	sw, sh := w*2, h*4
	fh := float64(sh)
	t := float64(st.tick) * 0.1

	// ── Pássaros ── na frente de tudo.
	for _, p := range st.birds {
		x, y := int(p.x*float64(sw)), int(p.y*fh)
		flap := int(math.Round(math.Sin(t*3+p.x*40) * 1.4))
		b.set(x, y, relaxRgBird)
		b.set(x-1, y-flap, relaxRgBird)
		b.set(x+1, y-flap, relaxRgBird)
	}

	// ── Névoa no fundo do vale ── faixa baixa e rala, escorrendo devagar.
	valley := int(fh * 0.80)
	for y := valley; y < sh; y++ {
		band := 1 - float64(y-valley)/float64(maxInt(1, sh-valley))
		for x := 0; x < sw; x++ {
			v := 0.30 + 0.28*math.Sin(float64(x)*0.035+st.mist+float64(y)*0.10)
			if relaxHalftone(x, y) > band*v {
				continue
			}
			b.set(x, y, relaxRgMist)
		}
	}

	// ── Cadeias e nuvens, alternando da frente pro fundo ──
	// A nuvem da profundidade d fica atrás do morro d e na frente do d+1: é
	// desenhar nessa ordem que põe cada uma no seu lugar.
	for d := 0; d < relaxRgLayers; d++ {
		base, amp := relaxRgRidge(d)
		lvl := relaxRgHill + relaxRgLayers - 1 - d
		k := d * 3
		for x := 0; x < sw; x++ {
			fx := float64(x)
			// Três harmônicas: um seno só entrega o desenho na hora.
			v := base - amp*(math.Abs(math.Sin(fx*0.0105+st.ph[k]))*0.85+
				0.45*math.Abs(math.Sin(fx*0.0261+st.ph[k+1]))+
				0.22*math.Sin(fx*0.0533+st.ph[k+2]))
			for y := int(v * fh); y < sh; y++ {
				b.set(x, y, lvl)
			}
		}
		for _, c := range st.clouds {
			if c.depth == d {
				// Perto = a nuvem que ainda pega o sol; longe = pálida.
				relaxRgDrawCloud(b, c, sw, sh, t, relaxRgCloud+relaxRgCloudN-1-minInt(d, relaxRgCloudN-1))
			}
		}
	}

	// ── Sol baixo ── quase no horizonte, atrás de tudo.
	sx, sy := float64(sw)*0.24, fh*0.30
	sr := math.Min(float64(sw), fh*2) * 0.05
	for dy := -int(sr); dy <= int(sr); dy++ {
		for dx := -int(sr); dx <= int(sr); dx++ {
			if float64(dx*dx+dy*dy) <= sr*sr {
				b.set(int(sx)+dx, int(sy)+dy, relaxRgSun)
			}
		}
	}

	// ── Céu ── cheio, porque aqui ele é a cor da cena e não o fundo: é fim de
	// tarde, o céu é a coisa mais clara do quadro.
	for y := 0; y < sh; y++ {
		lvl := relaxRgSky + minInt(int(float64(y)/fh*1.55*relaxRgSkyN), relaxRgSkyN-1)
		for x := 0; x < sw; x++ {
			b.set(x, y, lvl)
		}
	}
}
