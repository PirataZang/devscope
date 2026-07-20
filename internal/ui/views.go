package ui

type View int

const (
	ViewDashboard View = iota
	ViewProject
	ViewHelp
)

type dashboardSubview int

const (
	dashboardSubviewList dashboardSubview = iota
	dashboardSubviewShellReturn
)

type Tab int

const (
	TabOverview Tab = iota
	TabGit
	TabContainers
	TabKubernetes
	TabHealth
	TabLogs
	TabMetrics
	TabAPI
	TabDatabase
	TabJSON
	TabJWT
	TabRoutes
	TabWebSocket
	TabNgrok
)

func (t Tab) String() string {
	switch t {
	case TabOverview:
		return "Visão Geral"
	case TabGit:
		return "Git"
	case TabContainers:
		return "Containers"
	case TabKubernetes:
		return "Kubernetes"
	case TabHealth:
		return "Health"
	case TabLogs:
		return "Logs"
	case TabMetrics:
		return "Metrics"
	case TabAPI:
		return "API"
	case TabDatabase:
		return "Database"
	case TabJSON:
		return "JSON"
	case TabJWT:
		return "JWT"
	case TabRoutes:
		return "Rotas"
	case TabWebSocket:
		return "WS"
	case TabNgrok:
		return "Ngrok"
	default:
		return "Overview"
	}
}

// AllTabs follows sidebar order (SCOPE → WATCH → TOOLS → UTILS).
var AllTabs = []Tab{
	TabOverview, TabGit, TabContainers, TabKubernetes,
	TabHealth, TabLogs, TabMetrics,
	TabAPI, TabDatabase, TabWebSocket, TabNgrok,
	TabJSON, TabJWT, TabRoutes,
}
