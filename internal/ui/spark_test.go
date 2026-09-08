package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

// Cada célula Braille carrega duas amostras, então a sparkline mostra o dobro
// de leituras que ocupa em colunas.
func TestBrailleSparkPacksTwoSamplesPerCell(t *testing.T) {
	if got := brailleSpark(make([]float64, 16), 8); len([]rune(got)) != 8 {
		t.Fatalf("8 células devem render 8 runas, got %d (%q)", len([]rune(got)), got)
	}
	subida := brailleSpark([]float64{0, 10, 25, 40, 55, 70, 85, 100}, 4)
	descida := brailleSpark([]float64{100, 85, 70, 55, 40, 25, 10, 0}, 4)
	if subida == descida {
		t.Fatalf("subida e descida não podem desenhar igual: %q", subida)
	}
	// Série curta preenche pela esquerda com vazio; esticar mentiria sobre a
	// tendência.
	if got := brailleSpark([]float64{100, 100}, 4); !strings.HasPrefix(got, "⠀⠀⠀") {
		t.Fatalf("histórico curto deve alinhar à direita: %q", got)
	}
	if got := brailleSpark(nil, 4); got != "⠀⠀⠀⠀" {
		t.Fatalf("sem amostras: %q", got)
	}
	// Zero medido acende um ponto; nunca medido não acende nada.
	if brailleSpark([]float64{0, 0}, 1) == brailleSpark(nil, 1) {
		t.Fatal("zero medido tem que ser distinguível de sem leitura")
	}
}

func TestSparkHistoryKeepsWindow(t *testing.T) {
	var h sparkHistory
	for i := 0; i < 50; i++ {
		h.push(float64(i), 16)
	}
	if len(h.samples) != 16 {
		t.Fatalf("janela deveria ser 16, got %d", len(h.samples))
	}
	if h.last() != 49 {
		t.Fatalf("última amostra: %v", h.last())
	}
}

// O alerta só existe quando algo está prestes a doer — senão vira ruído fixo.
func TestHostAlertOnlyWhenCritical(t *testing.T) {
	if got := hostAlert(core.HostMetrics{CPUPercent: 99, DiskPercent: 40, MemoryPercent: 40}); got != "" {
		t.Fatalf("CPU alta sozinha não é alerta: %q", stripANSI(got))
	}
	if got := stripANSI(hostAlert(core.HostMetrics{DiskPercent: 97})); !strings.Contains(got, "disco") {
		t.Fatalf("disco cheio deve alertar: %q", got)
	}
	if got := stripANSI(hostAlert(core.HostMetrics{MemoryPercent: 95})); !strings.Contains(got, "memória") {
		t.Fatalf("memória no limite deve alertar: %q", got)
	}
}

// A faixa de medidores só cabe em tela alta; em compacta os valores voltam
// para o canto do cabeçalho, mas nunca somem.
func TestHostStripOnlyWhenTall(t *testing.T) {
	host := core.HostMetrics{CPUPercent: 17, MemoryPercent: 51, DiskPercent: 97, OSInfo: "Linux"}
	p := core.Project{Name: "demo", Path: "/p/demo", Status: core.StatusRunning}
	snap := core.Snapshot{Projects: []core.Project{p}, HostMetrics: host}

	tall := &App{width: 120, height: 40, view: ViewDashboard, snapshot: snap}
	tall.sampleHostMetrics()
	got := stripANSI(tall.renderDashboard())
	if !strings.Contains(got, "DISK 97%") || !strings.Contains(got, "⚠") {
		t.Fatalf("tela alta deve ter faixa e alerta:\n%s", got)
	}

	short := &App{width: 120, height: 24, view: ViewDashboard, snapshot: snap}
	short.sampleHostMetrics()
	got = stripANSI(short.renderDashboard())
	for _, want := range []string{"CPU 17%", "RAM 51%", "DISK 97%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("modo compacto não pode perder %q:\n%s", want, got)
		}
	}
}

// A faixa de status é pedida em COLUNAS DE PONTO; um número ímpar deixa a
// última metade do caractere vazia em vez de arredondar para cima.
func TestStatusWaveWidthInDotColumns(t *testing.T) {
	for _, cols := range []int{2, 7, 10} {
		want := (cols + 1) / 2
		for _, kind := range []string{"running", "starting", "restarting", "exited"} {
			got := len([]rune(statusWave(kind, cols, 3)))
			if got != want {
				t.Fatalf("%s com %d colunas → %d caracteres, esperado %d", kind, cols, got, want)
			}
		}
	}
}

// Cada estado precisa de uma forma própria, senão a cor vira a única pista.
func TestStatusWaveShapesAreDistinct(t *testing.T) {
	kinds := []string{"running", "starting", "stopping", "restarting", "unhealthy", "paused", "exited"}
	// Assinatura = a sequência de quadros; duas formas iguais ao longo de um
	// ciclo inteiro seriam indistinguíveis mesmo animadas.
	seen := map[string]string{}
	for _, k := range kinds {
		var sig string
		for f := 0; f < 20; f++ {
			sig += statusWave(k, 10, f)
		}
		if other, dup := seen[sig]; dup {
			t.Fatalf("%q e %q desenham igual em todos os quadros", k, other)
		}
		seen[sig] = k
	}
	// starting e stopping não podem esvaziar de todo: vazio é o exited.
	flat := statusWave("exited", 10, 0)
	for f := 0; f < 20; f++ {
		if statusWave("starting", 10, f) == flat {
			t.Fatalf("starting fica idêntico ao exited no quadro %d", f)
		}
		if statusWave("stopping", 10, f) == flat {
			t.Fatalf("stopping fica idêntico ao exited no quadro %d", f)
		}
	}
}
