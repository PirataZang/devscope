package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/cfutil"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jenkinsutil"
	"github.com/devscope/devscope/internal/ngrokutil"
	"github.com/devscope/devscope/internal/sshutil"
)

// toolLandingMsg carries async probe results for module landings.
// Landings must never call CLI/network in View — only read cached fields.
type toolLandingMsg struct {
	tab Tab

	ngrokAvail bool
	ngrokAgent ngrokutil.AgentInfo
	ngrokVer   string

	ghaInfo  collectors.GHAInfo
	ghaProcs int

	cfAuth cfutil.AuthInfo

	sshAvail bool
	sshVer   string
	sshLive  int

	swarmAvail   bool
	swarmInfo    collectors.SwarmInfo
	swarmCompose string

	k8sAvail     bool
	k8sCtx       string
	k8sManifests int

	jenkinsCfg jenkinsutil.ProjectConfig
}

func (a *App) probeToolLanding(tab Tab, p *core.Project) tea.Cmd {
	path, remote := "", ""
	if p != nil {
		path = p.Path
		if p.Git != nil {
			remote = p.Git.Remote
		}
	}
	return func() tea.Msg {
		msg := toolLandingMsg{tab: tab}
		switch tab {
		case TabNgrok:
			msg.ngrokAvail = ngrokutil.Available()
			msg.ngrokAgent = ngrokutil.PingAgent()
			if msg.ngrokAvail {
				msg.ngrokVer = ngrokutil.Version()
			}
		case TabActions:
			msg.ghaInfo = collectors.GHARepoInfo(path, remote)
			if procs, err := collectors.GHAListLocalWorkflowFiles(path); err == nil {
				msg.ghaProcs = len(procs)
			}
		case TabCFTunnel:
			msg.cfAuth = cfutil.Auth()
		case TabSSH:
			msg.sshAvail = sshutil.Available()
			msg.sshLive = len(sshutil.ListLiveTunnels())
			if msg.sshAvail {
				msg.sshVer = sshutil.Version()
			}
		case TabSwarm:
			msg.swarmAvail = collectors.SwarmAvailable()
			msg.swarmInfo = collectors.SwarmClusterInfo()
			msg.swarmCompose = collectors.DiscoverSwarmCompose(path)
		case TabKubernetes:
			msg.k8sAvail = collectors.K8sAvailable()
			msg.k8sCtx = collectors.K8sCurrentContext()
			msg.k8sManifests = len(collectors.DiscoverProjectManifests(path))
		case TabJenkins:
			msg.jenkinsCfg = jenkinsutil.LoadProject(path)
		}
		return msg
	}
}

func (a *App) handleToolLandingMsg(msg toolLandingMsg) {
	if a.tab != msg.tab {
		return
	}
	switch msg.tab {
	case TabNgrok:
		a.landingNgrokAvail = msg.ngrokAvail
		a.landingNgrokAgent = msg.ngrokAgent
		a.landingNgrokVer = msg.ngrokVer
		a.landingNgrokOK = true
	case TabActions:
		a.landingGHA = msg.ghaInfo
		a.landingGHAProcs = msg.ghaProcs
		a.landingGHAOK = true
	case TabCFTunnel:
		a.landingCF = msg.cfAuth
		a.landingCFOK = true
	case TabSSH:
		a.landingSSHAvail = msg.sshAvail
		a.landingSSHVer = msg.sshVer
		a.landingSSHLive = msg.sshLive
		a.landingSSHOK = true
	case TabSwarm:
		a.landingSwarmAvail = msg.swarmAvail
		a.landingSwarm = msg.swarmInfo
		a.landingSwarmCompose = msg.swarmCompose
		a.landingSwarmOK = true
	case TabKubernetes:
		a.landingK8sAvail = msg.k8sAvail
		a.landingK8sCtx = msg.k8sCtx
		a.landingK8sManifests = msg.k8sManifests
		a.landingK8sOK = true
	case TabJenkins:
		a.landingJenkins = msg.jenkinsCfg
		a.landingJenkinsOK = true
	}
}
