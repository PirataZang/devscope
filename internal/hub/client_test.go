package hub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

// fakeHub é o hub de mentira: cada rota responde com uma função, e toda
// requisição fica registrada para o teste conferir.
type fakeHub struct {
	*httptest.Server
	mu     sync.Mutex
	calls  []fakeCall
	routes map[string]func(w http.ResponseWriter, body map[string]any)
}

type fakeCall struct {
	path   string
	body   map[string]any
	header http.Header
}

func newFakeHub(t *testing.T, routes map[string]func(http.ResponseWriter, map[string]any)) *fakeHub {
	f := &fakeHub{routes: routes}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		f.calls = append(f.calls, fakeCall{r.URL.Path, body, r.Header.Clone()})
		f.mu.Unlock()
		if h, ok := f.routes[r.URL.Path]; ok {
			h(w, body)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeHub) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if path == "" || c.path == path {
			n++
		}
	}
	return n
}

func (f *fakeHub) last(path string) fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].path == path {
			return f.calls[i]
		}
	}
	return fakeCall{}
}

func reply(status int, body string, header ...string) func(http.ResponseWriter, map[string]any) {
	return func(w http.ResponseWriter, _ map[string]any) {
		for i := 0; i+1 < len(header); i += 2 {
			w.Header().Set(header[i], header[i+1])
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// sequence responde cada chamada com o próximo corpo; o último se repete.
func sequence(bodies ...string) func(http.ResponseWriter, map[string]any) {
	var mu sync.Mutex
	i := 0
	return func(w http.ResponseWriter, _ map[string]any) {
		mu.Lock()
		b := bodies[min(i, len(bodies)-1)]
		i++
		mu.Unlock()
		_, _ = io.WriteString(w, b)
	}
}

func stub[T any](t *testing.T, p *T, v T) {
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

// isolate dá ao teste um HOME vazio e tira gh e git do caminho.
func isolate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stub(t, &ghaAvailable, func() bool { return false })
	stub(t, &hasWorkflows, func(string) bool { return false })
	stub(t, &pollWait, 0)
	indicator.Store(indicatorOff)
}

func pairMachine(t *testing.T) string {
	t.Helper()
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKey(key); err != nil {
		t.Fatal(err)
	}
	if err := UpdateState(func(s *State) { s.Owner = &Owner{Username: "fulano", IDMasked: "5512••••"} }); err != nil {
		t.Fatal(err)
	}
	return key
}

func testConfig(url string) *config.Config {
	cfg := config.Default()
	cfg.Bot.HubURL = url
	cfg.Bot.MachineName = "vps-digiliza"
	return cfg
}

func TestNewKeyFormat(t *testing.T) {
	isolate(t)
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "dsk_") || len(key) != 4+52 || strings.ToLower(key) != key || strings.Contains(key, "=") {
		t.Fatalf("key fora do formato: %q (%d)", key, len(key))
	}
	if err := SaveKey(key); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(KeyPath())
	di, _ := os.Stat(Dir())
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatalf("permissões: key %v, dir %v", fi.Mode().Perm(), di.Mode().Perm())
	}
	if got, _ := LoadKey(); got != key {
		t.Fatalf("LoadKey = %q", got)
	}
}

func TestClientPairingRoutes(t *testing.T) {
	isolate(t)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/register": reply(200, `{"machine_id":"m_7f3a9c21"}`),
		"/v1/pair": reply(200, `{"code":"K7F-29Q","expires_at":"2026-10-21T21:24:00Z","bot":"devscope_vps_bot",
			"link":"https://t.me/devscope_vps_bot?start=K7F-29Q"}`),
		"/v1/pair/wait": sequence(`{}`,
			`{"request":{"request_id":"pr_1","chat":{"username":"fulano","name":"Fulano Silva","id_masked":"5512••••"}}}`,
			`{"expired":true}`),
		"/v1/pair/confirm": reply(200, `{"owner":{"username":"fulano","id_masked":"5512••••"}}`),
	})
	c := NewClient(f.URL+"/", "dsk_test")
	ctx := context.Background()

	id, err := c.Register(ctx, "vps-digiliza", "linux")
	if err != nil || id != "m_7f3a9c21" {
		t.Fatalf("register = %q, %v", id, err)
	}
	call := f.last("/v1/register")
	if call.body["machine_name"] != "vps-digiliza" || call.body["os"] != "linux" || call.body["devscope_version"] == "" {
		t.Fatalf("corpo do register: %v", call.body)
	}
	if call.header.Get("Authorization") != "Bearer dsk_test" || call.header.Get("X-DevScope-Protocol") != "1" ||
		!strings.HasPrefix(call.header.Get("User-Agent"), "devscope/") {
		t.Fatalf("cabeçalhos: %v", call.header)
	}

	code, err := c.Pair(ctx)
	if err != nil || code.Code != "K7F-29Q" || code.ExpiresAt.IsZero() || code.Link == "" {
		t.Fatalf("pair = %+v, %v", code, err)
	}

	w, err := c.PairWait(ctx, "K7F-29Q", 50)
	if err != nil || w.Request != nil || w.Expired {
		t.Fatalf("wait vazio = %+v, %v", w, err)
	}
	if b := f.last("/v1/pair/wait").body; b["code"] != "K7F-29Q" || b["wait"] != float64(50) {
		t.Fatalf("corpo do wait: %v", b)
	}
	w, err = c.PairWait(ctx, "K7F-29Q", 50)
	if err != nil || w.Request == nil || w.Request.Chat.Username != "fulano" || w.Request.Chat.Name != "Fulano Silva" {
		t.Fatalf("wait com pedido = %+v, %v", w, err)
	}
	w, err = c.PairWait(ctx, "K7F-29Q", 50)
	if err != nil || !w.Expired {
		t.Fatalf("wait vencido = %+v, %v", w, err)
	}

	owner, err := c.PairConfirm(ctx, "pr_1", true)
	if err != nil || owner.Username != "fulano" || owner.IDMasked != "5512••••" {
		t.Fatalf("confirm = %+v, %v", owner, err)
	}
	if b := f.last("/v1/pair/confirm").body; b["request_id"] != "pr_1" || b["accept"] != true {
		t.Fatalf("corpo do confirm: %v", b)
	}
}

func TestClientErrors(t *testing.T) {
	isolate(t)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll":     reply(401, `{"error":"unauthorized","message":"esta máquina foi desconectada"}`),
		"/v1/register": reply(426, `{"error":"upgrade_required","message":"Este hub fala o protocolo 1. Atualize o DevScope."}`),
		"/v1/events":   reply(403, `<html><title>Error 403 No valid crumb was included in the request</title></html>`),
		"/v1/result":   reply(502, `<html>Bad Gateway</html>`),
		"/v1/pair":     reply(429, `{"error":"rate_limited","message":"muitos códigos"}`, "Retry-After", "7"),
		"/v1/snapshot": reply(403, `{"error":"registration_closed","message":"cadastro fechado"}`),
	})
	c := NewClient(f.URL, "dsk_test")
	ctx := context.Background()

	_, err := c.Poll(ctx, "tui", false, 0)
	if !IsCode(err, CodeUnauthorized) || err.Error() != "esta máquina foi desconectada" {
		t.Fatalf("401: %v", err)
	}
	_, err = c.Register(ctx, "x", "linux")
	if !IsCode(err, CodeUpgrade) || err.Error() != "Este hub fala o protocolo 1. Atualize o DevScope." {
		t.Fatalf("426: %v", err)
	}
	_, err = c.Pair(ctx)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeRateLimited || e.RetryAfter != 7*time.Second || e.Message != "muitos códigos" {
		t.Fatalf("429: %#v", err)
	}
	var b backoff
	if d := b.next(err); d != 7*time.Second {
		t.Fatalf("429 deve esperar o Retry-After, esperou %v", d)
	}
	if err := c.Snapshot(ctx, Snapshot{}); !IsCode(err, CodeRegistrationClosed) {
		t.Fatalf("403: %v", err)
	}
	// Outro serviço na porta (o Jenkins, por exemplo): 403 sem o corpo do
	// Contrato não é registration_closed, e muito menos unauthorized.
	err = c.Events(ctx, nil)
	if !IsCode(err, CodeNotHub) || !strings.Contains(err.Error(), "não é o hub do devscope: confira bot.hub_url") {
		t.Fatalf("403 de outro serviço: %v", err)
	}
	if err := c.Result(ctx, Result{}); !IsCode(err, CodeOffline) {
		t.Fatalf("502 do proxy: %v", err)
	}

	f.Close()
	err = c.Bye(ctx)
	if !IsCode(err, CodeOffline) || err.Error() != "não consegui falar com "+c.Host() {
		t.Fatalf("sem rede: %v", err)
	}
}

func TestBackoff(t *testing.T) {
	var b backoff
	var got []time.Duration
	for range 8 {
		got = append(got, b.next(errors.New("x")))
	}
	want := []time.Duration{1, 2, 4, 8, 16, 32, 60, 60}
	for i := range want {
		if got[i] != want[i]*time.Second {
			t.Fatalf("backoff = %v", got)
		}
	}
	b.reset()
	if b.next(nil) != time.Second {
		t.Fatal("reset não voltou para 1 s")
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("esperei demais por %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAgentPollDeliversCommandsAndResults(t *testing.T) {
	isolate(t)
	key := pairMachine(t)
	var mu sync.Mutex
	results := map[string]map[string]any{}
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll": sequence(`{"commands":[
			{"id":"c_1","type":"host","args":{},"deadline":"2099-01-01T00:00:00Z"},
			{"id":"c_2","type":"apaga_tudo","args":{}},
			{"id":"c_3","type":"restart_project","args":{"project_id":"x","mode":"all"}},
			{"id":"c_4","type":"logs","args":{"tail":"muitas"}}]}`, `{"commands":[]}`),
		"/v1/result": func(w http.ResponseWriter, body map[string]any) {
			mu.Lock()
			results[body["command_id"].(string)] = body
			mu.Unlock()
			_, _ = io.WriteString(w, `{}`)
		},
	})
	store := core.NewStateStore(nil)
	store.SetHostMetrics(core.HostMetrics{CPUPercent: 12.4, Uptime: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Connect(ctx, key, store, testConfig(f.URL), "tui") }()

	waitFor(t, "4 resultados", func() bool { mu.Lock(); defer mu.Unlock(); return len(results) == 4 })
	waitFor(t, "o snapshot", func() bool { return f.count("/v1/snapshot") > 0 })
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Connect: %v", err)
	}

	host := results["c_1"]
	if host["ok"] != true || host["data"].(map[string]any)["name"] != "vps-digiliza" || host["data"].(map[string]any)["cpu"] != float64(12) {
		t.Fatalf("host: %v", host)
	}
	if r := results["c_2"]; r["ok"] != false || r["error"] != "unknown_command" {
		t.Fatalf("tipo desconhecido: %v", r)
	}
	if r := results["c_3"]; r["ok"] != false || r["error"] != "actions_disabled" {
		t.Fatalf("ação com as ações desligadas: %v", r)
	}
	if r := results["c_4"]; r["error"] != "bad_request" {
		t.Fatalf("argumento torto: %v", r)
	}
	poll := f.last("/v1/poll").body
	if poll["mode"] != "tui" || poll["actions_enabled"] != false {
		t.Fatalf("corpo do poll: %v", poll)
	}
	snap := f.last("/v1/snapshot").body
	if snap["mode"] != "tui" || snap["actions_enabled"] != false || snap["host"].(map[string]any)["name"] != "vps-digiliza" {
		t.Fatalf("snapshot: %v", snap)
	}
	if f.count("/v1/bye") != 1 {
		t.Fatalf("bye ao fechar: %d", f.count("/v1/bye"))
	}
	if LoadState().LastSeen.IsZero() {
		t.Fatal("o último contato não foi gravado")
	}
}

func TestAgentStopsOnUnauthorized(t *testing.T) {
	isolate(t)
	key := pairMachine(t)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll": reply(401, `{"error":"unauthorized","message":"vps-digiliza foi desconectada no Telegram"}`),
	})
	err := Connect(context.Background(), key, core.NewStateStore(nil), testConfig(f.URL), "service")
	if !IsCode(err, CodeUnauthorized) {
		t.Fatalf("Connect = %v", err)
	}
	if st := LoadState(); st.Revoked != "vps-digiliza foi desconectada no Telegram" || st.RevokedAt.IsZero() {
		t.Fatalf("estado não marcou a revogação: %+v", st)
	}
	if _, _, ok := Paired(); ok {
		t.Fatal("máquina revogada não pode continuar pareada")
	}
	if f.count("/v1/bye") != 0 {
		t.Fatal("máquina revogada não se despede")
	}
}

func TestAgentStopsOnUpgradeRequired(t *testing.T) {
	isolate(t)
	key := pairMachine(t)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll": reply(426, `{"error":"upgrade_required","message":"Este hub fala o protocolo 1. Atualize o DevScope."}`),
	})
	err := Connect(context.Background(), key, core.NewStateStore(nil), testConfig(f.URL), "service")
	if !IsCode(err, CodeUpgrade) {
		t.Fatalf("Connect = %v", err)
	}
	if LoadState().Revoked != "" {
		t.Fatal("protocolo velho não é revogação")
	}
}

func TestStartTUIWithoutPairingMakesNoRequests(t *testing.T) {
	isolate(t)
	f := newFakeHub(t, nil)
	cfg := testConfig(f.URL)

	stop := StartTUI(core.NewStateStore(nil), cfg) // sem key
	time.Sleep(50 * time.Millisecond)
	stop()

	key, _ := NewKey() // key sem dono: o pareamento não terminou
	_ = SaveKey(key)
	stop = StartTUI(core.NewStateStore(nil), cfg)
	time.Sleep(50 * time.Millisecond)
	stop()

	if n := f.count(""); n != 0 {
		t.Fatalf("o hub falso recebeu %d requisições", n)
	}
	if text, _ := Indicator(); text != "" {
		t.Fatalf("indicador sem pareamento: %q", text)
	}
	if _, err := os.Stat(LockPath()); err == nil {
		t.Fatal("sem pareamento não há lock")
	}
}

func TestStartTUIConnectsAndReleases(t *testing.T) {
	isolate(t)
	pairMachine(t)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll": func(w http.ResponseWriter, _ map[string]any) {
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, `{"commands":[]}`)
		},
	})
	stop := StartTUI(core.NewStateStore(nil), testConfig(f.URL))
	waitFor(t, "o indicador online", func() bool { _, online := Indicator(); return online })
	if LockHolder() != os.Getpid() {
		t.Fatal("a TUI conectada segura o lock")
	}
	// Outra TUI na mesma máquina não sobe um segundo agente.
	stop2 := StartTUI(core.NewStateStore(nil), testConfig(f.URL))
	stop2()
	stop()
	if f.count("/v1/bye") != 1 || LockHolder() != 0 {
		t.Fatalf("ao fechar: bye=%d lock=%d", f.count("/v1/bye"), LockHolder())
	}
	if text, _ := Indicator(); text != "" {
		t.Fatalf("indicador depois de fechar: %q", text)
	}
}

// A TUI aberta antes do pareamento (ou atrás de outra TUI) conecta sozinha
// quando o pareamento termina ou o lock fica livre.
func TestStartTUIConnectsLater(t *testing.T) {
	isolate(t)
	stub(t, &tuiRecheck, 10*time.Millisecond)
	f := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/poll": func(w http.ResponseWriter, _ map[string]any) {
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, `{"commands":[]}`)
		},
	})
	stop := StartTUI(core.NewStateStore(nil), testConfig(f.URL))
	defer stop()
	time.Sleep(50 * time.Millisecond)
	if n := f.count(""); n != 0 {
		t.Fatalf("sem pareamento o hub recebeu %d requisições", n)
	}

	release, err := AcquireLock() // outro devscope segura o lock
	if err != nil {
		t.Fatal(err)
	}
	pairMachine(t)
	time.Sleep(50 * time.Millisecond)
	if n := f.count(""); n != 0 {
		t.Fatalf("com o lock ocupado o hub recebeu %d requisições", n)
	}
	release()
	waitFor(t, "a TUI conectar depois do pareamento", func() bool { _, online := Indicator(); return online })
}

func TestLock(t *testing.T) {
	isolate(t)
	release, err := AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	var le *LockedError
	if _, err := AcquireLock(); !errors.As(err, &le) || le.PID != os.Getpid() {
		t.Fatalf("segundo agente subiu: %v", err)
	}
	release()
	if LockHolder() != 0 {
		t.Fatal("release não soltou o lock")
	}

	// Lock de um processo que já morreu é tomado.
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skip("sem o comando true")
	}
	if err := os.WriteFile(LockPath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err = AcquireLock()
	if err != nil {
		t.Fatalf("lock de PID morto não foi tomado: %v", err)
	}
	if LockHolder() != os.Getpid() {
		t.Fatal("o lock tomado deveria ter o PID atual")
	}
	release()
}

// Sem bot.hub_url o cliente acha o hub desta máquina e ignora outro serviço
// na porta; sem hub nenhum, o erro diz o que configurar.
func TestFindLocalHub(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "<html>404</html>", http.StatusNotFound)
	}))
	defer other.Close()
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUpgradeRequired)
		_, _ = w.Write([]byte(`{"error":"upgrade_required","message":"Atualize o DevScope."}`))
	}))
	defer hubSrv.Close()
	port := func(u string) string { return u[strings.LastIndex(u, ":")+1:] }

	if got := findHub(port(other.URL), port(hubSrv.URL)); got != hubSrv.URL {
		t.Fatalf("achou %q, queria %q", got, hubSrv.URL)
	}
	if got := findHub(port(other.URL), "1"); got != "" {
		t.Fatalf("nenhum hub, achou %q", got)
	}
	c := &Client{HTTP: http.DefaultClient}
	if _, err := c.Register(context.Background(), "m", "linux"); err == nil || !strings.Contains(err.Error(), "bot.hub_url") {
		t.Fatalf("erro sem hub: %v", err)
	}
}
