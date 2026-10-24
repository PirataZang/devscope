package hub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/devscopeutil"
)

// Coletores usados aqui. São variáveis para os testes trocarem por dados
// fixos; em produção são os mesmos da TUI.
var (
	dockerLogs      = collectors.DockerLogs
	dockerLogsSince = collectors.DockerLogsSince
	dockerExitState = inspectExit
	ghaAvailable    = collectors.GHAAvailable
	ghaAuthOK       = collectors.GHAAuthStatusOK
	ghaListRuns     = collectors.GHAListRuns
	ghaFailedLogs   = collectors.GHARunFailedLogs
	hasWorkflows    = func(path string) bool {
		_, err := os.Stat(collectors.GHAWorkflowsDir(path))
		return err == nil
	}
	gitCollect       = collectors.CollectAt
	gitBranch        = collectors.GitCurrentBranchName
	gitCommits       = collectors.CollectCommitsAt
	gitCommitMessage = collectors.CollectCommitFullMessage
	gitCommitFiles   = collectors.CollectCommitFiles
	gitWorkingDiff   = collectors.CollectWorkingTreeDiff
	gitExec          = collectors.GitExec
	gitWorktrees     = collectors.GitWorktrees
)

const (
	maxFileBytes = 1 << 20 // arquivo do Contrato: 1 MB antes do base64
	ciTTL        = 60 * time.Second
)

// ─── tipos do Contrato ──────────────────────────────────────────────────────

type Host struct {
	Name    string `json:"name"`
	CPU     int    `json:"cpu"`
	RAM     int    `json:"ram"`
	Disk    int    `json:"disk"`
	UptimeS int64  `json:"uptime_s"`
}

type CI struct {
	Conclusion string `json:"conclusion"`
	Workflow   string `json:"workflow"`
	Branch     string `json:"branch"`
	AgoS       int64  `json:"ago_s"`
}

type ProjectSummary struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Path            string           `json:"path"`
	State           string           `json:"state"`
	ContainersUp    int              `json:"containers_up"`
	ContainersTotal int              `json:"containers_total"`
	Down            []string         `json:"down"`
	Branch          string           `json:"branch"`
	Ahead           int              `json:"ahead"`
	Behind          int              `json:"behind"`
	Changes         int              `json:"changes"`
	Upstream        bool             `json:"upstream"`
	CI              *CI              `json:"ci"`
	Ports           []int            `json:"ports"`
	PortConflicts   map[int][]string `json:"port_conflicts"`
}

type Container struct {
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Health   string  `json:"health"`
	Status   string  `json:"status"`
	Image    string  `json:"image"`
	CPU      float64 `json:"cpu"`
	MemMB    int64   `json:"mem_mb"`
	Ports    string  `json:"ports"`
	Restart  string  `json:"restart"`
	IsData   bool    `json:"is_data"`
	ExitCode *int    `json:"exit_code,omitempty"`
	OOM      *bool   `json:"oom,omitempty"`
}

type ProjectDetail struct {
	ProjectSummary
	Containers []Container `json:"containers"`
}

type Commit struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	AgoS    int64  `json:"ago_s"`
}

type File struct {
	Name       string `json:"name"`
	ContentB64 string `json:"content_b64"`
}

type FileChange struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// Args reúne os argumentos de todos os comandos; cada handler lê os seus.
type Args struct {
	ProjectID  string `json:"project_id"`
	Container  string `json:"container"`
	Tail       int    `json:"tail"`
	Filter     string `json:"filter"`
	ErrorsOnly bool   `json:"errors_only"`
	SinceS     int    `json:"since_s"`
	AsFile     bool   `json:"as_file"`
	Limit      int    `json:"limit"`
	RunID      flexID `json:"run_id"`
	Hash       string `json:"hash"`
	Mode       string `json:"mode"`
}

// flexID aceita o id do run do GitHub como texto ou como número.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexID(n.String())
	return nil
}

// cmdError vira o {"ok": false, "error", "message"} do /v1/result.
type cmdError struct{ code, msg string }

func (e *cmdError) Error() string { return e.msg }

func cmdErr(code, msg string) error { return &cmdError{code, msg} }

func errorCode(err error) string {
	var e *cmdError
	if errors.As(err, &e) {
		return e.code
	}
	return "failed"
}

// ─── tabela de comandos ─────────────────────────────────────────────────────

type handler func(ctx context.Context, a *Agent, args Args) (any, error)

var handlers = map[string]handler{
	"projects":      handleProjects,
	"host":          handleHost,
	"project":       handleProject,
	"container":     handleContainer,
	"logs":          handleLogs,
	"ci_runs":       handleCIRuns,
	"ci_failed_log": handleCIFailedLog,
	"git_summary":   handleGitSummary,
	"git_commits":   handleGitCommits,
	"git_commit":    handleGitCommit,
	"git_changed":   handleGitChanged,
	"git_diff":      handleGitDiff,
	"git_branches":  handleGitBranches,
	"git_worktrees": handleGitWorktrees,
	"git_incoming":  handleGitIncoming,
	"git_pending":   handleGitPending,
}

// actionTypes são as ações do Contrato. Elas chegam na fase 2; até lá todas
// respondem actions_disabled, não importa o que o hub mande.
var actionTypes = map[string]bool{
	"restart_container": true, "restart_project": true, "restart_all": true,
	"up_project": true, "stop_project": true, "git_pull": true,
}

// ─── projetos e containers ──────────────────────────────────────────────────

func handleProjects(_ context.Context, a *Agent, _ Args) (any, error) {
	return map[string]any{"projects": a.summaries(a.store.Get().Projects)}, nil
}

func handleHost(_ context.Context, a *Agent, _ Args) (any, error) {
	return a.host(a.store.Get().HostMetrics), nil
}

func handleProject(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.project(args.ProjectID)
	if err != nil {
		return nil, err
	}
	d := ProjectDetail{ProjectSummary: a.summaries([]core.Project{p})[0], Containers: []Container{}}
	for _, c := range p.Containers {
		d.Containers = append(d.Containers, containerOut(p, c))
	}
	return d, nil
}

func handleContainer(_ context.Context, a *Agent, args Args) (any, error) {
	p, c, err := a.container(args)
	if err != nil {
		return nil, err
	}
	out := containerOut(p, c)
	if !collectors.IsContainerRunning(c) {
		if code, oom, err := dockerExitState(c.ID); err == nil {
			out.ExitCode, out.OOM = &code, &oom
		}
	}
	return out, nil
}

var errorLineRe = regexp.MustCompile(`(?i)error|warn|fatal|exception|sqlstate|panic`)

func handleLogs(_ context.Context, a *Agent, args Args) (any, error) {
	p, c, err := a.container(args)
	if err != nil {
		return nil, err
	}
	tail := args.Tail
	if tail <= 0 {
		tail = 200
	}
	tail = min(tail, 2000)
	var raw string
	if args.SinceS > 0 {
		raw, err = dockerLogsSince(c.ID, args.SinceS, tail)
	} else {
		raw, err = dockerLogs(c.ID, tail)
	}
	if err != nil && strings.TrimSpace(raw) == "" {
		return nil, cmdErr("docker", "docker logs falhou: "+err.Error())
	}
	// Filtra depois de mascarar: um filtro no texto cru deixaria adivinhar o
	// segredo letra por letra pelo que volta ou não volta.
	filter := strings.ToLower(args.Filter)
	lines := []string{}
	for _, l := range splitLines(raw) {
		l = devscopeutil.MaskLine(l)
		if filter != "" && !strings.Contains(strings.ToLower(l), filter) {
			continue
		}
		if args.ErrorsOnly && !errorLineRe.MatchString(l) {
			continue
		}
		lines = append(lines, l)
	}
	name := fmt.Sprintf("%s-%s.log", shortName(p, c), a.now().Format("1504"))
	text := strings.Join(lines, "\n")
	if args.AsFile || len(text) > maxFileBytes {
		return fileData(name, text), nil
	}
	return map[string]any{"lines": lines}, nil
}

// ─── CI ─────────────────────────────────────────────────────────────────────

type CIRun struct {
	ID         string `json:"id"`
	Workflow   string `json:"workflow"`
	Branch     string `json:"branch"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	AgoS       int64  `json:"ago_s"`
	URL        string `json:"url"`
}

func handleCIRuns(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.project(args.ProjectID)
	if err != nil {
		return nil, err
	}
	if !ghaAvailable() {
		return nil, cmdErr("gh_unavailable", "o gh não está instalado nesta máquina")
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	runs, err := ghaListRuns(p.Path, min(limit, 20))
	if err != nil {
		return nil, cmdErr("gh", "gh run list falhou: "+err.Error())
	}
	out := []CIRun{}
	for _, r := range runs {
		out = append(out, CIRun{
			ID: r.ID, Workflow: r.Workflow, Branch: r.Branch, Status: r.Status,
			Conclusion: r.Conclusion, AgoS: a.agoISO(r.UpdatedAt, r.CreatedAt), URL: r.URL,
		})
	}
	return map[string]any{"runs": out}, nil
}

var runIDRe = regexp.MustCompile(`^\d+$`)

func handleCIFailedLog(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.project(args.ProjectID)
	if err != nil {
		return nil, err
	}
	id := string(args.RunID)
	if !runIDRe.MatchString(id) {
		return nil, cmdErr("bad_request", "run_id inválido")
	}
	out, err := ghaFailedLogs(p.Path, id)
	if err != nil {
		return nil, cmdErr("gh", "gh run view falhou: "+err.Error())
	}
	return fileData("ci-"+id+".log", maskText(out)), nil
}

// projectCI é o último run do projeto, com cache de 60 s: o snapshot sai a
// cada 30 s e cada leitura custa um processo gh.
func (a *Agent) projectCI(p core.Project) *CI {
	a.ciMu.Lock()
	e, ok := a.ci[p.ID]
	a.ciMu.Unlock()
	if ok && a.now().Sub(e.at) < ciTTL {
		return e.ci
	}
	var runs []collectors.GHARun
	if ciCapable(p.Path) {
		runs, _ = ghaListRuns(p.Path, 1)
	}
	return a.setCI(p.ID, runs)
}

func ciCapable(path string) bool {
	return hasWorkflows(path) && ghaAvailable() && ghaAuthOK(path)
}

type ciEntry struct {
	at time.Time
	ci *CI
}

// setCI guarda o run mais recente; o vigia de CI também passa por aqui, o que
// poupa o gh do snapshot.
func (a *Agent) setCI(projectID string, runs []collectors.GHARun) *CI {
	var ci *CI
	if len(runs) > 0 {
		r := runs[0]
		ci = &CI{Conclusion: r.Conclusion, Workflow: r.Workflow,
			Branch: r.Branch, AgoS: a.agoISO(r.UpdatedAt, r.CreatedAt)}
	}
	a.ciMu.Lock()
	a.ci[projectID] = ciEntry{at: a.now(), ci: ci}
	a.ciMu.Unlock()
	return ci
}

// ─── git ────────────────────────────────────────────────────────────────────

type GitSummary struct {
	Branch    string   `json:"branch"`
	Upstream  string   `json:"upstream"` // "origin/main"; vazio sem upstream
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
	Modified  int      `json:"modified"`
	Staged    int      `json:"staged"`
	Untracked int      `json:"untracked"`
	Stash     int      `json:"stash"`
	Commits   []Commit `json:"commits"`
}

func handleGitSummary(_ context.Context, a *Agent, args Args) (any, error) {
	p, info, err := a.gitInfo(args)
	if err != nil {
		return nil, err
	}
	upstream, err := gitExec(p.Path, "rev-parse", "--abbrev-ref", "@{u}")
	if err != nil {
		upstream = ""
	}
	return GitSummary{
		// CollectGitSummary lê .git/HEAD e falha em worktree, onde .git é arquivo.
		Branch: gitBranch(p.Path), Upstream: upstream,
		Ahead: info.Ahead, Behind: info.Behind,
		Modified: info.Modified, Staged: info.Staged, Untracked: info.Untracked, Stash: info.StashCount,
		Commits: a.commits(info.Commits[:min(5, len(info.Commits))]),
	}, nil
}

func handleGitCommits(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.gitProject(args)
	if err != nil {
		return nil, err
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	return map[string]any{"commits": a.commits(gitCommits(p.Path, "HEAD", min(limit, 20)))}, nil
}

// hashRe barra qualquer coisa que o git leria como opção ("--output=…").
var hashRe = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

func handleGitCommit(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.gitProject(args)
	if err != nil {
		return nil, err
	}
	if !hashRe.MatchString(args.Hash) {
		return nil, cmdErr("bad_request", "hash inválido")
	}
	meta, err := gitExec(p.Path, "show", "-s", "--date=format:%Y-%m-%d %H:%M", "--format=%h%x09%an%x09%cd", args.Hash)
	f := strings.SplitN(meta, "\t", 3)
	if err != nil || len(f) != 3 {
		return nil, cmdErr("not_found", "commit não encontrado")
	}
	files := []FileChange{}
	for _, c := range gitCommitFiles(p.Path, args.Hash) {
		files = append(files, FileChange{Status: c.Status, Path: c.Path})
	}
	return map[string]any{
		"hash": f[0], "author": f[1], "date": f[2],
		"message": maskText(gitCommitMessage(p.Path, args.Hash)),
		"files":   files,
	}, nil
}

func handleGitChanged(_ context.Context, a *Agent, args Args) (any, error) {
	_, info, err := a.gitInfo(args)
	if err != nil {
		return nil, err
	}
	files := []FileChange{}
	for _, f := range info.Files {
		files = append(files, FileChange{Status: strings.TrimSpace(f.Staging + f.Worktree), Path: f.Path})
	}
	return map[string]any{"files": files}, nil
}

func handleGitDiff(_ context.Context, a *Agent, args Args) (any, error) {
	if args.Hash == "" {
		p, info, err := a.gitInfo(args)
		if err != nil {
			return nil, err
		}
		var parts []string
		for _, f := range info.Files {
			parts = append(parts, gitWorkingDiff(p.Path, f.Path))
		}
		return fileData("working-tree.diff", maskText(strings.Join(parts, "\n"))), nil
	}
	p, err := a.gitProject(args)
	if err != nil {
		return nil, err
	}
	if !hashRe.MatchString(args.Hash) {
		return nil, cmdErr("bad_request", "hash inválido")
	}
	out, err := gitExec(p.Path, "show", "--format=", args.Hash)
	if err != nil {
		return nil, cmdErr("not_found", "commit não encontrado")
	}
	return fileData(args.Hash+".diff", maskText(out)), nil
}

var (
	aheadRe  = regexp.MustCompile(`ahead (\d+)`)
	behindRe = regexp.MustCompile(`behind (\d+)`)
)

func handleGitBranches(_ context.Context, a *Agent, args Args) (any, error) {
	p, info, err := a.gitInfo(args)
	if err != nil {
		return nil, err
	}
	// Um processo só para o ahead/behind de todas: "ahead 2, behind 1".
	track, _ := gitExec(p.Path, "for-each-ref", "--format=%(refname:short)%09%(upstream:track,nobracket)", "refs/heads/")
	ab := map[string][2]int{}
	for _, l := range splitLines(track) {
		name, t, _ := strings.Cut(l, "\t")
		ab[name] = [2]int{atoiMatch(aheadRe, t), atoiMatch(behindRe, t)}
	}
	type branch struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
		Remote  bool   `json:"remote"`
		Ahead   int    `json:"ahead"`
		Behind  int    `json:"behind"`
	}
	out := []branch{}
	for _, b := range info.Branches {
		out = append(out, branch{b.Name, b.Current, b.Remote, ab[b.Name][0], ab[b.Name][1]})
	}
	return map[string]any{"branches": out}, nil
}

func handleGitWorktrees(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.gitProject(args)
	if err != nil {
		return nil, err
	}
	list, err := gitWorktrees(p.Path)
	if err != nil {
		return nil, cmdErr("git", "git worktree list falhou: "+err.Error())
	}
	type worktree struct {
		Path    string `json:"path"`
		Branch  string `json:"branch"`
		Changes int    `json:"changes"`
	}
	out := []worktree{}
	for _, w := range list {
		out = append(out, worktree{tildePath(w.Path), w.Branch, w.Changes})
	}
	return map[string]any{"worktrees": out}, nil
}

// handleGitIncoming mostra o que um pull traria segundo o último fetch. Sem
// upstream a lista vem vazia.
func handleGitIncoming(_ context.Context, a *Agent, args Args) (any, error) {
	p, err := a.gitProject(args)
	if err != nil {
		return nil, err
	}
	out, err := gitExec(p.Path, "log", "--format=%h%x09%s%x09%an%x09%ct", "HEAD..@{u}", "-10")
	commits := []Commit{}
	if err == nil {
		for _, l := range splitLines(out) {
			f := strings.SplitN(l, "\t", 4)
			if len(f) != 4 {
				continue
			}
			unix, _ := strconv.ParseInt(f[3], 10, 64)
			commits = append(commits, Commit{Hash: f[0], Subject: devscopeutil.MaskLine(f[1]), Author: f[2], AgoS: a.agoUnix(unix)})
		}
	}
	return map[string]any{"commits": commits}, nil
}

func handleGitPending(_ context.Context, a *Agent, _ Args) (any, error) {
	type pending struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
		Branch    string `json:"branch"`
		Changes   int    `json:"changes"`
		Ahead     int    `json:"ahead"`
		Behind    int    `json:"behind"`
		Upstream  bool   `json:"upstream"`
	}
	out := []pending{}
	for _, p := range a.store.Get().Projects {
		if p.Pending == nil {
			continue
		}
		out = append(out, pending{p.ID, p.Name, branchOf(p), p.Pending.Changes, p.Pending.Ahead, p.Pending.Behind, p.Pending.Upstream})
	}
	return map[string]any{"projects": out}, nil
}

// ─── leitura do estado ──────────────────────────────────────────────────────

func (a *Agent) project(id string) (core.Project, error) {
	for _, p := range a.store.Get().Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return core.Project{}, cmdErr("not_found", "projeto não encontrado nesta máquina")
}

// container acha pelo nome curto (o que o hub recebeu), pelo nome do docker
// ou pelo id.
func (a *Agent) container(args Args) (core.Project, core.Container, error) {
	p, err := a.project(args.ProjectID)
	if err != nil {
		return p, core.Container{}, err
	}
	for _, c := range p.Containers {
		if args.Container != "" && (args.Container == shortName(p, c) || args.Container == c.Name || args.Container == c.ID) {
			return p, c, nil
		}
	}
	return p, core.Container{}, cmdErr("not_found", "container não encontrado: "+args.Container)
}

func (a *Agent) gitProject(args Args) (core.Project, error) {
	p, err := a.project(args.ProjectID)
	if err == nil && (p.Git == nil || !p.Git.IsRepo) {
		err = cmdErr("not_git", "o projeto não é um repositório git")
	}
	return p, err
}

// gitInfo lê o repositório inteiro na hora: o estado da TUI só guarda o
// resumo (branch) da maioria dos projetos.
func (a *Agent) gitInfo(args Args) (core.Project, *core.GitInfo, error) {
	p, err := a.gitProject(args)
	if err != nil {
		return p, nil, err
	}
	info := gitCollect(p.Path)
	if info == nil || !info.IsRepo {
		return p, nil, cmdErr("not_git", "o projeto não é um repositório git")
	}
	return p, info, nil
}

func (a *Agent) host(m core.HostMetrics) Host {
	return Host{
		Name: a.cfg.Bot.Name(), CPU: pct(m.CPUPercent), RAM: pct(m.MemoryPercent),
		Disk: pct(m.DiskPercent), UptimeS: int64(m.Uptime.Seconds()),
	}
}

// summaries busca o CI dos projetos em paralelo: um gh lento não atrasa os
// outros.
func (a *Agent) summaries(projects []core.Project) []ProjectSummary {
	out := make([]ProjectSummary, len(projects))
	var wg sync.WaitGroup
	for i, p := range projects {
		out[i] = summarize(p)
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i].CI = a.projectCI(p)
		}()
	}
	wg.Wait()
	return out
}

func summarize(p core.Project) ProjectSummary {
	s := ProjectSummary{
		ID: p.ID, Name: p.Name, Path: tildePath(p.Path), Branch: branchOf(p),
		Ports: p.Ports, PortConflicts: p.PortConflicts,
	}
	if s.Ports == nil {
		s.Ports = []int{}
	}
	if s.PortConflicts == nil {
		s.PortConflicts = map[int][]string{}
	}
	s.State, s.ContainersUp, s.ContainersTotal, s.Down = projectState(p)
	if g := p.Pending; g != nil {
		s.Ahead, s.Behind, s.Changes, s.Upstream = g.Ahead, g.Behind, g.Changes, g.Upstream
	}
	return s
}

var exitedRe = regexp.MustCompile(`(?i)exited \((\d+)\)`)

// crashed separa queda de parada de propósito. ponytail: o código de saída
// decide; 0 e 143 (SIGTERM do docker stop) contam como parada, e um app que
// saia com outro código ao ser parado aparece como "down".
func crashed(c core.Container) bool {
	switch c.Status {
	case "restarting", "dead":
		return true
	case "running":
		return strings.EqualFold(c.Health, "unhealthy")
	}
	code := atoiMatch(exitedRe, c.State)
	return code != 0 && code != 143
}

// projectState: ok (tudo no ar), partial, down (algum caiu) ou stopped
// (parado de propósito). Projeto sem container usa o status da TUI.
func projectState(p core.Project) (state string, up, total int, down []string) {
	down = []string{}
	bad := false
	for _, c := range p.Containers {
		total++
		if collectors.IsContainerRunning(c) {
			up++
		} else {
			down = append(down, shortName(p, c))
		}
		bad = bad || crashed(c)
	}
	switch {
	case total == 0 && p.Status == core.StatusRunning:
		state = "ok"
	case total == 0 && p.Status == core.StatusDegraded:
		state = "partial"
	case total == 0:
		state = "stopped"
	case bad:
		state = "down"
	case up == total:
		state = "ok"
	case up == 0:
		state = "stopped"
	default:
		state = "partial"
	}
	return state, up, total, down
}

func containerOut(p core.Project, c core.Container) Container {
	out := Container{
		// core.Container guarda o estado do docker em Status e o texto
		// ("Up 2 days") em State; o Contrato usa o contrário.
		Name: shortName(p, c), State: c.Status, Health: c.Health, Status: c.State,
		Image: c.Image, CPU: math.Round(c.CPU*10) / 10, MemMB: c.Memory >> 20,
		Ports: c.Ports, Restart: c.Restart, IsData: collectors.IsDataContainer(c),
	}
	// O hub só desenha "parado de propósito" com exit_code 0; o "Exited (0)"
	// do docker ps já diz isso sem um docker inspect por container.
	if m := exitedRe.FindStringSubmatch(c.State); m != nil && !collectors.IsContainerRunning(c) {
		code, _ := strconv.Atoi(m[1])
		out.ExitCode = &code
	}
	return out
}

// shortName é o serviço do compose ("php-fpm"), a não ser que dois containers
// do projeto tenham o mesmo serviço: aí vale o nome do docker.
func shortName(p core.Project, c core.Container) string {
	if c.Service == "" {
		return c.Name
	}
	for _, o := range p.Containers {
		if o.ID != c.ID && o.Service == c.Service {
			return c.Name
		}
	}
	return c.Service
}

func inspectExit(id string) (code int, oom bool, err error) {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.ExitCode}} {{.State.OOMKilled}}", id).Output()
	if err != nil {
		return 0, false, err
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return 0, false, fmt.Errorf("docker inspect: saída inesperada %q", out)
	}
	code, err = strconv.Atoi(f[0])
	return code, f[1] == "true", err
}

// ─── utilidades ─────────────────────────────────────────────────────────────

func (a *Agent) commits(in []core.GitCommit) []Commit {
	out := make([]Commit, 0, len(in))
	for _, c := range in {
		out = append(out, Commit{Hash: c.Hash, Subject: devscopeutil.MaskLine(c.Message), Author: c.Author, AgoS: a.agoUnix(c.Unix)})
	}
	return out
}

func (a *Agent) agoUnix(unix int64) int64 {
	if unix <= 0 {
		return 0
	}
	return max(0, a.now().Unix()-unix)
}

func (a *Agent) agoISO(values ...string) int64 {
	for _, v := range values {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return a.agoUnix(t.Unix())
		}
	}
	return 0
}

// fileData monta o arquivo do Contrato. Acima de 1 MB corta pelo começo, numa
// linha inteira, e o nome ganha "-cortado".
func fileData(name, content string) map[string]File {
	if len(content) > maxFileBytes {
		content = content[len(content)-maxFileBytes:]
		if i := strings.IndexByte(content, '\n'); i >= 0 && i < len(content)-1 {
			content = content[i+1:]
		}
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext) + "-cortado" + ext
	}
	return map[string]File{"file": {Name: name, ContentB64: base64.StdEncoding.EncodeToString([]byte(content))}}
}

func maskText(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = devscopeutil.MaskLine(l)
	}
	return strings.Join(lines, "\n")
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && (p == home || strings.HasPrefix(p, home+string(filepath.Separator))) {
		return "~" + p[len(home):]
	}
	return p
}

func branchOf(p core.Project) string {
	if p.Git != nil {
		return p.Git.Branch
	}
	return ""
}

func pct(v float64) int { return int(math.Round(v)) }

func atoiMatch(re *regexp.Regexp, s string) int {
	if m := re.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}
