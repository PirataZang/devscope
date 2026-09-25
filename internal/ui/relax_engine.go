package ui

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ── Animation engine ──────────────────────────────────────────────────────────
//
// Um relógio só para todo o Relax. O tick do Bubble Tea é o "frame" de render
// (~30fps); a simulação anda em passos fixos consumidos de um acumulador de
// tempo real. Se um tick atrasa, o engine dá os passos que faltaram; se chega
// adiantado, só desenha. Assim a cena não acelera nem trava conforme a carga
// da máquina — e nada teleporta ao voltar de uma pausa longa.
//
// Separação: advance() = update (física/estado), frames() de cada cena =
// render puro. O framework nunca é chamado por frame de animação.

const (
	relaxRenderInterval = 33 * time.Millisecond  // ~30fps de desenho
	relaxSimStep        = 100 * time.Millisecond // passo fixo da simulação
	relaxMaxCatchUp     = 4                      // passos por frame no máximo
	relaxSceneIn        = 0.45                   // segundos de entrada da cena
	relaxTransition     = 0.55                   // segundos de troca de cena
)

// Sub-passos por passo de simulação. Cenas com física de partículas integram em
// sub-passos fixos de relaxSimStep/relaxSubSteps (~33ms) alimentados por tempo
// real: depois de 100ms o estado é o mesmo, mas houve ~3x mais quadros de
// movimento — é o que dá a fluidez de 30fps sem mudar velocidade nem ritmo.
const relaxSubSteps = 3

// relaxSubDT é a duração de um sub-passo, em segundos.
const relaxSubDT = 0.1 / relaxSubSteps

// relaxSubClock converte o relógio contínuo (sceneTime, em segundos) em
// sub-passos fixos. A cena guarda um destes e chama tick(now) uma vez por frame
// de render; o retorno é quantos sub-passos de relaxSubDT rodar agora. Um teto
// baixo evita despejar simulação ao voltar de uma pausa longa.
type relaxSubClock struct {
	last  float64
	acc   float64
	start bool
}

func (c *relaxSubClock) tick(now float64) int {
	if !c.start {
		c.start, c.last = true, now
		return 0
	}
	dt := now - c.last
	c.last = now
	if dt <= 0 {
		return 0
	}
	if dt > 0.5 { // volta de suspensão: não acumula minutos
		dt = 0.1
	}
	c.acc += dt
	n := 0
	for c.acc >= relaxSubDT && n < relaxSubSteps*relaxMaxCatchUp {
		c.acc -= relaxSubDT
		n++
	}
	if n == relaxSubSteps*relaxMaxCatchUp {
		c.acc = 0
	}
	return n
}

type relaxEngine struct {
	last     time.Time
	acc      time.Duration
	elapsed  float64 // segundos dentro do Relax
	scene    float64 // segundos na cena atual
	trans    float64 // segundos restantes da transição
	steps    int
	fps      float64
	frameMS  float64
	reduced  bool
	switched bool // já trocou a cena no meio da transição
}

func (e *relaxEngine) reset() {
	*e = relaxEngine{reduced: os.Getenv("DEVSCOPE_REDUCED_MOTION") != ""}
}

// advance consome o tempo real desde o último frame e chama step() o número
// certo de vezes. Retorna quantos passos rodaram.
func (e *relaxEngine) advance(now time.Time, step func()) int {
	if e.last.IsZero() {
		e.last = now
		return 0
	}
	dt := now.Sub(e.last)
	e.last = now
	if dt <= 0 {
		return 0
	}
	// Volta de suspensão/terminal parado: não despeja minutos de simulação.
	if dt > 500*time.Millisecond {
		dt = relaxSimStep
	}
	secs := dt.Seconds()
	e.elapsed += secs
	e.scene += secs
	if e.trans > 0 {
		e.trans = math.Max(0, e.trans-secs)
	}
	// Média móvel: número estável o bastante pra ler no debug.
	e.frameMS = lerp(e.frameMS, secs*1000, 0.12)
	e.fps = lerp(e.fps, 1/secs, 0.12)

	sim := relaxSimStep
	if e.reduced {
		sim *= 2 // movimento reduzido: metade da velocidade, mesmo desenho
	}
	e.acc += dt
	n := 0
	for e.acc >= sim && n < relaxMaxCatchUp {
		step()
		e.acc -= sim
		e.steps++
		n++
	}
	if n == relaxMaxCatchUp {
		e.acc = 0 // ficou longe demais: descarta em vez de acumular dívida
	}
	return n
}

// sceneTime é o relógio contínuo das cenas: segundos dentro do Relax. Cena que
// anima por tempo (e não por passo de simulação) desenha movimento novo em cada
// um dos ~30 frames por segundo, em vez de repetir o mesmo quadro dez vezes.
// Com movimento reduzido ele anda na metade, igual à simulação.
func (e *relaxEngine) sceneTime() float64 {
	if e.reduced {
		return e.elapsed * 0.5
	}
	return e.elapsed
}

func (e *relaxEngine) beginTransition() {
	e.trans = relaxTransition
	e.switched = false
}

// half indica que a transição passou do meio — hora de trocar o estado da cena,
// com a tela já apagada.
func (e *relaxEngine) half() bool { return e.trans > 0 && e.trans <= relaxTransition/2 }

// fade é o brilho global: 0 apagado, 1 cheio. Cobre entrada da cena e a
// transição (apaga a que sai, acende a que entra), sempre com easing.
func (e *relaxEngine) fade() float64 {
	if e.trans > 0 {
		p := 1 - e.trans/relaxTransition
		if p < 0.5 {
			return easeInOutSine(1 - p*2)
		}
		return easeInOutSine((p - 0.5) * 2)
	}
	if e.scene < relaxSceneIn {
		return easeOutCubic(e.scene / relaxSceneIn)
	}
	return 1
}

func (e *relaxEngine) debug(scene string, objects int) string {
	if os.Getenv("DEVSCOPE_RELAX_DEBUG") == "" {
		return ""
	}
	return StyleMuted.Render(fmt.Sprintf(
		"RELAX DEBUG · fps %.0f · frame %.1fms · sim %d · cena %s %.1fs · objetos %d · fade %.2f",
		e.fps, e.frameMS, e.steps, scene, e.scene, objects, e.fade()))
}

// ── Cor ───────────────────────────────────────────────────────────────────────

// relaxDim escurece um estilo interpolando a cor até o fundo do tema. É assim
// que fade/crossfade acontecem: em 24 bits a transição é contínua de verdade,
// diferente de ligar/desligar o atributo faint.
func relaxDim(s lipgloss.Style, f float64) lipgloss.Style {
	if f >= 0.999 {
		return s
	}
	if f <= 0.001 {
		return s.Foreground(ColorBg)
	}
	fr, fg, fb, ok := relaxHexRGB(fmt.Sprint(s.GetForeground()))
	br, bg, bb, okBg := relaxHexRGB(string(ColorBg))
	if !ok || !okBg {
		if f < 0.5 {
			return s.Faint(true)
		}
		return s
	}
	mix := func(a, b int) int { return int(float64(b) + (float64(a)-float64(b))*f + 0.5) }
	return s.Foreground(lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		mix(fr, br), mix(fg, bg), mix(fb, bb))))
}

func relaxHexRGB(s string) (int, int, int, bool) {
	var r, g, b int
	if n, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil || n != 3 {
		return 0, 0, 0, false
	}
	return r, g, b, true
}
