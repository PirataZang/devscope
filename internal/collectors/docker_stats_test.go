package collectors

import (
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestParseCPUPerc(t *testing.T) {
	if v := parseCPUPerc("12.34%"); v < 12.3 || v > 12.4 {
		t.Fatalf("got %v", v)
	}
}

func TestParseMemUsageBytes(t *testing.T) {
	if v := parseMemUsageBytes("128.5MiB / 16GiB"); v < 128*1024*1024 {
		t.Fatalf("got %d", v)
	}
}

func TestApplyDockerStats(t *testing.T) {
	projects := []core.Project{{
		Containers: []core.Container{{ID: "abc123def456"}},
	}}
	stats := map[string]struct {
		CPU    float64
		Memory int64
	}{
		"abc123def456": {CPU: 10.5, Memory: 100 * 1024 * 1024},
	}
	ApplyDockerStats(projects, stats)
	if projects[0].Metrics.CPUPercent != 10.5 {
		t.Fatalf("cpu %v", projects[0].Metrics.CPUPercent)
	}
	if projects[0].Metrics.MemoryMB != 100 {
		t.Fatalf("ram %v", projects[0].Metrics.MemoryMB)
	}
}

func TestParseContainerPorts(t *testing.T) {
	ports := parseContainerPorts("0.0.0.0:3000->3000/tcp, :::5173->5173/tcp")
	if len(ports) < 2 || ports[0] != 3000 {
		t.Fatalf("ports %v", ports)
	}
}

func TestParseContainerPortMappings(t *testing.T) {
	ms := ParseContainerPortMappings("0.0.0.0:3000->3000/tcp, 127.0.0.1:8080->80/tcp")
	if len(ms) != 2 {
		t.Fatalf("got %v", ms)
	}
	if ms[0].HostPort != 3000 || ms[0].ContainerPort != 3000 || ms[0].Proto != "tcp" {
		t.Fatalf("first %v", ms[0])
	}
	if ms[1].HostPort != 8080 || ms[1].ContainerPort != 80 || ms[1].HostIP != "127.0.0.1" {
		t.Fatalf("second %v", ms[1])
	}
}

func TestFormatPortsShort(t *testing.T) {
	s := FormatPortsShort([]int{3000, 5173, 8080}, 2)
	if s == "-" || s == "" {
		t.Fatal(s)
	}
}

func TestApplyProjectStatusDegraded(t *testing.T) {
	projects := []core.Project{{
		Path:   "/var/www/api",
		Health: core.HealthUnhealthy,
		Containers: []core.Container{{Status: "running"}},
	}}
	ApplyProjectStatus(projects, nil)
	if projects[0].Status != core.StatusDegraded {
		t.Fatalf("status %s", projects[0].Status)
	}
}

func TestSortPinnedFirst(t *testing.T) {
	projects := []core.Project{
		{Path: "/b"},
		{Path: "/a"},
	}
	out := SortPinnedFirst(projects, []string{"/b"})
	if out[0].Path != "/b" {
		t.Fatalf("got %s", out[0].Path)
	}
}
