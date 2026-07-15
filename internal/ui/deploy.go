package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

type deployDoneMsg struct {
	err error
}

type lazyGitDoneMsg struct {
	err error
}

func (a *App) runDeploy(p *core.Project) tea.Cmd {
	if p.DeployScript == "" {
		return nil
	}
	script := p.DeployScript
	return tea.ExecProcess(deployCommand(p.Path, script), func(err error) tea.Msg {
		return deployDoneMsg{err: err}
	})
}

func deployCommand(projectPath, script string) *exec.Cmd {
	cmd := exec.Command("/bin/bash", "-c", script)
	cmd.Dir = projectPath
	if script == "make deploy" {
		cmd = exec.Command("make", "deploy")
		cmd.Dir = projectPath
	} else if script == "npm run deploy" {
		cmd = exec.Command("npm", "run", "deploy")
		cmd.Dir = projectPath
	} else if filepath.Base(script) == "deploy.sh" {
		cmd = exec.Command("/bin/bash", script)
		cmd.Dir = projectPath
	}
	return cmd
}

func (a *App) openLazyGit(path string) tea.Cmd {
	if _, err := exec.LookPath("lazygit"); err != nil {
		a.statusMsg = "lazygit não encontrado no PATH"
		return nil
	}
	cmd := exec.Command("lazygit")
	cmd.Dir = path
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return lazyGitDoneMsg{err: err}
	})
}

func (a *App) openProjectURL(p *core.Project) {
	url := projectPrimaryURL(p)
	if url == "" {
		a.statusMsg = "nenhuma URL detectada"
		return
	}
	_ = exec.Command("xdg-open", url).Start()
}

func projectPrimaryURL(p *core.Project) string {
	for _, d := range p.Domains {
		if d.Host != "" && d.Host != "_" {
			scheme := "http"
			if d.SSL {
				scheme = "https"
			}
			return scheme + "://" + d.Host + "/"
		}
	}
	if len(p.Ports) > 0 && p.Ports[0] > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d/", p.Ports[0])
	}
	return ""
}
