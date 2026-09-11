package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// As larguras em que o app já quebrou antes.
var foundationWidths = []int{40, 60, 80, 100, 120, 160, 200}

func TestRuleNeverExceedsWidth(t *testing.T) {
	for _, w := range foundationWidths {
		if got := lipgloss.Width(rule(w)); got != w {
			t.Errorf("rule(%d) mede %d colunas", w, got)
		}
		if got := lipgloss.Width(ruleColored(w, ColorAccent)); got != w {
			t.Errorf("ruleColored(%d) mede %d colunas", w, got)
		}
	}
	for _, w := range []int{0, -1, -80} {
		if got := rule(w); got != "" {
			t.Errorf("rule(%d) devia ser vazio, veio %q", w, got)
		}
		if got := ruleColored(w, ColorAccent); got != "" {
			t.Errorf("ruleColored(%d) devia ser vazio, veio %q", w, got)
		}
	}
}

func TestSectionHeaderFitsAndKeepsTitle(t *testing.T) {
	for _, w := range foundationWidths {
		got := sectionHeader("BRANCHES", w, ColorAccent)
		if lipgloss.Width(got) > w {
			t.Errorf("sectionHeader em %d colunas estourou para %d", w, lipgloss.Width(got))
		}
		if !strings.Contains(stripANSI(got), "BRANCHES") {
			t.Errorf("sectionHeader em %d colunas perdeu o nome: %q", w, stripANSI(got))
		}
	}
	// Apertado: a régua some antes do nome — o nome é o dado.
	tight := stripANSI(sectionHeader("BRANCHES", 8, ColorAccent))
	if strings.Contains(tight, "─") {
		t.Errorf("em 8 colunas a régua devia sumir, veio %q", tight)
	}
	if lipgloss.Width(sectionHeader("BRANCHES", 5, ColorAccent)) > 5 {
		t.Error("sectionHeader não corta o nome quando não cabe")
	}
	if sectionHeader("X", 0, ColorAccent) != "" {
		t.Error("sectionHeader com largura 0 devia ser vazio")
	}
}

func TestPanelTitleUsesSeparatorAndSkipsEmpty(t *testing.T) {
	cases := []struct {
		name  string
		parts []string
		want  string
	}{
		{"LISTA", nil, "LISTA"},
		{"LISTA", []string{"12"}, "LISTA · 12"},
		{"RUNS", []string{"8/40", "main"}, "RUNS · 8/40 · main"},
		{"RUNS", []string{"", "  ", "main"}, "RUNS · main"},
		{"  LOGS  ", []string{"3"}, "LOGS · 3"},
	}
	for _, c := range cases {
		if got := panelTitle(c.name, c.parts...); got != c.want {
			t.Errorf("panelTitle(%q, %v) = %q, queria %q", c.name, c.parts, got, c.want)
		}
	}
}

func TestKeyHintCarriesKeyAndDescription(t *testing.T) {
	got := stripANSI(keyHint("shift+P", "push"))
	if !strings.Contains(got, "shift+P") || !strings.Contains(got, "push") {
		t.Errorf("keyHint perdeu conteúdo: %q", got)
	}
	if got := stripANSI(keyHint("q", "")); got != "q" {
		t.Errorf("keyHint sem descrição devia ser só a tecla, veio %q", got)
	}
}

func TestPanelBoxRendersExactBox(t *testing.T) {
	long := strings.Repeat("conteúdo muito comprido ", 20)
	for _, w := range foundationWidths {
		for _, h := range []int{3, 6, 12, 24} {
			got := panelBox(panelTitle("LISTA", "42"), []string{long, long}, w, h, false)
			lines := strings.Split(got, "\n")
			if len(lines) != h {
				t.Fatalf("panelBox %dx%d rendeu %d linhas", w, h, len(lines))
			}
			for i, l := range lines {
				if lipgloss.Width(l) != w {
					t.Fatalf("panelBox %dx%d: linha %d mede %d colunas", w, h, i, lipgloss.Width(l))
				}
			}
		}
	}
}

func TestPadRightVisibleCountsColumnsNotBytes(t *testing.T) {
	styled := StyleHealthy.Render("⣿ rodando")
	for _, w := range []int{4, 9, 20, 40} {
		if got := lipgloss.Width(padRightVisible(styled, w)); got != w {
			t.Errorf("padRightVisible(estilizado, %d) mede %d", w, got)
		}
	}
	// Acentuado: "ção" tem 3 colunas e 5 bytes.
	if got := lipgloss.Width(padRightVisible("ação", 10)); got != 10 {
		t.Errorf("padRightVisible acentuado mede %d", got)
	}
}

// TestGlyphVocabularyIsSingleColumn: as duas libs precisam CONCORDAR que o
// glifo ocupa 1 coluna. Onde discordam, a linha sai alinhada num terminal e
// torta no outro — foi assim que ⚡ e ☰ quebraram a sidebar.
func TestGlyphVocabularyIsSingleColumn(t *testing.T) {
	for _, g := range glyphVocabulary {
		rw, lw := runewidth.StringWidth(g), lipgloss.Width(g)
		if rw != 1 || lw != 1 {
			t.Errorf("glifo %q: runewidth=%d lipgloss=%d — só entra no vocabulário o que mede 1 nas duas", g, rw, lw)
		}
	}
	for _, g := range glyphBanned {
		if runewidth.StringWidth(g) == 1 && lipgloss.Width(g) == 1 {
			t.Errorf("glifo %q está banido mas mede 1 coluna nas duas libs — tire da lista", g)
		}
	}
}

// TestTabGlyphsAreSingleColumn: a sidebar alinha nome e realce pela largura do
// glifo do módulo. Módulo novo com glifo de 2 colunas desalinha a coluna toda.
func TestTabGlyphsAreSingleColumn(t *testing.T) {
	for _, g := range sidebarGroups() {
		for _, tab := range g.tabs {
			glyph := tabGlyph(tab)
			if runewidth.StringWidth(glyph) != 1 || lipgloss.Width(glyph) != 1 {
				t.Errorf("tabGlyph(%v) = %q mede mais de 1 coluna", tab, glyph)
			}
		}
	}
}

// TestNoBannedGlyphInSource varre o código: glifo de largura ambígua não pode
// voltar por um módulo novo. Comentários ficam de fora — é onde a lista e o
// motivo estão escritos.
func TestNoBannedGlyphInSource(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f == "foundation.go" || f == "foundation_test.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			for _, g := range glyphBanned {
				if strings.Contains(line, g) {
					t.Errorf("%s:%d usa o glifo banido %q (docs/DESIGN.md §8.4)", f, i+1, g)
				}
			}
		}
	}
}

// TestDensityTiers trava as três faixas: são elas que decidem o que cabe, e
// mudar um número aqui muda o layout de todo módulo.
func TestDensityTiers(t *testing.T) {
	cases := []struct {
		w, h                    int
		tiny, compact, dashComp bool
	}{
		{200, 60, false, false, false},
		{120, 40, false, false, false},
		{120, 30, false, true, false}, // altura 30: compacto no módulo (< 34), ainda folgado na tela inicial (< 28)
		{100, 40, false, true, false},
		{80, 24, false, true, true},
		{80, 20, true, true, true},
		{0, 0, false, false, false}, // antes do primeiro WindowSizeMsg
	}
	for _, c := range cases {
		a := &App{width: c.w, height: c.h}
		if got := a.projectTiny(); got != c.tiny {
			t.Errorf("%dx%d projectTiny=%v, queria %v", c.w, c.h, got, c.tiny)
		}
		if got := a.projectCompact(); got != c.compact {
			t.Errorf("%dx%d projectCompact=%v, queria %v", c.w, c.h, got, c.compact)
		}
		if got := a.dashboardCompact(); got != c.dashComp {
			t.Errorf("%dx%d dashboardCompact=%v, queria %v", c.w, c.h, got, c.dashComp)
		}
	}
}

// TestFactLabelsFitTheColumn: o rótulo tem largura fixa (factLabelW) e é ele
// que alinha as linhas de fato umas com as outras. Rótulo maior é cortado com
// "…" — "manifests" virava "manifes…", que não é palavra nenhuma.
func TestFactLabelsFitTheColumn(t *testing.T) {
	for _, label := range []string{
		"stack", "runtime", "git", "portas", "recursos", "cpu  g", "mem  g", "net  g",
		"cli", "agente", "config", "cliente", "ativos", "login", "achado", "entradas",
		"tipos", "contexto", "yaml", "nodes", "salvos", "local", "fluxo", "url",
		"últimas", "repo", "gh", "fluxos", "servidor", "usuário", "alg", "modos",
		"extra", "entrada", "saída", "faz", "lê", "destino", "aceita", "origem", "follow",
	} {
		if w := lipgloss.Width(label); w > factLabelW {
			t.Errorf("rótulo %q mede %d colunas, o teto é %d — seria cortado", label, w, factLabelW)
		}
		if got := stripANSI(factLine(label, []string{"x"}, 60)); strings.Contains(got, "…") {
			t.Errorf("rótulo %q foi truncado: %q", label, got)
		}
	}
}
