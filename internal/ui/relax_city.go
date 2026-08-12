package ui

import (
	"math"
	"math/rand"
)

// ── Cidade dormindo ───────────────────────────────────────────────────────────
//
// Um recorte de prédios à noite. Quase nada acontece: uma janela acende, outra
// apaga, um carro atravessa a rua lá embaixo e um poste está com defeito.
//
// As janelas não piscam por sorteio a cada frame — cada uma tem um tempo próprio
// até a próxima troca, e a troca é um fade. É a diferença entre uma cidade e uma
// árvore de natal.

var relaxCityStops = []string{"#0B1120", "#1C2540", "#2E3D63", "#42568A"}
var relaxWinStops = []string{"#3A3418", "#6E6224", "#A89138", "#D8BE5C", "#F4E4A2"}

const (
	relaxCityWallN = 4
	relaxCityWinN  = 5
)

const (
	relaxCityWall = 0
	relaxCityWin  = relaxCityWallN
	relaxCitySky  = relaxCityWin + relaxCityWinN
	relaxCityStar = relaxCitySky + 6
	relaxCityMoon = relaxCityStar + 1
	relaxCityLamp = relaxCityMoon + 1
	relaxCityCar  = relaxCityLamp + 1
	relaxCityRoad = relaxCityCar + 1
	relaxCityTV   = relaxCityRoad + 1
)

var relaxCityRamp = func() []relaxColor {
	out := make([]relaxColor, relaxCityTV+1)
	copy(out[relaxCityWall:], relaxRamp(relaxCityStops, relaxCityWallN))
	copy(out[relaxCityWin:], relaxRamp(relaxWinStops, relaxCityWinN))
	copy(out[relaxCitySky:], relaxRamp([]string{"#0A0F1F", "#141C36", "#243256"}, 6))
	out[relaxCityStar] = "#B8C4E0"
	out[relaxCityMoon] = "#E8E4D0"
	out[relaxCityLamp] = "#F0C878"
	out[relaxCityCar] = "#FFF0C0"
	out[relaxCityRoad] = "#1B2130"
	out[relaxCityTV] = "#7EB4D8"
	return out
}()

type relaxWindow struct {
	x, y  int // subpixel do canto
	size  int
	lit   float64 // 0–1, com fade
	on    bool
	blue  bool // luz fria de televisão
	timer int
}

// relaxBuilding guarda também o remate do topo: prédio de silhueta toda igual
// é o que faz um horizonte parecer código, e não cidade.
type relaxBuilding struct {
	x, w, top int // subpixels
	back      bool
	roof      int // 0 reto · 1 recuo · 2 antena · 3 caixa d'água
	rw, rh    int // recuo/caixa: largura e altura
}

type relaxCityState struct {
	inited bool
	tick   int
	sw, sh int

	blds  []relaxBuilding
	wins  []relaxWindow
	stars []relaxSkyPt

	carX, carV float64 // carro: <0 quando não há
	nextCar    int
	lampX      []float64
	flickIdx   int
	flick      float64
}

func relaxCityBuild(st *relaxCityState, sw, sh int) {
	tick := st.tick
	*st = relaxCityState{inited: true, tick: tick, sw: sw, sh: sh, carX: -1}
	ground := int(float64(sh) * 0.82)

	// Duas fileiras: a de trás é mais baixa, sem janela, e só serve pra tirar o
	// horizonte da linha única.
	//
	// Os prédios são LARGOS de propósito. Nesta resolução, prédio estreito com
	// janelinha vira chuvisco: o olho precisa de massa pra ler silhueta, e de
	// janela grande e espaçada pra ler janela.
	for row := 0; row < 2; row++ {
		back := row == 0
		for x := -6 + rand.Intn(10); x < sw-6; {
			bw := 18 + rand.Intn(24)
			if back {
				bw = 26 + rand.Intn(30)
			}
			lo, hi := 0.15, 0.62
			if back {
				lo, hi = 0.11, 0.26
			}
			bh := int(float64(sh) * (lo + rand.Float64()*(hi-lo)))
			bd := relaxBuilding{x: x, w: bw, top: ground - bh, back: back}
			if !back {
				switch r := rand.Intn(10); {
				case r < 3:
					bd.roof, bd.rw, bd.rh = 1, bw/2+rand.Intn(bw/3+1), 3+rand.Intn(6) // recuo
				case r < 5:
					bd.roof, bd.rh = 2, 4+rand.Intn(9) // antena
				case r < 7:
					bd.roof, bd.rw, bd.rh = 3, 3+rand.Intn(3), 2+rand.Intn(3) // caixa d'água
				}
			}
			st.blds = append(st.blds, bd)

			if !back {
				// Cada prédio tem seu passo de janela: grade única em todos
				// entrega que o desenho é um laço.
				gx := 7 + rand.Intn(3)
				gy := 6 + rand.Intn(2)
				ww := 3 + rand.Intn(2)
				// Margem lateral igual dos dois lados: janela colada na quina
				// estraga a silhueta.
				cols := (bw - 4) / gx
				if cols < 1 {
					cols = 1
				}
				pad := (bw - cols*gx + gx - ww) / 2
				for wy := bd.top + 4; wy < ground-4; wy += gy {
					for c := 0; c < cols; c++ {
						// Só uma em cada cinco acesa: é o contraste que faz a
						// janela existir, não a quantidade.
						on := rand.Intn(5) == 0
						l := 0.0
						if on {
							l = 1
						}
						st.wins = append(st.wins, relaxWindow{
							x: x + pad + c*gx, y: wy, on: on, lit: l,
							timer: 40 + rand.Intn(900), size: ww,
							blue: rand.Intn(7) == 0, // uma ou outra é luz de TV
						})
					}
				}
			}
			x += bw + 2 + rand.Intn(7)
		}
	}

	for i, n := 0, 40+rand.Intn(30); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.7})
	}
	for i, n := 0, 3+rand.Intn(3); i < n; i++ {
		st.lampX = append(st.lampX, 0.08+rand.Float64()*0.84)
	}
	st.flickIdx = rand.Intn(maxInt(1, len(st.lampX)))
	st.nextCar = 60 + rand.Intn(160)
}

func stepRelaxCity(st *relaxCityState) {
	if !st.inited {
		relaxCityBuild(st, 160, 88)
	}
	st.tick++

	for i := range st.wins {
		wn := &st.wins[i]
		if wn.timer--; wn.timer <= 0 {
			wn.on = !wn.on
			// Apagar é mais comum que acender: a cidade está indo dormir.
			wn.timer = 250 + rand.Intn(1400)
			if !wn.on {
				wn.timer /= 2
			}
		}
		target := 0.0
		if wn.on {
			target = 1
		}
		wn.lit = smoothDamp(wn.lit, target, 0.35, 0.1)
	}

	// Poste com mau contato: apaga e volta em rajadas curtas.
	st.flick = 1
	if (st.tick/3)%37 < 3 && st.tick%2 == 0 {
		st.flick = 0.15
	}

	if st.carX >= 0 {
		st.carX += st.carV
		if st.carX < -0.12 || st.carX > 1.12 {
			st.carX = -1
		}
	} else if st.nextCar--; st.nextCar <= 0 {
		st.carV = 0.0075 + rand.Float64()*0.004
		st.carX = -0.10
		if rand.Intn(2) == 0 {
			st.carV, st.carX = -st.carV, 1.10
		}
		st.nextCar = 200 + rand.Intn(420)
	}
}

// relaxCityDrawBld põe o corpo e o remate do prédio.
func relaxCityDrawBld(b *relaxBraille, bd relaxBuilding, ground, sw int) {
	body := relaxCityWall + 1
	if bd.back {
		body = relaxCityWall // fileira de trás é um tom mais escura
	}
	cx := bd.x + bd.w/2
	switch bd.roof {
	case 1: // recuo: um bloco menor centrado no topo
		for x := cx - bd.rw/2; x <= cx+bd.rw/2; x++ {
			for y := bd.top - bd.rh; y < bd.top; y++ {
				b.set(x, y, body)
			}
		}
	case 2: // antena com luz de sinalização na ponta
		for y := bd.top - bd.rh; y < bd.top; y++ {
			b.set(cx, y, body+1)
		}
		b.set(cx, bd.top-bd.rh, relaxCityLamp)
	case 3: // caixa d'água sobre pernas
		for x := cx - bd.rw; x <= cx+bd.rw; x++ {
			for y := bd.top - bd.rh - 2; y < bd.top-2; y++ {
				b.set(x, y, body+1)
			}
		}
		b.set(cx-bd.rw+1, bd.top-2, body)
		b.set(cx+bd.rw-1, bd.top-2, body)
	}
	for x := maxInt(0, bd.x); x < minInt(sw, bd.x+bd.w); x++ {
		lvl := body
		if x < bd.x+2 {
			lvl = body + 1 // quina iluminada
		}
		for y := bd.top; y < ground; y++ {
			b.set(x, y, lvl)
		}
	}
}

func relaxCityFrames(st *relaxCityState, width, height int, fade float64) ([]string, string) {
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	if !st.inited || st.sw != w*2 || st.sh != h*4 {
		relaxCityBuild(st, w*2, h*4)
	}
	b := newRelaxBrailleVote(w, h)
	relaxCityDraw(st, b, w, h)
	status := "a cidade está dormindo"
	if st.carX >= 0 {
		status = "um carro passa"
	}
	return b.lines(relaxStyles(relaxCityRamp, fade)), StyleMuted.Render(status)
}

func relaxCityDraw(st *relaxCityState, b *relaxBraille, w, h int) {
	sw, sh := st.sw, st.sh
	ground := int(float64(sh) * 0.82)
	t := float64(st.tick) * 0.1

	// Carro: faróis e o clarão que eles jogam no asfalto.
	if st.carX >= 0 {
		cx := st.carX * float64(sw)
		cy := float64(ground) + float64(sh)*0.07
		for dx := -3; dx <= 3; dx++ {
			b.set(int(cx)+dx, int(cy), relaxCityCar)
			b.set(int(cx)+dx, int(cy)-1, relaxCityCar)
		}
		dir := 1.0
		if st.carV < 0 {
			dir = -1
		}
		for k := 2.0; k < 26; k += 0.5 {
			spread := k * 0.22
			for s := -spread; s <= spread; s += 1 {
				if relaxHalftone(int(cx+dir*k), int(cy+s)) < 1-k/26 {
					b.set(int(cx+dir*k), int(cy+s+2), relaxCityLamp)
				}
			}
		}
	}

	// Postes: haste, luminária e o cone de luz na calçada.
	for i, lx := range st.lampX {
		x := int(lx * float64(sw))
		amp := 1.0
		if i == st.flickIdx {
			amp = st.flick
		}
		for y := ground; y < ground+int(float64(sh)*0.06); y++ {
			b.set(x, y, relaxCityWall+1)
		}
		if amp > 0.4 {
			for dx := -1; dx <= 1; dx++ {
				b.set(x+dx, ground-1, relaxCityLamp)
			}
			// Poça de luz: elipse achatada no asfalto, esgarçando na borda.
			prx, pry := 11.0, 4.0
			for dy := -int(pry); dy <= int(pry); dy++ {
				for dx := -int(prx); dx <= int(prx); dx++ {
					d := math.Hypot(float64(dx)/prx, float64(dy)/pry)
					if d > 1 {
						continue
					}
					if relaxHalftone(x+dx, ground+3+dy) > (1-d*d)*0.6*amp {
						continue
					}
					b.set(x+dx, ground+3+dy, relaxCityLamp)
				}
			}
		}
	}

	// Janelas acesas (as apagadas somem na parede).
	for _, wn := range st.wins {
		if wn.lit < 0.06 {
			continue
		}
		lvl := relaxCityWin + minInt(int(wn.lit*float64(relaxCityWinN)), relaxCityWinN-1)
		if wn.blue {
			lvl = relaxCityTV
		}
		// Janela é retangular: mais larga que alta, como janela de prédio.
		for dy := 0; dy < wn.size-1; dy++ {
			for dx := 0; dx < wn.size; dx++ {
				b.set(wn.x+dx, wn.y+dy, lvl)
			}
		}
	}

	// Prédios: os da frente primeiro, pra taparem os de trás.
	for _, bd := range st.blds {
		if bd.back {
			continue
		}
		relaxCityDrawBld(b, bd, ground, sw)
	}
	for _, bd := range st.blds {
		if bd.back {
			relaxCityDrawBld(b, bd, ground, sw)
		}
	}

	// Rua.
	for y := ground; y < sh; y++ {
		for x := 0; x < sw; x++ {
			b.set(x, y, relaxCityRoad)
		}
	}

	// Lua e estrelas.
	mx, my := float64(sw)*0.78, float64(sh)*0.14
	mr := float64(minInt(sw, sh)) * 0.055
	for dy := -int(mr / 2); dy <= int(mr/2); dy++ {
		for dx := -int(mr); dx <= int(mr); dx++ {
			nx, ny := float64(dx)/mr, float64(dy)/(mr/2)
			if nx*nx+ny*ny <= 1 {
				b.set(int(mx)+dx, int(my)+dy, relaxCityMoon)
			}
		}
	}
	for i, p := range st.stars {
		x, y := int(p.x*float64(sw-1)), int(p.y*float64(sh-1))
		if relaxHalftone(x, y) < 0.30+0.16*math.Sin(t*0.5+float64(i)) {
			b.set(x, y, relaxCityStar)
		}
	}

	// Céu por último, e NÃO preenchido: com o céu cheio de pontos a tela toda
	// ficava com a mesma textura e a silhueta dos prédios sumia. Vazio em cima,
	// meio-tom clareando perto do horizonte — que é onde a cidade suja o céu.
	for y := 0; y < ground; y++ {
		f := float64(y) / float64(maxInt(1, ground))
		lvl := relaxCitySky + minInt(int(f*6), 5)
		for x := 0; x < sw; x++ {
			if relaxHalftone(x, y) > f*f*0.55 {
				continue
			}
			b.set(x, y, lvl)
		}
	}

}
