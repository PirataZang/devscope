package ui

import (
	"math"
	"math/rand"
)

// ── Gerador de planetas ─────────────────────────────────────────────────────────
//
// Um planeta grande em Braille, no mesmo espírito da Lua: para cada subpixel
// dentro do círculo se reconstrói a normal da esfera e daí sai o terminador por
// iluminação de verdade — degradê de densidade, não linha serrilhada. A luz varre
// a face DEVAGAR (oscila em torno da frente, ~90s de período).
//
// A cada geração o mundo é sorteado numa TABELA DE RARIDADE (soma 100):
//
//	40%  redondo normal
//	40%  redondo com órbita (anel de asteroides)
//	 5%  quadrado (Minecraft)
//	 5%  com mordida (Galactus passou por aqui)
//	 5%  Terra
//	 2%  unicórnio (chifre arco-íris)
//	 1%  plano (é sério)
//	 1%  rosquinha (torus)
//	 1%  nave espacial de passagem
//
// O tipo de superfície (rochoso/gasoso/gelado/lava) e a cor também são sorteados,
// então dois planetas nunca saem iguais.

// ── Tabela de raridade ───────────────────────────────────────────────────────

const (
	pvRound   = iota // redondo normal
	pvRing           // redondo com órbita
	pvCube           // quadrado, Minecraft
	pvBitten         // com mordida (Galactus)
	pvEarth          // Terra
	pvUnicorn        // com chifre de unicórnio
	pvFlat           // plano
	pvDonut          // rosquinha
	pvShip           // nave espacial
	pvCount
)

// relaxPlWeights são as chances declaradas de frente, em pontos percentuais.
var relaxPlWeights = [pvCount]int{
	pvRound:   40,
	pvRing:    40,
	pvCube:    5,
	pvBitten:  5,
	pvEarth:   5,
	pvUnicorn: 2,
	pvFlat:    1,
	pvDonut:   1,
	pvShip:    1,
}

var relaxPlVariantNames = [pvCount]string{
	pvRound:   "mundo",
	pvRing:    "mundo com anéis",
	pvCube:    "mundo cúbico",
	pvBitten:  "mundo mordido",
	pvEarth:   "Terra",
	pvUnicorn: "mundo unicórnio",
	pvFlat:    "mundo plano",
	pvDonut:   "mundo rosquinha",
	pvShip:    "nave de passagem",
}

// relaxPlPickVariant sorteia a variante respeitando os pesos.
func relaxPlPickVariant(rng *rand.Rand) int {
	total := 0
	for _, w := range relaxPlWeights {
		total += w
	}
	r := rng.Intn(total)
	for v, w := range relaxPlWeights {
		if r < w {
			return v
		}
		r -= w
	}
	return pvRound
}

// Tipos de superfície.
const (
	plRocky = iota
	plGas
	plIce
	plLava
	plTypes
)

var relaxPlKindNames = []string{"rochoso", "gasoso", "gelado", "de lava"}

// ── Paleta ───────────────────────────────────────────────────────────────────

const (
	relaxPlBodyN = 16 // rampa do corpo (sombra → luz)
	relaxPlLandN = 6  // continentes (Terra) / grama (cubo)
	relaxPlHornN = 7  // arco-íris do chifre
)

const (
	relaxPlBody   = 0
	relaxPlLand   = relaxPlBody + relaxPlBodyN
	relaxPlHorn   = relaxPlLand + relaxPlLandN
	relaxPlStar   = relaxPlHorn + relaxPlHornN
	relaxPlStar2  = relaxPlStar + 1
	relaxPlRing   = relaxPlStar2 + 1
	relaxPlRingHi = relaxPlRing + 1
	relaxPlCloud  = relaxPlRingHi + 1
	relaxPlRim    = relaxPlCloud + 1
	relaxPlDark   = relaxPlRim + 1 // borda da mordida / vãos
	relaxPlHull   = relaxPlDark + 1
	relaxPlGlass  = relaxPlHull + 1
	relaxPlFlame  = relaxPlGlass + 1
	relaxPlEnd    = relaxPlFlame + 1
)

// relaxPlBuildPalette monta a rampa do corpo tingida pelo matiz + as camadas
// fixas. Terra e unicórnio têm cores próprias.
func relaxPlBuildPalette(variant, kind int, hue float64) []relaxColor {
	out := make([]relaxColor, relaxPlEnd)

	sat := 0.42
	switch kind {
	case plGas:
		sat = 0.50
	case plIce:
		sat = 0.30
	case plLava:
		sat = 0.62
	}
	if variant == pvEarth {
		hue, sat = 212, 0.62 // oceano azul
	}
	for i := 0; i < relaxPlBodyN; i++ {
		f := float64(i) / float64(relaxPlBodyN-1)
		// A luz esquenta um tico o matiz — efeito de cor na iluminação.
		h := hue + 14*f
		l := 0.04 + 0.66*math.Pow(f, 0.9)
		out[i] = relaxHSL(math.Mod(h+360, 360), sat*(0.5+0.5*f), l)
	}

	// Continentes / grama: verde escuro → verde claro → areia.
	landStops := []string{"#173A1C", "#245A26", "#3A8034", "#63A648", "#9AC46A", "#D8CE94"}
	if variant == pvEarth {
		landStops = []string{"#123A1E", "#1E5C28", "#357F32", "#6FA648", "#A8B061", "#CBB183"}
	}
	copy(out[relaxPlLand:], relaxRamp(landStops, relaxPlLandN))

	// Arco-íris do chifre.
	for i := 0; i < relaxPlHornN; i++ {
		h := float64(i) / float64(relaxPlHornN) * 320
		out[relaxPlHorn+i] = relaxHSL(h, 0.85, 0.66)
	}

	out[relaxPlStar] = "#7C8496"
	out[relaxPlStar2] = "#C6D0E4"
	out[relaxPlRing] = "#5A5348"
	out[relaxPlRingHi] = relaxHSL(math.Mod(hue+30, 360), 0.30, 0.72)
	out[relaxPlCloud] = "#E8ECF2"
	out[relaxPlRim] = relaxHSL(math.Mod(hue+20, 360), 0.65, 0.72)
	out[relaxPlDark] = "#0A0B10"
	out[relaxPlHull] = "#9AA6B4"
	out[relaxPlGlass] = "#6FD8F0"
	out[relaxPlFlame] = "#FFB84A"
	return out
}

// ── Estado ───────────────────────────────────────────────────────────────────

// relaxPlSpot é um acidente de superfície (continente, mancha, cratera) em
// coordenada de esfera: vetor unitário, raio angular e efeito no albedo.
type relaxPlSpot struct {
	x, y, z float64
	r       float64
	lim     float64
	albedo  float64
	rim     float64
}

type plAsteroid struct {
	ang  float64
	rad  float64
	spd  float64
	size int8
}

type relaxSwordState struct {
	inited bool
	tick   int
	clock  relaxSubClock

	// Compat com o teste antigo (ciclo de vento).
	phase float64
	gust  float64
	anim  float64

	variant int
	kind    int
	genT    float64
	genDur  float64
	spin    float64
	light   float64
	lightT  float64
	axis    float64
	bands   int
	hasRing bool
	ringTlt float64
	spots   []relaxPlSpot
	stars   []relaxSkyPt
	ast     []plAsteroid
	pal     []relaxColor
	name    string

	// Mordida do Galactus (calota removida).
	biteX, biteY, biteZ float64
	biteR               float64

	// Buffers de profundidade (só a rosquinha usa).
	zbuf []float32
	lbuf []int16

	rng *rand.Rand // gerador próprio da cena
}

// ── Geração procedural ───────────────────────────────────────────────────────

func relaxPlGenerate(st *relaxSwordState) {
	// RNG próprio da cena: semeado UMA vez do gerador global (então varia a cada
	// execução) e reusado nas gerações seguintes. Assim a cena não fica
	// consumindo o rand global a cada troca de planeta — o que embaralharia o
	// sorteio das outras cenas.
	if st.rng == nil {
		st.rng = rand.New(rand.NewSource(rand.Int63()))
	}
	rng := st.rng
	st.variant = relaxPlPickVariant(rng)
	st.kind = rng.Intn(plTypes)
	if st.variant == pvEarth {
		st.kind = plRocky
	}
	st.genT = 0
	st.genDur = 26 + rng.Float64()*16
	st.spin = rng.Float64() * 2 * math.Pi
	// Luz sempre vinda mais de frente pra o disco ficar bem visível.
	st.light = math.Pi/2 + (rng.Float64()-0.5)*1.4
	st.lightT = rng.Float64() * 90
	st.axis = (rng.Float64() - 0.5) * 0.9
	if st.variant == pvEarth {
		st.axis = 0.41 // inclinação da Terra, de verdade
	}
	st.bands = 5 + rng.Intn(6)
	st.hasRing = st.variant == pvRing
	st.ringTlt = 0.28 + rng.Float64()*0.5
	st.name = relaxPlVariantNames[st.variant]
	if st.variant == pvRound || st.variant == pvRing {
		st.name += " " + relaxPlKindNames[st.kind]
	}

	hue := rng.Float64() * 360
	switch st.kind {
	case plGas:
		hue = 24 + rng.Float64()*40
	case plIce:
		hue = 180 + rng.Float64()*60
	case plLava:
		hue = rng.Float64() * 24
	}
	st.pal = relaxPlBuildPalette(st.variant, st.kind, hue)

	// Mordida: uma calota na borda superior-direita, cortando a silhueta.
	bd := [3]float64{0.80 + rng.Float64()*0.2, 0.30 + rng.Float64()*0.35, 0.22 + rng.Float64()*0.25}
	if rng.Intn(2) == 0 {
		bd[0] = -bd[0]
	}
	bl := math.Sqrt(bd[0]*bd[0] + bd[1]*bd[1] + bd[2]*bd[2])
	st.biteX, st.biteY, st.biteZ = bd[0]/bl, bd[1]/bl, bd[2]/bl
	st.biteR = 0.52 + rng.Float64()*0.16

	// Acidentes de superfície.
	st.spots = st.spots[:0]
	switch {
	case st.variant == pvEarth:
		// Continentes: blobs grandes; e ilhas pequenas.
		for i, n := 0, 6+rng.Intn(4); i < n; i++ {
			lon := rng.Float64() * 2 * math.Pi
			lat := (rng.Float64() - 0.5) * 2.0
			st.spots = append(st.spots, relaxPlSpot{
				x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
				r: 0.26 + rng.Float64()*0.30, albedo: 1,
			})
		}
		for i, n := 0, 5+rng.Intn(6); i < n; i++ {
			lon := rng.Float64() * 2 * math.Pi
			lat := math.Asin(rng.Float64()*2 - 1)
			st.spots = append(st.spots, relaxPlSpot{
				x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
				r: 0.06 + rng.Float64()*0.09, albedo: 1,
			})
		}
	case st.kind == plRocky || st.kind == plIce:
		bias := rng.Float64() * 2 * math.Pi
		for i, n := 0, 5+rng.Intn(5); i < n; i++ {
			lon := bias + (rng.Float64()-0.5)*3.0
			lat := (rng.Float64() - 0.5) * 2.2
			alb := 0.28 + rng.Float64()*0.22
			if rng.Float64() < 0.5 {
				alb = -(0.24 + rng.Float64()*0.20)
			}
			st.spots = append(st.spots, relaxPlSpot{
				x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
				r: 0.20 + rng.Float64()*0.30, albedo: alb,
			})
		}
		for i, n := 0, 8+rng.Intn(10); i < n; i++ {
			lon := rng.Float64() * 2 * math.Pi
			lat := math.Asin(rng.Float64()*2 - 1)
			st.spots = append(st.spots, relaxPlSpot{
				x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
				r: 0.05 + rng.Float64()*0.07, albedo: -0.10 - rng.Float64()*0.10,
				rim: 0.16 + rng.Float64()*0.16,
			})
		}
	case st.kind == plLava:
		for i, n := 0, 10+rng.Intn(10); i < n; i++ {
			lon := rng.Float64() * 2 * math.Pi
			lat := math.Asin(rng.Float64()*2 - 1)
			st.spots = append(st.spots, relaxPlSpot{
				x: math.Cos(lat) * math.Sin(lon), y: math.Sin(lat), z: math.Cos(lat) * math.Cos(lon),
				r: 0.08 + rng.Float64()*0.16, albedo: 0.30 + rng.Float64()*0.30,
			})
		}
	}
	for i := range st.spots {
		st.spots[i].lim = (1.4 * st.spots[i].r) * (1.4 * st.spots[i].r)
	}

	// Estrelas de fundo.
	st.stars = st.stars[:0]
	for i, n := 0, 60+rng.Intn(40); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rng.Float64(), y: rng.Float64()})
	}

	// Anel de asteroides — banda elíptica próxima ao planeta.
	st.ast = st.ast[:0]
	if st.hasRing {
		for r, nr := 0, 2+rng.Intn(2); r < nr; r++ {
			r0 := 1.24 + float64(r)*0.16 + rng.Float64()*0.05
			for i, n := 0, 90+rng.Intn(60); i < n; i++ {
				st.ast = append(st.ast, plAsteroid{
					ang:  rng.Float64() * 2 * math.Pi,
					rad:  r0 + (rng.Float64()-0.5)*0.05,
					spd:  0.05 + rng.Float64()*0.04,
					size: int8(rng.Intn(3)),
				})
			}
		}
	}
}

// ── Ciclo de vento (mantido só pro teste) ──────────────────────────────────────

const (
	relaxSwGustTicks = 16
	relaxSwRestTicks = 30
	relaxSwCycle     = relaxSwGustTicks + relaxSwRestTicks
)

func relaxSwGust(ph float64) float64 {
	if ph < 0 || ph >= relaxSwGustTicks {
		return 0
	}
	u := ph / relaxSwGustTicks
	p := math.Pow(u, 0.72)
	return (math.Sin(math.Pi*p) + 0.36*math.Sin(6*math.Pi*p)*p*(1-p)) * (1 - math.Pow(u, 6))
}

// ── Passo ────────────────────────────────────────────────────────────────────

func relaxSwInit(st *relaxSwordState) {
	st.inited = true
	relaxPlGenerate(st)
	st.phase = relaxSwGustTicks + relaxSwRestTicks/2
}

func relaxSwAdvance(st *relaxSwordState, d float64) {
	if st.phase += d; st.phase >= relaxSwCycle {
		st.phase -= relaxSwCycle
	}
	st.gust = relaxSwGust(st.phase)
	st.anim += d * 0.1

	dt := d * 0.1
	// O terminador varre a face DEVAGAR, oscilando em torno da frente.
	st.lightT += dt
	st.light = math.Pi/2 + 0.9*math.Sin(st.lightT*0.07)
	// O planeta gira devagar em torno do eixo.
	st.spin += 2 * math.Pi / 360 * d
	if st.spin > 2*math.Pi {
		st.spin -= 2 * math.Pi
	}
	for i := range st.ast {
		st.ast[i].ang += st.ast[i].spd * dt
	}
	st.genT += dt
	if st.genT >= st.genDur {
		relaxPlGenerate(st)
	}
}

func stepRelaxSword(st *relaxSwordState) {
	if !st.inited {
		relaxSwInit(st)
	}
	st.tick++
	relaxSwAdvance(st, 1)
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxSwordFrames(st *relaxSwordState, width, height int, fade, now float64) ([]string, string) {
	if !st.inited {
		relaxSwInit(st)
	}
	for i, n := 0, st.clock.tick(now); i < n; i++ {
		relaxSwAdvance(st, relaxSubDT/0.1)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 40))
	// Cubo e nave têm famílias de cor que não podem virar média entre si (grama
	// vs terra, casco vs vidro vs fogo): nesses o buffer é de VOTO. A esfera e a
	// rosquinha querem a média, que é o que suaviza o terminador.
	var b *relaxBraille
	if st.variant == pvCube || st.variant == pvShip {
		b = newRelaxBrailleVote(w, h)
	} else {
		b = newRelaxBraille(w, h)
	}
	sw, sh := w*2, h*4

	cx, cy := float64(sw-1)/2, float64(sh-1)*0.48
	rad := math.Min(float64(sw)*0.36, float64(sh)*0.46)
	// O unicórnio precisa de céu acima da cabeça: encolhe o mundo e o abaixa,
	// senão o chifre sai pela borda de cima do palco.
	if st.variant == pvUnicorn {
		rad *= 0.72
		cy += rad * 0.34
	}

	// Aparece e some suave ao trocar de planeta.
	genFade := clamp01(st.genT / 1.2)
	if dg := st.genDur - st.genT; dg < 1.2 {
		genFade = clamp01(dg / 1.2)
	}

	// Estrelas primeiro: o corpo é desenhado depois e o Braille não sobrescreve,
	// então o planeta precisa vir por cima. (relaxPlDrawStars pula o disco.)
	relaxPlDrawStars(st, b, sw, sh, cx, cy, rad, now)

	switch st.variant {
	case pvShip:
		relaxPlDrawShip(st, b, sw, sh, cx, cy, rad, genFade)
	case pvDonut:
		relaxPlDrawDonut(st, b, sw, sh, cx, cy, rad, genFade)
	case pvCube:
		relaxPlDrawCube(st, b, cx, cy, rad, genFade)
	case pvFlat:
		relaxPlDrawFlat(st, b, sw, sh, cx, cy, rad, genFade)
	default:
		// Anel de trás, esfera, anel da frente.
		if st.hasRing {
			relaxPlDrawRing(st, b, cx, cy, rad, false, genFade)
		}
		relaxPlDrawSphere(st, b, sw, sh, cx, cy, rad, genFade, now)
		if st.variant == pvUnicorn {
			relaxPlDrawHorn(st, b, cx, cy, rad, genFade)
		}
		if st.hasRing {
			relaxPlDrawRing(st, b, cx, cy, rad, true, genFade)
		}
	}

	return b.lines(relaxStyles(st.pal, fade)), StyleMuted.Render(st.name)
}

// ── Esfera (redondo, anel, mordida, Terra, unicórnio) ──────────────────────────

func relaxPlDrawSphere(st *relaxSwordState, b *relaxBraille, sw, sh int, cx, cy, rad, genFade, now float64) {
	x0, x1 := maxInt(0, int(cx-rad)-1), minInt(sw-1, int(cx+rad)+1)
	y0, y1 := maxInt(0, int(cy-rad)-1), minInt(sh-1, int(cy+rad)+1)

	sa, ca := math.Sin(st.axis), math.Cos(st.axis)
	lx, lz := math.Cos(st.light), math.Sin(st.light)
	ssp, csp := math.Sin(st.spin), math.Cos(st.spin)
	bitten := st.variant == pvBitten
	earth := st.variant == pvEarth

	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			dx, dy := (float64(x)-cx)/rad, (float64(y)-cy)/rad
			nx := dx*ca + dy*sa
			nyt := -dx*sa + dy*ca
			d2 := nx*nx + nyt*nyt
			if d2 > 1 {
				continue
			}
			nz := math.Sqrt(1 - d2)
			ny := nyt

			// Coordenada de superfície (roda a longitude pela rotação).
			ux := nx*csp - nz*ssp
			uz := nx*ssp + nz*csp
			uy := ny

			// Mordida do Galactus: a calota é arrancada.
			if bitten {
				bdx, bdy, bdz := ux-st.biteX, uy-st.biteY, uz-st.biteZ
				bd2 := bdx*bdx + bdy*bdy + bdz*bdz
				if bd2 < st.biteR*st.biteR {
					// Borda da mordida: um fio escuro dá a espessura da casca.
					if bd2 > (st.biteR-0.055)*(st.biteR-0.055) {
						b.set(x, y, relaxPlDark)
					}
					continue
				}
			}

			land := false
			albedo := relaxPlBaseAlbedo(st, ux, uy, uz, now)
			for si := range st.spots {
				s := st.spots[si]
				ddx, ddy, ddz := ux-s.x, uy-s.y, uz-s.z
				d2s := ddx*ddx + ddy*ddy + ddz*ddz
				if d2s > s.lim {
					continue
				}
				if ux*s.x+uy*s.y+uz*s.z <= 0 {
					continue
				}
				dist := math.Sqrt(d2s) / s.r
				if earth {
					if dist < 1 {
						land = true
					}
					continue
				}
				if dist < 1 {
					albedo += s.albedo * (1 - dist*dist*dist)
				} else if s.rim > 0 {
					albedo += s.rim * (1 - (dist-1)/0.4)
				}
			}

			// Calota polar da Terra.
			ice := earth && math.Abs(uy) > 0.86

			// Nuvens móveis.
			cloud := 0.0
			if st.kind == plRocky || st.kind == plIce || earth {
				cloud = relaxPlCloudAt(ux, uy, uz, now)
				if !earth {
					albedo += cloud * 0.5
				}
			}

			lam := nx*lx + nz*lz
			lum := 0.10
			if lam > 0 {
				lum += clamp01(albedo) * (0.34 + 0.66*math.Pow(lam, 0.7))
			}
			lum *= genFade
			if lum <= relaxHalftone(x, y)*0.78 {
				// Realce quente do limbo iluminado.
				if lam > 0.1 && d2 > 0.86 && relaxHalftone(x, y) < 0.5*lam*genFade {
					b.set(x, y, relaxPlRim)
				}
				continue
			}
			switch {
			case ice:
				b.set(x, y, relaxPlCloud)
			case earth && cloud > 0.62 && lam > 0:
				b.set(x, y, relaxPlCloud)
			case earth && land:
				lvl := relaxPlLand + minInt(int(clamp01(lum*1.25)*float64(relaxPlLandN)), relaxPlLandN-1)
				b.set(x, y, lvl)
			case cloud > 0.6 && lam > 0 && !earth:
				b.set(x, y, relaxPlCloud)
			default:
				b.set(x, y, int(clamp01(lum)*float64(relaxPlBodyN-1)))
			}
		}
	}

	// Migalhas da mordida flutuando (Galactus é desleixado).
	if bitten {
		for i := 0; i < 7; i++ {
			a := float64(i)*1.9 + st.anim*0.25
			rr := rad * (1.12 + 0.16*float64(i%3))
			px := cx + (st.biteX*0.7+math.Cos(a)*0.35)*rr
			py := cy + (st.biteY*0.7+math.Sin(a)*0.30)*rr
			if px >= 0 && py >= 0 && int(px) < sw && int(py) < sh {
				b.set(int(px), int(py), relaxPlRing)
			}
		}
	}
}

// relaxPlBaseAlbedo é o albedo de base da superfície — dá as bandas do gasoso.
func relaxPlBaseAlbedo(st *relaxSwordState, ux, uy, uz, t float64) float64 {
	if st.variant == pvEarth {
		return 0.54 // oceano
	}
	switch st.kind {
	case plGas:
		lat := math.Asin(clampF(uy, -1, 1))
		band := math.Sin(lat*float64(st.bands) + 0.4*math.Sin(lat*3+t*0.05))
		return 0.62 + 0.26*band
	case plIce:
		return 0.66 + 0.30*math.Pow(math.Abs(uy), 3)
	case plLava:
		return 0.50
	default:
		return 0.58
	}
}

// relaxPlCloudAt é o véu de nuvem: ruído contínuo que anda devagar.
func relaxPlCloudAt(ux, uy, uz, t float64) float64 {
	lon := math.Atan2(ux, uz)
	lat := math.Asin(clampF(uy, -1, 1))
	v := relaxNoise(lon*2.4+t*0.06, 4242) + 0.6*relaxNoise(lat*3.1-t*0.03, 99)
	return clamp01(v*0.8 - 0.35)
}

// ── Chifre de unicórnio ──────────────────────────────────────────────────────

// relaxPlDrawHorn desenha um chifre espiralado saindo do topo, em arco-íris.
func relaxPlDrawHorn(st *relaxSwordState, b *relaxBraille, cx, cy, rad, genFade float64) {
	if genFade < 0.4 {
		return
	}
	baseY := cy - rad*0.94
	length := rad * 0.70
	baseR := rad * 0.15
	steps := maxInt(14, int(length*1.4))
	for i := 0; i <= steps; i++ {
		f := float64(i) / float64(steps) // 0 base → 1 ponta
		yy := baseY - f*length
		if yy < 0 {
			break // não desenha fora do palco
		}
		rr := baseR * (1 - f*0.92)
		// Torção: o chifre é espiralado, então a espessura "pisca" de lado.
		tw := math.Sin(f*11 + st.anim*0.8)
		lvl := relaxPlHorn + int(f*float64(relaxPlHornN-1)+0.5)
		if lvl > relaxPlHorn+relaxPlHornN-1 {
			lvl = relaxPlHorn + relaxPlHornN - 1
		}
		for dxp := -rr; dxp <= rr; dxp += 0.5 {
			// A estria da espiral clareia/escurece ao longo da volta.
			s := lvl
			if tw*dxp > rr*0.25 {
				s = relaxPlHorn + (int(f*float64(relaxPlHornN-1))+3)%relaxPlHornN
			}
			b.set(int(cx+dxp), int(yy), s)
		}
	}
	// Faíscas em volta da ponta.
	for i := 0; i < 5; i++ {
		a := float64(i)*1.3 + st.anim*1.1
		px := cx + math.Cos(a)*rad*0.24
		py := baseY - length - math.Sin(a)*rad*0.12
		if px < 0 || py < 0 || int(px) >= b.w*2 || int(py) >= b.h*4 {
			continue
		}
		b.text(int(px)/2, int(py)/4, "✦", relaxPlHorn+(i*2)%relaxPlHornN)
	}
}

// ── Cubo (Minecraft) ─────────────────────────────────────────────────────────

// relaxPlDrawCube desenha um planeta cúbico: três faces visíveis com Lambert
// chapado (é isso que dá o ar de bloco), grama no topo, terra nas laterais, e a
// grade de blocos por cima.
func relaxPlDrawCube(st *relaxSwordState, b *relaxBraille, cx, cy, rad, genFade float64) {
	s := rad * 0.56
	yaw, pitch := st.spin+0.785, -0.55 // 45° e visto DE CIMA (pitch negativo)
	sy, cyw := math.Sin(yaw), math.Cos(yaw)
	sp, cp := math.Sin(pitch), math.Cos(pitch)
	rot := func(v [3]float64) [3]float64 {
		x := v[0]*cyw + v[2]*sy
		z := -v[0]*sy + v[2]*cyw
		y := v[1]*cp - z*sp
		return [3]float64{x, y, v[1]*sp + z*cp}
	}
	corner := func(i int) [3]float64 {
		x, y, z := -s, -s, -s
		if i&1 != 0 {
			x = s
		}
		if i&2 != 0 {
			y = s
		}
		if i&4 != 0 {
			z = s
		}
		return rot([3]float64{x, y, z})
	}
	var v [8][3]float64
	for i := 0; i < 8; i++ {
		v[i] = corner(i)
	}
	faces := [6][4]int{{1, 3, 7, 5}, {0, 4, 6, 2}, {2, 6, 7, 3}, {0, 1, 5, 4}, {4, 5, 7, 6}, {0, 2, 3, 1}}
	norms := [6][3]float64{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	lx, lz := math.Cos(st.light), math.Sin(st.light)

	type vis struct {
		idx int
		z   float64
		upY float64 // y da normal já rodada: mais negativo = aponta pra cima
	}
	var order []vis
	for fi := 0; fi < 6; fi++ {
		n := rot(norms[fi])
		if n[2] <= 0.02 {
			continue // face virada pra trás
		}
		zs := 0.0
		for _, ci := range faces[fi] {
			zs += v[ci][2]
		}
		order = append(order, vis{fi, zs / 4, n[1]})
	}
	// Mais perto primeiro (o Braille não sobrescreve).
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && order[j].z > order[j-1].z; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	// O topo é a face cuja NORMAL aponta pra cima (y negativo na tela) — é ela
	// que leva a grama. Pelo y médio dos cantos a frente ganhava por engano.
	topIdx := -1
	best := 0.0
	for _, o := range order {
		if o.upY < best {
			best, topIdx = o.upY, o.idx
		}
	}

	for _, o := range order {
		fi := o.idx
		n := rot(norms[fi])
		lam := clamp01(n[0]*lx + n[2]*lz)
		lum := (0.22 + 0.78*lam) * genFade
		var q [4][2]float64
		for k, ci := range faces[fi] {
			q[k] = [2]float64{cx + v[ci][0], cy + v[ci][1]}
		}
		lvl := int(clamp01(lum) * float64(relaxPlBodyN-1))
		grass := fi == topIdx
		if grass {
			lvl = relaxPlLand + minInt(int(clamp01(lum*1.2)*float64(relaxPlLandN)), relaxPlLandN-1)
		}
		dark := maxInt(0, lvl-3)
		if grass {
			dark = relaxPlLand
		}
		// Preenchimento PARAMÉTRICO: varre a face no seu próprio (s,t), então a
		// grade de blocos sai de graça — e desenhada junto, não depois (o
		// Braille não sobrescreve ponto aceso, e grade por cima não apareceria).
		side := math.Hypot(q[1][0]-q[0][0], q[1][1]-q[0][1])
		side2 := math.Hypot(q[3][0]-q[0][0], q[3][1]-q[0][1])
		ns := maxInt(6, int(math.Max(side, side2)*1.6))
		const cells = 3 // blocos maiores: a linha da grade sobrevive ao voto
		for is := 0; is <= ns; is++ {
			s := float64(is) / float64(ns)
			ax := lerp(q[0][0], q[1][0], s)
			ay := lerp(q[0][1], q[1][1], s)
			bx := lerp(q[3][0], q[2][0], s)
			by := lerp(q[3][1], q[2][1], s)
			for it := 0; it <= ns; it++ {
				t := float64(it) / float64(ns)
				px := lerp(ax, bx, t)
				py := lerp(ay, by, t)
				// Linha de bloco: perto de uma divisão em s ou t. A faixa é larga
				// de propósito — linha de um subponto some na média/voto da célula.
				gs := math.Abs(math.Mod(s*cells, 1) - 0.5)
				gt := math.Abs(math.Mod(t*cells, 1) - 0.5)
				use := lvl
				if gs > 0.40 || gt > 0.40 {
					use = dark
				} else if relaxHalftone(int(px), int(py)) < 0.10 {
					use = maxInt(0, lvl-1) // grão do bloco
				}
				b.set(int(px), int(py), use)
			}
		}
	}
}

// ── Planeta plano ────────────────────────────────────────────────────────────

// relaxPlDrawFlat desenha um disco achatado visto de viés, com espessura na
// borda — o mundo plano, com a tartaruga de folga.
func relaxPlDrawFlat(st *relaxSwordState, b *relaxBraille, sw, sh int, cx, cy, rad, genFade float64) {
	rx := rad * 1.08
	ry := rad * 0.26
	thick := rad * 0.13
	lx, lz := math.Cos(st.light), math.Sin(st.light)
	lit := clamp01(0.35 + 0.65*lz) // a face de cima recebe a luz de frente
	// Face de cima.
	for y := int(cy-ry) - 1; y <= int(cy+ry)+1; y++ {
		for x := int(cx-rx) - 1; x <= int(cx+rx)+1; x++ {
			if x < 0 || y < 0 || x >= sw || y >= sh {
				continue
			}
			u, vv := (float64(x)-cx)/rx, (float64(y)-cy)/ry
			d2 := u*u + vv*vv
			if d2 > 1 {
				continue
			}
			// Relevo simples: anéis de terreno + um pouco de granulado.
			terr := 0.5 + 0.5*math.Sin(math.Sqrt(d2)*7+st.spin)
			lum := (0.30 + 0.55*lit) * (0.72 + 0.28*terr) * genFade
			lum -= 0.10 * clamp01(u*lx) // sombra do lado oposto à luz
			if lum <= relaxHalftone(x, y)*0.7 {
				continue
			}
			lvl := relaxPlLand + minInt(int(clamp01(lum)*float64(relaxPlLandN)), relaxPlLandN-1)
			if d2 > 0.90 {
				lvl = relaxPlBody + int(clamp01(lum)*float64(relaxPlBodyN-1)) // borda rochosa
			}
			b.set(x, y, lvl)
		}
	}
	// Espessura: a lateral do prato.
	for y := int(cy + ry*0.1); y <= int(cy+ry+thick); y++ {
		for x := int(cx-rx) - 1; x <= int(cx+rx)+1; x++ {
			if x < 0 || y < 0 || x >= sw || y >= sh || b.taken(x, y) {
				continue
			}
			u := (float64(x) - cx) / rx
			if u*u > 1 {
				continue
			}
			edge := ry * math.Sqrt(1-u*u)
			if float64(y) < cy+edge*0.55 || float64(y) > cy+edge+thick {
				continue
			}
			lum := (0.16 + 0.30*clamp01(-u*lx+0.4)) * genFade
			if lum <= relaxHalftone(x, y)*0.6 {
				continue
			}
			b.set(x, y, relaxPlBody+int(clamp01(lum)*float64(relaxPlBodyN-1)))
		}
	}
}

// ── Rosquinha (torus) ────────────────────────────────────────────────────────

// relaxPlDrawDonut varre o torus em (u,v) e resolve a oclusão com um z-buffer —
// sem ele os pedaços de trás furam a frente do anel.
func relaxPlDrawDonut(st *relaxSwordState, b *relaxBraille, sw, sh int, cx, cy, rad, genFade float64) {
	n := sw * sh
	if len(st.zbuf) != n {
		st.zbuf = make([]float32, n)
		st.lbuf = make([]int16, n)
	}
	for i := 0; i < n; i++ {
		st.zbuf[i] = -1e9
		st.lbuf[i] = -1
	}
	const R, r = 0.70, 0.30
	tilt := 0.60
	stl, ctl := math.Sin(tilt), math.Cos(tilt)
	ssp, csp := math.Sin(st.spin), math.Cos(st.spin)
	lx, lz := math.Cos(st.light), math.Sin(st.light)

	uSteps := maxInt(180, sw*2)
	vSteps := 72
	for iu := 0; iu < uSteps; iu++ {
		u := float64(iu) / float64(uSteps) * 2 * math.Pi
		cu, su := math.Cos(u), math.Sin(u)
		for iv := 0; iv < vSteps; iv++ {
			vv := float64(iv) / float64(vSteps) * 2 * math.Pi
			cv, sv := math.Cos(vv), math.Sin(vv)
			px := (R + r*cv) * cu
			py := r * sv
			pz := (R + r*cv) * su
			nx, ny, nz := cv*cu, sv, cv*su
			// gira em torno de Y
			px, pz = px*csp-pz*ssp, px*ssp+pz*csp
			nx, nz = nx*csp-nz*ssp, nx*ssp+nz*csp
			// inclina em torno de X
			py, pz = py*ctl-pz*stl, py*stl+pz*ctl
			ny, nz = ny*ctl-nz*stl, ny*stl+nz*ctl

			X, Y := int(cx+px*rad), int(cy+py*rad)
			if X < 0 || Y < 0 || X >= sw || Y >= sh {
				continue
			}
			idx := Y*sw + X
			if float32(pz) <= st.zbuf[idx] {
				continue
			}
			st.zbuf[idx] = float32(pz)
			lam := nx*lx + nz*lz
			lum := 0.10
			if lam > 0 {
				lum += 0.78 * math.Pow(lam, 0.7)
			}
			// Glacê: uma faixa clara na metade de cima do tubo.
			if sv > 0.25 {
				lum += 0.12
			}
			lum *= genFade
			if lum <= relaxHalftone(X, Y)*0.72 {
				st.lbuf[idx] = -1
				continue
			}
			st.lbuf[idx] = int16(int(clamp01(lum) * float64(relaxPlBodyN-1)))
		}
	}
	for i := 0; i < n; i++ {
		if l := st.lbuf[i]; l >= 0 {
			b.set(i%sw, i/sw, int(l))
		}
	}
}

// ── Nave espacial ────────────────────────────────────────────────────────────

// relaxPlDrawShip desenha uma nave passando devagar: casco, cabine de vidro,
// asas e o fogo do motor piscando.
func relaxPlDrawShip(st *relaxSwordState, b *relaxBraille, sw, sh int, cx, cy, rad, genFade float64) {
	if genFade < 0.3 {
		return
	}
	// Passa devagar da esquerda pra direita, subindo e descendo de leve.
	prog := math.Mod(st.genT/st.genDur, 1.0)
	x := lerp(float64(sw)*0.12, float64(sw)*0.88, prog)
	y := cy + math.Sin(st.anim*0.5)*rad*0.10
	L := rad * 0.95 // metade do comprimento
	Hh := rad * 0.22

	pen := relaxPen{step: 0.9, put: func(px, py float64, tone int) {
		b.set(int(px), int(py), tone)
	}}
	// Asas: dois triângulos pra trás.
	b.tri(x-L*0.15, y, x-L*0.85, y-Hh*1.9, x-L*0.30, y-Hh*0.2, relaxPlHull)
	b.tri(x-L*0.15, y, x-L*0.85, y+Hh*1.9, x-L*0.30, y+Hh*0.2, relaxPlHull)
	// Casco: fuselagem alongada com o bico à direita.
	pen.ellipse(x-L*0.10, y, L*0.78, Hh, relaxPlHull)
	b.tri(x+L*0.62, y-Hh*0.75, x+L*1.02, y, x+L*0.62, y+Hh*0.75, relaxPlHull)
	// Cabine.
	pen.ellipse(x+L*0.24, y-Hh*0.18, L*0.20, Hh*0.52, relaxPlGlass)
	// Motor: fogo piscando atrás.
	flick := 0.6 + 0.4*math.Sin(st.anim*7)
	for i := 0; i < 3; i++ {
		fl := L * (0.28 + 0.22*flick) * (1 - float64(i)*0.28)
		pen.ellipse(x-L*0.92-fl*0.5, y+float64(i-1)*Hh*0.55, fl*0.5, Hh*0.28, relaxPlFlame)
	}
	// Janelinhas.
	for i := 0; i < 3; i++ {
		b.text(int(x-L*0.30+float64(i)*L*0.16)/2, int(y)/4, "▫", relaxPlGlass)
	}
}

// ── Estrelas e anel ──────────────────────────────────────────────────────────

func relaxPlDrawStars(st *relaxSwordState, b *relaxBraille, sw, sh int, cx, cy, rad, t float64) {
	// A nave e o mundo plano não escondem tanto o fundo; a esfera sim.
	hide := rad + 2
	if st.variant == pvShip || st.variant == pvFlat {
		hide = 0
	}
	for i, p := range st.stars {
		x, y := p.x*float64(sw-1), p.y*float64(sh-1)
		if hide > 0 && math.Hypot(x-cx, y-cy) < hide {
			continue
		}
		if relaxHalftone(int(x), int(y)) < 0.55 {
			tw := math.Sin(t*0.6 + float64(i)*1.3)
			lvl := relaxPlStar
			if tw > 0.4 {
				lvl = relaxPlStar2
			}
			b.set(int(x), int(y), lvl)
			if i%9 == 0 && tw > 0.7 {
				gl := "✧"
				if tw > 0.92 {
					gl = "✦"
				}
				b.text(int(x)/2, int(y)/4, gl, relaxPlStar2)
			} else if i%13 == 0 && tw < -0.75 {
				b.text(int(x)/2, int(y)/4, "˙", relaxPlStar)
			}
		}
	}
}

// relaxPlDrawRing desenha o cinturão de asteroides como uma elipse inclinada. O
// parâmetro front seleciona a metade da frente (z > 0) ou de trás.
func relaxPlDrawRing(st *relaxSwordState, b *relaxBraille, cx, cy, rad float64, front bool, genFade float64) {
	if genFade < 0.5 {
		return
	}
	sTlt, cTlt := math.Sin(st.ringTlt), math.Cos(st.ringTlt)
	lx, lz := math.Cos(st.light), math.Sin(st.light)
	for _, a := range st.ast {
		ox := math.Cos(a.ang) * a.rad
		oz := math.Sin(a.ang) * a.rad
		px := ox
		py := oz * sTlt
		pz := oz * cTlt
		if (pz > 0) != front {
			continue
		}
		if !front && math.Hypot(px, py) < 1.02 {
			continue // escondido atrás do disco
		}
		sx, sy := int(cx+px*rad), int(cy+py*rad)
		if sx < 0 || sy < 0 || sx >= b.w*2 || sy >= b.h*4 {
			continue
		}
		lvl := relaxPlRing
		if clamp01(math.Cos(a.ang)*lx+math.Sin(a.ang)*lz) > 0.5 && a.size > 0 {
			lvl = relaxPlRingHi
		}
		b.set(sx, sy, lvl)
		if a.size >= 2 {
			b.set(sx+1, sy, lvl)
		}
	}
}
