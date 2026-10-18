package ui

type View int

const (
	ViewDashboard View = iota
	ViewProject
	ViewHelp
	ViewRelax
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
	TabLogs
	TabAPI
	TabDatabase
	TabJSON
	TabJWT
	TabWebSocket
	TabNgrok
	TabCFTunnel
	TabJenkins
	TabActions
	TabNginx
	TabWorktrees
)

func (t Tab) String() string {
	switch t {
	case TabOverview:
		return "Visão Geral"
	case TabGit:
		return "Git"
	case TabWorktrees:
		return "Worktrees"
	case TabContainers:
		return "Containers"
	case TabKubernetes:
		return "Kubernetes"
	case TabSwarm:
		return "Swarm"
	case TabLogs:
		return "Logs"
	case TabAPI:
		return "API"
	case TabDatabase:
		return "Database"
	case TabJSON:
		return "JSON"
	case TabJWT:
		return "JWT"
	case TabWebSocket:
		return "WS"
	case TabNgrok:
		return "Ngrok"
	case TabCFTunnel:
		return "CF Tunnel"
	case TabJenkins:
		return "Jenkins"
	case TabActions:
		return "GH Actions"
	case TabNginx:
		return "Nginx"
	default:
		return "Overview"
	}
}

// AllTabs é a ordem de navegação do `tab`, e tem de ser a MESMA da sidebar
// (PROJETO → CÓDIGO → EXECUÇÃO → REDE → DADOS): quando as duas divergem, o
// `tab` pula para uma linha que está acima na tela e a navegação vira loteria.
// project_sidebar_test.go trava as duas juntas.
//
// É a lista COMPLETA. Quem decide o que aparece é App.visibleTabs(), que filtra
// por capacidade do projeto (module_caps.go).
var AllTabs = []Tab{
	TabOverview,
	TabGit, TabWorktrees, TabActions, TabJenkins,
	TabContainers, TabSwarm, TabKubernetes,
	TabNginx, TabNgrok, TabCFTunnel,
	TabAPI, TabDatabase, TabWebSocket,
}
