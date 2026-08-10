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
	TabSwarm
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
	TabCFTunnel
	TabSSH
	TabJenkins
	TabActions
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
	case TabSwarm:
		return "Swarm"
	case TabHealth:
		return "Status"
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
	case TabCFTunnel:
		return "CF Tunnel"
	case TabSSH:
		return "SSH Tunnel"
	case TabJenkins:
		return "Jenkins"
	case TabActions:
		return "GH Actions"
	default:
		return "Overview"
	}
}

// AllTabs follows sidebar order (WATCH → SCOPE → AUTOMATION → MANAGER → TUNNEL → TOOLS).
var AllTabs = []Tab{
	TabOverview, TabMetrics, TabHealth,
	TabGit, TabContainers,
	TabActions, TabJenkins,
	TabSwarm, TabKubernetes,
	TabNgrok, TabSSH, TabCFTunnel,
	TabRoutes, TabAPI, TabDatabase, TabWebSocket,
}
