package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ghaCatalogRel   = ".devscope/actions.yaml"
	ghaNotesRel     = ".devscope/actions-notes.yaml"
	ghaWorkflowsRel = ".github/workflows"
)

// GHACatalog is the project-central registry of managed processes.
type GHACatalog struct {
	Version   int          `yaml:"version"`
	Processes []GHAProcess `yaml:"processes"`
}

// GHAProcess is one pipeline managed by DevScope (one workflow file).
type GHAProcess struct {
	Name        string `yaml:"name"`
	File        string `yaml:"file"`
	Description string `yaml:"description,omitempty"`
	Template    string `yaml:"template,omitempty"` // ci | deploy | manual
}

type GHAWorkflow struct {
	ID    string
	Name  string
	State string
	Path  string
}

type GHARun struct {
	ID           string
	Name         string
	DisplayTitle string
	Status       string
	Conclusion   string
	Workflow     string
	Branch       string
	Event        string
	CreatedAt    string
	StartedAt    string
	UpdatedAt    string
	URL          string
}

// GHAJob is one job inside a workflow run (matrix row / named job).
type GHAJob struct {
	ID          string
	Name        string
	Status      string
	Conclusion  string
	StartedAt   string
	CompletedAt string
	Steps       []GHAStep
}

// GHAStep is a step inside a job.
type GHAStep struct {
	Name        string
	Status      string
	Conclusion  string
	Number      int
	StartedAt   string
	CompletedAt string
}

// GHAWorkflowInput is a workflow_dispatch input from the YAML.
type GHAWorkflowInput struct {
	Name        string
	Description string
	Default     string
	Required    bool
	Type        string   // string | boolean | choice | environment
	Options     []string // for choice
}

// GHANote marks a run with a short annotation (incidente, etc).
type GHANote struct {
	RunID   string `yaml:"run_id"`
	Note    string `yaml:"note"`
	At      string `yaml:"at,omitempty"`
	Process string `yaml:"process,omitempty"`
}

type GHANotesFile struct {
	Version int       `yaml:"version"`
	Notes   []GHANote `yaml:"notes"`
}

// GHAFailBucket is fails-per-process for the heatmap card.
type GHAFailBucket struct {
	Process string
	Fails   int
}

// GHAMinuteBucket is estimated billable minutes per workflow.
type GHAMinuteBucket struct {
	Workflow string
	Minutes  float64
}

// GHAActionsBilling is account/org Actions quota from the GitHub billing API.
type GHAActionsBilling struct {
	Included  float64
	Used      float64
	PaidUsed  float64
	Remaining float64
	DaysLeft  int
	Source    string // org | user | ""
	OK        bool
	Error     string
}

type GHAInfo struct {
	Available bool
	Authed    bool
	Owner     string
	Repo      string
	Error     string
}

func GHAAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// GHAInstallHint returns short install instructions for the current platform.
func GHAInstallHint() string {
	return "sudo apt install gh\n# ou: sudo snap install gh\n# docs: https://cli.github.com/"
}

// GHAAuthLoginCmd starts interactive GitHub CLI login (browser + device flow).
func GHAAuthLoginCmd() *exec.Cmd {
	// web login is the least painful default for TUI users
	return exec.Command("gh", "auth", "login", "-h", "github.com", "-p", "https", "-w")
}

func GHAAuthStatusOK(projectPath string) bool {
	if !GHAAvailable() {
		return false
	}
	_, err := runGH(projectPath, 8*time.Second, "auth", "status")
	return err == nil
}

func runGH(dir string, timeout time.Duration, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
		s := strings.TrimSpace(string(out))
		if err != nil {
			if s != "" {
				return s, fmt.Errorf("%s", firstLine(s))
			}
			return s, err
		}
		return s, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("gh timeout")
	}
}

func GHARepoInfo(projectPath, remote string) GHAInfo {
	info := GHAInfo{Available: GHAAvailable()}
	owner, repo, ok := ParseGitHubRepo(remote)
	if !ok && projectPath != "" && info.Available {
		out, err := runGH(projectPath, 8*time.Second, "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
		if err == nil && strings.Contains(out, "/") {
			parts := strings.SplitN(out, "/", 2)
			owner, repo, ok = parts[0], parts[1], true
		}
	}
	if ok {
		info.Owner, info.Repo = owner, repo
	}
	if !info.Available {
		if !ok {
			info.Error = "gh não encontrado · remote GitHub não detectado"
		} else {
			info.Error = "gh não encontrado (runs/trigger precisam do CLI)"
		}
		return info
	}
	if !ok {
		info.Error = "remote GitHub não detectado"
		return info
	}
	if _, err := runGH(projectPath, 8*time.Second, "auth", "status"); err != nil {
		info.Error = "gh não autenticado — rode: gh auth login"
		return info
	}
	info.Authed = true
	return info
}

// GHAActionsURL builds a browser URL for the Actions area or a workflow file.
func GHAActionsURL(owner, repo, workflowFile string) string {
	if owner == "" || repo == "" {
		return ""
	}
	base := fmt.Sprintf("https://github.com/%s/%s/actions", owner, repo)
	if workflowFile == "" {
		return base
	}
	name := filepath.Base(workflowFile)
	return base + "/workflows/" + name
}

// GHARunURL builds the browser URL for a specific workflow run.
func GHARunURL(owner, repo, runID string) string {
	if owner == "" || repo == "" || runID == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s", owner, repo, runID)
}

func GHACatalogPath(projectPath string) string {
	return filepath.Join(projectPath, ghaCatalogRel)
}

func GHAWorkflowsDir(projectPath string) string {
	return filepath.Join(projectPath, ghaWorkflowsRel)
}

func LoadGHACatalog(projectPath string) (GHACatalog, error) {
	path := GHACatalogPath(projectPath)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GHACatalog{Version: 1}, nil
		}
		return GHACatalog{}, err
	}
	var c GHACatalog
	if err := yaml.Unmarshal(b, &c); err != nil {
		return GHACatalog{}, err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	return c, nil
}

func SaveGHACatalog(projectPath string, c GHACatalog) error {
	if c.Version == 0 {
		c.Version = 1
	}
	path := GHACatalogPath(projectPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// SyncGHACatalogFromDisk discovers workflow files and merges into the catalog.
func SyncGHACatalogFromDisk(projectPath string) (GHACatalog, error) {
	c, err := LoadGHACatalog(projectPath)
	if err != nil {
		return c, err
	}
	byName := map[string]int{}
	for i, p := range c.Processes {
		byName[p.Name] = i
	}
	dir := GHAWorkflowsDir(projectPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasSuffix(low, ".yml") && !strings.HasSuffix(low, ".yaml") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		rel := filepath.ToSlash(filepath.Join(ghaWorkflowsRel, name))
		if _, ok := byName[base]; ok {
			continue
		}
		c.Processes = append(c.Processes, GHAProcess{
			Name:        base,
			File:        rel,
			Description: "discovered",
		})
		byName[base] = len(c.Processes) - 1
	}
	_ = SaveGHACatalog(projectPath, c)
	return c, nil
}

func GHAListLocalWorkflowFiles(projectPath string) ([]GHAProcess, error) {
	c, err := SyncGHACatalogFromDisk(projectPath)
	if err != nil {
		return nil, err
	}
	return c.Processes, nil
}

func sanitizeGHAProcessName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "process"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// GHAWorkflowTemplate returns YAML body for a new process file.
func GHAWorkflowTemplate(name, template string) string {
	name = sanitizeGHAProcessName(name)
	title := strings.ReplaceAll(name, "-", " ")
	switch strings.ToLower(template) {
	case "deploy":
		return fmt.Sprintf(`name: %s

on:
  workflow_dispatch:
  push:
    tags: ["v*"]

permissions:
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Deploy
        run: echo "Deploy %s — customize this job"
`, title, name)
	case "manual":
		return fmt.Sprintf(`name: %s

on:
  workflow_dispatch:

permissions:
  contents: read

jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run
        run: echo "Manual process %s"
`, title, name)
	default: // ci
		return fmt.Sprintf(`name: %s

on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
  workflow_dispatch:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Test
        run: echo "CI process %s — add your steps"
`, title, name)
	}
}

func GHACreateProcess(projectPath, name, description, template string) (GHAProcess, error) {
	name = sanitizeGHAProcessName(name)
	if name == "" {
		return GHAProcess{}, fmt.Errorf("nome inválido")
	}
	rel := filepath.ToSlash(filepath.Join(ghaWorkflowsRel, name+".yml"))
	abs := filepath.Join(projectPath, rel)
	if _, err := os.Stat(abs); err == nil {
		return GHAProcess{}, fmt.Errorf("já existe: %s", rel)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return GHAProcess{}, err
	}
	body := GHAWorkflowTemplate(name, template)
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		return GHAProcess{}, err
	}
	c, err := LoadGHACatalog(projectPath)
	if err != nil {
		return GHAProcess{}, err
	}
	proc := GHAProcess{
		Name:        name,
		File:        rel,
		Description: description,
		Template:    template,
	}
	if proc.Description == "" {
		proc.Description = template
	}
	// replace if same name
	found := false
	for i, p := range c.Processes {
		if p.Name == name {
			c.Processes[i] = proc
			found = true
			break
		}
	}
	if !found {
		c.Processes = append(c.Processes, proc)
	}
	if err := SaveGHACatalog(projectPath, c); err != nil {
		return proc, err
	}
	return proc, nil
}

func GHADeleteProcess(projectPath, name string) error {
	name = sanitizeGHAProcessName(name)
	c, err := LoadGHACatalog(projectPath)
	if err != nil {
		return err
	}
	var kept []GHAProcess
	var file string
	for _, p := range c.Processes {
		if p.Name == name {
			file = p.File
			continue
		}
		kept = append(kept, p)
	}
	if file == "" {
		file = filepath.ToSlash(filepath.Join(ghaWorkflowsRel, name+".yml"))
	}
	abs := filepath.Join(projectPath, file)
	_ = os.Remove(abs)
	// also try .yaml
	_ = os.Remove(strings.TrimSuffix(abs, ".yml") + ".yaml")
	c.Processes = kept
	return SaveGHACatalog(projectPath, c)
}

func GHAReadProcessFile(projectPath, relOrName string) (string, error) {
	rel := relOrName
	if !strings.Contains(rel, "/") {
		rel = filepath.ToSlash(filepath.Join(ghaWorkflowsRel, sanitizeGHAProcessName(rel)+".yml"))
	}
	b, err := os.ReadFile(filepath.Join(projectPath, rel))
	if err != nil {
		alt := strings.TrimSuffix(rel, ".yml") + ".yaml"
		b, err = os.ReadFile(filepath.Join(projectPath, alt))
		if err != nil {
			return "", err
		}
	}
	return string(b), nil
}

func GHAListWorkflows(projectPath string) ([]GHAWorkflow, error) {
	out, err := runGH(projectPath, 20*time.Second, "workflow", "list", "--all",
		"--json", "id,name,state,path")
	if err != nil {
		return nil, err
	}
	return parseGHAWorkflowJSON(out)
}

func GHAListRuns(projectPath string, limit int) ([]GHARun, error) {
	if limit <= 0 {
		limit = 20
	}
	out, err := runGH(projectPath, 25*time.Second, "run", "list",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "databaseId,name,displayTitle,status,conclusion,workflowName,headBranch,event,createdAt,startedAt,updatedAt,url")
	if err != nil {
		return nil, err
	}
	return parseGHARunJSON(out)
}

func GHARunLogs(projectPath, runID string) (string, error) {
	if runID == "" {
		return "", fmt.Errorf("run id vazio")
	}
	// --log can be huge; prefer view summary then attempt log
	out, err := runGH(projectPath, 45*time.Second, "run", "view", runID, "--log")
	if err != nil {
		sum, err2 := runGH(projectPath, 20*time.Second, "run", "view", runID)
		if err2 != nil {
			return "", err
		}
		return sum, err
	}
	if len(out) > 80_000 {
		out = out[len(out)-80_000:]
		out = "...(truncated)\n" + out
	}
	return out, nil
}

// GHARunFailedLogs returns only failed step logs for a run.
func GHARunFailedLogs(projectPath, runID string) (string, error) {
	if runID == "" {
		return "", fmt.Errorf("run id vazio")
	}
	out, err := runGH(projectPath, 45*time.Second, "run", "view", runID, "--log-failed")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "nenhum step falhou (ou logs ainda indisponíveis)", nil
	}
	if len(out) > 80_000 {
		out = out[len(out)-80_000:]
		out = "...(truncated)\n" + out
	}
	return out, nil
}

// GHAListRunJobs returns jobs (+ steps) for a workflow run.
func GHAListRunJobs(projectPath, runID string) ([]GHAJob, error) {
	if runID == "" {
		return nil, fmt.Errorf("run id vazio")
	}
	out, err := runGH(projectPath, 25*time.Second, "run", "view", runID, "--json", "jobs")
	if err != nil {
		return nil, err
	}
	return parseGHAJobsJSON(out)
}

// FormatGHAJobsText renders jobs/steps for the TUI Jobs tab.
func FormatGHAJobsText(jobs []GHAJob) string {
	return FormatGHATimelineText(jobs)
}

// FormatGHATimelineText renders queued→jobs→steps with a simple duration bar.
func FormatGHATimelineText(jobs []GHAJob) string {
	if len(jobs) == 0 {
		return "nenhum job neste run"
	}
	maxSec := 1.0
	for _, j := range jobs {
		if s := ghaDurationSec(j.StartedAt, j.CompletedAt); s > maxSec {
			maxSec = s
		}
		for _, st := range j.Steps {
			if s := ghaDurationSec(st.StartedAt, st.CompletedAt); s > maxSec {
				maxSec = s
			}
		}
	}
	var b strings.Builder
	b.WriteString("TIMELINE  queued → jobs → steps\n")
	for i, j := range jobs {
		if i > 0 {
			b.WriteByte('\n')
		}
		mark := ghaStatusMark(j.Status, j.Conclusion)
		sec := ghaDurationSec(j.StartedAt, j.CompletedAt)
		b.WriteString(fmt.Sprintf("%s %-22s %s %s\n",
			mark, truncateRunes(j.Name, 22), ghaBar(sec, maxSec, 12), ghaFmtDur(sec)))
		if len(j.Steps) == 0 {
			b.WriteString("  · (sem steps)\n")
			continue
		}
		for _, s := range j.Steps {
			sm := ghaStatusMark(s.Status, s.Conclusion)
			name := s.Name
			if name == "" {
				name = fmt.Sprintf("step %d", s.Number)
			}
			ss := ghaDurationSec(s.StartedAt, s.CompletedAt)
			b.WriteString(fmt.Sprintf("  %s %-20s %s %s\n",
				sm, truncateRunes(name, 20), ghaBar(ss, maxSec, 10), ghaFmtDur(ss)))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func ghaDurationSec(start, end string) float64 {
	t0, ok0 := parseGHATime(start)
	if !ok0 {
		return 0
	}
	t1, ok1 := parseGHATime(end)
	if !ok1 {
		t1 = time.Now().UTC()
	}
	if t1.Before(t0) {
		return 0
	}
	return t1.Sub(t0).Seconds()
}

func parseGHATime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func ghaFmtDur(sec float64) string {
	if sec <= 0 {
		return "—"
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", int(sec))
	}
	m := int(sec) / 60
	s := int(sec) % 60
	if m >= 60 {
		return fmt.Sprintf("%dh%dm", m/60, m%60)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

func ghaBar(sec, maxSec float64, width int) string {
	if width < 4 {
		width = 4
	}
	if maxSec <= 0 {
		maxSec = 1
	}
	filled := 0
	if sec > 0 {
		filled = int(sec/maxSec*float64(width) + 0.5)
		if filled < 1 {
			filled = 1
		}
		if filled > width {
			filled = width
		}
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// GHAFailHeatmap counts failures per workflow in the first n runs (newest-first).
func GHAFailHeatmap(runs []GHARun, n int) []GHAFailBucket {
	if n <= 0 || n > len(runs) {
		n = len(runs)
	}
	counts := map[string]int{}
	order := []string{}
	for i := 0; i < n; i++ {
		r := runs[i]
		if r.Conclusion != "failure" && r.Conclusion != "timed_out" && r.Conclusion != "startup_failure" {
			continue
		}
		name := firstNonEmpty(r.Workflow, r.Name, "?")
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name]++
	}
	out := make([]GHAFailBucket, 0, len(order))
	for _, name := range order {
		out = append(out, GHAFailBucket{Process: name, Fails: counts[name]})
	}
	return out
}

// GHABillingEstimate estimates minutes from startedAt→updatedAt on completed runs.
func GHABillingEstimate(runs []GHARun, n int) []GHAMinuteBucket {
	if n <= 0 || n > len(runs) {
		n = len(runs)
	}
	mins := map[string]float64{}
	order := []string{}
	for i := 0; i < n; i++ {
		r := runs[i]
		sec := ghaDurationSec(firstNonEmpty(r.StartedAt, r.CreatedAt), firstNonEmpty(r.UpdatedAt, r.CreatedAt))
		if sec <= 0 {
			continue
		}
		name := firstNonEmpty(r.Workflow, r.Name, "?")
		if _, ok := mins[name]; !ok {
			order = append(order, name)
		}
		mins[name] += sec / 60.0
	}
	out := make([]GHAMinuteBucket, 0, len(order))
	for _, name := range order {
		out = append(out, GHAMinuteBucket{Workflow: name, Minutes: mins[name]})
	}
	return out
}

func FormatGHAFailHeatmap(buckets []GHAFailBucket, width int) string {
	if len(buckets) == 0 {
		return "0 fails"
	}
	max := 1
	for _, b := range buckets {
		if b.Fails > max {
			max = b.Fails
		}
	}
	parts := make([]string, 0, len(buckets))
	for _, b := range buckets {
		barW := 6
		filled := b.Fails * barW / max
		if filled < 1 {
			filled = 1
		}
		parts = append(parts, fmt.Sprintf("%s %s%d",
			truncateRunes(b.Process, maxIntGHA(6, width/len(buckets)-4)),
			strings.Repeat("▓", filled)+strings.Repeat("░", barW-filled),
			b.Fails))
	}
	return strings.Join(parts, "  ")
}

func FormatGHABilling(buckets []GHAMinuteBucket) string {
	if len(buckets) == 0 {
		return "0m"
	}
	total := SumGHAMinutes(buckets)
	top := ""
	topM := 0.0
	for _, b := range buckets {
		if b.Minutes > topM {
			topM = b.Minutes
			top = b.Workflow
		}
	}
	if top == "" {
		return FormatGHAMinutes(total)
	}
	return fmt.Sprintf("%s  %s %s", FormatGHAMinutes(total), truncateRunes(top, 10), FormatGHAMinutes(topM))
}

func SumGHAMinutes(buckets []GHAMinuteBucket) float64 {
	total := 0.0
	for _, b := range buckets {
		total += b.Minutes
	}
	return total
}

// GHAMinutesFromRuns sums estimated minutes across the given runs.
func GHAMinutesFromRuns(runs []GHARun) float64 {
	return SumGHAMinutes(GHABillingEstimate(runs, len(runs)))
}

func FormatGHAMinutes(m float64) string {
	if m <= 0 {
		return "0m"
	}
	if m < 10 {
		return fmt.Sprintf("~%.1fm", m)
	}
	return fmt.Sprintf("~%.0fm", m)
}

// GHAFetchActionsBilling loads included/used minutes for the owner (org then user).
func GHAFetchActionsBilling(projectPath, owner string) GHAActionsBilling {
	out := GHAActionsBilling{}
	owner = strings.TrimSpace(owner)
	if owner == "" || !GHAAvailable() {
		out.Error = "owner/gh indisponível"
		return out
	}
	paths := []struct {
		api    string
		source string
	}{
		{"orgs/" + owner + "/settings/billing/actions", "org"},
		{"users/" + owner + "/settings/billing/actions", "user"},
	}
	var lastErr string
	for _, p := range paths {
		raw, err := runGH(projectPath, 12*time.Second, "api", p.api)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		var payload struct {
			TotalMinutesUsed     json.Number `json:"total_minutes_used"`
			TotalPaidMinutesUsed json.Number `json:"total_paid_minutes_used"`
			IncludedMinutes      json.Number `json:"included_minutes"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			lastErr = err.Error()
			continue
		}
		out.Used = jsonNumberFloat(payload.TotalMinutesUsed)
		out.PaidUsed = jsonNumberFloat(payload.TotalPaidMinutesUsed)
		out.Included = jsonNumberFloat(payload.IncludedMinutes)
		if out.Included <= 0 {
			lastErr = "included_minutes vazio"
			continue
		}
		out.Remaining = out.Included - out.Used
		if out.Remaining < 0 {
			out.Remaining = 0
		}
		out.Source = p.source
		out.OK = true
		out.DaysLeft = ghaBillingDaysLeft(projectPath, owner, p.source)
		return out
	}
	out.Error = firstNonEmpty(lastErr, "billing indisponível")
	return out
}

func ghaBillingDaysLeft(projectPath, owner, source string) int {
	api := "users/" + owner + "/settings/billing/shared-storage"
	if source == "org" {
		api = "orgs/" + owner + "/settings/billing/shared-storage"
	}
	raw, err := runGH(projectPath, 8*time.Second, "api", api)
	if err != nil {
		return 0
	}
	var payload struct {
		DaysLeft json.Number `json:"days_left_in_billing_cycle"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return 0
	}
	n, _ := payload.DaysLeft.Int64()
	return int(n)
}

func jsonNumberFloat(n json.Number) float64 {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		i, err2 := n.Int64()
		if err2 != nil {
			return 0
		}
		return float64(i)
	}
	return f
}

func maxIntGHA(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ghaStatusMark(status, conclusion string) string {
	c := strings.ToLower(conclusion)
	st := strings.ToLower(status)
	switch {
	case c == "success":
		return "✓"
	case c == "failure", c == "timed_out", c == "startup_failure":
		return "✗"
	case c == "cancelled", c == "skipped":
		return "○"
	case st == "in_progress", st == "queued", st == "waiting", st == "pending":
		return "●"
	default:
		return "·"
	}
}

func GHATriggerWorkflow(projectPath, workflow string) (string, error) {
	return GHATriggerWorkflowRef(projectPath, workflow, "")
}

// GHATriggerWorkflowRef triggers workflow_dispatch on a branch/tag ref.
func GHATriggerWorkflowRef(projectPath, workflow, ref string) (string, error) {
	return GHATriggerWorkflowRefInputs(projectPath, workflow, ref, nil)
}

// GHATriggerWorkflowRefInputs triggers with optional -f key=value inputs.
func GHATriggerWorkflowRefInputs(projectPath, workflow, ref string, inputs map[string]string) (string, error) {
	if workflow == "" {
		return "", fmt.Errorf("workflow vazio")
	}
	args := []string{"workflow", "run", workflow}
	if ref != "" {
		args = append(args, "--ref", ref)
	}
	for k, v := range inputs {
		if strings.TrimSpace(k) == "" {
			continue
		}
		args = append(args, "-f", k+"="+v)
	}
	return runGH(projectPath, 30*time.Second, args...)
}

// GHAParseWorkflowInputs reads workflow_dispatch inputs from a local workflow file.
func GHAParseWorkflowInputs(projectPath, workflowFile string) ([]GHAWorkflowInput, error) {
	body, err := GHAReadProcessFile(projectPath, workflowFile)
	if err != nil {
		return nil, err
	}
	return parseWorkflowDispatchInputs(body)
}

func parseWorkflowDispatchInputs(yamlBody string) ([]GHAWorkflowInput, error) {
	var doc struct {
		On interface{} `yaml:"on"`
	}
	if err := yaml.Unmarshal([]byte(yamlBody), &doc); err != nil {
		return nil, err
	}
	// on: workflow_dispatch  OR  on: { workflow_dispatch: { inputs: ... } }
	m, ok := doc.On.(map[string]interface{})
	if !ok {
		// on: [push, workflow_dispatch]
		return nil, nil
	}
	wd, ok := m["workflow_dispatch"]
	if !ok || wd == nil {
		return nil, nil
	}
	wdMap, ok := wd.(map[string]interface{})
	if !ok {
		return nil, nil // bare workflow_dispatch: null / true
	}
	rawInputs, ok := wdMap["inputs"].(map[string]interface{})
	if !ok || len(rawInputs) == 0 {
		return nil, nil
	}
	out := make([]GHAWorkflowInput, 0, len(rawInputs))
	for name, raw := range rawInputs {
		in := GHAWorkflowInput{Name: name, Type: "string"}
		switch v := raw.(type) {
		case map[string]interface{}:
			if d, ok := v["description"].(string); ok {
				in.Description = d
			}
			if t, ok := v["type"].(string); ok {
				in.Type = t
			}
			if req, ok := v["required"].(bool); ok {
				in.Required = req
			}
			switch def := v["default"].(type) {
			case string:
				in.Default = def
			case bool:
				in.Default = strconv.FormatBool(def)
			case int:
				in.Default = strconv.Itoa(def)
			case float64:
				in.Default = strconv.FormatFloat(def, 'f', -1, 64)
			}
			if opts, ok := v["options"].([]interface{}); ok {
				for _, o := range opts {
					in.Options = append(in.Options, fmt.Sprint(o))
				}
			}
		case string:
			in.Description = v
		}
		out = append(out, in)
	}
	// stable order by name
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// GitBranchAheadCount returns how many local commits are not on origin/<branch>.
func GitBranchAheadCount(projectPath, branch string) int {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return 0
	}
	out := strings.TrimSpace(gitOutput(projectPath, "rev-list", "--count", "origin/"+branch+".."+branch))
	n, _ := strconv.Atoi(out)
	return n
}

// GitPushBranch pushes the branch to origin (sets upstream if needed).
func GitPushBranch(projectPath, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", fmt.Errorf("branch vazia")
	}
	return gitRunOutput(projectPath, "push", "-u", "origin", branch)
}

func GHANotesPath(projectPath string) string {
	return filepath.Join(projectPath, ghaNotesRel)
}

func LoadGHANotes(projectPath string) (GHANotesFile, error) {
	path := GHANotesPath(projectPath)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GHANotesFile{Version: 1}, nil
		}
		return GHANotesFile{}, err
	}
	var f GHANotesFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return GHANotesFile{}, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

func SaveGHANote(projectPath, runID, note, process string) error {
	if runID == "" || strings.TrimSpace(note) == "" {
		return fmt.Errorf("run/note vazios")
	}
	f, err := LoadGHANotes(projectPath)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	found := false
	for i := range f.Notes {
		if f.Notes[i].RunID == runID {
			f.Notes[i].Note = note
			f.Notes[i].At = now
			f.Notes[i].Process = process
			found = true
			break
		}
	}
	if !found {
		f.Notes = append(f.Notes, GHANote{RunID: runID, Note: note, At: now, Process: process})
	}
	if err := os.MkdirAll(filepath.Dir(GHANotesPath(projectPath)), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(GHANotesPath(projectPath), b, 0o644)
}

func GHANoteForRun(projectPath, runID string) string {
	f, err := LoadGHANotes(projectPath)
	if err != nil {
		return ""
	}
	for _, n := range f.Notes {
		if n.RunID == runID {
			return n.Note
		}
	}
	return ""
}

func DeleteGHANote(projectPath, runID string) error {
	f, err := LoadGHANotes(projectPath)
	if err != nil {
		return err
	}
	out := f.Notes[:0]
	for _, n := range f.Notes {
		if n.RunID != runID {
			out = append(out, n)
		}
	}
	f.Notes = out
	b, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(GHANotesPath(projectPath), b, 0o644)
}

func GHACancelRun(projectPath, runID string) (string, error) {
	return runGH(projectPath, 20*time.Second, "run", "cancel", runID)
}

func GHARerun(projectPath, runID string) (string, error) {
	return runGH(projectPath, 20*time.Second, "run", "rerun", runID)
}

func GHAOpenRunURL(url string) error {
	if url == "" {
		return fmt.Errorf("url vazia")
	}
	for _, bin := range []string{"xdg-open", "gio", "sensible-browser"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		args := []string{url}
		if bin == "gio" {
			args = []string{"open", url}
		}
		cmd := exec.Command(bin, args...)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("nenhum opener (xdg-open) disponível")
}

func decodeJSONNumber(out string, v any) error {
	dec := json.NewDecoder(strings.NewReader(out))
	dec.UseNumber()
	return dec.Decode(v)
}

// jsonScalarID keeps large GitHub IDs as full digits (never scientific notation).
func jsonScalarID(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case json.Number:
		return x.String()
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if f, err := strconv.ParseFloat(s, 64); err == nil && !strings.ContainsAny(s, "eE") {
			return strconv.FormatInt(int64(f), 10)
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return strconv.FormatInt(int64(f), 10)
		}
		return s
	}
}

func parseGHAWorkflowJSON(out string) ([]GHAWorkflow, error) {
	type row struct {
		ID    interface{} `json:"id"`
		Name  string      `json:"name"`
		State string      `json:"state"`
		Path  string      `json:"path"`
	}
	var rows []row
	if err := decodeJSONNumber(out, &rows); err != nil {
		return nil, err
	}
	list := make([]GHAWorkflow, 0, len(rows))
	for _, r := range rows {
		list = append(list, GHAWorkflow{
			ID:    jsonScalarID(r.ID),
			Name:  r.Name,
			State: r.State,
			Path:  r.Path,
		})
	}
	return list, nil
}

func parseGHARunJSON(out string) ([]GHARun, error) {
	type row struct {
		DatabaseID   interface{} `json:"databaseId"`
		Name         string      `json:"name"`
		DisplayTitle string      `json:"displayTitle"`
		Status       string      `json:"status"`
		Conclusion   string      `json:"conclusion"`
		WorkflowName string      `json:"workflowName"`
		HeadBranch   string      `json:"headBranch"`
		Event        string      `json:"event"`
		CreatedAt    string      `json:"createdAt"`
		StartedAt    string      `json:"startedAt"`
		UpdatedAt    string      `json:"updatedAt"`
		URL          string      `json:"url"`
	}
	var rows []row
	if err := decodeJSONNumber(out, &rows); err != nil {
		return nil, err
	}
	list := make([]GHARun, 0, len(rows))
	for _, r := range rows {
		list = append(list, GHARun{
			ID:           jsonScalarID(r.DatabaseID),
			Name:         r.Name,
			DisplayTitle: r.DisplayTitle,
			Status:       r.Status,
			Conclusion:   r.Conclusion,
			Workflow:     r.WorkflowName,
			Branch:       r.HeadBranch,
			Event:        r.Event,
			CreatedAt:    r.CreatedAt,
			StartedAt:    r.StartedAt,
			UpdatedAt:    r.UpdatedAt,
			URL:          r.URL,
		})
	}
	return list, nil
}

func parseGHAJobsJSON(out string) ([]GHAJob, error) {
	type stepRow struct {
		Name        string      `json:"name"`
		Status      string      `json:"status"`
		Conclusion  string      `json:"conclusion"`
		Number      interface{} `json:"number"`
		StartedAt   string      `json:"startedAt"`
		CompletedAt string      `json:"completedAt"`
	}
	type jobRow struct {
		DatabaseID  interface{} `json:"databaseId"`
		Name        string      `json:"name"`
		Status      string      `json:"status"`
		Conclusion  string      `json:"conclusion"`
		StartedAt   string      `json:"startedAt"`
		CompletedAt string      `json:"completedAt"`
		Steps       []stepRow   `json:"steps"`
	}
	var wrap struct {
		Jobs []jobRow `json:"jobs"`
	}
	if err := decodeJSONNumber(out, &wrap); err != nil {
		return nil, err
	}
	list := make([]GHAJob, 0, len(wrap.Jobs))
	for _, j := range wrap.Jobs {
		job := GHAJob{
			ID:          jsonScalarID(j.DatabaseID),
			Name:        j.Name,
			Status:      j.Status,
			Conclusion:  j.Conclusion,
			StartedAt:   j.StartedAt,
			CompletedAt: j.CompletedAt,
		}
		for _, s := range j.Steps {
			n, _ := strconv.Atoi(jsonScalarID(s.Number))
			job.Steps = append(job.Steps, GHAStep{
				Name:        s.Name,
				Status:      s.Status,
				Conclusion:  s.Conclusion,
				Number:      n,
				StartedAt:   s.StartedAt,
				CompletedAt: s.CompletedAt,
			})
		}
		list = append(list, job)
	}
	return list, nil
}
