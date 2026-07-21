package jenkinsutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSaveLoadProject(t *testing.T) {
	dir := t.TempDir()
	cfg := ProjectConfig{
		URL:        "https://ci.example.com/",
		User:       "admin",
		Token:      "secret-token",
		Folder:     "team",
		RefreshSec: 7,
	}
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir)
	if got.URL != "https://ci.example.com" {
		t.Fatalf("url=%q", got.URL)
	}
	if got.User != "admin" || got.Token != "secret-token" || got.Folder != "team" || got.RefreshSec != 7 {
		t.Fatalf("got %+v", got)
	}
	if !got.Configured() {
		t.Fatal("expected configured")
	}
	if got.Host() != "ci.example.com" {
		t.Fatalf("host=%q", got.Host())
	}
}

func TestLoadProjectMissing(t *testing.T) {
	got := LoadProject(t.TempDir())
	if got.RefreshSec != 5 || got.Configured() {
		t.Fatalf("got %+v", got)
	}
}

func TestMaskToken(t *testing.T) {
	if MaskToken("") != "(vazio)" {
		t.Fatal(MaskToken(""))
	}
	if MaskToken("ab") != "****" {
		t.Fatal(MaskToken("ab"))
	}
	m := MaskToken("abcdefgh")
	if m != "****efgh" {
		t.Fatalf("got %q", m)
	}
}

func TestColorToStatus(t *testing.T) {
	cases := map[string]string{
		"blue":       "success",
		"blue_anime": "running",
		"red":        "failure",
		"yellow":     "unstable",
		"aborted":    "aborted",
		"disabled":   "disabled",
		"":           "unknown",
	}
	for in, want := range cases {
		if got := ColorToStatus(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestJobAPIPath(t *testing.T) {
	if got := jobAPIPath("my-job"); got != "/job/my-job" {
		t.Fatal(got)
	}
	if got := jobAPIPath("folder/child"); got != "/job/folder/job/child" {
		t.Fatal(got)
	}
}

func TestClientListJobsAndConsole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":         "NORMAL",
			"nodeName":     "built-in",
			"numExecutors": 2,
			"jobs": []map[string]any{
				{
					"name":        "app",
					"url":         "http://example/job/app/",
					"color":       "blue",
					"description": "main",
					"buildable":   true,
					"inQueue":     false,
					"lastBuild":   map[string]any{"number": 42},
				},
			},
		})
	})
	mux.HandleFunc("/job/app/42/consoleText", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Started by user\nFinished: SUCCESS\n"))
	})
	mux.HandleFunc("/job/app/api/json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":        "app",
			"fullName":    "app",
			"url":         "http://example/job/app/",
			"color":       "blue",
			"description": "main",
			"buildable":   true,
			"builds": []map[string]any{
				{"number": 42, "url": "u", "result": "SUCCESS", "building": false, "duration": 12000, "timestamp": 1, "displayName": "#42"},
			},
			"lastBuild": map[string]any{"number": 42},
		})
	})
	mux.HandleFunc("/job/app/build", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/crumbIssuer/api/json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"crumb":             "abc",
			"crumbRequestField": "Jenkins-Crumb",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(ProjectConfig{URL: srv.URL, User: "u", Token: "t"})
	info := c.Ping()
	if !info.Connected || info.Err != "" {
		t.Fatalf("ping: %+v", info)
	}
	jobs, err := c.ListJobs()
	if err != nil || len(jobs) != 1 || jobs[0].LastBuild != 42 || jobs[0].Status != "success" {
		t.Fatalf("jobs=%v err=%v", jobs, err)
	}
	job, builds, err := c.GetJob("app")
	if err != nil || job.Name != "app" || len(builds) != 1 {
		t.Fatalf("job=%+v builds=%v err=%v", job, builds, err)
	}
	log, err := c.BuildConsole("app", 42)
	if err != nil || log == "" {
		t.Fatalf("console %q err=%v", log, err)
	}
	if err := c.TriggerBuild("app"); err != nil {
		t.Fatal(err)
	}
}

func TestConfigPath(t *testing.T) {
	p := ConfigPath("/tmp/proj")
	if filepath.Base(filepath.Dir(p)) != ".devscope" || filepath.Base(p) != "jenkins.json" {
		t.Fatal(p)
	}
}

func TestFormatHelpers(t *testing.T) {
	if FormatDuration(0) != "—" {
		t.Fatal(FormatDuration(0))
	}
	if FormatDuration(45000) != "45s" {
		t.Fatal(FormatDuration(45000))
	}
	if BuildStatus(Build{Building: true}) != "running" {
		t.Fatal("running")
	}
	if BuildStatus(Build{Result: "FAILURE"}) != "failure" {
		t.Fatal("failure")
	}
}
