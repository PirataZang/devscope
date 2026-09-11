package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

func helpTestApp(w, h int) *App {
	return &App{
		width: w, height: h, view: ViewDashboard, helpOn: true,
		cfg:      &config.Config{},
		snapshot: core.Snapshot{Projects: []core.Project{{Name: "api", Path: "/var/www/api"}}},
	}
}

// Cada grupo usa a cor que tem na barra lateral: quem procura o comando do Git
// procura no amarelo, onde o módulo mora.
func TestHelpGroupsMatchSidebarColors(t *testing.T) {
	want := map[string]lipgloss.Color{}
	for _, g := range sidebarGroups() {
		want[g.title] = g.color
	}
	for _, g := range helpGroups() {
		if g.title == "GERAL" {
			continue // não é um grupo da sidebar; é o global da aplicação
		}
		c, ok := want[g.title]
		if !ok {
			t.Fatalf("grupo %q não existe na barra lateral", g.title)
		}
		if g.color != c {
			t.Fatalf("grupo %q: cor %v, sidebar usa %v", g.title, g.color, c)
		}
	}
}

// Cada tela do DevScope tem que ter um bloco próprio dentro do grupo dela.
func TestHelpHasABlockPerScreen(t *testing.T) {
	blocks := map[string]string{}
	for _, g := range helpGroups() {
		for _, b := range g.blocks {
			if prev, dup := blocks[b.title]; dup {
				t.Fatalf("bloco %q repetido (%s e %s)", b.title, prev, g.title)
			}
			blocks[b.title] = g.title
			if len(b.entries) == 0 {
				t.Fatalf("bloco %q sem nenhum comando", b.title)
			}
		}
	}
	for _, want := range []string{
		"GIT · BRANCHES", "CONTAINERS", "NGINX", "NGROK", "SSH TUNNEL", "CLOUDFLARE TUNNEL",
		"GITHUB ACTIONS", "JENKINS", "SWARM", "KUBERNETES", "API", "DATABASE", "WEBSOCKET",
	} {
		if _, ok := blocks[want]; !ok {
			t.Fatalf("tela %q não aparece na ajuda", want)
		}
	}
}

func TestHelpScreenFitsAnyTerminal(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120, 160, 200} {
		a := helpTestApp(w, 30)
		for tab := range helpGroups() {
			a.helpTab = tab
			a.helpScroll = 0
			out := a.renderHelpScreen(a.renderCurrentView())
			for i, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > w {
					t.Fatalf("grupo %d, terminal %d: linha %d tem %d colunas", tab, w, i+1, got)
				}
			}
		}
	}
}

// Número abre o grupo, seta circula, esc fecha — o que a barra promete.
func TestHelpNavigationKeys(t *testing.T) {
	a := helpTestApp(120, 30)
	a.updateHelp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if a.helpTab != 2 {
		t.Fatalf("3 devia abrir o terceiro grupo, veio %d", a.helpTab)
	}
	a.updateHelp(tea.KeyMsg{Type: tea.KeyLeft})
	if a.helpTab != 1 {
		t.Fatalf("seta esquerda: %d", a.helpTab)
	}
	a.helpTab = 0
	a.updateHelp(tea.KeyMsg{Type: tea.KeyLeft})
	if a.helpTab != helpGroupCount()-1 {
		t.Fatal("no primeiro grupo a seta esquerda deve dar a volta")
	}
	a.updateHelp(tea.KeyMsg{Type: tea.KeyEsc})
	if a.helpOn {
		t.Fatal("esc precisa fechar a ajuda")
	}
}

// Tecla em caixa alta é sempre Shift: "A" sozinho não diz como digitar.
func TestHelpSpellsOutShift(t *testing.T) {
	for _, g := range helpGroups() {
		for _, b := range g.blocks {
			for _, e := range b.entries {
				for _, token := range strings.Fields(e.keys) {
					if len([]rune(token)) == 1 && token >= "A" && token <= "Z" {
						t.Fatalf("%s · %s: %q devia ser shift+%s", g.title, b.title, e.keys, strings.ToLower(token))
					}
				}
			}
		}
	}
}
