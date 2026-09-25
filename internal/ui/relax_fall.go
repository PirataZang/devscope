package ui

import (
	"math"
	"math/rand"
)

// ── Folhas caindo · Pétalas voando ────────────────────────────────────────────
//
// Mesmo sistema de partículas com dois presets: folha cai girando e pousa;
// pétala é levada pelo vento, quase de lado, e sai pelo outro canto. Posições
// em per-mille (0–1000) mapeadas no render, então a cena acompanha o terminal.
// Rajadas de vento entram e saem com easing e atingem mais as camadas da frente.
//
// Fluidez: a física anda em sub-passos de tempo real (~30fps), escalados por um
// fator `d` (1.0 = passo cheio de 100ms). Somas por-tick multiplicam por d;
// decaimentos multiplicativos viram math.Pow; contadores viram float e avançam
// +=d. Os spawns por-tick usam acumulador fracionário pra manter a MESMA taxa
// média de partículas por segundo. Depois de 100ms o estado é ~idêntico ao de
// antes, só que com ~3x mais quadros.

type relaxFallKind int

const (
	relaxFallLeaves relaxFallKind = iota
	relaxFallPetals
)

type relaxFallPart struct {
	x, y       float64
	vx, vy     float64
	sway, swp  float64 // amplitude e fase do bamboleio
	swSpeed    float64
	spin, spdV float64 // fase e velocidade do giro (troca de glifo)
	layer      int     // 0 fundo · 1 meio · 2 frente
	tone       int
	rest       float64 // frames pousada (só folha)
	big        bool    // as maiores ganham glifo de texto
}

type relaxFallState struct {
	inited    bool
	tick      int
	clock     relaxSubClock
	simT      float64 // tempo contínuo da simulação, em "ticks" (0.1s cada)
	parts     []relaxFallPart
	gust      float64
	gustLeft  float64
	gustDur   float64
	gustPeak  float64
	nextGust  float64
	nextSpawn float64
}

// ── Aparência ─────────────────────────────────────────────────────────────────
//
// Folha e pétala são desenhadas em Braille: uma elipse girada pelo ângulo de
// tombo, com a largura encolhendo conforme ela vira de perfil. É esse
// encolhimento que dá o giro de verdade — trocar de glifo a cada quarto de
// volta dava um piscar, não um tombo.
//
// Enriquecimento pontual: as MAIORES folhas/pétalas da frente ganham um glifo
// de texto no centro (❧ ❦ ⟑ ‛ / ✿ ❀ ·), que o Braille não desenha bem. Poucas,
// só o miolo; a silhueta continua Braille e o fade segue pela paleta (lvl).

var relaxLeafStops = []string{"#3A2412", "#6B4423", "#9A6730", "#C08F4A", "#E0B472"}
var relaxPetalStops = []string{"#3E1E2A", "#6E3548", "#A2536B", "#CE8AA0", "#F0C2D2"}

const relaxFallLevels = 10

var relaxFallRamps = [2][]relaxColor{
	relaxRamp(relaxLeafStops, relaxFallLevels),
	relaxRamp(relaxPetalStops, relaxFallLevels),
}

// Glifos pontuais por tipo. Poucos e discretos: só o miolo das maiores.
var relaxFallGlyphs = [2][]string{
	{"❧", "❦", "⟑", "‛"}, // folhas
	{"✿", "❀", "·"},      // pétalas
}

func relaxFallMax(kind relaxFallKind) int {
	if kind == relaxFallPetals {
		return 52
	}
	return 40
}

func relaxFallSpawn(kind relaxFallKind) relaxFallPart {
	layer := rand.Intn(3)
	p := relaxFallPart{
		layer:   layer,
		tone:    layer + rand.Intn(2),
		swp:     rand.Float64() * 2 * math.Pi,
		swSpeed: 0.05 + rand.Float64()*0.07,
		spin:    rand.Float64() * 4,
		spdV:    0.05 + rand.Float64()*0.12,
	}
	if p.tone > 3 {
		p.tone = 3
	}
	// Só as da frente e grandes o suficiente ganham glifo — poucas.
	p.big = layer == 2 && rand.Intn(3) == 0
	if kind == relaxFallPetals {
		// Entra por um lado e é levada pelo vento: quase horizontal.
		p.x, p.y = -30, 80+rand.Float64()*700
		p.vx = 9 + float64(layer)*3 + rand.Float64()*7
		p.vy = 1 + rand.Float64()*3
		p.sway = 8 + rand.Float64()*14
		return p
	}
	p.x, p.y = rand.Float64()*1000, -40
	p.vy = 6 + float64(layer)*2.5 + rand.Float64()*4
	p.vx = (rand.Float64() - 0.5) * 3
	p.sway = 5 + rand.Float64()*10
	return p
}

// relaxFallInit prepara o estado: primeira leva já espalhada na cena.
func relaxFallInit(st *relaxFallState, kind relaxFallKind) {
	st.inited = true
	st.nextGust = float64(40 + rand.Intn(90))
	for i, n := 0, relaxFallMax(kind)/2; i < n; i++ {
		p := relaxFallSpawn(kind)
		p.x = rand.Float64() * 1000 // primeira leva já espalhada na cena
		p.y = rand.Float64() * 1000
		st.parts = append(st.parts, p)
	}
}

// relaxFallAdvance anda a simulação por `d` ticks (1.0 = passo de 100ms),
// fracionário. Somas por-tick × d; decaimentos multiplicativos elevados a d;
// contadores/tempos em float avançam por d. Spawns usam contagem regressiva
// fracionária pra manter a taxa média original.
func relaxFallAdvance(st *relaxFallState, kind relaxFallKind, d float64) {
	st.simT += d

	// Rajada: sobe e desce com easing, nunca liga/desliga de uma vez.
	if st.gustLeft > 0 {
		st.gustLeft -= d
		if st.gustLeft < 0 {
			st.gustLeft = 0
		}
		f := 1 - st.gustLeft/st.gustDur
		st.gust = st.gustPeak * math.Sin(f*math.Pi)
	} else {
		st.gust *= math.Pow(0.9, d)
		if st.nextGust -= d; st.nextGust <= 0 {
			st.gustDur = float64(30 + rand.Intn(50))
			st.gustLeft = st.gustDur
			st.gustPeak = 4 + rand.Float64()*9
			if kind == relaxFallLeaves && rand.Intn(3) == 0 {
				st.gustPeak = -st.gustPeak // folha às vezes vai pro outro lado
			}
			st.nextGust = float64(70 + rand.Intn(140))
		}
	}

	if st.nextSpawn -= d; st.nextSpawn <= 0 && len(st.parts) < relaxFallMax(kind) {
		st.parts = append(st.parts, relaxFallSpawn(kind))
		st.nextSpawn = float64(4 + rand.Intn(10))
	}

	// Brisa de fundo por noise (contínua): usa o relógio contínuo da simulação,
	// equivalente ao antigo float64(tick)*0.012.
	breeze := relaxNoise(st.simT*0.12, 31) * 2.2

	live := st.parts[:0]
	for _, p := range st.parts {
		if p.rest > 0 { // folha pousada: descansa e some
			p.rest -= d
			if p.rest > 0 {
				live = append(live, p)
			}
			continue
		}
		p.swp += p.swSpeed * d
		p.spin += p.spdV * d
		wind := (st.gust + breeze) * (0.4 + 0.3*float64(p.layer))
		p.x += (p.vx + wind + p.sway*math.Sin(p.swp)*0.35) * d
		p.y += (p.vy + math.Cos(p.swp)*0.6) * d // bamboleio também na descida
		if p.x < -60 || p.x > 1060 {
			continue
		}
		if p.y >= 1000 {
			if kind == relaxFallLeaves {
				// Espalha um pouco a altura de pouso: com todas em 1000 elas se
				// encostavam e o chão virava uma faixa contínua.
				p.y = 962 + rand.Float64()*38
				p.rest = float64(12 + rand.Intn(40))
				live = append(live, p)
			}
			continue
		}
		live = append(live, p)
	}
	st.parts = live
}

// stepRelaxFall é um passo cheio de simulação (100ms). Mantido pro teste; o
// render anda por tempo real em relaxFallFrames.
func stepRelaxFall(st *relaxFallState, kind relaxFallKind) {
	if !st.inited {
		relaxFallInit(st, kind)
	}
	st.tick++
	relaxFallAdvance(st, kind, 1)
}

func relaxFallFrames(st *relaxFallState, kind relaxFallKind, width, height int, fade, now float64) ([]string, string) {
	if !st.inited {
		relaxFallInit(st, kind)
	}
	// A física anda pelo relógio de render: sub-passos por tempo real, ~30fps.
	for i, n := 0, st.clock.tick(now); i < n; i++ {
		relaxFallAdvance(st, kind, relaxSubDT/0.1)
	}
	w := maxInt(24, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	b := newRelaxBraille(w, h)
	sw, sh := float64(w*2-1), float64(h*4-1)

	// A pétala é menor e mais redonda; a folha é comprida e tem nervura.
	long, wide := 7.0, 3.2
	if kind == relaxFallPetals {
		long, wide = 4.8, 3.0
	}
	glyphs := relaxFallGlyphs[kind]

	for _, p := range st.parts {
		cx, cy := p.x*sw/1000, p.y*sh/1000
		// Camada de trás é menor e mais escura: é o que dá profundidade.
		depth := 0.62 + 0.19*float64(p.layer)
		a := long * depth
		// Tombo: de frente mostra a largura toda, de perfil vira um risco.
		bb := wide * depth * (0.22 + 0.78*math.Abs(math.Cos(p.spin)))
		ang := p.spin * 0.6
		if p.rest > 0 {
			a, bb, ang = long*depth, wide*depth*0.5, 0 // pousada, deitada
		}
		sn, cs := math.Sin(ang), math.Cos(ang)
		base := 1 + p.layer*2 + p.tone
		if p.rest > 0 && p.rest < 12 {
			base -= 2 // apagando no chão
		}

		for dy := -int(a) - 1; dy <= int(a)+1; dy++ {
			for dx := -int(a) - 1; dx <= int(a)+1; dx++ {
				fx, fy := float64(dx), float64(dy)
				ex, ey := (fx*cs+fy*sn)/a, (-fx*sn+fy*cs)/bb
				d2 := ex*ex + ey*ey
				if d2 > 1 {
					continue
				}
				lvl := base + int(3.0*(1-d2)) // miolo mais claro que a borda
				if kind == relaxFallLeaves && math.Abs(ey) < 0.22 {
					lvl -= 2 // nervura
				}
				b.set(int(cx)+dx, int(cy)+dy, minInt(maxInt(lvl, 0), relaxFallLevels-1))
			}
		}

		// Glifo pontual no miolo das maiores da frente, quando estão de cara
		// (não de perfil, senão o glifo destoa da silhueta fina). Discreto.
		if p.big && p.rest == 0 && math.Abs(math.Cos(p.spin)) > 0.6 {
			g := glyphs[int(math.Abs(p.spin/1.6))%len(glyphs)]
			lvl := minInt(maxInt(base+3, 0), relaxFallLevels-1)
			b.text(int(cx)/2, int(cy)/4, g, lvl)
		}
	}

	status := "folhas caindo"
	if kind == relaxFallPetals {
		status = "pétalas ao vento"
	}
	if math.Abs(st.gust) > 5 {
		status = "uma rajada passa"
	}
	return b.lines(relaxStyles(relaxFallRamps[kind], fade)), StyleMuted.Render(status)
}
