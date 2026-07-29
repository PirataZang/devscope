package cfutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ListLiveTunnels returns managed processes plus any cloudflared already
// running on the machine (quick / named / token).
func ListLiveTunnels() []Tunnel {
	metrics := probeMetricsPorts()
	managed := listManagedTunnels()
	byPID := map[int]bool{}
	out := make([]Tunnel, 0, len(managed)+4)
	for _, t := range managed {
		if t.PublicURL == "" {
			enrichFromMetrics(&t, metrics)
		}
		if t.PID > 0 {
			byPID[t.PID] = true
		}
		out = append(out, t)
	}
	for _, t := range discoverSystemTunnels(metrics) {
		if t.PID > 0 && byPID[t.PID] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func listManagedTunnels() []Tunnel {
	runMu.Lock()
	defer runMu.Unlock()
	out := make([]Tunnel, 0, len(running))
	for _, st := range running {
		if st.cmd == nil || st.cmd.Process == nil {
			continue
		}
		pub := st.publicURL
		host := st.hostname
		if host == "" {
			host = publicHost(pub)
		}
		out = append(out, Tunnel{
			Name:      st.name,
			Port:      parseAddrPort(st.url),
			Hostname:  host,
			PublicURL: pub,
			LocalURL:  st.url,
			Status:    "online",
			Mode:      st.mode,
			Uptime:    formatUptime(time.Since(st.started)),
			PID:       st.cmd.Process.Pid,
		})
	}
	return out
}

func discoverSystemTunnels(metrics []metricsSnap) []Tunnel {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []Tunnel
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		cmdline := readProcCmdline(pid)
		if len(cmdline) == 0 || !isCloudflaredCmd(cmdline) {
			continue
		}
		t := tunnelFromCmdline(pid, cmdline)
		enrichFromMetrics(&t, metrics)
		if t.Name == "" {
			t.Name = "cloudflared-" + strconv.Itoa(pid)
		}
		if t.Status == "" {
			t.Status = "online"
		}
		if t.Uptime == "" {
			t.Uptime = procUptime(pid)
		}
		out = append(out, t)
	}
	return out
}

func isCloudflaredCmd(args []string) bool {
	if len(args) == 0 {
		return false
	}
	base := filepath.Base(args[0])
	if base != "cloudflared" {
		return false
	}
	joined := strings.Join(args, " ")
	return strings.Contains(joined, "tunnel")
}

func readProcCmdline(pid int) []string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(b) == 0 {
		return nil
	}
	parts := strings.Split(string(b), "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func tunnelFromCmdline(pid int, args []string) Tunnel {
	t := Tunnel{PID: pid, Status: "online", Project: "(sistema)"}
	hasRun := false
	hasToken := false
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--url" && i+1 < len(args):
			t.LocalURL = args[i+1]
			t.Port = parseAddrPort(args[i+1])
			i++
		case strings.HasPrefix(a, "--url="):
			u := strings.TrimPrefix(a, "--url=")
			t.LocalURL = u
			t.Port = parseAddrPort(u)
		case a == "--token-file", a == "--token", strings.HasPrefix(a, "--token-file="), strings.HasPrefix(a, "--token="):
			hasToken = true
			if !strings.Contains(a, "=") && i+1 < len(args) {
				i++
			}
		case a == "--credentials-file", a == "--cred-file", strings.HasPrefix(a, "--credentials-file="), strings.HasPrefix(a, "--cred-file="):
			if !strings.Contains(a, "=") && i+1 < len(args) {
				i++
			}
		case a == "run":
			hasRun = true
		case strings.HasPrefix(a, "-"):
			// skip other flags; consume value if present as next token without dash
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.Contains(a, "=") {
				i++
			}
		case a == "tunnel", a == "cloudflared", strings.HasSuffix(a, "/cloudflared"):
			// ignore
		default:
			if filepath.Base(a) == "cloudflared" {
				continue
			}
			positional = append(positional, a)
		}
	}
	switch {
	case hasToken:
		t.Mode = "token"
		t.Name = "token"
	case hasRun:
		t.Mode = "named"
		if len(positional) > 0 {
			t.Name = sanitizeName(positional[len(positional)-1])
		}
	default:
		t.Mode = "quick"
	}
	if t.Name == "" && t.Port > 0 {
		t.Name = "quick-" + strconv.Itoa(t.Port)
	}
	if t.Name == "" {
		t.Name = "cloudflared-" + strconv.Itoa(pid)
	}
	return t
}

type metricsSnap struct {
	localURL  string
	quickHost string
	ha        int
}

func probeMetricsPorts() []metricsSnap {
	client := &http.Client{Timeout: 250 * time.Millisecond}
	var out []metricsSnap
	for port := 20241; port <= 20250; port++ {
		base := "http://127.0.0.1:" + strconv.Itoa(port)
		snap := metricsSnap{}
		if b, err := httpGet(client, base+"/quicktunnel"); err == nil {
			var q struct {
				Hostname string `json:"hostname"`
			}
			if json.Unmarshal(b, &q) == nil {
				snap.quickHost = strings.TrimSpace(q.Hostname)
			}
		}
		if b, err := httpGet(client, base+"/config"); err == nil {
			var cfg struct {
				Config struct {
					Ingress []struct {
						Service string `json:"service"`
					} `json:"ingress"`
				} `json:"config"`
			}
			if json.Unmarshal(b, &cfg) == nil {
				for _, in := range cfg.Config.Ingress {
					if in.Service != "" && !strings.HasPrefix(in.Service, "http_status:") {
						snap.localURL = in.Service
						break
					}
				}
			}
		}
		if b, err := httpGet(client, base+"/ready"); err == nil {
			var r struct {
				ReadyConnections int `json:"readyConnections"`
			}
			if json.Unmarshal(b, &r) == nil {
				snap.ha = r.ReadyConnections
			}
		}
		if snap.quickHost == "" && snap.localURL == "" && snap.ha == 0 {
			continue
		}
		out = append(out, snap)
	}
	return out
}

func httpGet(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 2048)
	for {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > 64*1024 {
				break
			}
		}
		if readErr != nil {
			break
		}
	}
	return buf, nil
}

func enrichFromMetrics(t *Tunnel, snaps []metricsSnap) {
	for _, s := range snaps {
		match := false
		if t.LocalURL != "" && s.localURL != "" && normalizeLocal(t.LocalURL) == normalizeLocal(s.localURL) {
			match = true
		}
		if t.Port > 0 && s.localURL != "" && parseAddrPort(s.localURL) == t.Port {
			match = true
		}
		if !match && t.Mode == "token" && s.localURL == "" && s.quickHost == "" && s.ha > 0 {
			// token tunnels often have empty ingress hostname; weak match last resort skipped
		}
		if !match {
			continue
		}
		if s.localURL != "" && t.LocalURL == "" {
			t.LocalURL = s.localURL
			t.Port = parseAddrPort(s.localURL)
		}
		if s.quickHost != "" {
			t.Hostname = s.quickHost
			t.PublicURL = "https://" + s.quickHost
			if t.Mode == "" {
				t.Mode = "quick"
			}
		}
		return
	}
	// Fallback: unique quicktunnel with same port already handled; if only one quick snap and this is quick, use it.
	if t.Mode == "quick" && t.PublicURL == "" {
		var quicks []metricsSnap
		for _, s := range snaps {
			if s.quickHost != "" {
				quicks = append(quicks, s)
			}
		}
		if len(quicks) == 1 {
			t.Hostname = quicks[0].quickHost
			t.PublicURL = "https://" + quicks[0].quickHost
			if t.LocalURL == "" && quicks[0].localURL != "" {
				t.LocalURL = quicks[0].localURL
				t.Port = parseAddrPort(quicks[0].localURL)
			}
		} else {
			for _, s := range quicks {
				if t.Port > 0 && parseAddrPort(s.localURL) == t.Port {
					t.Hostname = s.quickHost
					t.PublicURL = "https://" + s.quickHost
					return
				}
			}
		}
	}
}

func procUptime(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	// field 22 (1-based) is starttime in clock ticks after comm
	s := string(b)
	rparen := strings.LastIndex(s, ")")
	if rparen < 0 || rparen+2 >= len(s) {
		return ""
	}
	fields := strings.Fields(s[rparen+2:])
	if len(fields) < 20 {
		return ""
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return ""
	}
	upB, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	upFields := strings.Fields(string(upB))
	if len(upFields) == 0 {
		return ""
	}
	upSec, err := strconv.ParseFloat(upFields[0], 64)
	if err != nil {
		return ""
	}
	// ponytail: assume 100 Hz; fine for display
	const hz = 100.0
	sec := upSec - float64(startTicks)/hz
	if sec < 0 {
		sec = 0
	}
	return formatUptime(time.Duration(sec * float64(time.Second)))
}

// StopPID sends interrupt to an external cloudflared process.
func StopPID(pid int) error {
	if pid <= 1 {
		return os.ErrInvalid
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return proc.Kill()
	}
	return nil
}
