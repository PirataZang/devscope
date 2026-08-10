package collectors

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

// hostPort→containerPort[/proto], optional host IP prefix.
var portMappingFullRe = regexp.MustCompile(`(?:([\d.]+|\[?[0-9a-fA-F:]+\]?):)?(\d+)->(\d+)(?:/(tcp|udp))?`)

// PortMapping is one published host→container port from docker ps.
type PortMapping struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Proto         string
	Raw           string
}

// AssignPortsToProjects fills Project.Ports from container mappings and compose files.
func AssignPortsToProjects(projects []core.Project, _ map[int]bool) {
	for i := range projects {
		seen := make(map[int]bool)
		var ports []int
		add := func(p int) {
			if p <= 0 || seen[p] {
				return
			}
			seen[p] = true
			ports = append(ports, p)
		}

		for _, c := range projects[i].Containers {
			for _, p := range parseContainerPorts(c.Ports) {
				add(p)
			}
		}
		for _, p := range ParseComposePorts(projects[i].Path) {
			add(p)
		}
		projects[i].Ports = ports
	}
}

func parseContainerPorts(s string) []int {
	var ports []int
	seen := make(map[int]bool)
	for _, m := range ParseContainerPortMappings(s) {
		if m.HostPort <= 0 || seen[m.HostPort] {
			continue
		}
		seen[m.HostPort] = true
		ports = append(ports, m.HostPort)
	}
	return ports
}

// ParseContainerPortMappings parses docker ps Ports column into structured mappings.
func ParseContainerPortMappings(s string) []PortMapping {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	var out []PortMapping
	for _, m := range portMappingFullRe.FindAllStringSubmatch(s, -1) {
		hostPort, err1 := strconv.Atoi(m[2])
		contPort, err2 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil || hostPort <= 0 {
			continue
		}
		proto := m[4]
		if proto == "" {
			proto = "tcp"
		}
		hostIP := m[1]
		raw := m[0]
		if hostIP != "" {
			raw = hostIP + ":" + m[2] + "->" + m[3] + "/" + proto
		} else {
			raw = m[2] + "->" + m[3] + "/" + proto
		}
		out = append(out, PortMapping{
			HostIP:        hostIP,
			HostPort:      hostPort,
			ContainerPort: contPort,
			Proto:         proto,
			Raw:           raw,
		})
	}
	return out
}

// ProbePortPreview GETs http://127.0.0.1:<port>/ and returns a short text preview.
func ProbePortPreview(port int) string {
	if port <= 0 {
		return "porta inválida"
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("GET %s\n%s", url, err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var b strings.Builder
	fmt.Fprintf(&b, "GET %s\nHTTP %s\n", url, resp.Status)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		fmt.Fprintf(&b, "Content-Type: %s\n", ct)
	}
	b.WriteString("\n")
	text := strings.ReplaceAll(string(body), "\r", "")
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		text = stripHTMLRough(text)
	}
	b.WriteString(strings.TrimSpace(text))
	return strings.TrimRight(b.String(), "\n")
}

func stripHTMLRough(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// ReadListeningPorts reads /proc/net/tcp and /proc/net/tcp6 for LISTEN sockets.
func ReadListeningPorts() map[int]bool {
	result := make(map[int]bool)
	readProcTCP("/proc/net/tcp", result)
	readProcTCP("/proc/net/tcp6", result)
	return result
}

func readProcTCP(path string, out map[int]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		if fields[3] != "0A" { // LISTEN
			continue
		}
		portHex := fields[1]
		if idx := strings.Index(portHex, ":"); idx >= 0 {
			portHex = portHex[idx+1:]
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err == nil && port > 0 {
			out[int(port)] = true
		}
	}
}

// FormatPortsShort renders ports for dashboard, e.g. ":3000 :5173".
func FormatPortsShort(ports []int, max int) string {
	if len(ports) == 0 {
		return "-"
	}
	if max <= 0 {
		max = 2
	}
	var parts []string
	for i, p := range ports {
		if i >= max {
			parts = append(parts, fmt.Sprintf("+%d", len(ports)-max))
			break
		}
		parts = append(parts, fmt.Sprintf(":%d", p))
	}
	return strings.Join(parts, " ")
}

// PortFromHex parses hex port from /proc/net/tcp address field (unused helper for tests).
func PortFromHex(hexPort string) (int, error) {
	b, err := strconv.ParseUint(hexPort, 16, 16)
	if err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint16([]byte{byte(b >> 8), byte(b)})), nil
}

// MatchProjectPorts filters listening ports likely belonging to a project (container host ports).
func MatchProjectPorts(p core.Project) []int {
	return p.Ports
}
