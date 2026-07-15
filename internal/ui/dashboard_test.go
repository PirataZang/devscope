package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

func TestFilterNestedProjects(t *testing.T) {
	projects := []core.Project{
		{Path: "/apps/projeto", Name: "projeto"},
		{Path: filepath.Join("/apps/projeto", "compose"), Name: "compose"},
		{Path: "/apps/chat", Name: "chat"},
	}
	got := filterNestedProjects(projects)
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}
}

func TestTableRowNeverExceedsTerminalWidth(t *testing.T) {
	for _, termW := range []int{80, 95, 120, 160} {
		tableW := safeTableWidth(termW)
		if tableW > termW {
			t.Fatalf("table width %d exceeds terminal %d", tableW, termW)
		}
		cols := tableColumns(tableW)
		row := renderTableRow(cols, tableRow{
			icon: "L", name: "projeto-api",
			branch: "feat/kanban-long-branch", ctrs: "12",
		}, StyleNormal, ptrStatus(core.StatusRunning), false)
		if strings.Contains(row, "\n") {
			t.Fatalf("row contains newline at termW=%d", termW)
		}
		if lipgloss.Width(row) > tableW+2 {
			t.Fatalf("row width %d > tableW %d at termW=%d", lipgloss.Width(row), tableW, termW)
		}
	}
}

func TestSafeTableWidthNoForcedMinimum(t *testing.T) {
	if safeTableWidth(90) > 90 {
		t.Fatal("table width must not exceed terminal width")
	}
}

func TestDashboardColumnsNoPath(t *testing.T) {
	cols := tableColumns(78)
	row := renderTableRow(cols, tableRow{
		name: "projeto", branch: "develop", ctrs: "6",
	}, StyleNormal, ptrStatus(core.StatusRunning), false)
	if strings.Contains(row, "~/") || strings.Contains(row, "/home/") {
		t.Fatal("dashboard row should not contain path")
	}
}

func TestProjectStatusColors(t *testing.T) {
	run := renderStatusCell(12, core.StatusRunning, false)
	stop := renderStatusCell(12, core.StatusStopped, false)
	if run == stop {
		t.Fatal("running and stopped status should render differently")
	}
	if !strings.Contains(run, "Run") || !strings.Contains(stop, "Stop") {
		t.Fatal("status labels missing")
	}
}

func TestDashboardProjectsViewport(t *testing.T) {
	a := &App{height: 24}
	v := a.dashboardProjectsViewport()
	if v >= 24 {
		t.Fatalf("viewport %d should be less than terminal height", v)
	}
	if v < 3 {
		t.Fatal("viewport too small")
	}
}

func TestWrapText(t *testing.T) {
	lines := wrapText("hello world foo bar", 10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %d", len(lines))
	}
}

func ptrStatus(s core.ProjectStatus) *core.ProjectStatus {
	return &s
}
