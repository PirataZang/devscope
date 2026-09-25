package ui

import (
	"math"
	"math/rand"
)

// ── Dragão batendo asa ────────────────────────────────────────────────────────
//
// Um dragão de perfil, parado no ar, batendo asa contra a lua. Nada de sprite
// pronto: o bicho é anatomia. Tronco, pescoço, cabeça, cauda, pernas e as duas
// asas saem de curvas com espessura, e o que se move é o esqueleto — não um
// desenho que troca de quadro.
//
// A ASA é a cena, e ela tem braço, antebraço, quatro dedos e o pano entre eles:
//
//   - o golpe pra BAIXO ocupa 38% do ciclo e a subida os outros 62% — asa não é
//     seno, e é essa assimetria que faz o bicho parecer que se sustenta no ar
//     em vez de acenar;
//   - na subida a asa DOBRA (o antebraço fecha e o pano encolhe) e no alto do
//     golpe ela abre inteira, que é o instante em que precisa de área;
//   - o pano ATRASA em relação ao osso, proporcional ao quadrado da distância
//     da raiz: é o atraso da ponta que dá o chicote e a ideia de vento;
//   - o corpo sobe DEPOIS do golpe, nunca junto: a sustentação chega atrasada.
//
// Três alfabetos no mesmo quadro, de propósito:
//
//	BRAILLE  o bicho, a serra, as nuvens e a lua — 2×4 subpontos por célula é o
//	         que dá silhueta curva num terminal.
//	ASCII    o fogo é rampa de densidade (". : - = + * # % @"), com o @ no miolo
//	         branco; as brasas sobem como @ * + . ' e as estrelas graúdas viram
//	         * e +.
//	LETRAS   o rugido sai escrito da boca, letra por letra, e sobe apagando.
//
// A cena anda em DOIS relógios. A simulação (10 passos/s, como todas) cuida do
// que é evento: trocar de voo, rugir, nascer e morrer brasa. Tudo que é
// movimento contínuo — fase da asa, balanço do corpo, onda da cauda, chama,
// trajetória de cada brasa — é função do tempo de RENDER, e por isso a 30fps o
// desenho anda trinta vezes por segundo em vez de dez. É também por isso que
// brasa é trajetória analítica (p = p₀ + v·t + ½·g·t²): sem posição guardada,
// não há o que pular de dez em dez.

// ── Paleta ────────────────────────────────────────────────────────────────────

const (
	dgScaleN = 8 // couro do dorso
	dgBellyN = 4 // placas da barriga
	dgBoneN  = 3 // chifre, garra, dente
	dgMembN  = 7 // pano da asa da frente
	dgFarN   = 5 // asa de trás: já nasce escura, não é a mesma rampa
	dgFireN  = 6
	dgSmokeN = 3
	dgHillN  = 4
	dgCloudN = 3
	dgMoonN  = 3
)

const (
	dgScale = 0
	dgBelly = dgScale + dgScaleN
	dgBone  = dgBelly + dgBellyN
	dgMemb  = dgBone + dgBoneN
	dgFar   = dgMemb + dgMembN
	dgFire  = dgFar + dgFarN
	dgSmoke = dgFire + dgFireN
	dgHill  = dgSmoke + dgSmokeN
	dgCloud = dgHill + dgHillN
	dgMoon  = dgCloud + dgCloudN
	dgStar  = dgMoon + dgMoonN
	dgGlint = dgStar + 1
	dgPalN  = dgGlint + 1
)

var relaxDragonPal = func() []relaxColor {
	out := make([]relaxColor, dgPalN)
	copy(out[dgScale:], relaxRamp([]string{"#04120C", "#0A2317", "#123A24", "#1D5233", "#276A40", "#35864E", "#48A462", "#66C27C"}, dgScaleN))
	copy(out[dgBelly:], relaxRamp([]string{"#4E3A12", "#8A6820", "#C09832", "#EAC868"}, dgBellyN))
	copy(out[dgBone:], relaxRamp([]string{"#6C6350", "#A89B80", "#E0D6B8"}, dgBoneN))
	// A membrana é vermelho-vinho e não verde: asa da cor do couro vira uma
	// mancha só quando ela passa na frente do corpo.
	copy(out[dgMemb:], relaxRamp([]string{"#170710", "#2C0F1E", "#48182C", "#6B2338", "#8F3446", "#B85158", "#DC7F76"}, dgMembN))
	copy(out[dgFar:], relaxRamp([]string{"#080308", "#140810", "#22101A", "#321826", "#442334"}, dgFarN))
	copy(out[dgFire:], relaxRamp([]string{"#7A1503", "#C43A06", "#F07510", "#FFAE2C", "#FFDD72", "#FFF8DC"}, dgFireN))
	copy(out[dgSmoke:], relaxRamp([]string{"#14161C", "#242A34", "#3A4250"}, dgSmokeN))
	// Perspectiva aérea: a serra do fundo é mais CLARA porque o ar entre ela e
	// você tem cor. Sem isso as duas cadeias viram um borrão.
	copy(out[dgHill:], relaxRamp([]string{"#070A12", "#0E1320", "#182034", "#26314C"}, dgHillN))
	copy(out[dgCloud:], relaxRamp([]string{"#161C2A", "#252E42", "#3A465E"}, dgCloudN))
	copy(out[dgMoon:], relaxRamp([]string{"#3E3A28", "#CFC69C", "#F8F2D6"}, dgMoonN))
	out[dgStar] = "#95A0BC"
	out[dgGlint] = "#FFE8A0"
	return out
}()

// ── Palco ─────────────────────────────────────────────────────────────────────
//
// O mundo é medido em comprimentos de tronco: x pra frente (o bicho olha pro
// +x), y pra cima, (0,0) no meio do palco. O que cabe na tela sai da escala, e
// a escala sai do lado que aperta primeiro.

const (
	dgHalfW = 2.60
	dgHalfH = 1.55
	dgMoonX = -1.75
	dgMoonY = 0.95
	dgMoonR = 0.34
)

type dgCanvas struct {
	b      *relaxBraille
	sw, sh int
	ox, oy float64
	scale  float64
}

func dgCanvasFor(b *relaxBraille) dgCanvas {
	sw, sh := b.w*2, b.h*4
	scale := math.Min(float64(sw)/(2*dgHalfW), float64(sh)/(2*dgHalfH))
	return dgCanvas{b: b, sw: sw, sh: sh, ox: float64(sw) / 2, oy: float64(sh) / 2, scale: scale}
}

func (c dgCanvas) subX(x float64) int { return int(math.Round(c.ox + x*c.scale)) }
func (c dgCanvas) subY(y float64) int { return int(math.Round(c.oy - y*c.scale)) }

// cellAt devolve o centro da célula (cx, cy) em coordenadas de mundo. É o que o
// fogo em ASCII precisa: ele desenha por CÉLULA, não por subponto.
func (c dgCanvas) cellAt(cx, cy int) (float64, float64) {
	return (float64(cx*2+1) - c.ox) / c.scale, (c.oy - float64(cy*4+2)) / c.scale
}

func (c dgCanvas) cellOf(x, y float64) (int, int) { return c.subX(x) / 2, c.subY(y) / 4 }

// dgFrame é o referencial do bicho: a anatomia é escrita em coordenadas locais
// e o frame a leva pro mundo com a deriva e a inclinação do voo.
type dgFrame struct{ px, py, sn, cs float64 }

func dgFrameAt(px, py, rot float64) dgFrame {
	sn, cs := math.Sincos(rot)
	return dgFrame{px, py, sn, cs}
}

func (f dgFrame) at(x, y float64) (float64, float64) {
	return f.px + x*f.cs - y*f.sn, f.py + x*f.sn + y*f.cs
}

// pen entrega um pincel que aceita coordenadas locais. p.step é um subponto
// medido em unidades de mundo — girar o frame não muda isso.
func (c dgCanvas) pen(f dgFrame) relaxPen {
	return relaxPen{step: 1 / c.scale, put: func(x, y float64, tone int) {
		wx, wy := f.at(x, y)
		c.b.set(c.subX(wx), c.subY(wy), tone)
	}}
}

// ── Corpo: tubos ao longo de curvas ───────────────────────────────────────────

type dgSection struct{ x, y, r float64 }

func dgQuad(a, b, c [2]float64, u float64) (float64, float64) {
	m := 1 - u
	return m*m*a[0] + 2*m*u*b[0] + u*u*c[0], m*m*a[1] + 2*m*u*b[1] + u*u*c[1]
}

func dgPolar(o [2]float64, ang, r float64) [2]float64 {
	return [2]float64{o[0] + math.Cos(ang)*r, o[1] + math.Sin(ang)*r}
}

func dgSteps(p relaxPen, length float64) int {
	return maxInt(4, int(length/p.step*1.15))
}

// dgTube desenha um trecho de bicho — pescoço, tronco, cauda, perna — como uma
// fila de seções perpendiculares à curva. O tom sai da posição DENTRO da seção
// (v = −1 barriga, +1 dorso), não da posição na tela: textura em coordenada de
// tela escorrega quando o bicho se mexe e parece chuvisco.
//
// A curva é percorrida do começo pro fim e o lado do dorso sai do sentido dela,
// então trecho desenhado de trás pra frente troca barriga por dorso — a cauda
// é desenhada da PONTA pro quadril por causa disso.
func dgTube(p relaxPen, at func(u float64) dgSection, steps int, tone func(u, v float64) int) {
	e := 0.5 / float64(steps)
	for i := 0; i <= steps; i++ {
		u := float64(i) / float64(steps)
		s := at(u)
		if s.r <= 0 {
			continue
		}
		a, c := at(math.Max(0, u-e)), at(math.Min(1, u+e))
		dx, dy := c.x-a.x, c.y-a.y
		l := math.Hypot(dx, dy)
		if l < 1e-9 {
			continue
		}
		nx, ny := -dy/l, dx/l
		for d := -s.r; d <= s.r; d += p.step * 0.7 {
			p.put(s.x+nx*d, s.y+ny*d, tone(u, d/s.r))
		}
	}
}

// dgLimb é o tubo reto de tom único: osso, haste de chifre, dedo, garra.
func dgLimb(p relaxPen, a, b [2]float64, r0, r1 float64, tone int) {
	length := math.Hypot(b[0]-a[0], b[1]-a[1])
	dgTube(p, func(u float64) dgSection {
		return dgSection{lerp(a[0], b[0], u), lerp(a[1], b[1], u), lerp(r0, r1, u)}
	}, dgSteps(p, length), func(u, v float64) int { return tone })
}

// dgSpike é o espinho do dorso: triângulo fino saindo da linha das costas.
func dgSpike(p relaxPen, base [2]float64, ang, h float64, tone int) {
	dgLimb(p, base, dgPolar(base, ang, h), h*0.30, p.step*0.6, tone)
}

// dgScaleTone resolve o couro: dorso na lua, barriga em contraluz, e a escama
// entrando como textura no CORPO. Um degrau só de claro — mais que isso vira
// estampa de camisa.
func dgScaleTone(u, v float64) int {
	if v < -0.40 {
		plate := 0
		if math.Sin(u*44) > 0 {
			plate = 1
		}
		return dgBelly + minInt(1+plate+int(clamp01(-v-0.4)*1.8), dgBellyN-1)
	}
	lum := 0.46 + 0.38*v
	if math.Sin(u*52+v*6)*math.Sin(u*19-v*11) > 0.30 {
		lum += 0.11
	}
	lum += 0.05 * relaxNoise(u*24, 913)
	return dgScale + minInt(maxInt(int(lum*float64(dgScaleN)), 0), dgScaleN-1)
}

// ── Asa ───────────────────────────────────────────────────────────────────────

const (
	dgArmUp   = 2.08 // radianos do braço no alto do golpe (pra cima e pra trás)
	dgArmDown = 4.30 // e no fim da descida (pra baixo e pra trás)
	dgDownFr  = 0.38 // fração do ciclo gasta descendo
	dgFingers = 4
	dgArmSeg  = 0.26 // braço, em frações da envergadura
	dgForeSeg = 0.20 // antebraço
	dgVane    = 1.05 // dedo do bordo de ataque, o mais longo
)

// dgStroke devolve onde o golpe está (1 = asa no alto, 0 = embaixo) e a
// velocidade do golpe em ciclos (positiva subindo). A descida é curta e forte,
// a subida longa e mansa.
func dgStroke(ph float64) (float64, float64) {
	ph -= math.Floor(ph)
	if ph < dgDownFr {
		u := ph / dgDownFr
		return 1 - easeInOutSine(u), -math.Sin(math.Pi*u) / dgDownFr
	}
	u := (ph - dgDownFr) / (1 - dgDownFr)
	return easeInOutSine(u), math.Sin(math.Pi*u) / (1 - dgDownFr)
}

type dgWingSpec struct {
	sh, hip [2]float64 // ombro e o ponto do corpo onde o pano da raiz se prende
	span    float64
	arm     float64 // ângulo do braço
	fan     float64 // abertura do leque de dedos, em radianos
	lag     float64 // atraso do pano, em radianos na ponta
	fold    float64 // 0 asa aberta, 1 dobrada
	lit     float64 // 0–1: quanto a face virada pra lua pega luz
	memb    int     // primeiro nível da rampa da membrana
	membN   int
	armTone int // bordo de ataque: é grosso, tem couro
	digit   int // dedo: escuro, é ele que costura o pano
	claw    int
}

// dgDrawWing desenha uma asa inteira.
//
// A asa é um POLÍGONO com o punho no vértice: dele saem os quatro dedos, e o
// pano é o leque de triângulos entre pontas vizinhas. A última ponta não é dedo
// — é o quadril, e é ela que fecha o pano no corpo em vez de deixá-lo terminar
// no ar. O leque abre mais no alto do golpe e fecha na descida, que é a
// perspectiva de uma asa vista de lado sem que ela chegue a desaparecer.
//
// Ordem: OSSO primeiro, pano depois. Braille não sobrescreve subponto aceso,
// então o que fica por cima é desenhado antes e a membrana preenche o resto.
func dgDrawWing(p relaxPen, wg dgWingSpec) {
	fore := wg.arm - 0.20 + 0.62*wg.fold
	el := dgPolar(wg.sh, wg.arm, wg.span*dgArmSeg)
	wr := dgPolar(el, fore, wg.span*dgForeSeg*(1-0.26*wg.fold))

	// Ângulo e alcance de cada ponta, medidos do punho e SEM o atraso: o atraso
	// entra na hora de desenhar, igual pro osso e pro pano. Baixá-lo aqui
	// deixaria a membrana torta em relação ao dedo que a segura.
	//
	// O leque tem abertura PRÓPRIA (wg.fan) em vez de ir do bordo de ataque até
	// o quadril: no meio do golpe o punho passa perto do quadril, e um leque
	// medido por ele fecharia até virar um risco — a asa desaparecia meio ciclo.
	hx, hy := wg.hip[0]-wr[0], wg.hip[1]-wr[1]
	ang0 := fore + 0.16
	vane := wg.span * dgVane * (1 - 0.22*wg.fold)
	var ang, rad, lag [dgFingers + 1]float64
	for i := 0; i < dgFingers; i++ {
		t := float64(i) / (dgFingers - 1)
		ang[i] = ang0 + wg.fan*t
		// A vela afina de leve do bordo de ataque pra raiz: encurtar muito aqui
		// esvazia a asa, e ela deixa de ter área pra bater.
		rad[i] = vane * (1 - 0.20*math.Pow(t, 1.4))
		lag[i] = wg.lag * (1 - t*0.55)
	}
	// A última ponta não é dedo: é o quadril, e é ele que fecha o pano no corpo.
	// Preso, ele não chicoteia.
	ang[dgFingers], rad[dgFingers] = math.Atan2(hy, hx), math.Hypot(hx, hy)
	lag[dgFingers] = wg.lag * 0.12

	// ── Osso ── braço e antebraço grossos: eles são o bordo de ataque, a parte
	// da asa que corta o ar e a única que tem volume. O dedo é desenhado em três
	// trechos porque ele CURVA com o atraso, junto com o pano.
	dgLimb(p, wg.sh, el, wg.span*0.058, wg.span*0.044, wg.armTone)
	dgLimb(p, el, wr, wg.span*0.044, wg.span*0.030, wg.armTone)
	at := func(i int, f float64) [2]float64 {
		x, y := dgFingerAt(wr, ang[i], lag[i], f, rad[i]*f)
		return [2]float64{x, y}
	}
	for i := 0; i < dgFingers; i++ {
		for k := 0; k < 3; k++ {
			f0, f1 := float64(k)/3, float64(k+1)/3
			r0 := lerp(wg.span*0.026, p.step*0.7, f0)
			r1 := lerp(wg.span*0.026, p.step*0.7, f1)
			dgLimb(p, at(i, f0), at(i, f1), r0, r1, wg.digit)
		}
		if i == 0 { // garra no polegar da asa
			end := at(i, 1)
			dgLimb(p, end, dgPolar(end, ang[i]+lag[i]-0.9, wg.span*0.07), wg.span*0.016, p.step*0.6, wg.claw)
		}
	}

	// ── Pano ── um triângulo por par de pontas, varrido em raios saindo do
	// punho. O raio para na RETA entre as duas pontas, senão a borda de fuga sai
	// abaulada e a asa perde o recorte.
	for i := 0; i < dgFingers; i++ {
		t0 := [2]float64{math.Cos(ang[i]) * rad[i], math.Sin(ang[i]) * rad[i]}
		t1 := [2]float64{math.Cos(ang[i+1]) * rad[i+1], math.Sin(ang[i+1]) * rad[i+1]}
		d := math.Mod(ang[i+1]-ang[i]+3*math.Pi, 2*math.Pi) - math.Pi // caminho curto
		ex, ey := t1[0]-t0[0], t1[1]-t0[1]
		cross := t0[0]*ey - t0[1]*ex
		maxLen := math.Max(rad[i], rad[i+1])
		da := clampF(0.75*p.step/math.Max(1e-6, math.Abs(d)*maxLen), 0.004, 0.05)
		for a := 0.0; a <= 1; a += da {
			ray := ang[i] + d*a
			ca, sa := math.Cos(ray), math.Sin(ray)
			den := ca*ey - sa*ex
			if math.Abs(den) < 1e-9 {
				continue
			}
			l := cross / den
			if l <= 0 {
				continue
			}
			// Festão: o pano entre dois dedos não é reto, ele afunda. Sem esse
			// afundamento a asa vira leque de papel.
			l = math.Min(l, maxLen) * (1 - 0.12*math.Sin(math.Pi*a))
			lg := lerp(lag[i], lag[i+1], a)
			for r := p.step; r <= l; r += p.step * 0.8 {
				fr := r / l
				x, y := dgFingerAt(wr, ray, lg, fr, r)
				// Borda de fuga esgarçada em meio-tom, no espaço DA ASA: em
				// espaço de tela ela cintilaria a cada batida.
				if fr > 0.86 && relaxHalftone(int(a*911), int(fr*911)) > (1-fr)/0.14 {
					continue
				}
				lum := 0.24 + 0.44*fr + 0.13*math.Sin(math.Pi*a) + 0.22*wg.lit - 0.12*wg.fold
				p.put(x, y, wg.memb+minInt(maxInt(int(lum*float64(wg.membN)), 1), wg.membN-1))
			}
		}
	}
}

// dgFingerAt devolve o ponto a uma fração f do dedo. O atraso cresce com o
// quadrado da distância da raiz: a ponta chega depois, e é isso que faz o pano
// chicotear em vez de girar inteiro como ponteiro.
func dgFingerAt(wr [2]float64, ang, lag, f, r float64) (float64, float64) {
	a := ang + lag*f*f
	return wr[0] + math.Cos(a)*r, wr[1] + math.Sin(a)*r
}

// ── Fogo, brasa e rugido ──────────────────────────────────────────────────────

// dgFireRamp é a rampa de densidade clássica do ASCII: quanto mais quente, mais
// tinta o caractere tem, e o @ fecha o miolo.
const dgFireRamp = ".:-=+*#%@"

var (
	dgEmberGlyph = []string{"@", "*", "+", ".", "'"}
	dgSmokeGlyph = []string{"%", "*", "o", ":", "."}
	dgStarGlyph  = []string{"*", "+", "."}
	dgRoarWords  = []string{"GRAAAH", "RAAWR", "KHRAAA"}
)

// dgFireEnv é o envelope do jato: acende de uma vez e sobra calor no fim.
func dgFireEnv(age float64) float64 {
	switch {
	case age < 0:
		return 0
	case age < 0.18:
		return easeOutCubic(age / 0.18)
	case age < 1.15:
		return 1
	case age < 1.70:
		return 1 - easeInOutSine((age-1.15)/0.55)
	}
	return 0
}

// dgJawEnv abre a boca ANTES do fogo e só fecha DEPOIS que o jato morreu: é o
// que dá causa e efeito em vez de duas coisas no mesmo instante.
func dgJawEnv(age float64) float64 {
	switch {
	case age < -0.32:
		return 0
	case age < 0:
		return easeInOutSine((age + 0.32) / 0.32)
	case age < 1.74:
		return 1
	case age < 2.24:
		return 1 - easeInOutSine((age-1.74)/0.50)
	}
	return 0
}

type dgEmber struct {
	x, y, vx, vy float64
	born, life   float64
	seed         int
	smoke        bool
}

// at devolve onde a brasa está no instante now. Trajetória fechada: nada é
// integrado passo a passo, então ela sobe lisa a 30fps.
func (e dgEmber) at(now float64) (float64, float64, float64) {
	dt := now - e.born
	f := clamp01(dt / e.life)
	x := e.x + e.vx*dt + 0.055*relaxNoise(dt*1.9, e.seed)
	y := e.y + e.vy*dt + 0.10*dt*dt // o ar quente empurra pra cima
	return x, y, f
}

type dgLetter struct {
	ch         string
	x, y       float64
	vx, vy     float64
	born, life float64
}

type dgPuff struct {
	x, y, rx, ry, vx float64
	seed             int
}

// ── Estado ────────────────────────────────────────────────────────────────────

const (
	dgModeFlap int8 = iota
	dgModeGlide
	dgModeRoar
)

const (
	dgFlapHz  = 0.78
	dgGlideHz = 0.16
	dgRoarHz  = 0.55
	dgSpan    = 0.90
)

type relaxDragonState struct {
	inited bool
	seed   int
	clock  float64 // segundos do engine no último passo de simulação
	mode   int8
	until  float64
	next   float64 // quando o próximo rugido sai
	roarAt float64 // instante em que o fogo começa; o queixo abre antes

	// A fase da asa é acumulada no passo e extrapolada no render: a frequência
	// muda entre voo batido e planado, e fase calculada direto do relógio
	// pularia a cada mudança de ritmo.
	flapPh float64
	rate   float64
	amp    float64

	word    string
	spoken  int
	embers  []dgEmber
	letters []dgLetter
	clouds  []dgPuff
	stars   []relaxSkyPt
	hills   [6]float64
}

func stepRelaxDragon(st *relaxDragonState, now float64) {
	if !st.inited {
		relaxDragonInit(st, now)
	}
	dt := clampF(now-st.clock, 0, 0.25)
	st.clock = now

	wantRate, wantAmp := dgFlapHz, 1.0
	switch st.mode {
	case dgModeGlide:
		wantRate, wantAmp = dgGlideHz, 0.24
		if now > st.until {
			st.mode, st.until = dgModeFlap, now+4+rand.Float64()*5
		}
	case dgModeRoar:
		wantRate, wantAmp = dgRoarHz, 1.05
		if now > st.until {
			st.mode, st.until = dgModeFlap, now+3+rand.Float64()*4
			st.next = now + 11 + rand.Float64()*12
		}
	default:
		if now > st.until {
			st.mode, st.until = dgModeGlide, now+3+rand.Float64()*4
		}
	}
	// O rugido interrompe o que estiver acontecendo: é o evento da cena.
	if st.mode != dgModeRoar && now >= st.next {
		st.mode = dgModeRoar
		st.roarAt = now + 0.32
		st.until = st.roarAt + 2.4
		st.word = dgRoarWords[rand.Intn(len(dgRoarWords))]
		st.spoken = 0
	}
	st.rate = smoothDamp(st.rate, wantRate, 0.30, dt)
	st.amp = smoothDamp(st.amp, wantAmp, 0.40, dt)
	st.flapPh += st.rate * dt
	st.flapPh -= math.Floor(st.flapPh)

	age := now - st.roarAt
	env := dgFireEnv(age)
	// Brasa nasce na chama; fumaça nasce depois, quando o jato já morreu.
	if env > 0.12 && len(st.embers) < 70 {
		mx, my := dgMouthAt(st, now)
		for i, n := 0, 2+rand.Intn(3); i < n; i++ {
			s := rand.Float64()
			st.embers = append(st.embers, dgEmber{
				x: mx + s*(0.35+1.0*env), y: my - 0.10*s + (rand.Float64()-0.5)*0.10,
				vx:   0.10 + rand.Float64()*0.34,
				vy:   0.05 + rand.Float64()*0.22,
				born: now, life: 0.9 + rand.Float64()*1.3,
				seed: rand.Intn(9999),
			})
		}
	}
	if age > 0.7 && age < 2.6 && len(st.embers) < 90 && rand.Intn(2) == 0 {
		mx, my := dgMouthAt(st, now)
		st.embers = append(st.embers, dgEmber{
			x: mx + 0.30 + rand.Float64()*0.9, y: my + rand.Float64()*0.12,
			vx: 0.06 + rand.Float64()*0.14, vy: 0.16 + rand.Float64()*0.14,
			born: now, life: 1.4 + rand.Float64()*1.4,
			seed: rand.Intn(9999), smoke: true,
		})
	}
	live := st.embers[:0]
	for _, e := range st.embers {
		if now-e.born < e.life {
			live = append(live, e)
		}
	}
	st.embers = live

	// O rugido sai letra por letra, no compasso da chama. Cada uma nasce um
	// pouco à frente e mais alta que a anterior: soltas do mesmo ponto, elas se
	// escrevem em cima da outra e só a última aparece.
	if st.word != "" && st.spoken < len([]rune(st.word)) && age > float64(st.spoken)*0.13 {
		k := st.spoken
		r := []rune(st.word)[k]
		st.spoken++
		mx, my := dgMouthAt(st, now)
		st.letters = append(st.letters, dgLetter{
			ch: string(r), x: mx + 0.12 + 0.13*float64(k), y: my + 0.16 + 0.06*float64(k),
			vx:   0.30 + rand.Float64()*0.18,
			vy:   0.55 + rand.Float64()*0.20,
			born: now, life: 1.4 + rand.Float64()*0.5,
		})
	}
	kept := st.letters[:0]
	for _, l := range st.letters {
		if now-l.born < l.life {
			kept = append(kept, l)
		}
	}
	st.letters = kept
}

func relaxDragonInit(st *relaxDragonState, now float64) {
	st.inited = true
	st.seed = rand.Intn(9999)
	st.clock = now
	st.mode, st.until = dgModeFlap, now+5+rand.Float64()*4
	st.next = now + 3.5 + rand.Float64()*5
	st.roarAt = -99
	st.rate, st.amp = dgFlapHz, 1
	st.flapPh = rand.Float64()
	for i := range st.hills {
		st.hills[i] = rand.Float64() * 2 * math.Pi
	}
	for i, n := 0, 62+rand.Intn(28); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64()})
	}
	for i := 0; i < 4; i++ {
		st.clouds = append(st.clouds, dgPuff{
			x: rand.Float64(),
			// Nuvem baixa e comprida, e só na metade de cima do céu: passando na
			// altura do bicho ela viraria risco atravessando o quadro.
			y:    0.18 + rand.Float64()*0.20,
			rx:   0.10 + rand.Float64()*0.12,
			ry:   0.055 + rand.Float64()*0.045,
			vx:   0.004 + rand.Float64()*0.009,
			seed: rand.Intn(9999),
		})
	}
}

// ── Pose ──────────────────────────────────────────────────────────────────────

type dgPose struct {
	x, y    float64 // deriva do bicho no mundo
	pitch   float64
	stroke  float64 // 1 asa no alto, 0 embaixo
	vel     float64 // velocidade do golpe, em ciclos
	amp     float64
	fold    float64
	jaw     float64
	roar    float64
	fire    float64
	tailPh  float64
	tailAmp float64
	legs    float64
}

// dgPoseAt resolve a pose no instante de RENDER. A fase da asa é a do último
// passo mais o que o relógio andou desde então — é essa extrapolação que dá
// 30fps de asa com 10 passos por segundo de simulação.
func dgPoseAt(st *relaxDragonState, now float64) dgPose {
	ph := st.flapPh + st.rate*clampF(now-st.clock, 0, 0.25)
	s, vel := dgStroke(ph)
	amp := st.amp
	// Amplitude menor não pode virar asa meio-batida: o planado é a asa aberta
	// no meio do curso, tremendo de leve.
	s = 0.5 + (s-0.5)*amp
	age := now - st.roarAt
	po := dgPose{
		stroke: s, vel: vel * amp, amp: amp,
		// Dobra só na SUBIDA, e no meio dela: no alto e no fim da descida a asa
		// está aberta, que é justamente quando ela precisa de área.
		fold:    clamp01(vel/1.61) * 0.85 * amp,
		jaw:     dgJawEnv(age),
		fire:    dgFireEnv(age),
		tailPh:  now * 0.42,
		tailAmp: 0.45 + 0.55*amp,
		legs:    0.06 * amp * math.Sin(2*math.Pi*(ph-0.10)),
	}
	po.roar = po.jaw
	// O corpo sobe DEPOIS do golpe: sustentação chega atrasada.
	bob := 0.075 * amp * math.Sin(2*math.Pi*(ph-0.30))
	po.x = -0.26 + 0.10*relaxNoise(now*0.23, st.seed)
	po.y = 0.02 + bob + 0.07*relaxNoise(now*0.19, st.seed+5)
	po.pitch = 0.06*amp*math.Sin(2*math.Pi*(ph-0.45)) + 0.05*po.roar
	return po
}

// dgMouthAt devolve a ponta do focinho no mundo — é dali que sai fogo, brasa e
// letra, e só a pose sabe onde ela está neste instante.
func dgMouthAt(st *relaxDragonState, now float64) (float64, float64) {
	po := dgPoseAt(st, now)
	f := dgFrameAt(po.x, po.y, po.pitch)
	return f.at(1.42, 0.40+0.10*po.roar)
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxDragonFrames(st *relaxDragonState, width, height int, fade, now float64) ([]string, string) {
	if !st.inited {
		stepRelaxDragon(st, now)
	}
	b := newRelaxBrailleVote(maxInt(26, minInt(width, 120)), maxInt(8, minInt(height, 34)))
	po := dgPoseAt(st, now)
	relaxDragonScene(dgCanvasFor(b), st, po, now)

	status := "a asa bate, e o corpo sobe atrás"
	switch {
	case po.fire > 0.05:
		status = "a boca abriu e o escuro ficou laranja"
	case po.jaw > 0.05:
		status = "ele encheu o peito de ar"
	case len(st.embers) > 3:
		status = "faísca subindo, cheiro de queimado"
	case st.mode == dgModeGlide && po.amp < 0.5:
		status = "planando — a asa só corrige o vento"
	}
	return b.lines(relaxStyles(relaxDragonPal, fade)), StyleMuted.Render(status)
}

// relaxDragonScene monta o quadro. Ordem da FRENTE pro FUNDO, porque b.set
// respeita o primeiro que acendeu o subponto: inverter isso é apagar o que está
// na frente.
func relaxDragonScene(c dgCanvas, st *relaxDragonState, po dgPose, now float64) {
	f := dgFrameAt(po.x, po.y, po.pitch)
	p := c.pen(f)

	// ── Fogo, brasa e rugido ── na frente de tudo, inclusive do focinho: é da
	// boca que eles saem.
	mx, my := f.at(1.42, 0.40+0.10*po.roar)
	dgDrawFire(c, st, mx, my, -0.16+po.pitch, po.fire, now)
	dgDrawEmbers(c, st, now)

	// ── Asa da frente ── ela passa na frente do corpo, então vem antes dele.
	arm := lerp(dgArmDown, dgArmUp, po.stroke)
	dgDrawWing(p, dgWingSpec{
		sh: [2]float64{0.24, 0.18}, hip: [2]float64{-0.46, -0.08},
		span: dgSpan, arm: arm, fan: 0.95 + 0.45*po.stroke, lag: 0.135 * po.vel, fold: po.fold,
		lit:  clamp01(0.75 - 0.75*po.stroke),
		memb: dgMemb, membN: dgMembN,
		armTone: dgScale + 3, digit: dgMemb + 1, claw: dgBone + 2,
	})

	dgDrawHead(c, p, f, po)
	dgDrawTrunk(p, po)

	// ── Asa de trás ── mais curta, mais escura e atrás do corpo: é só ela que
	// dá profundidade ao bicho.
	dgDrawWing(p, dgWingSpec{
		sh: [2]float64{0.32, 0.27}, hip: [2]float64{-0.34, 0.04},
		span: dgSpan * 0.84, arm: arm - 0.12, fan: 0.90 + 0.42*po.stroke, lag: 0.11 * po.vel, fold: po.fold,
		lit:  clamp01(0.6 - 0.6*po.stroke),
		memb: dgFar, membN: dgFarN,
		armTone: dgFar + 2, digit: dgFar + 1, claw: dgFar + 3,
	})

	// Congela o bicho: o que vem depois é fundo, e fundo não pode roubar o voto
	// de maioria de uma célula de asa que passa na frente da serra.
	dgSeal(c.b)
	dgDrawHills(c, st)
	dgDrawClouds(c, st, now)
	dgSeal(c.b)
	dgDrawMoon(c, st)
	dgSeal(c.b)
	dgDrawStars(c, st, now)
}

// dgSeal fixa a cor de toda célula já desenhada.
func dgSeal(b *relaxBraille) {
	for cy := 0; cy < b.h; cy++ {
		for cx := 0; cx < b.w; cx++ {
			b.lock(cx, cy)
		}
	}
}

// dgDrawTrunk desenha tronco, cauda, pernas e a crista de espinhos.
func dgDrawTrunk(p relaxPen, po dgPose) {
	// ── Espinhos ── antes do corpo: a base deles fica por cima do couro.
	for i := 0; i < 9; i++ {
		u := 0.10 + float64(i)/9*0.86
		x, y := dgQuad([2]float64{-0.60, -0.05}, [2]float64{-0.10, 0.16}, [2]float64{0.46, 0.08}, u)
		h := 0.045 + 0.035*math.Sin(math.Pi*u)
		dgSpike(p, [2]float64{x, y + 0.14}, 2.05, h, dgBone+1)
	}
	// ── Pernas ── recolhidas, do jeito que bicho voando carrega a pata. Elas
	// balançam com a batida, senão parecem penduradas de gesso.
	sw := po.legs
	hip := [2]float64{-0.30, -0.14}
	knee := dgPolar(hip, -1.28+sw, 0.30)
	ankle := dgPolar(knee, 3.05+sw*0.7, 0.32)
	dgLimb(p, hip, knee, 0.085, 0.055, dgScale+2)
	dgLimb(p, knee, ankle, 0.055, 0.032, dgScale+3)
	for k := -1; k <= 1; k++ {
		dgLimb(p, ankle, dgPolar(ankle, 3.35+float64(k)*0.34, 0.10), 0.020, p.step*0.6, dgBone+2)
	}
	sho := [2]float64{0.30, -0.06}
	elb := dgPolar(sho, -1.15+sw, 0.24)
	paw := dgPolar(elb, 2.55+sw*0.7, 0.20)
	dgLimb(p, sho, elb, 0.065, 0.042, dgScale+2)
	dgLimb(p, elb, paw, 0.042, 0.026, dgScale+3)
	for k := -1; k <= 1; k++ {
		dgLimb(p, paw, dgPolar(paw, 2.90+float64(k)*0.34, 0.085), 0.017, p.step*0.6, dgBone+2)
	}

	// ── Cauda ── onda viajante: a amplitude cresce da raiz pra ponta, então a
	// cauda chicoteia em vez de girar como vara.
	tail := func(u float64) dgSection {
		k := 1 - u // 0 no quadril, 1 na ponta
		x := -0.62 - 1.40*k
		y := -0.05 - 0.12*k*k + 0.17*po.tailAmp*math.Pow(k, 1.35)*
			math.Sin(1.35*k*2*math.Pi-po.tailPh*2*math.Pi)
		return dgSection{x, y, 0.150*math.Pow(1-k, 0.80) + 0.014}
	}
	dgTube(p, tail, dgSteps(p, 1.5), dgScaleTone)
	// Leque da ponta, na direção em que a cauda terminou.
	t0, t1 := tail(0.0), tail(0.06)
	ang := math.Atan2(t1.y-t0.y, t1.x-t0.x) + math.Pi
	fin := dgPolar([2]float64{t0.x, t0.y}, ang, 0.20)
	for _, s := range [2]float64{-1, 1} {
		dgLimb(p, [2]float64{t0.x, t0.y}, dgPolar(fin, ang+s*1.25, 0.12), 0.030, p.step*0.7, dgMemb+3)
	}
	dgLimb(p, [2]float64{t0.x, t0.y}, fin, 0.035, 0.012, dgMemb+4)

	// ── Espinhos da cauda ──
	for i := 1; i < 6; i++ {
		u := float64(i) / 6
		s := tail(1 - u*0.92)
		dgSpike(p, [2]float64{s.x, s.y + s.r*0.7}, 2.15, 0.055*(1-u*0.7)+0.012, dgBone+1)
	}

	// ── Tronco ──
	dgTube(p, func(u float64) dgSection {
		x, y := dgQuad([2]float64{-0.60, -0.05}, [2]float64{-0.10, 0.14}, [2]float64{0.46, 0.06}, u)
		return dgSection{x, y, 0.150 + 0.115*math.Sin(math.Pi*math.Pow(u, 0.90))}
	}, dgSteps(p, 1.2), dgScaleTone)
}

// dgDrawHead desenha pescoço, cabeça, chifres, queixo e o olho. O detalhe vem
// antes do volume: olho e dente sumiriam debaixo do couro se viessem depois.
func dgDrawHead(c dgCanvas, p relaxPen, f dgFrame, po dgPose) {
	lift := 0.10 * po.roar
	base := [2]float64{0.95, 0.47 + lift}
	tip := [2]float64{1.42, 0.40 + lift}
	hinge := [2]float64{1.05, 0.42 + lift}

	// ── Dentes ── só aparecem com a boca aberta, e é a boca aberta que faz o
	// bicho parecer bicho.
	if po.jaw > 0.12 {
		for i := 0; i < 3; i++ {
			x := 1.14 + float64(i)*0.09
			dgLimb(p, [2]float64{x, 0.385 + lift}, [2]float64{x - 0.012, 0.345 + lift}, 0.016, p.step*0.6, dgBone+2)
		}
	}
	// ── Sobrancelha ── o vinco que fecha a cara.
	dgLimb(p, [2]float64{1.06, 0.545 + lift}, [2]float64{1.22, 0.515 + lift}, 0.022, 0.012, dgScale+1)

	// ── Chifres e babados ──
	for i, h := range [2][3]float64{{0.62, 0.92, 0.034}, {0.66, 0.80, 0.026}} {
		root := [2]float64{1.00 - float64(i)*0.05, 0.58 + lift}
		mid := [2]float64{lerp(root[0], h[0], 0.55) + 0.03, lerp(root[1], h[1]+lift, 0.62)}
		dgLimb(p, root, mid, h[2], h[2]*0.6, dgBone+1)
		dgLimb(p, mid, [2]float64{h[0], h[1] + lift}, h[2]*0.6, p.step*0.7, dgBone+2)
	}
	for k := 0; k < 2; k++ {
		root := [2]float64{0.99, 0.40 + lift + float64(k)*0.06}
		dgLimb(p, root, dgPolar(root, 3.05+float64(k)*0.30, 0.15), 0.024, p.step*0.7, dgBone)
	}

	// ── Queixo ── gira em torno da dobradiça: é o queixo que abre, não a
	// cabeça que se parte em duas.
	open := -0.52 * po.jaw
	sn, cs := math.Sincos(open)
	jawPt := func(x, y float64) [2]float64 {
		dx, dy := x-hinge[0], y-hinge[1]
		return [2]float64{hinge[0] + dx*cs - dy*sn, hinge[1] + dx*sn + dy*cs}
	}
	dgTube(p, func(u float64) dgSection {
		x, y := dgQuad(jawPt(1.03, 0.40), jawPt(1.22, 0.35), jawPt(1.40, 0.37), u)
		return dgSection{x, y, lerp(0.055, 0.022, u)}
	}, dgSteps(p, 0.4), func(u, v float64) int {
		if v < -0.2 {
			return dgBelly + 1
		}
		return dgScale + 2
	})

	// ── Cabeça ── cunha do occipital ao focinho.
	dgTube(p, func(u float64) dgSection {
		x, y := dgQuad(base, [2]float64{1.20, 0.50 + lift}, tip, u)
		return dgSection{x, y, lerp(0.098, 0.040, math.Pow(u, 0.80))}
	}, dgSteps(p, 0.55), dgScaleTone)

	// ── Pescoço ──
	dgTube(p, func(u float64) dgSection {
		x, y := dgQuad([2]float64{0.40, 0.09}, [2]float64{0.76, 0.22 + lift}, base, u)
		return dgSection{x, y, lerp(0.150, 0.085, u)}
	}, dgSteps(p, 0.6), dgScaleTone)
	for i := 0; i < 5; i++ {
		u := 0.15 + float64(i)/5*0.8
		x, y := dgQuad([2]float64{0.40, 0.09}, [2]float64{0.76, 0.22 + lift}, base, u)
		dgSpike(p, [2]float64{x - 0.04, y + 0.09}, 2.30, 0.038+0.012*math.Sin(math.Pi*u), dgBone+1)
	}

	// ── Olho ── uma célula inteira, fixada: um ponto de oito perderia a média
	// da célula e o bicho ficaria cego.
	ex, ey := f.at(1.12, 0.495+lift)
	cx, cy := c.cellOf(ex, ey)
	c.b.paint(cx, cy, dgGlint)
}

// dgDrawFire desenha o jato em ASCII, por CÉLULA: rampa de densidade pro miolo
// e halo pontilhado em braille na borda. É a mistura de alfabetos que faz o
// fogo parecer fogo — braille sozinho vira mancha, ASCII sozinho vira letreiro.
func dgDrawFire(c dgCanvas, st *relaxDragonState, mx, my, ang, env, now float64) {
	if env <= 0.02 {
		return
	}
	length := 0.45 + 0.95*env
	sn, cs := math.Sincos(ang)
	x0, y0 := c.cellOf(mx-0.3, my+0.9)
	x1, y1 := c.cellOf(mx+length+0.3, my-0.9)
	for cy := minInt(y0, y1); cy <= maxInt(y0, y1); cy++ {
		for cx := minInt(x0, x1); cx <= maxInt(x0, x1); cx++ {
			wx, wy := c.cellAt(cx, cy)
			dx, dy := wx-mx, wy-my
			s := dx*cs + dy*sn  // distância ao longo do jato
			q := -dx*sn + dy*cs // e o afastamento do eixo
			if s < -0.03 || s > length {
				continue
			}
			fr := clamp01(s / length)
			// Bico fino na boca, aberto na ponta, e serpenteando: cone limpo
			// parece casquinha de sorvete.
			wid := (0.045 + 0.40*math.Pow(fr, 0.85)) * (1 + 0.30*relaxNoise(fr*5.2-now*3.1, st.seed))
			q += 0.11 * fr * relaxNoise(fr*4.4-now*2.6, st.seed+31)
			r := math.Abs(q) / math.Max(0.01, wid)
			if r > 1 {
				continue
			}
			v := env * (1 - r*r) * (1.05 - 0.72*fr)
			// A chama pulsa em bolotas que caminham pro fim do jato.
			v *= 0.70 + 0.58*relaxNoise(fr*9-now*7.5, st.seed+7)
			if v <= 0.055 {
				continue
			}
			if v < 0.17 { // halo: braille em meio-tom, a chama esgarçando
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						px, py := cx*2+dx, cy*4+dy
						if relaxHalftone(px, py) < v*2.6 {
							c.b.set(px, py, dgFire+minInt(int(v*9), 1))
						}
					}
				}
				continue
			}
			ramp := []rune(dgFireRamp)
			g := ramp[minInt(int(v*float64(len(ramp))), len(ramp)-1)]
			c.b.text(cx, cy, string(g), dgFire+minInt(int(v*float64(dgFireN)*1.15), dgFireN-1))
		}
	}
}

// dgDrawEmbers desenha brasa, fumaça e as letras do rugido — tudo caractere,
// tudo com posição fechada no tempo.
func dgDrawEmbers(c dgCanvas, st *relaxDragonState, now float64) {
	for _, e := range st.embers {
		x, y, f := e.at(now)
		cx, cy := c.cellOf(x, y)
		if e.smoke {
			g := dgSmokeGlyph[minInt(int(f*float64(len(dgSmokeGlyph))), len(dgSmokeGlyph)-1)]
			c.b.text(cx, cy, g, dgSmoke+minInt(int((1-f)*float64(dgSmokeN)), dgSmokeN-1))
			continue
		}
		g := dgEmberGlyph[minInt(int(f*float64(len(dgEmberGlyph))), len(dgEmberGlyph)-1)]
		// A brasa apaga descendo a rampa: branco, laranja, vermelho, escuro.
		c.b.text(cx, cy, g, dgFire+minInt(maxInt(int((1-f)*float64(dgFireN)), 0), dgFireN-1))
	}
	for _, l := range st.letters {
		dt := now - l.born
		f := clamp01(dt / l.life)
		x := l.x + l.vx*dt
		y := l.y + l.vy*dt - 0.06*dt*dt
		cx, cy := c.cellOf(x, y)
		c.b.text(cx, cy, l.ch, dgFire+minInt(maxInt(int((1-f)*float64(dgFireN)), 0), dgFireN-1))
	}
}

// dgDrawHills desenha as duas cadeias, da frente pro fundo. Três harmônicas
// porque um seno só entrega o desenho na hora.
func dgDrawHills(c dgCanvas, st *relaxDragonState) {
	for d := 0; d < 2; d++ {
		base, amp, k, tone := -1.36, 0.12, 0, dgHill
		if d == 1 { // a do fundo: pico mais alto e cor lavada pelo ar
			base, amp, k, tone = -1.28, 0.20, 3, dgHill+2
		}
		for x := 0; x < c.sw; x++ {
			u := float64(x) / float64(c.sw) * 2 * math.Pi
			crest := base + amp*(0.62*math.Abs(math.Sin(u*1.3+st.hills[k]))+
				0.30*math.Abs(math.Sin(u*2.7+st.hills[k+1]))+
				0.12*math.Sin(u*5.3+st.hills[k+2]))
			top := c.subY(crest)
			for y := maxInt(0, top); y < c.sh; y++ {
				if c.b.taken(x, y) {
					continue
				}
				t := tone
				if y > top+2 {
					t++ // o pé da encosta afunda no escuro
				}
				c.b.set(x, y, t)
			}
		}
	}
}

// dgDrawClouds desenha faixas de nuvem em meio-tom, andando devagar. A posição
// é função do tempo — nuvem integrada por passo pularia com a cena.
func dgDrawClouds(c dgCanvas, st *relaxDragonState, now float64) {
	for _, cl := range st.clouds {
		fx := math.Mod(cl.x+cl.vx*now, 1)
		if fx < 0 {
			fx++
		}
		cxs := fx * float64(c.sw)
		cys := cl.y * float64(c.sh)
		rxs, rys := cl.rx*float64(c.sw), cl.ry*float64(c.sh)
		for y := int(cys - rys*1.6); y <= int(cys+rys*1.6); y++ {
			for x := int(cxs - rxs*1.3); x <= int(cxs+rxs*1.3); x++ {
				nx, ny := (float64(x)-cxs)/rxs, (float64(y)-cys)/rys
				if ny > 0 {
					ny *= 1.7 // base chata: nuvem redonda por baixo é algodão
				}
				v := 1 - math.Pow(math.Abs(nx), 1.5) - ny*ny
				v += 0.16 * relaxNoise(float64(x)*0.06+float64(cl.seed), cl.seed)
				// Limiar alto de propósito: ponto solto de nuvem toma a célula
				// inteira no voto de maioria e vira sujeira no céu.
				if v <= 0.12 || c.b.taken(x, y) {
					continue
				}
				if relaxHalftone(x, y) > (v-0.10)*0.85 {
					continue
				}
				c.b.set(x, y, dgCloud+minInt(int(v*float64(dgCloudN)*1.2), dgCloudN-1))
			}
		}
	}
}

// dgDrawMoon é o disco com halo: é ele que dá o contraluz do bicho.
func dgDrawMoon(c dgCanvas, st *relaxDragonState) {
	mx, my := float64(c.subX(dgMoonX)), float64(c.subY(dgMoonY))
	r := dgMoonR * c.scale
	for y := int(my - r*2.0); y <= int(my+r*2.0); y++ {
		for x := int(mx - r*2.0); x <= int(mx+r*2.0); x++ {
			if c.b.taken(x, y) {
				continue
			}
			nx, ny := (float64(x)-mx)/r, (float64(y)-my)/r
			d := nx*nx + ny*ny
			switch {
			case d <= 1:
				t := dgMoon + 2
				// Mare: manchas de meio-tom, não crateras desenhadas — nesse
				// tamanho cratera de contorno vira sujeira.
				if relaxNoise(nx*2.6+ny*4.1, st.seed+3) > 0.25 {
					t = dgMoon + 1
				}
				c.b.set(x, y, t)
			case d <= 2.6 && relaxHalftone(x, y) < (2.6-d)*0.10:
				c.b.set(x, y, dgMoon)
			}
		}
	}
}

// dgDrawStars pisca o céu. As mais graúdas viram caractere — é a mesma mistura
// do fogo, aplicada ao fundo.
func dgDrawStars(c dgCanvas, st *relaxDragonState, now float64) {
	for i, s := range st.stars {
		x := int(s.x * float64(c.sw-1))
		y := int(s.y * float64(c.sh) * 0.80)
		if c.b.taken(x, y) {
			continue
		}
		if relaxHalftone(x, y) > 0.30+0.22*math.Sin(now*1.4+float64(i)) {
			continue
		}
		if i%11 == 0 {
			c.b.text(x/2, y/4, dgStarGlyph[i%len(dgStarGlyph)], dgStar)
			continue
		}
		c.b.set(x, y, dgStar)
	}
}
