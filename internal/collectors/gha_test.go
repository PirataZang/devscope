package collectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeGHAProcessName(t *testing.T) {
	if got := sanitizeGHAProcessName("My CI!"); got != "my-ci" {
		t.Fatalf("got %q", got)
	}
}

func TestGHACreateAndDeleteProcess(t *testing.T) {
	dir := t.TempDir()
	proc, err := GHACreateProcess(dir, "unit-test", "test process", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if proc.Name != "unit-test" {
		t.Fatalf("name=%q", proc.Name)
	}
	body, err := os.ReadFile(filepath.Join(dir, proc.File))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "workflow_dispatch") {
		t.Fatalf("template missing dispatch: %s", body)
	}
	cat, err := LoadGHACatalog(dir)
	if err != nil || len(cat.Processes) != 1 {
		t.Fatalf("catalog=%+v err=%v", cat, err)
	}
	if err := GHADeleteProcess(dir, "unit-test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, proc.File)); !os.IsNotExist(err) {
		t.Fatalf("file should be gone: %v", err)
	}
	cat, _ = LoadGHACatalog(dir)
	if len(cat.Processes) != 0 {
		t.Fatalf("catalog still has %d", len(cat.Processes))
	}
}

func TestSyncGHACatalogFromDisk(t *testing.T) {
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "ci.yml"), []byte("name: CI\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := SyncGHACatalogFromDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Processes) != 1 || c.Processes[0].Name != "ci" {
		t.Fatalf("got %+v", c.Processes)
	}
}

func TestGHAActionsURL(t *testing.T) {
	got := GHAActionsURL("acme", "demo", ".github/workflows/ci.yml")
	want := "https://github.com/acme/demo/actions/workflows/ci.yml"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if GHAActionsURL("", "x", "") != "" {
		t.Fatal("empty owner")
	}
}

func TestGHARunURL(t *testing.T) {
	got := GHARunURL("acme", "demo", "30829919439")
	want := "https://github.com/acme/demo/actions/runs/30829919439"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if GHARunURL("acme", "demo", "") != "" {
		t.Fatal("empty run id")
	}
}

func TestGHARepoInfoKeepsRemoteWithoutGH(t *testing.T) {
	// Even without asserting gh presence, ParseGitHubRepo path must fill owner/repo.
	info := GHAInfo{}
	owner, repo, ok := ParseGitHubRepo("git@github.com:acme/demo.git")
	if !ok || owner != "acme" || repo != "demo" {
		t.Fatalf("%v %s/%s", ok, owner, repo)
	}
	_ = info
}

func TestGHAWorkflowTemplate(t *testing.T) {
	ci := GHAWorkflowTemplate("build", "ci")
	if !strings.Contains(ci, "pull_request") {
		t.Fatal(ci)
	}
	dep := GHAWorkflowTemplate("ship", "deploy")
	if !strings.Contains(dep, "tags") {
		t.Fatal(dep)
	}
}

func TestParseGHARunJSONKeepsFullDatabaseID(t *testing.T) {
	// Large IDs must not become scientific notation (float fmt.Sprint bug).
	raw := `[{"databaseId":30829919439,"name":"ci","displayTitle":"CI","status":"completed","conclusion":"success","workflowName":"android-develop","headBranch":"main","event":"workflow_dispatch","createdAt":"2026-01-01T00:00:00Z","url":"https://example.com"}]`
	runs, err := parseGHARunJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != "30829919439" {
		t.Fatalf("id=%q", runs[0].ID)
	}
	if strings.Contains(runs[0].ID, "e+") || strings.Contains(runs[0].ID, "E+") {
		t.Fatalf("scientific notation: %q", runs[0].ID)
	}
}

func TestJsonScalarIDFromFloat(t *testing.T) {
	// Simulate legacy float decode path.
	if got := jsonScalarID(float64(30829919439)); got != "30829919439" {
		t.Fatalf("got %q", got)
	}
}

func TestParseAndFormatGHAJobs(t *testing.T) {
	raw := `{"jobs":[{"databaseId":11,"name":"build","status":"completed","conclusion":"failure","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:02:00Z","steps":[{"name":"Checkout","status":"completed","conclusion":"success","number":1,"startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:00:30Z"},{"name":"Compile","status":"completed","conclusion":"failure","number":2,"startedAt":"2026-01-01T00:00:30Z","completedAt":"2026-01-01T00:02:00Z"}]}]}`
	jobs, err := parseGHAJobsJSON(raw)
	if err != nil || len(jobs) != 1 || jobs[0].ID != "11" || len(jobs[0].Steps) != 2 {
		t.Fatalf("%+v err=%v", jobs, err)
	}
	text := FormatGHATimelineText(jobs)
	if !strings.Contains(text, "TIMELINE") || !strings.Contains(text, "✗") || !strings.Contains(text, "[") || !strings.Contains(text, "⣿") {
		t.Fatalf("%q", text)
	}
}

func TestGHAFailHeatmapAndBilling(t *testing.T) {
	runs := []GHARun{
		{Workflow: "ci", Conclusion: "failure", StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:10:00Z"},
		{Workflow: "ci", Conclusion: "success", StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:05:00Z"},
		{Workflow: "deploy", Conclusion: "failure", StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:20:00Z"},
	}
	heat := GHAFailHeatmap(runs, 10)
	if len(heat) != 2 || heat[0].Process != "ci" || heat[0].Fails != 1 {
		t.Fatalf("%+v", heat)
	}
	bill := GHABillingEstimate(runs, 10)
	if FormatGHABilling(bill) == "0m" {
		t.Fatal("expected minutes")
	}
	if SumGHAMinutes(bill) < 30 {
		t.Fatalf("sum=%v", SumGHAMinutes(bill))
	}
	if FormatGHAMinutes(5.2) != "~5.2m" {
		t.Fatalf("fmt=%q", FormatGHAMinutes(5.2))
	}
	if jsonNumberFloat(json.Number("1234.5")) != 1234.5 {
		t.Fatal("json float")
	}
}

func TestParseWorkflowDispatchInputs(t *testing.T) {
	body := `
name: CI
on:
  workflow_dispatch:
    inputs:
      environment:
        description: env
        type: choice
        required: true
        default: staging
        options:
          - staging
          - prod
      skip_tests:
        type: boolean
        default: false
`
	ins, err := parseWorkflowDispatchInputs(body)
	if err != nil || len(ins) != 2 {
		t.Fatalf("%+v err=%v", ins, err)
	}
}

func TestGHANotesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SaveGHANote(dir, "99", "incidente", "ci"); err != nil {
		t.Fatal(err)
	}
	if got := GHANoteForRun(dir, "99"); got != "incidente" {
		t.Fatalf("%q", got)
	}
}
