package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// O realce só pode pintar. Se mexer no texto visível, a linha deixa de ser a
// que o container escreveu — e é ela que se cola num chamado.
func TestHighlightKeepsVisibleText(t *testing.T) {
	cases := map[containerDetailTab]string{
		containerDetailTabLogs:    "2026-09-08T15:00:12.443Z ERROR [app] falha ao conectar host=db port=5432",
		containerDetailTabConfig:  `  "State": { "Status": "running", "Pid": 4821 },`,
		containerDetailTabCompose: `    image: digiliza/checkout:1.4.2 # fixado`,
		containerDetailTabTop:     "root  4821  4800  0  09:12  ?    00:00:12  php-fpm: master",
		containerDetailTabFile:    "-rw-r--r-- 1 root root  220 Sep  6 09:12 .env",
	}
	for tab, line := range cases {
		if got := stripANSI(highlightDetailLine(tab, line)); got != line {
			t.Fatalf("aba %s alterou o texto:\n want %q\n  got %q", tab.shortLabel(), line, got)
		}
	}
}

func TestEnvSecretsAreMasked(t *testing.T) {
	got := stripANSI(highlightDetailLine(containerDetailTabEnv, "APP_KEY=base64:9f8a7b6c5d"))
	if strings.Contains(got, "9f8a7b6c5d") {
		t.Fatalf("segredo impresso na tela: %q", got)
	}
	if !strings.HasPrefix(got, "APP_KEY=") {
		t.Fatalf("o nome da variável precisa continuar legível: %q", got)
	}
	plain := "DB_HOST=checkout-db"
	if got := stripANSI(highlightDetailLine(containerDetailTabEnv, plain)); got != plain {
		t.Fatalf("valor sem segredo não deve ser mascarado: %q", got)
	}
}

// A janela tem no máximo 40 amostras e a caixa passa de 60 colunas: sem
// esticar, dois terços do gráfico ficam em branco.
func TestHistoryBarsFillWidth(t *testing.T) {
	rows := renderHistoryBarRows([]float64{1, 2, 3}, 30, 4, 3, StyleNormal)
	if len(rows) != 4 {
		t.Fatalf("esperava 4 linhas, veio %d", len(rows))
	}
	for i, r := range rows {
		if w := lipgloss.Width(stripANSI(r)); w != 30 {
			t.Fatalf("linha %d tem %d colunas, não 30", i, w)
		}
	}
	if !strings.Contains(rows[0], "█") {
		t.Fatal("o pico precisa alcançar a primeira linha")
	}
}

func TestStatsScaleTopSteps(t *testing.T) {
	for _, c := range []struct {
		peak float64
		want float64
	}{{4, 10}, {12, 25}, {40, 50}, {90, 100}} {
		if got := statsScaleTop([]float64{c.peak}); got != c.want {
			t.Fatalf("pico %.0f: teto %.0f, esperado %.0f", c.peak, got, c.want)
		}
	}
}

// As setas passaram a rolar o texto de lado; a troca de aba é só pelo número.
func TestContainerDetailNavigationKeys(t *testing.T) {
	a := &App{
		width: 100, height: 30, view: ViewProject, tab: TabContainers,
		containerSubview:       containerSubviewDetail,
		containerDetailTab:     containerDetailTabLogs,
		containerDetailName:    "web",
		containerDetailID:      "abc",
		containerDetailContent: strings.Repeat("x", 500),
	}
	p := &core.Project{}

	a.handleContainerDetailKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if a.containerDetailTab != containerDetailTabLogs {
		t.Fatal("seta não pode mais trocar de aba")
	}
	if a.containerDetailHScroll == 0 {
		t.Fatal("seta direita precisa rolar o texto de lado")
	}

	a.handleContainerDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")}, p)
	if a.containerDetailHScroll != 0 {
		t.Fatalf("0 volta ao início da linha, veio %d", a.containerDetailHScroll)
	}

	a.handleContainerDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")}, p)
	if a.containerDetailTab != containerDetailTabConfig {
		t.Fatalf("4 devia abrir CONFIG, abriu %s", a.containerDetailTab.shortLabel())
	}
}

// Sem alinhar, cada linha do `docker top` começa numa coluna diferente.
func TestLayoutAlignsTabularContent(t *testing.T) {
	top := layoutDetailLines(containerDetailTabTop, []string{
		"UID   PID   PPID  C  STIME  TTY  TIME      CMD",
		"root  4821  4800  0  09:12  ?    00:00:12  nginx: master process",
		"nginx 40  4821  0  09:12  ?    00:00:03  nginx: worker",
	})
	col := strings.Index(top[0], "PID")
	for i, line := range top[1:] {
		if got := strings.Index(line, strings.Fields(line)[1]); got != col {
			t.Fatalf("linha %d: PID na coluna %d, cabeçalho na %d", i+2, got, col)
		}
	}

	env := layoutDetailLines(containerDetailTabEnv, []string{"PATH=/usr/bin", "NGINX_VERSION=1.27"})
	if a, b := strings.Index(env[0], "/usr"), strings.Index(env[1], "1.27"); a != b {
		t.Fatalf("valores do env desalinhados: %d vs %d", a, b)
	}
}
