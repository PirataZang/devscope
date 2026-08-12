package ui

import (
	"math"
	"math/rand"
)

// ── Café ──────────────────────────────────────────────────────────────────────
//
// Uma xícara vista de cima, no pires. O ciclo é uma cena pequena: o bule entra,
// inclina e enche; sai; e aí o nível vai baixando em goles, com uma pausa entre
// eles — é a pausa que faz parecer alguém bebendo, e não um ralo aberto.
//
// Ver de cima é o que exige o interior: entre a borda e o café aparece a parede
// de dentro da xícara, em sombra. Quanto menos café, mais parede — e é só isso
// que comunica o nível, já que de cima não há linha d'água visível de perfil.

var relaxCupStops = []string{"#26231F", "#413C35", "#635C51", "#8C8478", "#B5AC9E", "#D8D0C2", "#F2ECE0"}
var relaxBrewStops = []string{"#140A04", "#2C1609", "#4E2810", "#7A431C", "#A8642C", "#C98A45"}
var relaxPotStops = []string{"#1B1E24", "#2E343E", "#49515E", "#6B7484", "#98A2B2"}

const (
	relaxCupLevels  = 10
	relaxBrewLevels = 7
	relaxPotLevels  = 6
	relaxSteamLvls  = 4
)

const (
	relaxCup   = 0
	relaxBrew  = relaxCupLevels
	relaxPot   = relaxBrew + relaxBrewLevels
	relaxSteam = relaxPot + relaxPotLevels
)

var relaxCoffeeRamp = func() []relaxColor {
	out := make([]relaxColor, relaxSteam+relaxSteamLvls)
	copy(out[relaxCup:], relaxRamp(relaxCupStops, relaxCupLevels))
	copy(out[relaxBrew:], relaxRamp(relaxBrewStops, relaxBrewLevels))
	copy(out[relaxPot:], relaxRamp(relaxPotStops, relaxPotLevels))
	copy(out[relaxSteam:], relaxRamp([]string{"#2E3238", "#565C64", "#8C939C", "#CAD0D8"}, relaxSteamLvls))
	return out
}()

// O bule despeja de cima e de lado: parado em relaxPotPour, o bico fica logo
// acima do centro da xícara. Inclinação positiva é o bico descendo.
const (
	relaxPotAway = -22.0
	relaxPotPour = 20.0
	relaxPotTilt = 0.62
)

type relaxCoffeePhase int

const (
	coffeeEmpty relaxCoffeePhase = iota
	coffeePotIn
	coffeePour
	coffeePotOut
	coffeeFull
	coffeeSip
)

// relaxCoffeeRipple é a onda que sai do ponto onde o jato bate — e também a que
// o gole deixa quando o nível cai de repente.
type relaxCoffeeRipple struct {
	x, r, amp float64
}

type relaxCoffeeState struct {
	inited bool
	tick   int
	phase  relaxCoffeePhase
	t      int
	dur    int

	level  float64 // 0 vazia · 1 cheia
	target float64
	potX   float64 // posição do bule em unidades de projeto
	tilt   float64
	heat   float64 // vapor: forte logo depois de encher, some esfriando

	sips    int
	sipWait int
	ripples []relaxCoffeeRipple
	seed    [3]float64

	foam  float64 // espuma da servida, some em poucos segundos
	stain float64 // até onde o café já chegou: a marca que fica na louça
	dropY float64 // última gota do bico (<0 = nenhuma)
	dropV float64
}

// Duração de cada etapa, em passos de 100ms.
func relaxCoffeeDur(p relaxCoffeePhase) int {
	switch p {
	case coffeePotIn:
		return 26
	case coffeePour:
		return 42
	case coffeePotOut:
		return 24
	case coffeeFull:
		return 45
	case coffeeSip:
		return 260
	default:
		return 28
	}
}

func relaxCoffeeNext(st *relaxCoffeeState) {
	st.phase = (st.phase + 1) % 6
	st.t = 0
	st.dur = relaxCoffeeDur(st.phase)
	if st.phase == coffeeSip {
		st.sips = 4 + rand.Intn(3)
		st.sipWait = 18 + rand.Intn(14)
	}
}

func stepRelaxCoffee(st *relaxCoffeeState) {
	if !st.inited {
		st.inited = true
		for i := range st.seed {
			st.seed[i] = rand.Float64() * 10
		}
		st.phase, st.dur, st.potX = coffeeEmpty, relaxCoffeeDur(coffeeEmpty), relaxPotAway
	}
	st.tick++
	st.t++
	p := float64(st.t) / float64(maxInt(1, st.dur))

	switch st.phase {
	case coffeeEmpty:
		st.target, st.potX, st.tilt = 0, relaxPotAway, 0
	case coffeePotIn:
		st.potX = lerp(relaxPotAway, relaxPotPour, easeInOutSine(p))
		st.tilt = relaxPotTilt * easeInOut(clamp01((p-0.55)/0.45))
	case coffeePour:
		st.potX, st.tilt = relaxPotPour, relaxPotTilt
		st.target = clamp01(p * 1.06)
		st.foam = 1
		if st.tick%3 == 0 {
			st.ripples = append(st.ripples, relaxCoffeeRipple{x: 0, amp: 0.9})
		}
		st.heat = 1
	case coffeePotOut:
		// A última gota: sai do bico quando ele começa a levantar.
		if st.t == 2 {
			st.dropY, st.dropV = 0, 0
		}
		st.tilt = relaxPotTilt * (1 - easeInOut(clamp01(p/0.4)))
		st.potX = lerp(relaxPotPour, relaxPotAway, easeInOutSine(clamp01((p-0.25)/0.75)))
	case coffeeFull:
		st.potX, st.tilt = relaxPotAway, 0
	case coffeeSip:
		// Um gole por vez, com pausa entre eles: é a pausa que faz parecer
		// alguém bebendo em vez de a xícara vazar.
		if st.sips > 0 {
			if st.sipWait--; st.sipWait <= 0 {
				st.sips--
				st.target = math.Max(0, st.target-1.0/float64(st.sips+1))
				st.ripples = append(st.ripples, relaxCoffeeRipple{x: 0, amp: 1.3})
				st.sipWait = 26 + rand.Intn(26)
			}
		} else if st.t > st.dur/2 {
			st.target = 0
		}
	}
	if st.t >= st.dur && !(st.phase == coffeeSip && st.level > 0.02) {
		relaxCoffeeNext(st)
	}

	st.level = smoothDamp(st.level, st.target, 0.22, 0.1)
	st.heat = math.Max(st.heat*0.9955, st.level*0.35)
	st.foam *= 0.982
	// A marca de maré: sobe com o café e nunca desce sozinha — só some quando
	// a xícara é servida de novo.
	if st.level > st.stain {
		st.stain = st.level
	}
	if st.phase == coffeePour {
		st.stain = st.level
	}
	if st.dropY >= 0 {
		st.dropV += 0.16
		st.dropY += st.dropV
		if st.dropY > 22 {
			st.dropY = -1
			st.ripples = append(st.ripples, relaxCoffeeRipple{amp: 0.6})
		}
	}

	keep := st.ripples[:0]
	for _, r := range st.ripples {
		r.r += 0.55
		r.amp *= 0.88
		if r.amp > 0.05 {
			keep = append(keep, r)
		}
	}
	st.ripples = keep
}

func relaxCoffeeFrames(st *relaxCoffeeState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxCoffee(st)
	}
	w := maxInt(26, minInt(width, 100))
	h := maxInt(8, minInt(height, 28))
	b := newRelaxBrailleVote(w, h)
	relaxCoffeeDraw(st, b, w, h)

	status := "a xícara está vazia"
	switch st.phase {
	case coffeePotIn, coffeePotOut:
		status = "o bule chega"
	case coffeePour:
		status = "enchendo"
	case coffeeFull:
		status = "café servido"
	case coffeeSip:
		status = "alguém está bebendo"
	}
	return b.lines(relaxStyles(relaxCoffeeRamp, fade)), StyleMuted.Render(status)
}

func relaxCoffeeDraw(st *relaxCoffeeState, b *relaxBraille, w, h int) {
	sw, sh := float64(w*2), float64(h*4)
	sc := math.Min(sw/68, sh/40)
	ox, oy := (sw-68*sc)/2, (sh-40*sc)/2
	X := func(v float64) float64 { return ox + v*sc }
	Y := func(v float64) float64 { return oy + v*sc }
	L := func(v float64) float64 { return v * sc }
	t := float64(st.tick) * 0.1

	// Vista de cima: a boca é quase um círculo achatado, o pires é bem maior.
	const (
		cupX    = 32.0
		mouthY  = 16.0
		mouthRX = 12.6
		mouthRY = 6.6 // razão 0,52: bem mais aberta que de perfil
		baseY   = 24.0
		baseRX  = 9.2
	)
	// Superfície do café: sobe e alarga com o nível, porque a xícara é cônica.
	// Os limites são escolhidos pra ela nunca sair da boca — de cima, café que
	// vaza pra fora da elipse não parece café, parece mancha.
	surfY := lerp(baseY-1.6, mouthY+1.0, st.level)
	surfRX := lerp(baseRX*0.82, mouthRX*0.88, st.level)
	surfRY := surfRX * mouthRY / mouthRX

	ellClip := func(cx, cy, rx, ry float64, in func(x, y int) bool, lvl func(nx, ny float64) int) {
		for y := int(cy - ry); y <= int(cy+ry)+1; y++ {
			for x := int(cx - rx); x <= int(cx+rx)+1; x++ {
				nx, ny := (float64(x)-cx)/rx, (float64(y)-cy)/ry
				if nx*nx+ny*ny > 1 || (in != nil && !in(x, y)) {
					continue
				}
				b.set(x, y, lvl(nx, ny))
			}
		}
	}
	ell := func(cx, cy, rx, ry float64, lvl func(nx, ny float64) int) {
		ellClip(cx, cy, rx, ry, nil, lvl)
	}

	// ── Vapor ── sai da superfície, e só enquanto há calor.
	if st.heat > 0.06 {
		for i := 0; i < 3; i++ {
			x0 := X(cupX - 6 + float64(i)*6)
			ph := st.seed[i]*6 + t*0.55
			for s := 0.0; s < 1; s += 0.007 {
				amp := L(1.0 + 4.6*s*s)
				x := x0 + amp*math.Sin(s*3.4+ph) + L(2.0)*s*math.Sin(ph*0.31)
				y := Y(surfY) - L(1.5) - s*L(15)
				str := st.heat * (1 - s) * (0.55 + 0.45*math.Sin(ph*0.7+s*7))
				r := L(0.55 + 1.2*s)
				for dy := -int(r) - 1; dy <= int(r)+1; dy++ {
					for dx := -int(r) - 1; dx <= int(r)+1; dx++ {
						d := math.Hypot(float64(dx)/r, float64(dy)/r)
						if d > 1 {
							continue
						}
						v := str * (1 - d*d)
						ix, iy := int(x)+dx, int(y)+dy
						if v <= relaxHalftone(ix, iy)*1.15 {
							continue
						}
						b.set(ix, iy, relaxSteam+minInt(int(v*float64(relaxSteamLvls)), relaxSteamLvls-1))
					}
				}
			}
		}
	}

	// ── Bule e jato ── na frente de tudo.
	if st.potX > relaxPotAway+1 {
		spx, spy := relaxCoffeePot(b, X, Y, L, st.potX, st.tilt)
		if st.phase == coffeePour {
			for s := 0.0; s < 1; s += 0.004 {
				y := lerp(spy, Y(surfY), s)
				// O fio afina e oscila de leve na queda.
				x := lerp(spx, X(cupX+1), s*s) + L(0.5)*math.Sin(t*7+s*9)
				b.set(int(x), int(y), relaxBrew+relaxBrewLevels-2)
				b.set(int(x)+1, int(y), relaxBrew+relaxBrewLevels-4)
			}
		}
	}

	// ── Borda da xícara ── anel de cerâmica, o mais claro da cena.
	for a := 0.0; a < 6.29; a += 0.003 {
		ca, sa := math.Cos(a), math.Sin(a)
		for k := 0.0; k <= 1.0; k += 0.16 {
			rk := 1 + k*0.075
			// A borda de trás pega mais luz que a da frente.
			lvl := relaxCupLevels - 1
			if sa > 0.2 {
				lvl = relaxCupLevels - 4
			}
			b.set(int(X(cupX)+L(mouthRX*rk)*ca), int(Y(mouthY)+L(mouthRY*rk)*sa), lvl)
		}
	}

	// Gota final caindo do bico.
	if st.dropY >= 0 && st.potX > relaxPotAway+1 {
		dx, dy := X(cupX+1), Y(11)+L(st.dropY)
		for k := -1; k <= 1; k++ {
			b.set(int(dx)+k, int(dy), relaxBrew+relaxBrewLevels-2)
			b.set(int(dx), int(dy)+k, relaxBrew+relaxBrewLevels-2)
		}
	}

	// ── Café ── com crema girando e as ondas do jato/gole.
	if st.level > 0.015 {
		// Recorte pela boca: com pouco café a borda da frente esconde parte da
		// superfície, que é o que se vê de verdade numa xícara quase vazia.
		mx, my := X(cupX), Y(mouthY)
		mrx, mry := L(mouthRX*0.955), L(mouthRY*0.955)
		ellClip(X(cupX), Y(surfY), L(surfRX), L(surfRY), func(x, y int) bool {
			nx, ny := (float64(x)-mx)/mrx, (float64(y)-my)/mry
			return nx*nx+ny*ny <= 1
		}, func(nx, ny float64) int {
			rr := math.Hypot(nx, ny)
			ang := math.Atan2(ny, nx)
			v := 0.30 + 0.70*clamp01(rr*rr+0.30*math.Sin(ang*3+t*0.5+rr*4))
			for _, rp := range st.ripples {
				// Onda concêntrica saindo do ponto de impacto.
				v += rp.amp * 0.5 * math.Cos((rr-rp.r/mouthRX)*9) * clamp01(1-math.Abs(rr-rp.r/mouthRX)*2.2)
			}
			// Espuma da servida: anel de bolhas na borda, que estoura devagar.
			if st.foam > 0.05 && rr > 0.62 {
				if relaxHalftone(int(nx*40), int(ny*40)) < st.foam*(rr-0.62)*2.6 {
					return relaxBrew + relaxBrewLevels - 1
				}
			}
			return relaxBrew + minInt(maxInt(int(v*float64(relaxBrewLevels-1)+0.5), 0), relaxBrewLevels-1)
		})
	}

	// Marca de maré: um anel escuro na louça onde o café já esteve. Só aparece
	// quando o nível baixou, que é justamente quando ela ficaria à mostra.
	if st.stain > st.level+0.04 {
		sy := lerp(baseY-1.6, mouthY+1.0, st.stain)
		sr := lerp(baseRX*0.82, mouthRX*0.88, st.stain)
		for a := 0.0; a < 6.29; a += 0.004 {
			ca, sa := math.Cos(a), math.Sin(a)
			if sa < -0.15 {
				continue // só a parte do fundo da parede fica visível
			}
			b.set(int(X(cupX)+L(sr)*ca), int(Y(sy)+L(sr*mouthRY/mouthRX)*sa), relaxBrew+1)
		}
	}

	// ── Parede de dentro ── o que sobra da boca acima do café.
	ell(X(cupX), Y(mouthY), L(mouthRX*0.955), L(mouthRY*0.955), func(nx, ny float64) int {
		// Mais fundo, mais escuro; a parede do fundo pega um pouco de luz.
		return relaxCup + minInt(maxInt(int(2.4-1.6*ny), 0), 4)
	})

	// ── Corpo ── entre a boca e a base, com a luz vindo da esquerda.
	for y := int(Y(mouthY)); y <= int(Y(baseY)); y++ {
		f := (float64(y) - Y(mouthY)) / (Y(baseY) - Y(mouthY))
		half := L(lerp(mouthRX, baseRX, f))
		// A parede só aparece abaixo da elipse da boca.
		for x := int(X(cupX) - half); x <= int(X(cupX)+half); x++ {
			nx := (float64(x) - X(cupX)) / half
			if 1-nx*nx > 0 && float64(y) < Y(mouthY)+L(mouthRY)*math.Sqrt(1-nx*nx) {
				continue
			}
			lvl := int(clamp01(0.74-0.40*nx-0.20*f)*float64(relaxCupLevels-1) + 0.5)
			b.set(x, y, minInt(maxInt(lvl, 1), relaxCupLevels-1))
		}
	}

	// ── Asa ── à direita, um anel achatado.
	for a := -1.30; a < 1.30; a += 0.004 {
		for k := 0.0; k <= 1.0; k += 0.28 {
			rr := L(4.6 + k*1.9)
			lvl := relaxCupLevels - 3
			if k > 0.6 {
				lvl = 2
			}
			b.set(int(X(cupX+11.5)+rr*math.Cos(a)), int(Y(21.5)+rr*math.Sin(a)*0.8), lvl)
		}
	}

	// ── Colher ── no pires, à frente e à direita da xícara.
	{
		sx, sy := X(cupX+13.5), Y(27.5)
		ell(sx, sy, L(3.4), L(1.9), func(nx, ny float64) int {
			return relaxCupLevels - 2 - minInt(int(1.6+1.4*ny), 3)
		})
		for k := 0.0; k <= 1; k += 0.02 {
			hx, hy := lerp(sx+L(3.0), sx+L(11.0), k), lerp(sy+L(0.4), sy+L(2.6), k)
			half := L(lerp(0.9, 0.5, k))
			for d := -half; d <= half; d += 0.7 {
				b.set(int(hx), int(hy+d), relaxCupLevels-3)
			}
		}
	}

	// ── Sombra ── da xícara no pires: sem ela a xícara flutua.
	ell(X(cupX+1.6), Y(26.6), L(11.5), L(4.6), func(nx, ny float64) int {
		return relaxCup + 1
	})

	// ── Pires ── por último: fica atrás de tudo.
	ell(X(cupX), Y(26.0), L(18.5), L(9.4), func(nx, ny float64) int {
		rr := math.Hypot(nx, ny)
		if rr < 0.52 {
			return relaxCup + 2 // a cavidade onde a xícara assenta
		}
		return relaxCup + minInt(maxInt(int(6.5-2.6*ny-1.2*nx), 1), relaxCupLevels-1)
	})

}

// relaxCoffeePot desenha o bule inclinado em torno do próprio corpo e devolve
// a ponta do bico, que é de onde o café cai.
func relaxCoffeePot(b *relaxBraille, X, Y, L func(float64) float64, px, tilt float64) (float64, float64) {
	cx, cy := X(px), Y(4.5)
	sn, cs := math.Sin(tilt), math.Cos(tilt)
	// rot leva coordenada local (em unidades de projeto) pra tela.
	rot := func(lx, ly float64) (float64, float64) {
		return cx + L(lx*cs-ly*sn), cy + L(lx*sn+ly*cs)
	}
	blob := func(lx, ly, rx, ry float64, lvl int) {
		px0, py0 := rot(lx, ly)
		for y := int(py0 - L(ry)); y <= int(py0+L(ry))+1; y++ {
			for x := int(px0 - L(rx)); x <= int(px0+L(rx))+1; x++ {
				nx, ny := (float64(x)-px0)/L(rx), (float64(y)-py0)/L(ry)
				if nx*nx+ny*ny > 1 {
					continue
				}
				b.set(x, y, lvl+minInt(maxInt(int(1.6-1.4*ny-0.7*nx), 0), 2))
			}
		}
	}
	// Bico: cone saindo do ombro direito, é por ele que o café cai.
	for s := 0.0; s <= 1; s += 0.008 {
		x0, y0 := rot(lerp(4.6, 11.0, s), lerp(-1.2, 0.6, s))
		half := L(lerp(1.4, 0.6, s))
		for d := -half; d <= half; d += 0.7 {
			b.set(int(x0), int(y0+d), relaxPot+2)
		}
	}
	// Asa à esquerda.
	for a := 1.9; a < 4.4; a += 0.008 {
		for k := 0.0; k <= 1; k += 0.5 {
			x0, y0 := rot(-5.4+math.Cos(a)*(2.8+k), math.Sin(a)*(2.8+k))
			b.set(int(x0), int(y0), relaxPot+1)
		}
	}
	blob(0, 0, 6.0, 4.6, relaxPot+1) // corpo
	blob(0, -4.3, 3.6, 1.2, relaxPot+2)
	blob(0, -5.5, 1.0, 0.9, relaxPot+3) // pegador da tampa
	return rot(11.4, 0.8)
}
