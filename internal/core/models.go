package core

import "time"

type ProjectStatus string

const (
	StatusRunning  ProjectStatus = "Running"
	StatusStopped  ProjectStatus = "Stopped"
	StatusDegraded ProjectStatus = "Degraded"
	StatusUnknown  ProjectStatus = "Unknown"
)

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "Healthy"
	HealthUnhealthy HealthStatus = "Unhealthy"
	HealthUnknown   HealthStatus = "Unknown"
)

type ServiceType string

const (
	ServiceDocker  ServiceType = "Docker"
	ServicePM2     ServiceType = "PM2"
	ServiceSystemd ServiceType = "Systemd"
	ServiceNginx   ServiceType = "Nginx"
	ServiceRedis   ServiceType = "Redis"
	ServicePostgres ServiceType = "Postgres"
)

type FrameworkInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Language string `json:"language"`
}

type GitFileStatus struct {
	Staging  string `json:"staging"`
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
}

type GitCommit struct {
	Hash        string `json:"hash"`
	Message     string `json:"message"`
	Author      string `json:"author"`
	Date        string `json:"date"`
	FullMessage string `json:"full_message,omitempty"`
}

type GitCommitFileChange struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type GitBranch struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Remote  bool   `json:"remote"`
}

type GitInfo struct {
	Branch         string          `json:"branch"`
	Ahead          int             `json:"ahead"`
	Behind         int             `json:"behind"`
	LastCommit     string          `json:"last_commit"`
	LastCommitMsg  string          `json:"last_commit_msg"`
	LastCommitDate time.Time       `json:"last_commit_date"`
	Author         string          `json:"author"`
	Modified       int             `json:"modified"`
	Untracked      int             `json:"untracked"`
	StashCount     int             `json:"stash_count"`
	Remote         string          `json:"remote"`
	IsRepo         bool            `json:"is_repo"`
	Files          []GitFileStatus `json:"files,omitempty"`
	Commits        []GitCommit     `json:"commits,omitempty"`
	Branches       []GitBranch     `json:"branches,omitempty"`
}

type Service struct {
	Type   ServiceType `json:"type"`
	Name   string      `json:"name"`
	Status string      `json:"status"`
	PID    int         `json:"pid,omitempty"`
	Port   int         `json:"port,omitempty"`
	CPU    float64     `json:"cpu"`
	Memory int64       `json:"memory"`
	Role   string      `json:"role,omitempty"`
}

type Container struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Image       string  `json:"image"`
	Status      string  `json:"status"`
	State       string  `json:"state"`
	Health      string  `json:"health,omitempty"`
	CPU         float64 `json:"cpu"`
	Memory      int64   `json:"memory"`
	Ports       string  `json:"ports,omitempty"`
	Restart     string  `json:"restart,omitempty"`
	ProjectPath string  `json:"project_path,omitempty"`
}

type Worker struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	Memory  int64   `json:"memory"`
	Restarts int    `json:"restarts"`
}

type Domain struct {
	Host    string `json:"host"`
	SSL     bool   `json:"ssl"`
	Port    int    `json:"port"`
	ProxyTo string `json:"proxy_to,omitempty"`
}

type SSLCert struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
	AutoRenew bool      `json:"auto_renew"`
}

type ProjectModule struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Role string `json:"role"`
}

type ProjectMetrics struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   int64   `json:"memory_mb"`
}

type HealthCheckResult struct {
	URL       string       `json:"url"`
	Status    HealthStatus `json:"status"`
	LatencyMS int64        `json:"latency_ms"`
	Message   string       `json:"message,omitempty"`
	CheckedAt time.Time    `json:"checked_at"`
}

type Project struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Path            string          `json:"path"`
	Framework       FrameworkInfo   `json:"framework"`
	Frameworks      []FrameworkInfo `json:"frameworks,omitempty"`
	Status          ProjectStatus   `json:"status"`
	Health          HealthStatus    `json:"health"`
	Git             *GitInfo        `json:"git,omitempty"`
	Services        []Service       `json:"services,omitempty"`
	Containers      []Container     `json:"containers,omitempty"`
	Workers         []Worker        `json:"workers,omitempty"`
	Domains         []Domain        `json:"domains,omitempty"`
	SSL             []SSLCert       `json:"ssl,omitempty"`
	Metrics         ProjectMetrics        `json:"metrics"`
	HealthChecks    []HealthCheckResult   `json:"health_checks,omitempty"`
	DeployScript    string                `json:"deploy_script,omitempty"`
	Ports           []int                 `json:"ports,omitempty"`
	LastDeploy      *time.Time      `json:"last_deploy,omitempty"`
	Uptime          time.Duration   `json:"uptime"`
	Modules         []ProjectModule `json:"modules,omitempty"`
	ContainerCount  int             `json:"container_count"`
	WorkerCount     int             `json:"worker_count"`
	HasDockerCompose bool           `json:"has_docker_compose"`
	HasDockerfile    bool           `json:"has_dockerfile"`
}

type HostMetrics struct {
	CPUPercent    float64       `json:"cpu_percent"`
	MemoryPercent float64       `json:"memory_percent"`
	MemoryUsedMB  int64         `json:"memory_used_mb"`
	MemoryTotalMB int64         `json:"memory_total_mb"`
	DiskPercent   float64       `json:"disk_percent"`
	DiskUsedGB    float64       `json:"disk_used_gb"`
	DiskTotalGB   float64       `json:"disk_total_gb"`
	SwapPercent   float64       `json:"swap_percent"`
	Uptime        time.Duration `json:"uptime"`
	LoadAvg       string        `json:"load_avg"`
	ProcessCount  int           `json:"process_count"`
	DockerRunning int           `json:"docker_running"`
	LoggedInUsers int           `json:"logged_in_users"`
	OSInfo        string        `json:"os_info"`
}

type Snapshot struct {
	Projects    []Project   `json:"projects"`
	HostMetrics HostMetrics `json:"host_metrics"`
	ScannedAt   time.Time   `json:"scanned_at"`
	ScanPaths   []string    `json:"scan_paths"`
	ProjectCount int        `json:"project_count"`
}
