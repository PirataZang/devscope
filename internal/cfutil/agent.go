package cfutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tunnel is a merged view of a running process + project config.
type Tunnel struct {
	Name      string
	Project   string
	Port      int
	Hostname  string
	PublicURL string
	LocalURL  string
	Status    string // online | offline | starting
	Mode      string // quick | named
	TunnelID  string
	Uptime    string
	PID       int
}

// AccountTunnel is a named tunnel registered in the Cloudflare account.
type AccountTunnel struct {
	ID          string
	Name        string
	CreatedAt   time.Time
	Connections int
	DeletedAt   *time.Time
}

type AuthInfo struct {
	LoggedIn bool
	CertPath string
	Version  string
	CLI      bool
}

type runState struct {
	cmd       *exec.Cmd
	name      string
	url       string
	mode      string
	hostname  string
	publicURL string
	started   time.Time
	logLines  []string
}

var (
	runMu    sync.Mutex
	running  = map[string]*runState{}
	lastExit = map[string]string{} // últimas linhas de quem morreu, para o erro
)

func Available() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}

func Version() string {
	out, err := exec.Command("cloudflared", "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	// cloudflared version 2026.7.3 (built …)
	s := strings.TrimSpace(string(out))
	s = strings.TrimPrefix(s, "cloudflared version ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s
	}
	return fields[0]
}

func CertPath() string {
	if v := os.Getenv("TUNNEL_ORIGIN_CERT"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".cloudflared", "cert.pem"),
		filepath.Join(home, ".cloudflare-warp", "cert.pem"),
		"/etc/cloudflared/cert.pem",
		"/usr/local/etc/cloudflared/cert.pem",
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(home, ".cloudflared", "cert.pem")
}

func LoggedIn() bool {
	p := CertPath()
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func HasToken() bool {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		"/etc/cloudflared/token",
		filepath.Join(home, ".cloudflared", "token"),
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func Auth() AuthInfo {
	return AuthInfo{
		LoggedIn: LoggedIn() || HasToken(),
		CertPath: CertPath(),
		Version:  Version(),
		CLI:      Available(),
	}
}

// Login opens the Cloudflare browser auth flow (blocks until done or error).
func Login() error {
	if !Available() {
		return fmt.Errorf("cloudflared não encontrado — use Install ou instale o CLI")
	}
	cmd := exec.Command("cloudflared", "tunnel", "login")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("login: %s", msg)
	}
	if !LoggedIn() {
		return fmt.Errorf("login concluído mas cert.pem não encontrado em %s", CertPath())
	}
	return nil
}

// Install downloads cloudflared into ~/.local/bin when missing from PATH.
func Install() (string, error) {
	if Available() {
		return "já instalado: " + Version(), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(binDir, "cloudflared")
	url, err := releaseURL()
	if err != nil {
		return "", err
	}
	tmp := dest + ".tmp"
	if err := downloadFile(url, tmp); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if !Available() {
		return "instalado em " + dest + " — adicione ~/.local/bin ao PATH", nil
	}
	return "instalado: " + Version(), nil
}

func releaseURL() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("arch não suportada: %s", arch)
	}
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-%s", arch), nil
	case "darwin":
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-%s", arch), nil
	default:
		return "", fmt.Errorf("SO não suportado: %s", runtime.GOOS)
	}
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func ListAccountTunnels() ([]AccountTunnel, error) {
	if !Available() {
		return nil, fmt.Errorf("cloudflared não encontrado")
	}
	if !LoggedIn() {
		return nil, fmt.Errorf("não autenticado — rode Login (L)")
	}
	out, err := exec.Command("cloudflared", "tunnel", "list", "--output", "json").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	var raw []struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		CreatedAt   time.Time  `json:"created_at"`
		DeletedAt   *time.Time `json:"deleted_at"`
		Connections []any      `json:"connections"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse list: %w", err)
	}
	tunnels := make([]AccountTunnel, 0, len(raw))
	for _, t := range raw {
		if t.DeletedAt != nil && !t.DeletedAt.IsZero() {
			continue
		}
		tunnels = append(tunnels, AccountTunnel{
			ID:          t.ID,
			Name:        t.Name,
			CreatedAt:   t.CreatedAt,
			Connections: len(t.Connections),
			DeletedAt:   t.DeletedAt,
		})
	}
	return tunnels, nil
}

// CreateTunnel registers a named tunnel on the Cloudflare account.
func CreateTunnel(name string) (AccountTunnel, error) {
	name = sanitizeName(name)
	if name == "" {
		return AccountTunnel{}, fmt.Errorf("nome vazio")
	}
	if !LoggedIn() {
		return AccountTunnel{}, fmt.Errorf("não autenticado — rode Login (L)")
	}
	out, err := exec.Command("cloudflared", "tunnel", "create", "--output", "json", name).CombinedOutput()
	if err != nil {
		// Fallback: create without json (older CLIs), then list.
		out2, err2 := exec.Command("cloudflared", "tunnel", "create", name).CombinedOutput()
		if err2 != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = strings.TrimSpace(string(out2))
			}
			return AccountTunnel{}, fmt.Errorf("%s", msg)
		}
		list, listErr := ListAccountTunnels()
		if listErr != nil {
			return AccountTunnel{Name: name}, nil
		}
		for _, t := range list {
			if t.Name == name {
				return t, nil
			}
		}
		return AccountTunnel{Name: name}, nil
	}
	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &created); err == nil && created.ID != "" {
		return AccountTunnel{ID: created.ID, Name: created.Name}, nil
	}
	return AccountTunnel{Name: name}, nil
}

func DeleteAccountTunnel(nameOrID string) error {
	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return fmt.Errorf("nome/id vazio")
	}
	out, err := exec.Command("cloudflared", "tunnel", "delete", "-f", nameOrID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func RouteDNS(tunnel, hostname string) error {
	tunnel = sanitizeName(tunnel)
	hostname = strings.TrimSpace(strings.ToLower(hostname))
	if tunnel == "" || hostname == "" {
		return fmt.Errorf("tunnel e hostname obrigatórios")
	}
	out, err := exec.Command("cloudflared", "tunnel", "route", "dns", tunnel, hostname).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Modes são os modos de túnel, na ordem em que o wizard cicla. Uma lista só
// porque wizard e agente têm de concordar: com a string solta em cada if, um
// modo novo aparece no menu e o agente cai no default sem ninguém notar.
var Modes = []string{"quick", "named", "http2"}

// NormalizeMode devolve um modo válido. Vazio se resolve pelo hostname — quem
// informou hostname quer named —, e o que não está na lista vira quick.
func NormalizeMode(mode, hostname string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		if strings.TrimSpace(hostname) != "" {
			return "named"
		}
		return "quick"
	}
	for _, m := range Modes {
		if m == mode {
			return mode
		}
	}
	return "quick"
}

// StartTunnel expõe um destino local (porta, host:porta ou URL completa) e
// devolve a URL pública assim que o cloudflared anuncia.
func StartTunnel(name, target, mode, hostname string) (string, error) {
	local := NormalizeURL(target)
	if local == "" {
		return "", fmt.Errorf("URL local inválida: %q", target)
	}
	name = sanitizeName(name)
	if name == "" {
		name = "tunnel"
	}
	mode = NormalizeMode(mode, hostname)
	runMu.Lock()
	if st, ok := running[name]; ok && st.cmd != nil && st.cmd.Process != nil {
		runMu.Unlock()
		return "", fmt.Errorf("%s já está rodando (pid %d)", name, st.cmd.Process.Pid)
	}
	runMu.Unlock()

	if mode == "named" && !LoggedIn() {
		return "", fmt.Errorf("túnel named exige login (L)")
	}
	cmd := exec.Command("cloudflared", tunnelArgs(name, local, mode)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	st := &runState{
		cmd:      cmd,
		name:     name,
		url:      local,
		mode:     mode,
		hostname: hostname,
		started:  time.Now(),
	}
	if hostname != "" {
		st.publicURL = "https://" + hostname
	}
	runMu.Lock()
	running[name] = st
	delete(lastExit, name)
	runMu.Unlock()

	// cloudflared loga na stderr; ler os dois pipes em paralelo (um MultiReader
	// só chegaria na stderr depois da stdout fechar, ou seja, ao morrer).
	go consumeOutput(name, stdout)
	go consumeOutput(name, stderr)
	go func() {
		_ = cmd.Wait()
		runMu.Lock()
		if cur, ok := running[name]; ok && cur.cmd == cmd {
			lastExit[name] = tail(cur.logLines, 3)
			delete(running, name)
		}
		runMu.Unlock()
	}()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		runMu.Lock()
		st, alive := running[name]
		pub, exit := "", lastExit[name]
		if alive {
			pub = st.publicURL
		}
		runMu.Unlock()
		if pub != "" {
			return pub, nil
		}
		if !alive {
			if exit == "" {
				exit = "sem saída"
			}
			return "", fmt.Errorf("cloudflared saiu: %s", exit)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", nil
}

// PublicURL devolve a URL anunciada por um túnel gerenciado.
func PublicURL(name string) string {
	runMu.Lock()
	defer runMu.Unlock()
	if st, ok := running[sanitizeName(name)]; ok {
		return st.publicURL
	}
	return ""
}

func StopTunnel(name string) error {
	name = sanitizeName(name)
	runMu.Lock()
	st, ok := running[name]
	runMu.Unlock()
	if ok && st.cmd != nil && st.cmd.Process != nil {
		if err := st.cmd.Process.Signal(os.Interrupt); err != nil {
			_ = st.cmd.Process.Kill()
		}
		done := make(chan struct{})
		go func() {
			_ = st.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = st.cmd.Process.Kill()
		}
		runMu.Lock()
		delete(running, name)
		runMu.Unlock()
		return nil
	}
	// External process started outside this session.
	for _, t := range ListLiveTunnels() {
		if sanitizeName(t.Name) == name && t.PID > 0 {
			return StopPID(t.PID)
		}
	}
	return fmt.Errorf("%s não está rodando", name)
}

func RecentLogs(name string, limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	runMu.Lock()
	defer runMu.Unlock()
	st, ok := running[sanitizeName(name)]
	if !ok {
		return nil
	}
	n := len(st.logLines)
	if n == 0 {
		return nil
	}
	start := n - limit
	if start < 0 {
		start = 0
	}
	return append([]string(nil), st.logLines[start:]...)
}

func tail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func consumeOutput(name string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		runMu.Lock()
		st, ok := running[name]
		if !ok {
			runMu.Unlock()
			return
		}
		st.logLines = append(st.logLines, line)
		if len(st.logLines) > 200 {
			st.logLines = st.logLines[len(st.logLines)-200:]
		}
		if st.publicURL == "" {
			if u := extractPublicURL(line); u != "" {
				st.publicURL = u
			}
		}
		runMu.Unlock()
	}
}

// extractPublicURL pega o endereço público anunciado no log do quick tunnel.
func extractPublicURL(line string) string {
	for _, field := range strings.Fields(line) {
		field = strings.Trim(field, "<>\"',|")
		if !strings.HasPrefix(field, "https://") {
			continue
		}
		if strings.Contains(field, ".trycloudflare.com") || strings.Contains(field, ".cfargotunnel.com") {
			return field
		}
	}
	return ""
}

// tunnelArgs monta a chamada do cloudflared: quick usa `tunnel --url <url>`,
// named usa `tunnel run --url <url> <name>` e http2 é o quick forçando o
// transporte — serve pra rede que bloqueia QUIC na UDP 7844, onde o quick fica
// tentando reconectar sem nunca subir.
func tunnelArgs(name, url, mode string) []string {
	switch mode {
	case "named":
		return []string{"tunnel", "run", "--url", url, name}
	case "http2":
		return []string{"tunnel", "--protocol", "http2", "--url", url}
	}
	return []string{"tunnel", "--url", url}
}

// NormalizeURL aceita "4321", "localhost:4321" ou "http://localhost:4321" e
// devolve o valor pronto para o --url do cloudflared. localhost é trocado por
// 127.0.0.1: se ele resolver para ::1 e o serviço não atender em IPv6, o
// cloudflared não alcança a origem e responde 502.
func NormalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if p, err := strconv.Atoi(s); err == nil {
		if p < 1 || p > 65535 {
			return ""
		}
		return localURL(p)
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(strings.TrimSuffix(s, "/"))
	if err != nil {
		return ""
	}
	if u.Hostname() == "localhost" {
		host := "127.0.0.1"
		if p := u.Port(); p != "" {
			host += ":" + p
		}
		u.Host = host
	}
	return u.String()
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "tunnel"
	}
	return out
}

func localURL(port int) string {
	if port <= 0 {
		return ""
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}

func publicHost(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func parseAddrPort(addr string) int {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	if i := strings.Index(addr, "/"); i >= 0 {
		addr = addr[:i]
	}
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		p, _ := strconv.Atoi(addr[i+1:])
		return p
	}
	p, _ := strconv.Atoi(addr)
	return p
}

func normalizeLocal(addr string) string {
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "://") {
		return addr
	}
	return "http://" + addr
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}
