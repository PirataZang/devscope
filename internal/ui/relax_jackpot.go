package ui

import (
	"math"
	"math/rand"
)

// ── Jackpoint ─────────────────────────────────────────────────────────────────
//
// Uma caça-níquel miúda dentro do terminal. Durante o giro a tela tem três
// caixas e mais nada — sem moldura, sem placar, sem texto. Entrar na cena já
// começa um sorteio, e ela segue sozinha: gira, para, mostra o prêmio, limpa,
// gira de novo.
//
// As camadas são separadas de propósito, porque cada uma muda por um motivo
// diferente:
//
//	relaxJpSpin   sorteio puro, recebe o gerador de fora (teste fixa a semente)
//	relaxJpReel   o rolo: de onde parte, onde para e a curva de freio
//	relaxPen    o pincel: os símbolos são vetor num quadrado normalizado
//	relaxJpPart   partícula genérica da cena, com um desenho por tipo
//	fases         máquina de estados do tempo (giro → espera → prêmio → limpa)
//
// Símbolo novo é uma entrada em relaxJpArts e uma cor em relaxJpStops; prêmio
// novo é um case em relaxJpShowDur e outro em relaxJpShow. O renderer não muda.
//
// Sobre o desenho na tela: quem cuida de cursor, quadro parcial e ANSI é o
// renderer do app (Bubble Tea faz diff de linha e só reescreve o que mudou), e
// o motor do Relax já dá o tique fixo de 100ms e o crossfade. A cena só devolve
// as linhas — entrar por baixo disso seria criar um segundo renderer.

// ── Símbolos ──────────────────────────────────────────────────────────────────

const (
	jpCherry = iota
	jpCoin
	jpJack
	jpTNT
	jpBanana
	jpSymbols
)

// ── Sorteio ───────────────────────────────────────────────────────────────────
//
// relaxJpTriple é a chance de CADA trinca, declarada de frente. O que sobra
// (1 − soma ≈ 0,902) é sorteio comum com rejeição de trinca, então a soma é
// exatamente a probabilidade de sair três iguais — nada de rolo manipulado
// depois do fato: o rolo para no que este sorteio devolveu.
//
//	três iguais ≈ 1/10,2      J J J = 1/30 (o mais raro dos prêmios)
var relaxJpTriple = [jpSymbols]float64{
	jpCherry: 1.0 / 55,
	jpCoin:   1.0 / 55,
	jpJack:   1.0 / 30,
	jpTNT:    1.0 / 70,
	jpBanana: 1.0 / 70,
}

// relaxJpSpin sorteia as três posições. O gerador vem de fora para o teste
// poder fixar a semente sem encostar em nada da apresentação.
func relaxJpSpin(rng *rand.Rand) [3]int8 {
	p := rng.Float64()
	for s, chance := range relaxJpTriple {
		if p -= chance; p < 0 {
			return [3]int8{int8(s), int8(s), int8(s)}
		}
	}
	// Rejeição: sem ela, o sorteio comum devolveria trinca de vez em quando e a
	// tabela acima deixaria de valer.
	for i := 0; i < 64; i++ {
		r := [3]int8{int8(rng.Intn(jpSymbols)), int8(rng.Intn(jpSymbols)), int8(rng.Intn(jpSymbols))}
		if r[0] != r[1] || r[1] != r[2] {
			return r
		}
	}
	return [3]int8{jpCherry, jpCoin, jpJack}
}

// ── Paleta ────────────────────────────────────────────────────────────────────

const relaxJpTones = 4 // tons por símbolo: 0 é sombra, 3 é o alto da luz

const (
	jpPalSym   = 0
	jpPalBox   = jpPalSym + jpSymbols*relaxJpTones
	jpPalCard  = jpPalBox + 3
	jpPalPip   = jpPalCard + 3
	jpPalChip  = jpPalPip + 2
	jpPalFire  = jpPalChip + 4
	jpPalLaugh = jpPalFire + 6
	jpPalN     = jpPalLaugh + 3
)

var relaxJpStops = [jpSymbols][]string{
	jpCherry: {"#2A0508", "#7E1018", "#C81E2C", "#FF7A78"},
	jpCoin:   {"#3A2A08", "#8C6616", "#DCA62C", "#FFEEA8"},
	jpJack:   {"#181030", "#402C76", "#7052C6", "#CBB0FF"},
	jpTNT:    {"#2A0A06", "#7E2608", "#C8420E", "#FFA850"},
	jpBanana: {"#2E2606", "#7E6812", "#D8B62C", "#FFF3A6"},
}

var relaxJpPal = func() []relaxColor {
	out := make([]relaxColor, jpPalN)
	for s, stops := range relaxJpStops {
		copy(out[jpPalSym+s*relaxJpTones:], relaxRamp(stops, relaxJpTones))
	}
	copy(out[jpPalBox:], relaxRamp([]string{"#18202A", "#2C3A48", "#516576"}, 3))
	copy(out[jpPalCard:], relaxRamp([]string{"#4A4A52", "#ACACB4", "#F4F4F8"}, 3))
	copy(out[jpPalPip:], relaxRamp([]string{"#8C1020", "#E42C3C"}, 2))
	copy(out[jpPalChip:], relaxRamp([]string{"#10382A", "#1C7454", "#2FA878", "#5CE0A8"}, 4))
	copy(out[jpPalFire:], relaxRamp([]string{"#2A0A04", "#7C2006", "#C64C0A", "#F28C18", "#FFD474", "#FFFFFF"}, 6))
	copy(out[jpPalLaugh:], relaxRamp([]string{"#241E06", "#5E5010", "#A88C22"}, 3))
	return out
}()

func relaxJpSymLvl(sym int8, tone int) int {
	return jpPalSym + int(sym)*relaxJpTones + minInt(maxInt(tone, 0), relaxJpTones-1)
}

// ── Desenho dos cinco símbolos ────────────────────────────────────────────────

// ORDEM IMPORTA: o Braille não sobrescreve ponto aceso, então cada arte é
// desenhada da FRENTE pro FUNDO. Realce, aro e fagulha vêm antes do corpo — na
// ordem natural eles seriam engolidos pela massa desenhada primeiro.
var relaxJpArts = [jpSymbols]func(relaxPen){
	jpCherry: func(p relaxPen) {
		p.stroke(-0.30, -0.06, 0.02, -0.84, 0.10, 1)
		p.stroke(0.30, 0.08, 0.08, -0.84, 0.10, 1)
		p.ellipse(0.46, -0.72, 0.26, 0.13, 2) // folha
		p.dot(-0.52, 0.16, 0.13, 3)           // brilho, antes da fruta
		p.dot(0.22, 0.30, 0.11, 3)
		p.disc(-0.38, 0.34, 0.44, 2)
		p.disc(0.36, 0.46, 0.38, 2)
	},
	jpCoin: func(p relaxPen) {
		p.arc(0, 0, 0.44, 3.5, 5.2, 0.16, 3) // reflexo
		p.rect(-0.10, -0.30, 0.10, 0.30, 3)  // cifra
		p.ring(0, 0, 0.62, 0.13, 3)          // aro interno
		p.disc(0, 0, 0.80, 2)
	},
	jpJack: func(p relaxPen) {
		p.rect(-0.28, -0.76, 0.60, -0.54, 3)
		p.rect(0.18, -0.76, 0.44, 0.22, 3)
		p.arc(-0.04, 0.20, 0.44, 0, math.Pi, 0.24, 3)
	},
	jpTNT: func(p relaxPen) {
		// Bomba redonda com estopim: o feixe de bastões perde os vãos quando o
		// símbolo encolhe, a esfera com fagulha continua legível.
		p.disc(0.44, -0.92, 0.14, 3) // fagulha
		for i := 0; i < 6; i++ {
			a := float64(i)/6*2*math.Pi + 0.3
			p.dot(0.44+math.Cos(a)*0.27, -0.92+math.Sin(a)*0.27, 0.05, 3)
		}
		p.stroke(0.02, -0.50, 0.28, -0.64, 0.09, 1) // estopim
		p.stroke(0.28, -0.64, 0.36, -0.84, 0.09, 1)
		p.rect(-0.18, -0.58, 0.18, -0.40, 1) // bocal
		p.dot(-0.26, -0.02, 0.15, 3)         // realce, antes do corpo
		p.disc(0, 0.24, 0.74, 2)
	},
	jpBanana: func(p relaxPen) {
		// Cacho de três. Uma banana sozinha, grossa o bastante pra ler em vinte
		// subpontos, vira batata — o que diz "banana" nesse tamanho é o leque
		// saindo de um cabo só.
		// sweep varre a banana: linha de centro descendo e deitando pra fora, e
		// a espessura saindo pela normal dela. edge desenha a mesma varredura
		// um tico mais gorda e escura — como o Braille não sobrescreve, ela só
		// pega a franja, e é esse contorno que separa uma banana da outra. Sem
		// ele o cacho vira uma massa só.
		sweep := func(rot, fat float64, tone func(u float64) int) {
			cr, sr := math.Cos(rot), math.Sin(rot)
			for t := 0.03; t <= 1; t += 0.008 {
				lx, ly := -0.66*t*t, 1.34*t
				nx, ny := 1.34, 1.32*t
				nl := math.Hypot(nx, ny)
				w := fat * 0.155 * math.Pow(math.Sin(math.Pi*math.Pow(t, 0.72)), 0.5)
				for u := -1.0; u <= 1.001; u += 0.2 {
					x := lx + u*w*nx/nl
					y := ly + u*w*ny/nl
					p.dot(x*cr-y*sr, x*sr+y*cr-0.62, p.step*0.85, tone(u))
				}
			}
		}
		body := func(u float64) int {
			switch {
			case u < -0.3:
				return 3 // quina de cima
			case u > 0.45:
				return 1 // barriga na sombra
			}
			return 2
		}
		dark := func(float64) int { return 0 }
		p.dot(0, -0.74, 0.13, 1) // cabo, na frente de todas
		p.stroke(0, -0.74, 0.06, -0.94, 0.09, 1)
		for _, rot := range [3]float64{0, -0.50, 0.50} { // a do meio na frente
			sweep(rot, 1, body)
			sweep(rot, 1.5, dark)
		}
	},
}

// relaxJpFit mede cada arte uma vez e guarda centro e escala. Assim a arte pode
// ser desenhada solta — inclinada, fora do meio — que o símbolo entra centrado
// e do mesmo tamanho dos outros. Sem isso cada desenho tem de acertar o próprio
// enquadramento na mão, e foi assim que a banana nasceu jogada num canto.
var relaxJpFit = func() [jpSymbols][3]float64 {
	var out [jpSymbols][3]float64
	for s, art := range relaxJpArts {
		x0, y0 := math.Inf(1), math.Inf(1)
		x1, y1 := math.Inf(-1), math.Inf(-1)
		art(relaxPen{step: 0.03, put: func(x, y float64, _ int) {
			x0, y0 = math.Min(x0, x), math.Min(y0, y)
			x1, y1 = math.Max(x1, x), math.Max(y1, y)
		}})
		out[s] = [3]float64{(x0 + x1) / 2, (y0 + y1) / 2, 2 / math.Max(x1-x0, y1-y0)}
	}
	return out
}()

func (p relaxPen) ring(cx, cy, r, th float64, tone int) {
	p.arc(cx, cy, r, 0, 2*math.Pi, th, tone)
}

// ── Rolo ──────────────────────────────────────────────────────────────────────
//
// O rolo anda em "símbolos": a parte inteira de pos diz qual está no centro e a
// fracionária é o deslocamento, então a tira desliza de verdade em vez de
// trocar de glifo. Ele parte de onde está e cai numa parada INTEIRA cujo resto
// por 5 é o símbolo sorteado — por isso o freio nunca precisa mentir.

type relaxJpReel struct {
	pos, vel float64
	from, to float64
	dur      int
	stopped  bool
}

// relaxJpBrake é a curva do giro: arranca a 3,4× a média e freia longo.
func relaxJpBrake(u float64) float64 { return 1 - math.Pow(1-clamp01(u), 3.4) }

// relaxJpSettle é o tranco do fim: o rolo passa um quarto de símbolo do ponto e
// volta. É esse exagero curto que dá peso de máquina de verdade.
func relaxJpSettle(u float64) float64 {
	return 0.26 * math.Sin(math.Pi*clamp01((u-0.78)/0.22))
}

func (r *relaxJpReel) launch(sym int8, dur int, rng *rand.Rand) {
	r.from, r.dur, r.stopped = r.pos, dur, false
	turns := 15.0 + rng.Float64()*4
	to := math.Ceil(r.pos + turns)
	for int(math.Mod(to, jpSymbols)) != int(sym) {
		to++
	}
	r.to = to
}

func (r *relaxJpReel) step(t int) {
	u := clamp01(float64(t) / float64(maxInt(1, r.dur)))
	p := r.from + (r.to-r.from)*relaxJpBrake(u) + relaxJpSettle(u)
	r.vel, r.pos = p-r.pos, p
	r.stopped = t >= r.dur
}

// ── Partículas ────────────────────────────────────────────────────────────────

const (
	jpPartCherry = iota
	jpPartCoin
	jpPartCard
	jpPartChip
	jpPartSpark
)

type relaxJpPart struct {
	x, y   float64 // em subpontos
	vx, vy float64
	spin   float64
	spv    float64
	size   float64
	kind   int8
	tone   int8
	life   float64 // 1 → 0; só a faísca morre de velhice
	drag   float64
}

// ── Estado ────────────────────────────────────────────────────────────────────

const (
	jpPhaseSpin = iota
	jpPhaseHold // resultado parado na tela
	jpPhaseShow // animação do prêmio
	jpPhaseRest // tela limpa antes do próximo sorteio
)

const (
	jpHoldDur = 12 // 1,2s com o resultado parado
	jpRestDur = 7
)

type relaxJackpotState struct {
	inited bool
	tick   int
	rng    *rand.Rand

	phase  int8
	t      int
	result [3]int8
	reel   [3]relaxJpReel
	parts  []relaxJpPart

	flash float64 // clarão do TNT
	shock float64 // raio da onda de choque, em fração da tela
	laugh int     // quantos "HA" já entraram
	won   int8    // símbolo da trinca, -1 quando não houve
}

func stepRelaxJackpot(st *relaxJackpotState) {
	if !st.inited {
		st.inited = true
		// Semente do gerador global: cada entrada na cena é um sorteio novo,
		// mas o gerador da cena é próprio, e é ele que o teste substitui.
		st.rng = rand.New(rand.NewSource(rand.Int63()))
		relaxJpNewSpin(st)
	}
	st.tick++
	st.t++

	switch st.phase {
	case jpPhaseSpin:
		done := true
		for i := range st.reel {
			st.reel[i].step(st.t)
			done = done && st.reel[i].stopped
		}
		if done {
			st.phase, st.t = jpPhaseHold, 0
			st.won = -1
			if st.result[0] == st.result[1] && st.result[1] == st.result[2] {
				st.won = st.result[0]
			}
		}
	case jpPhaseHold:
		if st.t >= jpHoldDur {
			st.phase, st.t = jpPhaseShow, 0
			st.flash, st.shock, st.laugh = 0, 0, 0
			st.parts = st.parts[:0]
		}
	case jpPhaseShow:
		relaxJpShowStep(st)
		if st.t >= relaxJpShowDur(st.won) {
			st.phase, st.t = jpPhaseRest, 0
		}
	default:
		if st.t >= jpRestDur {
			relaxJpNewSpin(st)
		}
	}
	relaxJpStepParts(st)
}

func relaxJpNewSpin(st *relaxJackpotState) {
	st.result = relaxJpSpin(st.rng)
	st.phase, st.t, st.won = jpPhaseSpin, 0, -1
	st.parts = st.parts[:0]
	st.flash, st.shock, st.laugh = 0, 0, 0
	for i := range st.reel {
		// Paradas escalonadas: a primeira para, a segunda ainda gira, a
		// terceira segura o resultado. É o escalonamento que dá suspense.
		st.reel[i].launch(st.result[i], 30+i*8+st.rng.Intn(4), st.rng)
	}
}

// relaxJpShowDur é quanto dura cada prêmio, em passos de 100ms.
func relaxJpShowDur(won int8) int {
	switch won {
	case jpCherry, jpCoin:
		return 46
	case jpJack:
		return 72
	case jpTNT:
		return 40
	case jpBanana:
		return 48
	}
	return 4 // sem trinca: quase nada, só um respiro
}

// ── Fábrica de partículas ─────────────────────────────────────────────────────

func relaxJpFall(st *relaxJackpotState, kind int8, sw float64) relaxJpPart {
	r := st.rng
	p := relaxJpPart{
		x:    r.Float64() * sw,
		y:    -r.Float64() * 26,
		vx:   (r.Float64() - 0.5) * 0.5,
		vy:   0.5 + r.Float64()*1.6,
		spin: r.Float64() * 6.28,
		spv:  (r.Float64() - 0.5) * 0.46,
		kind: kind,
		life: 1,
		tone: int8(2 + r.Intn(2)),
		size: 1,
	}
	// Cada tipo cai com um peso: sem isso a chuva desce em bloco, que é o
	// jeito mais rápido de a queda parecer um sprite deslizando.
	switch kind {
	case jpPartCard:
		p.vy *= 0.72
		p.spv *= 1.7
		p.size = 1.15
	case jpPartChip:
		p.vy *= 0.9
	case jpPartCherry:
		p.size = 0.9
	}
	return p
}

func relaxJpStepParts(st *relaxJackpotState) {
	live := st.parts[:0]
	for _, p := range st.parts {
		p.vy += 0.16
		if p.drag > 0 {
			p.vx *= p.drag
			p.vy *= p.drag
		}
		p.x += p.vx
		p.y += p.vy
		p.spin += p.spv
		if p.kind == jpPartSpark {
			p.life -= 0.045
		}
		// Some quem já passou do palco mais alto que faz sentido: a simulação
		// não conhece a altura do terminal, e sem esse teto a chuva do jackpot
		// seguiria sendo simulada muito depois de sair da tela.
		if p.life > 0 && p.y < 420 {
			live = append(live, p)
		}
	}
	st.parts = live
}

// ── Passo de cada prêmio ──────────────────────────────────────────────────────

func relaxJpShowStep(st *relaxJackpotState) {
	// A largura em subpontos só é conhecida no render; 240 é a referência do
	// palco largo, e o render reescala. Guardar a largura no estado acoplaria
	// a simulação ao tamanho do terminal.
	const ref = 240.0
	spawn := func(kind int8, n int) {
		for i := 0; i < n && len(st.parts) < 900; i++ {
			st.parts = append(st.parts, relaxJpFall(st, kind, ref))
		}
	}
	switch st.won {
	case jpCherry:
		if st.t < 30 && st.tick%2 == 0 {
			spawn(jpPartCherry, 2)
		}
	case jpCoin:
		if st.t < 32 {
			spawn(jpPartCoin, 3)
		}
	case jpJack:
		// Jackpot é exagero: carta e ficha juntas, e muito mais delas.
		if st.t < 52 {
			spawn(jpPartCard, 2)
			spawn(jpPartChip, 3)
			if st.tick%3 == 0 {
				spawn(jpPartCoin, 2)
			}
		}
	case jpTNT:
		switch {
		case st.t == 6:
			st.flash = 1
		case st.t == 8:
			// Estouro: tudo sai do centro com arrasto, então as partículas
			// desaceleram como se o ar segurasse — explosão sem arrasto vira
			// chuva de meteoro.
			for i := 0; i < 190; i++ {
				a := st.rng.Float64() * 6.28
				v := 1.2 + st.rng.Float64()*5.4
				st.parts = append(st.parts, relaxJpPart{
					x: ref / 2, y: 0, vx: math.Cos(a) * v, vy: math.Sin(a)*v - 0.6,
					kind: jpPartSpark, life: 0.7 + st.rng.Float64()*0.3,
					tone: int8(st.rng.Intn(6)), drag: 0.90,
				})
			}
		}
		st.flash *= 0.72
		if st.t >= 8 {
			st.shock = float64(st.t-8) * 0.055
		}
	case jpBanana:
		// HA, HAHA, HAHAHA: entra de dois em dois.
		st.laugh = minInt(3, 1+st.t/9)
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxJackpotFrames(st *relaxJackpotState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxJackpot(st)
	}
	w := maxInt(18, minInt(width, 110))
	h := maxInt(7, minInt(height, 30))
	// Voto, não média: a paleta é indexada por família (cereja é vermelha,
	// moeda é dourada) e média entre duas famílias dá a cor de uma terceira.
	b := newRelaxBrailleVote(w, h)
	relaxJpDraw(st, b)
	// Nada de texto durante o sorteio: um ponto é a única coisa que a cena
	// escreve, e ela escreve sempre o mesmo.
	return b.lines(relaxStyles(relaxJpPal, fade)), StyleMuted.Render("·")
}

func relaxJpDraw(st *relaxJackpotState, b *relaxBraille) {
	sw, sh := b.w*2, b.h*4
	// Caixa quadrada, três delas com folga de 0,42 de lado entre si.
	side := math.Min(float64(sw)*0.92/3.84, float64(sh)*0.74)
	gap := side * 0.42
	cy := float64(sh) / 2

	// No TNT a máquina some no estouro: sem caixa, sem símbolo, só o clarão.
	if !(st.won == jpTNT && st.phase == jpPhaseShow && st.t >= 8) {
		for i := 0; i < 3; i++ {
			cx := float64(sw)/2 + (float64(i)-1)*(side+gap)
			relaxJpBox(b, cx, cy, side)
			relaxJpReelDraw(st, b, i, cx, cy, side)
		}
	}
	// O HAHAHA vem DEPOIS da máquina: o Braille não sobrescreve, então ele só
	// preenche o que sobrou e fica atrás. Desenhado antes, ele comia o símbolo
	// dentro da caixa — que é o oposto de presença de fundo.
	if st.won == jpBanana && st.phase == jpPhaseShow {
		relaxJpLaugh(st, b, sw, sh, func(x, y float64) bool {
			for i := 0; i < 3; i++ {
				cx := float64(sw)/2 + (float64(i)-1)*(side+gap)
				if math.Abs(x-cx) < side/2+1 && math.Abs(y-cy) < side/2+1 {
					return false
				}
			}
			return true
		})
	}
	if st.won == jpTNT && st.phase == jpPhaseShow {
		relaxJpBlast(st, b, sw, sh)
	}
	relaxJpDrawParts(st, b, sw, sh)
}

// relaxJpBox é a caixa: um traço fino com os cantos acesos. Fechada demais ela
// vira interface; aberta demais o símbolo fica solto no escuro.
func relaxJpBox(b *relaxBraille, cx, cy, side float64) {
	half := side / 2
	x0, x1 := int(cx-half), int(cx+half)
	y0, y1 := int(cy-half), int(cy+half)
	corner := int(side * 0.22)
	for x := x0; x <= x1; x++ {
		lvl := jpPalBox
		if x-x0 < corner || x1-x < corner {
			lvl = jpPalBox + 2
		}
		b.set(x, y0, lvl)
		b.set(x, y1, lvl)
	}
	for y := y0; y <= y1; y++ {
		lvl := jpPalBox
		if y-y0 < corner || y1-y < corner {
			lvl = jpPalBox + 2
		}
		b.set(x0, y, lvl)
		b.set(x1, y, lvl)
	}
}

// relaxJpReelDraw desenha a janela do rolo: o símbolo do centro e os vizinhos
// espiando por cima e por baixo, recortados na caixa. Em velocidade alta entram
// fantasmas em posições intermediárias — é borrão de movimento de verdade, e é
// o que faz o rolo parecer girar em vez de piscar entre glifos.
func relaxJpReelDraw(st *relaxJackpotState, b *relaxBraille, i int, cx, cy, side float64) {
	r := st.reel[i]
	pitch := side * 0.94
	inner := side/2 - 1.5
	// Índice na tira: a tira é 0..4 repetida, então o símbolo é o resto.
	sym := func(idx int) int8 {
		return int8((idx%jpSymbols + jpSymbols) % jpSymbols)
	}
	ghosts := minInt(4, int(math.Abs(r.vel)*1.7))
	for g := ghosts; g >= 0; g-- {
		// O fantasma fica atrás no tempo: pos menos um pedaço do que o rolo
		// andou neste passo.
		pos := r.pos - r.vel*float64(g)/float64(ghosts+1)
		fr := pos - math.Floor(pos)
		dim := g // cada fantasma é um tom mais apagado
		for k := -1; k <= 1; k++ {
			off := (float64(k) - fr) * pitch
			if math.Abs(off) > inner+pitch*0.6 {
				continue
			}
			relaxJpSymbol(b, sym(int(math.Floor(pos))+k), cx, cy+off, side*0.72, dim,
				func(x, y float64) bool {
					return math.Abs(x-cx) <= inner && math.Abs(y-cy) <= inner
				})
		}
	}
}

// relaxJpSymbol põe um símbolo na tela. dim apaga tons (fantasma do borrão) e
// clip recorta na janela do rolo.
func relaxJpSymbol(b *relaxBraille, sym int8, cx, cy, size float64, dim int, clip func(x, y float64) bool) {
	half := size / 2
	fit := relaxJpFit[sym]
	pen := relaxPen{
		step: 1 / (half * fit[2]),
		put: func(x, y float64, tone int) {
			px, py := cx+(x-fit[0])*fit[2]*half, cy+(y-fit[1])*fit[2]*half
			if clip != nil && !clip(px, py) {
				return
			}
			b.set(int(px), int(py), relaxJpSymLvl(sym, tone-dim))
		},
	}
	relaxJpArts[sym](pen)
}

// ── Prêmios ───────────────────────────────────────────────────────────────────

func relaxJpDrawParts(st *relaxJackpotState, b *relaxBraille, sw, sh int) {
	const ref = 240.0
	k := float64(sw) / ref // a simulação corre numa largura de referência
	for _, p := range st.parts {
		x, y := p.x*k, p.y*k
		if x < -8 || x > float64(sw)+8 || y > float64(sh)+8 {
			continue
		}
		pen := relaxPen{step: 1, put: func(dx, dy float64, tone int) {
			b.set(int(x+dx), int(y+dy), tone)
		}}
		relaxJpPartArt(pen, p, k)
	}
}

// relaxJpPartArt desenha uma partícula. Moeda e ficha achatam com o giro — é a
// largura encolhendo que faz o disco virar de perfil; carta é um retângulo
// tombando; faísca é um ponto que esfria.
func relaxJpPartArt(pen relaxPen, p relaxJpPart, k float64) {
	sz := p.size * k * 1.9
	switch p.kind {
	case jpPartCoin:
		w := math.Max(0.35, math.Abs(math.Cos(p.spin))) * sz
		pen.ellipse(0, 0, w, sz, jpPalSym+jpCoin*relaxJpTones+2)
		pen.ellipse(0, 0, w*0.45, sz*0.5, jpPalSym+jpCoin*relaxJpTones+3)
	case jpPartCherry:
		pen.ellipse(0, 0, sz, sz, jpPalSym+jpCherry*relaxJpTones+2)
		pen.ellipse(-sz*0.3, -sz*0.3, sz*0.28, sz*0.28, jpPalSym+jpCherry*relaxJpTones+3)
	case jpPartChip:
		w := math.Max(0.30, math.Abs(math.Cos(p.spin))) * sz * 1.1
		pen.ellipse(0, 0, w, sz, jpPalChip+2)
		pen.ellipse(0, 0, w*0.55, sz*0.55, jpPalChip+3)
	case jpPartCard:
		// Retângulo tombando: os quatro cantos girados, e o naipe no meio.
		cs, sn := math.Cos(p.spin), math.Sin(p.spin)
		hw, hh := sz*0.9, sz*1.35
		var q [4][2]float64
		for i, c := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			q[i] = [2]float64{c[0]*hw*cs - c[1]*hh*sn, c[0]*hw*sn + c[1]*hh*cs}
		}
		pen.quadFill(q, jpPalCard+2)
		pen.dot(0, 0, sz*0.32, jpPalPip+1)
	default:
		t := int(clamp01(p.life) * 5)
		pen.dot(0, 0, math.Max(0.6, sz*0.45*p.life), jpPalFire+minInt(maxInt(t, 0), 5))
	}
}

// relaxJpBlast é o TNT: clarão que toma a tela e onda de choque abrindo. As
// partículas do estouro são as mesmas do sistema — só nascem do centro.
func relaxJpBlast(st *relaxJackpotState, b *relaxBraille, sw, sh int) {
	if st.flash > 0.02 {
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				if relaxHalftone(x, y) < st.flash*0.9 {
					b.set(x, y, jpPalFire+5)
				}
			}
		}
	}
	if st.shock <= 0 || st.shock > 1.5 {
		return
	}
	cx, cy := float64(sw)/2, float64(sh)/2
	r := st.shock * float64(sw) * 0.55
	fade := clamp01(1.4 - st.shock*1.6)
	n := maxInt(24, int(r*3))
	for i := 0; i < n; i++ {
		a := float64(i) / float64(n) * 2 * math.Pi
		for t := -1.5; t <= 1.5; t += 0.8 {
			x, y := cx+math.Cos(a)*(r+t), cy+math.Sin(a)*(r+t)*0.75
			if relaxHalftone(int(x), int(y)) > fade {
				continue
			}
			b.set(int(x), int(y), jpPalFire+2+minInt(int(fade*3), 3))
		}
	}
}

// relaxJpLaugh é o HAHAHA da banana: presença no fundo, não recado de
// interface. Entra de dois em dois e apaga junto com o fim da fase.
func relaxJpLaugh(st *relaxJackpotState, b *relaxBraille, sw, sh int, free func(x, y float64) bool) {
	word := []byte("HAHAHA")[:minInt(6, st.laugh*2)]
	if len(word) == 0 {
		return
	}
	u := float64(st.t) / float64(relaxJpShowDur(jpBanana))
	// Nasce, firma e some. O teto de meia densidade é o que separa presença de
	// fundo de recado na tela: sólido, o HAHAHA vira legenda e engole a máquina.
	alpha := clamp01(math.Sin(u*math.Pi)*1.6) * 0.52
	lvl := jpPalLaugh + minInt(int(alpha*5), 2)
	span := float64(sw) * 0.82
	lw := span / float64(len(word))
	lh := math.Min(float64(sh)*0.66, lw*1.9)
	pen := relaxPen{step: 1, put: func(x, y float64, tone int) {
		if free(x, y) && relaxHalftone(int(x), int(y)) < alpha {
			b.set(int(x), int(y), lvl)
		}
	}}
	for i, ch := range word {
		cx := (float64(sw)-span)/2 + (float64(i)+0.5)*lw
		relaxJpLetter(pen, ch, cx, float64(sh)/2, lw*0.64, lh, 0)
	}
}

// relaxJpLetter tem duas letras porque a cena só precisa de duas. Fonte de
// verdade seria peso morto.
func relaxJpLetter(p relaxPen, ch byte, cx, cy, w, h float64, tone int) {
	bar := w * 0.22
	switch ch {
	case 'H':
		p.rect(cx-w/2, cy-h/2, cx-w/2+bar, cy+h/2, tone)
		p.rect(cx+w/2-bar, cy-h/2, cx+w/2, cy+h/2, tone)
		p.rect(cx-w/2, cy-h*0.10, cx+w/2, cy+h*0.10, tone)
	case 'A':
		p.stroke(cx-w/2, cy+h/2, cx-w*0.05, cy-h/2, bar, tone)
		p.stroke(cx+w/2, cy+h/2, cx+w*0.05, cy-h/2, bar, tone)
		p.rect(cx-w*0.26, cy+h*0.04, cx+w*0.26, cy+h*0.24, tone)
	}
}
