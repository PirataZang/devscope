package ui

import (
	"fmt"
	"math"
	"math/rand"
)

// ── Space Invaders ────────────────────────────────────────────────────────────
//
// Uma partida acontecendo sozinha, desenhada em Braille: os sprites são os
// bitmaps clássicos, um subpixel por pixel, então a lula, o caranguejo e o polvo
// aparecem como no fliperama em vez de virarem dois caracteres.
//
// Toda a simulação está em subpixels do palco (não em células), o que faz o
// tiro, a erosão dos escudos e a colisão trabalharem na mesma grade do desenho.
// Mudou o tamanho do terminal, a onda recomeça no tamanho novo.

// relaxInvSprite: bitmap com uma linha por string, '#' = pixel aceso.
type relaxInvSprite struct {
	w, h int
	rows []uint32
}

func relaxInvParse(art []string) relaxInvSprite {
	s := relaxInvSprite{h: len(art)}
	for _, line := range art {
		if len(line) > s.w {
			s.w = len(line)
		}
		var bits uint32
		for i, c := range line {
			if c == '#' {
				bits |= 1 << uint(i)
			}
		}
		s.rows = append(s.rows, bits)
	}
	return s
}

// Duas poses por bicho: a frota inteira alterna e é isso que dá a marcha.
var relaxInvAliens = [3][2]relaxInvSprite{
	{ // lula
		relaxInvParse([]string{"...##...", "..####..", ".######.", "##.##.##", "########", "..#..#..", ".#.##.#.", "#.#..#.#"}),
		relaxInvParse([]string{"...##...", "..####..", ".######.", "##.##.##", "########", ".#.##.#.", "#......#", ".#....#."}),
	},
	{ // caranguejo
		relaxInvParse([]string{"..#.....#..", "...#...#...", "..#######..", ".##.###.##.", "###########", "#.#######.#", "#.#.....#.#", "...##.##..."}),
		relaxInvParse([]string{"..#.....#..", "#..#...#..#", "#.#######.#", "###.###.###", "###########", ".#########.", "..#.....#..", ".#.......#."}),
	},
	{ // polvo
		relaxInvParse([]string{"....####....", ".##########.", "############", "###..##..###", "############", "...##..##...", "..##.##.##..", "##........##"}),
		relaxInvParse([]string{"....####....", ".##########.", "############", "###..##..###", "############", "..###..###..", ".##......##.", "..##....##.."}),
	},
}

var relaxInvShip = relaxInvParse([]string{
	"......#......", ".....###.....", ".....###.....",
	".###########.", "#############", "#############", "#############",
})

var relaxInvUfo = relaxInvParse([]string{
	"....########....", "..############..", ".##############.",
	"################", "..##.##.##.##...",
})

var relaxInvBunkerArt = relaxInvParse([]string{
	"...############...", ".################.", "##################",
	"##################", "##################", "##################",
	"#####........#####", "####..........####", "###............###",
	"###............###",
})

// Um nível de cor por entidade: paleta indexada, cor por voto de maioria.
const (
	relaxInvLvlSquid = iota
	relaxInvLvlCrab
	relaxInvLvlOcto
	relaxInvLvlShip
	relaxInvLvlBunker
	relaxInvLvlShot
	relaxInvLvlBomb
	relaxInvLvlUfo
	relaxInvLvlBoom
	relaxInvLvlStar
)

var relaxInvRamp = []relaxColor{
	"#3DD6C0", "#E86AA8", "#C9C24A", "#5FD36A", "#3F9E52",
	"#EAF2F0", "#E2604A", "#E24A6A", "#F2C14A", "#2E3644",
}

const (
	relaxInvRowH  = 15 // espaçamento vertical entre fileiras, em subpixels
	relaxInvColW  = 17
	relaxInvRows  = 3
	relaxInvSprH  = 8
	relaxInvSpeed = 2.0 // subpixels por passo
)

type relaxAlien struct {
	col, row int
	alive    bool
	death    float64
}

type relaxInvShot struct {
	x, y  float64
	vy    float64
	alien bool
}

type relaxInvBunker struct {
	x, y float64
	rows []uint32
}

type relaxInvadersState struct {
	inited bool
	tick   int
	sw, sh int

	aliens []relaxAlien
	cols   int
	fleetX float64
	fleetY float64
	dir    float64
	speed  float64

	bunkers []relaxInvBunker
	stars   []relaxSkyPt

	shipX, shipTarget float64
	shipCD            int
	shipHit           int

	shots []relaxInvShot
	parts []relaxSpark

	ufoX, ufoDir float64
	ufoOn        bool
	nextUfo      int

	wave      int
	waveBreak int
	score     int
}

func relaxInvNewWave(st *relaxInvadersState) {
	st.wave++
	st.cols = maxInt(4, minInt(7, (st.sw-24)/relaxInvColW))
	st.aliens = st.aliens[:0]
	for r := 0; r < relaxInvRows; r++ {
		for c := 0; c < st.cols; c++ {
			st.aliens = append(st.aliens, relaxAlien{col: c, row: r, alive: true})
		}
	}
	st.fleetX = float64(st.sw-st.cols*relaxInvColW) / 2
	st.fleetY = 6
	st.dir = 1
	st.speed = relaxInvSpeed * (0.8 + 0.12*float64(minInt(st.wave, 4)))
	st.shots = st.shots[:0]
	st.waveBreak = 0
}

// relaxInvResize refaz a cena no tamanho do palco. A simulação inteira mora em
// subpixels, então mudar de tamanho é começar outra onda, não reescalar tudo.
func relaxInvResize(st *relaxInvadersState, sw, sh int) {
	if st.inited && st.sw == sw && st.sh == sh {
		return
	}
	wave, score := st.wave, st.score
	*st = relaxInvadersState{inited: true, sw: sw, sh: sh, wave: wave, score: score}
	st.shipX = float64(sw) / 2
	st.shipTarget = st.shipX

	// Escudos: quatro, entre a frota e a nave.
	by := float64(sh) - 30
	for i := 0; i < 4; i++ {
		b := relaxInvBunker{x: float64(sw)*(float64(i)+0.5)/4 - float64(relaxInvBunkerArt.w)/2, y: by}
		b.rows = append(b.rows, relaxInvBunkerArt.rows...)
		st.bunkers = append(st.bunkers, b)
	}
	for i, n := 0, 26+rand.Intn(18); i < n; i++ {
		st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.75})
	}
	st.nextUfo = 120 + rand.Intn(200)
	relaxInvNewWave(st)
}

func relaxInvPos(st *relaxInvadersState, a relaxAlien) (float64, float64) {
	return st.fleetX + float64(a.col*relaxInvColW), st.fleetY + float64(a.row*relaxInvRowH)
}

func relaxInvAlive(st *relaxInvadersState) int {
	n := 0
	for _, a := range st.aliens {
		if a.alive {
			n++
		}
	}
	return n
}

// relaxInvErode abre um buraco no escudo. Devolve true se acertou algo.
func relaxInvErode(st *relaxInvadersState, x, y float64, r int) bool {
	for i := range st.bunkers {
		b := &st.bunkers[i]
		bx, by := int(x-b.x), int(y-b.y)
		if bx < -r || by < -r || bx > relaxInvBunkerArt.w+r || by > len(b.rows)+r {
			continue
		}
		if bx >= 0 && by >= 0 && bx < relaxInvBunkerArt.w && by < len(b.rows) && b.rows[by]&(1<<uint(bx)) == 0 {
			continue // passou por um buraco que já existia
		}
		hit := false
		for dy := -r; dy <= r; dy++ {
			ry := by + dy
			if ry < 0 || ry >= len(b.rows) {
				continue
			}
			for dx := -r; dx <= r; dx++ {
				rx := bx + dx
				if rx < 0 || rx >= relaxInvBunkerArt.w || dx*dx+dy*dy > r*r {
					continue
				}
				if m := uint32(1) << uint(rx); b.rows[ry]&m != 0 {
					b.rows[ry] &^= m
					hit = true
				}
			}
		}
		if hit {
			return true
		}
	}
	return false
}

func stepRelaxInvaders(st *relaxInvadersState) {
	if !st.inited {
		relaxInvResize(st, 160, 88)
	}
	st.tick++

	if st.waveBreak > 0 {
		if st.waveBreak--; st.waveBreak == 0 {
			relaxInvNewWave(st)
		}
		relaxInvStepParts(st)
		return
	}

	// Frota: anda de lado e desce um degrau ao encostar na borda.
	live := relaxInvAlive(st)
	if live > 0 {
		lo, hi := math.MaxFloat64, -math.MaxFloat64
		for _, a := range st.aliens {
			if !a.alive {
				continue
			}
			x, _ := relaxInvPos(st, a)
			lo, hi = math.Min(lo, x), math.Max(hi, x+12)
		}
		// Quanto menos bicho sobra, mais rápido eles andam — como no original.
		st.fleetX += st.dir * st.speed * (1 + 1.6*(1-float64(live)/float64(len(st.aliens))))
		if (st.dir > 0 && hi > float64(st.sw)-4) || (st.dir < 0 && lo < 4) {
			st.dir = -st.dir
			st.fleetY += 5
		}
		if st.fleetY+float64((relaxInvRows-1)*relaxInvRowH+relaxInvSprH) > float64(st.sh)-12 {
			st.waveBreak = 22 // a frota chegou embaixo: recomeça
		}
	} else if st.waveBreak == 0 {
		st.waveBreak = 20
	}

	// Nave: escolhe a coluna viva mais à mão e desliza até lá.
	if live > 0 && st.tick%7 == 0 {
		best, bestD := st.shipTarget, math.MaxFloat64
		for _, a := range st.aliens {
			if !a.alive {
				continue
			}
			x, _ := relaxInvPos(st, a)
			if d := math.Abs(x + 5 - st.shipX); d < bestD {
				best, bestD = x+5, d
			}
		}
		st.shipTarget = clampF(best, 8, float64(st.sw-8))
	}
	st.shipX = smoothDamp(st.shipX, st.shipTarget, 0.45, 0.1)

	if st.shipCD > 0 {
		st.shipCD--
	} else if live > 0 && math.Abs(st.shipX-st.shipTarget) < 3 {
		st.shots = append(st.shots, relaxInvShot{x: st.shipX, y: float64(st.sh) - 12, vy: -7})
		st.shipCD = 7 + rand.Intn(6)
	}
	if st.shipHit > 0 {
		st.shipHit--
	}

	// Bombas: só a fileira de baixo de cada coluna atira, e raramente.
	if live > 0 && rand.Intn(11) == 0 {
		var lowest *relaxAlien
		c := rand.Intn(st.cols)
		for i := range st.aliens {
			if a := &st.aliens[i]; a.alive && a.col == c && (lowest == nil || a.row > lowest.row) {
				lowest = a
			}
		}
		if lowest != nil {
			x, y := relaxInvPos(st, *lowest)
			st.shots = append(st.shots, relaxInvShot{x: x + 5, y: y + relaxInvSprH, vy: 3.4, alien: true})
		}
	}

	keep := st.shots[:0]
	for _, s := range st.shots {
		s.y += s.vy
		if s.y < 0 || s.y > float64(st.sh) {
			continue
		}
		if relaxInvErode(st, s.x, s.y, 3) {
			relaxInvBoom(st, s.x, s.y, 4)
			continue
		}
		if s.alien {
			if math.Abs(s.x-st.shipX) < 7 && s.y > float64(st.sh)-12 {
				st.shipHit = 12
				relaxInvBoom(st, st.shipX, float64(st.sh)-8, 10)
				continue
			}
			keep = append(keep, s)
			continue
		}
		hit := false
		for i := range st.aliens {
			a := &st.aliens[i]
			if !a.alive {
				continue
			}
			x, y := relaxInvPos(st, *a)
			if s.x >= x-1 && s.x <= x+12 && s.y >= y && s.y <= y+relaxInvSprH {
				a.alive, a.death = false, 1
				st.score += 10 * (relaxInvRows - a.row)
				relaxInvBoom(st, x+5, y+4, 8)
				hit = true
				break
			}
		}
		if !hit && st.ufoOn && s.y < 12 && math.Abs(s.x-st.ufoX) < 9 {
			st.ufoOn, hit = false, true
			st.score += 150
			relaxInvBoom(st, st.ufoX, 6, 14)
		}
		if !hit {
			keep = append(keep, s)
		}
	}
	st.shots = keep

	for i := range st.aliens {
		if a := &st.aliens[i]; !a.alive && a.death > 0 {
			a.death -= 0.16
		}
	}

	// OVNI: atravessa o topo de vez em quando, valendo bem mais.
	if st.ufoOn {
		st.ufoX += st.ufoDir * 1.7
		if st.ufoX < -18 || st.ufoX > float64(st.sw)+18 {
			st.ufoOn = false
		}
	} else if st.nextUfo--; st.nextUfo <= 0 {
		st.ufoOn, st.ufoDir = true, 1
		st.ufoX = -16
		if rand.Intn(2) == 0 {
			st.ufoDir, st.ufoX = -1, float64(st.sw)+16
		}
		st.nextUfo = 200 + rand.Intn(320)
	}
	relaxInvStepParts(st)
}

func relaxInvBoom(st *relaxInvadersState, x, y float64, n int) {
	for i := 0; i < n; i++ {
		a := rand.Float64() * 2 * math.Pi
		v := 1.2 + rand.Float64()*2.6
		st.parts = append(st.parts, relaxSpark{
			x: x, y: y, vx: math.Cos(a) * v, vy: math.Sin(a) * v,
			ttl: 5 + rand.Intn(6),
		})
	}
}

func relaxInvStepParts(st *relaxInvadersState) {
	keep := st.parts[:0]
	for _, p := range st.parts {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.25
		if p.life++; p.life < p.ttl {
			keep = append(keep, p)
		}
	}
	st.parts = keep
}

func relaxInvBlit(b *relaxBraille, s relaxInvSprite, x, y float64, lvl int) {
	ix, iy := int(math.Round(x)), int(math.Round(y))
	for r, bits := range s.rows {
		for c := 0; c < s.w; c++ {
			if bits&(1<<uint(c)) != 0 {
				b.set(ix+c, iy+r, lvl)
			}
		}
	}
}

func relaxInvadersFrames(st *relaxInvadersState, width, height int, fade float64) ([]string, string) {
	w := maxInt(26, minInt(width, 110))
	h := maxInt(8, minInt(height, 30))
	relaxInvResize(st, w*2, h*4)
	b := newRelaxBrailleVote(w, h)

	for _, p := range st.stars {
		b.set(int(p.x*float64(st.sw-1)), int(p.y*float64(st.sh-1)), relaxInvLvlStar)
	}
	if st.ufoOn {
		relaxInvBlit(b, relaxInvUfo, st.ufoX-8, 3, relaxInvLvlUfo)
	}

	pose := (st.tick / 6) % 2
	for _, a := range st.aliens {
		x, y := relaxInvPos(st, a)
		if !a.alive {
			if a.death > 0 { // clarão curto no lugar do bicho
				r := int(4 * a.death)
				for dy := -r; dy <= r; dy++ {
					for dx := -r * 2; dx <= r*2; dx++ {
						if dx*dx+4*dy*dy <= 4*r*r {
							b.set(int(x)+5+dx, int(y)+4+dy, relaxInvLvlBoom)
						}
					}
				}
			}
			continue
		}
		// Os três bichos têm larguras diferentes; centralizar na coluna mantém
		// as fileiras alinhadas.
		sp := relaxInvAliens[a.row][pose]
		relaxInvBlit(b, sp, x+float64(12-sp.w)/2, y, relaxInvLvlSquid+a.row)
	}

	for _, bk := range st.bunkers {
		for r, bits := range bk.rows {
			for c := 0; c < relaxInvBunkerArt.w; c++ {
				if bits&(1<<uint(c)) != 0 {
					b.set(int(bk.x)+c, int(bk.y)+r, relaxInvLvlBunker)
				}
			}
		}
	}

	for _, s := range st.shots {
		lvl, n := relaxInvLvlShot, 4
		if s.alien {
			lvl, n = relaxInvLvlBomb, 5
		}
		for i := 0; i < n; i++ {
			dy := i
			if !s.alien {
				dy = -i
			}
			b.set(int(s.x), int(s.y)+dy, lvl)
		}
	}
	for _, p := range st.parts {
		b.set(int(p.x), int(p.y), relaxInvLvlBoom)
	}

	shipLvl := relaxInvLvlShip
	if st.shipHit > 0 && st.shipHit%4 < 2 {
		shipLvl = relaxInvLvlBoom
	}
	relaxInvBlit(b, relaxInvShip, st.shipX-6, float64(st.sh-9), shipLvl)

	status := fmt.Sprintf("onda %d · %d pontos", st.wave, st.score)
	switch {
	case st.ufoOn:
		status = "disco misterioso passando"
	case st.waveBreak > 0 && relaxInvAlive(st) == 0:
		status = "onda limpa, outra chegando"
	case st.waveBreak > 0:
		status = "a frota passou, recomeçando"
	}
	return b.lines(relaxStyles(relaxInvRamp, fade)), StyleMuted.Render(status)
}
