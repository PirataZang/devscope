package collectors

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

var portMappingRe = regexp.MustCompile(`:(\d+)->`)

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
	for _, m := range portMappingRe.FindAllStringSubmatch(s, -1) {
		if len(m) > 1 {
			if p, err := strconv.Atoi(m[1]); err == nil {
				ports = append(ports, p)
			}
		}
	}
	return ports
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
