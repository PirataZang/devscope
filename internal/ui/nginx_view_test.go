package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/nginxutil"
)

func nginxConsoleApp(w, h int) *App {
	p := core.Project{Name: "laradock", Path: "/home/igor/x/laradock"}
	sites := []nginxutil.Site{
		{File: "sites/apihub.conf", Name: "apihub", Kind: nginxutil.KindHub,
			HubDir: "/home/igor/x/laradock/sites/apihub", ServerNames: []string{"api.digiliza.com.br"},
			Listen: "443 ssl", SSL: true,
			Raw: "server {\n    listen 443 ssl;\n    ssl_certificate /etc/letsencrypt/live/api.digiliza.com.br/fullchain.pem; # renovado pelo certbot todo mês, não editar na mão\n}"},
		{File: "sites/checkout.conf", Name: "checkout", Kind: nginxutil.KindSingle,
			ServerNames: []string{"checkout.digiliza.com.br"}, Listen: "80",
			ProxyPass: "http://127.0.0.1:8080", Project: "digiliza-checkout",
			Raw: "server {\n    listen 80;\n}"},
	}
	return &App{
		width: w, height: h, view: ViewProject, tab: TabNginx, now: time.Now(),
		selectedProject: &p, nginxOpen: true, nginxSites: sites, nginxForeign: 1,
		nginxLayout: nginxutil.Layout{SitesDir: "/home/igor/x/laradock/sites"},
		snapshot:    core.Snapshot{Projects: []core.Project{p}},
	}
}

func TestNginxConsoleFitsAnyTerminal(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120, 160, 200} {
		for _, view := range []nginxView{nginxViewRoutes, nginxViewFile} {
			a := nginxConsoleApp(w, 30)
			a.nginxView = view
			out := a.renderNginxTab(a.selectedProject)
			for i, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > w {
					t.Fatalf("aba %d, terminal %d: linha %d tem %d colunas", view, w, i+1, got)
				}
			}
			if got := lipgloss.Height(out); got != a.screenHeight() {
				t.Fatalf("terminal %d: altura %d, esperado %d", w, got, a.screenHeight())
			}
		}
	}
}

// As colunas saem da largura do painel: a tabela divide a tela com o arquivo.
func TestNginxColumnsNeverExceedPanel(t *testing.T) {
	for _, w := range []int{40, 50, 60, 80, 120, 200} {
		for _, topLevel := range []bool{true, false} {
			c := nginxColumns(w, topLevel, true)
			if got := nginxColsWidth(c); got > w {
				t.Fatalf("painel %d (topo=%v): colunas somam %d", w, topLevel, got)
			}
		}
	}
}

// Número troca de aba, seta rola de lado, esc sobe um nível sem fechar o módulo.
func TestNginxNavigationKeys(t *testing.T) {
	a := nginxConsoleApp(120, 30)
	p := a.selectedProject

	a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, p)
	if a.nginxView != nginxViewFile {
		t.Fatal("2 devia abrir a aba ARQUIVO")
	}
	a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if a.nginxFileHScroll == 0 {
		t.Fatal("seta direita precisa rolar o arquivo de lado")
	}
	if a.nginxView != nginxViewFile {
		t.Fatal("seta não pode trocar de aba")
	}
	a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")}, p)
	if a.nginxFileHScroll != 0 {
		t.Fatalf("0 volta ao início da linha, veio %d", a.nginxFileHScroll)
	}

	a.handleNginxKeys(tea.KeyMsg{Type: tea.KeyEsc}, p)
	if a.nginxView != nginxViewRoutes || !a.nginxOpen {
		t.Fatalf("esc na aba ARQUIVO volta para ROTAS sem fechar: view=%d open=%v", a.nginxView, a.nginxOpen)
	}
}

func TestNginxHighlightOnlyPaints(t *testing.T) {
	for _, line := range []string{
		"server {",
		"    listen 443 ssl;",
		"    proxy_pass http://127.0.0.1:8080;",
		"    include /etc/nginx/sites-available/apihub/*.inc;",
		"# comentário com ; e { dentro",
		"}",
		"",
	} {
		if got := stripANSI(highlightNginxLine(line)); got != line {
			t.Fatalf("realce alterou o texto:\n want %q\n  got %q", line, got)
		}
	}
}

// Rota de outro projeto continua identificável mesmo quando a coluna PROJETO
// não cabe: o nome do arquivo vai em amarelo.
func TestNginxForeignRouteIsMarked(t *testing.T) {
	a := nginxConsoleApp(120, 30)
	own := a.nginxNameStyle(a.nginxSites[0])
	foreign := a.nginxNameStyle(a.nginxSites[1])
	if own.GetForeground() == foreign.GetForeground() {
		t.Fatal("rota de outro projeto precisa de cor diferente da do projeto aberto")
	}
	if foreign.GetForeground() != StyleWarning.GetForeground() {
		t.Fatalf("esperado amarelo de aviso, veio %v", foreign.GetForeground())
	}
}
