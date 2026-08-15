package ui

import (
	"math"
	"math/rand"
)

// ── Raposa e vaga-lumes ───────────────────────────────────────────────────────
//
// A raposa é sprite fixo em Braille: forma desenhada à mão lê melhor que
// qualquer silhueta que eu monte com elipse. O corpo não se mexe — só a cauda
// dobra, e de tempos em tempos ela dá um tapa no capim da frente.
//
// O sprite tem TETO: cresce com o terminal até relaxFxMaxW×relaxFxMaxH e para.
// O que sobra do palco vira paisagem — serra no fundo, capim na frente — porque
// bicho que enche a tela não tem onde estar.
//
// Tudo cai no mesmo buffer Braille de paleta indexada (voto de maioria por
// célula): capim, vaga-lume e pelo se compõem na mesma célula sem que um abra
// buraco no outro. Como b.set respeita o primeiro que acendeu o ponto, a ordem
// de desenho é da FRENTE pro FUNDO — capim, raposa, serra, céu.
//
// Andam: os vaga-lumes, o capim, o piscar das estrelas e a cauda.
var relaxFoxArt = []string{
	"⠀⠀⠀⠀⠀⢀⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⣸⣿⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⢠⡟⠘⣷⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⢰⡟⠀⠀⠈⠻⣷⣤⣤⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⣠⡟⠀⠀⠀⠀⠀⠈⢻⡿⣷⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⣰⡟⠀⠀⠀⠀⠀⠀⠀⠀⠻⠸⣷⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⢰⡏⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⣇⠀⣠⣄⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣀⠀⠀⠀",
	"⣾⡁⢀⣠⠴⠒⠲⣤⣠⠶⠋⠳⣤⣸⣿⣰⣿⣿⣿⣷⣄⠀⠀⠀⠀⠀⠀⠀⠀⣠⣾⣿⣿⣿⡄⠀⠀",
	"⣿⠟⠉⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⣽⠏⣿⡿⢿⣿⣿⣿⣷⣄⠀⠀⠀⠀⢠⣾⣿⣿⣿⠋⢹⡇⠀⠀",
	"⢹⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⡟⠀⣿⠁⠀⠙⣿⡛⠛⢿⡶⠶⠶⠶⣿⣄⣀⣰⠃⠀⢸⡇⠀⠀",
	"⠈⢷⡀⠀⠀⠀⠀⠀⠀⠀⠀⢰⡿⠁⠀⣿⠀⠀⠀⠈⢷⡀⠘⠛⠀⠀⠀⠀⠈⠉⠳⣄⠀⢸⡇⠀⠀",
	"⠀⠈⢿⣦⡀⠀⠀⠀⠀⠀⢀⣿⣇⣀⠀⢻⠀⠀⠀⠀⢰⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠳⣾⠃⠀⠀",
	"⠀⠀⠀⠉⠻⢶⣄⣠⣴⠞⠛⠉⠉⠙⠻⢾⣇⠀⢀⣰⠏⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⡄⠀⠀",
	"⠀⠀⠀⠀⣠⣴⠟⠉⠀⠀⠀⠀⠀⠀⠀⠀⢹⡷⠞⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⡇⠀⠀",
	"⠀⢀⣠⡾⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣇⠀⠀⠀⣴⣿⠀⠀⠀⠀⠀⠀⢠⣶⠀⠀⣸⡇⠀⠀",
	"⢸⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣿⣄⣈⣀⣙⣁⠀⠀⣶⣾⡶⠀⠻⠿⠀⢠⣿⣁⡀⠀",
	"⠈⠛⠻⠿⠶⠶⠶⡤⣤⣤⣤⣄⣀⣤⣀⣠⣤⣀⣀⣹⣿⣿⣿⣿⣤⣽⣿⣴⣶⣶⡦⢼⣿⣿⣿⣿⠇",
}

// ── Paleta ────────────────────────────────────────────────────────────────────

const (
	relaxFxFurN      = 12
	relaxFxWhtN      = 5
	relaxFxFlyN      = 6
	relaxFxHillTones = 3
	relaxFxHillN     = 3 * relaxFxHillTones
	relaxFxGrassN    = 4
)

const (
	relaxFxFur   = 0
	relaxFxWht   = relaxFxFur + relaxFxFurN
	relaxFxFly   = relaxFxWht + relaxFxWhtN
	relaxFxHill  = relaxFxFly + relaxFxFlyN   // 0 = a serra do fundo
	relaxFxGrass = relaxFxHill + relaxFxHillN // 0 = chão, 3 = ponta do capim
	relaxFxStar  = relaxFxGrass + relaxFxGrassN
	relaxFxPalN  = relaxFxStar + 1
)

var relaxFoxPal = func() []relaxColor {
	out := make([]relaxColor, relaxFxPalN)
	copy(out[relaxFxFur:], relaxRamp([]string{"#2A0E06", "#4E1A0A", "#7A2C10", "#A63E16", "#C8551E", "#E06E28", "#F08C3C", "#F8AC62"}, relaxFxFurN))
	copy(out[relaxFxWht:], relaxRamp([]string{"#6E6A62", "#9E9A90", "#C6C2B8", "#E4E0D6", "#F6F3EA"}, relaxFxWhtN))
	copy(out[relaxFxFly:], relaxRamp([]string{"#3A4A10", "#6A8418", "#9ECC30", "#C4E840", "#EAFC78", "#FBFFD0"}, relaxFxFlyN))
	// Perspectiva aérea, também à noite: a serra do fundo é mais CLARA, porque
	// o ar entre ela e você tem cor. Sem isso, três silhuetas viram uma mancha.
	// Cada serra tem três tons próprios — pé, corpo e crista —, porque
	// silhueta chapada não tem encosta: a montanha só ganha volume quando a
	// face virada pra lua clareia e a de trás apaga.
	for d, stops := range [3][]string{
		{"#222A4C", "#39456E", "#5A69A0"}, // a do fundo, lavada pelo ar
		{"#151B36", "#262E4C", "#3B4874"},
		{"#090C1A", "#12172C", "#1E2748"}, // a da frente, quase recorte
	} {
		copy(out[relaxFxHill+d*relaxFxHillTones:], relaxRamp(stops, relaxFxHillTones))
	}
	copy(out[relaxFxGrass:], relaxRamp([]string{"#0C1710", "#17321B", "#255028", "#3E8034"}, relaxFxGrassN))
	out[relaxFxStar] = "#7C88A0"
	return out
}()

// ── Sprite ────────────────────────────────────────────────────────────────────

var relaxFoxDots, relaxFoxInk, relaxFoxDotW, relaxFoxDotH = func() ([]bool, []bool, int, int) {
	ink, dw, dh := relaxArtDots(relaxFoxArt)

	// Alagamento a partir da borda: o que ele não alcança está dentro do bicho.
	out := make([]bool, dw*dh)
	stack := make([][2]int, 0, dw*dh)
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= dw || y >= dh {
			return
		}
		if i := y*dw + x; !out[i] && !ink[i] {
			out[i] = true
			stack = append(stack, [2]int{x, y})
		}
	}
	for x := 0; x < dw; x++ {
		push(x, 0)
		push(x, dh-1)
	}
	for y := 0; y < dh; y++ {
		push(0, y)
		push(dw-1, y)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		push(p[0]+1, p[1])
		push(p[0]-1, p[1])
		push(p[0], p[1]+1)
		push(p[0], p[1]-1)
	}
	dots := make([]bool, dw*dh)
	for i := range dots {
		dots[i] = !out[i]
	}
	return dots, ink, dw, dh
}()

// Teto do sprite, em células. Este é o tamanho em que a raposa foi desenhada
// pra ser vista: daí pra cima quem cresce é a paisagem, não ela.
const (
	relaxFxMaxW = 37
	relaxFxMaxH = 17
)

// A cauda é o triângulo que sobe pro canto de cima à esquerda do desenho; abaixo
// de relaxFxTailBase começa o corpo. Se a cauda do desenho for outra parte, é só
// mexer nestes três números — nada mais depende deles.
const (
	relaxFxTailBase  = 0.44 // fração da altura do sprite onde a cauda vira corpo
	relaxFxTailPivot = 0.32 // fração da largura onde ela se prende ao corpo
	relaxFxTailSwing = 0.17 // radianos no ponto mais aberto
)

type relaxFoxSprite struct {
	w, h  int     // em células
	scale float64 // 1 = desenho inteiro
	lvl   []int8  // nível da paleta por subponto; -1 = apagado
	cell  []int8  // nível dominante da célula; -1 = célula vazia
}

// Terminal não muda de tamanho a cada frame, então o sprite de cada tamanho é
// resolvido uma vez e fica guardado.
var relaxFoxCache = map[[2]int]*relaxFoxSprite{}

// relaxFxTone resolve a cor de um subponto pela posição dentro do bicho: luz
// vindo de cima e da esquerda, mais um meio-tom. É o que dá volume ao corpo em
// vez de um recorte laranja chapado.
func relaxFxTone(fx, fy, ink float64, x, y int) int8 {
	// O traço do desenho entra como sombra, não como cor: é ele que devolve
	// focinho, orelha e pata dentro do laranja chapado.
	tone := 0.80 - 0.34*fy - 0.14*fx - 0.34*ink
	// A cauda sobe pro canto de cima à esquerda e pega luz por cima: é a parte
	// mais clara do bicho depois da ponta dela.
	if fx < 0.42 && fy < relaxFxTailBase {
		tone += 0.26
	}
	// A testa fica no laranja: o retângulo branco que morava aqui cobria os
	// olhos e a boca que o desenho já tem, e como caixa reta ele aparecia como
	// retângulo mesmo, não como pelo.
	white := fx > 0.10 && fx < 0.32 && fy < 0.22 // ponta da cauda
	// Beiço branco em volta do focinho: na raposa de verdade é ele que faz o
	// nariz aparecer no meio do laranja. Elipse, não caixa — e o traço do
	// desenho pesa mais aqui dentro, senão o nariz clareia junto com o beiço e
	// a cara perde o ponto escuro que a fecha.
	if mx, my := (fx-0.663)/0.150, (fy-0.935)/0.085; mx*mx+my*my < 1 {
		white = true
		tone += 0.42*(1-(mx*mx+my*my)) - 0.20*ink
	}
	// Olhinhos fechados: o desenho põe dois traços onde eles ficam, mas um
	// traço de um ponto não sobrevive à média da célula quando o sprite
	// encolhe. Aqui eles ganham escuro próprio, achatados como quem dorme.
	if fy > 0.845 && fy < 0.895 && ((fx > 0.505 && fx < 0.575) || (fx > 0.700 && fx < 0.770)) {
		tone -= 0.42
	}
	// Concha das duas orelhas: clara por dentro, senão elas somem no volume da
	// cabeça — na paleta do pelo, não na branca, que as deixaria de gesso.
	if fy > 0.42 && fy < 0.62 && ((fx > 0.46 && fx < 0.66) || (fx > 0.80 && fx < 0.96)) {
		tone += 0.24
	}
	tone = clamp01(tone + 0.06*(relaxHalftone(x, y)-0.5))
	if white {
		return int8(relaxFxWht + minInt(int(tone*float64(relaxFxWhtN-1)+0.5), relaxFxWhtN-1))
	}
	return int8(relaxFxFur + minInt(int(tone*float64(relaxFxFurN-1)+0.5), relaxFxFurN-1))
}

// relaxFxCells resolve a cor de cada célula do sprite. A raposa é opaca: onde
// ela pisa, a célula é dela. Sem isso, a borda do bicho — que é meia célula de
// pelo e meia de morro — perderia o voto de maioria e a silhueta esfarelaria.
func relaxFxCells(lvl []int8, w, h int) []int8 {
	cell := make([]int8, w*h)
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			// Cada rampa soma na sua: a média entre pelo e branco cairia no
			// meio da paleta, que é a cor de outro bicho.
			var sum, n [2]int
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					l := int(lvl[(cy*4+dy)*w*2+cx*2+dx])
					if l < 0 {
						continue
					}
					k := 0
					if l >= relaxFxWht {
						k = 1
					}
					sum[k] += l
					n[k]++
				}
			}
			k := 0
			if n[1] > n[0] {
				k = 1
			}
			if n[k] == 0 {
				cell[cy*w+cx] = -1
				continue
			}
			cell[cy*w+cx] = int8(sum[k] / n[k])
		}
	}
	return cell
}

// relaxFoxAt devolve a raposa no maior tamanho que cabe em width×height,
// sempre na proporção original — esticar um bicho é pior que encolher — e
// nunca maior que o teto.
func relaxFoxAt(width, height int) *relaxFoxSprite {
	width = maxInt(2, minInt(width, relaxFxMaxW))
	height = maxInt(2, minInt(height, relaxFxMaxH))
	key := [2]int{width, height}
	if sp, ok := relaxFoxCache[key]; ok {
		return sp
	}
	scale := math.Min(1, math.Min(
		float64(width*2)/float64(relaxFoxDotW),
		float64(height*4)/float64(relaxFoxDotH)))
	dw := maxInt(2, int(float64(relaxFoxDotW)*scale))
	dh := maxInt(4, int(float64(relaxFoxDotH)*scale))
	w, h := (dw+1)/2, (dh+3)/4

	// Cada ponto do destino olha o retângulo de pontos que lhe corresponde na
	// origem e acende se boa parte dele estiver acesa. O limiar abaixo de meio
	// é de propósito: segura os traços finos (patas, orelhas) que a média
	// apagaria.
	lvl := make([]int8, w*2*h*4)
	for i := range lvl {
		lvl[i] = -1
	}
	for y := 0; y < dh; y++ {
		sy0 := y * relaxFoxDotH / dh
		sy1 := maxInt(sy0+1, (y+1)*relaxFoxDotH/dh)
		for x := 0; x < dw; x++ {
			sx0 := x * relaxFoxDotW / dw
			sx1 := maxInt(sx0+1, (x+1)*relaxFoxDotW/dw)
			on, ink, total := 0, 0, 0
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					total++
					if relaxFoxDots[sy*relaxFoxDotW+sx] {
						on++
					}
					if relaxFoxInk[sy*relaxFoxDotW+sx] {
						ink++
					}
				}
			}
			if float64(on) < 0.38*float64(total) {
				continue
			}
			fx := float64(x) / float64(maxInt(1, dw-1))
			fy := float64(y) / float64(maxInt(1, dh-1))
			lvl[y*w*2+x] = relaxFxTone(fx, fy, float64(ink)/float64(total), x, y)
		}
	}

	sp := &relaxFoxSprite{w: w, h: h, scale: scale, lvl: lvl, cell: relaxFxCells(lvl, w, h)}
	relaxFoxCache[key] = sp
	return sp
}

// ── Estado ────────────────────────────────────────────────────────────────────

// A cabeça no sprite (0–1), pro status saber quando um vaga-lume passa perto.
const (
	relaxFxHeadX = 0.67
	relaxFxHeadY = 0.58
)

// Onde a pata da frente encosta no chão: é dali que sai o tapa no capim.
const relaxFxPawX = 0.74

// Linha do chão: a serra morre nela, o capim nasce nela, a raposa senta nela.
const relaxFxGround = 0.82

const relaxFxBladeN = 110

type relaxFxBug struct {
	x, y   float64
	vx, vy float64
	ph     float64
	blink  float64
	rate   float64
	glow   float64
	seed   int
}

type relaxFxBlade struct {
	x     float64 // 0–1 no palco
	root  float64 // 0–1 dentro da faixa de chão: quem nasce embaixo está na frente
	h     float64 // altura em fração da altura do palco
	ph    float64
	sway  float64 // amplitude do balanço, em subpontos
	front bool
}

type relaxFoxState struct {
	inited bool
	tick   int
	flies  []relaxFxBug
	blades []relaxFxBlade
	stars  []relaxSkyPt
	hills  [relaxFxHillN * 3]float64
	near   float64
	// Onde a cabeça e a pata caíram no palco. Só o render sabe o tamanho que a
	// raposa coube, e o passo precisa disso pra medir a distância do vaga-lume
	// e pra saber onde o tapa no capim acerta.
	headX, headY, pawX float64

	// Cauda e brincadeira. A fase da cauda é própria porque ela acelera quando
	// a raposa brinca — amplitude sozinha não passa a ideia de abanar.
	tailPh   float64
	play     int
	playDur  int
	nextPlay int
}

func relaxFxNewFly() relaxFxBug {
	return relaxFxBug{
		x: rand.Float64(),
		// Vaga-lume vive entre o capim e a copa que não existe: acima da serra
		// ele viraria estrela verde.
		y:  0.34 + rand.Float64()*0.58,
		vx: (rand.Float64() - 0.5) * 0.0022,
		vy: (rand.Float64() - 0.5) * 0.0014,
		ph: rand.Float64() * 6.28,
		// Cada um com seu compasso: piscar junto é árvore de natal, não
		// vaga-lume.
		blink: rand.Float64() * 6.28,
		rate:  0.045 + rand.Float64()*0.070,
		seed:  rand.Intn(9999),
	}
}

func relaxFxNewBlade(front bool) relaxFxBlade {
	b := relaxFxBlade{
		x:     rand.Float64(),
		ph:    rand.Float64() * 6.28,
		sway:  0.8 + rand.Float64()*1.8,
		front: front,
		root:  rand.Float64() * 0.30,
		h:     0.022 + rand.Float64()*0.042,
	}
	if front {
		// O da frente nasce mais embaixo, é mais alto e balança mais: é a
		// diferença de altura, não a cor, que dá a profundidade do capim.
		b.root = 0.30 + rand.Float64()*0.70
		b.h = 0.050 + rand.Float64()*0.075
		b.sway += 1.2
	}
	// Um em cada dez é mato, não grama: sem esses tufos altos a faixa vira
	// escova de dente.
	if rand.Intn(10) == 0 {
		b.h *= 1.8
	}
	return b
}

func stepRelaxFox(st *relaxFoxState) {
	if !st.inited {
		st.inited = true
		st.headX, st.headY, st.pawX = 0.5, 0.5, 0.5
		st.nextPlay = 60 + rand.Intn(140)
		for i, n := 0, 14+rand.Intn(6); i < n; i++ {
			st.flies = append(st.flies, relaxFxNewFly())
		}
		for i, n := 0, 44+rand.Intn(22); i < n; i++ {
			st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.55})
		}
		for i := range st.hills {
			st.hills[i] = rand.Float64() * 2 * math.Pi
		}
		for i := 0; i < relaxFxBladeN; i++ {
			st.blades = append(st.blades, relaxFxNewBlade(i%5 < 2))
		}
	}
	st.tick++

	st.tailPh += 0.052
	if st.play > 0 {
		st.tailPh += 0.09
		if st.play--; st.play == 0 {
			st.nextPlay = 110 + rand.Intn(170)
		}
	} else if st.nextPlay--; st.nextPlay <= 0 {
		st.playDur = 24 + rand.Intn(14)
		st.play = st.playDur
	}

	for i := range st.flies {
		f := &st.flies[i]
		f.ph += 0.035
		f.blink += f.rate
		// Acende e apaga com pausa apagado no meio — daí o max(0, sin). Ao
		// quadrado porque brasa acende devagar e some rápido; seno puro pisca
		// feito LED.
		g := math.Max(0, math.Sin(f.blink))
		f.glow = g * g
		// Vagar por ruído contínuo: com seno o bicho anda em figura de
		// Lissajous, que é bonito e nada parecido com um vaga-lume.
		f.x += f.vx + relaxNoise(f.ph*0.7, f.seed)*0.0024
		f.y += f.vy + relaxNoise(f.ph*0.55, f.seed+77)*0.0016
		if f.x < -0.04 || f.x > 1.04 || f.y < 0.30 || f.y > 0.96 {
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
		if d := math.Hypot(f.x-st.headX, (f.y-st.headY)*1.9); d < st.near {
			st.near = d
		}
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxFoxFrames(st *relaxFoxState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxFox(st)
	}
	b := newRelaxBrailleVote(maxInt(14, minInt(width, 120)), maxInt(5, minInt(height, 34)))
	relaxFoxScene(st, b)

	status := "o rabo dela balança devagar"
	switch {
	case st.play > 0:
		status = "ela está brincando com o capim"
	case st.near < 0.10:
		status = "um vaga-lume passou rente à orelha"
	case st.near < 0.25:
		status = "ela está olhando aquele ali"
	case st.tick%600 < 110:
		status = "o capim balança, ela quase não"
	}
	return b.lines(relaxStyles(relaxFoxPal, fade)), StyleMuted.Render(status)
}

// relaxFoxScene desenha o quadro inteiro. Ordem da FRENTE pro FUNDO, porque
// b.set respeita o primeiro que acendeu o subponto — inverter isso é o mesmo
// que apagar o que está na frente.
func relaxFoxScene(st *relaxFoxState, b *relaxBraille) {
	w, h := b.w, b.h
	sw, sh := w*2, h*4
	hy := int(float64(sh) * relaxFxGround)
	t := float64(st.tick) * 0.1
	// Palco menor pede menos gente nele: a conta mantém a densidade em vez do
	// número, senão o terminal apertado vira sopa de pontos.
	dens := clamp01(float64(sw) / 220)
	// Rajada: o capim inteiro respira junto e a fase própria de cada folha
	// desmancha a onda. Só isso já parece vento.
	wind := 0.55 + 0.45*relaxNoise(t*0.12, 4242)

	// A raposa senta com as patas na linha do chão e tem uma linha de céu
	// acima da orelha. O que sobra do quadro é paisagem.
	sp := relaxFoxAt(w*3/5, hy/4)
	fx0, fy0 := (w-sp.w)/2, maxInt(0, hy/4+1-sp.h)
	// O passo precisa saber onde a cabeça caiu pra medir a distância do
	// vaga-lume, e só aqui se sabe o tamanho que a raposa coube.
	st.headX = (float64(fx0) + relaxFxHeadX*float64(sp.w)) / float64(w)
	st.headY = (float64(fy0) + relaxFxHeadY*float64(sp.h)) / float64(h)
	st.pawX = (float64(fx0) + relaxFxPawX*float64(sp.w)) / float64(w)

	// Tapa no capim: golpe com decaimento, não onda. u vai de 1 a 0 ao longo da
	// brincadeira, então u² apaga o movimento e o seno interno dá o chicote.
	swat := 0.0
	if st.play > 0 && st.playDur > 0 {
		u := float64(st.play) / float64(st.playDur)
		swat = u * u * math.Sin((1-u)*13)
	}

	// ── Capim da frente ── primeiro de todos: quem acende o ponto fica com ele.
	relaxFxDrawGrass(b, st, sw, sh, hy, t, wind, dens, swat, st.pawX, true)

	// ── Vaga-lumes ── na frente da raposa, atrás do capim.
	relaxFxDrawFlies(b, st, sw, sh, dens)

	// ── Raposa ── o corpo é sprite parado; a cauda dobra. Mapeamento INVERSO —
	// pra cada ponto do destino eu pergunto de onde ele veio — senão a rotação
	// abre buraco no pelo. O ângulo cresce com o quadrado da distância da raiz,
	// então a cauda dobra em vez de girar inteira feito ponteiro.
	dw, dh := sp.w*2, sp.h*4
	bend := relaxFxTailSwing * math.Sin(st.tailPh)
	if st.play > 0 {
		bend *= 1.9
	}
	px, py := float64(dw)*relaxFxTailPivot, float64(dh)*relaxFxTailBase
	for y := 0; y < dh; y++ {
		fy := float64(y) / float64(dh)
		curl := fy < relaxFxTailBase
		ca, sa := 1.0, 0.0
		if curl {
			k := (relaxFxTailBase - fy) / relaxFxTailBase
			a := -bend * k * k
			ca, sa = math.Cos(a), math.Sin(a)
		}
		for x := 0; x < dw; x++ {
			sx, sy := x, y
			if curl {
				ox, oy := float64(x)-px, float64(y)-py
				sx = int(px + ox*ca + oy*sa)
				sy = int(py - ox*sa + oy*ca)
				if sx < 0 || sy < 0 || sx >= dw || sy >= dh {
					continue
				}
			}
			if l := sp.lvl[sy*dw+sx]; l >= 0 {
				b.set(fx0*2+x, fy0*4+y, int(l))
			}
		}
	}
	// A célula com pelo dentro é do pelo — menos a que um vaga-lume já tomou,
	// que é o único que passa na frente dela. A faixa da cauda fica de fora: ela
	// se move, então a célula pré-calculada não vale mais ali; e lá em cima só
	// tem céu atrás, que o voto de maioria já resolve.
	for cy := int(relaxFxTailBase*float64(sp.h)) + 1; cy < sp.h; cy++ {
		for cx := 0; cx < sp.w; cx++ {
			if l := sp.cell[cy*sp.w+cx]; l >= 0 && b.force[(fy0+cy)*w+fx0+cx] < 0 {
				b.paint(fx0+cx, fy0+cy, int(l))
			}
		}
	}

	// ── Capim do fundo ── nasce na linha do chão, atrás das patas.
	relaxFxDrawGrass(b, st, sw, sh, hy, t, wind, dens, swat*0.4, st.pawX, false)

	// ── Serra ── da frente pro fundo, cada cadeia morrendo na linha do chão.
	for d := relaxFxHillN - 1; d >= 0; d-- {
		relaxFxDrawHill(b, st, sw, hy, d)
	}

	// ── Chão ── fechado embaixo e esgarçado na linha do horizonte, senão a
	// faixa vira tarja e o capim parece colado num degrau.
	for y := hy; y < sh; y++ {
		f := float64(y-hy) / float64(maxInt(1, sh-hy))
		lvl := relaxFxGrass
		if f > 0.5 {
			lvl++
		}
		for x := 0; x < sw; x++ {
			if relaxHalftone(x, y) < 1.3*f*f-0.06 && !b.taken(x, y) {
				b.set(x, y, lvl)
			}
		}
	}

	// ── Estrelas ── por último: o que a serra já acendeu não vira estrela.
	for i, p := range st.stars {
		if float64(i) > float64(len(st.stars))*dens {
			break
		}
		x, y := int(p.x*float64(sw-1)), int(p.y*float64(hy))
		if relaxHalftone(x, y) > 0.50+0.20*math.Sin(t*0.4+float64(i)) {
			continue
		}
		b.set(x, y, relaxFxStar)
	}
}

// relaxFxDrawHill desenha uma cadeia: três harmônicas (um seno só entrega o
// desenho na hora) preenchidas até a linha do chão.
func relaxFxDrawHill(b *relaxBraille, st *relaxFoxState, sw, hy, d int) {
	// d = 0 é a do fundo: pico mais alto e cor mais clara.
	far := float64(2-d) / 2
	amp := float64(hy) * (0.11 + 0.17*far)
	k := d * 3
	// A crista sai primeiro pro perfil inteiro: a encosta precisa da vizinha
	// pra saber de que lado está virada, e recalcular seno por coluna duas
	// vezes é o dobro do custo do desenho inteiro.
	crest := make([]float64, sw)
	for x := 0; x < sw; x++ {
		// Frequência em fração da largura, não em subponto: assim a serra tem
		// o mesmo tanto de pico no palco largo e no estreito. Em subponto, o
		// terminal pequeno pegava meia onda e virava rampa.
		u := float64(x) / float64(sw) * 2 * math.Pi
		crest[x] = float64(hy) - amp*(math.Abs(math.Sin(u*1.7+st.hills[k]))*0.80+
			0.45*math.Abs(math.Sin(u*3.3+st.hills[k+1]))+
			0.20*math.Sin(u*6.1+st.hills[k+2]))
	}
	for x := 0; x < sw; x++ {
		v := crest[x]
		// Inclinação da crista. A luz vem da esquerda (é de onde vem a lua da
		// cena), então encosta que sobe pra direita está virada pra ela.
		sl := 0.0
		switch {
		case x == 0:
			sl = crest[1] - crest[0]
		case x == sw-1:
			sl = crest[x] - crest[x-1]
		default:
			sl = (crest[x+1] - crest[x-1]) / 2
		}
		lit := clamp01(0.5 - sl*0.85)
		for y := int(v); y < hy; y++ {
			if b.taken(x, y) {
				continue
			}
			// A crista pega o céu; o pé da encosta afunda no escuro. Sem essa
			// queda a serra vira faixa de cor uniforme até o chão.
			depth := clamp01((float64(y) - v) / (0.45*amp + 1))
			lum := 0.30 + 0.70*lit - 0.62*depth
			b.set(x, y, relaxFxHill+d*relaxFxHillTones+
				minInt(maxInt(int(lum*float64(relaxFxHillTones)), 0), relaxFxHillTones-1))
		}
	}
}

// relaxFxDrawGrass desenha uma camada de capim. A folha é uma curva: quase reta
// na base e cada vez mais deitada na ponta — folha que balança inteira parece
// limpador de para-brisa.
func relaxFxDrawGrass(b *relaxBraille, st *relaxFoxState, sw, sh, hy int, t, wind, dens, swat, swatX float64, front bool) {
	band := float64(sh - hy)
	for i, bl := range st.blades {
		if bl.front != front || float64(i) > float64(len(st.blades))*dens {
			continue
		}
		x0 := bl.x * float64(sw-1)
		y0 := float64(hy) + bl.root*band
		n := maxInt(2, int(bl.h*float64(sh)))
		bend := bl.sway * wind * math.Sin(t*0.55+bl.ph)
		// Só a moita ao alcance da pata sente o tapa, e sente proporcional à
		// distância — o capim inteiro chicotear junto viraria rajada, não tapa.
		if swat != 0 {
			if d := math.Abs(bl.x - swatX); d < 0.17 {
				bend += swat * (1 - d/0.17) * bl.sway * 3.4
			}
		}
		lvl := relaxFxGrass + 1
		if front {
			lvl++
		}
		for j := 0; j <= n; j++ {
			f := float64(j) / float64(n)
			l := lvl
			if f > 0.62 {
				l++ // a ponta pega a luz que a base não pega
			}
			bx, by := int(x0+bend*f*f*1.7), int(y0)-j
			if front || !b.taken(bx, by) {
				b.set(bx, by, minInt(l, relaxFxGrass+relaxFxGrassN-1))
			}
		}
	}
}

// relaxFxDrawFlies desenha o halo em subponto com meio-tom: a borda esgarça em
// vez de cortar, e é isso que faz o brilho parecer luz e não bloco verde. O
// miolo pinta a célula inteira — a única coisa da cena que passa na frente da
// raposa sem abrir buraco nela, porque paint troca a cor e mantém os pontos.
func relaxFxDrawFlies(b *relaxBraille, st *relaxFoxState, sw, sh int, dens float64) {
	for i, f := range st.flies {
		if f.glow < 0.04 || float64(i) > float64(len(st.flies))*dens {
			continue
		}
		cx, cy := f.x*float64(sw-1), f.y*float64(sh-1)
		r := (1.3 + 3.2*f.glow) * clampF(float64(sh)/80, 0.6, 1.15)
		for dy := -int(r) - 1; dy <= int(r)+1; dy++ {
			for dx := -int(r) - 1; dx <= int(r)+1; dx++ {
				d := math.Hypot(float64(dx), float64(dy)) / r
				if d > 1 {
					continue
				}
				x, y := int(cx)+dx, int(cy)+dy
				v := f.glow * (1 - d*d)
				if v < 0.08 || relaxHalftone(x, y) > v {
					continue
				}
				lvl := relaxFxFly + minInt(int(v*float64(relaxFxFlyN)), relaxFxFlyN-1)
				b.set(x, y, lvl)
				if v > 0.72 {
					b.paint(x/2, y/4, lvl)
				}
			}
		}
	}
}
