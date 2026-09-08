package ui

import (
	"math"
	"strings"
)

// Sparkline em Braille: cada célula 2×4 carrega DUAS amostras — coluna
// esquerda e coluna direita —, então 8 colunas mostram 16 leituras. É a forma
// mais densa de histórico que cabe num terminal, e mantém o vocabulário
// Braille do resto do app.

// Bits do Braille, de baixo para cima, em cada coluna.
var (
	brailleLeftBits  = []byte{0x40, 0x04, 0x02, 0x01} // pontos 7,3,2,1
	brailleRightBits = []byte{0x80, 0x20, 0x10, 0x08} // pontos 8,6,5,4
)

// sparkHistory guarda as últimas amostras de uma métrica (0-100).
type sparkHistory struct {
	samples []float64
	limit   int
}

func (h *sparkHistory) push(v float64, limit int) {
	if limit <= 0 {
		limit = 32
	}
	h.limit = limit
	h.samples = append(h.samples, v)
	if len(h.samples) > limit {
		h.samples = h.samples[len(h.samples)-limit:]
	}
}

// last devolve a média das amostras mais recentes — o valor instantâneo pisca
// demais para caber num rótulo.
func (h *sparkHistory) last() float64 {
	if len(h.samples) == 0 {
		return 0
	}
	return h.samples[len(h.samples)-1]
}

// brailleSpark desenha as últimas 2*cells amostras. Sem histórico suficiente,
// preenche pela esquerda com vazio em vez de esticar o pouco que tem — uma
// série curta esticada mente sobre a tendência.
func brailleSpark(samples []float64, cells int) string {
	if cells <= 0 {
		return ""
	}
	want := cells * 2
	vals := make([]float64, want)
	for i := range vals {
		vals[i] = -1 // -1 = ainda não medido
	}
	if n := len(samples); n > 0 {
		if n > want {
			samples = samples[n-want:]
			n = want
		}
		copy(vals[want-n:], samples)
	}

	var b strings.Builder
	for c := 0; c < cells; c++ {
		var mask byte
		mask |= sparkColumn(vals[c*2], brailleLeftBits)
		mask |= sparkColumn(vals[c*2+1], brailleRightBits)
		b.WriteRune(rune(0x2800 + int(mask)))
	}
	return b.String()
}

// sparkColumn converte um percentual na altura preenchida da coluna. Qualquer
// valor medido acende ao menos um ponto: uma coluna vazia significa "sem
// leitura", não "zero".
func sparkColumn(pct float64, bits []byte) byte {
	if pct < 0 {
		return 0
	}
	h := int(pct/25) + 1
	if pct <= 0 {
		h = 1
	}
	if h > 4 {
		h = 4
	}
	var mask byte
	for i := 0; i < h; i++ {
		mask |= bits[i]
	}
	return mask
}

// sparkCells é a largura das sparklines do dashboard; 8 células = 16 amostras.
const sparkCells = 8

// sampleHostMetrics registra uma leitura por tick (300 ms) — 16 amostras
// cobrem os últimos ~5 s, que é a janela útil para ver uma escalada.
func (a *App) sampleHostMetrics() {
	m := a.snapshot.HostMetrics
	limit := sparkCells * 2
	a.hostCPUHist.push(m.CPUPercent, limit)
	a.hostRAMHist.push(m.MemoryPercent, limit)
	a.hostDiskHist.push(m.DiskPercent, limit)
}

// ─── onda de status ─────────────────────────────────────────────────────────
//
// Um caractere Braille tem só 2 colunas, e 2 colunas não dão forma suficiente
// para separar seis estados. Emendando N células vira uma faixa de 2N colunas,
// onde a FORMA diz o estado e a COR diz a gravidade.

// brailleWave monta uma faixa de `cols` COLUNAS DE PONTO (0 = mais à esquerda)
// a partir da altura 0-4 de cada uma. Como cada caractere Braille carrega duas
// colunas, um número ímpar deixa a última metade vazia — foi feito assim para
// que a largura possa ser pedida em colunas, não em caracteres.
func brailleWave(cols int, height func(col int) int) string {
	if cols <= 0 {
		return ""
	}
	h := func(col int) int {
		if col >= cols {
			return 0
		}
		return height(col)
	}
	cells := (cols + 1) / 2
	var b strings.Builder
	for c := 0; c < cells; c++ {
		var mask byte
		mask |= waveColumn(h(c*2), brailleLeftBits)
		mask |= waveColumn(h(c*2+1), brailleRightBits)
		b.WriteRune(rune(0x2800 + int(mask)))
	}
	return b.String()
}

// ─── formas de status ───────────────────────────────────────────────────────

// statusWave é a faixa de status compartilhada por todas as telas. A FORMA diz
// o estado, a COR (escolhida por quem chama) diz a gravidade:
//
//	running     onda meio cheia caminhando   está trabalhando
//	starting    preenche da esquerda p/ dir. está subindo
//	stopping    esvazia da direita p/ esq.   está descendo
//	restarting  linha baixa com pico passando algo atravessando
//	unhealthy   plana e uniforme, respirando no ar, porém mal
//	paused      todas iguais, imóveis        congelado
//	exited      rasteiras, imóveis           sem energia
//	idle        pontinhos alternados         estado desconhecido
func statusWave(kind string, cols, frame int) string {
	if frame < 0 {
		frame = -frame
	}
	switch kind {
	case "running":
		return brailleWave(cols, func(col int) int {
			return clampWaveHeight(2 + int(math.Round(1.6*math.Sin(float64(col+frame)/1.7))))
		})
	case "starting":
		// Enche da esquerda para a direita e recomeça: está subindo. Nunca
		// esvazia de todo — vazia seria idêntica ao exited.
		fill := 1 + frame%cols
		return brailleWave(cols, func(col int) int {
			if col < fill {
				return 4
			}
			return 1
		})
	case "stopping":
		// O inverso: esvazia da direita para a esquerda, sem chegar a zero.
		fill := cols - frame%cols
		return brailleWave(cols, func(col int) int {
			if col < fill {
				return 4
			}
			return 1
		})
	case "restarting":
		peak := frame % cols
		return brailleWave(cols, func(col int) int {
			switch d := ((col-peak)%cols + cols) % cols; {
			case d == 0:
				return 4
			case d == 1 || d == cols-1:
				return 2
			default:
				return 1
			}
		})
	case "unhealthy":
		return brailleWave(cols, func(int) int { return 2 + (frame/4)%2 })
	case "paused":
		return brailleWave(cols, func(int) int { return 4 })
	case "idle":
		// Pontinhos alternados, deslizando: não sabemos o estado.
		return brailleWave(cols, func(col int) int {
			if (col+frame/3)%2 == 0 {
				return 1
			}
			return 0
		})
	default: // exited, created
		return brailleWave(cols, func(int) int { return 1 })
	}
}

func clampWaveHeight(h int) int {
	if h < 1 {
		return 1
	}
	if h > 4 {
		return 4
	}
	return h
}

func waveColumn(h int, bits []byte) byte {
	if h < 0 {
		h = 0
	}
	if h > len(bits) {
		h = len(bits)
	}
	var mask byte
	for i := 0; i < h; i++ {
		mask |= bits[i]
	}
	return mask
}
