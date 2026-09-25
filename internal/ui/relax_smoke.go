package ui

import (
	"fmt"
	"math"
	"math/rand"
)

// ── Fumaça colorida ───────────────────────────────────────────────────────────
//
// Um gerador solta fumaça de DUAS cores, elas se enrolam uma na outra, a coluna
// sobe, abre e dissipa — e aí sai outra, com duas cores novas. O ciclo é o do
// enunciado: 5s de jato e 15s até não sobrar nada.
//
// O QUE FAZ A FUMAÇA PARECER FUMAÇA:
//
//   - CURL DE RUÍDO. A turbulência não é velocidade tirada do ruído (isso dá
//     jato de spray): é o ROTACIONAL de um campo de ruído — (∂n/∂y, −∂n/∂x). Um
//     campo de curl é livre de divergência, então ele gira o ar em vez de
//     inflá-lo, e é daí que vem o rabisco que a fumaça de verdade faz.
//   - DENSIDADE, não sprite. Cada baforada deposita um borrão macio num campo;
//     onde várias se somam a fumaça é corpo, onde uma passou sozinha é véu. No
//     desenho, a densidade compete com o limiar de meio-tom: acima dele a célula
//     FECHA, abaixo ela esgarça em pontinhos. É isso que dá as partes
//     preenchidas e as partes soltas ao mesmo tempo, com um cálculo só.
//   - FARRAPOS. Uma em cada três baforadas é pequena, leve e sofre o dobro de
//     turbulência. Sem elas a coluna tem borda lisa de balão.
//   - EMPUXO QUE ESFRIA + ARRASTO. O gás sobe rápido no começo e vai perdendo
//     força; é o arrasto que faz a coluna desacelerar e abrir em vez de virar
//     foguete.
//
// AS DUAS CORES: o matiz é sorteado por lançamento, e o segundo fica a 60°–180°
// do primeiro — perto demais e a mescla não aparece, opostos e viram duas cenas
// separadas. A paleta é a rampa entre eles pelo CAMINHO CURTO do círculo de
// matiz (de magenta a âmbar se passa pelo vermelho, não pelo verde), cruzada com
// quatro degraus de densidade: fumaça fina é escura e lavada, o miolo é aceso e
// saturado. A válvula alterna entre as duas cores em FITA (não sorteia por
// baforada, senão a coluna vira chuvisco bicolor), e cada baforada caminha pro
// meio da rampa conforme envelhece — dois corantes no mesmo ar acabam na cor do
// meio.
//
// Dois relógios, como todas as cenas: o passo de simulação (10/s) só decide o
// EVENTO de relançar; emissão, física e desenho andam no relógio de render, em
// sub-passos de tempo real, e por isso a coluna é lisa nos ~30fps.

const (
	smkEmit  = 5.0  // segundos de jato
	smkCycle = 15.0 // segundos até dissipar por completo — e aí lança de novo
	smkRate  = 19.0 // baforadas por segundo
	smkMax   = 170  // teto de baforadas vivas
)

// ── Paleta ────────────────────────────────────────────────────────────────────

const (
	smkMixN = 6 // degraus entre a cor A e a cor B
	smkLumN = 4 // degraus de densidade
	smkN    = smkMixN * smkLumN
)

const (
	smkCan  = smkN // corpo do gerador
	smkRim  = smkCan + 1
	smkPalN = smkRim + 1
)

// smkPalette monta a rampa das duas cores. Índice = mescla·smkLumN + densidade,
// então andar de um em um é mudar de brilho e andar de smkLumN em smkLumN é
// caminhar de uma cor pra outra.
func smkPalette(hA, hB float64) []relaxColor {
	out := make([]relaxColor, smkPalN)
	// Caminho curto no círculo: o arco menor entre os dois matizes.
	d := math.Mod(hB-hA+540, 360) - 180
	for m := 0; m < smkMixN; m++ {
		f := float64(m) / float64(smkMixN-1)
		h := math.Mod(hA+d*f+360, 360)
		for l := 0; l < smkLumN; l++ {
			g := float64(l) / float64(smkLumN-1)
			// Fumaça fina é quase ar: escura e lavada. O miolo é denso, aceso e
			// saturado. Sem essa diferença a coluna vira um adesivo colorido.
			out[m*smkLumN+l] = relaxHSL(h, 0.30+0.48*g, 0.13+0.52*g)
		}
	}
	out[smkCan] = "#141821"
	out[smkRim] = "#4A5568"
	return out
}

// ── Estado ────────────────────────────────────────────────────────────────────

// smkPuff é uma baforada. x é o afastamento do bico e y a altura, os dois em
// alturas de palco — assim o mesmo movimento serve pra qualquer terminal, e o
// subponto Braille sendo quadrado, o borrão sai redondo.
type smkPuff struct {
	x, y   float64
	vx, vy float64
	r, r0  float64 // raio atual e o de nascimento: a razão entre eles é o dissipar
	lift   float64 // empuxo próprio: umas sobem na frente, outras ficam atrás
	born   float64
	life   float64
	mix    float64 // 0 = cor A, 1 = cor B
	dens   float64
	wisp   bool // farrapo: menor, mais leve, mais turbulento
}

type relaxSmokeState struct {
	inited bool
	seed   int
	clock  relaxSubClock
	launch float64 // instante do lançamento atual
	hueA   float64
	hueB   float64
	pal    []relaxColor
	puffs  []smkPuff
	emit   float64 // acumulador fracionário da vazão
	shots  int

	// Campos de densidade e de mescla, reaproveitados entre quadros: são
	// dezenas de milhares de células, e alocar isso 30 vezes por segundo é
	// lixo de graça.
	den, mix []float32
	dw, dh   int
}

func smkInit(st *relaxSmokeState, now float64) {
	st.inited = true
	st.seed = rand.Intn(9999)
	smkLaunch(st, now)
}

// smkLaunch começa uma fumaça nova: duas cores sorteadas e o campo limpo. É o
// que roda ao entrar na tela, e de novo a cada smkCycle.
func smkLaunch(st *relaxSmokeState, now float64) {
	st.launch = now
	st.puffs = st.puffs[:0]
	st.emit = 0
	st.shots++
	st.hueA = rand.Float64() * 360
	// 60°–180° de distância: mais perto que isso a mescla não aparece, e opostos
	// viram duas fumaças que não conversam.
	d := 60 + rand.Float64()*120
	if rand.Intn(2) == 0 {
		d = -d
	}
	st.hueB = math.Mod(st.hueA+d+360, 360)
	st.pal = smkPalette(st.hueA, st.hueB)
}

// stepRelaxSmoke é a simulação: só o ciclo. Relançar é EVENTO, e evento mora no
// passo fixo; emissão e física andam no relógio de render.
func stepRelaxSmoke(st *relaxSmokeState, now float64) {
	if !st.inited {
		smkInit(st, now)
		return
	}
	if now-st.launch >= smkCycle {
		smkLaunch(st, now)
	}
}

// smkNewPuff nasce no bico. A cor vem da FITA: a válvula caminha de uma cor à
// outra devagar, então saem laços alternados que se enrolam no ar. Sorteio de
// cor por baforada daria chuvisco bicolor, não mescla.
func smkNewPuff(st *relaxSmokeState, now, age float64) smkPuff {
	// A onda é quadrada de leve (tanh): a válvula fica um tempo em cada cor e
	// troca rápido, e é isso que sai como LAÇO de cor em vez de degradê morno.
	rib := 0.5 + 0.5*math.Tanh(1.9*math.Sin(age*1.7+float64(st.seed%11)))
	p := smkPuff{
		x:    (rand.Float64() - 0.5) * 0.05,
		y:    0.085,
		vx:   (rand.Float64() - 0.5) * 0.09,
		vy:   0.14 + rand.Float64()*0.10,
		r:    0.030 + rand.Float64()*0.024,
		born: now,
		// A vida é longa de propósito: a última baforada do jato ainda tem de
		// estar sumindo lá em cima quando o ciclo de 15s fecha.
		life: 9.0 + rand.Float64()*3.0,
		// Empuxo próprio: sem essa variação a coluna sobe em bloco, como um
		// elevador. Com ela umas vão na frente e outras ficam, e a fumaça se
		// ESTICA na vertical, que é o que ela faz de verdade.
		lift: 0.78 + rand.Float64()*0.52,
		mix:  clamp01(rib + (rand.Float64()-0.5)*0.22),
		dens: 0.80 + rand.Float64()*0.55,
	}
	if rand.Intn(3) == 0 {
		p.wisp = true
		p.r *= 0.55
		p.dens *= 0.50
		p.vy += 0.07
		p.life *= 0.85
	}
	p.r0 = p.r
	return p
}

// smkCurl é o rotacional do campo de ruído: (∂n/∂y, −∂n/∂x). Livre de
// divergência, então ele ENROLA o ar em vez de inflar — é a diferença entre
// fumaça e spray. O campo também escorre no tempo, senão a fumaça enrola sempre
// nos mesmos lugares.
func smkCurl(x, y, now float64, seed int) (float64, float64) {
	// A escala é GRANDE em relação à coluna: baforadas vizinhas têm de sentir o
	// mesmo redemoinho, senão a fumaça se despedaça em bolhas soltas em vez de
	// serpentear inteira. E o campo escorre devagar no tempo, senão ela enrola
	// sempre nos mesmos lugares.
	n := func(px, py float64) float64 {
		return relaxNoise(px*1.5+py*0.7+now*0.16, seed) +
			0.55*relaxNoise(py*2.4-px*1.3-now*0.11, seed+91)
	}
	const e = 0.10
	return (n(x, y+e) - n(x, y-e)) / (2 * e), -(n(x+e, y) - n(x-e, y)) / (2 * e)
}

// smkAdvance anda um sub-passo de tempo real (~33ms).
func smkAdvance(st *relaxSmokeState, now, dt float64) {
	// Jato: só nos primeiros smkEmit segundos. O acumulador fracionário mantém a
	// mesma vazão por segundo qualquer que seja o passo.
	if age := now - st.launch; age < smkEmit {
		st.emit += smkRate * dt
		for st.emit >= 1 && len(st.puffs) < smkMax {
			st.emit--
			st.puffs = append(st.puffs, smkNewPuff(st, now, age))
		}
		if st.emit > 2 {
			st.emit = 0 // não acumula dívida de vazão
		}
	}

	// Uma brisa lenta por cima de tudo: a coluna inteira pende de leve pra um
	// lado e volta.
	wind := 0.020 * math.Sin(now*0.17+float64(st.seed%7))
	live := st.puffs[:0]
	for _, p := range st.puffs {
		f := (now - p.born) / p.life
		if f >= 1 {
			continue
		}
		// Empuxo que esfria: sobe forte e vai perdendo força. O número é baixo de
		// propósito — a coluna tem de levar quase a vida inteira pra chegar ao
		// topo, senão a fumaça foge do quadro antes de dissipar.
		p.vy += 0.15 * p.lift * (1 - f) * dt
		p.vx += wind * dt
		cx, cy := smkCurl(p.x, p.y, now, st.seed)
		turb := 0.09
		if p.wisp {
			turb = 0.17 // farrapo é leve: o ar manda mais nele
		}
		// A turbulência tem de ser MENOR que o empuxo: ela desvia a coluna, não
		// manda nela. Com ela forte, o redemoinho chega a cancelar a subida e a
		// fumaça escorre pelo chão em vez de subir.
		// Os dois eixos entram PODADOS, e o vertical muito mais que o lateral: o
		// redemoinho tem de desviar a coluna, nunca competir com o empuxo. Solto
		// no eixo y ele vira corrente descendente parada em cima da coluna e a
		// fumaça não sobe — medido, a subida em 5s ia de 0,07 a 0,84 dependendo
		// do sorteio do campo. Solto no x, leva a coluna pela borda e deixa o
		// quadro vazio antes da hora.
		p.vx += cx * turb * 0.55 * dt
		p.vy += cy * turb * 0.18 * dt
		// Arrasto: é ele que desacelera a coluna e a faz ABRIR em vez de subir
		// reto pra sempre.
		const drag = 1.6
		p.vx -= p.vx * drag * dt
		p.vy -= p.vy * drag * dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		// A baforada abre com a idade: ar entrando nela. O raio tem teto — sem
		// ele, uma baforada velha cobre meia tela e o quadro fica caro.
		gr := 0.013
		if p.wisp {
			gr = 0.008
		}
		if p.r < 0.19 {
			p.r += gr * (1 + 1.3*f) * dt
		}
		live = append(live, p)
	}
	st.puffs = live
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxSmokeFrames(st *relaxSmokeState, width, height int, fade, now float64) ([]string, string) {
	if !st.inited {
		smkInit(st, now)
	}
	// Física e emissão pelo relógio de render: sub-passos de tempo real, ~30fps.
	for i, n := 0, st.clock.tick(now); i < n; i++ {
		smkAdvance(st, now, relaxSubDT)
	}
	b := newRelaxBrailleVote(maxInt(24, minInt(width, 120)), maxInt(8, minInt(height, 34)))
	smkDraw(st, b, now)
	return b.lines(relaxStyles(st.pal, fade)), StyleMuted.Render(smkStatus(st, now))
}

func smkDraw(st *relaxSmokeState, b *relaxBraille, now float64) {
	sw, sh := b.w*2, b.h*4
	fsh := float64(sh)
	nx := float64(sw) * 0.5
	canH := math.Max(3, 0.075*fsh)
	ny := fsh - 1 - canH // o bico, de onde a fumaça sai

	if st.dw != sw || st.dh != sh {
		st.den, st.mix, st.dw, st.dh = make([]float32, sw*sh), make([]float32, sw*sh), sw, sh
	}
	den, mix := st.den, st.mix
	for i := range den {
		den[i], mix[i] = 0, 0
	}

	// Rabo do ciclo: aos smkCycle segundos não pode ter sobrado NADA, e a última
	// baforada do jato é 5s mais nova que a primeira. Um desvanecimento do ciclo
	// fecha essa conta sem encurtar a vida de ninguém — a fumaça acaba de sumir
	// entre 13 e 14,4s, e o palco entra limpo no lançamento seguinte.
	cyc := 1 - clamp01((now-st.launch-13.0)/1.4)
	if cyc <= 0 {
		smkCanister(b, st, nx, fsh, canH, now)
		return
	}

	// ── Campo ── cada baforada deposita um borrão macio; onde elas se somam a
	// fumaça vira corpo. O peso do borrão cai com o quadrado da distância ao
	// quadrado (q²), que é macio o bastante pra não deixar anel de borda.
	for _, p := range st.puffs {
		age := now - p.born
		f := clamp01(age / p.life)
		// Entra em 0,4s e some com a idade. E o que dissipa de verdade é a razão
		// entre o raio de nascimento e o de agora: a mesma massa espalhada no
		// dobro do raio é metade da densidade — é conservação, não fade.
		a := p.dens * clamp01(age/0.4) * math.Pow(1-f, 1.0) * (p.r0 / p.r) * cyc
		if a <= 0.02 {
			continue
		}
		// A mescla caminha pro meio da rampa com a idade: dois corantes no mesmo
		// ar acabam na cor do meio.
		m := float32(lerp(p.mix, 0.5, f*0.40))
		cx, cy := nx+p.x*fsh, ny-p.y*fsh
		r := math.Max(1, p.r*fsh)
		x0, x1 := maxInt(0, int(cx-r)), minInt(sw-1, int(cx+r))
		y0, y1 := maxInt(0, int(cy-r)), minInt(sh-1, int(cy+r))
		r2, af := float32(r*r), float32(a)
		for y := y0; y <= y1; y++ {
			dy := float32(float64(y) - cy)
			row := y * sw
			for x := x0; x <= x1; x++ {
				dx := float32(float64(x) - cx)
				q := 1 - (dx*dx+dy*dy)/r2
				if q <= 0 {
					continue
				}
				w := af * q * q
				den[row+x] += w
				mix[row+x] += w * m
			}
		}
	}

	// ── Fumaça ── a densidade vira OPACIDADE por absorção (Beer-Lambert:
	// 1−e^(−kd)), e a opacidade vira densidade de PONTOS via meio-tom. É essa
	// dupla que transforma soma de borrões em fumaça: o miolo fecha a célula, o
	// meio fica em meio-tom e a borda esgarça em pontinhos soltos — tudo com a
	// mesma conta, sem máscara nem contorno. Somar densidade direto no limiar
	// (sem a absorção) satura tudo e a fumaça vira adesivo chapado.
	for y := 0; y < sh; y++ {
		row := y * sw
		for x := 0; x < sw; x++ {
			d := float64(den[row+x])
			if d <= 0.012 {
				continue
			}
			op := 1 - math.Exp(-1.25*d)
			if relaxHalftone(x, y) > op {
				continue
			}
			m := minInt(maxInt(int(float64(mix[row+x])/d*float64(smkMixN-1)+0.5), 0), smkMixN-1)
			b.set(x, y, m*smkLumN+minInt(int(op*float64(smkLumN)), smkLumN-1))
		}
	}

	// ── Gerador ── por último: a fumaça passa na FRENTE dele.
	smkCanister(b, st, nx, fsh, canH, now)
}

// smkCanister é o pote no chão: cilindro escuro com a quina virada pra esquerda
// clareada, e o bico aceso enquanto o jato sai.
func smkCanister(b *relaxBraille, st *relaxSmokeState, nx, fsh, canH, now float64) {
	pen := relaxPen{step: 0.75, put: func(x, y float64, tone int) {
		b.set(int(math.Round(x)), int(math.Round(y)), tone)
	}}
	halfW := math.Max(1.3, 0.024*fsh)
	base := fsh - 1
	top := base - canH
	pen.rect(nx-halfW, top+1, nx+halfW, base, smkCan)
	pen.rect(nx-halfW, top+1, nx-halfW*0.35, base, smkRim)
	// Boca do pote, um pouco mais estreita.
	pen.rect(nx-halfW*0.6, top, nx+halfW*0.6, top+1.5, smkRim)

	if age := now - st.launch; age < smkEmit {
		// O bico acende na cor da fita que está saindo agora.
		rib := 0.5 + 0.5*math.Tanh(1.9*math.Sin(age*1.7+float64(st.seed%11)))
		lvl := minInt(int(rib*float64(smkMixN-1)+0.5), smkMixN-1)*smkLumN + smkLumN - 1
		pen.dot(nx, top-0.5, math.Max(1.2, halfW*0.7), lvl)
	}
}

// smkStatus conta em que pé está o ciclo e diz as duas cores por nome — é o que
// deixa claro que cada lançamento sorteia um par novo.
func smkStatus(st *relaxSmokeState, now float64) string {
	age := now - st.launch
	a, c := smkHueName(st.hueA), smkHueName(st.hueB)
	switch {
	case age < smkEmit:
		return fmt.Sprintf("lançando %s e %s · %.0fs de jato", a, c, math.Max(0, smkEmit-age))
	case age < smkCycle*0.72:
		return fmt.Sprintf("%s e %s se enrolando no ar", a, c)
	}
	return fmt.Sprintf("dissipando · outra fumaça em %.0fs", math.Max(0, smkCycle-age))
}

var smkHues = []struct {
	h    float64
	name string
}{
	{0, "vermelho"}, {20, "laranja"}, {40, "âmbar"}, {56, "amarelo"},
	{78, "verde-limão"}, {120, "verde"}, {160, "esmeralda"}, {182, "ciano"},
	{200, "azul-piscina"}, {222, "azul"}, {250, "anil"}, {278, "violeta"},
	{300, "magenta"}, {322, "rosa"}, {342, "carmim"},
}

func smkHueName(h float64) string {
	best, bd := "vermelho", 999.0
	for _, e := range smkHues {
		d := math.Abs(math.Mod(math.Abs(h-e.h), 360))
		if d > 180 {
			d = 360 - d
		}
		if d < bd {
			bd, best = d, e.name
		}
	}
	return best
}
