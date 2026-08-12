package ui

import "math"

// ── Movimento ─────────────────────────────────────────────────────────────────
//
// Helpers compartilhados pelas cenas do Relax. A regra é: nada troca de valor
// de uma vez — posição, velocidade, ângulo e brilho sempre caminham até o alvo.

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// lerpAngle vai pelo caminho curto do círculo (senão o objeto gira 350° pra
// corrigir 10°).
func lerpAngle(a, b, t float64) float64 {
	d := math.Mod(b-a+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return a + (d-math.Pi)*t
}

// smoothDamp aproxima cur de target com meia-vida independente do passo: o
// mesmo movimento sai igual a 10 ou 30 passos por segundo.
func smoothDamp(cur, target, halfLife, dt float64) float64 {
	if halfLife <= 0 {
		return target
	}
	return target + (cur-target)*math.Exp2(-dt/halfLife)
}

func easeOutCubic(t float64) float64 {
	t = clamp01(t)
	u := 1 - t
	return 1 - u*u*u
}

// easeInOut é smoothstep — nada de movimento linear seco.
func easeInOut(t float64) float64 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}

func easeInOutSine(t float64) float64 { return 0.5 * (1 - math.Cos(clamp01(t)*math.Pi)) }

func easeOutExpo(t float64) float64 {
	if t >= 1 {
		return 1
	}
	return 1 - math.Pow(2, -10*clamp01(t))
}

func clamp01(t float64) float64 { return math.Max(0, math.Min(1, t)) }

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// relaxNoise é value noise 1D: contínuo, sem os saltos de random() a cada
// frame. Usado em vento, deriva e microvariações.
func relaxNoise(x float64, seed int) float64 {
	i := math.Floor(x)
	f := x - i
	h := func(n float64) float64 {
		return float64(relaxHash(int(n), seed)%2000)/1000 - 1
	}
	return lerp(h(i), h(i+1), easeInOut(f))
}
