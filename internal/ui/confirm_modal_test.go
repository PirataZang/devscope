package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestRenderDeleteConfirmBox(t *testing.T) {
	got := stripANSI(renderDeleteConfirmBox(deleteConfirmOpts{
		Brand:    "DOCKER",
		Color:    tabAccentColor(TabContainers),
		Title:    "Excluir container",
		Subtitle: "docker rm",
		Label:    "container",
		Target:   "api-1",
		Detail:   "running · nginx",
	}, 80, 20))
	for _, want := range []string{"DOCKER", "Excluir container", "api-1", "y confirma", "n/esc"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestTunnelDeleteUsesSharedModal(t *testing.T) {
	got := stripANSI(renderTunnelDeleteConfirmBox("SSH", tabAccentColor(TabSSH), "app", "remote :3000", 80, 20))
	if !strings.Contains(got, "Excluir túnel") || !strings.Contains(got, "app") {
		t.Fatalf("%q", got)
	}
}

func TestGitDeleteConfirmModal(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/p", Git: &core.GitInfo{IsRepo: true, Branch: "main"}}
	a := &App{
		width: 100, height: 30, view: ViewProject, tab: TabGit,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		gitConfirmOn: true, gitConfirmAction: "delete", gitConfirmBranch: "feat/x",
	}
	got := stripANSI(a.renderGitConfirm())
	for _, want := range []string{"Excluir branch", "feat/x", "y confirma"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestGitPullStrategyModal(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/p", Git: &core.GitInfo{IsRepo: true, Branch: "feat"}}
	a := &App{
		width: 100, height: 30, view: ViewProject, tab: TabGit,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		gitConfirmOn: true, gitConfirmAction: "pull-strategy", gitConfirmBranch: "develop",
	}
	got := stripANSI(a.renderGitConfirm())
	for _, want := range []string{"Pull divergente", "origin/develop", "m merge", "r rebase"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestContainerDeleteConfirmOverlay(t *testing.T) {
	p := core.Project{
		Name: "demo", Path: "/p",
		Containers: []core.Container{{Name: "web", Image: "nginx", Status: "running", ID: "abc"}},
	}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabContainers,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		containerConfirmRemove: true, tabCursor: 0,
	}
	got := stripANSI(a.renderContainersTab(&p))
	for _, want := range []string{"Excluir container", "web", "y confirma"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
