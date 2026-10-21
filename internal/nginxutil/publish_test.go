package nginxutil

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/config"
)

// testHub monta ~/projetos num tempdir: nginx-main/sites é o hub, e os
// projetos são irmãos dele — como na VPS.
func testHub(t *testing.T) Hub {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "nginx-main", "sites"))
	dockerPS = func() string { return "" }
	t.Cleanup(func() {
		dockerPS = func() string { out, _ := runCmd("docker", "ps", "--format", "{{.Names}}\t{{.Ports}}"); return out }
	})
	return Hub{
		Dir:            filepath.Join(root, "nginx-main"),
		SitesDir:       filepath.Join(root, "nginx-main", "sites"),
		Container:      "nginx-main",
		PublicURL:      "http://178.104.78.64",
		Gateway:        "172.17.0.1",
		HostRoot:       root,
		ContainerRoot:  "/var/www/html",
		BodySize:       "20m",
		ReadTimeout:    "120s",
		DistCandidates: defaultDistCandidates,
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func put(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// As referências que rodam em produção, geradas a partir do plano.
func TestRenderMatchesGoldenFiles(t *testing.T) {
	h := testHub(t)
	cases := map[string]Plan{
		"financial-bot": {Slug: "financial-bot", Dir: filepath.Join(h.HostRoot, "financial-bot"), Profile: ProfileSaaS,
			Port: 8001, Dist: "frontend/dist", Extras: []string{"/portal/"}},
		"peppercore-admin": {Slug: "peppercore-admin", Dir: filepath.Join(h.HostRoot, "peppercore-admin"), Profile: ProfileFront,
			Dist: "dist"},
	}
	for name, plan := range cases {
		want, err := os.ReadFile(filepath.Join("testdata", name+".inc"))
		if err != nil {
			t.Fatal(err)
		}
		if got := Render(plan, h); got != string(want) {
			t.Fatalf("%s difere do golden:\n%s", name, got)
		}
	}
}

func TestRenderReadPlanRoundTrip(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "app")
	for _, p := range []Plan{
		{Slug: "app", Dir: dir, Profile: ProfileSaaS, Port: 8001, Dist: "frontend/dist", BodySize: "50m", ReadTimeout: "300s", Extras: []string{"/portal/"}},
		{Slug: "app", Dir: dir, Profile: ProfileFront, Dist: "dist"},
		{Slug: "app", Dir: dir, Profile: ProfileAPI, Port: 9000},
	} {
		first := Render(p, h)
		back := ReadPlan(first, dir, h)
		if again := Render(back, h); again != first {
			t.Fatalf("%s: render → ReadPlan → render mudou:\n%s\n---\n%s", p.Profile, first, again)
		}
		if back.Profile != p.Profile || back.Slug != p.Slug {
			t.Fatalf("%+v", back)
		}
		inc := parseInc("sites/app.inc", first)
		if (inc.ProxyPass != "") != p.Profile.HasAPI() || (inc.Root != "") != p.Profile.HasFront() {
			t.Fatalf("parseInc não lê a .inc gerada: %+v", inc)
		}
	}
}

// O financial-bot do jeito que ele está no repositório: BACKEND_PORT no .env
// ganha do padrão do compose.
func TestDetectEnvPortWinsOverCompose(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "financial-bot")
	put(t, filepath.Join(dir, ".env"), "# portas\nBACKEND_PORT=8001\n")
	put(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  backend:\n    ports:\n      - \"${BACKEND_PORT:-8000}:8000\"\n")
	put(t, filepath.Join(dir, "frontend", "dist", "index.html"), `<script src="/financial-bot/assets/x.js"></script>`)

	p, checks := DetectProject(dir, h)
	if p.Slug != "financial-bot" || p.Profile != ProfileSaaS || p.Port != 8001 || p.PortFrom != ".env BACKEND_PORT" || p.Dist != "frontend/dist" {
		t.Fatalf("%+v", p)
	}
	if c, blocked := Blocked(checks); blocked {
		t.Fatalf("não devia bloquear: %s", c.Text)
	}
}

func TestDetectComposeDefault(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "api-only")
	put(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  backend:\n    ports:\n      - \"${BACKEND_PORT:-8000}:8000\"\n")
	p := DetectPlan(dir, h)
	if p.Port != 8000 || p.PortFrom != "compose backend" || p.Profile != ProfileAPI {
		t.Fatalf("%+v", p)
	}
}

func TestDetectTwoPortsIsAmbiguous(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "dupla")
	put(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  backend:\n    ports:\n      - \"8001:8000\"\n      - \"127.0.0.1:9001:9000\"\n")
	p, checks := DetectProject(dir, h)
	if p.Port != 0 || len(p.PortChoices) != 2 || p.PortChoices[1].Value != "9001" {
		t.Fatalf("não pode chutar: %+v", p)
	}
	if c, blocked := Blocked(checks); !blocked || !strings.Contains(c.Text, "2 portas") {
		t.Fatalf("%+v", checks)
	}
}

func TestDetectDockerFallback(t *testing.T) {
	h := testHub(t)
	dockerPS = func() string {
		return "outro_backend\t0.0.0.0:5000->8000/tcp\nloja-backend-1\t0.0.0.0:7001->8000/tcp, :::7001->8000/tcp\n"
	}
	p := DetectPlan(filepath.Join(h.HostRoot, "loja"), h)
	if p.Port != 7001 || p.PortFrom != "docker loja-backend-1" {
		t.Fatalf("%+v", p)
	}
}

func TestDetectBuildPendingAndWrongBase(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "site")
	put(t, filepath.Join(dir, "frontend", "package.json"), `{"devDependencies": {"vite": "^5"}}`)
	put(t, filepath.Join(dir, "frontend", "vite.config.js"), "export default { base: process.env.VITE_BASE_PATH || '/' }\n")
	p, checks := DetectProject(dir, h)
	if !p.BuildPending || p.Dist != "frontend/dist" || p.Profile != ProfileFront {
		t.Fatalf("%+v", p)
	}
	if !hasCheck(checks, CheckWarn, "build pendente") || !hasCheck(checks, CheckWarn, "VITE_BASE_PATH") {
		t.Fatalf("%+v", checks)
	}

	// Com o build feito na base errada e a variável no .env do front.
	put(t, filepath.Join(dir, "frontend", "dist", "index.html"), `<script type="module" src="/assets/index.js"></script>`)
	put(t, filepath.Join(dir, "frontend", ".env.production"), "VITE_BASE_PATH=/site/\nVITE_API_URL=http://localhost:8001/api\n")
	_, checks = DetectProject(dir, h)
	for _, want := range []struct {
		l    Level
		text string
	}{{CheckOK, "vite base /site/"}, {CheckWarn, "aponta para /assets/"}, {CheckWarn, "não termina em /site/api"}} {
		if !hasCheck(checks, want.l, want.text) {
			t.Fatalf("faltou %q: %+v", want.text, checks)
		}
	}
}

func TestConflictWithAnotherIncBlocks(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "app")
	put(t, filepath.Join(dir, "dist", "index.html"), "<html>")
	put(t, filepath.Join(h.SitesDir, "velho.inc"), "location ^~ /app/ {\n    alias /x/;\n}\n")
	put(t, filepath.Join(h.SitesDir, "parecido.inc"), "location ^~ /app-v2/ {\n    alias /y/;\n}\n")
	_, checks := DetectProject(dir, h)
	c, blocked := Blocked(checks)
	if !blocked || !strings.HasSuffix(c.File, "velho.inc") {
		t.Fatalf("%+v", checks)
	}
	for _, x := range checks {
		if strings.Contains(x.Text, "parecido") {
			t.Fatal("/app-v2/ não é /app/")
		}
	}
}

// Republicar mantém o que foi ajustado à mão na .inc em produção.
func TestDetectKeepsExtrasFromPublishedInc(t *testing.T) {
	h := testHub(t)
	dir := filepath.Join(h.HostRoot, "financial-bot")
	golden, _ := os.ReadFile(filepath.Join("testdata", "financial-bot.inc"))
	put(t, h.IncPath("financial-bot"), strings.Replace(string(golden), "20m", "64m", 1))
	p := DetectPlan(dir, h)
	if len(p.Extras) != 1 || p.Extras[0] != "/portal/" || p.BodySize != "64m" {
		t.Fatalf("%+v", p)
	}
}

func hasCheck(checks []Check, l Level, text string) bool {
	for _, c := range checks {
		if c.Level == l && strings.Contains(c.Text, text) {
			return true
		}
	}
	return false
}

func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		out[e.Name()] = string(b)
	}
	return out
}

func fakeCmd(t *testing.T, fn func(args []string) (string, error)) {
	t.Helper()
	old := runCmd
	runCmd = func(_ string, args ...string) (string, error) { return fn(args) }
	t.Cleanup(func() { runCmd = old })
}

// nginx -t recusando deixa o hub exatamente como estava — republicação e
// publicação nova — e nada é recarregado.
func TestPublishRollsBackWhenTestFails(t *testing.T) {
	h := testHub(t)
	reloaded := false
	fakeCmd(t, func(args []string) (string, error) {
		if args[len(args)-1] == "reload" {
			reloaded = true
			return "", nil
		}
		return "nginx: [emerg] unknown directive \"x\" in /etc/nginx/sites/app.inc:7\nnginx: configuration file /etc/nginx/nginx.conf test failed", os.ErrInvalid
	})
	plan := Plan{Slug: "app", Dir: filepath.Join(h.HostRoot, "app"), Profile: ProfileAPI, Port: 8001}

	for _, before := range []map[string]string{
		{"outro.inc": "location /outro/ {}\n", "app.inc": "# versão antiga\n", "app.inc.bak": "# mais antiga\n"},
		{"outro.inc": "location /outro/ {}\n"},
	} {
		for _, name := range []string{"app.inc", "app.inc.bak"} {
			os.Remove(filepath.Join(h.SitesDir, name))
		}
		for name, content := range before {
			put(t, filepath.Join(h.SitesDir, name), content)
		}
		res := Publish(h, plan, nil)
		if res.Err == nil || res.TestLine != 7 {
			t.Fatalf("%+v", res)
		}
		if got := snapshotDir(t, h.SitesDir); len(got) != len(before) {
			t.Fatalf("hub mudou: %v", got)
		} else {
			for name, content := range before {
				if got[name] != content {
					t.Fatalf("%s mudou: %q", name, got[name])
				}
			}
		}
	}
	if reloaded {
		t.Fatal("recarregou depois de um nginx -t recusado")
	}
}

func TestPublishRunsStepsAndSmokeTest(t *testing.T) {
	h := testHub(t)
	smokeTries = 1
	t.Cleanup(func() { smokeTries = 3 })
	var calls []string
	fakeCmd(t, func(args []string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "nginx: configuration file /etc/nginx/nginx.conf test is successful", nil
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app":
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
		case "/app/":
			w.Header().Set("Content-Type", "text/html")
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	h.PublicURL = srv.URL

	dir := filepath.Join(h.HostRoot, "app")
	put(t, filepath.Join(dir, "dist", "index.html"), "<html>")
	plan := Plan{Slug: "app", Dir: dir, Profile: ProfileSaaS, Port: 8001, Dist: "dist"}
	var steps []Step
	res := Publish(h, plan, func(s Step) { steps = append(steps, s) })

	if got := strings.Join(calls, " | "); got != "exec nginx-main nginx -t | exec nginx-main nginx -s reload" {
		t.Fatalf("comandos: %s", got)
	}
	if b, _ := os.ReadFile(h.IncPath("app")); string(b) != Render(plan, h) {
		t.Fatal("a .inc publicada é a renderizada")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "container parado ou porta errada") {
		t.Fatalf("o 502 do back-end tem de aparecer: %v", res.Err)
	}
	final := map[int]StepState{}
	for _, s := range steps {
		final[s.Index] = s.State
	}
	if len(final) != 6 || final[4] != StepOK || final[5] != StepFail {
		t.Fatalf("passos: %+v", steps)
	}
}

func TestUnpublishKeepsBackup(t *testing.T) {
	h := testHub(t)
	fakeCmd(t, func([]string) (string, error) { return "", nil })
	put(t, h.IncPath("app"), "# publicada\n")
	if res := Unpublish(h, "app", nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	if _, err := os.Stat(h.IncPath("app")); !os.IsNotExist(err) {
		t.Fatal("a .inc tinha de sair")
	}
	if b, _ := os.ReadFile(h.IncPath("app") + ".bak"); string(b) != "# publicada\n" {
		t.Fatal("a anterior fica no .bak")
	}
}

// O hub vem do compose do nginx-main e do server_name do conf.d/.
func TestLoadHubDetectsFromCompose(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "projetos")
	hubDir := filepath.Join(projects, "nginx-main")
	mkdir(t, filepath.Join(hubDir, "sites"))
	put(t, filepath.Join(hubDir, "docker-compose.yml"), `services:
  nginx:
    image: nginx:alpine
    container_name: nginx-main
    volumes:
      - ./conf.d:/etc/nginx/conf.d:ro
      - ./sites:/etc/nginx/sites:ro
      - `+projects+`:/var/www/html:ro
`)
	put(t, filepath.Join(hubDir, "conf.d", "main.conf"), "server {\n    listen 80;\n    server_name 178.104.78.64;\n    include /etc/nginx/sites/*.inc;\n}\n")

	h := LoadHub(config.NginxPrefs{Dir: hubDir})
	if h.SitesDir != filepath.Join(hubDir, "sites") || h.HostRoot != projects || h.ContainerRoot != "/var/www/html" ||
		h.Container != "nginx-main" || h.PublicURL != "http://178.104.78.64" || !h.Available() {
		t.Fatalf("%+v", h)
	}
	if got := LoadHub(config.NginxPrefs{Dir: hubDir, PublicHost: "https://exemplo.com", Container: "outro"}); got.PublicURL != "https://exemplo.com" || got.Container != "outro" {
		t.Fatalf("o user_config.txt ganha da detecção: %+v", got)
	}
}

func TestLineDiff(t *testing.T) {
	got := LineDiff("a\nb\nc\n", "a\nx\nc\n")
	var ops string
	for _, d := range got {
		ops += string(d.Op)
	}
	if ops != " -+ " {
		t.Fatalf("%q", ops)
	}
}

func TestLaravelTrustProxies(t *testing.T) {
	private := "$middleware->trustProxies(at: array_map('trim', explode(',', env(\n    'TRUSTED_PROXIES',\n    '127.0.0.0/8,10.0.0.0/8,172.16.0.0/12',\n))));"
	if !trustsGateway(private, "172.17.0.1") || !trustsGateway("$middleware->trustProxies(at: '*');", "172.17.0.1") {
		t.Fatal("faixa privada e * confiam no gateway")
	}
	if trustsGateway("$middleware->trustProxies(at: ['10.0.0.0/8']);", "172.17.0.1") {
		t.Fatal("10/8 não contém o gateway")
	}
}
