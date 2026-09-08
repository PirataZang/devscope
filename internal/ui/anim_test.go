package ui

import (
	"strings"
	"testing"
	"time"
)

func TestAnimSpinnerCycles(t *testing.T) {
	if len(animSpinnerFrames) < 8 || len(animSpinnerFrames) > 12 {
		t.Fatalf("spinner frames=%d want 8–12", len(animSpinnerFrames))
	}
	seen := map[string]bool{}
	for i := 0; i < len(animSpinnerFrames)*2; i++ {
		seen[animSpinner(i)] = true
	}
	if len(seen) != len(animSpinnerFrames) {
		t.Fatalf("cycle incomplete: %d unique", len(seen))
	}
	if animSpinner(0) != animSpinnerFrames[0] {
		t.Fatal("frame 0")
	}
}

func TestAnimPulseAndArc(t *testing.T) {
	if animPulse(0) == "" || animArc(1) == "" {
		t.Fatal("empty glyph")
	}
	if animPulse(0) == animPulse(1) && animPulse(1) == animPulse(2) {
		t.Fatal("pulse should change across frames")
	}
}

func TestTunnelStatusBadgeAnim(t *testing.T) {
	online := stripANSI(tunnelStatusBadge("online", 0))
	if !strings.Contains(online, "online") {
		t.Fatalf("online=%q", online)
	}
	a := stripANSI(tunnelStatusBadge("starting", 0))
	b := stripANSI(tunnelStatusBadge("starting", 3))
	if a == b || !strings.Contains(a, "subindo") {
		t.Fatalf("subindo deve animar: %q vs %q", a, b)
	}
	// Padrão do projeto: todo status é Braille animado, inclusive online.
	if stripANSI(tunnelStatusBadge("online", 0)) == stripANSI(tunnelStatusBadge("online", 6)) {
		t.Fatal("online deve animar entre quadros")
	}
}

func TestGHALiveBadgeAnim(t *testing.T) {
	run := stripANSI(ghaLiveBadge("running", 0))
	if !strings.Contains(run, "rodando") {
		t.Fatalf("%q", run)
	}
	if stripANSI(ghaLiveBadge("failure", 0)) == run {
		t.Fatal("falha e rodando devem ler diferente")
	}
	a := &App{animFrame: 2}
	got := stripANSI(a.loadingText("Carregando…"))
	if !strings.Contains(got, "Carregando") || !strings.Contains(got, animSpinner(2)) {
		t.Fatalf("%q", got)
	}
}

func TestAnimIntervalInRange(t *testing.T) {
	fps := float64(time.Second) / float64(animInterval)
	if fps < 8 || fps > 12 {
		t.Fatalf("fps=%.1f want 8–12", fps)
	}
}
