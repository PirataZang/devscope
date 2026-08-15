package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Camada Braille ────────────────────────────────────────────────────────────
//
// Cada célula do terminal vira 2×4 subpixels via bloco Braille (U+2800 + máscara
// de 8 pontos). A célula é ~1:2, então o subpixel Braille é quadrado — desenho
// aqui não precisa de correção de aspecto, diferente do relaxCanvas.
//
// Cada ponto entra com um nível de cor (0..n-1 da paleta da cena); a cor da
// célula é a média dos pontos acesos nela. Assim a densidade de pontos dá a
// forma e o nível dá a luz, que é o que faz a imagem parecer pixel art e não
// um bloco de texto.

// relaxBrailleDot[linha][coluna] = bit do ponto. A ordem é a herdada do Braille
// de 6 pontos (1-2-3 descendo, depois 7), não a sequencial.
var relaxBrailleDot = [4][2]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// relaxArtDots lê um desenho pronto em Braille e devolve o bitmap de subpontos
// (largura e altura em pontos, não em células). Trabalhar em ponto é o que
// deixa um desenho encolher pro tamanho do palco sem esfarelar: reduzir o
// bitmap e re-encodar preserva a silhueta, jogar coluna fora não.
func relaxArtDots(art []string) ([]bool, int, int) {
	cols := 0
	for _, l := range art {
		if n := len([]rune(l)); n > cols {
			cols = n
		}
	}
	dw, dh := cols*2, len(art)*4
	dots := make([]bool, dw*dh)
	for y, l := range art {
		for x, r := range []rune(l) {
			if r < 0x2800 || r > 0x28FF {
				continue
			}
			bits := byte(r - 0x2800)
			for row := 0; row < 4; row++ {
				for col := 0; col < 2; col++ {
					if bits&relaxBrailleDot[row][col] != 0 {
						dots[(y*4+row)*dw+x*2+col] = true
					}
				}
			}
		}
	}
	return dots, dw, dh
}

type relaxBraille struct {
	w, h  int
	bits  []byte
	sum   []int32
	cnt   []int32
	force []int16  // nível fixado na célula (-1 = usa a média)
	over  []string // glifo de texto que substitui a célula inteira

	// vote troca a média pela maioria. Numa rampa contínua (folhagem, luar) a
	// média é o que suaviza; numa paleta indexada ela é errada — a média entre
	// laranja e preto dá amarelo, que é a cor de outro adesivo.
	vote   bool
	dom    []int16
	domCnt []int16
}

func newRelaxBraille(w, h int) *relaxBraille {
	b := &relaxBraille{w: w, h: h, bits: make([]byte, w*h), sum: make([]int32, w*h),
		cnt: make([]int32, w*h), force: make([]int16, w*h), over: make([]string, w*h)}
	for i := range b.force {
		b.force[i] = -1
	}
	return b
}

// newRelaxBrailleVote é a variante de paleta indexada: a cor da célula é a do
// nível com mais pontos nela (voto de maioria de Boyer-Moore, O(1) por ponto).
func newRelaxBrailleVote(w, h int) *relaxBraille {
	b := newRelaxBraille(w, h)
	b.vote = true
	b.dom, b.domCnt = make([]int16, w*h), make([]int16, w*h)
	return b
}

// paint fixa a cor de uma célula inteira. Detalhe pequeno — olho, focinho —
// ocupa um ou dois pontos de oito, e a média da célula o devolveria como pelo.
func (b *relaxBraille) paint(cx, cy, lvl int) {
	if cx < 0 || cy < 0 || cx >= b.w || cy >= b.h {
		return
	}
	b.force[cy*b.w+cx] = int16(lvl)
}

// text troca a célula por um glifo comum. Braille não escreve "z" nem "♪".
func (b *relaxBraille) text(cx, cy int, glyph string, lvl int) {
	if cx < 0 || cy < 0 || cx >= b.w || cy >= b.h {
		return
	}
	i := cy*b.w + cx
	b.over[i], b.force[i] = glyph, int16(lvl)
}

// taken diz que a célula já tem dono: cor fixada por paint/lock. Fundo não
// entra nela — preencher os pontos vazios de uma célula de um objeto fino
// engorda a silhueta uma célula inteira, e aí o contorno vira escada.
func (b *relaxBraille) taken(px, py int) bool {
	if px < 0 || py < 0 || px >= b.w*2 || py >= b.h*4 {
		return true // fora do palco: b.set ignoraria de todo jeito
	}
	return b.force[(py/4)*b.w+px/2] >= 0
}

// lock congela a célula na cor que ela tem agora. Diferente de paint, que
// escolhe a cor: aqui a cor é a que os pontos já votaram. É o que salva traço
// fino — lâmina, haste — de virar céu quando o fundo enche a mesma célula.
func (b *relaxBraille) lock(cx, cy int) {
	if cx < 0 || cy < 0 || cx >= b.w || cy >= b.h {
		return
	}
	i := cy*b.w + cx
	if b.cnt[i] == 0 || b.force[i] >= 0 {
		return
	}
	if b.vote {
		b.force[i] = b.dom[i]
		return
	}
	b.force[i] = int16(b.sum[i] / b.cnt[i])
}

// set acende o subpixel (px, py) com o nível de cor lvl. Ponto já aceso não
// conta de novo, senão a média da célula pesaria o mesmo ponto duas vezes.
func (b *relaxBraille) set(px, py int, lvl int) {
	if px < 0 || py < 0 || px >= b.w*2 || py >= b.h*4 {
		return
	}
	i := (py/4)*b.w + px/2
	d := relaxBrailleDot[py%4][px%2]
	if b.bits[i]&d != 0 {
		return
	}
	b.bits[i] |= d
	b.sum[i] += int32(lvl)
	b.cnt[i]++
	if !b.vote {
		return
	}
	switch {
	case b.domCnt[i] == 0:
		b.dom[i], b.domCnt[i] = int16(lvl), 1
	case b.dom[i] == int16(lvl):
		b.domCnt[i]++
	default:
		b.domCnt[i]--
	}
}

// relaxStyleWrap extrai o par abre/fecha de um estilo uma vez só, renderizando
// um sentinela. A copa tem milhares de células e chamar Render por trecho custa
// mais que desenhar a árvore inteira; com o par em mãos, montar a linha vira
// concatenação de string.
func relaxStyleWrap(s lipgloss.Style) (string, string) {
	r := s.Render("\x00")
	i := strings.IndexByte(r, 0)
	if i < 0 {
		return "", ""
	}
	return r[:i], r[i+1:]
}

// lines monta as linhas agrupando células vizinhas de mesmo nível num trecho só.
func (b *relaxBraille) lines(palette []lipgloss.Style) []string {
	pre := make([]string, len(palette))
	post := make([]string, len(palette))
	for i, st := range palette {
		pre[i], post[i] = relaxStyleWrap(st)
	}
	out := make([]string, 0, b.h)
	var line, run strings.Builder
	for y := 0; y < b.h; y++ {
		line.Reset()
		run.Reset()
		level := -1
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if level >= 0 && level < len(palette) {
				line.WriteString(pre[level])
				line.WriteString(run.String())
				line.WriteString(post[level])
			} else {
				line.WriteString(run.String())
			}
			run.Reset()
		}
		for x := 0; x < b.w; x++ {
			i := y*b.w + x
			lvl := -1
			if b.cnt[i] > 0 {
				if b.vote {
					lvl = int(b.dom[i])
				} else {
					lvl = int(b.sum[i] / b.cnt[i])
				}
			}
			if b.force[i] >= 0 && (b.cnt[i] > 0 || b.over[i] != "") {
				lvl = int(b.force[i])
			}
			if lvl != level {
				flush()
				level = lvl
			}
			switch {
			case b.over[i] != "":
				run.WriteString(b.over[i])
			case lvl < 0:
				run.WriteByte(' ')
			default:
				run.WriteRune(rune(0x2800 + int(b.bits[i])))
			}
		}
		flush()
		out = append(out, line.String())
	}
	return out
}

// ── Pincel vetorial ───────────────────────────────────────────────────────
//
// Desenho por forma, e não por ponto: o pincel recebe coordenadas em qualquer
// escala e um "tom", e quem resolve o que tom significa é o dono do put — o
// mesmo desenho serve de símbolo aceso, de fantasma de borrão e de partícula.
//
// ORDEM IMPORTA em tudo que passa por aqui: o Braille não sobrescreve ponto
// aceso, então detalhe que fica por cima é desenhado ANTES do corpo.

type relaxPen struct {
	put  func(x, y float64, tone int)
	step float64 // um subponto, em unidades normalizadas
}

func (p relaxPen) dot(cx, cy, r float64, tone int) {
	for y := cy - r; y <= cy+r; y += p.step {
		for x := cx - r; x <= cx+r; x += p.step {
			if dx, dy := x-cx, y-cy; dx*dx+dy*dy <= r*r {
				p.put(x, y, tone)
			}
		}
	}
}

// disc é o dot com volume: luz de cima à esquerda. Fruta e moeda chapadas
// pareciam adesivo; o degrau de um tom já resolve numa forma deste tamanho.
func (p relaxPen) disc(cx, cy, r float64, tone int) {
	for y := cy - r; y <= cy+r; y += p.step {
		for x := cx - r; x <= cx+r; x += p.step {
			dx, dy := (x-cx)/r, (y-cy)/r
			d := dx*dx + dy*dy
			if d > 1 {
				continue
			}
			t := tone
			switch {
			case dx+dy < -0.45 && d < 0.72:
				t++
			case dx+dy > 0.80:
				t--
			}
			p.put(x, y, t)
		}
	}
}

func (p relaxPen) ellipse(cx, cy, rx, ry float64, tone int) {
	if rx <= 0 || ry <= 0 {
		return
	}
	for y := cy - ry; y <= cy+ry; y += p.step {
		for x := cx - rx; x <= cx+rx; x += p.step {
			dx, dy := (x-cx)/rx, (y-cy)/ry
			if dx*dx+dy*dy <= 1 {
				p.put(x, y, tone)
			}
		}
	}
}

func (p relaxPen) rect(x0, y0, x1, y1 float64, tone int) {
	for y := y0; y <= y1; y += p.step {
		for x := x0; x <= x1; x += p.step {
			p.put(x, y, tone)
		}
	}
}

func (p relaxPen) stroke(x0, y0, x1, y1, th float64, tone int) {
	n := maxInt(2, int(math.Hypot(x1-x0, y1-y0)/p.step))
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		p.dot(lerp(x0, x1, f), lerp(y0, y1, f), th/2, tone)
	}
}

func (p relaxPen) arc(cx, cy, r, a0, a1, th float64, tone int) {
	n := maxInt(4, int(math.Abs(a1-a0)*r/p.step))
	for i := 0; i <= n; i++ {
		a := lerp(a0, a1, float64(i)/float64(n))
		p.dot(cx+math.Cos(a)*r, cy+math.Sin(a)*r, th/2, tone)
	}
}

// quadFill preenche um quadrilátero já girado, por varredura simples: são
// dezenas de cartas por quadro, e o preenchimento por linha custa menos que
// duas chamadas de triângulo do buffer.
func (p relaxPen) quadFill(q [4][2]float64, tone int) {
	minY, maxY := q[0][1], q[0][1]
	minX, maxX := q[0][0], q[0][0]
	for _, v := range q {
		minY, maxY = math.Min(minY, v[1]), math.Max(maxY, v[1])
		minX, maxX = math.Min(minX, v[0]), math.Max(maxX, v[0])
	}
	inside := func(x, y float64) bool {
		sign := 0
		for i := 0; i < 4; i++ {
			a, bb := q[i], q[(i+1)%4]
			d := (bb[0]-a[0])*(y-a[1]) - (bb[1]-a[1])*(x-a[0])
			s := 1
			if d < 0 {
				s = -1
			}
			if sign == 0 {
				sign = s
			} else if s != sign {
				return false
			}
		}
		return true
	}
	for y := minY; y <= maxY; y += p.step {
		for x := minX; x <= maxX; x += p.step {
			if inside(x, y) {
				p.put(x, y, tone)
			}
		}
	}
}

// ── Primitivas ────────────────────────────────────────────────────────────────

// tri preenche um triângulo por teste baricentral. Quad e polígono convexo saem
// daqui; é o suficiente pra projetar faces 3D.
func (b *relaxBraille) tri(x0, y0, x1, y1, x2, y2 float64, lvl int) {
	side := func(ax, ay, bx, by, px, py float64) float64 {
		return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
	}
	loX, hiX := int(math.Min(x0, math.Min(x1, x2))), int(math.Max(x0, math.Max(x1, x2)))+1
	loY, hiY := int(math.Min(y0, math.Min(y1, y2))), int(math.Max(y0, math.Max(y1, y2)))+1
	for y := loY; y <= hiY; y++ {
		for x := loX; x <= hiX; x++ {
			px, py := float64(x), float64(y)
			d0 := side(x0, y0, x1, y1, px, py)
			d1 := side(x1, y1, x2, y2, px, py)
			d2 := side(x2, y2, x0, y0, px, py)
			if (d0 >= 0 && d1 >= 0 && d2 >= 0) || (d0 <= 0 && d1 <= 0 && d2 <= 0) {
				b.set(x, y, lvl)
			}
		}
	}
}

// quad preenche um quadrilátero convexo (dois triângulos). Os cantos têm de
// vir em ordem, horária ou anti-horária.
func (b *relaxBraille) quad(p [4][2]float64, lvl int) {
	b.tri(p[0][0], p[0][1], p[1][0], p[1][1], p[2][0], p[2][1], lvl)
	b.tri(p[0][0], p[0][1], p[2][0], p[2][1], p[3][0], p[3][1], lvl)
}

func (b *relaxBraille) line(x0, y0, x1, y1 float64, lvl int) {
	n := int(math.Hypot(x1-x0, y1-y0)) + 1
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		b.set(int(math.Round(lerp(x0, x1, f))), int(math.Round(lerp(y0, y1, f))), lvl)
	}
}

// ── Meio-tom ──────────────────────────────────────────────────────────────────

// relaxBayer8 é o limiar ordenado 8×8, normalizado. Sozinho ele desenha trama
// regular, por isso relaxHalftone mistura com ruído fixo por posição.
var relaxBayer8 = func() [64]float64 {
	var m [64]float64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v, mask := 0, 4
			for i := 0; i < 3; i++ {
				bx, by := (x/mask)&1, (y/mask)&1
				v = v<<2 | (bx^by)<<1 | bx
				mask >>= 1
			}
			m[y*8+x] = (float64(v) + 0.5) / 64
		}
	}
	return m
}()

// relaxHalftone devolve o limiar do subpixel (x, y): estável entre frames, que
// é o que impede a imagem de cintilar quando a luz muda devagar.
//
// Hash próprio, não relaxHash: este é chamado uma vez por subpixel — dezenas de
// milhares de vezes por frame — e a versão variádica com laço aparecia no perfil.
func relaxHalftone(x, y int) float64 {
	h := (x*374761393 + y*668265263) ^ 0x5bf03635
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return 0.42*relaxBayer8[(y&7)*8+x&7] + 0.58*float64(h&1023)/1024
}

// ── Paleta ────────────────────────────────────────────────────────────────────

// relaxColor é só um apelido pra lipgloss.Color, pra as paletas das cenas não
// arrastarem o import do lipgloss.
type relaxColor = lipgloss.Color

// relaxRamp interpola uma lista de paradas hex em n degraus.
func relaxRamp(stops []string, n int) []lipgloss.Color {
	out := make([]lipgloss.Color, n)
	for i := 0; i < n; i++ {
		p := float64(i) / float64(maxInt(1, n-1)) * float64(len(stops)-1)
		k := minInt(int(p), len(stops)-2)
		f := p - float64(k)
		r0, g0, b0, _ := relaxHexRGB(stops[k])
		r1, g1, b1, _ := relaxHexRGB(stops[k+1])
		out[i] = lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
			int(lerp(float64(r0), float64(r1), f)+0.5),
			int(lerp(float64(g0), float64(g1), f)+0.5),
			int(lerp(float64(b0), float64(b1), f)+0.5)))
	}
	return out
}

// relaxStyles aplica o fade da cena numa paleta, uma vez por frame.
func relaxStyles(colors []lipgloss.Color, fade float64) []lipgloss.Style {
	out := make([]lipgloss.Style, len(colors))
	for i, c := range colors {
		out[i] = relaxDim(lipgloss.NewStyle().Foreground(c), fade)
	}
	return out
}
