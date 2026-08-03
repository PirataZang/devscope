package sshutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tunnel is a merged view of a live ssh process + project config.
type Tunnel struct {
	Name       string
	Project    string
	Mode       string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Target     string
	Identity   string
	Status     string // online | offline | starting
	Uptime     string
	PID        int
	Forward    string // e.g. localhost:5433 → 127.0.0.1:5432
}

type runState struct {
	cmd        *exec.Cmd
	name       string
	mode       string
	localPort  int
	remoteHost string
	remotePort int
	target     string
	identity   string
	started    time.Time
	logLines   []string
}

var (
	runMu    sync.Mutex
	running  = map[string]*runState{}
	lastExit = map[string]string{}
)

func Available() bool {
	_, err := exec.LookPath("ssh")
	return err == nil
}

func Version() string {
	out, err := exec.Command("ssh", "-V").CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	// OpenSSH_9.6p1 … often on stderr; CombinedOutput covers both.
	s := strings.TrimSpace(string(out))
	if i := strings.IndexAny(s, " \n"); i > 0 {
		return s[:i]
	}
	return s
}

func SuggestPort(ports []int, framework string) int {
	if len(ports) > 0 {
		return ports[0]
	}
	switch strings.ToLower(framework) {
	case "laravel":
		return 8000
	case "vite", "vue", "nuxt.js", "nuxt":
		return 5173
	case "angular":
		return 4200
	case "go", "spring", "gin", "echo":
		return 8080
	case "asp.net":
		return 5000
	default:
		return 3000
	}
}

func FormatForward(mode string, localPort int, remoteHost string, remotePort int) string {
	mode = NormalizeMode(mode)
	switch mode {
	case ModeDynamic:
		return fmt.Sprintf("socks5://127.0.0.1:%d", localPort)
	case ModeRemote:
		// porta no servidor → serviço no PC
		return fmt.Sprintf("R :%d → PC %s:%d", localPort, firstNonEmpty(remoteHost, "127.0.0.1"), remotePort)
	default:
		return fmt.Sprintf("L :%d → %s:%d", localPort, firstNonEmpty(remoteHost, "127.0.0.1"), remotePort)
	}
}

func ListLiveTunnels() []Tunnel {
	runMu.Lock()
	defer runMu.Unlock()
	out := make([]Tunnel, 0, len(running))
	for _, st := range running {
		if st.cmd == nil || st.cmd.Process == nil {
			continue
		}
		out = append(out, Tunnel{
			Name:       st.name,
			Mode:       st.mode,
			LocalPort:  st.localPort,
			RemoteHost: st.remoteHost,
			RemotePort: st.remotePort,
			Target:     st.target,
			Identity:   st.identity,
			Status:     "online",
			Uptime:     formatUptime(st.started),
			PID:        st.cmd.Process.Pid,
			Forward:    FormatForward(st.mode, st.localPort, st.remoteHost, st.remotePort),
		})
	}
	return out
}

func StartTunnel(cfg TunnelConfig) error {
	cfg.normalize()
	if cfg.Name == "" {
		return fmt.Errorf("nome vazio")
	}
	if cfg.Target == "" {
		return fmt.Errorf("target SSH obrigatório (user@host)")
	}
	if cfg.LocalPort <= 0 {
		return fmt.Errorf("porta local inválida")
	}
	if cfg.Mode != ModeDynamic && cfg.RemotePort <= 0 {
		return fmt.Errorf("porta remota inválida")
	}

	runMu.Lock()
	if st, ok := running[cfg.Name]; ok && st.cmd != nil && st.cmd.Process != nil {
		runMu.Unlock()
		return fmt.Errorf("%s já está rodando (pid %d)", cfg.Name, st.cmd.Process.Pid)
	}
	runMu.Unlock()

	args := tunnelArgs(cfg)
	cmd := exec.Command("ssh", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	st := &runState{
		cmd:        cmd,
		name:       cfg.Name,
		mode:       cfg.Mode,
		localPort:  cfg.LocalPort,
		remoteHost: cfg.RemoteHost,
		remotePort: cfg.RemotePort,
		target:     cfg.Target,
		identity:   cfg.Identity,
		started:    time.Now(),
	}
	runMu.Lock()
	running[cfg.Name] = st
	delete(lastExit, cfg.Name)
	runMu.Unlock()

	go consumeOutput(cfg.Name, stdout)
	go consumeOutput(cfg.Name, stderr)
	go func() {
		_ = cmd.Wait()
		runMu.Lock()
		if cur, ok := running[cfg.Name]; ok && cur.cmd == cmd {
			lastExit[cfg.Name] = tail(cur.logLines, 3)
			delete(running, cfg.Name)
		}
		runMu.Unlock()
	}()

	// Brief settle: ExitOnForwardFailure makes failed binds exit quickly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runMu.Lock()
		_, alive := running[cfg.Name]
		exit := lastExit[cfg.Name]
		runMu.Unlock()
		if !alive {
			if exit == "" {
				exit = "sem saída"
			}
			return fmt.Errorf("ssh saiu: %s", exit)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func StopTunnel(name string) error {
	name = sanitizeName(name)
	runMu.Lock()
	st, ok := running[name]
	runMu.Unlock()
	if !ok || st.cmd == nil || st.cmd.Process == nil {
		return fmt.Errorf("%s não está rodando", name)
	}
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

func tunnelArgs(cfg TunnelConfig) []string {
	args := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		// accept-new: 1ª conexão grava host key; evita falha muda do BatchMode
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if cfg.Identity != "" {
		args = append(args, "-i", cfg.Identity)
	}
	host := firstNonEmpty(cfg.RemoteHost, "127.0.0.1")
	switch cfg.Mode {
	case ModeRemote:
		// Abre porta no servidor → aponta pro serviço no PC (projeto).
		args = append(args, "-R", fmt.Sprintf("%d:%s:%d", cfg.LocalPort, host, cfg.RemotePort))
	case ModeDynamic:
		args = append(args, "-D", strconv.Itoa(cfg.LocalPort))
	default:
		args = append(args, "-L", fmt.Sprintf("%d:%s:%d", cfg.LocalPort, host, cfg.RemotePort))
	}
	args = append(args, cfg.Target)
	return args
}

// SuggestSSHTarget picks user@host from git remotes that aren't public git hosts.
func SuggestSSHTarget(remotes ...string) string {
	for _, remote := range remotes {
		if t := parseGitSSHTarget(remote); t != "" {
			return t
		}
	}
	return ""
}

func parseGitSSHTarget(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	var user, host string
	switch {
	case strings.HasPrefix(remote, "ssh://"):
		u := strings.TrimPrefix(remote, "ssh://")
		if i := strings.Index(u, "/"); i >= 0 {
			u = u[:i]
		}
		if i := strings.Index(u, "@"); i >= 0 {
			user, host = u[:i], u[i+1:]
		} else {
			host = u
		}
		if j := strings.Index(host, ":"); j >= 0 {
			host = host[:j]
		}
	case strings.Contains(remote, "@") && strings.Contains(remote, ":"):
		// git@host:path.git
		left, _, ok := strings.Cut(remote, ":")
		if !ok {
			return ""
		}
		if i := strings.Index(left, "@"); i >= 0 {
			user, host = left[:i], left[i+1:]
		}
	default:
		return ""
	}
	host = strings.TrimSpace(host)
	if host == "" || isPublicGitHost(host) {
		return ""
	}
	user = strings.TrimSpace(user)
	if user == "" || user == "git" {
		// remote git@vps — prefer deploy-style login on personal VPS
		return host
	}
	return user + "@" + host
}

func isPublicGitHost(host string) bool {
	h := strings.ToLower(host)
	switch h {
	case "github.com", "gitlab.com", "bitbucket.org", "ssh.dev.azure.com":
		return true
	}
	return strings.HasSuffix(h, ".github.com") || strings.HasSuffix(h, ".gitlab.com")
}

// DefaultRemoteTunnel builds a reverse tunnel exposing the project port on the remote.
func DefaultRemoteTunnel(projectName string, ports []int, framework, target string) TunnelConfig {
	port := SuggestPort(ports, framework)
	name := sanitizeName(projectName)
	if name == "" || name == "tunnel" {
		name = "app"
	}
	return TunnelConfig{
		Name:       name,
		Mode:       ModeRemote,
		LocalPort:  port,
		RemoteHost: "127.0.0.1",
		RemotePort: port,
		Target:     strings.TrimSpace(target),
	}
}

func consumeOutput(name string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		runMu.Lock()
		if st, ok := running[name]; ok {
			st.logLines = append(st.logLines, line)
			if len(st.logLines) > 200 {
				st.logLines = st.logLines[len(st.logLines)-200:]
			}
		}
		runMu.Unlock()
	}
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

func formatUptime(started time.Time) string {
	if started.IsZero() {
		return ""
	}
	d := time.Since(started).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func tail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ParseBind splits "host:port" or ":port" into host and port.
func ParseBind(s string) (host string, port int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, fmt.Errorf("bind vazio")
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		host = s[:i]
		if host == "" {
			host = "127.0.0.1"
		}
		port, err = strconv.Atoi(s[i+1:])
		if err != nil || port <= 0 {
			return "", 0, fmt.Errorf("porta inválida")
		}
		return host, port, nil
	}
	port, err = strconv.Atoi(s)
	if err != nil || port <= 0 {
		return "", 0, fmt.Errorf("porta inválida")
	}
	return "127.0.0.1", port, nil
}
