package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

// drainNginxPub roda os comandos da publicação até a fila acabar, como o
// Bubble Tea faria.
func drainNginxPub(a *App, cmd tea.Cmd) {
	for cmd != nil {
		cmd = a.handleNginxPublishMsg(cmd())
	}
}

// Critério de pronto: projeto novo no padrão, p → enter, sem digitar caminho
// nem porta — .inc escrita, nginx -t, reload e os três GETs verdes.
func TestNginxPublishPThenEnter(t *testing.T) {
	root := t.TempDir()
	hubDir := filepath.Join(root, "nginx-main")
	writeTestFile(t, filepath.Join(hubDir, "docker-compose.yml"), "services:\n  nginx:\n    image: nginx:alpine\n    container_name: nginx-main\n    volumes:\n      - ./sites:/etc/nginx/sites:ro\n      - "+root+":/var/www/html:ro\n")
	if err := os.MkdirAll(filepath.Join(hubDir, "sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	writeTestFile(t, filepath.Join(app, ".env"), "BACKEND_PORT=8001\n")
	writeTestFile(t, filepath.Join(app, "frontend", "dist", "index.html"), `<script src="/app/assets/i.js"></script>`)

	bin := t.TempDir()
	writeTestFile(t, filepath.Join(bin, "docker"), "#!/bin/sh\necho \"$*\" >> \""+filepath.Join(bin, "log")+"\"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app":
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
		case "/app/":
			w.Header().Set("Content-Type", "text/html")
		default:
			w.WriteHeader(http.StatusNotFound) // o Laravel respondeu: não é 502
		}
	}))
	defer srv.Close()

	p := core.Project{Path: app, Name: "app"}
	a := &App{
		width: 140, height: 48, view: ViewProject, tab: TabNginx,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		cfg: &config.Config{Nginx: config.NginxPrefs{Dir: hubDir, PublicHost: srv.URL}},
	}
	a.handleToolLandingMsg(a.probeToolLanding(TabNginx, &p)().(toolLandingMsg))
	if landing := stripANSI(a.renderNginxLanding(&p)); !strings.Contains(landing, "não publicado") || !strings.Contains(landing, "/app/") {
		t.Fatalf("a abertura oferece publicar:\n%s", landing)
	}

	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	drainNginxPub(a, cmd)
	view := stripANSI(a.renderNginxPublish(&p))
	for _, want := range []string{"PUBLICAR · app", "8001", ".env BACKEND_PORT", "frontend/dist", "✓ back-end na porta 8001", "proxy_pass http://172.17.0.1:8001;"} {
		if !strings.Contains(view, want) {
			t.Fatalf("faltou %q:\n%s", want, view)
		}
	}

	_, cmd = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	drainNginxPub(a, cmd)
	res := a.nginxPub.result
	if res == nil || res.Err != nil {
		t.Fatalf("publicação: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(hubDir, "sites", "app.inc")); err != nil {
		t.Fatal("a .inc tinha de estar no hub")
	}
	log, _ := os.ReadFile(filepath.Join(bin, "log"))
	if string(log) != "exec nginx-main nginx -t\nexec nginx-main nginx -s reload\n" {
		t.Fatalf("docker: %q", log)
	}
	view = stripANSI(a.renderNginxPublish(&p))
	for _, want := range []string{"✓ GET /app → 301", "✓ GET /app/ → 200 html", "✓ GET /app/api/ → back-end", "no ar", srv.URL + "/app/"} {
		if !strings.Contains(view, want) {
			t.Fatalf("faltou %q:\n%s", want, view)
		}
	}

	// Republicar com outra porta pede confirmação e mostra o diff.
	a.nginxPub.plan.Port = 9000
	a.recheckNginxPublish()
	if view = stripANSI(a.renderNginxPublish(&p)); !strings.Contains(view, "- ") || !strings.Contains(view, "diff com a publicada") {
		t.Fatalf("diff:\n%s", view)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.nginxPub.confirm {
		t.Fatal("sobrescrever pede confirmação")
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEsc})
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEsc})
	if a.nginxPub != nil {
		t.Fatal("esc fecha a revisão")
	}
}

// Duas portas: e cai no campo da porta, ↑↓ percorre as candidatas e enter
// aplica — e a checagem que bloqueava some.
func TestNginxPublishEditResolvesAmbiguity(t *testing.T) {
	root := t.TempDir()
	hubDir := filepath.Join(root, "nginx-main")
	if err := os.MkdirAll(filepath.Join(hubDir, "sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "dupla")
	writeTestFile(t, filepath.Join(app, "docker-compose.yml"), "services:\n  backend:\n    ports:\n      - \"8001:8000\"\n      - \"9001:9000\"\n")
	p := core.Project{Path: app, Name: "dupla"}
	a := &App{
		width: 140, height: 48, view: ViewProject, tab: TabNginx, selectedProject: &p,
		cfg: &config.Config{Nginx: config.NginxPrefs{Dir: hubDir, HostRoot: root, ContainerRoot: "/var/www/html"}},
	}
	drainNginxPub(a, a.startNginxPublish(&p))
	if _, blocked := firstBlocked(a); !blocked {
		t.Fatal("duas portas bloqueiam")
	}
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("e")},
		{Type: tea.KeyDown},
		{Type: tea.KeyDown},
		{Type: tea.KeyEnter},
	} {
		_, _ = a.updateKey(k)
	}
	if a.nginxPub.plan.Port != 9001 || a.nginxPub.editing {
		t.Fatalf("%+v", a.nginxPub.plan)
	}
	if c, blocked := firstBlocked(a); blocked {
		t.Fatalf("ainda bloqueado: %s", c)
	}
}

func firstBlocked(a *App) (string, bool) {
	for _, c := range a.nginxPub.checks {
		if c.Level == 2 {
			return c.Text, true
		}
	}
	return "", false
}

// No próprio nginx-main o p pergunta qual projeto.
func TestNginxPublishPickerOnHub(t *testing.T) {
	root := t.TempDir()
	hubDir := filepath.Join(root, "nginx-main")
	if err := os.MkdirAll(filepath.Join(hubDir, "sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(hubDir, "sites", "loja.inc"), "location = /loja {}\n")
	hub := core.Project{Path: hubDir, Name: "nginx-main"}
	loja := core.Project{Path: filepath.Join(root, "loja"), Name: "loja"}
	fora := core.Project{Path: "/opt/fora", Name: "fora"}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabNginx, selectedProject: &hub,
		snapshot: core.Snapshot{Projects: []core.Project{hub, loja, fora}},
		cfg:      &config.Config{Nginx: config.NginxPrefs{Dir: hubDir, HostRoot: root}},
	}
	a.startNginxPublish(&hub)
	if !a.nginxPub.picker || len(a.nginxPub.choices) != 1 {
		t.Fatalf("só projetos dentro de ~/projetos, sem o próprio hub: %+v", a.nginxPub.choices)
	}
	if view := stripANSI(a.renderNginxPublish(&hub)); !strings.Contains(view, "publicado em /loja/") {
		t.Fatalf("%s", view)
	}
}
