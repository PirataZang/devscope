package collectors

import (
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestMatchScoreComposeRoot(t *testing.T) {
	project := "/home/user/myapp"
	m := containerMeta{
		WorkingDir: project,
	}
	if score := matchScore(project, m); score < 10000 {
		t.Fatalf("expected high score for exact compose root, got %d", score)
	}
}

func TestMatchScoreMountInsideProject(t *testing.T) {
	project := "/home/user/myapp"
	m := containerMeta{
		Mounts: []string{filepath.Join(project, "src")},
	}
	if score := matchScore(project, m); score < 7000 {
		t.Fatalf("expected mount inside project to match, got %d", score)
	}
}

func TestMatchScoreParentMountRejected(t *testing.T) {
	parent := "/home/user"
	child := filepath.Join(parent, "myapp")
	m := containerMeta{
		Mounts: []string{parent},
	}
	if score := matchScore(child, m); score != 0 {
		t.Fatalf("parent mount should not match child project, got %d", score)
	}
}

func TestParseHealthFromStatus(t *testing.T) {
	if parseHealthFromStatus("Up 2 hours (healthy)") != "healthy" {
		t.Fatal("expected healthy")
	}
	if parseHealthFromStatus("Up 1 minute (unhealthy)") != "unhealthy" {
		t.Fatal("expected unhealthy")
	}
}

func TestAssignContainersUnique(t *testing.T) {
	projects := []core.Project{
		{Path: "/apps/alpha", Name: "alpha"},
		{Path: "/apps/beta", Name: "beta"},
	}
	containers := []core.Container{
		{ID: "abc123", Name: "alpha-web"},
		{ID: "def456", Name: "beta-db"},
	}
	meta := map[string]containerMeta{
		"abc123": {WorkingDir: "/apps/alpha"},
		"def456": {WorkingDir: "/apps/beta"},
	}

	AssignContainersToProjects(projects, containers, meta)

	if len(projects[0].Containers) != 1 || projects[0].Containers[0].Name != "alpha-web" {
		t.Fatalf("alpha got wrong containers: %+v", projects[0].Containers)
	}
	if len(projects[1].Containers) != 1 || projects[1].Containers[0].Name != "beta-db" {
		t.Fatalf("beta got wrong containers: %+v", projects[1].Containers)
	}
}

func TestAssignContainersProjectPathFallback(t *testing.T) {
	projects := []core.Project{{Path: "/apps/alpha", Name: "alpha"}}
	containers := []core.Container{{
		ID: "abc123", Name: "web", ProjectPath: "/apps/alpha",
	}}
	AssignContainersToProjects(projects, containers, nil)
	if len(projects[0].Containers) != 1 {
		t.Fatalf("expected fallback match, got %+v", projects[0].Containers)
	}
}
