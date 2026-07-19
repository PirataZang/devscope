package ui

import (
	"fmt"
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
			path: "/home/user/projects/projeto-api", branch: "feature/dashboard", ctrs: "12",
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

func TestDashboardShowsProjectPath(t *testing.T) {
	cols := tableColumns(78)
	row := renderTableRow(cols, tableRow{
		name: "projeto", path: "/home/user/projeto", branch: "main", ctrs: "6",
	}, StyleNormal, ptrStatus(core.StatusRunning), false)
	if !strings.Contains(row, "/home/user/projeto") {
		t.Fatal("dashboard row should contain project path")
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

func TestCurrentProjectUsesPathWhenIDsAreEmpty(t *testing.T) {
	a := &App{
		snapshot: core.Snapshot{Projects: []core.Project{
			{Name: "first", Path: "/projects/first"},
			{Name: "second", Path: "/projects/second"},
		}},
		selectedProject: &core.Project{Path: "/projects/second"},
	}

	if got := a.currentProject(); got == nil || got.Name != "second" {
		t.Fatalf("expected second project, got %+v", got)
	}
}

func TestOverlayCenteredRendersPopup(t *testing.T) {
	background := strings.Repeat("background\n", 10)
	got := overlayCentered(background, "┌────┐\n│help│\n└────┘", 30, 10)
	if !strings.Contains(got, "│help│") {
		t.Fatal("popup was not rendered")
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatal("overlay should not use terminal cursor sequences")
	}
}

func TestProjectSidebarShowsVerticalTabs(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabContainers}
	got := a.renderProjectSidebar()
	plain := stripANSI(got)

	for _, want := range []string{"SCOPE", "WATCH", "TOOLS", "Overview", "Containers", "API", "Database", "1-8", "cpu"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in sidebar: %q", want, plain)
		}
	}
	// One outer rail (single top border), not 7 stacked cards.
	if strings.Count(plain, "╭") != 1 && strings.Count(plain, "┌") != 1 {
		t.Fatalf("expected one sidebar rail box: %q", plain)
	}
}

func TestProjectSidebarShowsLiveMeta(t *testing.T) {
	p := core.Project{
		Name:   "demo",
		Path:   "/p",
		Status: core.StatusDegraded,
		Health: core.HealthUnhealthy,
		Git:    &core.GitInfo{IsRepo: true, Branch: "develop", Modified: 3},
	}
	a := &App{
		width:           120,
		height:          40,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "demo") {
		t.Fatalf("project brand missing: %q", got)
	}
	if !strings.Contains(got, "Deg") {
		t.Fatalf("status missing in brand: %q", got)
	}
	if !strings.Contains(got, "develop") {
		t.Fatalf("branch missing in brand: %q", got)
	}
	if strings.Contains(got, "containers") {
		t.Fatalf("brand must not show containers: %q", got)
	}
	if !strings.Contains(got, "cpu") || !strings.Contains(got, "ram") {
		t.Fatalf("footer meters missing: %q", got)
	}
}

func TestProjectPanelKeepsFixedHeight(t *testing.T) {
	a := &App{}
	var body []string
	for i := 0; i < 30; i++ {
		body = append(body, fmt.Sprintf("line %d", i))
	}
	content := StylePanel.Width(54).Render(strings.Join(body, "\n"))

	got := a.renderProjectPanel(content, lipgloss.Width(content), 12)
	if lipgloss.Height(got) != 12 {
		t.Fatalf("expected fixed height 12, got %d", lipgloss.Height(got))
	}
	if !strings.Contains(got, "linhas") {
		t.Fatal("expected overflow indicator")
	}
}

func TestProjectRuntimeMetricsUsesOnlyProjectProcesses(t *testing.T) {
	p := &core.Project{
		Containers: []core.Container{{CPU: 12.5, Memory: 100 * 1024 * 1024}},
		Workers: []core.Worker{
			{Status: "online", CPU: 3.5, Memory: 20 * 1024 * 1024},
			{Status: "stopped", CPU: 99, Memory: 99 * 1024 * 1024},
		},
	}

	cpu, memory := projectRuntimeMetrics(p)
	if cpu != 16 || memory != 120 {
		t.Fatalf("expected CPU 16 and RAM 120 MB, got %.1f and %d", cpu, memory)
	}
}

func TestCompactProjectViewHidesHeader(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	a := &App{
		width:           100,
		height:          28,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
	}

	got := a.renderProject()
	if strings.Contains(got, "DevScope") {
		t.Fatal("compact project view should hide the header")
	}
	if !strings.Contains(stripANSI(got), "Overview") || !strings.Contains(stripANSI(got), "SCOPE") {
		t.Fatal("compact project view should keep the sidebar")
	}
}

func TestCompactGitColumnsFitContent(t *testing.T) {
	for _, width := range []int{50, 65, 90} {
		a := &App{width: width}
		used := a.gitBranchColWidth() + a.gitCommitColWidth() + 15
		if used > width {
			t.Fatalf("git columns use %d cells in width %d", used, width)
		}
	}
}

func TestCompactContainerRowFitsContent(t *testing.T) {
	a := &App{width: 60}
	row := a.renderContainerRow(core.Container{
		Status: "running",
		Name:   "long-container-name",
		Image:  "registry/example/long-image-name",
	}, false)
	if lipgloss.Width(row) > a.width-10 {
		t.Fatalf("container row width %d exceeds content width", lipgloss.Width(row))
	}
}

func TestContainerDetailFillsTerminalHeight(t *testing.T) {
	a := &App{
		width:                  100,
		height:                 30,
		containerDetailName:    "web",
		containerDetailContent: "one line",
	}
	got := a.renderContainerDetail(&core.Project{})
	if lipgloss.Height(got) < a.height-2 {
		t.Fatalf("detail height %d too small for terminal %d", lipgloss.Height(got), a.height)
	}
}

func TestContainerLogLineFitsPanel(t *testing.T) {
	a := &App{width: 60}
	line := a.renderContainerDetailLine(
		containerDetailTabLogs,
		"\x1b[31m"+strings.Repeat("very long log entry ", 20)+"\r\x1b[0m",
	)
	if lipgloss.Width(line) > a.width-10 {
		t.Fatalf("log line width %d exceeds panel content", lipgloss.Width(line))
	}
	if strings.Contains(line, "\r") || strings.Contains(line, "\x1b[31m") {
		t.Fatal("terminal control sequences must be removed from logs")
	}
}

func TestCompactContainerDetailTabsFitOneLine(t *testing.T) {
	a := &App{width: 60, containerDetailTab: containerDetailTabEnv}
	got := a.renderContainerDetailTabBar(50)
	if strings.Contains(got, "\n") || lipgloss.Width(got) > 50 {
		t.Fatalf("tab bar width %d does not fit compact panel", lipgloss.Width(got))
	}
	if !strings.Contains(got, "Env") {
		t.Fatal("active Env tab must remain fully visible")
	}
}

func TestContainerDetailActiveTabNeverTruncated(t *testing.T) {
	for i := 0; i < containerDetailTabTotal; i++ {
		tab := containerDetailTab(i)
		a := &App{width: 40, containerDetailTab: tab}
		got := a.renderContainerDetailTabBar(36)
		if !strings.Contains(got, tab.shortLabel()) {
			t.Fatalf("active tab %s was truncated away: %q", tab.shortLabel(), got)
		}
	}
}

func TestFitProjectPanelKeepsExactHeight(t *testing.T) {
	content := StylePanel.Render(strings.Repeat("line\n", 30))
	got := fitProjectPanel(content, lipgloss.Width(content), 12)
	if lipgloss.Height(got) != 12 {
		t.Fatalf("panel height %d, expected 12", lipgloss.Height(got))
	}
	if strings.Contains(got, "linhas") {
		t.Fatal("docker detail must not use the outer scroll indicators")
	}
}

func TestContainerDetailScrollReachesLastLine(t *testing.T) {
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("log %d", i))
	}
	a := &App{
		width:                  100,
		height:                 30,
		containerDetailTab:     containerDetailTabLogs,
		containerDetailContent: strings.Join(lines, "\n"),
	}
	a.containerDetailScrollBy(100)

	got := a.renderContainerDetail(&core.Project{})
	if !strings.Contains(got, "log 30") {
		t.Fatal("last log line is not visible at maximum scroll")
	}
	if strings.Contains(got, "acima") || strings.Contains(got, "abaixo") {
		t.Fatal("scroll indicators must not displace log lines")
	}
}

func TestContainerLogsOpenAtLatestLine(t *testing.T) {
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("log %d", i))
	}
	a := &App{
		width:              100,
		height:             28,
		containerDetailTab: containerDetailTabLogs,
		containerDetailID:  "container-id",
	}
	a.handleContainerDetailLoaded(containerDetailLoadedMsg{
		tab:     containerDetailTabLogs,
		id:      "container-id",
		content: strings.Join(lines, "\n"),
	})

	got := a.renderContainerTextScreen()
	if !strings.Contains(got, "log 30") {
		t.Fatal("logs screen must open at the latest entries")
	}
}

func TestContainerLogsUseDedicatedFullScreen(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	a := &App{
		width:                  100,
		height:                 28,
		view:                   ViewProject,
		tab:                    TabContainers,
		containerSubview:       containerSubviewDetail,
		containerDetailTab:     containerDetailTabLogs,
		containerDetailName:    "web",
		containerDetailContent: "first\nsecond",
		selectedProject:        &project,
		snapshot:               core.Snapshot{Projects: []core.Project{project}},
	}

	got := a.renderProject()
	if strings.Contains(stripANSI(got), "SCOPE") {
		t.Fatal("dedicated logs screen must not render the project sidebar")
	}
	if !strings.Contains(got, "web") || !strings.Contains(got, "▶ Logs") || !strings.Contains(got, "first") {
		t.Fatal("dedicated logs screen is missing its title or content")
	}
	if lipgloss.Width(got) > a.width+2 {
		t.Fatalf("logs screen width %d exceeds terminal width %d", lipgloss.Width(got), a.width)
	}
}

func TestContainerFilesUseDedicatedFullScreen(t *testing.T) {
	for _, tab := range []containerDetailTab{containerDetailTabCompose, containerDetailTabFile, containerDetailTabStats} {
		project := core.Project{Path: "/projects/app", Name: "app"}
		a := &App{
			width:                  100,
			height:                 28,
			view:                   ViewProject,
			tab:                    TabContainers,
			containerSubview:       containerSubviewDetail,
			containerDetailTab:     tab,
			containerDetailName:    "web",
			containerDetailContent: "services:\n  web:",
			selectedProject:        &project,
			snapshot:               core.Snapshot{Projects: []core.Project{project}},
		}

		got := a.renderProject()
		if strings.Contains(stripANSI(got), "SCOPE") {
			t.Fatalf("%s screen must not render the project sidebar", tab.shortLabel())
		}
		if !strings.Contains(got, "web") || !strings.Contains(got, tab.shortLabel()) || !strings.Contains(got, "services:") {
			t.Fatalf("%s screen is missing its title or content", tab.shortLabel())
		}
	}
}

func TestAppendContainerLogsDedupesOverlap(t *testing.T) {
	got := appendContainerLogs("line1\nline2\n", "line2\nline3\n")
	if strings.Count(got, "line2") != 1 || !strings.Contains(got, "line3") {
		t.Fatalf("unexpected merge: %q", got)
	}
}

func TestContainerDetailSearchJumpsToMatch(t *testing.T) {
	a := &App{
		width:                      100,
		height:                     28,
		containerDetailTab:         containerDetailTabLogs,
		containerDetailContent:     "alpha\nbeta search-here\ngamma\n",
		containerDetailSearchQuery: "search-here",
	}
	a.jumpContainerDetailSearch(0)
	matches := a.containerDetailSearchMatches()
	if len(matches) != 1 || matches[0] != 1 {
		t.Fatalf("unexpected matches: %v", matches)
	}
}

func TestContainerDetailFollowAppendsAndStaysAtEnd(t *testing.T) {
	a := &App{
		width:                    100,
		height:                   28,
		containerDetailTab:       containerDetailTabLogs,
		containerDetailID:        "c1",
		containerDetailContent:   "old\n",
		containerDetailFollow:    true,
		containerDetailFollowGen: 3,
		containerDetailCache:     map[containerDetailTab]string{},
	}
	a.containerDetailScroll = len(a.containerDetailLines())
	_ = a.handleContainerDetailFollow(containerDetailFollowMsg{
		id: "c1", gen: 3, logs: "new-line\n",
	})
	if !strings.Contains(a.containerDetailContent, "new-line") {
		t.Fatal("follow did not append logs")
	}
	if !a.isContainerDetailAtEnd() {
		t.Fatal("follow should keep sticky end when already at bottom")
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
