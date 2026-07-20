package ngrokutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultAgent = "http://127.0.0.1:4040"

// Tunnel is a merged view of a live agent tunnel + project config.
type Tunnel struct {
	Name      string
	Project   string
	Port      int
	Proto     string
	Domain    string
	PublicURL string
	LocalURL  string
	Status    string // online | offline | starting
	Region    string
	Uptime    string
	Requests  int64
	BytesIn   int64
	BytesOut  int64
	PID       int
}

type Request struct {
	ID        string
	Time      time.Time
	Method    string
	Status    int
	Path      string
	Host      string
	LatencyMS int64
	IP        string
	UserAgent string
}

type AgentInfo struct {
	Connected bool
	Version   string
	URI       string
}

func Available() bool {
	_, err := exec.LookPath("ngrok")
	return err == nil
}

func Version() string {
	out, err := exec.Command("ngrok", "version").CombinedOutput()
	if err != nil {
		return ""
	}
	// "ngrok version 3.5.0\n"
	s := strings.TrimSpace(string(out))
	s = strings.TrimPrefix(s, "ngrok version ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s
	}
	return fields[0]
}

func AgentBase() string {
	if v := os.Getenv("NGROK_AGENT_ADDR"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAgent
}

func PingAgent() AgentInfo {
	info := AgentInfo{URI: AgentBase(), Version: Version()}
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(AgentBase() + "/api/tunnels")
	if err != nil {
		return info
	}
	defer resp.Body.Close()
	info.Connected = resp.StatusCode == 200
	return info
}

func ListLiveTunnels() ([]Tunnel, error) {
	body, err := agentGET("/api/tunnels")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Tunnels []struct {
			Name      string `json:"name"`
			ID        string `json:"id"`
			PublicURL string `json:"public_url"`
			Proto     string `json:"proto"`
			Config    struct {
				Addr string `json:"addr"`
			} `json:"config"`
			Metrics struct {
				Conns struct {
					Count int64 `json:"count"`
				} `json:"conns"`
				HTTP struct {
					Count int64 `json:"count"`
				} `json:"http"`
			} `json:"metrics"`
		} `json:"tunnels"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]Tunnel, 0, len(raw.Tunnels))
	for _, t := range raw.Tunnels {
		port := parseAddrPort(t.Config.Addr)
		reqs := t.Metrics.HTTP.Count
		if reqs == 0 {
			reqs = t.Metrics.Conns.Count
		}
		name := t.Name
		if name == "" {
			name = t.ID
		}
		if name == "" {
			name = "tunnel"
		}
		domain := publicHost(t.PublicURL)
		out = append(out, Tunnel{
			Name:      name,
			Port:      port,
			Proto:     t.Proto,
			Domain:    domain,
			PublicURL: t.PublicURL,
			LocalURL:  normalizeLocal(t.Config.Addr),
			Status:    "online",
			Requests:  reqs,
		})
	}
	return out, nil
}

func ListHTTPRequests(limit int) ([]Request, error) {
	if limit <= 0 {
		limit = 40
	}
	body, err := agentGET("/api/requests/http")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Requests []struct {
			URI        string `json:"uri"`
			ID         string `json:"id"`
			TunnelName string `json:"tunnel_name"`
			Start      string `json:"start"`
			Duration   int64  `json:"duration"`
			Request    struct {
				Method  string            `json:"method"`
				URI     string            `json:"uri"`
				Headers map[string][]string `json:"headers"`
			} `json:"request"`
			Response struct {
				Status string `json:"status"`
			} `json:"response"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]Request, 0, len(raw.Requests))
	for i, r := range raw.Requests {
		if i >= limit {
			break
		}
		st := 0
		fmt.Sscanf(r.Response.Status, "%d", &st)
		ua, ip, host := "", "", ""
		if h := r.Request.Headers; h != nil {
			if v := h["User-Agent"]; len(v) > 0 {
				ua = v[0]
			}
			if v := h["X-Forwarded-For"]; len(v) > 0 {
				ip = v[0]
			}
			if v := h["Host"]; len(v) > 0 {
				host = v[0]
			}
		}
		ts, _ := time.Parse(time.RFC3339Nano, r.Start)
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339, r.Start)
		}
		out = append(out, Request{
			ID:        r.ID,
			Time:      ts,
			Method:    r.Request.Method,
			Status:    st,
			Path:      r.Request.URI,
			Host:      host,
			LatencyMS: r.Duration / int64(time.Millisecond),
			IP:        ip,
			UserAgent: ua,
		})
	}
	return out, nil
}

func StartTunnel(name string, port int, proto string) error {
	if port <= 0 {
		return fmt.Errorf("porta inválida")
	}
	if proto == "" {
		proto = "http"
	}
	name = sanitizeName(name)
	if PingAgent().Connected {
		return startViaAPI(name, port, proto)
	}
	return startProcess(name, port, proto)
}

func StopTunnel(name string) error {
	name = sanitizeName(name)
	if !PingAgent().Connected {
		return fmt.Errorf("agente ngrok offline")
	}
	req, err := http.NewRequest(http.MethodDelete, AgentBase()+"/api/tunnels/"+name, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func startViaAPI(name string, port int, proto string) error {
	payload := map[string]any{
		"name":  name,
		"addr":  strconv.Itoa(port),
		"proto": proto,
	}
	if proto == "http" || proto == "https" {
		payload["proto"] = "http"
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(AgentBase()+"/api/tunnels", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	return nil
}

func startProcess(name string, port int, proto string) error {
	args := []string{proto, strconv.Itoa(port), "--log=stdout"}
	if name != "" {
		args = append(args, "--name="+name)
	}
	cmd := exec.Command("ngrok", args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Detach: don't wait; agent API will appear shortly.
	go func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if PingAgent().Connected {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("ngrok iniciado mas API local não respondeu em :4040")
}

func agentGET(path string) ([]byte, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(AgentBase() + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("agent %s: %s", path, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func parseAddrPort(addr string) int {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
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

func publicHost(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
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

// SuggestPort picks a likely local port from project ports / stack defaults.
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
