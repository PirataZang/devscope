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
	TabHealth
	TabLogs
	TabMetrics
	TabAPI
	TabDB
)

func (t Tab) String() string {
	switch t {
	case TabOverview:
		return "Overview"
	case TabGit:
		return "Git"
	case TabContainers:
		return "Containers"
	case TabHealth:
		return "Health"
	case TabLogs:
		return "Logs"
	case TabMetrics:
		return "Metrics"
	case TabAPI:
		return "API"
	case TabDB:
		return "Database"
	default:
		return "Overview"
	}
}

var AllTabs = []Tab{TabOverview, TabGit, TabContainers, TabHealth, TabLogs, TabMetrics, TabAPI, TabDB}
