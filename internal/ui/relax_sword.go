package ui

import (
	"math"
	"math/rand"
)

// ── A espada na pedra ─────────────────────────────────────────────────────────
//
// Quadro parado de propósito: a lâmina cravada na pedra, o morro atrás, o céu
// no fim da tarde. Nada disso se mexe — a única coisa viva é o capim, e mesmo
// ele só anda em rajada: as folhas deitam todas juntas, voltam, e o campo fica
// ~3s em silêncio até a próxima. É a pausa que faz a cena; capim balançando
// sem parar viraria só mais um fundo animado.
//
// Tudo cai no mesmo buffer Braille de paleta indexada (voto de maioria): aço,
// ouro, pedra e capim têm cores que não podem virar média entre si. Como b.set
// respeita quem acendeu o ponto primeiro, o desenho vai da FRENTE pro FUNDO —
// capim da frente, pedra, espada, capim do fundo, chão, morro, céu. A pedra
// antes da espada é o que enterra a ponta.

// O desenho é fixo: espada cravada e a pedra em volta. Vira bitmap de
// subpontos uma vez e encolhe pro tamanho do palco — nada aqui se anima.
var relaxSwordArt = []string{
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣴⣿⡿⣿⢦⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠻⣿⣧⣽⢼⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢷⣬⡟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣿⢿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⡼⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⡿⢷⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⢹⣇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⡇⣹⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⣴⡶⠞⠷⠟⣻⣿⣿⣿⠟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠛⠛⢶⣿⣭⢽⣿⡿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⣯⡳⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⡇⠑⣌⠻⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢿⣿⣶⣌⣷⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢻⣬⡓⢾⡟⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣇⠉⠛⣷⣤⠀⠀⠀⠀⠀⠀⠀⣼⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⡄⢲⣾⣿⠀⠀⠀⠀⠀⠀⢸⢻⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⡇⠘⣖⢿⠀⠀⠀⠀⠀⠀⣏⢸⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⠀⢿⠻⡄⠀⠀⠀⠀⠀⢿⡀⠹⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⡆⠘⣶⣿⠀⠀⠀⠀⠀⠈⢣⡀⠹⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣷⠀⣿⡾⡆⠀⠀⠀⠀⠀⠈⢳⣶⣷⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⡀⢸⡿⣧⠀⠀⠀⢠⡄⢀⣾⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣧⠀⣿⢿⠀⠀⠀⢸⣇⣸⠛⢸⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⡀⢸⣿⣇⠀⢀⡜⢹⣧⡆⣾⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣿⡏⢘⣿⣿⢰⠏⢀⣹⣿⡇⡿⢰⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣾⠃⠀⠀⣷⣿⣾⣿⣿⣿⠂⣸⡟⢘⡿⠁⣼⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣸⣻⠀⠀⣴⠋⣀⣿⣿⣿⣿⢠⠿⠁⢸⡀⠀⣯⢿⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢰⡝⣿⠳⡄⠏⠀⣿⣿⢿⢿⠃⠀⡀⠀⠈⠷⡄⣿⠈⣷⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠰⣿⣧⣸⠃⠙⠀⡀⠛⠁⠘⠀⢤⣶⣿⣿⡛⠀⠙⡇⠀⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⡿⠃⠀⣠⠞⢧⠀⠀⠀⠀⢠⣿⣿⡟⠃⠀⠀⣿⠀⢈⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⠎⠀⣠⠞⠁⡀⠀⠳⣄⠀⣠⣿⣿⣋⠙⠁⠀⠀⠉⣤⡛⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⠀⡞⠁⣠⠞⠁⢀⡴⡇⠀⠀⢀⡴⠋⠻⠏⠀⠀⠀⢀⣀⣋⣀⠙⠳⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⡆⡇⠈⣡⣄⡠⠎⠁⠹⠶⠶⢾⣥⠀⠀⣠⡖⣃⣰⡾⣫⠟⠁⢠⠆⠹⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⣶⣿⣧⡞⠉⠉⢀⣠⡴⣲⡄⠀⠟⡏⠠⠾⢟⣋⣉⣴⡶⠋⠀⡰⣿⠀⠀⢳⣤⡤⠀⢀⡀⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣻⠿⢿⡿⠃⠀⣰⡿⠃⠀⡿⡷⠟⠁⠁⢠⣶⣿⣿⣿⣯⠀⢀⠞⢱⠃⠀⠀⠀⢵⡄⣀⠀⣷⡄⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⢀⣤⣞⣿⣯⡀⣤⢶⠂⠐⠚⠹⠁⣠⠜⠁⠀⠀⠀⠀⠘⣿⣿⡿⠋⢹⡄⣸⠀⣿⠀⣄⠀⠀⠀⠹⣯⡉⠉⠳⣴⡄⠀⠀⠀⠀",
	"⠀⠀⠀⠀⣠⡴⣯⣿⣿⣿⣿⣿⣿⣿⣿⣿⣗⠢⢤⣤⣴⠾⢿⡤⠀⠀⠀⣼⣷⣿⡤⠀⡇⠙⣶⣿⠶⠿⢿⣿⣦⣤⣼⣬⡧⠀⠀⢿⣀⠀⠀⠀",
	"⠀⢀⣴⣻⣷⣿⣏⣛⠛⢿⣿⣦⡀⠀⠈⠉⠻⣿⣿⣮⣿⣷⣾⣿⡀⢀⣾⣿⣿⠾⠛⣶⣿⠛⠛⠀⠀⠀⣿⣿⡿⢿⣽⠟⠁⠀⠀⠀⠙⢦⡀⠀",
	"⠀⣼⠟⣡⠚⣉⣉⣽⣿⣦⡙⢦⣹⠆⢀⣶⣆⣼⣿⣿⣟⠋⣽⣁⣿⣿⣾⣿⣿⣿⣿⠿⢿⣿⣿⣿⡿⠿⢿⣿⣿⣿⢧⣄⠀⢀⣠⣶⣤⣬⣧⠀",
	"⢸⢷⡾⠷⠾⠿⠿⡿⠿⣟⠛⠛⢀⢠⠾⠾⡟⠉⡉⡑⠿⠾⠓⠒⡾⣿⠻⠿⠟⠛⢋⢰⡾⡿⣶⠶⠿⠿⣿⢩⡿⡟⡳⢾⠿⠟⣛⣚⣿⠷⡶⠄",
}

var (
	relaxSwSkyGrad   = []string{"#0C1322", "#161F38", "#26304F", "#463F5E", "#7A5560", "#B8815F"}
	relaxSwHillGrad  = []string{"#232C42", "#1E2A34", "#1A2A25"}
	relaxSwGrassGrad = []string{"#0A120D", "#152B18", "#234A26", "#387434", "#5E9C44"}
	relaxSwRockGrad  = []string{"#0F0F14", "#1E1E26", "#2E2E38", "#41414D", "#565663", "#6E6E7A", "#8C8C97"}
	relaxSwSteelGrad = []string{"#232A33", "#3E4A58", "#67788A", "#94A6B8", "#C6D6E6", "#F4FAFF"}
	relaxSwGoldGrad  = []string{"#4A3208", "#8A6512", "#C9A02A", "#F5E3A0"}
)

const (
	relaxSwSkyN   = 9
	relaxSwHillN  = 3
	relaxSwGrassN = 5
	relaxSwRockN  = 7
	relaxSwSteelN = 6
	relaxSwGoldN  = 4
)

const (
	relaxSwSky   = 0
	relaxSwHill  = relaxSwSky + relaxSwSkyN
	relaxSwGrass = relaxSwHill + relaxSwHillN
	relaxSwRock  = relaxSwGrass + relaxSwGrassN
	relaxSwSteel = relaxSwRock + relaxSwRockN
	relaxSwGold  = relaxSwSteel + relaxSwSteelN
	relaxSwStar  = relaxSwGold + relaxSwGoldN
)

var relaxSwPal = func() []relaxColor {
	out := make([]relaxColor, relaxSwStar+1)
	copy(out[relaxSwSky:], relaxRamp(relaxSwSkyGrad, relaxSwSkyN))
	copy(out[relaxSwHill:], relaxRamp(relaxSwHillGrad, relaxSwHillN))
	copy(out[relaxSwGrass:], relaxRamp(relaxSwGrassGrad, relaxSwGrassN))
	copy(out[relaxSwRock:], relaxRamp(relaxSwRockGrad, relaxSwRockN))
	copy(out[relaxSwSteel:], relaxRamp(relaxSwSteelGrad, relaxSwSteelN))
	copy(out[relaxSwGold:], relaxRamp(relaxSwGoldGrad, relaxSwGoldN))
	out[relaxSwStar] = "#9FA8C4"
	return out
}()

// Passo da simulação é 100ms, então tick é décimo de segundo: a rajada dura
// ~1,6s e o campo fica ~3s parado depois dela.
const (
	relaxSwGustTicks = 16
	relaxSwRestTicks = 30
	relaxSwCycle     = relaxSwGustTicks + relaxSwRestTicks
	// Atraso da rajada de um lado ao outro do campo, em ticks. Pouco de
	// propósito: é pra tirar o ar de metrônomo, não pra virar onda viajando.
	relaxSwGustLag = 2.5
)

// relaxSwGust é o envelope da rajada num instante do ciclo: sobe rápido, larga
// devagar e volta exatamente a zero — folha que não descansa no lugar de
// origem faz o campo derivar pro lado ao longo dos minutos.
func relaxSwGust(ph float64) float64 {
	if ph < 0 || ph >= relaxSwGustTicks {
		return 0
	}
	u := ph / relaxSwGustTicks
	p := math.Pow(u, 0.72) // empurrão curto, soltura longa
	// O (1-u⁶) é o pouso: sem ele a folha ainda está andando no último passo e
	// trava no lugar, que é o jeito de fazer capim parecer sprite.
	return (math.Sin(math.Pi*p) + 0.36*math.Sin(6*math.Pi*p)*p*(1-p)) * (1 - math.Pow(u, 6))
}

const relaxSwBladeN = 170

type relaxSwLeaf struct {
	x     float64 // 0–1 no palco
	root  float64 // 0–1 dentro da faixa de chão: quem nasce embaixo está na frente
	h     float64 // altura em fração da altura do palco
	sway  float64 // deslocamento da ponta na rajada, em subpontos
	front bool
}

type relaxSwordState struct {
	inited bool
	tick   int
	phase  int     // posição dentro do ciclo rajada+silêncio
	gust   float64 // o quanto o campo está deitado agora (só pro status)
	leaves []relaxSwLeaf
	hills  [relaxSwHillN * 3]float64
	stars  []relaxSkyPt
}

func relaxSwNewLeaf(front bool) relaxSwLeaf {
	l := relaxSwLeaf{
		x:     rand.Float64(),
		front: front,
		root:  rand.Float64() * 0.32,
		h:     0.020 + rand.Float64()*0.040,
		sway:  1.6 + rand.Float64()*1.6,
	}
	if front {
		// O da frente nasce mais embaixo, é mais alto e deita mais: a
		// profundidade do capim vem da altura, não da cor.
		l.root = 0.32 + rand.Float64()*0.68
		l.h = 0.048 + rand.Float64()*0.070
		l.sway += 1.4
	}
	if rand.Intn(9) == 0 {
		l.h *= 1.7 // um em cada nove é mato, senão a faixa vira escova
	}
	return l
}

func stepRelaxSword(st *relaxSwordState) {
	if !st.inited {
		st.inited = true
		for i := range st.hills {
			st.hills[i] = rand.Float64() * 2 * math.Pi
		}
		for i, n := 0, 40+rand.Intn(20); i < n; i++ {
			st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.45})
		}
		for i := 0; i < relaxSwBladeN; i++ {
			st.leaves = append(st.leaves, relaxSwNewLeaf(i%5 < 2))
		}
		// Começa no silêncio: a cena abre parada e a primeira rajada chega.
		st.phase = relaxSwGustTicks + relaxSwRestTicks/2
	}
	st.tick++
	if st.phase++; st.phase >= relaxSwCycle {
		st.phase = 0
	}
	st.gust = relaxSwGust(float64(st.phase))
}

// ── Render ────────────────────────────────────────────────────────────────────

func relaxSwordFrames(st *relaxSwordState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxSword(st)
	}
	b := newRelaxBrailleVote(maxInt(16, minInt(width, 120)), maxInt(6, minInt(height, 34)))
	relaxSwordScene(st, b)

	status := "ninguém veio buscar ainda"
	switch {
	case st.gust > 0.55:
		status = "o vento deitou o capim inteiro"
	case st.gust > 0.05:
		status = "as folhas voltando ao lugar"
	case st.phase > relaxSwCycle-5:
		status = "vem outra"
	case st.tick%400 < 90:
		status = "a pedra segura desde sempre"
	}
	return b.lines(relaxStyles(relaxSwPal, fade)), StyleMuted.Render(status)
}

func relaxSwordScene(st *relaxSwordState, b *relaxBraille) {
	sw, sh := b.w*2, b.h*4
	gy := float64(sh) * 0.72 // linha do chão
	cx := float64(sw) * 0.48 // a espada fica um tico fora do centro
	// Palco apertado carrega menos capim: mantém a densidade, não a contagem.
	dens := clampF(float64(sw)/220, 0.45, 1)

	relaxSwDrawGrass(st, b, sw, sh, gy, dens, true)
	relaxSwDrawArt(b, sw, sh, gy, cx)
	relaxSwDrawGrass(st, b, sw, sh, gy, dens, false)

	// ── Chão ── fechado embaixo e esgarçado na linha do horizonte, senão a
	// faixa vira tarja e o capim parece colado num degrau.
	for y := int(gy); y < sh; y++ {
		f := (float64(y) - gy) / math.Max(1, float64(sh)-gy)
		lvl := relaxSwGrass
		if f > 0.55 {
			lvl++
		}
		for x := 0; x < sw; x++ {
			if relaxHalftone(x, y) < 1.15*f*f-0.05 && !b.taken(x, y) {
				b.set(x, y, lvl)
			}
		}
	}

	// ── Morros ── da frente pro fundo, cada cadeia morrendo na linha do chão.
	for d := relaxSwHillN - 1; d >= 0; d-- {
		far := float64(relaxSwHillN-1-d) / float64(relaxSwHillN-1)
		amp := gy * (0.04 + 0.09*far)
		k := d * 3
		for x := 0; x < sw; x++ {
			u := float64(x) / float64(sw) * 2 * math.Pi
			v := gy - amp*(math.Abs(math.Sin(u*1.4+st.hills[k]))*0.85+
				0.40*math.Abs(math.Sin(u*3.1+st.hills[k+1]))+
				0.18*math.Sin(u*5.9+st.hills[k+2]))
			for y := int(v); y < int(gy); y++ {
				if !b.taken(x, y) {
					b.set(x, y, relaxSwHill+d)
				}
			}
		}
	}

	// ── Estrelas ── só as de cima; embaixo o céu ainda tem luz do dia.
	for i, p := range st.stars {
		if float64(i) > float64(len(st.stars))*dens {
			break
		}
		x, y := int(p.x*float64(sw-1)), int(p.y*gy)
		if relaxHalftone(x, y) < 0.42 && !b.taken(x, y) {
			b.set(x, y, relaxSwStar)
		}
	}

	// ── Céu ── meio-tom vazando de cima pra baixo: raro no alto, cheio no
	// horizonte, onde ainda queima o fim de tarde.
	for y := 0; y < int(gy); y++ {
		fy := float64(y) / gy
		lvl := relaxSwSky + minInt(int(fy*fy*float64(relaxSwSkyN)*1.15), relaxSwSkyN-1)
		for x := 0; x < sw; x++ {
			if relaxHalftone(x, y) <= 0.06+0.78*fy*fy && !b.taken(x, y) {
				b.set(x, y, lvl)
			}
		}
	}
}

// relaxSwDrawGrass desenha uma camada de capim. A folha é uma curva: quase reta
// na base e cada vez mais deitada na ponta — folha que balança inteira parece
// limpador de para-brisa.
func relaxSwDrawGrass(st *relaxSwordState, b *relaxBraille, sw, sh int, gy, dens float64, front bool) {
	band := float64(sh) - gy
	for i, l := range st.leaves {
		if l.front != front || float64(i) > float64(len(st.leaves))*dens {
			continue
		}
		// A rajada chega de um lado e atravessa o campo em 0,25s: perto do
		// "todas de uma vez", longe do metrônomo.
		ph := math.Mod(float64(st.phase)-l.x*relaxSwGustLag+relaxSwCycle, relaxSwCycle)
		bend := l.sway * relaxSwGust(ph)

		x0 := l.x * float64(sw-1)
		y0 := gy + l.root*band
		n := maxInt(2, int(l.h*float64(sh)))
		lvl := relaxSwGrass + 1
		if front {
			lvl++
		}
		for j := 0; j <= n; j++ {
			f := float64(j) / float64(n)
			ll := lvl
			if f > 0.62 {
				ll++ // a ponta pega a luz que a base não pega
			}
			b.set(int(x0+bend*f*f*1.7), int(y0)-j, minInt(ll, relaxSwGrass+relaxSwGrassN-1))
		}
	}
}

type relaxSwSprite struct {
	w, h int // em subpontos
	dots []bool
	den  []float32 // fração de pontos acesos em volta; o desenho não muda, isto sai uma vez
}

var (
	relaxSwArtDots, relaxSwArtW, relaxSwArtH = relaxArtDots(relaxSwordArt)
	relaxSwCache                             = map[[2]int]*relaxSwSprite{}
)

// relaxSwArtAt devolve o desenho no maior tamanho que cabe em maxW×maxH
// subpontos, sempre na proporção original. O ponto do destino acende se um
// quarto do retângulo que ele cobre na origem estiver aceso: limiar baixo de
// propósito, senão o fio da lâmina some quando o desenho encolhe.
//
// Terminal não muda de tamanho a cada frame, então cada tamanho é resolvido
// uma vez e fica guardado.
func relaxSwArtAt(maxW, maxH int) *relaxSwSprite {
	key := [2]int{maxW, maxH}
	if sp, ok := relaxSwCache[key]; ok {
		return sp
	}
	scale := math.Min(1, math.Min(
		float64(maxW)/float64(relaxSwArtW), float64(maxH)/float64(relaxSwArtH)))
	w := maxInt(2, int(float64(relaxSwArtW)*scale))
	h := maxInt(2, int(float64(relaxSwArtH)*scale))
	dots := make([]bool, w*h)
	for y := 0; y < h; y++ {
		sy0 := y * relaxSwArtH / h
		sy1 := maxInt(sy0+1, (y+1)*relaxSwArtH/h)
		for x := 0; x < w; x++ {
			sx0 := x * relaxSwArtW / w
			sx1 := maxInt(sx0+1, (x+1)*relaxSwArtW/w)
			on, total := 0, 0
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					total++
					if relaxSwArtDots[sy*relaxSwArtW+sx] {
						on++
					}
				}
			}
			dots[y*w+x] = float64(on) >= 0.26*float64(total)
		}
	}
	// Densidade em volta de cada ponto. É ela que devolve o volume: traço
	// isolado tem densidade baixa, miolo de bloco tem alta, e a diferença
	// entre um ponto e o vizinho na direção da luz vira face iluminada ou
	// sombra. Mesma conta dos cúmulos do Mar.
	den := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on, total := 0, 0
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					total++
					if nx, ny := x+dx, y+dy; nx >= 0 && ny >= 0 && nx < w && ny < h && dots[ny*w+nx] {
						on++
					}
				}
			}
			den[y*w+x] = float32(on) / float32(total)
		}
	}

	sp := &relaxSwSprite{w: w, h: h, dots: dots, den: den}
	relaxSwCache[key] = sp
	return sp
}

// Onde o desenho deixa de ser espada e vira pedra. Os dois números saem do
// bitmap, não do olho: nas linhas do meio o desenho tem DOIS blocos separados
// por um vão — a lâmina termina na coluna ~53 e o pilar de pedra começa na ~57
// —, e da linha 24 pra baixo é tudo maciço. Cabo e guarda são o topo.
const (
	relaxSwHilt = 0.275 // fração da altura: cabo e guarda
	relaxSwEdge = 0.53  // fração da largura: à direita disso é pedra
	relaxSwBase = 0.56  // fração da altura em que a lâmina entra na pedra
)

func relaxSwDrawArt(b *relaxBraille, sw, sh int, gy, cx float64) {
	base := gy + float64(sh)*0.05 // o pé do desenho entra um tanto no chão
	sp := relaxSwArtAt(int(float64(sw)*0.60), int(base-float64(sh)*0.03))
	x0, y0 := int(cx)-sp.w/2, int(base)-sp.h

	var cells [][2]int
	for y := 0; y < sp.h; y++ {
		fy := float64(y) / float64(sp.h-1)
		for x := 0; x < sp.w; x++ {
			if !sp.dots[y*sp.w+x] {
				continue
			}
			px, py := x0+x, y0+y
			fx := float64(x) / float64(sp.w-1)
			// A pedra come a lâmina em meio-tom numa faixa curta: linha reta de
			// corte apareceria como degrau atravessando o aço.
			stone := fx >= relaxSwEdge || relaxHalftone(px, py) < clamp01((fy-relaxSwBase)/0.10)

			// Relevo: densidade daqui contra a de dois pontos na direção da luz,
			// que vem de cima à esquerda. Face virada pra luz clareia, a de trás
			// apaga — é o que tira o chapado sem precisar de sombra desenhada.
			d, toward := float64(sp.den[y*sp.w+x]), 0.0
			if nx, ny := x-2, y-2; nx >= 0 && ny >= 0 {
				toward = float64(sp.den[ny*sp.w+nx])
			}
			// Pedra é fosca, granulada e quebrada; aço é liso e reflete. Daí os
			// três pesos diferentes: a pedra pega relevo duro e grão grosso, o
			// aço pega meia-luz macia e quase nenhum grão.
			base, emboss, grain := 0.40, 0.58, 0.06
			if stone {
				base, emboss, grain = 0.30, 0.82, 0.20
			}
			lum := clamp01(base + 0.30*d + emboss*(d-toward) -
				0.22*fy + 0.14*(1-fx) + grain*(relaxHalftone(px, py)-0.5))

			var lvl int
			switch {
			case fy < relaxSwHilt:
				lvl = relaxSwGold + relaxSwLvl(lum, relaxSwGoldN)
			case stone:
				lvl = relaxSwRock + relaxSwLvl(lum, relaxSwRockN)
			default:
				lvl = relaxSwSteel + relaxSwLvl(lum, relaxSwSteelN)
			}
			b.set(px, py, lvl)
			cells = append(cells, [2]int{px / 2, py / 4})
		}
	}
	// O desenho é traço fino: sem congelar a célula, o céu e o morro que vêm
	// depois enchem os mesmos oito pontos e o voto entrega a espada pro fundo.
	for _, c := range cells {
		b.lock(c[0], c[1])
	}
}

func relaxSwLvl(lum float64, n int) int {
	return minInt(maxInt(int(lum*float64(n)), 0), n-1)
}
