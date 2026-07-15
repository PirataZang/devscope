package collectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

var envPortRe = regexp.MustCompile(`(?m)(?:^|\s)(?:PORT|APP_PORT|SERVER_PORT|HTTP_PORT|VITE_PORT)\s*=\s*(\d+)\s*$`)

// CollectHealth runs HTTP/TCP checks for each project.
func CollectHealth(ctx context.Context, projects []core.Project, cfg config.HealthConfig) {
	if cfg.Concurrent <= 0 {
		cfg.Concurrent = 10
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}

	sem := make(chan struct{}, cfg.Concurrent)
	var wg sync.WaitGroup

	for i := range projects {
		targets := inferHealthTargets(&projects[i])
		if len(targets) == 0 {
			projects[i].Health = core.HealthUnknown
			projects[i].HealthChecks = nil
			continue
		}

		results := make([]core.HealthCheckResult, len(targets))
		var mu sync.Mutex
		var hasFail, hasOK bool

		for ti, url := range targets {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, target string) {
				defer wg.Done()
				defer func() { <-sem }()
				res := checkTarget(ctx, target, cfg.Timeout)
				mu.Lock()
				results[idx] = res
				if res.Status == core.HealthHealthy {
					hasOK = true
				} else if res.Status == core.HealthUnhealthy {
					hasFail = true
				}
				mu.Unlock()
			}(ti, url)
		}
		wg.Wait()

		projects[i].HealthChecks = results
		switch {
		case hasFail && hasOK:
			projects[i].Health = core.HealthUnhealthy
		case hasFail:
			projects[i].Health = core.HealthUnhealthy
		case hasOK:
			projects[i].Health = core.HealthHealthy
		default:
			projects[i].Health = core.HealthUnknown
		}
	}
}

func inferHealthTargets(p *core.Project) []string {
	var targets []string
	seen := make(map[string]bool)
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		targets = append(targets, u)
	}

	for _, port := range p.Ports {
		add(fmt.Sprintf("http://127.0.0.1:%d/", port))
		add(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		add(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
		add(fmt.Sprintf("tcp://127.0.0.1:%d", port))
	}

	for _, c := range p.Containers {
		for _, port := range parseContainerPorts(c.Ports) {
			add(fmt.Sprintf("http://127.0.0.1:%d/", port))
			add(fmt.Sprintf("tcp://127.0.0.1:%d", port))
		}
	}

	if port := readEnvPort(p.Path); port > 0 {
		add(fmt.Sprintf("http://127.0.0.1:%d/", port))
	}

	for _, d := range p.Domains {
		if d.Host != "" && d.Host != "_" {
			scheme := "http"
			if d.SSL {
				scheme = "https"
			}
			port := d.Port
			if port <= 0 {
				port = 80
				if d.SSL {
					port = 443
				}
			}
			add(fmt.Sprintf("%s://127.0.0.1:%d/", scheme, port))
			add(fmt.Sprintf("%s://%s/", scheme, d.Host))
		}
	}

	if len(targets) > 6 {
		targets = targets[:6]
	}
	return targets
}

func readEnvPort(projectPath string) int {
	for _, name := range []string{".env", ".env.local", ".env.production"} {
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			continue
		}
		if m := envPortRe.FindSubmatch(data); len(m) > 1 {
			p, _ := strconv.Atoi(string(m[1]))
			if p > 0 {
				return p
			}
		}
	}
	return 0
}

func checkTarget(ctx context.Context, target string, timeout time.Duration) core.HealthCheckResult {
	start := time.Now()
	res := core.HealthCheckResult{
		URL:       target,
		Status:    core.HealthUnknown,
		CheckedAt: start,
	}

	if strings.HasPrefix(target, "tcp://") {
		addr := strings.TrimPrefix(target, "tcp://")
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		res.LatencyMS = time.Since(start).Milliseconds()
		if err != nil {
			res.Status = core.HealthUnhealthy
			res.Message = err.Error()
			return res
		}
		conn.Close()
		res.Status = core.HealthHealthy
		return res
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		res.Status = core.HealthUnhealthy
		res.Message = err.Error()
		return res
	}

	resp, err := client.Do(req)
	res.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		res.Status = core.HealthUnhealthy
		res.Message = err.Error()
		return res
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		res.Status = core.HealthHealthy
		res.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		res.Status = core.HealthUnhealthy
		res.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return res
}
