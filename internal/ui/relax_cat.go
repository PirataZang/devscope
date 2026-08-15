package ui

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Cat ───────────────────────────────────────────────────────────────────────
//
// Gato enrodilhado dormindo, laranja manchado de branco. Respira o tempo todo;
// a cada ~4–6s a ponta do rabo dá uma corrida e para.
//
// O desenho é line art em Braille embutida (relaxCatArt), não geometria. Colorir
// traço pronto sai mais fiel — e em muito menos código — do que tentar remontar
// a silhueta com elipses: o traço vira contorno escuro, o miolo vira pelo com
// gradiente, e as manchas brancas entram como elipses recortadas pela silhueta.

// relaxCatArt: 65×21 células = 130×84 subpixels.
const relaxCatArt = `⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣤⣴⣶⣶⠿⠿⠿⠿⠿⠿⠿⢷⣶⣶⣤⣄⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣤⣶⡾⠿⠛⠉⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠙⠻⠿⣷⣦⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣤⣾⠿⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⢦⡀⠀⠀⠀⠀⠀⠀⠉⠛⢿⣶⣤⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣴⣿⠏⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⣄⠀⠀⠀⠀⠀⠀⠀⠀⠈⠛⢿⣦⡀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣴⣿⣿⣿⣀⣀⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⢿⣦⡀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣤⣤⣶⣶⣶⣶⣶⡿⠟⠋⠚⢋⣭⣭⣽⠿⠷⠶⢶⣦⣤⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⣿⣦⡀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⣠⣴⡾⠟⠋⠉⠀⠀⠀⠀⠀⠁⠀⠀⠀⣀⣀⡉⠻⣄⠀⠀⠀⠀⠀⠉⠙⠻⠶⣦⣤⣀⣀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⢿⣷⡀⠀⠀
⢀⣀⣠⣤⣶⠿⠛⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠻⣿⢿⠄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠙⠛⢻⣷⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⢿⣷⡀⠀
⠻⣿⡉⠁⢀⣠⣤⣀⠀⠀⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣀⣠⡴⠋⠉⠉⢀⣼⠟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⣧⠀
⠀⠸⣿⣆⠀⠀⠀⠈⠙⣿⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⠄⠀⠀⠀⠀⠀⠹⣿⠁⠀⠀⠀⣠⡾⠋⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣿⡆
⠀⠀⠈⠻⣦⣀⠀⠀⢸⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠸⠁⠀⠀⠀⠀⠀⠀⠀⠹⣇⣀⣴⡾⠋⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⣗
⠀⠀⠀⠀⠈⠛⠷⣶⣾⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿
⠀⠀⠀⠀⠀⠀⠀⠀⣿⠀⠠⣶⠾⠶⠶⣶⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣴⡶⠟⠻⠷⣦⠀⢹⡆⠀⠀⠀⠀⠀⠀⠐⣦⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡀⠀⠀⠀⠀⠀⠀⢸⣿
⠀⠀⠀⠀⠀⠀⠀⢰⡟⠀⠀⠀⠀⠀⠀⠀⠙⣷⠀⠀⢀⣀⣀⡀⠀⢠⡿⠁⠀⠀⠀⠀⠀⠀⠸⣧⠀⠀⠀⠀⠀⠀⠀⠸⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡼⠁⠀⠀⠀⠀⠀⠀⢸⣿
⠀⠀⠀⠀⠀⠀⢰⡟⠁⠀⠀⠀⠀⠀⠀⠀⠀⠘⠀⠀⠀⢀⣀⣀⣄⠈⠁⠀⠀⠀⠀⠀⠀⠀⠀⣻⡧⠀⠀⠀⠀⠀⠀⠀⣿⠀⠀⠀⠀⠀⠀⢀⣤⠾⠋⠀⠀⠀⠀⠀⠀⠀⠀⣼⡇
⠀⠀⠀⠀⠀⠀⠈⢿⣗⠀⠀⠀⠀⠀⠀⣀⣀⣾⣀⠸⣯⣍⣉⣉⡿⠀⣘⣦⣀⣀⣀⡀⠀⢠⣾⠏⠀⠀⠀⠀⠀⠀⠀⠀⣿⠀⠀⢀⣠⣴⠾⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⡿⠀
⠀⠀⠀⠀⠀⠀⠀⢀⣽⠿⣦⣤⣐⠊⡩⠟⣻⣿⣎⣀⠀⣉⣿⡉⠀⢀⣴⢿⣙⠒⢦⣤⣴⠟⠁⠀⠀⠀⠀⠀⣀⣀⣠⣼⣿⠶⠟⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⡿⠁⠀
⠀⠀⠀⠀⠀⢀⣼⠟⡽⠀⣠⣭⡿⠿⠟⠛⠛⠛⠛⠛⠛⣿⠛⠛⠿⠿⠷⠶⠿⠿⠿⠿⠷⠶⠶⠶⠾⠟⠛⠛⠛⠉⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⣾⠟⠁⠀⠀
⠀⠀⠀⠀⠀⣾⣟⢸⣧⣾⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⣷⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣴⡾⠟⠁⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠙⠿⢾⣿⣿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣸⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣀⣤⣴⡶⠿⠛⠉⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⢿⣦⣤⣤⣤⣶⡶⠶⠶⠿⠿⠿⠿⠿⠿⠿⠷⠶⠶⠶⢶⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶⡶⠶⠿⠿⠛⠛⠋⠉⢀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀`

type relaxCatPhase int

const catSleeping relaxCatPhase = 0

type relaxZzz struct {
	x10, y10 int
	rise     int
	drift    int
	age, ttl int
	glyph    string
}

type relaxCatState struct {
	inited bool
	tick   int
	phase  relaxCatPhase

	breath     float64
	breathRate float64

	zzz   []relaxZzz
	nextZ int

	tailFlick int // frames restantes da mexida
	tailDur   int
	nextTail  int
}

func relaxCatBreathRate() float64 {
	return 2 * math.Pi / (38.0 + float64(rand.Intn(9)))
}

// Os Zzz sobem pelo canto vazio acima da cabeça — o resto do quadro é gato.
func relaxCatSpawnZ() relaxZzz {
	glyph := "z"
	if rand.Intn(3) == 0 {
		glyph = "Z"
	}
	return relaxZzz{
		x10:   250 + rand.Intn(200),
		y10:   100 + rand.Intn(60),
		rise:  3 + rand.Intn(3),
		drift: rand.Intn(3),
		ttl:   22 + rand.Intn(14),
		glyph: glyph,
	}
}

func stepRelaxCat(st *relaxCatState) {
	if !st.inited {
		st.inited = true
		st.phase = catSleeping
		st.breathRate = relaxCatBreathRate()
		st.nextZ = 5 + rand.Intn(12)
		st.nextTail = 42 + rand.Intn(19)
	}
	st.tick++
	if st.breath += st.breathRate; st.breath > 2*math.Pi {
		st.breath -= 2 * math.Pi
	}

	if st.tailFlick > 0 {
		st.tailFlick--
		if st.tailFlick == 0 {
			st.nextTail = 42 + rand.Intn(19)
		}
	} else if st.nextTail--; st.nextTail <= 0 {
		st.tailDur = 18 + rand.Intn(7)
		st.tailFlick = st.tailDur
	}

	if st.nextZ--; st.nextZ <= 0 {
		st.zzz = append(st.zzz, relaxCatSpawnZ())
		st.nextZ = 8 + rand.Intn(13)
	}

	live := st.zzz[:0]
	for _, z := range st.zzz {
		z.age++
		z.y10 -= z.rise
		z.x10 += z.drift
		if z.age < z.ttl && z.y10 > 0 {
			live = append(live, z)
		}
	}
	st.zzz = live
}

// ── Paleta ────────────────────────────────────────────────────────────────────

var relaxCatFurStops = []string{"#6B2E0A", "#B84810", "#E06A18", "#F08C30", "#F6B060"}

const relaxCatFurLevels = 10

const (
	relaxCatPink = relaxCatFurLevels + iota
	relaxCatPinkDim
	relaxCatWhite
	relaxCatWhiteDim
	relaxCatZzz
)

var relaxCatRamp = func() []lipgloss.Color {
	out := make([]lipgloss.Color, relaxCatZzz+1)
	copy(out, relaxRamp(relaxCatFurStops, relaxCatFurLevels))
	out[relaxCatPink] = lipgloss.Color("#E79AA6")
	out[relaxCatPinkDim] = lipgloss.Color("#B46675")
	out[relaxCatWhite] = lipgloss.Color("#F3EEE6")
	out[relaxCatWhiteDim] = lipgloss.Color("#A79E90")
	out[relaxCatZzz] = lipgloss.Color("#7C88A0")
	return out
}()

// relaxCatShade escurece uma cor sem trocar de rampa: é assim que o traço do
// desenho vira contorno. Média entre branco e laranja daria rosa na borda, então
// cada rampa tem o seu tom escuro em vez de um preto comum.
func relaxCatShade(lvl int) int {
	switch lvl {
	case relaxCatWhite:
		return relaxCatWhiteDim
	case relaxCatPink:
		return relaxCatPinkDim
	}
	if lvl < relaxCatFurLevels {
		return maxInt(0, lvl-6)
	}
	return lvl
}

// ── Modelo ────────────────────────────────────────────────────────────────────

const (
	relaxCatOut = iota
	relaxCatFur
	relaxCatLine
)

type relaxCatSprite struct {
	w, h int
	cls  []uint8
}

// relaxCatModel decodifica a arte uma vez: ponto aceso é traço, e o que o
// alagamento a partir da borda não alcança é miolo. É o alagamento que dá o
// gato preenchido sem eu ter de descrever região nenhuma — o desenho já sabe
// onde ele acaba.
var relaxCatModel = func() *relaxCatSprite {
	lines := strings.Split(strings.Trim(relaxCatArt, "\n"), "\n")
	w := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > w {
			w = n
		}
	}
	s := &relaxCatSprite{w: w * 2, h: len(lines) * 4}
	s.cls = make([]uint8, s.w*s.h)
	for cy, l := range lines {
		for cx, r := range []rune(l) {
			m := byte(r - 0x2800)
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					if m&relaxBrailleDot[dy][dx] != 0 {
						s.cls[(cy*4+dy)*s.w+cx*2+dx] = relaxCatLine
					}
				}
			}
		}
	}
	seen := make([]bool, len(s.cls))
	stack := make([][2]int, 0, len(s.cls))
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= s.w || y >= s.h {
			return
		}
		i := y*s.w + x
		if seen[i] || s.cls[i] == relaxCatLine {
			return
		}
		seen[i] = true
		stack = append(stack, [2]int{x, y})
	}
	for x := 0; x < s.w; x++ {
		push(x, 0)
		push(x, s.h-1)
	}
	for y := 0; y < s.h; y++ {
		push(0, y)
		push(s.w-1, y)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		push(p[0]+1, p[1])
		push(p[0]-1, p[1])
		push(p[0], p[1]+1)
		push(p[0], p[1]-1)
	}
	for i := range s.cls {
		if s.cls[i] != relaxCatLine && !seen[i] {
			s.cls[i] = relaxCatFur
		}
	}
	return s
}()

// Regiões em coordenadas da arte (130×84), na ordem em que se sobrepõem: a
// última que cobrir o ponto ganha. É daqui que sai o "dá pra ver que é gato" —
// cabeça mais clara que o dorso, focinho e peito brancos, orelha rosa por
// dentro. Nível abaixo de relaxCatFurLevels é pelo e ganha o degradê da elipse;
// os outros são cor chapada.
var relaxCatPatches = []struct {
	x, y, rx, ry float64
	lvl          int
	tail         bool // acompanha a mexida da cauda
}{
	{41, 61, 29, 19, 9, false},              // cabeça: o tom mais claro do bicho
	{12, 42, 5.5, 5.5, relaxCatPink, false}, // orelha esquerda por dentro
	{72, 41, 5.5, 5.5, relaxCatPink, false}, // orelha direita por dentro
	{43, 66, 16, 9, relaxCatWhite, false},   // focinho e queixo
	{26, 74, 19, 9, relaxCatWhite, false},   // peito e pata da frente
	{62, 76.5, 9, 3.4, relaxCatWhite, true}, // ponta da cauda, na faixa abaixo da costura
	{43, 58, 4.2, 2.8, relaxCatPink, false}, // nariz
}

// relaxCatLevelAt devolve a cor do ponto (x, y) da arte.
func relaxCatLevelAt(x, y, swing float64) int {
	lvl := int(6.4 - 2.2*y/84)
	for _, p := range relaxCatPatches {
		px, py := p.x, p.y
		if p.tail {
			px += swing * relaxCatTailRunX
			py += swing * relaxCatTailRunY
		}
		nx, ny := (x-px)/p.rx, (y-py)/p.ry
		if nx*nx+ny*ny > 1 {
			continue
		}
		lvl = p.lvl
		if p.lvl < relaxCatFurLevels {
			// Pelo pega volume; branco e rosa ficam chapados, senão o focinho
			// vira degradê cinza e some contra o pelo.
			lvl = clampInt(p.lvl-int(1.6*ny+0.5), 0, relaxCatFurLevels-1)
		}
	}
	return lvl
}

// relaxCatTailSeam devolve a distância até a curva que solta a cauda do corpo:
// uma linha só, saindo do fim da pata dianteira e subindo até a ponta da cauda.
// A curva é fixa: o corpo não mexe, só a ponta.
//
// Antes eu recortava a cauda inteira como uma cunha de três retas. Carvar a
// borda de uma cunha desenha o CONTORNO dela — no meio do gato aparecia um
// triângulo. Uma curva é o que separa duas partes; região é o que não separa.
//
// A distância é vertical, corrigida pela inclinação, senão o traço engrossa
// justo onde a curva sobe.
func relaxCatTailSeam(x, y float64) float64 {
	const x0, span = 45.0, 43.0
	u := x - x0
	if u < 0 || u > span {
		return 9
	}
	cy := 74.6 - 0.030*u - 0.006117*u*u
	slope := -0.030 - 0.012234*u
	return math.Abs(y-cy) / math.Sqrt(1+slope*slope)
}

// Meia largura da curva, em pontos da arte, e quanto a ponta da cauda corre na
// mexida. A cauda é o que fica ABAIXO da curva — a faixa que contorna a frente
// do corpo e vai morrer junto da pata. Só a mancha branca da ponta se mexe, e
// no eixo da faixa (pra direita, subindo junto com a costura): atravessar a
// costura punha branco do lado do corpo, que é onde a cauda não está.
const (
	relaxCatTailSeamW = 0.95
	relaxCatTailRunX  = 3.0
	relaxCatTailRunY  = -0.8
)

// relaxCatFlank pesa o respiro: 1 no lombo, 0 na cabeça e embaixo da barriga.
// Só o flanco sobe e desce — inflar o gato inteiro fazia a cabeça tremer junto,
// que lê como falha de render, não como respiração. A queda é suave nas duas
// bordas porque degrau seco rasgaria o bicho na fronteira da região.
func relaxCatFlank(x, y float64) float64 {
	return clamp01((x-38)/16) * clamp01((52-y)/16)
}

// Quanto o lombo sobe no pico da inspiração, em pontos da arte.
const relaxCatBreathLift = 2.8

func (p relaxCatPhase) status() string {
	return "dormindo…"
}

func relaxCatFrames(st *relaxCatState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxCat(st)
	}
	w := maxInt(26, minInt(width, 80))
	h := maxInt(7, minInt(height, 22))
	// Paleta indexada (pelo + branco + tinta do contorno): a média de níveis
	// entre mancha branca e laranja daria rosa solto na borda — voto de maioria
	// mantém cada cor pura.
	b := newRelaxBrailleVote(w, h)
	relaxCatDraw(st, b, w, h)
	return b.lines(relaxStyles(relaxCatRamp, fade)), StyleMuted.Render(st.phase.status())
}

func relaxCatDraw(st *relaxCatState, b *relaxBraille, w, h int) {
	m := relaxCatModel
	sw, sh := float64(w*2), float64(h*4)
	breath := math.Sin(st.breath)
	swing := 0.0
	if st.tailFlick > 0 && st.tailDur > 0 {
		swing = math.Sin((1 - float64(st.tailFlick)/float64(st.tailDur)) * 2 * math.Pi)
	}

	// A folga de relaxCatBreathLift em cima é do respiro: sem ela o lombo sobe
	// contra a borda do palco e o quadro apara o dorso em vez de inflar.
	sc := math.Min(sw/float64(m.w), sh/(float64(m.h)+relaxCatBreathLift))
	ox := (sw - float64(m.w)*sc) / 2
	oy := (sh-(float64(m.h)+relaxCatBreathLift)*sc)/2 + relaxCatBreathLift*sc
	inv := 1 / sc

	// Caixa de origem por subpixel de destino: com a arte reduzida o traço tem
	// menos de um ponto de largura e sumiria na amostragem por ponto. Aqui ele
	// vira contorno se cobrir um terço da caixa, e o resto da caixa decide se o
	// ponto é gato ou fundo.
	for dy := 0; dy < int(sh); dy++ {
		for dx := 0; dx < int(sw); dx++ {
			x0, x1 := (float64(dx)-ox)*inv, (float64(dx)+1-ox)*inv
			y0, y1 := (float64(dy)-oy)*inv, (float64(dy)+1-oy)*inv
			// Respiro: o flanco desliza sobre a origem. Amostrar mais embaixo
			// puxa o desenho pra cima, então o lombo infla sem esticar nada.
			cx, cy := (x0+x1)/2, (y0+y1)/2
			if lift := breath * relaxCatBreathLift * relaxCatFlank(cx, cy); lift != 0 {
				y0 += lift
				y1 += lift
				cy += lift
			}
			var line, fur, n int
			for sy := int(y0); sy <= int(y1); sy++ {
				if sy < 0 || sy >= m.h {
					continue
				}
				for sx := int(x0); sx <= int(x1); sx++ {
					if sx < 0 || sx >= m.w {
						continue
					}
					n++
					switch m.cls[sy*m.w+sx] {
					case relaxCatLine:
						line++
					case relaxCatFur:
						fur++
					}
				}
			}
			if n == 0 || (line+fur)*2 < n {
				continue
			}
			// A curva que solta a cauda do corpo: apagada, então o fundo passa
			// por ela e a cauda lê como peça à parte.
			if relaxCatTailSeam(cx, cy) < relaxCatTailSeamW {
				continue
			}
			lvl := relaxCatLevelAt(cx, cy, swing)
			if line*3 >= n {
				lvl = relaxCatShade(lvl)
			}
			b.set(dx, dy, lvl)
		}
	}

	for _, z := range st.zzz {
		cx := int(ox+float64(z.x10)/10*sc) / 2
		cy := int(oy+float64(z.y10)/10*sc) / 4
		b.text(cx, cy, z.glyph, relaxCatZzz)
	}
}
