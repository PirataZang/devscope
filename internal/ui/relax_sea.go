package ui

import (
	"math"
	"math/rand"
)

// ── Mar à noite ───────────────────────────────────────────────────────────────
//
// Horizonte pontilhado: cúmulos passando, lua no alto, e o mar rolando em
// perspectiva. As cristas caminham do horizonte pra cá; o reflexo da lua é um
// caminho de brilho que quebra em cada onda.

var (
	relaxSkyGrad  = []string{"#080C18", "#0C1428", "#141E38", "#1C2C4C", "#2A3E64"}
	relaxCloudSt  = []string{"#243044", "#3A4A62", "#5A6A84", "#8490A8", "#C0C8D4"}
	relaxSeaStops = []string{"#061018", "#0A2038", "#123050", "#1C4A70", "#2A6A90", "#5AA0C0", "#A8D4E4"}
)

const (
	relaxSeaSkyN   = 10
	relaxSeaCloudN = 8
	relaxSeaWaterN = 9
)

const (
	relaxSeaSky   = 0
	relaxSeaCloud = relaxSeaSkyN
	relaxSeaWater = relaxSeaCloud + relaxSeaCloudN
	relaxSeaFoam  = relaxSeaWater + relaxSeaWaterN
	relaxSeaMoon
	relaxSeaGlow
	relaxSeaStar
	relaxSeaGlint
)

var relaxSeaRamp = func() []relaxColor {
	out := make([]relaxColor, relaxSeaGlint+1)
	copy(out[relaxSeaSky:], relaxRamp(relaxSkyGrad, relaxSeaSkyN))
	copy(out[relaxSeaCloud:], relaxRamp(relaxCloudSt, relaxSeaCloudN))
	copy(out[relaxSeaWater:], relaxRamp(relaxSeaStops, relaxSeaWaterN))
	out[relaxSeaFoam] = "#D0E8F0"
	out[relaxSeaMoon] = "#F2D888"
	out[relaxSeaGlow] = "#C4A050"
	out[relaxSeaStar] = "#AEB8D4"
	out[relaxSeaGlint] = "#FFF4C8"
	return out
}()

type relaxSeaPuff struct {
	x, y, rx, ry, vx float64
	tip              float64 // <2 afina as pontas; 2 é elipse
}

type relaxSeaState struct {
	inited bool
	tick   int
	ph     [6]float64
	moonX  float64
	swell  float64
	stars  []relaxSkyPt
	puffs  []relaxSeaPuff
}

func stepRelaxSea(st *relaxSeaState) {
	if !st.inited {
		st.inited = true
		for i := range st.ph {
			st.ph[i] = rand.Float64() * 2 * math.Pi
		}
		st.moonX = 0.68 + rand.Float64()*0.14
		for i, n := 0, 55+rand.Intn(30); i < n; i++ {
			st.stars = append(st.stars, relaxSkyPt{x: rand.Float64(), y: rand.Float64() * 0.92})
		}
		for bnk := 0; bnk < 4; bnk++ {
			bx, by := rand.Float64(), 0.20+rand.Float64()*0.44
			vx := 0.0005 + rand.Float64()*0.00085
			wispy := bnk%2 == 0
			n := 2 + rand.Intn(2)
			if wispy {
				n = 1 + rand.Intn(2)
			}
			for k := 0; k < n; k++ {
				p := relaxSeaPuff{
					x:   math.Mod(bx+(rand.Float64()-0.5)*0.12+1, 1),
					y:   clamp01(by + (rand.Float64()-0.5)*0.08),
					rx:  0.11 + rand.Float64()*0.14,
					ry:  0.06 + rand.Float64()*0.08,
					vx:  vx,
					tip: 1.25 + rand.Float64()*0.25,
				}
				if !wispy {
					p.rx, p.ry, p.tip = 0.07+rand.Float64()*0.08, 0.10+rand.Float64()*0.10, 1.65+rand.Float64()*0.25
				}
				st.puffs = append(st.puffs, p)
			}
		}
	}
	st.tick++
	st.swell = 0.72 + 0.28*math.Sin(float64(st.tick)*0.1*0.052+st.ph[5])
	for i := range st.puffs {
		st.puffs[i].x += st.puffs[i].vx
		if st.puffs[i].x >= 1 {
			st.puffs[i].x -= 1
		}
	}
}

func relaxSeaFrames(st *relaxSeaState, width, height int, fade float64) ([]string, string) {
	if !st.inited {
		stepRelaxSea(st)
	}
	w := maxInt(26, minInt(width, 120))
	h := maxInt(8, minInt(height, 32))
	b := newRelaxBraille(w, h)
	relaxSeaDraw(st, b, w, h)
	status := "o mar vai e volta"
	switch {
	case st.swell > 0.94:
		status = "o mar engrossa"
	case st.swell < 0.52:
		status = "quase liso"
	}
	return b.lines(relaxStyles(relaxSeaRamp, fade)), StyleMuted.Render(status)
}

func relaxSeaPuffDen(p relaxSeaPuff, x, y, sw, hz int) float64 {
	dx := float64(x) - p.x*float64(sw)
	w := float64(sw)
	if dx > w/2 {
		dx -= w
	} else if dx < -w/2 {
		dx += w
	}
	nx := dx / math.Max(1, p.rx*w)
	ny := (float64(y) - p.y*float64(hz)) / math.Max(1, p.ry*float64(hz))
	if ny > 0 {
		ny *= 1.85
	}
	exp := p.tip
	if exp < 1.1 {
		exp = 1.35
	}
	v := 1 - math.Pow(math.Abs(nx), exp) - ny*ny
	if v <= 0 {
		return 0
	}
	if ny < 0 {
		v += 0.10 * math.Sin(dx*0.08+p.x*11) * math.Sin(float64(y)*0.14+p.y*7)
		if v <= 0 {
			return 0
		}
	}
	return v * v
}

func relaxSeaDraw(st *relaxSeaState, b *relaxBraille, w, h int) {
	sw, sh := w*2, h*4
	t := float64(st.tick) * 0.1

	hz := int(float64(sh) * 0.40)
	moonR := float64(minInt(sw, sh)) * 0.09
	moonX := st.moonX * float64(sw)
	moonY := math.Max(moonR*1.2, float64(hz)*0.36)

	// ── Lua ── só set(), sem paint: paint recortava a célula inteira em bloco.
	for y := int(moonY - moonR*1.6); y <= int(moonY+moonR*1.6); y++ {
		for x := int(moonX - moonR*1.6); x <= int(moonX+moonR*1.6); x++ {
			nx, ny := (float64(x)-moonX)/moonR, (float64(y)-moonY)/(moonR*0.92)
			d := nx*nx + ny*ny
			if d <= 1 {
				b.set(x, y, relaxSeaMoon)
				continue
			}
			if d <= 2.6 && relaxHalftone(x, y) < (2.6-d)*0.22 {
				b.set(x, y, relaxSeaGlow)
			}
		}
	}

	// ── Cúmulos ── metaballs com base chata e topo em lóbulos; somem no
	// horizonte em vez de levarem um corte reto.
	den := make([]float32, sw*hz)
	for y := 0; y < hz; y++ {
		fade := clamp01(float64(hz-1-y) / (float64(hz) * 0.16))
		row := y * sw
		for x := 0; x < sw; x++ {
			var d float64
			for _, p := range st.puffs {
				d += relaxSeaPuffDen(p, x, y, sw, hz)
			}
			den[row+x] = float32(d * fade)
		}
	}
	lx, ly := 4, -3
	if moonX < float64(sw)/2 {
		lx = -4
	}
	for y := 0; y < hz; y++ {
		for x := 0; x < sw; x++ {
			d := float64(den[y*sw+x])
			if d <= 0.04 {
				continue
			}
			toward := 0.0
			nx, ny := x+lx, y+ly
			if nx >= 0 && nx < sw && ny >= 0 && ny < hz {
				toward = float64(den[ny*sw+nx])
			}
			lum := 0.16 + 0.52*clamp01(d*1.35) + 0.48*clamp01((d-toward)*2.5)
			ds := math.Hypot(float64(x)-moonX, float64(y)-moonY) / (moonR * 3.2)
			if ds < 1 {
				lum += (1 - ds) * (1 - ds) * 0.45
			}
			if lum <= relaxHalftone(x, y)*0.36 {
				continue
			}
			b.set(x, y, relaxSeaCloud+minInt(int(lum*float64(relaxSeaCloudN)), relaxSeaCloudN-1))
		}
	}

	// ── Estrelas ── atrás das nuvens (ponto já aceso não sobrescreve).
	for i, p := range st.stars {
		x, y := int(p.x*float64(sw-1)), int(p.y*float64(hz-1))
		if math.Hypot(float64(x)-moonX, float64(y)-moonY) < moonR*1.6 {
			continue
		}
		if relaxHalftone(x, y) < 0.22+0.14*math.Sin(t*0.4+float64(i)) {
			b.set(x, y, relaxSeaStar)
		}
	}

	// ── Céu ──
	for y := 0; y < hz; y++ {
		fy := float64(y) / float64(hz)
		for x := 0; x < sw; x++ {
			v := fy * fy * 0.48
			d := math.Hypot(float64(x)-moonX, (float64(y)-moonY)*1.1) / (moonR * 5)
			v += 0.22 / (1 + d*d*8)
			if relaxHalftone(x, y) > v {
				continue
			}
			b.set(x, y, relaxSeaSky+minInt(int((0.15+0.85*fy)*float64(relaxSeaSkyN)), relaxSeaSkyN-1))
		}
	}

	// ── Mar ──
	rows := sh - hz
	for y := hz; y < sh; y++ {
		dy := float64(y-hz) + 0.8
		persp := dy / float64(rows)
		u := 1.0 / (persp + 0.055)
		kx := math.Min(0.42, 0.055/(persp+0.045))
		haze := clamp01(1 - persp*3.2)

		type osc struct{ s, c, ks, kc float64 }
		var o [3]osc
		for i, cfg := range [3]struct{ km, up, sp, ph float64 }{
			{1.0, 3.1, 1.55, 0}, {1.9, 5.3, -1.05, 1.7}, {3.7, 8.9, 2.30, 3.9},
		} {
			a := u*cfg.up + t*cfg.sp + st.ph[i] + cfg.ph
			o[i].s, o[i].c = math.Sincos(a)
			o[i].ks, o[i].kc = math.Sincos(kx * cfg.km)
		}

		halfW := moonR * (0.28 + 2.1*persp)
		wobble := 3.2 * persp * math.Sin(t*1.7+float64(y)*0.31+st.ph[4])
		prev := 0.0
		primed := false
		for x := 0; x < sw; x++ {
			wv := (o[0].s + 0.50*o[1].s + 0.16*o[2].s*persp) / 1.66 * st.swell
			for i := range o {
				o[i].s, o[i].c = o[i].s*o[i].kc+o[i].c*o[i].ks, o[i].c*o[i].kc-o[i].s*o[i].ks
			}
			if !primed {
				prev, primed = wv, true
			}
			slope := wv - prev
			prev = wv

			lum := 0.22 + 0.36*wv + 1.55*slope
			lum = lerp(lum, 0.38, haze)

			wx := moonX + wobble + slope*(10+36*persp)
			g := clamp01(1-math.Abs(float64(x)-wx)/halfW) *
				clamp01(0.40+slope*28) * (0.30 + 0.70*persp)
			if g > 0.22 && relaxHalftone(x, y) < g*0.80 {
				b.set(x, y, relaxSeaGlint)
				continue
			}

			if wv > 0.50 && slope > 0.04 && persp > 0.40 {
				lum += 0.45
				if lum > relaxHalftone(x, y)*0.40 {
					b.set(x, y, relaxSeaFoam)
					continue
				}
			}
			if lum <= relaxHalftone(x, y)*(0.58-0.18*persp) {
				continue
			}
			b.set(x, y, relaxSeaWater+minInt(maxInt(int(lum*float64(relaxSeaWaterN)), 0), relaxSeaWaterN-1))
		}
	}
}
