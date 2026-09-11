package ui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/cfutil"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jenkinsutil"
	"github.com/devscope/devscope/internal/nginxutil"
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
	// A sondagem já monta a lista; guardar só a contagem era jogar fora
	// justamente o que a abertura do módulo tem para mostrar.
	ghaProcNames []string

	cfAuth cfutil.AuthInfo

	sshAvail bool
	sshVer   string
	sshLive  int

	swarmAvail   bool
	swarmInfo    collectors.SwarmInfo
	swarmCompose string

	k8sAvail         bool
	k8sCtx           string
	k8sManifests     int
	k8sManifestNames []string

	jenkinsCfg jenkinsutil.ProjectConfig

	nginxFound bool
	nginxDir   string
	nginxCount int
	nginxNames []string
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
				for _, pr := range procs {
					msg.ghaProcNames = append(msg.ghaProcNames, firstNonEmpty(pr.Name, pr.File))
				}
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
			manifests := collectors.DiscoverProjectManifests(path)
			msg.k8sManifests = len(manifests)
			for _, m := range manifests {
				msg.k8sManifestNames = append(msg.k8sManifestNames, filepath.Base(m))
			}
		case TabJenkins:
			msg.jenkinsCfg = jenkinsutil.LoadProject(path)
		case TabNginx:
			layout, sites, err := nginxutil.Discover(path)
			msg.nginxFound = err == nil
			msg.nginxCount = len(sites)
			for _, site := range sites {
				msg.nginxNames = append(msg.nginxNames, site.Name)
			}
			if layout.SitesDir != "" {
				msg.nginxDir = layout.SitesDir
			}
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
		a.landingGHANames = msg.ghaProcNames
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
		a.landingK8sNames = msg.k8sManifestNames
		a.landingK8sOK = true
	case TabJenkins:
		a.landingJenkins = msg.jenkinsCfg
		a.landingJenkinsOK = true
	case TabNginx:
		a.landingNginxFound = msg.nginxFound
		a.landingNginxDir = msg.nginxDir
		a.landingNginxCount = msg.nginxCount
		a.landingNginxNames = msg.nginxNames
		a.landingNginxOK = true
	}
}
