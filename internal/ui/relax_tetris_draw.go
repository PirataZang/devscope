package ui

// ── Desenho do Tetris ─────────────────────────────────────────────────────────
//
// Em Braille cada peça é um bloco de 6×4 subpixels com bisel: aresta clara em
// cima e à esquerda, escura embaixo e à direita. É o que dá volume — o "██"
// anterior era chapado e as peças encostadas viravam uma mancha só.

var relaxTetStops = [7][]string{
	{"#0E3648", "#2E7EA0", "#7ED3EE"}, // I
	{"#4A3410", "#B08A4A", "#F2D79A"}, // O
	{"#331E4A", "#8A6BA8", "#D3BCEE"}, // T
	{"#123A22", "#5B9A72", "#A9E5C0"}, // S
	{"#4A1E1E", "#A85E5E", "#F0AFAF"}, // Z
	{"#1C2450", "#5E6FA8", "#B4C0F0"}, // J
	{"#4A2A0E", "#B07A45", "#F2C793"}, // L
}

const (
	relaxTetShades = 3
	relaxTetWall   = 7 * relaxTetShades
	relaxTetGhost  = relaxTetWall + 1
	relaxTetBW     = 6 // largura do bloco em subpixels
	relaxTetBH     = 4
)

var relaxTetRamp = func() []relaxColor {
	out := make([]relaxColor, relaxTetGhost+1)
	for i, stops := range relaxTetStops {
		copy(out[i*relaxTetShades:], relaxRamp(stops, relaxTetShades))
	}
	out[relaxTetWall] = "#3A4152"
	out[relaxTetGhost] = "#2B3040"
	return out
}()

// relaxTetBlock desenha um bloco com bisel. shade desloca a face inteira, o que
// serve pro clarão da linha saindo.
func relaxTetBlock(b *relaxBraille, x0, y0, kind int, dim bool) {
	base := kind * relaxTetShades
	face, light, dark := base+1, base+2, base
	if dim {
		face, light, dark = base, base+1, base
	}
	for dy := 0; dy < relaxTetBH; dy++ {
		for dx := 0; dx < relaxTetBW; dx++ {
			lvl := face
			switch {
			case dy == 0 || dx == 0:
				lvl = light
			case dy == relaxTetBH-1 || dx >= relaxTetBW-1:
				lvl = dark
			}
			b.set(x0+dx, y0+dy, lvl)
		}
	}
}
