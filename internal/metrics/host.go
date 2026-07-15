package metrics

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

type HostCollector struct {
	prevCPU cpuSample
}

type cpuSample struct {
	total uint64
	idle  uint64
	time  time.Time
}

func NewHostCollector() *HostCollector {
	return &HostCollector{}
}

func (c *HostCollector) Collect() core.HostMetrics {
	if runtime.GOOS == "windows" {
		return c.collectWindows()
	}
	return core.HostMetrics{
		CPUPercent:    c.cpuPercent(),
		MemoryPercent: c.memoryPercent(),
		MemoryUsedMB:  c.memoryUsedMB(),
		MemoryTotalMB: c.memoryTotalMB(),
		DiskPercent:   c.diskPercent("/"),
		DiskUsedGB:    c.diskUsedGB("/"),
		DiskTotalGB:   c.diskTotalGB("/"),
		SwapPercent:   c.swapPercent(),
		Uptime:        readUptime(),
		LoadAvg:       readLoadAvg(),
		ProcessCount:  countProcesses(),
		DockerRunning: countDockerRunning(),
		LoggedInUsers: countLoggedInUsers(),
		OSInfo:        readOSInfo(),
	}
}

func (c *HostCollector) cpuPercent() float64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}

	var vals []uint64
	for _, field := range fields[1:] {
		v, _ := strconv.ParseUint(field, 10, 64)
		vals = append(vals, v)
	}
	if len(vals) < 4 {
		return 0
	}

	var total uint64
	for _, v := range vals {
		total += v
	}
	idle := vals[3]

	if c.prevCPU.total > 0 {
		dTotal := float64(total - c.prevCPU.total)
		dIdle := float64(idle - c.prevCPU.idle)
		if dTotal > 0 {
			pct := (1.0 - dIdle/dTotal) * 100
			c.prevCPU = cpuSample{total: total, idle: idle, time: time.Now()}
			return pct
		}
	}
	c.prevCPU = cpuSample{total: total, idle: idle, time: time.Now()}
	return 0
}

func (c *HostCollector) memoryPercent() float64 {
	total := c.memoryTotalMB()
	used := c.memoryUsedMB()
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func (c *HostCollector) memoryTotalMB() int64 {
	lines := readProcMeminfo()
	total, _ := parseMemLine(lines, "MemTotal:")
	return total / 1024
}

func (c *HostCollector) memoryUsedMB() int64 {
	lines := readProcMeminfo()
	total, _ := parseMemLine(lines, "MemTotal:")
	free, _ := parseMemLine(lines, "MemAvailable:")
	if free == 0 {
		free, _ = parseMemLine(lines, "MemFree:")
	}
	used := total - free
	if used < 0 {
		used = 0
	}
	return used / 1024
}

func (c *HostCollector) swapPercent() float64 {
	lines := readProcMeminfo()
	total, _ := parseMemLine(lines, "SwapTotal:")
	free, _ := parseMemLine(lines, "SwapFree:")
	if total == 0 {
		return 0
	}
	return float64(total-free) / float64(total) * 100
}

func readProcMeminfo() map[string]int64 {
	result := make(map[string]int64)
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return result
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		val, _ := strconv.ParseInt(parts[1], 10, 64)
		result[key] = val
	}
	return result
}

func parseMemLine(lines map[string]int64, key string) (int64, bool) {
	v, ok := lines[key]
	return v, ok
}

func (c *HostCollector) diskPercent(path string) float64 {
	total, free, err := getDiskSpace(path)
	if err != nil || total == 0 {
		return 0
	}
	return float64(total-free) / float64(total) * 100
}

func (c *HostCollector) diskUsedGB(path string) float64 {
	total, free, err := getDiskSpace(path)
	if err != nil {
		return 0
	}
	return float64(total-free) / (1024 * 1024 * 1024)
}

func (c *HostCollector) diskTotalGB(path string) float64 {
	total, _, err := getDiskSpace(path)
	if err != nil {
		return 0
	}
	return float64(total) / (1024 * 1024 * 1024)
}

func readUptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

func readLoadAvg() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "0.00"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "0.00"
	}
	return fields[0]
}

func countProcesses() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err == nil {
			count++
		}
	}
	return count
}

func countDockerRunning() int {
	if _, err := exec.LookPath("docker"); err != nil {
		return 0
	}
	out, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil {
		return 0
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return 0
	}
	return len(strings.Fields(string(out)))
}

func countLoggedInUsers() int {
	out, err := exec.Command("who").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if lines[0] == "" {
		return 0
	}
	return len(lines)
}

func readOSInfo() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	osName := runtime.GOOS
	if runtime.GOOS == "linux" {
		osName = "Linux"
	} else if len(osName) > 0 {
		osName = strings.ToUpper(osName[:1]) + osName[1:]
	}
	return osName + " " + arch
}

func (c *HostCollector) collectWindows() core.HostMetrics {
	// Executa query PowerShell em 1 comando rápido para obter CPU, RAM, Memória Virtual e Uptime
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$os = Get-CimInstance Win32_OperatingSystem; `+
		`$cpu = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average; `+
		`$uptime = [math]::Round([DateTime]::Now.Subtract($os.LastBootUpTime).TotalSeconds); `+
		`Write-Host ("{0}|{1}|{2}|{3}|{4}|{5}" -f $cpu, $os.TotalVisibleMemorySize, $os.FreePhysicalMemory, $os.TotalVirtualMemorySize, $os.FreeVirtualMemory, $uptime)`)

	out, err := cmd.Output()
	if err != nil {
		return core.HostMetrics{
			OSInfo:        readOSInfo(),
			DockerRunning: countDockerRunning(),
			LoadAvg:       "N/A",
		}
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) < 6 {
		return core.HostMetrics{
			OSInfo:        readOSInfo(),
			DockerRunning: countDockerRunning(),
			LoadAvg:       "N/A",
		}
	}

	cpu, _ := strconv.ParseFloat(parts[0], 64)
	memTotalKB, _ := strconv.ParseInt(parts[1], 10, 64)
	memFreeKB, _ := strconv.ParseInt(parts[2], 10, 64)
	virtualTotalKB, _ := strconv.ParseInt(parts[3], 10, 64)
	virtualFreeKB, _ := strconv.ParseInt(parts[4], 10, 64)
	uptimeSecs, _ := strconv.ParseInt(parts[5], 10, 64)

	memTotalMB := memTotalKB / 1024
	memFreeMB := memFreeKB / 1024
	memUsedMB := memTotalMB - memFreeMB
	if memUsedMB < 0 {
		memUsedMB = 0
	}

	memPercent := 0.0
	if memTotalMB > 0 {
		memPercent = float64(memUsedMB) / float64(memTotalMB) * 100
	}

	// Swap virtual = total virtual - físico
	swapTotalKB := virtualTotalKB - memTotalKB
	swapFreeKB := virtualFreeKB - memFreeKB
	if swapTotalKB < 0 {
		swapTotalKB = 0
	}
	if swapFreeKB < 0 {
		swapFreeKB = 0
	}
	swapUsedKB := swapTotalKB - swapFreeKB
	if swapUsedKB < 0 {
		swapUsedKB = 0
	}

	swapPercent := 0.0
	if swapTotalKB > 0 {
		swapPercent = float64(swapUsedKB) / float64(swapTotalKB) * 100
	}

	return core.HostMetrics{
		CPUPercent:    cpu,
		MemoryPercent: memPercent,
		MemoryUsedMB:  memUsedMB,
		MemoryTotalMB: memTotalMB,
		DiskPercent:   c.diskPercent("C:"),
		DiskUsedGB:    c.diskUsedGB("C:"),
		DiskTotalGB:   c.diskTotalGB("C:"),
		SwapPercent:   swapPercent,
		Uptime:        time.Duration(uptimeSecs) * time.Second,
		LoadAvg:       "N/A",
		ProcessCount:  countProcessesWindows(),
		DockerRunning: countDockerRunning(),
		LoggedInUsers: 1,
		OSInfo:        readOSInfo(),
	}
}

func countProcessesWindows() int {
	out, err := exec.Command("tasklist", "/nh").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return len(lines)
}
