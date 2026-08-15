package ui

import (
	"math"
)

// ── Peça de xadrez ────────────────────────────────────────────────────────────
//
// Uma dama torneada, girando. O corpo é sólido de revolução: um perfil (raio
// por altura) varrido em torno do eixo. Só que sólido de revolução girando no
// próprio eixo não se mexe — a silhueta é a mesma em qualquer ângulo. Por isso
// a peça tem duas coisas que quebram a simetria: as CANELURAS da coroa, que
// modulam o raio com o ângulo, e o colar de contas. São elas que fazem o giro
// aparecer, junto com o brilho especular caminhando pela superfície.
//
// Desenho com z-buffer próprio em vez do desenho de trás pra frente: numa peça
// torneada a mesma coluna de tela tem frente e fundo, e ordenar por profundidade
// média erraria a borda. O buffer guarda nível e z por subponto e só no fim
// despeja no Braille.

// relaxChessProfile é o contorno da dama, do pé (y=0) ao topo (y=1): raio por
// altura. Mexer aqui é redesenhar a peça inteira — nada mais depende dele.
//
// A proporção é o que separa dama de peão: pé estreito (meio diâmetro para uma
// altura inteira, 2:1), haste longa e coroa alta. Peça atarracada lê como peão
// por mais coroa que tenha.
var relaxChessProfile = [][2]float64{
	{0.000, 0.000}, {0.252, 0.000}, {0.252, 0.028}, {0.234, 0.050},
	{0.198, 0.072}, {0.152, 0.096}, {0.116, 0.124}, {0.094, 0.156},
	{0.081, 0.202}, {0.073, 0.264}, {0.070, 0.332}, {0.074, 0.400},
	{0.084, 0.462}, {0.100, 0.512}, {0.124, 0.548}, {0.141, 0.572},
	{0.128, 0.596}, {0.104, 0.614}, {0.092, 0.638}, {0.088, 0.670},
	{0.099, 0.702}, {0.121, 0.732}, {0.149, 0.758}, {0.171, 0.788},
	{0.179, 0.822}, {0.174, 0.852}, {0.160, 0.880}, {0.140, 0.902},
	{0.118, 0.918}, {0.070, 0.930}, {0.058, 0.945}, {0.065, 0.962},
	{0.077, 0.978}, {0.066, 0.990}, {0.040, 0.997}, {0.000, 1.000},
}

const (
	relaxChFlutes = 9    // caneluras da coroa
	relaxChBeads  = 8    // contas do colar
	relaxChNU     = 190  // amostras ao longo do perfil
	relaxChNV     = 168  // amostras em volta do eixo
	relaxChCrown0 = 0.68 // faixa canelada, em fração da altura
	relaxChCrown1 = 0.90
	relaxChBeadY  = 0.566 // altura do colar
	relaxChBeadR  = 0.152 // raio em que as contas orbitam
)

// A paleta é uma escada de brilho só: reflexo, marfim, realce. A célula Braille
// aqui usa MÉDIA, não voto — a peça é superfície contínua, e a média é o que
// entrega meio-tom entre dois níveis vizinhos em vez de escada. Só funciona
// porque as três famílias estão em ordem de brilho: média entre vizinhos cai
// sempre numa cor plausível.
const (
	relaxChReflN  = 5
	relaxChIvoryN = 11
	relaxChRefl   = 0
	relaxChIvory  = relaxChRefl + relaxChReflN
	relaxChSpec   = relaxChIvory + relaxChIvoryN
	relaxChLast   = relaxChSpec
)

var relaxChessPal = func() []relaxColor {
	out := make([]relaxColor, relaxChLast+1)
	copy(out[relaxChRefl:], relaxRamp([]string{"#0B0E14", "#141B26", "#1F2A3A", "#2C3A4E", "#3B4C64"}, relaxChReflN))
	copy(out[relaxChIvory:], relaxRamp([]string{
		"#100C0C", "#241C18", "#3C2F26", "#584739", "#77614E",
		"#987E66", "#B69A80", "#D0B79A", "#E4CFB4", "#F3E4CE", "#FFF9EE"}, relaxChIvoryN))
	out[relaxChSpec] = "#FFFFFF"
	return out
}()

// relaxChessSample é um ponto do perfil já com a normal no plano (r, y). O
// perfil é fixo, então isto sai uma vez pro programa inteiro.
type relaxChessSample struct{ r, y, nr, ny float64 }

var relaxChessSamples = func() []relaxChessSample {
	out := make([]relaxChessSample, relaxChNU)
	for i := 0; i < relaxChNU; i++ {
		f := float64(i) / float64(relaxChNU-1) * float64(len(relaxChessProfile)-1)
		k := minInt(int(f), len(relaxChessProfile)-2)
		t := f - float64(k)
		p, q := relaxChessProfile[k], relaxChessProfile[k+1]
		dr, dy := q[0]-p[0], q[1]-p[1]
		l := math.Hypot(dr, dy)
		if l == 0 {
			l = 1
		}
		out[i] = relaxChessSample{
			r: lerp(p[0], q[0], t), y: lerp(p[1], q[1], t),
			// Normal do perfil: perpendicular à tangente, apontando pra fora.
			nr: dy / l, ny: -dr / l,
		}
	}
	return out
}()

type relaxChessState struct {
	inited bool
	tick   int
	spin   float64
	nod    float64
}

func stepRelaxChess(st *relaxChessState) {
	if !st.inited {
		st.inited = true
		st.spin = 0.7
	}
	st.tick++
	st.spin += 0.030
	// Cabeceio lento: sem ele a câmera fica travada numa altura só e a peça
	// perde a leitura de volume que só vem de ver o topo e depois o perfil.
	st.nod = 0.30 + 0.16*math.Sin(float64(st.tick)*0.0125)
}

// ── z-buffer ──────────────────────────────────────────────────────────────────

type relaxChessBuf struct {
	w, h int
	lvl  []int16
	z    []float32
}

func newRelaxChessBuf(w, h int) *relaxChessBuf {
	zb := &relaxChessBuf{w: w, h: h, lvl: make([]int16, w*h), z: make([]float32, w*h)}
	for i := range zb.lvl {
		zb.lvl[i] = -1
	}
	return zb
}

// put grava o subponto se ele estiver na frente do que já está lá. z maior é
// mais perto: a câmera olha do +z.
func (zb *relaxChessBuf) put(x, y int, z float64, lvl int) {
	if x < 0 || y < 0 || x >= zb.w || y >= zb.h {
		return
	}
	i := y*zb.w + x
	if zb.lvl[i] >= 0 && float64(zb.z[i]) >= z {
		return
	}
	zb.lvl[i], zb.z[i] = int16(lvl), float32(z)
}

func (zb *relaxChessBuf) blit(b *relaxBraille) {
	for y := 0; y < zb.h; y++ {
		for x := 0; x < zb.w; x++ {
			if l := zb.lvl[y*zb.w+x]; l >= 0 {
				b.set(x, y, int(l))
			}
		}
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxChessFrames(st *relaxChessState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxChess(st)
	}
	w := maxInt(20, minInt(width, 110))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBraille(w, h)
	relaxChessDraw(st, b)

	// A face que está de frente pra gente, em nome de casa de xadrez.
	status := "a dama gira devagar"
	switch int(math.Mod(st.spin/(2*math.Pi)*4+0.5, 4)) {
	case 1:
		status = "de perfil"
	case 2:
		status = "de costas"
	case 3:
		status = "voltando"
	}
	return b.lines(relaxStyles(relaxChessPal, fade)), StyleMuted.Render(status)
}

func relaxChessDraw(st *relaxChessState, b *relaxBraille) {
	sw, sh := b.w*2, b.h*4
	zb := newRelaxChessBuf(sw, sh)

	// Câmera: gira em torno do eixo e olha um pouco de cima.
	sa, ca := math.Sin(st.spin), math.Cos(st.spin)
	se, ce := math.Sin(st.nod), math.Cos(st.nod)
	scale := math.Min(float64(sw)/0.72, float64(sh)/1.44)
	cx, cy := float64(sw)/2, float64(sh)*0.40
	const persp = 5.2
	// A peça é desenhada com o pé na origem; o corpo sobe em y. Na tela y cresce
	// pra baixo, então o eixo é invertido aqui e em nenhum outro lugar.
	view := func(x, y, z float64) (float64, float64, float64) {
		y -= 0.56
		xr, zr := x*ca+z*sa, -x*sa+z*ca
		yr, zr2 := y*ce-zr*se, y*se+zr*ce
		k := persp / (persp - zr2)
		return cx + xr*scale*k, cy - yr*scale*k, zr2
	}
	// Luz de cima, à esquerda e um pouco à frente. Fixa na câmera: é o brilho
	// caminhando pela peça que denuncia o giro.
	lx, ly, lz := -0.50, 0.74, 0.45
	ll := math.Sqrt(lx*lx + ly*ly + lz*lz)
	lx, ly, lz = lx/ll, ly/ll, lz/ll
	// Meio-caminho entre luz e câmera, pro especular.
	hx, hy, hz := lx, ly, lz+1
	hl := math.Sqrt(hx*hx + hy*hy + hz*hz)
	hx, hy, hz = hx/hl, hy/hl, hz/hl

	// shade devolve o nível a partir da normal em coordenada de mundo, já
	// girada pela câmera. Difusa + especular + luz de borda.
	shade := func(nx, ny, nz float64, refl bool) int {
		// A canelura entorta a normal pro lado, então ela chega aqui fora de
		// escala. Sem normalizar, o produto escalar estoura e a coroa inteira
		// vira especular.
		if l := math.Sqrt(nx*nx + ny*ny + nz*nz); l > 1e-9 {
			nx, ny, nz = nx/l, ny/l, nz/l
		}
		// A normal passa pela mesma rotação dos vértices, sem perspectiva.
		xr, zr := nx*ca+nz*sa, -nx*sa+nz*ca
		yr, zr2 := ny*ce-zr*se, ny*se+zr*ce
		dif := math.Max(0, xr*lx+yr*ly+zr2*lz)
		spc := math.Max(0, xr*hx+yr*hy+zr2*hz)
		spc = spc * spc * spc * spc
		spc = spc * spc // ^8: marfim é polido, mas não é espelho
		rim := 1 - math.Abs(zr2)
		rim = rim * rim * rim
		// A ambiente não é zero de propósito: a metade que está de costas pra
		// luz precisa continuar sendo marfim, não recorte preto.
		lum := 0.17 + 0.62*dif + 0.50*spc + 0.14*rim
		if refl {
			return relaxChRefl + minInt(maxInt(int(lum*float64(relaxChReflN)), 0), relaxChReflN-1)
		}
		if lum > 1.04 {
			return relaxChSpec
		}
		return relaxChIvory + minInt(maxInt(int(clamp01(lum)*float64(relaxChIvoryN)), 0), relaxChIvoryN-1)
	}

	// ── Corpo ── varredura do perfil em torno do eixo. Nada de trigonometria
	// no laço de dentro: seno e cosseno do ângulo saem uma vez por meridiano.
	for j := 0; j < relaxChNV; j++ {
		th := float64(j) / relaxChNV * 2 * math.Pi
		ct, stn := math.Cos(th), math.Sin(th)
		fl := 0.5 + 0.5*math.Cos(relaxChFlutes*th) // 0..1 ao longo da volta
		dfl := -0.5 * relaxChFlutes * math.Sin(relaxChFlutes*th)
		for i := 0; i < relaxChNU; i++ {
			p := relaxChessSamples[i]
			r, nr, ny := p.r, p.nr, p.ny
			nt := 0.0 // componente tangencial da normal, só onde há canelura
			if p.y > relaxChCrown0 && p.y < relaxChCrown1 && r > 0.01 {
				// Suaviza a entrada e a saída da faixa canelada, senão a coroa
				// ganha dois degraus retos onde ela começa e termina.
				k := 0.17 * math.Sin(math.Pi*(p.y-relaxChCrown0)/(relaxChCrown1-relaxChCrown0))
				r *= 1 - k*fl
				nt = -(-p.r * k * dfl) / r
			}
			x, y, z := r*ct, p.y, r*stn
			// Normal do sólido de revolução: a do perfil girada, mais a parcela
			// tangencial que a canelura introduz.
			nx := nr*ct - nt*stn
			nz := nr*stn + nt*ct
			sx, sy, sz := view(x, y, z)
			zb.put(int(sx), int(sy), sz, shade(nx, ny, nz, false))
			// Reflexo: a mesma peça espelhada no tampo, apagando com a
			// distância. É ele que põe a peça sobre alguma coisa.
			if f := 1 - y*3.0; f > 0.05 {
				rx, ry, rz := view(x, -y*0.92, z)
				if relaxHalftone(int(rx), int(ry)) < f*0.85 {
					zb.put(int(rx), int(ry), rz-4, shade(nx, -ny, nz, true))
				}
			}
		}
	}

	// ── Contas do colar ── esferas em volta do pescoço. Junto com a canelura,
	// são elas que contam o giro: some uma de um lado, nasce outra do outro.
	for k := 0; k < relaxChBeads; k++ {
		th := float64(k) / relaxChBeads * 2 * math.Pi
		bx, bz := relaxChBeadR*math.Cos(th), relaxChBeadR*math.Sin(th)
		const br = 0.038
		for a := 0; a < 26; a++ {
			pa := float64(a) / 26 * math.Pi
			sp, cp := math.Sin(pa), math.Cos(pa)
			for c := 0; c < 44; c++ {
				pc := float64(c) / 44 * 2 * math.Pi
				nx, ny, nz := sp*math.Cos(pc), cp, sp*math.Sin(pc)
				sx, sy, sz := view(bx+nx*br, relaxChBeadY+ny*br, bz+nz*br)
				zb.put(int(sx), int(sy), sz, shade(nx, ny, nz, false))
			}
		}
	}

	zb.blit(b)
}
