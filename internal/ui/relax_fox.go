package ui

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Raposa e vaga-lumes ───────────────────────────────────────────────────────
//
// A raposa é sprite fixo em Braille: forma desenhada à mão lê melhor que
// qualquer silhueta que eu monte com elipse. Ela NÃO se mexe.
//
// O que as outras cenas fazem com geometria, aqui é feito com cor: cada célula
// do sprite recebe um nível da rampa de pelo pela posição (luz vindo de cima e
// da esquerda) mais um meio-tom, então o corpo tem volume em vez de ser um
// recorte laranja chapado. Ponta do rabo e concha da orelha puxam a rampa
// clara. Só os vaga-lumes e o piscar das estrelas andam.

var relaxFoxArt = []string{
	"⠀⠀⠀⣷⣆⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣀",
	"⠀⠀⠀⣿⣿⣿⣧⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⣤⣾⣿⠉",
	"⠀⠀⠛⣿⣿⣿⣿⣿⣷⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣶⣿⣿⣿⣿⣿",
	"⠀⠀⠤⣿⣿⣿⣿⣿⣿⣿⣿⣤⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⣤⣿⣿⣿⣿⣿⣿⣿",
	"⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣶⣦⠀⠀⠀⠀⠀⠀⠀⠀⣴⣶⣿⣿⣿⣿⣿⣿⣿⣿⡿",
	"⠀⠀⠀⠉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿",
	"⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡏",
	"⠀⠀⠀⠀⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡏",
	"⠀⠀⠀⠀⠀⠘⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠇",
	"⠀⠀⠀⠀⠀⣠⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡆",
	"⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣇",
	"⠀⠀⠀⢈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿",
	"⠀⠀⢀⠿⠿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟⠛⠒",
	"⠀⠀⠀⠀⠀⠉⠙⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠋⠉⠁",
	"⠀⠀⠀⠀⠀⠀⠀⠀⢀⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣇⡀",
	"⠀⠀⠀⠀⠀⠀⠀⣶⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⡄",
	"⠀⠀⠀⠀⠀⠀⣼⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⡄",
	"⠀⠀⠀⠀⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⡄",
	"⠀⠀⠀⠀⠉⠉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡏⠉⠃",
	"⠀⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⡄",
	"⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠀⠀⠀⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣶⣶⠒",
	"⠀⠀⣉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣶⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣴⣶⣿⣿⡟",
	"⠀⣤⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⠀⣤⣿⣿⣿⣿⣿⡇",
	"⢀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⠀⢠⣼⣿⣿⣿⣿⣿⣿⣿⡇",
	"⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⠤⢀⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠁",
	"⣼⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡄⠀⣠⣾⣶⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿",
	"⠸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣾⣷⣶⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠇",
	"⠀⠛⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡏⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⠀⢀⣆⣀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠇",
	"⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠿⠁⠀⠀⠀⠀⠀⢀⡀⠀⣦⣴⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇",
	"⠀⠀⠉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠉⠀⣤⣶⣶⣶⣾⣿⣤⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠇",
	"⠀⠀⠀⠉⠛⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟⠋⠁",
	"⠀⠀⠀⠀⠀⠉⢹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠿⠁",
	"⠀⠀⠀⠀⠀⠀⠈⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠛⠉⠁",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⠛⠻⠿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠟⠛⠛",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠛⠛⠛⠛⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟⠛⠛⠋⠉",
}

// relaxFoxCell é uma célula do sprite já resolvida: glifo e a que parte do
// bicho ela pertence.
type relaxFoxCell struct {
	glyph string
	x, y  int
	tone  float64 // 0..1 na rampa de pelo
	white bool
}

const (
	relaxFxFurN = 12
	relaxFxWhtN = 5
	relaxFxFlyN = 6
)

var (
	relaxFxFurRamp = relaxRamp([]string{"#2A0E06", "#4E1A0A", "#7A2C10", "#A63E16", "#C8551E", "#E06E28", "#F08C3C", "#F8AC62"}, relaxFxFurN)
	relaxFxWhtRamp = relaxRamp([]string{"#6E6A62", "#9E9A90", "#C6C2B8", "#E4E0D6", "#F6F3EA"}, relaxFxWhtN)
	relaxFxFlyRamp = relaxRamp([]string{"#3A4A10", "#6A8418", "#9ECC30", "#C4E840", "#EAFC78", "#FBFFD0"}, relaxFxFlyN)
	relaxFxStarCol = lipgloss.Color("#7C88A0")
)

// relaxFxDots é a ordem dos pontos dentro da célula Braille: bit i acende o
// ponto relaxFxDots[i] (coluna, linha).
var relaxFxDots = [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}

// O desenho vira bitmap de pontos uma vez só. Trabalhar em ponto, e não em
// caractere, é o que deixa a raposa encolher: reduzir o bitmap e re-encodar
// preserva a silhueta, enquanto jogar colunas fora só a esfarela.
var relaxFoxDots, relaxFoxDotW, relaxFoxDotH = func() ([]bool, int, int) {
	rows := make([][]rune, len(relaxFoxArt))
	cols := 0
	for i, l := range relaxFoxArt {
		rows[i] = []rune(l)
		if n := len(rows[i]); n > cols {
			cols = n
		}
	}
	dw, dh := cols*2, len(rows)*4
	dots := make([]bool, dw*dh)
	for y, row := range rows {
		for x, r := range row {
			if r < 0x2800 || r > 0x28FF {
				continue
			}
			bits := byte(r - 0x2800)
			for i, d := range relaxFxDots {
				if bits&(1<<uint(i)) != 0 {
					dots[(y*4+d[1])*dw+x*2+d[0]] = true
				}
			}
		}
	}
	return dots, dw, dh
}()

type relaxFoxSprite struct {
	w, h  int
	scale float64 // 1 = desenho inteiro; menor = terminal apertado
	cells []relaxFoxCell
}

// Terminal não muda de tamanho a cada frame, então o sprite de cada tamanho é
// resolvido uma vez e fica guardado.
var relaxFoxCache = map[[2]int]*relaxFoxSprite{}

// relaxFoxAt devolve a raposa no maior tamanho que cabe em width×height,
// sempre na proporção original — esticar um bicho é pior que encolher.
func relaxFoxAt(width, height int) *relaxFoxSprite {
	key := [2]int{width, height}
	if sp, ok := relaxFoxCache[key]; ok {
		return sp
	}
	scale := 1.0
	if width > 0 {
		scale = math.Min(scale, float64(width*2)/float64(relaxFoxDotW))
	}
	if height > 0 {
		scale = math.Min(scale, float64(height*4)/float64(relaxFoxDotH))
	}
	dw := maxInt(2, int(float64(relaxFoxDotW)*scale))
	dh := maxInt(4, int(float64(relaxFoxDotH)*scale))
	w, h := (dw+1)/2, (dh+3)/4

	// Cada ponto do destino olha o retângulo de pontos que lhe corresponde na
	// origem e acende se boa parte dele estiver acesa. O limiar abaixo de meio
	// é de propósito: segura os traços finos (patas, orelhas) que a média
	// apagaria.
	dots := make([]bool, w*2*h*4)
	for y := 0; y < dh; y++ {
		sy0 := y * relaxFoxDotH / dh
		sy1 := maxInt(sy0+1, (y+1)*relaxFoxDotH/dh)
		for x := 0; x < dw; x++ {
			sx0 := x * relaxFoxDotW / dw
			sx1 := maxInt(sx0+1, (x+1)*relaxFoxDotW/dw)
			on, total := 0, 0
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					total++
					if relaxFoxDots[sy*relaxFoxDotW+sx] {
						on++
					}
				}
			}
			if float64(on) >= 0.38*float64(total) {
				dots[y*w*2+x] = true
			}
		}
	}

	cells := make([]relaxFoxCell, 0, w*h/2)
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			bits := byte(0)
			for i, d := range relaxFxDots {
				if dots[(cy*4+d[1])*w*2+cx*2+d[0]] {
					bits |= 1 << uint(i)
				}
			}
			if bits == 0 {
				continue
			}
			fx, fy := float64(cx)/float64(maxInt(1, w-1)), float64(cy)/float64(maxInt(1, h-1))
			// Luz de cima e da esquerda; a barriga e o chão caem pra sombra.
			tone := 0.86 - 0.42*fy - 0.16*fx
			// O rabo (canto direito, metade de baixo) volta a subir: ele pega
			// luz por cima e é a parte mais clara do bicho depois da ponta.
			if fx > 0.55 && fy > 0.55 {
				tone += 0.30
			}
			white := fx > 0.80 && fy > 0.58 && fy < 0.92
			// Orelhas: as duas pontas lá em cima, com a concha clara.
			if fy < 0.10 && (fx < 0.12 || (fx > 0.35 && fx < 0.52)) {
				white = true
			}
			cells = append(cells, relaxFoxCell{
				glyph: string(rune(0x2800 + int(bits))),
				x:     cx,
				y:     cy,
				tone:  clamp01(tone),
				white: white,
			})
		}
	}
	sp := &relaxFoxSprite{w: w, h: h, scale: scale, cells: cells}
	relaxFoxCache[key] = sp
	return sp
}

// A cabeça no sprite (0–1), só pro status saber quando um vaga-lume passa perto.
const (
	relaxFxHeadX = 0.24
	relaxFxHeadY = 0.10
)

type relaxFxBug struct {
	x, y   float64
	vx, vy float64
	ph     float64
	blink  float64
	rate   float64
	glow   float64
}

type relaxFoxState struct {
	inited bool
	tick   int
	flies  []relaxFxBug
	stars  []relaxSkyPt
	near   float64
}

func relaxFxNewFly() relaxFxBug {
	return relaxFxBug{
		x:  rand.Float64(),
		y:  0.06 + rand.Float64()*0.80,
		vx: (rand.Float64() - 0.5) * 0.0024,
		vy: (rand.Float64() - 0.5) * 0.0016,
		ph: rand.Float64() * 6.28,
		// Cada um com seu compasso: piscar junto é árvore de natal, não
		// vaga-lume.
		blink: rand.Float64() * 6.28,
		rate:  0.055 + rand.Float64()*0.075,
	}
}

func stepRelaxFox(st *relaxFoxState) {
	if !st.inited {
		st.inited = true
		for i, n := 0, 12+rand.Intn(6); i < n; i++ {
			st.flies = append(st.flies, relaxFxNewFly())
		}
		for i, n := 0, 30+rand.Intn(18); i < n; i++ {
			st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.55})
		}
	}
	st.tick++

	for i := range st.flies {
		f := &st.flies[i]
		f.ph += 0.035
		f.blink += f.rate
		// Acende e apaga com pausa apagado no meio — daí o max(0, sin).
		f.glow = math.Max(0, math.Sin(f.blink))
		f.x += f.vx + math.Sin(f.ph*1.7)*0.0016
		f.y += f.vy + math.Cos(f.ph*1.3)*0.0011
		if f.x < -0.04 || f.x > 1.04 || f.y < 0.02 || f.y > 0.96 {
			*f = relaxFxNewFly()
			f.x = math.Mod(f.x+0.5, 1)
		}
	}

	st.near = 9
	for _, f := range st.flies {
		if f.glow < 0.30 {
			continue
		}
		// y pesa mais que x: a célula do terminal é o dobro de alta.
		if d := math.Hypot(f.x-relaxFxHeadX, (f.y-relaxFxHeadY)*1.9); d < st.near {
			st.near = d
		}
	}
}

// relaxFxGlow escolhe o glifo pelo brilho: ponto pequeno quase apagado, célula
// cheia no auge. É o que faz o vaga-lume acender em vez de piscar ligado.
func relaxFxGlow(v float64) string {
	switch {
	case v > 0.80:
		return "⣿"
	case v > 0.58:
		return "⣶"
	case v > 0.38:
		return "⠿"
	case v > 0.20:
		return "⠒"
	default:
		return "⠄"
	}
}

func relaxFoxFrames(st *relaxFoxState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxFox(st)
	}
	sp := relaxFoxAt(width, height)
	w, h := sp.w, sp.h
	glyph := make([]string, w*h)
	style := make([]lipgloss.Style, w*h)
	body := make([]bool, w*h)

	fur := make([]lipgloss.Style, relaxFxFurN)
	for i, c := range relaxFxFurRamp {
		fur[i] = relaxDim(lipgloss.NewStyle().Foreground(c), fade)
	}
	wht := make([]lipgloss.Style, relaxFxWhtN)
	for i, c := range relaxFxWhtRamp {
		wht[i] = relaxDim(lipgloss.NewStyle().Foreground(c), fade)
	}
	fly := make([]lipgloss.Style, relaxFxFlyN)
	for i, c := range relaxFxFlyRamp {
		fly[i] = relaxDim(lipgloss.NewStyle().Foreground(c), fade)
	}
	star := relaxDim(lipgloss.NewStyle().Foreground(relaxFxStarCol), fade)

	// Céu menor pede menos gente nele: a conta mantém a densidade em vez do
	// número, senão o terminal apertado vira sopa de pontos.
	dens := clamp01(0.30 + 0.70*sp.scale)

	// ── Estrelas ── só no céu vazio, e piscando devagar.
	t := float64(st.tick) * 0.1
	for i, p := range st.stars {
		if float64(i) > float64(len(st.stars))*dens {
			break
		}
		x, y := int(p.x*float64(w-1)), int(p.y*float64(h-1))
		if relaxHalftone(x, y) > 0.30+0.16*math.Sin(t*0.4+float64(i)) {
			continue
		}
		if i := y*w + x; glyph[i] == "" {
			glyph[i], style[i] = "⠂", star
		}
	}

	// ── Raposa ── o sprite, com o tom já resolvido. O meio-tom por célula tira
	// o ar de recorte chapado sem mexer na silhueta.
	for _, c := range sp.cells {
		i := c.y*w + c.x
		lvl := c.tone + 0.06*(relaxHalftone(c.x, c.y)-0.5)
		if c.white {
			style[i] = wht[minInt(maxInt(int(clamp01(lvl)*float64(relaxFxWhtN-1)+0.5), 0), relaxFxWhtN-1)]
		} else {
			style[i] = fur[minInt(maxInt(int(clamp01(lvl)*float64(relaxFxFurN-1)+0.5), 0), relaxFxFurN-1)]
		}
		glyph[i], body[i] = c.glyph, true
	}

	// ── Vaga-lumes ── na frente de tudo, com halo. Eles são a única coisa viva
	// na cena, então precisam de mais que um ponto aceso.
	for i, f := range st.flies {
		if f.glow < 0.05 || float64(i) > float64(len(st.flies))*dens {
			continue
		}
		cx, cy := f.x*float64(w-1), f.y*float64(h-1)
		// O halo acompanha o tamanho do desenho: um brilho de três células é
		// atmosfera num palco grande e um borrão em cima da raposa num pequeno.
		hr := (1.0 + 1.8*f.glow) * math.Max(0.5, sp.scale)
		for dy := -int(hr) - 1; dy <= int(hr)+1; dy++ {
			for dx := -int(hr) - 1; dx <= int(hr)+1; dx++ {
				x, y := int(cx)+dx, int(cy)+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				// A célula é o dobro de alta: o halo tem de ser mais largo que
				// alto, senão vira uma coluna.
				d := math.Hypot(float64(dx), float64(dy)*2.1) / hr
				if d > 1 {
					continue
				}
				v := f.glow * (1 - d*d)
				if v < 0.10 {
					continue
				}
				i := y*w + x
				// Nunca por cima do bicho: vaga-lume que apaga uma célula do
				// sprite abre buraco na silhueta, e buraco não pisca — some.
				if body[i] {
					continue
				}
				glyph[i] = relaxFxGlow(v)
				style[i] = fly[minInt(int(v*float64(relaxFxFlyN)), relaxFxFlyN-1)]
			}
		}
	}

	lines := make([]string, h)
	var b strings.Builder
	for y := 0; y < h; y++ {
		b.Reset()
		for x := 0; x < w; x++ {
			i := y*w + x
			if glyph[i] == "" {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(style[i].Render(glyph[i]))
		}
		lines[y] = b.String()
	}

	status := "a raposa não se mexe"
	switch {
	case st.near < 0.10:
		status = "um vaga-lume passou rente à orelha"
	case st.near < 0.25:
		status = "ela está olhando aquele ali"
	}
	return lines, StyleMuted.Render(status)
}
