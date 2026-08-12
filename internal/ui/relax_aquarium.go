package ui

import (
	"math"
	"math/rand"
)

// ── Aquário ───────────────────────────────────────────────────────────────────
//
// Peixes indo e vindo sem pressa, alga balançando, bolha subindo e feixes de luz
// atravessando a água. Cada peixe é desenhado por partes — corpo, cauda que bate,
// barbatana, olho —, então a batida da cauda é ângulo, não troca de sprite.

var relaxFishStops = [3][]string{
	{"#5A2A0C", "#A85418", "#E08A2E", "#F7C46A"}, // laranja
	{"#4A4410", "#96881E", "#D6C43A", "#F2E88C"}, // amarelo
	{"#0E2C4A", "#1D5A8E", "#3D93CE", "#8CC9EE"}, // azul
}

const relaxFishShades = 4

const (
	relaxAqFish  = 0
	relaxAqSand  = 3 * relaxFishShades
	relaxAqWeed  = relaxAqSand + 2
	relaxAqBub   = relaxAqWeed + 2
	relaxAqShaft = relaxAqBub + 1
	relaxAqEye   = relaxAqShaft + 2
)

var relaxAqRamp = func() []relaxColor {
	out := make([]relaxColor, relaxAqEye+1)
	for i, stops := range relaxFishStops {
		copy(out[i*relaxFishShades:], relaxRamp(stops, relaxFishShades))
	}
	out[relaxAqSand] = "#4A3F2C"
	out[relaxAqSand+1] = "#7A6A4C"
	out[relaxAqWeed] = "#123A2A"
	out[relaxAqWeed+1] = "#2E6E46"
	out[relaxAqBub] = "#A8D8E8"
	out[relaxAqShaft] = "#12303F"
	out[relaxAqShaft+1] = "#1C4658"
	out[relaxAqEye] = "#0A0C10"
	return out
}()

type relaxFish struct {
	x, y    float64 // unidades de projeto
	vx, bob float64
	phase   float64
	size    float64
	species int
	depth   int // 0 fundo · 1 frente
}

type relaxAquariumState struct {
	inited  bool
	tick    int
	fish    []relaxFish
	weeds   []float64 // posição x de cada alga
	bubbles []relaxSkyPt
	nextBub int
}

const (
	relaxAqW = 68.0
	relaxAqH = 34.0
)

func relaxAqInit(st *relaxAquariumState) {
	st.inited = true
	st.fish = st.fish[:0]
	for i, n := 0, 6+rand.Intn(4); i < n; i++ {
		dir := 1.0
		if rand.Intn(2) == 0 {
			dir = -1
		}
		depth := rand.Intn(2)
		st.fish = append(st.fish, relaxFish{
			x:       rand.Float64() * relaxAqW,
			y:       5 + rand.Float64()*20,
			vx:      dir * (0.16 + rand.Float64()*0.22) * (0.7 + 0.5*float64(depth)),
			bob:     rand.Float64() * 6.28,
			phase:   rand.Float64() * 6.28,
			size:    (0.7 + rand.Float64()*0.6) * (0.75 + 0.35*float64(depth)),
			species: rand.Intn(3),
			depth:   depth,
		})
	}
	st.weeds = st.weeds[:0]
	for i, n := 0, 4+rand.Intn(4); i < n; i++ {
		st.weeds = append(st.weeds, 5+rand.Float64()*(relaxAqW-10))
	}
	st.nextBub = 6
}

func stepRelaxAquarium(st *relaxAquariumState) {
	if !st.inited {
		relaxAqInit(st)
	}
	st.tick++
	t := float64(st.tick) * 0.1

	for i := range st.fish {
		f := &st.fish[i]
		f.x += f.vx
		f.y += 0.055 * math.Sin(t*0.7+f.bob)
		// Vira ao chegar na parede, com uma folga fora do quadro pra não
		// parecer que bateu no vidro.
		if (f.vx > 0 && f.x > relaxAqW+6) || (f.vx < 0 && f.x < -6) {
			f.vx = -f.vx
			f.y = 5 + rand.Float64()*20
		}
		f.y = clampF(f.y, 4, 26)
	}

	if st.nextBub--; st.nextBub <= 0 && len(st.weeds) > 0 {
		st.bubbles = append(st.bubbles, relaxSkyPt{
			x: st.weeds[rand.Intn(len(st.weeds))] + (rand.Float64()-0.5)*2,
			y: relaxAqH - 5,
		})
		st.nextBub = 5 + rand.Intn(14)
	}
	keep := st.bubbles[:0]
	for _, p := range st.bubbles {
		p.y -= 0.30 + 0.10*math.Sin(p.x)
		p.x += 0.09 * math.Sin(t*1.6+p.y*0.4)
		if p.y > 1 {
			keep = append(keep, p)
		}
	}
	st.bubbles = keep
}

func relaxAquariumFrames(st *relaxAquariumState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxAquarium(st)
	}
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBrailleVote(w, h)
	sw, sh := float64(w*2), float64(h*4)
	kx, ky := sw/relaxAqW, sh/relaxAqH
	X := func(v float64) float64 { return v * kx }
	Y := func(v float64) float64 { return v * ky }
	t := float64(st.tick) * 0.1

	// Feixes de luz: faixas inclinadas que oscilam devagar, bem apagadas. O
	// valor depende só de x·0,55+y, então uma tabela nessa diagonal basta e o
	// laço de 20 mil subpixels não chama seno nenhuma vez.
	nd := int(sw*0.55+sh) + 2
	shaft := make([]float64, nd)
	for d := 0; d < nd; d++ {
		fd := float64(d)
		shaft[d] = (math.Sin(fd*0.045+t*0.16)+0.6*math.Sin(fd*0.017-t*0.09))/1.6*0.5 + 0.5
	}
	for x := 0; x < int(sw); x++ {
		for y := 0; y < int(sh); y++ {
			v := shaft[int(float64(x)*0.55)+y]
			v *= 1 - float64(y)/sh*0.8 // a luz vem de cima
			if v < 0.72 || relaxHalftone(x, y) > (v-0.72)*2.4 {
				continue
			}
			lvl := relaxAqShaft
			if v > 0.86 {
				lvl = relaxAqShaft + 1
			}
			b.set(x, y, lvl)
		}
	}

	drawFish := func(f relaxFish) {
		dir := 1.0
		if f.vx < 0 {
			dir = -1
		}
		fx, fy := X(f.x), Y(f.y)
		rx, ry := X(2.6*f.size), Y(1.5*f.size)
		base := f.species * relaxFishShades
		// Batida da cauda: a mesma fase move cauda e barbatana.
		flap := math.Sin(t*4.4/(0.6+f.size) + f.phase)

		// Cauda: triângulo que abre e fecha atrás do corpo.
		tipX := fx - dir*(rx+X(2.4*f.size))
		tipY := fy + Y(1.1*f.size)*flap
		b.tri(fx-dir*rx*0.7, fy, tipX, tipY-Y(1.3*f.size), tipX, tipY+Y(1.3*f.size), base+1)
		// Barbatana dorsal.
		b.tri(fx-dir*rx*0.2, fy-ry*0.8, fx+dir*rx*0.3, fy-ry*0.8,
			fx-dir*rx*0.5, fy-ry-Y(1.2*f.size)*(0.6+0.4*flap), base+1)

		for y := int(fy - ry); y <= int(fy+ry)+1; y++ {
			for x := int(fx - rx); x <= int(fx+rx)+1; x++ {
				nx, ny := (float64(x)-fx)/rx, (float64(y)-fy)/ry
				// Corpo de peixe é uma gota: mais cheio na frente.
				if nx*nx/(1+0.35*dir*nx)+ny*ny > 1 {
					continue
				}
				lvl := base + 1 + int(clamp01(0.62-0.55*ny)*float64(relaxFishShades-2)+0.5)
				b.set(x, y, minInt(lvl, base+relaxFishShades-1))
			}
		}
		b.set(int(fx+dir*rx*0.55), int(fy-ry*0.3), relaxAqEye)
	}

	for _, f := range st.fish {
		if f.depth == 0 {
			drawFish(f)
		}
	}

	// Algas: fita que balança mais quanto mais longe da raiz.
	for i, wx := range st.weeds {
		hgt := 9.0 + 5*math.Sin(float64(i)*2.3)
		for s := 0.0; s < 1; s += 0.01 {
			sway := math.Sin(t*0.5+float64(i)*1.7+s*2.4) * s * s * 3.4
			x := X(wx + sway)
			y := Y(relaxAqH - 4 - s*hgt)
			half := X(0.55 * (1 - s*0.5))
			for dx := -int(half); dx <= int(half); dx++ {
				lvl := relaxAqWeed
				if dx < 0 {
					lvl = relaxAqWeed + 1
				}
				b.set(int(x)+dx, int(y), lvl)
			}
		}
	}

	for _, f := range st.fish {
		if f.depth == 1 {
			drawFish(f)
		}
	}

	for _, p := range st.bubbles {
		x, y := int(X(p.x)), int(Y(p.y))
		b.set(x, y, relaxAqBub)
		b.set(x+1, y, relaxAqBub)
		b.set(x, y+1, relaxAqBub)
	}

	// Areia: relevo baixo com grão.
	for x := 0; x < int(sw); x++ {
		top := Y(relaxAqH-3.6) + Y(0.9)*math.Sin(float64(x)*0.05) + Y(0.4)*math.Sin(float64(x)*0.17)
		for y := int(top); y < int(sh); y++ {
			lvl := relaxAqSand
			if float64(y) < top+2 || relaxHalftone(x, y) > 0.72 {
				lvl = relaxAqSand + 1
			}
			b.set(x, y, lvl)
		}
	}

	return b.lines(relaxStyles(relaxAqRamp, fade)), StyleMuted.Render("ninguém tem pressa aqui")
}
