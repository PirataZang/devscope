package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/hub"
)

var update = flag.Bool("update", false, "regrava os arquivos golden")

var botNow = time.Date(2026, 10, 21, 21, 14, 3, 0, time.UTC)

// botHub é o hub falso do pareamento: os pair/wait saem na ordem dada, e o
// pair/confirm guarda o que o terminal respondeu.
type botHub struct {
	*httptest.Server
	mu      sync.Mutex
	waits   []string
	codes   []string
	accepts []bool
}

func newBotHub(t *testing.T, waits ...string) *botHub {
	h := &botHub{waits: waits, codes: []string{"K7F-29Q", "M4P-8XZ"}}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		h.mu.Lock()
		defer h.mu.Unlock()
		switch r.URL.Path {
		case "/v1/register":
			io.WriteString(w, `{"machine_id":"m_7f3a9c21"}`)
		case "/v1/pair":
			code := h.codes[0]
			h.codes = h.codes[1:]
			io.WriteString(w, `{"code":"`+code+`","expires_at":"2026-10-21T21:24:00Z","bot":"devscope_vps_bot",
				"link":"https://t.me/devscope_vps_bot?start=`+code+`"}`)
		case "/v1/pair/wait":
			io.WriteString(w, h.waits[0])
			h.waits = h.waits[1:]
		case "/v1/pair/confirm":
			h.accepts = append(h.accepts, body["accept"] == true)
			io.WriteString(w, `{"owner":{"username":"fulano","id_masked":"5512••••"}}`)
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(h.Close)
	return h
}

const pairRequest = `{"request":{"request_id":"pr_1","chat":{"username":"fulano","name":"Fulano Silva","id_masked":"5512••••"}}}`

func botSetup(t *testing.T, in string, h *botHub) (*botUI, *bytes.Buffer, *config.Config) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NO_COLOR", "1")
	old := unitPath
	unitPath = filepath.Join(home, "devscope-bot.service")
	t.Cleanup(func() { unitPath = old })
	var buf bytes.Buffer
	u := &botUI{in: strings.NewReader(in), out: &buf, errOut: &buf, width: 80, now: func() time.Time { return botNow }}
	cfg := config.Default()
	cfg.Bot.MachineName = "vps-digiliza"
	if h != nil {
		cfg.Bot.HubURL = h.URL
	}
	return u, &buf, cfg
}

func golden(t *testing.T, name string, h *botHub, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, strings.TrimPrefix(h.URL, "http://"), "bot.SEU-DOMINIO")
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (rode go test -run %s -update)", err, t.Name())
	}
	if got != string(want) {
		t.Fatalf("saída diferente de %s:\n%s", path, got)
	}
}

func TestBotPairFirstTime(t *testing.T) {
	h := newBotHub(t, `{}`, pairRequest)
	u, buf, cfg := botSetup(t, "s\n", h)
	if err := u.finish(context.Background(), u.pair(context.Background(), cfg)); err != nil {
		t.Fatal(err)
	}
	golden(t, "bot_first", h, buf.String())
	if len(h.accepts) != 1 || !h.accepts[0] {
		t.Fatalf("confirm: %v", h.accepts)
	}
	key, st, ok := hub.Paired()
	if !ok || st.MachineID != "m_7f3a9c21" || st.Owner.Username != "fulano" || st.Owner.Name != "Fulano Silva" || st.Bot != "devscope_vps_bot" {
		t.Fatalf("estado depois do pareamento: %+v", st)
	}
	if strings.Contains(buf.String(), key) {
		t.Fatal("a key nunca aparece no terminal")
	}
}

func TestBotPairExpiredCode(t *testing.T) {
	h := newBotHub(t, `{"expired":true}`, pairRequest)
	u, buf, cfg := botSetup(t, "\ns\n", h) // Enter vazio no [S/n] gera outro código
	if err := u.pair(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	golden(t, "bot_expired", h, buf.String())
}

func TestBotPairRefused(t *testing.T) {
	h := newBotHub(t, pairRequest)
	u, buf, cfg := botSetup(t, "\n", h) // Enter vazio recusa
	if err := u.pair(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	golden(t, "bot_refused", h, buf.String())
	if len(h.accepts) != 1 || h.accepts[0] {
		t.Fatalf("confirm deveria levar accept=false: %v", h.accepts)
	}
	if _, _, ok := hub.Paired(); ok {
		t.Fatal("recusado não pareia")
	}
}

func TestBotAlreadyPaired(t *testing.T) {
	h := newBotHub(t)
	u, buf, cfg := botSetup(t, "n\n", h)
	key, _ := hub.NewKey()
	_ = hub.SaveKey(key)
	_ = hub.UpdateState(func(s *hub.State) {
		s.Owner = &hub.Owner{Username: "fulano", IDMasked: "5512••••"}
		s.LastSeen = botNow.Add(-3 * time.Minute)
	})
	if err := u.pair(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	golden(t, "bot_already_paired", h, buf.String())
	if len(h.accepts) != 0 {
		t.Fatal("respondeu não e mesmo assim pareou")
	}
}

// Conta do Telegram sem @username aparece pelo nome, não como "@".
func TestBotOwnerWithoutUsername(t *testing.T) {
	h := newBotHub(t, `{"request":{"request_id":"pr_1","chat":{"username":"","name":"Fulano Silva","id_masked":"5512••••"}}}`)
	u, buf, cfg := botSetup(t, "s\nn\n", h)
	_ = u.pair(context.Background(), cfg)
	_ = hub.UpdateState(func(s *hub.State) { s.Owner.Username = "" }) // o hub falso devolve owner com username
	_ = u.pair(context.Background(), cfg)
	out := buf.String()
	for _, want := range []string{"\nFulano Silva quer conectar a vps-digiliza.", "dono:     Fulano Silva · ", "Isso desconecta Fulano Silva."} {
		if !strings.Contains(out, want) {
			t.Fatalf("sem %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "@ ") || strings.Contains(out, "@.") {
		t.Fatalf("sobrou @ sem username:\n%s", out)
	}
}

func TestBotNarrowTerminal(t *testing.T) {
	h := newBotHub(t, pairRequest)
	u, buf, cfg := botSetup(t, "s\n", h)
	u.width = 60
	_ = u.pair(context.Background(), cfg)
	out := buf.String()
	if !strings.Contains(out, "▀\nQR do mesmo link\ncódigo vale até 21:24\nuso único\n") {
		t.Fatalf("com menos de 80 colunas a legenda vai embaixo:\n%s", out)
	}

	h = newBotHub(t, pairRequest)
	u, buf, cfg = botSetup(t, "s\n", h)
	u.width = 40
	_ = u.pair(context.Background(), cfg)
	if strings.ContainsAny(buf.String(), "▀▄") || !strings.Contains(buf.String(), "código vale até 21:24 · uso único") {
		t.Fatalf("com menos de 45 colunas, sem QR:\n%s", buf.String())
	}
}

func TestBotErrors(t *testing.T) {
	u, buf, cfg := botSetup(t, "", nil)
	cfg.Bot.HubURL = "http://127.0.0.1:1" // ninguém escutando
	err := u.finish(context.Background(), u.pair(context.Background(), cfg))
	if ExitCode(err) != 1 || !strings.Contains(buf.String(), "✗ não consegui falar com 127.0.0.1:1\n") {
		t.Fatalf("sem rede: %v\n%s", err, buf.String())
	}

	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(429)
		io.WriteString(w, `{"error":"rate_limited","message":"muitos pedidos desta máquina"}`)
	}))
	defer limited.Close()
	u, buf, cfg = botSetup(t, "", nil)
	cfg.Bot.HubURL = limited.URL
	err = u.finish(context.Background(), u.pair(context.Background(), cfg))
	if ExitCode(err) != 1 || !strings.Contains(buf.String(), "✗ muitos pedidos desta máquina — tente de novo em 1m30s\n") {
		t.Fatalf("rate_limited: %v\n%s", err, buf.String())
	}

	// Ctrl+C esperando o Telegram: sai com 130.
	h := newBotHub(t)
	h.waits = nil
	blocking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/pair/wait" {
			_, _ = io.Copy(io.Discard, r.Body) // sem ler o corpo o servidor não percebe o cliente indo embora
			<-r.Context().Done()
			return
		}
		h.Config.Handler.ServeHTTP(w, r)
	}))
	defer blocking.Close()
	u, _, cfg = botSetup(t, "", nil)
	cfg.Bot.HubURL = blocking.URL
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if code := ExitCode(u.finish(ctx, u.pair(ctx, cfg))); code != 130 {
		t.Fatalf("Ctrl+C saiu com %d", code)
	}
}

func TestRenderUnit(t *testing.T) {
	unit := renderUnit("igor", "/home/igor/Projeto Pessoal/devscope", "")
	for _, want := range []string{"User=igor\n", `ExecStart="/home/igor/Projeto Pessoal/devscope" bot run` + "\n",
		"Restart=always\n", "RestartSec=5\n", "RestartPreventExitStatus=3\n", "WantedBy=multi-user.target\n"} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit sem %q:\n%s", want, unit)
		}
	}
}
