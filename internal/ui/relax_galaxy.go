package ui

import (
	"math"
	"math/rand"
)

// ── Galáxia ───────────────────────────────────────────────────────────────────
//
// Braille: cada estrela é um subpixel, então dá pra pôr milhares delas e os
// braços aparecem por densidade de estrela, como numa foto, em vez de por
// glifos escolhidos a dedo.
//
// Os braços vêm de órbitas elípticas precessantes: cada estrela percorre a sua
// elipse e a orientação da elipse gira com o raio. Onde as elipses se apinham
// nasce o braço — que fica nítido para sempre, em vez de se enrolar num borrão,
// que é o que acontece girando estrelas soltas em círculo.
//
// Só isso, porém, desenha dois braços e nada mais — um S girando. Foto de
// galáxia é um DISCO LUMINOSO contínuo (bilhões de estrelas que nenhum
// telescópio separa) com os pontos brilhantes por cima. Então há duas camadas:
//
//   · um campo de brilho de superfície: disco exponencial × espiral logarítmica,
//     com barra central e faixas de poeira — é ele que dá corpo;
//   · as estrelas em órbita, os berçários e o bulbo desenhados por cima, que
//     dão o granulado e o movimento.
//
// Quem decide se o ponto acende é o meio-tom: brilho contra um limiar fixo por
// posição. É isso que faz o miolo ficar sólido, o braço granulado e a periferia
// esgarçar, tudo com a mesma linha de código.

var (
	relaxGxCoolStops = []string{"#101830", "#1D2B54", "#31508E", "#5B80C2", "#93B4E4", "#C8DCF7", "#F0F6FF"}
	relaxGxWarmStops = []string{"#1E160E", "#43301A", "#7C5926", "#B98B44", "#E4BC77", "#F7E3B8", "#FFF8E8"}
)

// Doze degraus por rampa custava caro no agrupamento de cores da linha (mais
// nível, mais troca de trecho); nove são indistinguíveis a olho e mais baratos.
const relaxGxLevels = 9

const (
	relaxGxCool = 0
	relaxGxWarm = relaxGxLevels
	relaxGxHII  = 2 * relaxGxLevels
	relaxGxFlar = relaxGxHII + 1
)

var relaxGxRamp = func() []relaxColor {
	out := make([]relaxColor, relaxGxFlar+1)
	copy(out[relaxGxCool:], relaxRamp(relaxGxCoolStops, relaxGxLevels))
	copy(out[relaxGxWarm:], relaxRamp(relaxGxWarmStops, relaxGxLevels))
	out[relaxGxHII] = "#E4739E" // berçário estelar
	out[relaxGxFlar] = "#FFFBF0"
	return out
}()

// relaxGalaxyStar guarda seno e cosseno em vez de ângulos. Para encher os
// braços são precisas dezenas de milhares de estrelas, e uma chamada de math.Sin
// por estrela por frame não cabe no orçamento — então a fase avança por rotação
// incremental (quatro multiplicações) e nada aqui chama trigonometria.
type relaxGalaxyStar struct {
	a, bfac  float64 // semieixo maior e razão do menor
	base     float64
	cth, sth float64 // fase na elipse
	cO, sO   float64 // rotação por passo
	cA, sA   float64 // orientação da elipse, a menos da fase global do padrão
	hii      bool
}

type relaxGalaxyState struct {
	inited  bool
	tick    int
	stars   []relaxGalaxyStar
	halo    []relaxSkyPt // aglomerados do halo, em normalizado -1..1
	twist   float64      // quanto a elipse gira por unidade de raio
	pattern float64
	omegaP  float64
	pitch   float64 // abertura da espiral do campo de brilho
	barA    float64 // orientação da barra central, relativa ao padrão
	barLen  float64

	snIdx  int
	snT    float64
	snNext float64
}

const relaxGalaxyFlash = 2.6

func relaxGalaxyInit(st *relaxGalaxyState) {
	st.inited = true
	st.twist = 3.0 + rand.Float64()*1.6
	st.pattern = rand.Float64() * 2 * math.Pi
	// O padrão espiral gira rígido: uma volta em ~35–50s. Devagar demais e a
	// galáxia parece uma foto, já que as estrelas brilhantes ficam nos braços.
	st.pitch = 0.30 + rand.Float64()*0.14 // tangente do ângulo de abertura
	st.barA = rand.Float64() * math.Pi
	st.barLen = 0.16 + rand.Float64()*0.16
	st.omegaP = 0.13 + rand.Float64()*0.05
	st.snIdx, st.snT, st.snNext = -1, -1, 5+rand.Float64()*9

	total := 6500 + rand.Intn(2000)
	st.stars = make([]relaxGalaxyStar, 0, total)
	for i := 0; i < total; i++ {
		a := 0.07 + 0.93*math.Pow(rand.Float64(), 0.72)
		th := rand.Float64() * 2 * math.Pi
		// Rotação diferencial: o miolo dá a volta em ~20s, a borda bem mais
		// devagar. Só o padrão é rígido.
		om := 0.052 / (0.42 + a*1.5)
		st.stars = append(st.stars, relaxGalaxyStar{
			a:    a,
			bfac: 1 - (0.54 + 0.20*a + rand.Float64()*0.08),
			base: 0.30 + (1-a)*0.34 + rand.Float64()*0.30,
			cth:  math.Cos(th), sth: math.Sin(th),
			cO: math.Cos(om), sO: math.Sin(om),
			cA: math.Cos(a * st.twist), sA: math.Sin(a * st.twist),
			// Berçários estelares só existem nos braços, e longe do bulbo.
			hii: a > 0.30 && rand.Intn(260) == 0,
		})
	}

	// Halo: aglomerados velhos, esparsos, numa esfera bem maior que o disco.
	st.halo = st.halo[:0]
	for i, n := 0, 60+rand.Intn(40); i < n; i++ {
		r := math.Pow(rand.Float64(), 0.5) * 1.45
		th, ph := rand.Float64()*2*math.Pi, math.Asin(rand.Float64()*2-1)
		st.halo = append(st.halo, relaxSkyPt{x: r * math.Cos(ph) * math.Cos(th), y: r * math.Sin(ph)})
	}
}

func stepRelaxGalaxy(st *relaxGalaxyState) {
	if !st.inited {
		relaxGalaxyInit(st)
	}
	st.tick++
	const dt = 0.1
	st.pattern += st.omegaP * dt
	for i := range st.stars {
		s := &st.stars[i]
		c, sn := s.cth*s.cO-s.sth*s.sO, s.sth*s.cO+s.cth*s.sO
		// Renormaliza: girar por multiplicação milhares de vezes derraparia o
		// raio, e as estrelas iam encolhendo pro centro.
		k := 1.5 - 0.5*(c*c+sn*sn)
		s.cth, s.sth = c*k, sn*k
	}
	if st.snT >= 0 {
		if st.snT += dt; st.snT > relaxGalaxyFlash {
			st.snT, st.snIdx = -1, -1
		}
		return
	}
	if st.snNext -= dt; st.snNext <= 0 && len(st.stars) > 0 {
		st.snIdx, st.snT = rand.Intn(len(st.stars)), 0
		st.snNext = 8 + rand.Float64()*14
	}
}

func relaxGalaxyFrames(st *relaxGalaxyState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxGalaxy(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBrailleVote(w, h)
	sw, sh := float64(w*2), float64(h*4)
	cx, cy := (sw-1)/2, (sh-1)/2

	t := float64(st.tick) * 0.1
	// A "câmera" tomba e roda devagar: o disco respira em vez de ficar chapado.
	tilt := 0.50 + 0.14*math.Sin(t*0.043)
	roll := 0.24 * math.Sin(t*0.029)
	sinR, cosR := math.Sin(roll), math.Cos(roll)
	// O raio é limitado pelo eixo mais apertado depois do tombo — subpixel é
	// quadrado, então não há correção de proporção a fazer aqui.
	rad := math.Min(sw*0.47, sh*0.47/(tilt+0.16))

	project := func(dx, dy float64) (float64, float64) {
		dy *= tilt
		return cx + (dx*cosR-dy*sinR)*rad, cy + (dx*sinR+dy*cosR)*rad
	}

	// Estrelas, bulbo e berçários vêm ANTES do campo: relaxBraille não
	// sobrescreve, e é o granulado que tem de ficar por cima do disco.
	relaxGalaxyStars(st, b, cx, cy, rad, tilt, sinR, cosR, t)

	// ── Campo de brilho ── o disco propriamente dito.
	relaxGalaxyDisk(st, b, cx, cy, rad, tilt, sinR, cosR, int(sw), int(sh))

	// Halo por último: é o mais apagado e só preenche o que sobrou.
	for i, p := range st.halo {
		x, y := project(p.x, p.y)
		ix, iy := int(x), int(y)
		if relaxHalftone(ix, iy) > 0.30+0.16*math.Sin(t*0.4+float64(i)) {
			continue
		}
		b.set(ix, iy, relaxGxWarm+2)
	}

	status := "uma galáxia girando devagar"
	if st.snT >= 0 && st.snIdx >= 0 && st.snIdx < len(st.stars) {
		s := st.stars[st.snIdx]
		cpat, spat := math.Cos(st.pattern), math.Sin(st.pattern)
		cp := s.cA*cpat - s.sA*spat
		sp := s.sA*cpat + s.cA*spat
		ex, ey := s.a*s.cth, s.a*s.bfac*s.sth
		x, y := project(ex*cp-ey*sp, ex*sp+ey*cp)
		relaxGalaxySupernova(b, x, y, math.Sin(st.snT/relaxGalaxyFlash*math.Pi))
		status = "uma supernova acende num braço"
	}
	return b.lines(relaxStyles(relaxGxRamp, fade)), StyleMuted.Render(status)
}

// relaxGalaxySupernova desenha o clarão: núcleo branco e quatro raios que se
// esticam e apagam junto com w (0→1→0).
func relaxGalaxySupernova(b *relaxBraille, x, y, w float64) {
	if w <= 0.02 {
		return
	}
	ix, iy := int(x), int(y)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			b.set(ix+dx, iy+dy, relaxGxFlar)
		}
	}
	reach := int(w * 11)
	for i := 2; i <= reach; i++ {
		f := 1 - float64(i)/float64(reach+1)
		if relaxHalftone(ix+i, iy) > f {
			continue
		}
		lvl := relaxGxCool + minInt(int(f*float64(relaxGxLevels-1))+4, relaxGxLevels-1)
		b.set(ix+i, iy, lvl)
		b.set(ix-i, iy, lvl)
		b.set(ix, iy+i/2, lvl)
		b.set(ix, iy-i/2, lvl)
	}
}

// relaxGalaxyStars desenha o granulado: bulbo, estrelas em órbita e berçários.
// Vem antes do campo porque relaxBraille não sobrescreve ponto aceso.
func relaxGalaxyStars(st *relaxGalaxyState, b *relaxBraille, cx, cy, rad, tilt, sinR, cosR, t float64) {
	// Bulbo: gaussiana quente e densa. O meio-tom faz o miolo virar bloco cheio
	// e as bordas esgarçarem sozinhas.
	brx := rad * 0.19
	bry := brx * tilt
	for dy := -int(bry) - 2; dy <= int(bry)+2; dy++ {
		for dx := -int(brx) - 2; dx <= int(brx)+2; dx++ {
			d := math.Hypot(float64(dx)/brx, float64(dy)/bry)
			if d > 1.25 {
				continue
			}
			lum := 1.15 * math.Exp(-2.1*d*d)
			ix, iy := int(cx)+dx, int(cy)+dy
			if lum <= relaxHalftone(ix, iy)*0.95 {
				continue
			}
			b.set(ix, iy, relaxGxWarm+minInt(int(lum*float64(relaxGxLevels-1)+0.5), relaxGxLevels-1))
		}
	}

	cpat, spat := math.Cos(st.pattern), math.Sin(st.pattern)
	shimmer := 0.90 + 0.10*math.Sin(t*1.7)
	for i := range st.stars {
		s := &st.stars[i]
		// A orientação da elipse é a*twist mais a fase global, e o cosseno da
		// soma sai dos dois pré-calculados.
		cp := s.cA*cpat - s.sA*spat
		sp := s.sA*cpat + s.cA*spat
		ex, ey := s.a*s.cth, s.a*s.bfac*s.sth
		// project em linha: são dezenas de milhares de chamadas por frame e a
		// closure aparecia no perfil.
		dx, dy := ex*cp-ey*sp, (ex*sp+ey*cp)*tilt
		ix := int(cx + (dx*cosR-dy*sinR)*rad)
		iy := int(cy + (dx*sinR+dy*cosR)*rad)

		// Nas ápsides as órbitas se apinham: é ali o braço, e é ali que nascem
		// as estrelas jovens e azuis.
		arm := s.cth * s.cth
		crowd := arm * arm
		lum := s.base * (0.24 + 0.40*arm + 0.55*crowd) * shimmer

		// Faixa de poeira num lado só do braço — a de verdade fica na borda de
		// dentro. Simétrica nos dois lados daria um borrão, não relevo.
		if s.sth > 0.26 && s.sth < 0.50 && s.a > 0.20 {
			lum *= 0.34
		}

		// O meio-tom é pra esgarçar a periferia, não pra ralear o disco: o
		// ganho põe braço e miolo acima do limiar quase sempre.
		if lum*1.9 <= relaxHalftone(ix, iy) {
			continue
		}
		if s.hii && crowd > 0.5 {
			b.set(ix, iy, relaxGxHII)
			b.set(ix+1, iy, relaxGxHII)
			continue
		}
		// Estrela do miolo é velha e dourada; a do braço é jovem e azul.
		lvl := minInt(int(clamp01(lum)*float64(relaxGxLevels-1)+0.5), relaxGxLevels-1)
		if s.a*0.62+crowd*0.5 < 0.42 {
			b.set(ix, iy, relaxGxWarm+lvl)
		} else {
			b.set(ix, iy, relaxGxCool+lvl)
		}
	}

}

// relaxGalaxyDisk pinta o brilho de superfície: disco exponencial modulado por
// uma espiral logarítmica, com barra central e faixa de poeira. É a camada que
// transforma "dois braços de pontinhos" em galáxia.
func relaxGalaxyDisk(st *relaxGalaxyState, b *relaxBraille, cx, cy, rad, tilt, sinR, cosR float64, sw, sh int) {
	// Tabelas por raio: o perfil do disco e a fase da espiral dependem só de r,
	// e ln() por subpixel sairia caro.
	const nr = 288
	var prof, sinL, cosL [nr]float64
	for i := 0; i < nr; i++ {
		r := float64(i) / nr * 1.32
		// Escala de 0,42 do raio visível: fisicamente o disco cai mais rápido
		// que isso, mas aí a periferia não acende ponto nenhum.
		prof[i] = math.Exp(-r / 0.42)
		if r > 0.02 {
			// Espiral logarítmica: o braço fica onde 2θ − ln(r)/tan(p) é fixo.
			sinL[i], cosL[i] = math.Sincos(math.Log(r) / st.pitch)
		}
	}

	// Barra: gira junto com o padrão, senão descolaria dos braços.
	bs, bc := math.Sincos(st.barA + st.pattern)
	pat2s, pat2c := math.Sincos(2 * st.pattern)

	x0, x1 := maxInt(0, int(cx-rad)-1), minInt(sw-1, int(cx+rad)+1)
	y0, y1 := maxInt(0, int(cy-rad)-1), minInt(sh-1, int(cy+rad)+1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			// Desfaz câmera: tela → coordenada de disco.
			ux := (float64(x) - cx) / rad
			uy := (float64(y) - cy) / rad
			dx := ux*cosR + uy*sinR
			dy := (-ux*sinR + uy*cosR) / tilt
			r2 := dx*dx + dy*dy
			if r2 >= 1.32*1.32 || r2 < 1e-6 {
				continue
			}
			r := math.Sqrt(r2)
			ri := int(r / 1.32 * nr)
			if ri >= nr {
				continue
			}

			// cos e sen de 2θ sem atan2, direto das componentes.
			c2 := (dx*dx - dy*dy) / r2
			s2 := 2 * dx * dy / r2
			// A espiral gira com o padrão: soma-se 2·pattern à fase.
			c2r := c2*pat2c - s2*pat2s
			s2r := s2*pat2c + c2*pat2s
			ph := c2r*cosL[ri] + s2r*sinL[ri] // cos(2θ − ln r/tan p)
			arm := 0.5 + 0.5*ph
			arm *= arm

			lum := prof[ri] * (0.26 + 1.05*arm)

			// Barra central: alongada num eixo, curta no outro.
			along := dx*bc + dy*bs
			perp := -dx*bs + dy*bc
			if math.Abs(along) < st.barLen*1.7 {
				lum += 1.05 * math.Exp(-(perp*perp)/(0.0042)-(along*along)/(st.barLen*st.barLen))
			}

			// Poeira: a faixa fica na borda de dentro do braço, e por isso a
			// fase dela é a do braço com um deslocamento.
			dust := 0.5 + 0.5*(ph*0.62-math.Sqrt(math.Max(0, 1-ph*ph))*0.78)
			lum *= 1 - 0.62*dust*dust*dust*clamp01((r-0.12)*4)

			if lum <= relaxHalftone(x, y)*0.26 {
				continue
			}
			lvl := minInt(int(clamp01(lum)*float64(relaxGxLevels-1)+0.5), relaxGxLevels-1)
			// Miolo e barra dourados, braços azulados.
			if r*1.7+arm*0.35 < 0.62 {
				b.set(x, y, relaxGxWarm+lvl)
			} else {
				b.set(x, y, relaxGxCool+lvl)
			}
		}
	}
}
