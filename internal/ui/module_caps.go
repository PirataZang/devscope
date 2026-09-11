package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

// Capacidades do projeto — quais módulos fazem sentido para o que está aberto.
//
// A sidebar listava os quinze módulos sempre, em todo projeto. Num projeto Go
// sem Docker, sem Jenkins e sem túnel isso é uma lista de coisas que não
// existem: o usuário lê quinze linhas para achar as três que usa.
//
// O DevScope já tinha uma descoberta de capacidade — probeToolLanding, em
// landing_probe.go —, mas ela é PREGUIÇOSA e POR ABA: só roda quando você já
// entrou no módulo. Serve para a landing do módulo dizer "kubectl não está
// instalado"; não serve para a sidebar, que precisa saber ANTES de você ir lá.
//
// Então aqui existe uma segunda descoberta, deliberadamente mais burra e mais
// barata: só exec.LookPath e os.Stat. Nenhum processo é criado, nenhuma rede é
// tocada — as sondagens caras (kubectl config current-context, ngrok ping,
// varredura do nginx) continuam com o probeToolLanding, onde já estavam.
//
// Duas regras que valem mais que a precisão da detecção:
//
//  1. NA DÚVIDA, MOSTRE. Enquanto a sondagem não voltou, tudo aparece. Módulo
//     escondido por engano é funcionalidade perdida; módulo a mais é uma linha.
//  2. NADA SOME DE VERDADE. O que não é relevante vai para trás de uma linha
//     "⋯ N módulos ocultos", que a tecla `t` abre. Ver moduleCapsHiddenHint.

// moduleCaps é o resultado da sondagem para UM projeto.
type moduleCaps struct {
	path  string // projeto medido — troca de projeto invalida
	ready bool
	on    map[Tab]bool
}

// relevant diz se o módulo entra na sidebar. Antes da sondagem responder,
// tudo é relevante.
func (c moduleCaps) relevant(t Tab) bool {
	if !c.ready {
		return true
	}
	return c.on[t]
}

type moduleCapsMsg struct {
	path string
	on   map[Tab]bool
}

// hasTool guarda o resultado do LookPath pelo tempo de vida do processo: o PATH
// não muda no meio da sessão, e a sondagem roda a cada troca de projeto.
var (
	toolOnce  sync.Mutex
	toolCache = map[string]bool{}
)

func hasTool(name string) bool {
	toolOnce.Lock()
	defer toolOnce.Unlock()
	if v, ok := toolCache[name]; ok {
		return v
	}
	_, err := exec.LookPath(name)
	toolCache[name] = err == nil
	return err == nil
}

func dirExists(parts ...string) bool {
	st, err := os.Stat(filepath.Join(parts...))
	return err == nil && st.IsDir()
}

func fileExists(parts ...string) bool {
	st, err := os.Stat(filepath.Join(parts...))
	return err == nil && !st.IsDir()
}

// probeModuleCaps mede o projeto fora do render (regra do DESIGN.md §13: View
// não chama o sistema). É barato o bastante para rodar a cada abertura de
// projeto e a cada `r`.
func (a *App) probeModuleCaps(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	// Copia o que precisa do projeto: a goroutine não pode ler o ponteiro que
	// a UI vai trocar debaixo dela.
	isRepo := p.Git != nil && p.Git.IsRepo
	docker := p.HasDockerCompose || p.HasDockerfile || p.ContainerCount > 0
	served := len(p.Ports) > 0 || len(p.Domains) > 0
	frameworks := len(projectFrameworks(*p)) > 0

	return func() tea.Msg {
		on := map[Tab]bool{
			// Sempre: é a porta de entrada do projeto.
			TabOverview: true,
			// Scratchpads: não dependem do projeto, são ferramentas de mesa.
			TabAPI: true, TabDatabase: true, TabWebSocket: true,

			TabGit:        isRepo,
			TabContainers: docker || hasTool("docker"),
			TabActions:    dirExists(path, ".github", "workflows"),
			TabJenkins:    fileExists(path, "Jenkinsfile") || fileExists(path, ".devscope", "jenkins.json"),
			TabSwarm:      hasTool("docker") && (dirExists(path, "swarm") || fileExists(path, "docker-stack.yml") || fileExists(path, "stack.yml")),
			TabKubernetes: hasTool("kubectl") && k8sManifestDir(path),
			TabNginx:      hasTool("nginx") || dirExists(path, "nginx") || fileExists(path, "nginx.conf"),
			TabRoutes:     frameworks || served,
			TabNgrok:      hasTool("ngrok"),
			TabCFTunnel:   hasTool("cloudflared"),
			TabSSH:        hasTool("ssh") && served,
		}
		return moduleCapsMsg{path: path, on: on}
	}
}

// k8sManifestDir espelha os diretórios que collectors.DiscoverProjectManifests
// varre — aqui só perguntamos se a pasta existe, sem entrar nela.
func k8sManifestDir(path string) bool {
	for _, d := range []string{"k8s", "kubernetes", "manifests", ".k8s", "deploy", "deployments"} {
		if dirExists(path, d) {
			return true
		}
	}
	return false
}

func (a *App) handleModuleCapsMsg(msg moduleCapsMsg) {
	if p := a.currentProject(); p == nil || p.Path != msg.path {
		return
	}
	a.moduleCaps = moduleCaps{path: msg.path, ready: true, on: msg.on}
}

// ─── o que a sidebar e o teclado enxergam ───────────────────────────────────

// tabVisible: o módulo aparece na sidebar e entra no ciclo do `tab`.
//
// A aba ATIVA é sempre visível, mesmo irrelevante — você pode ter chegado nela
// pelo modo "mostrar todos, e a sidebar não pode apagar a linha de onde você
// está.
func (a *App) tabVisible(t Tab) bool {
	if t == a.tab || a.showAllModules {
		return true
	}
	return a.moduleCaps.relevant(t)
}

// visibleTabs é a ordem de navegação do `tab`/`shift+tab`: só o que está na
// tela. Ciclar por módulo invisível faria a seleção sumir da sidebar.
func (a *App) visibleTabs() []Tab {
	out := make([]Tab, 0, len(AllTabs))
	for _, t := range AllTabs {
		if a.tabVisible(t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return AllTabs
	}
	return out
}

// hiddenTabCount é o que a linha "⋯ N módulos" anuncia.
func (a *App) hiddenTabCount() int {
	if a.showAllModules {
		return 0
	}
	n := 0
	for _, t := range AllTabs {
		if !a.tabVisible(t) {
			n++
		}
	}
	return n
}
