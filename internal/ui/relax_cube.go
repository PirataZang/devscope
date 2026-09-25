package ui

import (
	"fmt"
	"math"
	"math/rand"
)

// ── Magic Cube ────────────────────────────────────────────────────────────────
//
// Cubo mágico 3D de verdade em Braille: 27 cubinhos com adesivos por face, giro
// de camada animado e projeção com perspectiva leve. Não há solver — o cubo
// embaralha guardando os movimentos e depois os desfaz na ordem inversa, que dá
// uma resolução legítima de graça.
//
// Desenho de trás pra frente não daria: relaxBraille não sobrescreve ponto
// aceso. Por isso as faces são ordenadas da mais próxima pra mais distante.

// Direções das seis faces, na ordem dos índices de adesivo.
var relaxCubeDirs = [6][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}

var relaxCubeStops = [6][]string{
	{"#5A1414", "#B32626", "#F87171"}, // +x vermelho
	{"#6A3208", "#D97316", "#FDBA74"}, // -x laranja
	{"#8A8A82", "#DEDEDA", "#FFFFFF"}, // +y branco
	{"#6E5A0B", "#D6A912", "#FDE68A"}, // -y amarelo
	{"#0F4D24", "#1FA24C", "#86EFAC"}, // +z verde
	{"#12305E", "#2563C7", "#93C5FD"}, // -z azul
}

const (
	relaxCubeShades = 3
	relaxCubeBody   = 6 * relaxCubeShades // plástico entre os adesivos
)

var relaxCubeRamp = func() []relaxColor {
	out := make([]relaxColor, relaxCubeBody+1)
	for f, stops := range relaxCubeStops {
		copy(out[f*relaxCubeShades:], relaxRamp(stops, relaxCubeShades))
	}
	out[relaxCubeBody] = "#16181D"
	return out
}()

type relaxCubie struct {
	p       [3]int  // posição -1..1
	sticker [6]int8 // cor por direção de face; -1 = miolo
}

type relaxCubeMove struct{ axis, layer, dir int }

type relaxCubeState struct {
	inited  bool
	clock   float64 // último sceneTime visto (segundos), pro passo por tempo
	cubies  []relaxCubie
	history []relaxCubeMove // movimentos do embaralhamento, pra desfazer depois

	move     relaxCubeMove
	turning  bool
	turnT    float64 // 0..1 do giro atual
	solving  bool
	left     int     // movimentos que faltam nesta etapa
	hold     float64 // segundos de pausa antes do próximo movimento
	yaw, pit float64
}

const (
	// O cubo anda pelo relógio de render (~30fps), não pelo passo fixo da
	// simulação: yaw/pit e o giro de camada avançam por tempo real, então cada
	// um dos ~30 frames por segundo desenha movimento novo em vez de repetir o
	// mesmo quadro. Durações em segundos, independentes da taxa de frames.
	relaxCubeYawRate  = 0.48 // rad/s do giro livre no ar
	relaxCubeTurnDur  = 0.30 // segundos por giro de camada de 90°
	relaxCubeHoldMid  = 2.2  // pausa admirando o embaralhado
	relaxCubeHoldDone = 1.2  // pausa depois de resolver

	// Quantidade de movimentos do embaralhamento: mais movimentos, ciclo mais
	// longo. O total (embaralhar + desfazer) é o dobro disto, e a ~0,30s por
	// giro dá o tempo de animação. Faixa aleatória pra não repetir o mesmo
	// comprimento de sequência a cada volta.
	relaxCubeScrambleMin = 22
	relaxCubeScrambleVar = 13 // rand.Intn deste + Min = 22..34 movimentos
)

func relaxCubeInit(st *relaxCubeState) {
	st.inited = true
	st.cubies = st.cubies[:0]
	for x := -1; x <= 1; x++ {
		for y := -1; y <= 1; y++ {
			for z := -1; z <= 1; z++ {
				c := relaxCubie{p: [3]int{x, y, z}}
				for f, d := range relaxCubeDirs {
					c.sticker[f] = -1
					// Só há adesivo onde o cubinho encosta no lado de fora.
					if (d[0] != 0 && x == d[0]) || (d[1] != 0 && y == d[1]) || (d[2] != 0 && z == d[2]) {
						c.sticker[f] = int8(f)
					}
				}
				st.cubies = append(st.cubies, c)
			}
		}
	}
	st.history = st.history[:0]
	st.solving = false
	st.left = relaxCubeScrambleMin + rand.Intn(relaxCubeScrambleVar)
	st.yaw, st.pit = rand.Float64()*6.28, 0.55
}

// relaxCubeRot gira um vetor inteiro 90° em torno de um eixo.
func relaxCubeRot(v [3]int, axis, dir int) [3]int {
	a, b := (axis+1)%3, (axis+2)%3
	out := v
	if dir > 0 {
		out[a], out[b] = -v[b], v[a]
	} else {
		out[a], out[b] = v[b], -v[a]
	}
	return out
}

func relaxCubeDirIndex(v [3]int) int {
	for i, d := range relaxCubeDirs {
		if d == v {
			return i
		}
	}
	return 0
}

// relaxCubeApply consuma o giro: move os cubinhos da camada e leva os adesivos
// junto — o adesivo que olhava pra d passa a olhar pra R(d).
func relaxCubeApply(st *relaxCubeState, m relaxCubeMove) {
	for i := range st.cubies {
		c := &st.cubies[i]
		if c.p[m.axis] != m.layer {
			continue
		}
		c.p = relaxCubeRot(c.p, m.axis, m.dir)
		var ns [6]int8
		for f, d := range relaxCubeDirs {
			ns[relaxCubeDirIndex(relaxCubeRot(d, m.axis, m.dir))] = c.sticker[f]
		}
		c.sticker = ns
	}
}

func stepRelaxCube(st *relaxCubeState, now float64) {
	if !st.inited {
		relaxCubeInit(st)
		st.clock = now
	}
	// dt de tempo real, limitado pra não teleportar ao voltar de uma pausa.
	dt := clampF(now-st.clock, 0, 0.1)
	st.clock = now

	// O cubo gira sozinho no espaço o tempo todo, independente dos giros de
	// camada — é isso que deixa claro que é um sólido e não um desenho chapado.
	st.yaw += relaxCubeYawRate * dt
	st.pit = 0.52 + 0.20*math.Sin(now*0.33)

	if st.turning {
		// Giro de camada avança por tempo: fluido nos ~30fps de render.
		if st.turnT += dt / relaxCubeTurnDur; st.turnT < 1 {
			return
		}
		relaxCubeApply(st, st.move)
		st.turning, st.turnT = false, 0
		st.left--
		return
	}
	if st.hold > 0 {
		st.hold = math.Max(0, st.hold-dt)
		return
	}
	if st.left <= 0 {
		if st.solving {
			st.solving = false
			st.left = relaxCubeScrambleMin + rand.Intn(relaxCubeScrambleVar)
			st.hold = relaxCubeHoldDone
		} else {
			st.solving = true
			st.left = len(st.history)
			st.hold = relaxCubeHoldMid // admira o estrago antes de desfazer
		}
		return
	}

	if st.solving {
		m := st.history[len(st.history)-1]
		st.history = st.history[:len(st.history)-1]
		st.move = relaxCubeMove{axis: m.axis, layer: m.layer, dir: -m.dir}
	} else {
		m := relaxCubeMove{axis: rand.Intn(3), layer: rand.Intn(3) - 1, dir: 1 - 2*rand.Intn(2)}
		if m.layer == 0 { // girar a fatia do meio quase não muda nada de visível
			m.layer = 1 - 2*rand.Intn(2)
		}
		st.history = append(st.history, m)
		st.move = m
	}
	st.turning, st.turnT = true, 0
}

// relaxCubeSolved: adesivo resolvido é o que voltou pra face de origem, e o
// índice do adesivo é justamente essa origem.
func relaxCubeSolved(st *relaxCubeState) bool {
	for _, c := range st.cubies {
		for f, s := range c.sticker {
			if s >= 0 && int(s) != f {
				return false
			}
		}
	}
	return true
}

// relaxCubeFace é uma face projetada, guardada pra ordenar por profundidade.
type relaxCubeFace struct {
	sticker [4][2]float64
	body    [4][2]float64
	z       float64
	level   int
}

func relaxCubeFrames(st *relaxCubeState, width, height int, fade, now float64) ([]string, string) {
	// O cubo anda no relógio de render: avança aqui a cada frame (~30fps), não
	// só nos passos fixos da simulação — é o que tira o movimento em degraus.
	stepRelaxCube(st, now)
	w := maxInt(24, minInt(width, 100))
	h := maxInt(8, minInt(height, 28))
	b := newRelaxBrailleVote(w, h)
	sw, sh := float64(w*2), float64(h*4)

	// Interpola o giro em curso, com easing: o cubo real não parte nem para seco.
	turn := 0.0
	if st.turning {
		turn = float64(st.move.dir) * (math.Pi / 2) * easeInOutSine(st.turnT)
	}

	sy, cy := math.Sin(st.yaw), math.Cos(st.yaw)
	sp, cp := math.Sin(st.pit), math.Cos(st.pit)
	view := func(v [3]float64) [3]float64 {
		x, z := v[0]*cy+v[2]*sy, -v[0]*sy+v[2]*cy
		y, z2 := v[1]*cp-z*sp, v[1]*sp+z*cp
		return [3]float64{x, y, z2}
	}
	// Giro da camada, aplicado antes da câmera e só em quem está na fatia.
	sl, cl := math.Sin(turn), math.Cos(turn)
	layer := func(v [3]float64) [3]float64 {
		a, bb := (st.move.axis+1)%3, (st.move.axis+2)%3
		out := v
		out[a] = v[a]*cl - v[bb]*sl
		out[bb] = v[a]*sl + v[bb]*cl
		return out
	}

	scale := math.Min(sw, sh) / 5.6
	cx0, cy0 := sw/2, sh/2
	const persp = 9.0
	project := func(v [3]float64) ([2]float64, float64) {
		k := persp / (persp - v[2])
		return [2]float64{cx0 + v[0]*scale*k, cy0 + v[1]*scale*k}, v[2]
	}

	light := [3]float64{-0.45, -0.72, 0.52}
	faces := make([]relaxCubeFace, 0, 54)
	for _, c := range st.cubies {
		spin := st.turning && c.p[st.move.axis] == st.move.layer
		for f, d := range relaxCubeDirs {
			if c.sticker[f] < 0 {
				continue
			}
			n := [3]float64{float64(d[0]), float64(d[1]), float64(d[2])}
			ctr := [3]float64{float64(c.p[0]) + n[0]*0.5, float64(c.p[1]) + n[1]*0.5, float64(c.p[2]) + n[2]*0.5}
			// Dois eixos perpendiculares à normal, pros cantos do quadrado.
			u := [3]float64{n[1], n[2], n[0]}
			v := [3]float64{n[2]*n[2]*0 + (n[0]*u[1] - n[1]*u[0]), n[1]*u[2] - n[2]*u[1], n[0]*u[1] - n[1]*u[0]}
			v = [3]float64{n[1]*u[2] - n[2]*u[1], n[2]*u[0] - n[0]*u[2], n[0]*u[1] - n[1]*u[0]}
			if spin {
				ctr, n, u, v = layer(ctr), layer(n), layer(u), layer(v)
			}
			ctr, n, u, v = view(ctr), view(n), view(u), view(v)
			if n[2] <= 0.06 {
				continue // face virada pra trás
			}
			corners := func(k float64) [4][2]float64 {
				var q [4][2]float64
				for i, s := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
					q[i], _ = project([3]float64{
						ctr[0] + (u[0]*s[0]+v[0]*s[1])*k,
						ctr[1] + (u[1]*s[0]+v[1]*s[1])*k,
						ctr[2] + (u[2]*s[0]+v[2]*s[1])*k,
					})
				}
				return q
			}
			lam := clamp01(0.34 + 0.66*(n[0]*light[0]+n[1]*light[1]+n[2]*light[2]))
			shade := minInt(int(lam*relaxCubeShades), relaxCubeShades-1)
			faces = append(faces, relaxCubeFace{
				sticker: corners(0.37), body: corners(0.5),
				z: ctr[2], level: int(c.sticker[f])*relaxCubeShades + shade})
		}
	}
	// Mais perto primeiro: o primeiro a escrever fica.
	for i := 1; i < len(faces); i++ {
		for j := i; j > 0 && faces[j].z > faces[j-1].z; j-- {
			faces[j], faces[j-1] = faces[j-1], faces[j]
		}
	}
	for _, f := range faces {
		// Adesivo primeiro, plástico depois: relaxBraille não sobrescreve, então
		// a face cheia só pinta a moldura que o adesivo encolhido deixou. Fosse
		// ao contrário, o contorno não apareceria e adesivos vizinhos de mesma
		// cor virariam uma mancha só.
		b.quad(f.sticker, f.level)
		b.quad(f.body, relaxCubeBody)
	}

	status := StyleAccent.Render(fmt.Sprintf("embaralhando · %d movimentos", st.left))
	switch {
	case relaxCubeSolved(st) && !st.turning:
		status = StyleHealthy.Render("resolvido ✓")
	case st.solving:
		status = StyleHealthy.Render(fmt.Sprintf("desfazendo · faltam %d", st.left))
	}
	return b.lines(relaxStyles(relaxCubeRamp, fade)), status
}
