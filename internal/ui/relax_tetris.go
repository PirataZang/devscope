package ui

import (
	"fmt"
	"math/rand"
)

// ── Tetris automático ─────────────────────────────────────────────────────────
//
// Ninguém joga: uma heurística simples escolhe coluna e rotação, e a peça é
// levada até lá com ritmo lento — dá pra acompanhar cada encaixe. Linha cheia
// dissolve do meio pras bordas antes de o tabuleiro assentar.

const (
	relaxTetW    = 10
	relaxTetH    = 20 // altura máxima; a real vem do espaço do palco (st.h)
	relaxTetHMin = 8
	relaxTetGrav = 4 // frames por linha de queda
	relaxTetMove = 2 // frames por rotação/passo lateral
)

type relaxTetrisState struct {
	inited bool
	tick   int

	h    int                        // linhas jogáveis, ajustadas ao palco
	grid [relaxTetH][relaxTetW]int8 // 0 vazio, senão índice de cor + 1

	kind, rot, x, y int
	wantX, wantRot  int
	next            int
	spawned         bool

	clearing []int
	clearT   int
	pause    int
	wipe     int // limpeza do tabuleiro depois de encher

	score, lines int
}

// Peças em coordenadas; a rotação é calculada, evita tabela de 28 formas.
var relaxTetPieces = [7][4][2]int{
	{{0, 1}, {1, 1}, {2, 1}, {3, 1}}, // I
	{{1, 0}, {2, 0}, {1, 1}, {2, 1}}, // O
	{{1, 0}, {0, 1}, {1, 1}, {2, 1}}, // T
	{{1, 0}, {2, 0}, {0, 1}, {1, 1}}, // S
	{{0, 0}, {1, 0}, {1, 1}, {2, 1}}, // Z
	{{0, 0}, {0, 1}, {1, 1}, {2, 1}}, // J
	{{2, 0}, {0, 1}, {1, 1}, {2, 1}}, // L
}

// relaxTetShape gira a peça r vezes (90° CW) e normaliza pro canto.
func relaxTetShape(kind, r int) [4][2]int {
	s := relaxTetPieces[kind]
	for i := 0; i < r%4; i++ {
		for j := range s {
			s[j][0], s[j][1] = -s[j][1], s[j][0]
		}
	}
	minX, minY := s[0][0], s[0][1]
	for _, c := range s {
		minX, minY = minInt(minX, c[0]), minInt(minY, c[1])
	}
	for j := range s {
		s[j][0] -= minX
		s[j][1] -= minY
	}
	return s
}

func relaxTetFits(g *[relaxTetH][relaxTetW]int8, h, kind, r, px, py int) bool {
	for _, c := range relaxTetShape(kind, r) {
		x, y := px+c[0], py+c[1]
		if x < 0 || x >= relaxTetW || y >= h {
			return false
		}
		if y >= 0 && g[y][x] != 0 {
			return false
		}
	}
	return true
}

// relaxTetPlan avalia toda rotação × coluna e escolhe a menos ruim. Heurística
// clássica (altura, buracos, irregularidade, linhas) — não precisa jogar bem,
// precisa jogar de um jeito que dê gosto de olhar.
func relaxTetPlan(st *relaxTetrisState) {
	best := -1e18
	st.wantX, st.wantRot = st.x, st.rot
	for r := 0; r < 4; r++ {
		for x := -1; x < relaxTetW; x++ {
			if !relaxTetFits(&st.grid, st.h, st.kind, r, x, 0) {
				continue
			}
			y := 0
			for relaxTetFits(&st.grid, st.h, st.kind, r, x, y+1) {
				y++
			}
			g := st.grid
			for _, c := range relaxTetShape(st.kind, r) {
				if yy := y + c[1]; yy >= 0 {
					g[yy][x+c[0]] = int8(st.kind + 1)
				}
			}
			if s := relaxTetScore(&g, st.h); s > best {
				best, st.wantX, st.wantRot = s, x, r
			}
		}
	}
}

func relaxTetScore(g *[relaxTetH][relaxTetW]int8, h int) float64 {
	heights := [relaxTetW]int{}
	holes, total := 0, 0
	for x := 0; x < relaxTetW; x++ {
		seen := false
		for y := 0; y < h; y++ {
			if g[y][x] != 0 {
				if !seen {
					seen = true
					heights[x] = h - y
				}
			} else if seen {
				holes++
			}
		}
		total += heights[x]
	}
	bump := 0
	for x := 0; x+1 < relaxTetW; x++ {
		bump += absInt(heights[x] - heights[x+1])
	}
	full := 0
	for y := 0; y < h; y++ {
		cheia := true
		for x := 0; x < relaxTetW; x++ {
			if g[y][x] == 0 {
				cheia = false
				break
			}
		}
		if cheia {
			full++
		}
	}
	return -0.51*float64(total) + 0.76*float64(full) - 0.36*float64(holes) - 0.18*float64(bump)
}

func relaxTetSpawn(st *relaxTetrisState) {
	st.kind = st.next
	st.next = rand.Intn(7)
	st.rot = 0
	st.x = relaxTetW/2 - 2
	st.y = -1
	st.spawned = true
	if !relaxTetFits(&st.grid, st.h, st.kind, st.rot, st.x, st.y) {
		st.wipe = st.h // encheu: limpa devagar e recomeça
		st.spawned = false
		return
	}
	relaxTetPlan(st)
}

func stepRelaxTetris(st *relaxTetrisState) {
	if !st.inited {
		st.inited = true
		if st.h == 0 {
			st.h = relaxTetH
		}
		st.next = rand.Intn(7)
		relaxTetSpawn(st)
	}
	st.tick++

	if st.wipe > 0 { // tabuleiro sumindo de baixo pra cima
		if st.tick%3 == 0 {
			st.wipe--
			st.grid[st.wipe] = [relaxTetW]int8{}
			if st.wipe == 0 {
				st.score, st.lines = 0, 0
				relaxTetSpawn(st)
			}
		}
		return
	}
	if len(st.clearing) > 0 {
		if st.clearT++; st.clearT > 10 {
			relaxTetCollapse(st)
		}
		return
	}
	if st.pause > 0 {
		st.pause--
		return
	}
	if !st.spawned {
		relaxTetSpawn(st)
		return
	}

	// Alinha rotação e coluna antes de deixar cair — um passo por vez.
	if st.tick%relaxTetMove == 0 {
		switch {
		case st.rot != st.wantRot:
			if r := (st.rot + 1) % 4; relaxTetFits(&st.grid, st.h, st.kind, r, st.x, st.y) {
				st.rot = r
			} else {
				st.wantRot = st.rot
			}
		case st.x != st.wantX:
			dx := 1
			if st.wantX < st.x {
				dx = -1
			}
			if relaxTetFits(&st.grid, st.h, st.kind, st.rot, st.x+dx, st.y) {
				st.x += dx
			} else {
				st.wantX = st.x
			}
		}
	}

	if st.tick%relaxTetGrav != 0 {
		return
	}
	if relaxTetFits(&st.grid, st.h, st.kind, st.rot, st.x, st.y+1) {
		st.y++
		return
	}
	for _, c := range relaxTetShape(st.kind, st.rot) {
		if y := st.y + c[1]; y >= 0 {
			st.grid[y][st.x+c[0]] = int8(st.kind + 1)
		}
	}
	st.spawned = false
	st.pause = 6
	st.clearing = st.clearing[:0]
	for y := 0; y < st.h; y++ {
		cheia := true
		for x := 0; x < relaxTetW; x++ {
			if st.grid[y][x] == 0 {
				cheia = false
				break
			}
		}
		if cheia {
			st.clearing = append(st.clearing, y)
		}
	}
	st.clearT = 0
}

func relaxTetCollapse(st *relaxTetrisState) {
	for _, row := range st.clearing {
		for y := row; y > 0; y-- {
			st.grid[y] = st.grid[y-1]
		}
		st.grid[0] = [relaxTetW]int8{}
	}
	st.lines += len(st.clearing)
	st.score += []int{0, 100, 300, 500, 800}[minInt(len(st.clearing), 4)]
	st.clearing = st.clearing[:0]
	st.clearT = 0
	st.pause = 8
}

// relaxTetResize ajusta o tabuleiro ao palco. Antes o render cortava as linhas
// que não cabiam — e cortava por cima, justo onde a peça nasce, então em
// terminal baixo o jogo aparecia decapitado. Agora o tabuleiro é do tamanho que
// cabe e a peça sempre entra dentro do quadro.
// relaxTetResize ajusta o tabuleiro ao palco. Antes o render cortava as linhas
// que não cabiam — e cortava por cima, justo onde a peça nasce, então em
// terminal baixo o jogo aparecia decapitado. Agora o tabuleiro é do tamanho que
// cabe e a peça sempre entra dentro do quadro.
func relaxTetResize(st *relaxTetrisState, rows int) {
	rows = maxInt(relaxTetHMin, minInt(rows, relaxTetH))
	if st.h == rows {
		return
	}
	*st = relaxTetrisState{h: rows}
	stepRelaxTetris(st)
}

// relaxTetGhostY é onde a peça vai parar se cair agora. Desenhada em contorno
// apagado, é o que deixa acompanhar a intenção da IA antes de a peça descer.
func relaxTetGhostY(st *relaxTetrisState) int {
	y := st.y
	for relaxTetFits(&st.grid, st.h, st.kind, st.rot, st.x, y+1) {
		y++
	}
	return y
}

func relaxTetrisFrames(st *relaxTetrisState, width, height int, fade float64) ([]string, string) {
	w := maxInt(24, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	// Duas linhas de célula para a moldura, e cada linha do tabuleiro ocupa
	// relaxTetBH subpixels.
	relaxTetResize(st, (h*4-2*relaxTetBH)/relaxTetBH)
	if !st.inited {
		stepRelaxTetris(st)
	}
	b := newRelaxBrailleVote(w, h)

	boardW := relaxTetW * relaxTetBW
	boardH := st.h * relaxTetBH
	x0 := (w*2 - boardW) / 2
	y0 := maxInt(0, (h*4-boardH)/2)

	// Poço: parede fina dos dois lados e fundo, aberto em cima como no jogo.
	for y := y0 - 1; y < y0+boardH+2; y++ {
		b.set(x0-2, y, relaxTetWall)
		b.set(x0+boardW+1, y, relaxTetWall)
	}
	for x := x0 - 2; x <= x0+boardW+1; x++ {
		b.set(x, y0+boardH+1, relaxTetWall)
	}

	clearing := map[int]bool{}
	for _, y := range st.clearing {
		clearing[y] = true
	}

	// Sombra do destino, antes das peças pra não cobrir nada.
	if st.spawned && len(st.clearing) == 0 {
		gy := relaxTetGhostY(st)
		for _, c := range relaxTetShape(st.kind, st.rot) {
			if y := gy + c[1]; y >= 0 {
				// Bloco cheio e apagado, não contorno: a célula tem 8 pontos e
				// um contorno de um subpixel perde o voto de cor pro vizinho.
				gx := x0 + (st.x+c[0])*relaxTetBW
				py := y0 + y*relaxTetBH
				for dy := 0; dy < relaxTetBH; dy++ {
					for dx := 0; dx < relaxTetBW; dx++ {
						b.set(gx+dx, py+dy, relaxTetGhost)
					}
				}
			}
		}
	}

	for y := 0; y < st.h; y++ {
		for x := 0; x < relaxTetW; x++ {
			cell := st.grid[y][x]
			if cell == 0 {
				if !relaxTetCurrentAt(st, x, y) {
					continue
				}
				cell = int8(st.kind + 1)
			}
			if clearing[y] {
				// Dissolve do meio pras bordas.
				if edge := absInt(x*2-relaxTetW+1) + 1; st.clearT*2 > relaxTetW-edge {
					continue
				}
				relaxTetBlock(b, x0+x*relaxTetBW, y0+y*relaxTetBH, int(cell-1)%7, true)
				continue
			}
			relaxTetBlock(b, x0+x*relaxTetBW, y0+y*relaxTetBH, int(cell-1)%7, false)
		}
	}

	// Próxima peça, encostada na parede direita.
	nx := x0 + boardW + 6
	if nx+4*relaxTetBW < w*2 {
		for _, c := range relaxTetShape(st.next, 0) {
			relaxTetBlock(b, nx+c[0]*relaxTetBW, y0+2+c[1]*relaxTetBH, st.next, true)
		}
	}

	status := fmt.Sprintf("%d pontos · %d linhas", st.score, st.lines)
	if st.wipe > 0 {
		status = "tabuleiro cheio, recomeçando"
	} else if len(st.clearing) > 0 {
		status = fmt.Sprintf("%d linha(s) saindo", len(st.clearing))
	}
	return b.lines(relaxStyles(relaxTetRamp, fade)), StyleMuted.Render(status)
}

func relaxTetCurrentAt(st *relaxTetrisState, x, y int) bool {
	if !st.spawned {
		return false
	}
	for _, c := range relaxTetShape(st.kind, st.rot) {
		if st.x+c[0] == x && st.y+c[1] == y {
			return true
		}
	}
	return false
}
