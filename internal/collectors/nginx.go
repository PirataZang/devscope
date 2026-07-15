package collectors

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

var (
	serverNameRe = regexp.MustCompile(`server_name\s+([^;]+);`)
	proxyPassRe  = regexp.MustCompile(`proxy_pass\s+https?://[^/:]+:(\d+)`)
	listenRe     = regexp.MustCompile(`listen\s+(?:\[::\]:|)(?:\d+\.){0,3}(\d+)`)
)

// CollectNginxDomains parses nginx site configs for server_name and proxy_pass.
func CollectNginxDomains() []core.Domain {
	var domains []core.Domain
	paths := []string{
		"/etc/nginx/sites-enabled",
		"/etc/nginx/conf.d",
	}
	for _, root := range paths {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, e.Name()))
			if err != nil {
				continue
			}
			domains = append(domains, parseNginxConfig(string(data))...)
		}
	}
	return domains
}

func parseNginxConfig(content string) []core.Domain {
	var domains []core.Domain
	blocks := strings.Split(content, "server {")
	for _, block := range blocks {
		if !strings.Contains(block, "server_name") {
			continue
		}
		names := serverNameRe.FindStringSubmatch(block)
		if len(names) < 2 {
			continue
		}
		hosts := strings.Fields(strings.TrimSpace(names[1]))
		if len(hosts) == 0 || hosts[0] == "_" || hosts[0] == "localhost" {
			continue
		}
		port := 80
		if m := listenRe.FindStringSubmatch(block); len(m) > 1 {
			if p, err := strconv.Atoi(m[1]); err == nil {
				port = p
			}
		}
		proxyTo := ""
		if m := proxyPassRe.FindStringSubmatch(block); len(m) > 1 {
			proxyTo = ":" + m[1]
		}
		ssl := strings.Contains(block, "ssl") || port == 443
		for _, h := range hosts {
			if h == "_" {
				continue
			}
			domains = append(domains, core.Domain{
				Host:    h,
				Port:    port,
				SSL:     ssl,
				ProxyTo: proxyTo,
			})
		}
	}
	return domains
}

// AssignDomainsToProjects matches nginx domains to projects by port overlap.
func AssignDomainsToProjects(projects []core.Project, domains []core.Domain) {
	for i := range projects {
		projects[i].Domains = nil
	}
	for _, d := range domains {
		bestIdx := -1
		for i, p := range projects {
			for _, port := range p.Ports {
				if d.Port == port || strings.Contains(d.ProxyTo, strconv.Itoa(port)) {
					bestIdx = i
					break
				}
			}
		}
		if bestIdx >= 0 {
			projects[bestIdx].Domains = append(projects[bestIdx].Domains, d)
		}
	}
}
