package ui

import (
	"math"
	"math/rand"
)

// ── Motor V4 ──────────────────────────────────────────────────────────────────
//
// Corte de um V4 em funcionamento: dois Vs, quatro cilindros, com pistão,
// biela, virabrequim, VÁLVULAS e COMANDO à vista. Dá a partida, pega e cai na
// marcha lenta, com aceleradas de vez em quando. Fundo preto, cabeçote em
// vermelho.
//
// Eram oito cilindros antes, e é por isso que agora são quatro: com quatro Vs
// lado a lado cada cilindro ficava com um quinto da largura do terminal, e
// nesse tamanho válvula, anel e frente de chama não sobrevivem. Metade dos
// cilindros, o dobro de peça visível.
//
// A cinemática é a de verdade, não seno: a posição do pistão sai da BIELA-
// MANIVELA. Com o munhão em p e o eixo do cilindro em u, o pistão está em
//
//	d = p·u + √(L² − r² + (p·u)²)
//
// que é a distância ao longo do eixo em que a biela de comprimento L alcança o
// munhão. É essa raiz que faz o pistão passar depressa pelo ponto morto
// superior e demorar no inferior — seno puro sobe e desce igual, e é isso que
// faz animação de motor parecer bomba d'água.
//
// Quatro tempos de verdade também: cada cilindro queima a cada DUAS voltas, e a
// ordem de ignição não é escolhida — ela cai da geometria, porque o estouro
// acontece quando aquele pistão chega no ponto morto superior no tempo certo.
//
// O regime é câmera lenta assumida: a simulação anda a 10 passos por segundo e
// marcha lenta de verdade seriam 13 voltas por segundo, o que só daria borrão.
// Aqui a lenta é ~1 volta por segundo — é a mesma escolha de qualquer animação
// de corte de motor.

const (
	relaxV4Vs    = 2     // dois Vs, quatro cilindros
	relaxV4Tilt  = 0.46  // meia abertura do V, em radianos
	relaxV4Crank = 0.23  // raio da manivela
	relaxV4Rod   = 0.82  // comprimento da biela
	relaxV4Bore  = 0.300 // meia-largura do cilindro
	relaxV4Gap   = 2.60  // distância entre os Vs
	relaxV4Deck  = 1.28  // fim do curso, onde começa o cabeçote
	relaxV4Head  = 0.62  // altura do cabeçote, medida no eixo do cilindro
	relaxV4Seat  = 0.16  // afastamento das duas válvulas em relação ao eixo
)

// ── Paleta ────────────────────────────────────────────────────────────────────

const (
	v4SteelN = 6
	v4PistN  = 4
	v4RedN   = 4
	v4FireN  = 5
	v4SmokeN = 3
)

const (
	v4Steel = 0
	v4Pist  = v4Steel + v4SteelN
	v4Red   = v4Pist + v4PistN
	v4Fire  = v4Red + v4RedN
	v4Smoke = v4Fire + v4FireN
	v4PalN  = v4Smoke + v4SmokeN
)

var relaxV4Pal = func() []relaxColor {
	out := make([]relaxColor, v4PalN)
	copy(out[v4Steel:], relaxRamp([]string{"#0A0C10", "#161A22", "#252C38", "#3A4453", "#566374", "#7C8A9C"}, v4SteelN))
	copy(out[v4Pist:], relaxRamp([]string{"#3E4652", "#6E7A8A", "#A2AFC0", "#DCE6F2"}, v4PistN))
	copy(out[v4Red:], relaxRamp([]string{"#2C0407", "#7A0C12", "#C4161F", "#FF4D4D"}, v4RedN))
	copy(out[v4Fire:], relaxRamp([]string{"#8A0E06", "#D93A08", "#F58410", "#FFD24A", "#FFFFFF"}, v4FireN))
	copy(out[v4Smoke:], relaxRamp([]string{"#14161C", "#252A33", "#3C444F"}, v4SmokeN))
	return out
}()

// ── Cilindro ──────────────────────────────────────────────────────────────────

type relaxV4Cyl struct {
	cx     float64 // centro do virabrequim do V a que ele pertence
	ux, uy float64 // eixo do cilindro, unitário, apontando pro cabeçote
	off    float64 // defasagem do munhão deste V
	rel    float64 // ângulo relativo no passo anterior, pra achar o PMS
	power  bool    // alterna a cada PMS: quatro tempos queima em um de dois
	fire   float64 // brilho da queima, decai sozinho
}

type relaxV4Puff struct {
	x, y, vx, vy, life, size float64
}

// ── Estado ────────────────────────────────────────────────────────────────────

const (
	v4Crank = iota // arranque: o motor de partida girando
	v4Catch        // pegou: sobe de giro e assenta
	v4Idle         // marcha lenta, com acelerada de vez em quando
)

type relaxV4State struct {
	inited bool
	tick   int
	stage  int8
	t      int

	ang   float64 // ângulo do virabrequim
	rpm   float64 // radianos por passo
	want  float64
	cyl   [relaxV4Vs * 2]relaxV4Cyl
	puffs []relaxV4Puff
	shake float64
	blip  int
	next  int
}

func relaxV4Init(st *relaxV4State) {
	st.inited = true
	// Munhões a 0° e 180°: com dois Vs, é o que reparte as quatro explosões em
	// intervalos iguais dentro das duas voltas do ciclo.
	pins := [relaxV4Vs]float64{0, math.Pi}
	for v := 0; v < relaxV4Vs; v++ {
		cx := (float64(v) - float64(relaxV4Vs-1)/2) * relaxV4Gap
		for bank := 0; bank < 2; bank++ {
			a := relaxV4Tilt
			if bank == 0 {
				a = -a
			}
			st.cyl[v*2+bank] = relaxV4Cyl{
				cx: cx, off: pins[v],
				ux: math.Sin(a), uy: math.Cos(a),
				power: v%2 == bank, // metade começa no tempo de força
			}
		}
	}
	st.want = 0.30
	st.next = 70 + rand.Intn(90)
}

// relaxV4Piston devolve a distância do pistão ao centro do virabrequim, medida
// no eixo do cilindro, e a posição do munhão.
func relaxV4Piston(c relaxV4Cyl, ang float64) (float64, float64, float64) {
	pa := ang + c.off
	px, py := relaxV4Crank*math.Cos(pa), relaxV4Crank*math.Sin(pa)
	dp := px*c.ux + py*c.uy
	return dp + math.Sqrt(math.Max(0, relaxV4Rod*relaxV4Rod-relaxV4Crank*relaxV4Crank+dp*dp)), px, py
}

// relaxV4Cycle devolve onde o cilindro está no ciclo de quatro tempos, em
// radianos de 0 a 4π: 0 é o PMS de força. rel é meia história — ele volta a
// zero a cada volta —, e é o power que diz em qual das duas voltas estamos.
func relaxV4Cycle(c relaxV4Cyl) float64 {
	if c.power {
		return c.rel // força e escape
	}
	return c.rel + 2*math.Pi // admissão e compressão
}

// relaxV4Lift é o perfil do came: meio cosseno em torno do centro da janela,
// aberto por meia volta de came (uma volta de virabrequim). O came gira na
// METADE do giro do motor, e é por isso que o argumento entra dividido por
// dois — é essa divisão que faz o ciclo fechar em duas voltas.
func relaxV4Lift(cyc, center float64) float64 {
	c := math.Cos((cyc - center) / 2)
	if c <= 0 {
		return 0
	}
	return math.Pow(c, 1.6)
}

func stepRelaxV4(st *relaxV4State) {
	if !st.inited {
		relaxV4Init(st)
	}
	st.tick++
	st.t++

	switch st.stage {
	case v4Crank:
		// Arranque: o motor de partida puxa devagar e engasga. As tossidas são
		// o que faz a partida ter começo, meio e fim em vez de virar um fade.
		st.want = 0.26 + 0.05*math.Sin(float64(st.t)*0.7)
		if st.t == 14 || st.t == 22 {
			st.want = 0.44 // uma tossida: quase pegou
		}
		if st.t > 26 && rand.Intn(6) == 0 {
			st.stage, st.t = v4Catch, 0
			st.want = 1.35
		}
	case v4Catch:
		if st.t > 12 {
			st.want = 0.62
		}
		if st.t > 30 {
			st.stage, st.t = v4Idle, 0
		}
	default:
		st.want = 0.60 + 0.03*math.Sin(float64(st.tick)*0.18)
		if st.blip > 0 {
			// Acelerada: sobe rápido e volta devagar, que é como o pedal
			// solta — subir e descer igual pareceria um seno.
			u := float64(st.blip) / 26
			st.want = 0.60 + 1.55*math.Sin(math.Pi*math.Pow(u, 0.6))
			st.blip--
		} else if st.next--; st.next <= 0 {
			st.blip, st.next = 26, 90+rand.Intn(140)
		}
	}
	// O giro persegue o alvo: inércia de volante, não teletransporte.
	st.rpm += (st.want - st.rpm) * 0.18
	st.ang += st.rpm

	burning := st.stage != v4Crank
	for i := range st.cyl {
		c := &st.cyl[i]
		// A queima apaga ANTES de uma nova acender, senão o estouro já nasce
		// com um passo de decaimento e nunca chega ao branco.
		c.fire *= 0.62
		// Ângulo do munhão medido a partir do eixo do cilindro: quando ele
		// cruza zero, aquele pistão está no ponto morto superior.
		rel := math.Mod(st.ang+c.off-math.Atan2(c.uy, c.ux)+4*math.Pi, 2*math.Pi)
		if rel < c.rel { // deu a volta: passou pelo PMS
			if c.power = !c.power; c.power && burning {
				c.fire = 1
				relaxV4Puffs(st, i)
				st.shake = math.Min(1, st.shake+0.55)
			}
		}
		c.rel = rel
	}
	st.shake *= 0.72

	live := st.puffs[:0]
	for _, p := range st.puffs {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.006 // a fumaça sobe, e vai subindo mais rápido
		p.vx *= 0.94
		p.size += 0.012
		if p.life -= 0.05; p.life > 0 {
			live = append(live, p)
		}
	}
	st.puffs = live
}

// relaxV4Puffs solta o escape. Ele sai pela lateral DE FORA do banco, na altura
// do coletor — nascendo no meio do bloco, virava ponto solto no meio das bielas.
func relaxV4Puffs(st *relaxV4State, i int) {
	if len(st.puffs) > 40 {
		return
	}
	c := st.cyl[i]
	side := 1.0
	if c.ux < 0 {
		side = -1
	}
	st.puffs = append(st.puffs, relaxV4Puff{
		x:    c.cx + side*(0.80+rand.Float64()*0.06),
		y:    0.46 + rand.Float64()*0.14,
		vx:   side * (0.014 + rand.Float64()*0.010),
		vy:   0.020,
		life: 0.75 + rand.Float64()*0.25,
		size: 0.035 + rand.Float64()*0.025,
	})
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxV4Frames(st *relaxV4State, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxV4(st)
	}
	w := maxInt(24, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBrailleVote(w, h)
	relaxV4Draw(st, b)

	status := "marcha lenta"
	switch {
	case st.stage == v4Crank:
		status = "dando partida…"
	case st.stage == v4Catch:
		status = "pegou"
	case st.blip > 0:
		status = "acelerando"
	}
	return b.lines(relaxStyles(relaxV4Pal, fade)), StyleMuted.Render(status)
}

func relaxV4Draw(st *relaxV4State, b *relaxBraille) {
	sw, sh := b.w*2, b.h*4
	// O motor mede 5,3 de largura por 1,9 de altura; o palco decide qual das
	// duas aperta primeiro.
	span := relaxV4Gap*float64(relaxV4Vs-1) + 2*(relaxV4Deck*math.Sin(relaxV4Tilt)+relaxV4Bore+0.2) + 0.3
	// Altura real da peça, da tampa de válvulas ao fundo do cárter. Chutar esse
	// número é o que estava jogando o cabeçote pra fora do palco.
	top := (relaxV4Deck + relaxV4Head) * math.Cos(relaxV4Tilt)
	scale := math.Min(float64(sw)/span, float64(sh)/((top+0.70)*1.06))
	cx, cy := float64(sw)/2, float64(sh)/2+((top-0.64)/2)*scale
	// Tranco: cada estouro sacode o bloco inteiro por um instante.
	jx := st.shake * math.Sin(float64(st.tick)*2.1) * 0.9
	jy := st.shake * math.Sin(float64(st.tick)*1.7) * 0.6
	pen := relaxPen{step: 1 / scale, put: func(x, y float64, tone int) {
		b.set(int(cx+x*scale+jx), int(cy-y*scale+jy), tone)
	}}

	relaxV4Smoke(st, pen)
	for i := range st.cyl {
		relaxV4Cylinder(st, pen, i)
	}
	relaxV4Bottom(st, pen)
}

// relaxV4Cylinder desenha um cilindro inteiro, da queima ao cabeçote. Ordem da
// FRENTE pro FUNDO: é corte, então o que está dentro vem antes e a parede só
// preenche o que sobrou.
func relaxV4Cylinder(st *relaxV4State, p relaxPen, i int) {
	c := st.cyl[i]
	d, px, py := relaxV4Piston(c, st.ang)
	ax, ay := c.ux, c.uy  // eixo do cilindro
	nx, ny := c.uy, -c.ux // perpendicular, pra largura
	at := func(t, s float64) (float64, float64) {
		return c.cx + ax*t + nx*s, ay*t + ny*s
	}
	cyc := relaxV4Cycle(c)
	// Escape abre no meio do tempo de escape, admissão no meio da admissão.
	lift := [2]float64{relaxV4Lift(cyc, 1.5*math.Pi), relaxV4Lift(cyc, 2.5*math.Pi)}
	center := [2]float64{1.5 * math.Pi, 2.5 * math.Pi}
	// A válvula de escape fica do lado de fora do banco, a de admissão no meio
	// do V — que é onde elas ficam num motor de verdade.
	seat := [2]float64{relaxV4Seat, -relaxV4Seat}
	if c.ux < 0 {
		seat = [2]float64{-relaxV4Seat, relaxV4Seat}
	}

	// ── Queima ── a chama nasce na vela e desce pela câmara. O núcleo branco
	// apaga primeiro e o vermelho fica, que é o que dá a impressão de calor
	// sobrando depois do estouro.
	if c.fire > 0.02 {
		front := (1 - c.fire) * 1.6
		for t := d + 0.05; t <= relaxV4Deck+0.02; t += p.step {
			for s := -relaxV4Bore * 0.94; s <= relaxV4Bore*0.94; s += p.step {
				dist := math.Hypot(relaxV4Deck-t, s*1.2) / (relaxV4Bore * 2.4)
				v := c.fire * clamp01(1.30-1.5*math.Abs(dist-front)) * (1 - 0.35*dist)
				x, y := at(t, s)
				if v < 0.06 || relaxHalftone(int(x*46), int(y*46)) > v {
					continue
				}
				p.put(x, y, v4Fire+minInt(int(v*float64(v4FireN)*1.15), v4FireN-1))
			}
		}
	}

	// ── Pistão ── coroa clara, dois anéis e a saia mais escura.
	for t := d - 0.34; t <= d+0.02; t += p.step {
		tone := v4Pist + 2
		switch {
		case t > d-0.06:
			tone = v4Pist + 3 // coroa, virada pra câmara
		case t > d-0.13 && t < d-0.09, t > d-0.21 && t < d-0.17:
			tone = v4Pist // anéis
		case t < d-0.26:
			tone = v4Pist + 1
		}
		for s := -relaxV4Bore * 0.9; s <= relaxV4Bore*0.9; s += p.step {
			x, y := at(t, s)
			p.put(x, y, tone)
		}
	}

	// ── Biela ── afinando no meio, com pino e cabeça.
	jx, jy := at(d-0.22, 0)
	kx, ky := c.cx+px, py
	n := maxInt(3, int(math.Hypot(kx-jx, ky-jy)/p.step))
	for k := 0; k <= n; k++ {
		f := float64(k) / float64(n)
		p.dot(lerp(jx, kx, f), lerp(jy, ky, f), 0.062-0.026*math.Sin(math.Pi*f), v4Pist+1)
	}
	p.ring(jx, jy, 0.062, 0.034, v4Pist+2)
	p.disc(kx, ky, 0.085, v4Pist+1)

	// ── Válvulas e comando ── a haste desce quando o came empurra, e o came
	// aponta pra válvula exatamente no meio da abertura. É a peça que mostra os
	// quatro tempos: sem ela, o pistão sobe e desce sem dizer por quê.
	for v := 0; v < 2; v++ {
		s0 := seat[v]
		drop := lift[v] * 0.19
		hx, hy := at(relaxV4Deck-drop, s0)
		// Prato da válvula.
		for w := -0.115; w <= 0.115; w += p.step {
			x, y := at(relaxV4Deck-drop-0.035, s0+w)
			p.dot(x, y, 0.038, v4Steel+5)
		}
		tx, ty := at(relaxV4Deck+0.32, s0)
		p.stroke(hx, hy, tx, ty, 0.052, v4Steel+4) // haste
		// Came: gira meia volta por volta do motor e aponta o ressalto pra
		// válvula no pico do levante.
		cmx, cmy := at(relaxV4Deck+0.40, s0)
		rot := (cyc - center[v]) / 2
		lx := -(ax*math.Cos(rot) - nx*math.Sin(rot))
		ly := -(ay*math.Cos(rot) - ny*math.Sin(rot))
		p.stroke(cmx, cmy, cmx+lx*0.15, cmy+ly*0.15, 0.09, v4Steel+3)
		p.dot(cmx+lx*0.15, cmy+ly*0.15, 0.055, v4Steel+4)
		p.disc(cmx, cmy, 0.10, v4Steel+3)
		p.dot(cmx, cmy, 0.03, v4Steel+1) // furo do eixo
	}

	// ── Vela, entre as duas válvulas ──
	sx, sy := at(relaxV4Deck+0.30, 0)
	ex, ey := at(relaxV4Deck-0.02, 0)
	p.stroke(sx, sy, ex, ey, 0.05, v4Steel+4)
	if c.fire > 0.5 {
		p.dot(ex, ey, 0.05, v4Fire+4)
	}

	// ── Parede do cilindro ──
	for _, sg := range [2]float64{-1, 1} {
		for t := 0.26; t <= relaxV4Deck; t += p.step {
			x, y := at(t, sg*relaxV4Bore)
			p.dot(x, y, 0.042, v4Steel+4)
		}
	}

	// ── Cabeçote em corte ── platô, duas paredes laterais e a tampa por cima.
	// Vazado de propósito: maciço, o vermelho engole comando, válvula e vela,
	// que é justamente o que o corte existe pra mostrar.
	fill := func(t0, t1, s0, s1 float64, tone int) {
		for t := t0; t <= t1; t += p.step * 0.6 {
			for s := s0; s <= s1; s += p.step * 0.6 {
				x, y := at(t, s)
				p.put(x, y, tone)
			}
		}
	}
	wall := relaxV4Bore + 0.09
	fill(relaxV4Deck, relaxV4Deck+0.07, -wall, wall, v4Red+1) // platô
	for _, sg := range [2]float64{-1, 1} {
		fill(relaxV4Deck+0.07, relaxV4Deck+relaxV4Head-0.10,
			math.Min(sg*(relaxV4Bore+0.02), sg*wall), math.Max(sg*(relaxV4Bore+0.02), sg*wall), v4Red+2)
	}
	fill(relaxV4Deck+relaxV4Head-0.10, relaxV4Deck+relaxV4Head,
		-relaxV4Bore-0.11, relaxV4Bore+0.11, v4Red+3) // tampa de válvulas
}

// relaxV4Bottom é o virabrequim com os contrapesos e o cárter. O eixo é o único
// lugar da cena em que dá pra ver a volta inteira acontecendo.
func relaxV4Bottom(st *relaxV4State, p relaxPen) {
	for v := 0; v < relaxV4Vs; v++ {
		c := st.cyl[v*2]
		pa := st.ang + c.off
		px, py := relaxV4Crank*math.Cos(pa), relaxV4Crank*math.Sin(pa)
		// Contrapeso: meia-lua do lado OPOSTO ao munhão. Ele é o que deixa a
		// volta visível — sem massa girando, o virabrequim vira um ponto que
		// some no meio das bielas.
		for a := pa + math.Pi/2; a <= pa+3*math.Pi/2; a += 0.05 {
			ca, sa := math.Cos(a), math.Sin(a)
			for r := 0.0; r <= 0.24; r += p.step {
				tone := v4Steel + 2
				if r > 0.19 {
					tone = v4Steel + 3 // aresta do contrapeso pega a luz
				}
				p.dot(c.cx+ca*r, sa*r, p.step*0.8, tone)
			}
		}
		p.disc(c.cx+px, py, 0.085, v4Steel+4) // munhão
		p.disc(c.cx, 0, 0.07, v4Steel+3)      // mancal
	}
	// Eixo, mancais e cárter. O cárter é trapézio e escuro: ele é a base de
	// que tudo pendura, não uma peça pra se olhar.
	half := relaxV4Gap*float64(relaxV4Vs-1)/2 + 0.62
	p.rect(-half, -0.04, half, 0.04, v4Steel+3)
	for y := -0.64; y <= -0.32; y += p.step {
		f := clamp01((y + 0.64) / 0.32)
		w := half * (0.52 + 0.34*f)
		tone := v4Steel + 1
		if f > 0.86 {
			tone = v4Steel + 3 // borda de cima do cárter
		} else if f < 0.12 {
			tone = v4Steel
		}
		p.rect(-w, y, w, y+p.step*0.9, tone)
	}
}

func relaxV4Smoke(st *relaxV4State, p relaxPen) {
	for _, f := range st.puffs {
		tone := v4Smoke + minInt(int(f.life*3), v4SmokeN-1)
		for a := 0.0; a < 6.28; a += 0.5 {
			for r := 0.0; r <= f.size; r += p.step {
				x, y := f.x+math.Cos(a)*r, f.y+math.Sin(a)*r*0.8
				if relaxHalftone(int(x*44), int(y*44)) > f.life*0.42 {
					continue
				}
				p.dot(x, y, p.step*0.8, tone)
			}
		}
	}
}
