package hub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

var fixedNow = time.Date(2026, 10, 21, 21, 14, 3, 0, time.UTC)

const pid = "3f9a1c0b7d2e4f61"

// fixtureAgent é uma máquina com um projeto laradock: php-fpm caiu por falta
// de memória, postgres e nginx no ar.
func fixtureAgent(t *testing.T) *Agent {
	t.Helper()
	isolate(t)
	store := core.NewStateStore(nil)
	store.SetProjects([]core.Project{{
		ID: pid, Name: "digiliza", Path: "/srv/digiliza", Status: core.StatusRunning,
		Git:     &core.GitInfo{IsRepo: true, Branch: "DES-2900"},
		Pending: &core.GitPending{Changes: 4, Ahead: 2, Upstream: true},
		Ports:   []int{80, 443, 5432}, PortConflicts: map[int][]string{5432: {"crm"}},
		Containers: []core.Container{
			{ID: "aaa", Name: "laradock-php-fpm-1", Service: "php-fpm", Image: "laradock-php-fpm", Status: "exited",
				State: "Exited (137) 2 minutes ago", Restart: "unless-stopped"},
			{ID: "bbb", Name: "laradock-postgres-1", Service: "postgres", Image: "postgres:16", Status: "running",
				State: "Up 2 days (healthy)", Health: "healthy", CPU: 1.234, Memory: 84 << 20, Ports: "0.0.0.0:5432->5432/tcp"},
			{ID: "ccc", Name: "laradock-nginx-1", Service: "nginx", Image: "nginx", Status: "running", State: "Up 2 days"},
		},
	}, {ID: "0000000000000001", Name: "sem-git", Path: "/srv/sem-git", Status: core.StatusStopped}})
	store.SetHostMetrics(core.HostMetrics{CPUPercent: 33.6, MemoryPercent: 71.2, DiskPercent: 62, Uptime: 10 * 24 * time.Hour})
	a := newAgent(nil, store, testConfig("http://hub"), "tui")
	a.now = func() time.Time { return fixedNow }

	stub(t, &hasWorkflows, func(string) bool { return true })
	stub(t, &ghaAvailable, func() bool { return true })
	stub(t, &ghaAuthOK, func(string) bool { return true })
	stub(t, &ghaListRuns, func(string, int) ([]collectors.GHARun, error) {
		return []collectors.GHARun{{ID: "991", Workflow: "tests", Branch: "DES-2900", Status: "completed",
			Conclusion: "failure", UpdatedAt: "2026-10-21T21:02:03Z", URL: "https://github.com/x/y/actions/runs/991"}}, nil
	})
	stub(t, &dockerExitState, func(id string) (int, bool, error) { return 137, true, nil })
	return a
}

// run passa o comando pelo mesmo caminho do poll e devolve o data em JSON.
func run(t *testing.T, a *Agent, typ, args string) (string, error) {
	t.Helper()
	data, err := a.dispatch(context.Background(), Command{ID: "c_1", Type: typ, Args: json.RawMessage(args)})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), nil
}

func mustRun(t *testing.T, a *Agent, typ, args string) string {
	t.Helper()
	out, err := run(t, a, typ, args)
	if err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	return out
}

func expectJSON(t *testing.T, what, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("%s (want): %v", what, err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Fatalf("%s\n got %s\nwant %s", what, gb, wb)
	}
}

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *cmdError
	if !errors.As(err, &e) || e.code != code {
		t.Fatalf("esperava erro %q, veio %v", code, err)
	}
}

func decodeFile(t *testing.T, out string) (string, string) {
	t.Helper()
	var v struct{ File File }
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.File.Name == "" {
		t.Fatalf("não é arquivo: %s", out)
	}
	b, err := base64.StdEncoding.DecodeString(v.File.ContentB64)
	if err != nil {
		t.Fatal(err)
	}
	return v.File.Name, string(b)
}

const summaryJSON = `{"id":"3f9a1c0b7d2e4f61","name":"digiliza","path":"/srv/digiliza","state":"down",
	"containers_up":2,"containers_total":3,"down":["php-fpm"],"branch":"DES-2900","ahead":2,"behind":0,
	"changes":4,"upstream":true,"ci":{"conclusion":"failure","workflow":"tests","branch":"DES-2900","ago_s":720},
	"ports":[80,443,5432],"port_conflicts":{"5432":["crm"]}}`

func TestHandleProjectsAndHost(t *testing.T) {
	a := fixtureAgent(t)
	expectJSON(t, "projects", mustRun(t, a, "projects", `{}`), `{"projects":[`+summaryJSON+`,
		{"id":"0000000000000001","name":"sem-git","path":"/srv/sem-git","state":"stopped","containers_up":0,
		"containers_total":0,"down":[],"branch":"","ahead":0,"behind":0,"changes":0,"upstream":false,
		"ci":{"conclusion":"failure","workflow":"tests","branch":"DES-2900","ago_s":720},"ports":[],"port_conflicts":{}}]}`)
	expectJSON(t, "host", mustRun(t, a, "host", ``), `{"name":"vps-digiliza","cpu":34,"ram":71,"disk":62,"uptime_s":864000}`)
}

func TestCICacheAndAvailability(t *testing.T) {
	a := fixtureAgent(t)
	calls := 0
	stub(t, &ghaListRuns, func(string, int) ([]collectors.GHARun, error) { calls++; return nil, nil })
	p, _ := a.project(pid)
	a.projectCI(p)
	a.projectCI(p)
	if calls != 1 {
		t.Fatalf("cache de 60 s: %d chamadas ao gh", calls)
	}
	a.now = func() time.Time { return fixedNow.Add(61 * time.Second) }
	if a.projectCI(p); calls != 2 {
		t.Fatal("o cache não venceu")
	}
	// Run rodando vai com conclusion vazia: é o que o hub desenha como ◐.
	stub(t, &ghaListRuns, func(string, int) ([]collectors.GHARun, error) {
		return []collectors.GHARun{{Workflow: "tests", Status: "in_progress"}}, nil
	})
	a.now = func() time.Time { return fixedNow.Add(130 * time.Second) }
	if ci := a.projectCI(p); ci == nil || ci.Conclusion != "" {
		t.Fatalf("run em andamento: %+v", ci)
	}
	stub(t, &ghaAuthOK, func(string) bool { return false })
	a.now = func() time.Time { return fixedNow.Add(200 * time.Second) }
	if ci := a.projectCI(p); ci != nil || calls != 2 {
		t.Fatalf("sem gh autenticado o CI é null: %+v", ci)
	}
}

func TestHandleProjectAndContainer(t *testing.T) {
	a := fixtureAgent(t)
	out := mustRun(t, a, "project", `{"project_id":"`+pid+`"}`)
	want := strings.TrimSuffix(summaryJSON, "}") + `,"containers":[
		{"name":"php-fpm","state":"exited","health":"","status":"Exited (137) 2 minutes ago","image":"laradock-php-fpm","cpu":0,"mem_mb":0,"ports":"","restart":"unless-stopped","is_data":false,"exit_code":137},
		{"name":"postgres","state":"running","health":"healthy","status":"Up 2 days (healthy)","image":"postgres:16","cpu":1.2,"mem_mb":84,"ports":"0.0.0.0:5432->5432/tcp","restart":"","is_data":true},
		{"name":"nginx","state":"running","health":"","status":"Up 2 days","image":"nginx","cpu":0,"mem_mb":0,"ports":"","restart":"","is_data":false}]}`
	expectJSON(t, "project", out, want)

	expectJSON(t, "container parado", mustRun(t, a, "container", `{"project_id":"`+pid+`","container":"php-fpm"}`),
		`{"name":"php-fpm","state":"exited","health":"","status":"Exited (137) 2 minutes ago","image":"laradock-php-fpm",
		"cpu":0,"mem_mb":0,"ports":"","restart":"unless-stopped","is_data":false,"exit_code":137,"oom":true}`)
	// Pelo nome do docker também acha; rodando, sem código de saída.
	out = mustRun(t, a, "container", `{"project_id":"`+pid+`","container":"laradock-nginx-1"}`)
	if strings.Contains(out, "exit_code") {
		t.Fatalf("container rodando não leva exit_code: %s", out)
	}

	_, err := run(t, a, "container", `{"project_id":"`+pid+`","container":"redis"}`)
	expectCode(t, err, "not_found")
	_, err = run(t, a, "project", `{"project_id":"ffffffffffffffff"}`)
	expectCode(t, err, "not_found")
}

func TestHandleLogs(t *testing.T) {
	a := fixtureAgent(t)
	logs := "[10:00:01] iniciando php-fpm\n" +
		"[10:00:02] DB_PASSWORD=hunter22 carregado\n" +
		"[10:00:03] production.ERROR: SQLSTATE[08006] pgsql:host=db;password=hunter22\n" +
		"[10:00:04] WARNING: pool www seems busy\n"
	var gotID string
	var gotTail int
	stub(t, &dockerLogs, func(id string, tail int) (string, error) { gotID, gotTail = id, tail; return logs, nil })
	stub(t, &dockerLogsSince, func(id string, since, tail int) (string, error) {
		if since != 300 {
			t.Fatalf("since = %d", since)
		}
		return "[10:05:00] depois\n", nil
	})

	out := mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","tail":5000}`)
	if gotID != "aaa" || gotTail != 2000 {
		t.Fatalf("docker logs com id %q e tail %d", gotID, gotTail)
	}
	expectJSON(t, "logs", out, `{"lines":["[10:00:01] iniciando php-fpm","[10:00:02] DB_PASSWORD=hu•••••• carregado",
		"[10:00:03] production.ERROR: SQLSTATE[08006] pgsql:host=db;password=hu••••••","[10:00:04] WARNING: pool www seems busy"]}`)
	if strings.Contains(out, "hunter22") {
		t.Fatal("senha vazou no log")
	}

	expectJSON(t, "errors_only", mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","errors_only":true}`),
		`{"lines":["[10:00:03] production.ERROR: SQLSTATE[08006] pgsql:host=db;password=hu••••••","[10:00:04] WARNING: pool www seems busy"]}`)
	expectJSON(t, "filter", mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","filter":"POOL"}`),
		`{"lines":["[10:00:04] WARNING: pool www seems busy"]}`)
	// O filtro roda no texto mascarado: não dá para adivinhar a senha por ele.
	expectJSON(t, "filter no segredo", mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","filter":"hunter"}`), `{"lines":[]}`)
	expectJSON(t, "since_s", mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","since_s":300}`), `{"lines":["[10:05:00] depois"]}`)

	name, content := decodeFile(t, mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm","as_file":true}`))
	if name != "php-fpm-2114.log" || !strings.HasPrefix(content, "[10:00:01] iniciando") || strings.Contains(content, "hunter22") {
		t.Fatalf("arquivo %q: %q", name, content)
	}

	// Mais de 1 MB vira arquivo cortado pelo começo.
	big := strings.Repeat("linha de log comum com algum texto\n", 40000)
	stub(t, &dockerLogs, func(string, int) (string, error) { return big + "ÚLTIMA", nil })
	name, content = decodeFile(t, mustRun(t, a, "logs", `{"project_id":"`+pid+`","container":"php-fpm"}`))
	if name != "php-fpm-2114-cortado.log" || len(content) > maxFileBytes || !strings.HasSuffix(content, "ÚLTIMA") ||
		!strings.HasPrefix(content, "linha de log") {
		t.Fatalf("arquivo grande: %q (%d bytes)", name, len(content))
	}
}

func TestHandleCI(t *testing.T) {
	a := fixtureAgent(t)
	expectJSON(t, "ci_runs", mustRun(t, a, "ci_runs", `{"project_id":"`+pid+`","limit":3}`),
		`{"runs":[{"id":"991","workflow":"tests","branch":"DES-2900","status":"completed","conclusion":"failure",
		"ago_s":720,"url":"https://github.com/x/y/actions/runs/991"}]}`)

	stub(t, &ghaFailedLogs, func(path, id string) (string, error) {
		return "tests\tRun\tSQLSTATE[28P01] password authentication failed\nAuthorization: Bearer ghp_abcdefghijklmnopqrstuvwxyz\n", nil
	})
	name, content := decodeFile(t, mustRun(t, a, "ci_failed_log", `{"project_id":"`+pid+`","run_id":991}`))
	if name != "ci-991.log" || strings.Contains(content, "ghp_abcdef") {
		t.Fatalf("ci_failed_log %q: %q", name, content)
	}
	_, err := run(t, a, "ci_failed_log", `{"project_id":"`+pid+`","run_id":"--repo=x"}`)
	expectCode(t, err, "bad_request")

	stub(t, &ghaAvailable, func() bool { return false })
	_, err = run(t, a, "ci_runs", `{"project_id":"`+pid+`"}`)
	expectCode(t, err, "gh_unavailable")
}

// fakeGit responde os GitExec que os handlers fazem.
func fakeGit(t *testing.T) {
	stub(t, &gitExec, func(path string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --abbrev-ref @{u}":
			return "origin/DES-2900", nil
		case "show -s --date=format:%Y-%m-%d %H:%M --format=%h%x09%an%x09%cd a1b2c3d":
			return "a1b2c3d\tFulano\t2026-10-21 19:14", nil
		case "show --format= a1b2c3d":
			return "diff --git a/.env b/.env\n+DB_PASSWORD=hunter22", nil
		case "for-each-ref --format=%(refname:short)%09%(upstream:track,nobracket) refs/heads/":
			return "DES-2900\tahead 2, behind 1\nmain\t\nvelha\tgone", nil
		case "log --format=%h%x09%s%x09%an%x09%ct HEAD..@{u} -10":
			return "e5f6a7b\tcorrige frete\tCiclana\t1792609443", nil
		}
		return "", errors.New("fatal: comando inesperado " + strings.Join(args, " "))
	})
	stub(t, &gitBranch, func(string) string { return "DES-2900" })
	stub(t, &gitCollect, func(string) *core.GitInfo {
		return &core.GitInfo{IsRepo: true, Branch: "DES-2900", Ahead: 2, Behind: 1, Modified: 3, Staged: 1, Untracked: 1, StashCount: 2,
			Files:    []core.GitFileStatus{{Staging: "M", Worktree: " ", Path: "app/Frete.php"}, {Staging: "?", Worktree: "?", Path: "novo.txt"}},
			Branches: []core.GitBranch{{Name: "DES-2900", Current: true}, {Name: "main"}},
			Commits: []core.GitCommit{
				{Hash: "a1b2c3d", Message: "ajusta cálculo de frete", Author: "Fulano", Unix: fixedNow.Unix() - 7200},
				{Hash: "b2", Message: "2"}, {Hash: "b3", Message: "3"}, {Hash: "b4", Message: "4"}, {Hash: "b5", Message: "5"}, {Hash: "b6", Message: "6"},
			}}
	})
}

func TestHandleGit(t *testing.T) {
	a := fixtureAgent(t)
	fakeGit(t)
	args := `{"project_id":"` + pid + `"}`

	out := mustRun(t, a, "git_summary", args)
	if !strings.HasPrefix(out, `{"branch":"DES-2900","upstream":"origin/DES-2900","ahead":2,"behind":1,"modified":3,"staged":1,"untracked":1,"stash":2,"commits":[{"hash":"a1b2c3d","subject":"ajusta cálculo de frete","author":"Fulano","ago_s":7200}`) ||
		strings.Count(out, `"hash"`) != 5 {
		t.Fatalf("git_summary: %s", out)
	}

	stub(t, &gitCommits, func(path, branch string, limit int) []core.GitCommit {
		if branch != "HEAD" || limit != 20 {
			t.Fatalf("CollectCommitsAt(%q, %d)", branch, limit)
		}
		return []core.GitCommit{{Hash: "a1b2c3d", Message: "usa token ghp_abcdefghijklmnopqrstuvwxyz", Author: "Fulano", Unix: fixedNow.Unix() - 60}}
	})
	expectJSON(t, "git_commits", mustRun(t, a, "git_commits", `{"project_id":"`+pid+`","limit":50}`),
		`{"commits":[{"hash":"a1b2c3d","subject":"usa token ghp_••••••••","author":"Fulano","ago_s":60}]}`)

	stub(t, &gitCommitMessage, func(string, string) string { return "ajusta frete\n\nDB_PASSWORD=hunter22 no teste" })
	stub(t, &gitCommitFiles, func(string, string) []core.GitCommitFileChange {
		return []core.GitCommitFileChange{{Status: "M", Path: "app/Frete.php"}}
	})
	expectJSON(t, "git_commit", mustRun(t, a, "git_commit", `{"project_id":"`+pid+`","hash":"a1b2c3d"}`),
		`{"hash":"a1b2c3d","author":"Fulano","date":"2026-10-21 19:14","message":"ajusta frete\n\nDB_PASSWORD=hu•••••• no teste",
		"files":[{"status":"M","path":"app/Frete.php"}]}`)
	_, err := run(t, a, "git_commit", `{"project_id":"`+pid+`","hash":"--output=/tmp/x"}`)
	expectCode(t, err, "bad_request")

	expectJSON(t, "git_changed", mustRun(t, a, "git_changed", args),
		`{"files":[{"status":"M","path":"app/Frete.php"},{"status":"??","path":"novo.txt"}]}`)

	stub(t, &gitWorkingDiff, func(repo, file string) string { return "diff de " + file + "\n+APP_KEY=base64:abcdef" })
	name, content := decodeFile(t, mustRun(t, a, "git_diff", args))
	if name != "working-tree.diff" || content != "diff de app/Frete.php\n+APP_KEY=ba••••••••••\ndiff de novo.txt\n+APP_KEY=ba••••••••••" {
		t.Fatalf("git_diff da working tree %q: %q", name, content)
	}
	name, content = decodeFile(t, mustRun(t, a, "git_diff", `{"project_id":"`+pid+`","hash":"a1b2c3d"}`))
	if name != "a1b2c3d.diff" || strings.Contains(content, "hunter22") {
		t.Fatalf("git_diff do commit %q: %q", name, content)
	}

	expectJSON(t, "git_branches", mustRun(t, a, "git_branches", args), `{"branches":[
		{"name":"DES-2900","current":true,"remote":false,"ahead":2,"behind":1},
		{"name":"main","current":false,"remote":false,"ahead":0,"behind":0}]}`)

	stub(t, &gitWorktrees, func(string) ([]collectors.GitWorktree, error) {
		return []collectors.GitWorktree{{Path: "/srv/digiliza", Branch: "DES-2900", Changes: 4}, {Path: "/srv/digiliza-hotfix", Branch: "hotfix"}}, nil
	})
	expectJSON(t, "git_worktrees", mustRun(t, a, "git_worktrees", args), `{"worktrees":[
		{"path":"/srv/digiliza","branch":"DES-2900","changes":4},{"path":"/srv/digiliza-hotfix","branch":"hotfix","changes":0}]}`)

	expectJSON(t, "git_incoming", mustRun(t, a, "git_incoming", args),
		`{"commits":[{"hash":"e5f6a7b","subject":"corrige frete","author":"Ciclana","ago_s":`+
			itoa(fixedNow.Unix()-1792609443)+`}]}`)

	expectJSON(t, "git_pending", mustRun(t, a, "git_pending", ``), `{"projects":[
		{"project_id":"3f9a1c0b7d2e4f61","name":"digiliza","branch":"DES-2900","changes":4,"ahead":2,"behind":0,"upstream":true}]}`)

	_, err = run(t, a, "git_summary", `{"project_id":"0000000000000001"}`)
	expectCode(t, err, "not_git")
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestProjectState(t *testing.T) {
	c := func(status, state, health string) core.Container {
		return core.Container{ID: status + state, Status: status, State: state, Health: health}
	}
	cases := []struct {
		name       string
		containers []core.Container
		status     core.ProjectStatus
		want       string
	}{
		{"tudo no ar", []core.Container{c("running", "Up", ""), c("running", "Up", "healthy")}, "", "ok"},
		{"um parado de propósito", []core.Container{c("running", "Up", ""), c("exited", "Exited (0) 1 hour ago", "")}, "", "partial"},
		{"docker stop com SIGTERM", []core.Container{c("exited", "Exited (143) 1 hour ago", "")}, "", "stopped"},
		{"caiu", []core.Container{c("running", "Up", ""), c("exited", "Exited (1) 1 min ago", "")}, "", "down"},
		{"reiniciando", []core.Container{c("restarting", "Restarting (1)", "")}, "", "down"},
		{"unhealthy", []core.Container{c("running", "Up (unhealthy)", "unhealthy")}, "", "down"},
		{"sem containers, rodando", nil, core.StatusRunning, "ok"},
		{"sem containers, parado", nil, core.StatusStopped, "stopped"},
	}
	for _, tc := range cases {
		if got, _, _, _ := projectState(core.Project{Containers: tc.containers, Status: tc.status}); got != tc.want {
			t.Errorf("%s: %s, queria %s", tc.name, got, tc.want)
		}
	}
}
