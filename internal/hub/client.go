// Package hub é o lado da máquina na conversa com o hub do bot do Telegram
// (repositório devscope-bot). A máquina sempre inicia a conexão e nunca abre
// porta: tudo é POST em /v1/, como manda o Contrato (protocolo 1).
package hub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/pkg/version"
)

// Protocol é a versão do Contrato que esta máquina fala.
const Protocol = "1"

// Códigos de erro do Contrato. CodeOffline (sem rede ou hub fora) e
// CodeNotHub (o endereço não é um hub) são locais.
const (
	CodeUnauthorized       = "unauthorized"
	CodeUpgrade            = "upgrade_required"
	CodeRateLimited        = "rate_limited"
	CodeRegistrationClosed = "registration_closed"
	CodeNotPaired          = "not_paired"
	CodeOffline            = "offline"
	CodeNotHub             = "not_hub"
)

// Error é a resposta de erro do hub. Message já vem em português, pronta para
// mostrar à pessoa.
type Error struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *Error) Error() string { return e.Message }

// IsCode diz se err é um erro do hub com aquele código.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

type Client struct {
	URL  string
	Key  string
	HTTP *http.Client
}

func NewClient(hubURL, key string) *Client {
	if hubURL == "" {
		hubURL = localHub()
	}
	return &Client{URL: strings.TrimRight(hubURL, "/"), Key: key, HTTP: &http.Client{}}
}

// localHub acha um hub rodando nesta máquina, nas portas do compose do
// devscope-bot: 8080 no README, 8090 quando a 8080 já está ocupada.
var localHub = sync.OnceValue(func() string { return findHub("8080", "8090") })

// findHub devolve o primeiro 127.0.0.1:<porta> que é um hub. Sem o cabeçalho
// do protocolo o hub responde com o JSON de erro do Contrato; outro serviço na
// porta não. Vazio = nenhum hub.
func findHub(ports ...string) string {
	c := &http.Client{Timeout: 500 * time.Millisecond}
	for _, port := range ports {
		u := "http://127.0.0.1:" + port
		resp, err := c.Post(u+"/v1/register", "application/json", nil)
		if err != nil {
			continue
		}
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
		resp.Body.Close()
		if body.Error != "" {
			return u
		}
	}
	return ""
}

// Host é o que a pessoa reconhece do hub: "bot.exemplo.com".
func (c *Client) Host() string {
	if u, err := url.Parse(c.URL); err == nil && u.Host != "" {
		return u.Host
	}
	return c.URL
}

// call faz um POST do Contrato. Long poll ganha o tempo de espera mais folga;
// o resto tem 20 s.
func (c *Client) call(parent context.Context, path string, wait int, in, out any) error {
	if c.URL == "" {
		return &Error{Code: CodeOffline, Message: "nenhum hub do bot configurado: defina bot.hub_url em " +
			config.ConfigPath() + " (ou DEVSCOPE_BOT_HUB_URL) com o endereço do devscope-bot"}
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(wait+20)*time.Second)
	defer cancel()
	if in == nil {
		in = struct{}{}
	}
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return &Error{Code: CodeOffline, Message: "endereço do hub inválido: " + c.URL}
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("X-DevScope-Protocol", Protocol)
	req.Header.Set("User-Agent", "devscope/"+version.Version)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if parent.Err() != nil {
			return parent.Err()
		}
		return &Error{Code: CodeOffline, Message: "não consegui falar com " + c.Host()}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return c.apiError(resp, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &Error{Status: resp.StatusCode, Code: "bad_response", Message: "resposta inválida de " + c.Host()}
	}
	return nil
}

func (c *Client) apiError(resp *http.Response, data []byte) error {
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &body)
	e := &Error{Status: resp.StatusCode, Code: body.Error, Message: body.Message}
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		e.RetryAfter = time.Duration(s) * time.Second
	}
	switch {
	case e.Code != "" && e.Message == "":
		e.Message = fmt.Sprintf("o hub %s recusou: %s", c.Host(), e.Code)
	case e.Code != "":
	case resp.StatusCode >= 500:
		// Proxy na frente do hub respondendo por ele: o hub está fora.
		e.Code = CodeOffline
		e.Message = fmt.Sprintf("o hub em %s não está respondendo (HTTP %d)", c.Host(), resp.StatusCode)
	default:
		// Sem o corpo do Contrato quem respondeu não é o hub (outro serviço na
		// porta). Nunca vira unauthorized: isso apagaria o pareamento à toa.
		e.Code = CodeNotHub
		e.Message = fmt.Sprintf("%s respondeu HTTP %d, mas não é o hub do devscope: confira bot.hub_url em %s",
			c.Host(), resp.StatusCode, config.ConfigPath())
	}
	return e
}

// backoff dobra a espera a cada falha seguida: 1 s, 2 s, 4 s… até 60 s. Um 429
// respeita o Retry-After do hub.
type backoff struct{ n int }

func (b *backoff) next(err error) time.Duration {
	var e *Error
	if errors.As(err, &e) && e.RetryAfter > 0 {
		return e.RetryAfter
	}
	d := time.Second << min(b.n, 6)
	b.n++
	return min(d, 60*time.Second)
}

func (b *backoff) reset() { b.n = 0 }

// ─── rotas ──────────────────────────────────────────────────────────────────

type Owner struct {
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
	IDMasked string `json:"id_masked"`
}

// Who é como o dono aparece no terminal: nem toda conta do Telegram tem @username.
func (o Owner) Who() string {
	switch {
	case o.Username != "":
		return "@" + o.Username
	case o.Name != "":
		return o.Name
	}
	return o.IDMasked
}

type PairCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Bot       string    `json:"bot"`
	Link      string    `json:"link"`
}

type PairRequest struct {
	RequestID string `json:"request_id"`
	Chat      Owner  `json:"chat"`
}

type PairWait struct {
	Request *PairRequest `json:"request"`
	Expired bool         `json:"expired"`
}

type Command struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Args     json.RawMessage `json:"args"` // decodificado por comando: um argumento torto não derruba o poll inteiro
	Deadline time.Time       `json:"deadline"`
}

type Result struct {
	CommandID string `json:"command_id"`
	OK        bool   `json:"ok"`
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Snapshot struct {
	Host           Host             `json:"host"`
	Projects       []ProjectSummary `json:"projects"`
	ActionsEnabled bool             `json:"actions_enabled"`
	GH             bool             `json:"gh"`
	Mode           string           `json:"mode"`
	At             time.Time        `json:"at"`
}

type Event struct {
	Key       string    `json:"key"`
	Kind      string    `json:"kind"`
	ProjectID string    `json:"project_id,omitempty"`
	Project   string    `json:"project,omitempty"`
	Container string    `json:"container,omitempty"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Count     int       `json:"count,omitempty"`
	At        time.Time `json:"at"`
	Delayed   bool      `json:"delayed"`
}

func (c *Client) Register(ctx context.Context, machineName, goos string) (string, error) {
	var out struct {
		MachineID string `json:"machine_id"`
	}
	err := c.call(ctx, "/v1/register", 0, map[string]string{
		"machine_name": machineName, "os": goos, "devscope_version": version.Version,
	}, &out)
	return out.MachineID, err
}

func (c *Client) Pair(ctx context.Context) (PairCode, error) {
	var out PairCode
	return out, c.call(ctx, "/v1/pair", 0, nil, &out)
}

func (c *Client) PairWait(ctx context.Context, code string, wait int) (PairWait, error) {
	var out PairWait
	return out, c.call(ctx, "/v1/pair/wait", wait, map[string]any{"code": code, "wait": wait}, &out)
}

func (c *Client) PairConfirm(ctx context.Context, requestID string, accept bool) (Owner, error) {
	var out struct {
		Owner Owner `json:"owner"`
	}
	err := c.call(ctx, "/v1/pair/confirm", 0, map[string]any{"request_id": requestID, "accept": accept}, &out)
	return out.Owner, err
}

func (c *Client) Poll(ctx context.Context, mode string, actions bool, wait int) ([]Command, error) {
	var out struct {
		Commands []Command `json:"commands"`
	}
	err := c.call(ctx, "/v1/poll", wait, map[string]any{"mode": mode, "actions_enabled": actions, "wait": wait}, &out)
	return out.Commands, err
}

func (c *Client) Result(ctx context.Context, r Result) error {
	return c.call(ctx, "/v1/result", 0, r, nil)
}

func (c *Client) Snapshot(ctx context.Context, s Snapshot) error {
	return c.call(ctx, "/v1/snapshot", 0, s, nil)
}

func (c *Client) Events(ctx context.Context, evs []Event) error {
	if evs == nil {
		evs = []Event{}
	}
	return c.call(ctx, "/v1/events", 0, map[string]any{"events": evs}, nil)
}

func (c *Client) Bye(ctx context.Context) error {
	return c.call(ctx, "/v1/bye", 0, nil, nil)
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.call(ctx, "/v1/disconnect", 0, nil, nil)
}

// ─── arquivos locais ────────────────────────────────────────────────────────

// Dir é ~/.config/devscope, onde ficam a key, o estado e o lock.
func Dir() string { return filepath.Dir(config.ConfigPath()) }

func KeyPath() string   { return filepath.Join(Dir(), "hub.key") }
func StatePath() string { return filepath.Join(Dir(), "bot.json") }
func LockPath() string  { return filepath.Join(Dir(), "bot.lock") }

// NewKey gera a identidade da máquina: dsk_ e 32 bytes aleatórios em base32
// minúsculo sem padding (52 caracteres). O hub só guarda o sha256 dela.
func NewKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "dsk_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)), nil
}

// LoadKey devolve "" quando a máquina nunca rodou `devscope bot`.
func LoadKey() (string, error) {
	b, err := os.ReadFile(KeyPath())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(b))
	if !strings.HasPrefix(key, "dsk_") {
		return "", fmt.Errorf("%s não tem uma key válida", KeyPath())
	}
	return key, nil
}

func SaveKey(key string) error {
	if err := ensureDir(); err != nil {
		return err
	}
	return writeFile(KeyPath(), []byte(key+"\n"))
}

// ensureDir deixa o diretório em 0700: ele guarda a key.
func ensureDir() error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	return os.Chmod(Dir(), 0o700)
}

// writeFile grava por rename: quem lê no meio nunca vê arquivo pela metade.
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// State é o bot.json: quem é o dono, quando falou com o hub pela última vez e
// a fila de avisos que não conseguiram sair.
type State struct {
	MachineID string    `json:"machine_id,omitempty"`
	Bot       string    `json:"bot,omitempty"`
	Owner     *Owner    `json:"owner,omitempty"`
	PairedAt  time.Time `json:"paired_at"`
	LastSeen  time.Time `json:"last_seen"`
	Revoked   string    `json:"revoked,omitempty"` // mensagem do hub quando recusou a key
	RevokedAt time.Time `json:"revoked_at"`
	Queue     []Event   `json:"queue,omitempty"`
}

var stateMu sync.Mutex

func LoadState() State {
	var st State
	if b, err := os.ReadFile(StatePath()); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

// UpdateState relê o arquivo antes de mudar: o agente e o `devscope bot`
// podem mexer nele ao mesmo tempo, cada um no seu campo.
func UpdateState(fn func(*State)) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	st := LoadState()
	fn(&st)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := ensureDir(); err != nil {
		return err
	}
	return writeFile(StatePath(), b)
}

// ClearLocal apaga a key e o estado: a máquina volta a ser nova.
func ClearLocal() error {
	for _, p := range []string{KeyPath(), StatePath()} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Paired diz se esta máquina tem key e dono, e se o hub não a recusou.
func Paired() (key string, st State, ok bool) {
	key, _ = LoadKey()
	st = LoadState()
	return key, st, key != "" && st.Owner != nil && st.Revoked == ""
}
