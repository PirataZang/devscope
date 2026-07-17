package collectors

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

type dockerStatRow struct {
	ID           string `json:"ID"`
	CPUPerc      string `json:"CPUPerc"`
	MemUsage     string `json:"MemUsage"`
	MemPerc      string `json:"MemPerc"`
	BlockIO      string `json:"BlockIO"`
	NetIO        string `json:"NetIO"`
	PIDs         string `json:"PIDs"`
	Name         string `json:"Name"`
}

// CollectDockerStats returns CPU% and memory bytes keyed by short container ID.
func CollectDockerStats(ctx context.Context) map[string]struct {
	CPU    float64
	Memory int64
} {
	return collectDockerStats(ctx, nil)
}

// CollectDockerStatsForIDs returns stats only for the given container IDs.
func CollectDockerStatsForIDs(ctx context.Context, ids []string) map[string]struct {
	CPU    float64
	Memory int64
} {
	return collectDockerStats(ctx, ids)
}

func collectDockerStats(ctx context.Context, ids []string) map[string]struct {
	CPU    float64
	Memory int64
} {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}
	args := []string{"stats", "--no-stream",
		"--format", `{"ID":"{{.ID}}","CPUPerc":"{{.CPUPerc}}","MemUsage":"{{.MemUsage}}","Name":"{{.Name}}"}`,
	}
	if len(ids) > 0 {
		args = append(args, ids...)
	}
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil
	}

	result := make(map[string]struct {
		CPU    float64
		Memory int64
	})
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var row dockerStatRow
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		id := row.ID
		if len(id) > 12 {
			id = id[:12]
		}
		cpu := parseCPUPerc(row.CPUPerc)
		mem := parseMemUsageBytes(row.MemUsage)
		result[id] = struct {
			CPU    float64
			Memory int64
		}{CPU: cpu, Memory: mem}
	}
	return result
}

func ApplyDockerStats(projects []core.Project, stats map[string]struct {
	CPU    float64
	Memory int64
}) {
	if len(stats) == 0 {
		return
	}
	for i := range projects {
		var cpuTotal float64
		var memTotal int64
		for j := range projects[i].Containers {
			c := &projects[i].Containers[j]
			if s, ok := stats[c.ID]; ok {
				c.CPU = s.CPU
				c.Memory = s.Memory
				cpuTotal += s.CPU
				memTotal += s.Memory
			}
		}
		projects[i].Metrics = core.ProjectMetrics{
			CPUPercent: cpuTotal,
			MemoryMB:   memTotal / (1024 * 1024),
		}
	}
}

func parseCPUPerc(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseMemUsageBytes(s string) int64 {
	// format: "123.4MiB / 16GiB"
	parts := strings.Split(s, "/")
	if len(parts) == 0 {
		return 0
	}
	return parseSizeBytes(strings.TrimSpace(parts[0]))
}

func parseSizeBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "KiB"):
		mult = 1024
		s = strings.TrimSuffix(s, "KiB")
	case strings.HasSuffix(s, "MiB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "MiB")
	case strings.HasSuffix(s, "GiB"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GiB")
	case strings.HasSuffix(s, "KB"):
		mult = 1000
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "MB"):
		mult = 1000 * 1000
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "GB"):
		mult = 1000 * 1000 * 1000
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int64(v * float64(mult))
}
