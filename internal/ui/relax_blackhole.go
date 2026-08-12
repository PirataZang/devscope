package ui

import (
	"math"
	"math/rand"
)

// ── Buraco negro ──────────────────────────────────────────────────────────────
//
// A imagem que todo mundo conhece, e cada parte dela tem um motivo:
//
//   · a sombra no meio é o horizonte — ali não se desenha nada;
//   · o anel fino colado nela é o anel de fóton, luz dando a volta;
//   · o disco passa na frente por baixo, e a metade de trás aparece por cima,
//     curvada pela gravidade — não é simetria, é a lente;
//   · um lado é muito mais claro que o outro (efeito Doppler: o gás que vem na
//     nossa direção chega azul e forte, o que se afasta chega vermelho e fraco).
//
// As estrelas de fundo perto da borda são empurradas pra fora pela mesma lente.

var relaxBhStops = []string{
	"#0A0406", "#3A0D08", "#7A2408", "#B84A10", "#E07A18",
	"#F4A93A", "#FCD877", "#FEF2C0", "#FFFFFF", "#DCEBFF", "#AECDFF",
}

const relaxBhLevels = 26

const (
	relaxBhStar = relaxBhLevels + iota
	relaxBhStarDim
)

var relaxBhRamp = func() []relaxColor {
	out := make([]relaxColor, relaxBhStarDim+1)
	copy(out, relaxRamp(relaxBhStops, relaxBhLevels))
	out[relaxBhStar] = "#E8EEFF"
	out[relaxBhStarDim] = "#4B5570"
	return out
}()

// relaxBhStream é gás caindo: entra de fora, espirala e some no horizonte. É o
// que mostra a puxada — o disco sozinho só gira.
type relaxBhStream struct {
	r, ang float64
	vr     float64
	trail  [][2]float64 // rastro em (raio, ângulo)
}

// relaxBhSpot é um ponto quente orbitando no disco. Sgr A* tem esses de verdade,
// e são eles que dão escala à velocidade do gás.
type relaxBhSpot struct {
	r, ang, w float64
}

type relaxBlackHoleState struct {
	inited  bool
	tick    int
	stars   []relaxSkyPt // normalizado -1..1, em raios de sombra
	incl    float64      // inclinação do disco
	streams []relaxBhStream
	spots   []relaxBhSpot
	nextStr int
	flare   float64 // clarão quando um fluxo cruza o horizonte
}

func relaxBhInit(st *relaxBlackHoleState) {
	st.inited = true
	st.incl = 0.16 + rand.Float64()*0.10
	st.stars = st.stars[:0]
	for i, n := 0, 210+rand.Intn(110); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: (rand.Float64()*2 - 1) * 6, y: (rand.Float64()*2 - 1) * 6})
	}
	st.spots = st.spots[:0]
	for i, n := 0, 2+rand.Intn(2); i < n; i++ {
		r := 1.25 + rand.Float64()*1.3
		st.spots = append(st.spots, relaxBhSpot{
			r: r, ang: rand.Float64() * 2 * math.Pi,
			// Kepleriano: quanto mais perto, mais rápido.
			w: 0.42 / (r * math.Sqrt(r)),
		})
	}
	st.streams = st.streams[:0]
	st.nextStr = 10
}

func stepRelaxBlackHole(st *relaxBlackHoleState) {
	if !st.inited {
		relaxBhInit(st)
	}
	st.tick++
	for i := range st.spots {
		st.spots[i].ang += st.spots[i].w
	}
	st.flare *= 0.90

	if st.nextStr--; st.nextStr <= 0 && len(st.streams) < 3 {
		st.streams = append(st.streams, relaxBhStream{
			r: 4.6 + rand.Float64()*1.6, ang: rand.Float64() * 2 * math.Pi,
			vr: -0.012 - rand.Float64()*0.010,
		})
		st.nextStr = 26 + rand.Intn(50)
	}
	keep := st.streams[:0]
	for _, s := range st.streams {
		s.trail = append(s.trail, [2]float64{s.r, s.ang})
		if len(s.trail) > 26 {
			s.trail = s.trail[1:]
		}
		// Conservação de momento angular de mentirinha: caindo, gira cada vez
		// mais rápido. É daí que sai a espiral apertando.
		s.ang += 0.30 / (s.r * math.Sqrt(s.r))
		s.vr *= 1.045 // e cai cada vez mais depressa
		s.r += s.vr
		if s.r <= 1.02 {
			st.flare = 1 // atravessou o horizonte
			continue
		}
		keep = append(keep, s)
	}
	st.streams = keep
}

func relaxBlackHoleFrames(st *relaxBlackHoleState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxBlackHole(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBraille(w, h)
	sw, sh := w*2, h*4
	cx, cy := float64(sw-1)/2, float64(sh-1)/2
	t := float64(st.tick) * 0.1

	// Raio da sombra em subpixels; o disco vai até ~4 vezes isso.
	sr := math.Min(float64(sw)/8.0, float64(sh)/4.2)
	ci := st.incl

	// Estrelas de fundo empurradas pra fora pela lente. Perto da borda a imagem
	// não só se desloca: ela se ESTICA na direção tangencial, e é por isso que
	// as estrelas viram riscos curvos em vez de pontos. Quem cai dentro da
	// sombra some, o que dá a borda nítida do horizonte.
	for i, p := range st.stars {
		r := math.Hypot(p.x, p.y)
		if r < 0.05 {
			continue
		}
		r2 := r + 1.55/r
		k := r2 / r
		px, py := p.x*k, p.y*k
		lvl := relaxBhStarDim
		if relaxHalftone(int(cx+px*sr), int(cy+py*sr)) < 0.35+0.18*math.Sin(t*0.6+float64(i)) {
			lvl = relaxBhStar
		}
		// Ampliação tangencial: só quem chega perto do anel vira risco curvo;
		// longe dali a estrela continua um ponto. Sem esse corte a tela toda
		// fica riscada e a imagem se perde.
		na := math.Atan2(py, px)
		stretch := clamp01((3.6 - r2) / 1.1)
		arc := stretch * stretch * 0.11
		if arc < 0.01 {
			b.set(int(cx+px*sr), int(cy+py*sr*0.95), lvl)
			continue
		}
		for d := -arc; d <= arc; d += 0.02 {
			b.set(int(cx+math.Cos(na+d)*r2*sr), int(cy+math.Sin(na+d)*r2*sr*0.95), lvl)
		}
	}

	// O limiar cheio é de propósito: com ele o lado claro ainda ganha textura
	// em vez de virar uma chapa, e o lado que se afasta some quase todo.
	put := func(x, y, lum float64) {
		ix, iy := int(x), int(y)
		if lum <= relaxHalftone(ix, iy) {
			return
		}
		b.set(ix, iy, minInt(int(clamp01(lum)*float64(relaxBhLevels-1)+0.5), relaxBhLevels-1))
	}

	// Disco. Uma volta em ângulo por camada de raio; a metade de trás sobe pela
	// lente e a da frente passa por baixo do horizonte.
	//
	// Tudo o que só depende do ângulo entra em tabela: são ~10 mil amostras por
	// frame, e um math.Sin (ou pior, um math.Pow) dentro do laço custava mais
	// que o resto da cena inteira.
	steps := int(sr * 7)
	ca := make([]float64, steps)
	sa := make([]float64, steps)
	c3, s3 := make([]float64, steps), make([]float64, steps)
	c15, s15 := make([]float64, steps), make([]float64, steps)
	dop := make([]float64, steps)
	for i := 0; i < steps; i++ {
		a := float64(i) / float64(steps) * 2 * math.Pi
		sa[i], ca[i] = math.Sincos(a)
		s3[i], c3[i] = math.Sincos(3 * a)
		s15[i], c15[i] = math.Sincos(1.5 * a)
		// Doppler: o gás que vem na nossa direção chega muito mais brilhante.
		d := clamp01(0.5 + 0.5*ca[i])
		dop[i] = 0.24 + 1.55*d*d
	}

	for layer := 0.0; layer < 1.0; layer += 0.016 {
		rr := 1.10 + layer*2.1
		// Mais perto, mais quente; a borda de fora se apaga.
		radial := clamp01((rr-1.05)/0.4) * clamp01((3.3-rr)/1.5)
		if radial <= 0 {
			continue
		}
		// Kepleriano: o gás de dentro dá a volta muito mais rápido.
		// Estrias radiais finas: o disco não é liso, é feito de anéis.
		stria := 0.80 + 0.20*math.Sin(rr*34+t*0.4)
		bb := t * 2.6 / (rr * math.Sqrt(rr))
		sB, cB := math.Sincos(bb)
		sB2, cB2 := math.Sincos(0.5*bb - rr)
		for i := 0; i < steps; i++ {
			// sin(3a−B) e sin(1,5a−(B/2−r)), abertos em produto de tabelas.
			turb := 0.62 + 0.38*(s3[i]*cB-c3[i]*sB)*(s15[i]*cB2-c15[i]*sB2)
			lum := radial * turb * dop[i] * stria
			if lum < 0.02 {
				continue
			}
			sa, ca := sa[i], ca[i]
			// O disco tem espessura: sem esse tremor ele vira um risco de um
			// subpixel e perde o ar de gás.
			thick := float64(relaxHash(i, int(layer*1000))%100-50) / 50 * 0.05 * rr * sr
			if sa >= 0 {
				// Frente: o disco passa por baixo, na frente do horizonte.
				put(cx+rr*ca*sr, cy+rr*sa*ci*sr+thick, lum)
				continue
			}
			// Trás: a luz contorna o buraco e a imagem aparece comprimida num
			// arco colado ao anel de fóton — as camadas de fora quase não
			// abrem, que é o que faz o "halo" por cima em vez de um leque.
			lr := 1.05 + (rr-1.10)*0.16
			put(cx+lr*ca*sr, cy+lr*sa*sr*0.98+thick*0.4, lum*0.92)
			// Imagem secundária: um fio da mesma luz reaparece por baixo,
			// ainda mais comprimido.
			l2 := 1.03 + (rr-1.10)*0.05
			put(cx+l2*ca*sr, cy-l2*sa*sr*0.98, lum*0.30)
		}
	}

	// Pontos quentes: blobs mais brilhantes girando dentro do disco.
	for _, sp := range st.spots {
		for k := 0.0; k < 1; k += 0.06 {
			ra := sp.r + (k-0.5)*0.30
			for d := -0.34; d < 0.34; d += 0.02 {
				a := sp.ang + d
				sa, ca := math.Sincos(a)
				lum := 1.15 * (1 - math.Abs(d)/0.34) * (1 - math.Abs(k-0.5)*2)
				if sa >= 0 {
					put(cx+ra*ca*sr, cy+ra*sa*ci*sr, lum)
				} else {
					lr := 1.05 + (ra-1.10)*0.16
					put(cx+lr*ca*sr, cy+lr*sa*sr*0.98, lum*0.9)
				}
			}
		}
	}

	// Fluxos caindo: o rastro fica mais brilhante e mais esticado conforme se
	// aproxima, e desaparece no horizonte.
	for _, s := range st.streams {
		for j, q := range s.trail {
			f := float64(j) / float64(maxInt(1, len(s.trail)-1))
			rr, ang := q[0], q[1]
			sa, ca := math.Sincos(ang)
			lum := f * f * clamp01((6.4-rr)/3.0) * 1.5
			// Maré: perto do buraco o filamento se estica no sentido do giro.
			spread := 0.05 + 0.30/(rr*rr)
			for d := -spread; d <= spread; d += 0.02 {
				sa2, ca2 := math.Sincos(ang + d)
				if sa >= 0 {
					put(cx+rr*ca2*sr, cy+rr*sa2*ci*sr, lum)
				} else {
					lr := 1.05 + (rr-1.10)*0.16
					put(cx+lr*ca2*sr, cy+lr*sa2*sr*0.98, lum*0.85)
				}
			}
			_ = ca
		}
	}

	// Anel de fóton e sombra. A sombra é apagada por último: qualquer gás que
	// tenha caído dentro dela sai, menos o que passa na frente.
	for i := 0; i < steps; i++ {
		d := clamp01(0.5 + 0.5*ca[i])
		lum := (0.55 + 0.45*d*d) * (1 + 0.6*st.flare)
		lvl := minInt(int(clamp01(lum)*float64(relaxBhLevels-1)+0.5), relaxBhLevels-1)
		for _, k := range []float64{1.0, 1.02, 1.04} {
			b.set(int(cx+ca[i]*sr*k), int(cy+sa[i]*sr*k*0.98), lvl)
		}
	}

	status := "o gás cai em espiral"
	switch {
	case st.flare > 0.35:
		status = "alguma coisa atravessou o horizonte"
	case len(st.streams) == 0:
		status = "nada escapa daqui"
	}
	return b.lines(relaxStyles(relaxBhRamp, fade)), StyleMuted.Render(status)
}
