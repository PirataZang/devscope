package ui

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Relax é uma tela sem operação: só animações de terminal para descansar a cabeça.

type relaxGame int

const (
	relaxGameCube relaxGame = iota
	relaxGameAsteroid
	relaxGameCat
	relaxGameSky
	relaxGameGalaxy
	relaxGameNebula
	relaxGameBlackHole
	relaxGameMoon
	relaxGameLeaves
	relaxGamePetals
	relaxGameTree
	relaxGameFire
	relaxGameCoffee
	relaxGameSea
	relaxGameBurst
	relaxGameCity
	relaxGameFox
	relaxGameFireworks
	relaxGameTetris
	relaxGameSword
	relaxGameHourglass
	relaxGameChess
	relaxGameJackpot
	relaxGameV4
)

// relaxScene descreve uma cena: rótulo, passo de simulação e render. Tabela em
// vez de switch espalhado — cena nova é uma linha aqui e nada mais.
type relaxScene struct {
	name     string
	glyph    string
	subtitle string
	color    lipgloss.Color
	step     func(*App)
	frames   func(*App, int, int, float64) ([]string, string)
	objects  func(*App) int // contagem viva, só pro debug
}

var relaxScenes = []relaxScene{
	relaxGameCube: {
		name: "Magic Cube", glyph: "▧", subtitle: "embaralha e desfaz, girando no ar", color: ColorWarning,
		step: func(a *App) { stepRelaxCube(&a.relaxCube) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxCubeFrames(&a.relaxCube, w, h, f)
		},
	},
	relaxGameAsteroid: {
		name: "Asteroids", glyph: "➤", subtitle: "uma nave à deriva no espaço", color: ColorAccent,
		step:    func(a *App) { stepRelaxAsteroid(&a.relaxAst) },
		frames:  func(a *App, w, h int, f float64) ([]string, string) { return relaxAsteroidFrames(&a.relaxAst, w, h, f) },
		objects: func(a *App) int { return len(a.relaxAst.rocks) + len(a.relaxAst.shots) + len(a.relaxAst.parts) },
	},
	relaxGameCat: {
		name: "Cat", glyph: "≽", subtitle: "gatinho tirando um cochilo", color: ColorPink,
		step:    func(a *App) { stepRelaxCat(&a.relaxCat) },
		frames:  func(a *App, w, h int, f float64) ([]string, string) { return relaxCatFrames(&a.relaxCat, w, h, f) },
		objects: func(a *App) int { return len(a.relaxCat.zzz) },
	},
	relaxGameSky: {
		name: "Starfield", glyph: "✦", subtitle: "céu noturno, cometas raros", color: ColorHighlight,
		step:    func(a *App) { stepRelaxSky(&a.relaxSky) },
		frames:  func(a *App, w, h int, f float64) ([]string, string) { return relaxSkyFrames(&a.relaxSky, w, h, f) },
		objects: func(a *App) int { return len(a.relaxSky.stars) + len(a.relaxSky.comets) },
	},
	relaxGameGalaxy: {
		name: "Galaxy", glyph: "◎", subtitle: "estrelas orbitando um centro", color: ColorHighlight,
		step: func(a *App) { stepRelaxGalaxy(&a.relaxGalaxy) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxGalaxyFrames(&a.relaxGalaxy, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxGalaxy.stars) },
	},
	relaxGameNebula: {
		name: "Nebula", glyph: "❋", subtitle: "gás e poeira de uma estrela por nascer", color: ColorPink,
		step: func(a *App) { stepRelaxNebula(&a.relaxNeb) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxNebulaFrames(&a.relaxNeb, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxNeb.stars) + len(a.relaxNeb.young) },
	},
	relaxGameBlackHole: {
		name: "Black Hole", glyph: "◍", subtitle: "um disco caindo pra dentro", color: ColorWarning,
		step: func(a *App) { stepRelaxBlackHole(&a.relaxBh) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxBlackHoleFrames(&a.relaxBh, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxBh.stars) },
	},
	relaxGameMoon: {
		name: "Moon", glyph: "◐", subtitle: "a lua mudando de fase", color: ColorAccent,
		step:    func(a *App) { stepRelaxMoon(&a.relaxMoon) },
		frames:  func(a *App, w, h int, f float64) ([]string, string) { return relaxMoonFrames(&a.relaxMoon, w, h, f) },
		objects: func(a *App) int { return len(a.relaxMoon.stars) + len(a.relaxMoon.spots) },
	},
	relaxGameLeaves: {
		name: "Leaves", glyph: "◟", subtitle: "folhas caindo devagar", color: ColorWarning,
		step: func(a *App) { stepRelaxFall(&a.relaxLeaves, relaxFallLeaves) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxFallFrames(&a.relaxLeaves, relaxFallLeaves, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxLeaves.parts) },
	},
	relaxGamePetals: {
		name: "Petals", glyph: "◦", subtitle: "pétalas levadas pelo vento", color: ColorPink,
		step: func(a *App) { stepRelaxFall(&a.relaxPetals, relaxFallPetals) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxFallFrames(&a.relaxPetals, relaxFallPetals, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxPetals.parts) },
	},
	relaxGameTree: {
		name: "Tree", glyph: "⣿", subtitle: "a brisa passando pela copa", color: ColorSuccess,
		step: func(a *App) { stepRelaxTree(&a.relaxTree) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxTreeFrames(&a.relaxTree, w, h, f)
		},
		objects: func(a *App) int { return a.relaxTree.clusters },
	},
	relaxGameFire: {
		name: "Campfire", glyph: "▲", subtitle: "uma fogueira queimando devagar", color: ColorDanger,
		step: func(a *App) { stepRelaxFire(&a.relaxFire) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxFireFrames(&a.relaxFire, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxFire.sparks) },
	},
	relaxGameCoffee: {
		name: "Coffee", glyph: "◒", subtitle: "o bule enche, e alguém bebe", color: ColorWarning,
		step: func(a *App) { stepRelaxCoffee(&a.relaxCoffee) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxCoffeeFrames(&a.relaxCoffee, w, h, f)
		},
	},
	relaxGameSea: {
		name: "Sea", glyph: "≈", subtitle: "nuvens passando, o mar rolando", color: ColorAccent,
		step: func(a *App) { stepRelaxSea(&a.relaxSea) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxSeaFrames(&a.relaxSea, w, h, f)
		},
	},
	relaxGameBurst: {
		name: "Supernova", glyph: "✷", subtitle: "junta energia até não caber mais", color: ColorWarning,
		step: func(a *App) { stepRelaxBurst(&a.relaxBurst) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxBurstFrames(&a.relaxBurst, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxBurst.parts) },
	},
	relaxGameCity: {
		name: "Night City", glyph: "▥", subtitle: "janelas acendendo e apagando", color: ColorHighlight,
		step: func(a *App) { stepRelaxCity(&a.relaxCity) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxCityFrames(&a.relaxCity, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxCity.wins) },
	},
	relaxGameFox: {
		name: "Fox", glyph: "⌁", subtitle: "parada, olhando os vaga-lumes", color: ColorWarning,
		step: func(a *App) { stepRelaxFox(&a.relaxFox) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxFoxFrames(&a.relaxFox, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxFox.flies) },
	},
	relaxGameFireworks: {
		name: "Fireworks", glyph: "✺", subtitle: "o céu abrindo em cores", color: ColorPink,
		step: func(a *App) { stepRelaxFireworks(&a.relaxFw) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxFireworksFrames(&a.relaxFw, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxFw.parts) + len(a.relaxFw.rockets) },
	},
	relaxGameTetris: {
		name: "Tetris", glyph: "▤", subtitle: "jogando sozinho, sem pressa", color: ColorSuccess,
		step: func(a *App) { stepRelaxTetris(&a.relaxTetris) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxTetrisFrames(&a.relaxTetris, w, h, f)
		},
	},
	relaxGameSword: {
		name: "Sword", glyph: "†", subtitle: "a espada na pedra, e o vento no capim", color: ColorHighlight,
		step: func(a *App) { stepRelaxSword(&a.relaxSword) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxSwordFrames(&a.relaxSword, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxSword.leaves) },
	},
	relaxGameHourglass: {
		name: "Hourglass", glyph: "⧗", subtitle: "a areia é estrela, e vira galáxia", color: ColorHighlight,
		step: func(a *App) { stepRelaxHourglass(&a.relaxHg) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxHourglassFrames(&a.relaxHg, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxHg.stars) },
	},
	relaxGameChess: {
		name: "Chess", glyph: "♛", subtitle: "uma dama torneada, girando", color: ColorWarning,
		step: func(a *App) { stepRelaxChess(&a.relaxChess) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxChessFrames(&a.relaxChess, w, h, f)
		},
	},
	relaxGameJackpot: {
		name: "Jackpoint", glyph: "▣", subtitle: "três caixas e a sorte", color: ColorWarning,
		step: func(a *App) { stepRelaxJackpot(&a.relaxJp) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxJackpotFrames(&a.relaxJp, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxJp.parts) },
	},
	relaxGameV4: {
		name: "V4", glyph: "⋁", subtitle: "quatro pistões, corte ao vivo", color: ColorDanger,
		step: func(a *App) { stepRelaxV4(&a.relaxV4) },
		frames: func(a *App, w, h int, f float64) ([]string, string) {
			return relaxV4Frames(&a.relaxV4, w, h, f)
		},
		objects: func(a *App) int { return len(a.relaxV4.puffs) },
	},
}

var relaxGames = func() []relaxGame {
	all := make([]relaxGame, len(relaxScenes))
	for i := range relaxScenes {
		all[i] = relaxGame(i)
	}
	return all
}()

func (g relaxGame) scene() relaxScene {
	if int(g) < 0 || int(g) >= len(relaxScenes) {
		return relaxScenes[relaxGameCube]
	}
	return relaxScenes[g]
}

func (g relaxGame) String() string        { return g.scene().name }
func (g relaxGame) glyph() string         { return g.scene().glyph }
func (g relaxGame) subtitle() string      { return g.scene().subtitle }
func (g relaxGame) color() lipgloss.Color { return g.scene().color }

func (a *App) openRelax() {
	if a.view != ViewRelax {
		a.relaxReturnView = a.view
	}
	a.view = ViewRelax
	a.relaxEng.reset()
	a.resetRelaxScenes()
	a.statusMsg = ""
}

func (a *App) closeRelax() {
	// Sair do Relax solta o estado de todas as cenas; o loop de animação morre
	// sozinho no próximo tick (needsAnim). Nada de timer/goroutine vivo aqui.
	a.resetRelaxScenes()
	a.relaxEng = relaxEngine{}
	back := a.relaxReturnView
	if back == ViewRelax || (back == ViewProject && a.selectedProject == nil) {
		back = ViewDashboard
	}
	a.view = back
}

// resetRelaxScenes zera o estado de todas as cenas num lugar só — cena nova
// entra aqui e em relaxScenes, e mais nada precisa lembrar dela.
func (a *App) resetRelaxScenes() {
	a.relaxCube = relaxCubeState{}
	a.relaxAst = relaxAsteroidState{}
	a.relaxCat = relaxCatState{}
	a.relaxSky = relaxSkyState{}
	a.relaxGalaxy = relaxGalaxyState{}
	a.relaxMoon = relaxMoonState{}
	a.relaxLeaves = relaxFallState{}
	a.relaxPetals = relaxFallState{}
	a.relaxTree = relaxTreeState{}
	a.relaxNeb = relaxNebulaState{}
	a.relaxBh = relaxBlackHoleState{}
	a.relaxFire = relaxFireState{}
	a.relaxCoffee = relaxCoffeeState{}
	a.relaxSea = relaxSeaState{}
	a.relaxBurst = relaxBurstState{}
	a.relaxCity = relaxCityState{}
	a.relaxFox = relaxFoxState{}
	a.relaxFw = relaxFireworksState{}
	a.relaxTetris = relaxTetrisState{}
	a.relaxSword = relaxSwordState{}
	a.relaxHg = relaxHourglassState{}
	a.relaxChess = relaxChessState{}
	a.relaxJp = relaxJackpotState{}
	a.relaxV4 = relaxV4State{}
}

// relaxSceneObjects conta o que está vivo na cena atual (só pro debug).
func (a *App) relaxSceneObjects() int {
	if f := a.relaxGame.scene().objects; f != nil {
		return f(a)
	}
	return 0
}

func (a *App) stepRelax() {
	a.relaxGame.scene().step(a)
}

// selectRelaxGame não troca na hora: pede a transição ao engine e a cena só
// muda quando a tela já está apagada, no meio do crossfade.
func (a *App) selectRelaxGame(g relaxGame) {
	if a.relaxGame == g || a.relaxPendingOn {
		return
	}
	a.relaxPending, a.relaxPendingOn = g, true
	a.relaxEng.beginTransition()
}

// applyRelaxPending é chamada pelo loop quando o crossfade chega ao meio.
func (a *App) applyRelaxPending() {
	if !a.relaxPendingOn || !a.relaxEng.half() || a.relaxEng.switched {
		return
	}
	a.relaxGame = a.relaxPending
	a.relaxPendingOn = false
	a.relaxEng.switched = true
	a.relaxEng.scene = 0
	a.resetRelaxScenes()
}

// relaxSelectedGame é o que a lista destaca: durante o crossfade já é a cena
// nova, senão a navegação pareceria travada meio segundo.
func (a *App) relaxSelectedGame() relaxGame {
	if a.relaxPendingOn {
		return a.relaxPending
	}
	return a.relaxGame
}

func (a *App) moveRelaxGame(delta int) {
	idx := (int(a.relaxSelectedGame()) + delta + len(relaxGames)) % len(relaxGames)
	a.selectRelaxGame(relaxGames[idx])
}

func (a *App) updateRelax(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.closeRelax()
	case "up", "k", "shift+tab":
		a.moveRelaxGame(-1)
	case "down", "j", "tab":
		a.moveRelaxGame(1)
	default:
		// 1–9 e 0 vão direto na cena da lista.
		if i := strings.IndexRune("1234567890", []rune(msg.String() + " ")[0]); i >= 0 && len(msg.String()) == 1 && i < len(relaxGames) {
			a.selectRelaxGame(relaxGames[i])
		}
	}
	return a, nil
}

func (a *App) renderRelax() string {
	panelH := a.projectPanelHeight()
	sidebar := a.renderRelaxSidebar(panelH)
	sidebarW := lipgloss.Width(sidebar)
	contentW := maxInt(24, a.width-sidebarW-3)

	content := a.renderRelaxStage(contentW, panelH)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", content)
	hints := "↑↓ cena  1–0 direto  ctrl+t/esc voltar  q sair"
	if dbg := a.relaxEng.debug(a.relaxGame.String(), a.relaxSceneObjects()); dbg != "" {
		hints = dbg
	}
	if a.projectCompact() {
		return lipgloss.JoinVertical(lipgloss.Left, layout, a.renderStatusBar(hints))
	}
	return lipgloss.JoinVertical(lipgloss.Left, layout, "", a.renderStatusBar(hints))
}

// relaxRainbowTime é o relógio do título; com movimento reduzido a onda anda na
// metade da velocidade.
func (a *App) relaxRainbowTime() float64 {
	if a.relaxEng.reduced {
		return a.relaxEng.elapsed * 0.5
	}
	return a.relaxEng.elapsed
}

func (a *App) renderRelaxSidebar(height int) string {
	width := 30
	if a.width > 0 && a.width < 96 {
		width = 26
	}
	if a.width > 0 && a.width < 72 {
		width = 20
	}
	inner := maxInt(12, width-2)
	accent := a.relaxGame.color()
	contentH := maxInt(1, height-2)

	rows := make([]string, 0, contentH)
	rows = append(rows, relaxBanner(inner, a.relaxRainbowTime())...)
	rows = append(rows, StyleMuted.Render(truncate("respira · nada roda aqui", inner)))
	rows = append(rows, sidebarRule(inner, accent))
	rows = append(rows, sidebarGroupLabel("ANIMATIONS", inner, accent))
	// A lista rola pra manter a cena escolhida à vista: com dezesseis cenas
	// ela deixa de caber em terminal baixo, e antes as últimas simplesmente
	// sumiam do menu.
	foot := []string{sidebarRule(inner, ColorBorder), StyleMuted.Render("ctrl+t · esc")}
	listH := maxInt(3, contentH-len(rows)-len(foot))
	start := 0
	if len(relaxGames) > listH {
		start = minInt(maxInt(0, int(a.relaxSelectedGame())-listH/2), len(relaxGames)-listH)
	}
	for i := start; i < minInt(len(relaxGames), start+listH); i++ {
		rows = append(rows, a.renderRelaxSidebarRow(relaxGames[i], inner))
	}

	if len(rows)+len(foot) <= contentH {
		blank := contentH - len(rows) - len(foot)
		for i := 0; i < blank; i++ {
			rows = append(rows, "")
		}
		rows = append(rows, foot...)
	}
	if len(rows) > contentH {
		rows = rows[:contentH]
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(width).
		Height(contentH).
		Align(lipgloss.Left, lipgloss.Top).
		Render(strings.Join(rows, "\n"))
}

func (a *App) renderRelaxSidebarRow(g relaxGame, width int) string {
	name := g.String()
	if g == a.relaxSelectedGame() {
		left := "▌" + g.glyph() + " " + name
		pad := maxInt(0, width-lipgloss.Width(left))
		return lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorSelBg).
			Bold(true).
			Render(left + strings.Repeat(" ", pad))
	}
	accent := lipgloss.NewStyle().Foreground(g.color()).Bold(true)
	return " " + accent.Render(g.glyph()) + " " + StyleMuted.Render(truncate(name, maxInt(6, width-4)))
}

// relaxBanner escreve RELAX! em texto simples, uma letra por cor: o Rainbow
// Flow lê melhor num título miúdo do que na fonte de bloco.
func relaxBanner(width int, t float64) []string {
	const word = "RELAX!"
	var b strings.Builder
	for i, ch := range word {
		// Corta por letra, não com truncate(): a string já vem com os escapes
		// de cor e runewidth conta cada byte de ANSI como coluna — o título
		// inteiro virava "R…".
		if i >= width {
			break
		}
		b.WriteString(lipgloss.NewStyle().
			Foreground(relaxRainbowColor(i, t)).
			Bold(true).
			Render(string(ch)))
	}
	return []string{b.String()}
}

// Rainbow Flow: a onda anda pelo tempo decorrido (segundos), não por contagem
// de frames — no tick de render de ~30fps a cor caminha de verdade, contínua, e
// 360°→0° passa sem emenda. relaxRainbowSpeed é volta completa do espectro por
// segundo: 0,25 = 4s por volta. 0,4 acelera, 0,15 fica quase parado.
const (
	relaxRainbowSpeed   = 0.25
	relaxRainbowSpacing = 55.0 // defasagem de hue entre caracteres
	relaxRainbowEase    = 0.5  // segundos de arranque ao entrar no Relax
)

func relaxRainbowColor(i int, secs float64) lipgloss.Color {
	if secs < 0 {
		secs = 0
	}
	// Arranque suave: o título nasce com as cores base e a onda entra depois.
	t := secs * easeOutCubic(secs/relaxRainbowEase)
	hue := 280 + t*relaxRainbowSpeed*360 + float64(i)*relaxRainbowSpacing
	// Modulação orgânica mínima — tira o ar de rampa matemática sem virar psicodelia.
	hue += 6 * math.Sin(secs*0.7+float64(i))

	sat, light := 0.72, 0.70
	if relaxThemeIsLight() {
		sat, light = 0.62, 0.42 // no tema claro, cor pastel some no fundo branco
	}
	return relaxHSL(math.Mod(math.Mod(hue, 360)+360, 360), sat, light)
}

// relaxThemeIsLight decide pela luminância do fundo do tema atual, não pelo id —
// assim um tema claro novo já entra certo.
func relaxThemeIsLight() bool {
	var r, g, b int
	if _, err := fmt.Sscanf(string(ColorBg), "#%02x%02x%02x", &r, &g, &b); err != nil {
		return false
	}
	return (299*r+587*g+114*b)/1000 > 128
}

func relaxHSL(h, s, l float64) lipgloss.Color {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		int((r+m)*255+0.5), int((g+m)*255+0.5), int((b+m)*255+0.5)))
}

func (a *App) renderRelaxStage(width, height int) string {
	g := a.relaxGame
	title := strings.ToUpper(g.String())
	innerW := maxInt(8, width-4)
	bodyH := maxInt(3, height-2)

	art, status := g.scene().frames(a, innerW, bodyH-4, a.relaxEng.fade())
	block := relaxCenterBlock(art, innerW)
	caption := []string{"", relaxCenterLine(status, innerW), relaxCenterLine(StyleMuted.Render(g.subtitle()), innerW)}
	lines := make([]string, 0, bodyH)
	top := maxInt(0, (bodyH-len(block)-len(caption))/2)
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, block...)
	lines = append(lines, caption...)
	return renderApiTitledBox(title, fitExactLines(lines, bodyH), width, height, true)
}

func relaxCenterLine(line string, width int) string {
	pad := (width - lipgloss.Width(line)) / 2
	if pad <= 0 {
		return line
	}
	return strings.Repeat(" ", pad) + line
}

// relaxCenterBlock centraliza o desenho como bloco — padding igual em todas as
// linhas, senão o cisalhamento 3D do cubo se desfaz.
func relaxCenterBlock(lines []string, width int) []string {
	widest := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > widest {
			widest = w
		}
	}
	pad := (width - widest) / 2
	if pad <= 0 {
		return lines
	}
	indent := strings.Repeat(" ", pad)
	out := make([]string, len(lines))
	for i, l := range lines {
		if l == "" {
			out[i] = ""
			continue
		}
		out[i] = indent + l
	}
	return out
}

// relaxHash gera pseudo-aleatório determinístico por frame (sem estado).
func relaxHash(vals ...int) int {
	h := 2166136261
	for _, v := range vals {
		h = (h ^ v) * 16777619
	}
	if h < 0 {
		h = -h
	}
	return h
}
