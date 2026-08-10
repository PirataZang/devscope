package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestProjectFilterInlineLive(t *testing.T) {
	a := &App{
		view:   ViewDashboard,
		width:  120,
		height: 40,
		snapshot: core.Snapshot{
			Projects: []core.Project{
				{Name: "alpha", Path: "/apps/alpha", Framework: core.FrameworkInfo{Name: "go"}},
				{Name: "beta-api", Path: "/apps/beta", Framework: core.FrameworkInfo{Name: "laravel"}},
			},
		},
	}
	_, _ = a.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.filterOn {
		t.Fatal("/ should enable inline project filter on dashboard")
	}
	for _, ch := range "beta" {
		_, _ = a.updateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if a.filter != "beta" || !a.filterOn {
		t.Fatalf("live filter: on=%v filter=%q", a.filterOn, a.filter)
	}
	got := a.filteredProjects()
	if len(got) != 1 || got[0].Name != "beta-api" {
		t.Fatalf("filtered: %+v", got)
	}
	line := stripANSI(a.renderProjectsFilterLine(80))
	if !strings.Contains(line, "filter") || !strings.Contains(line, "beta") {
		t.Fatalf("inline filter line: %q", line)
	}
	view := stripANSI(a.renderDashboard())
	if !strings.Contains(view, "filter") {
		t.Fatal("dashboard should show inline filter, not a separate prompt screen")
	}
	if strings.Contains(view, "Filter: beta█") {
		t.Fatal("old full-screen filter prompt should be gone")
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

	for _, want := range []string{
		"WATCH", "SCOPE", "AUTOMATION", "MANAGER", "TUNNEL", "TOOLS",
		"Visão Geral", "Metrics", "Status",
		"Git", "Containers",
		"GH Actions", "Jenkins",
		"Swarm", "Kubernetes",
		"Ngrok", "SSH Tunnel", "CF Tunnel",
		"Rotas", "API", "Database", "WS",
		"tab · shift+tab",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in sidebar: %q", want, plain)
		}
	}
	if strings.Contains(plain, "Logs") {
		t.Fatalf("Logs removed from sidebar: %q", plain)
	}
	for _, ban := range []string{"RESUMO", "DISK"} {
		if strings.Contains(plain, ban) {
			t.Fatalf("host stats %q devem ficar só no dashboard: %q", ban, plain)
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
	if !strings.Contains(got, "devscope") {
		t.Fatalf("app brand missing: %q", got)
	}
	if !strings.Contains(got, "demo") {
		t.Fatalf("project name missing: %q", got)
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
	for _, ban := range []string{"RESUMO", "DISK", "CPU"} {
		if strings.Contains(got, ban) {
			t.Fatalf("host stats %q devem ficar só no dashboard: %q", ban, got)
		}
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

func TestProjectViewHidesHostMetricsBar(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	host := core.HostMetrics{CPUPercent: 9, MemoryPercent: 92, DiskPercent: 72}
	pills := stripANSI(renderMetricPills(host))
	for _, size := range []struct{ w, h int }{{100, 28}, {160, 48}} {
		a := &App{
			width:           size.w,
			height:          size.h,
			view:            ViewProject,
			tab:             TabOverview,
			selectedProject: &project,
			snapshot:        core.Snapshot{Projects: []core.Project{project}, HostMetrics: host},
		}

		got := stripANSI(a.renderProject())
		if strings.Contains(got, pills) {
			t.Fatalf("%dx%d: barra de métricas do host só no dashboard: %q", size.w, size.h, got)
		}
		if !strings.Contains(got, "Visão Geral") || !strings.Contains(got, "WATCH") {
			t.Fatalf("%dx%d: sidebar deve continuar", size.w, size.h)
		}
	}
}

func TestDashboardKeepsHostMetricsBar(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	a := &App{
		width: 160, height: 48, view: ViewDashboard,
		snapshot: core.Snapshot{
			Projects:    []core.Project{project},
			HostMetrics: core.HostMetrics{CPUPercent: 9, MemoryPercent: 92, DiskPercent: 72},
		},
	}
	got := stripANSI(a.renderDashboard())
	for _, want := range []string{"DevScope", "CPU 9%", "RAM 92%", "DISK 72%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dashboard deve manter %q: %q", want, got)
		}
	}
}

func TestMetricUsageStyleThresholds(t *testing.T) {
	cases := []struct {
		pct  float64
		want lipgloss.Color
	}{
		{0, ColorSuccess},
		{30, ColorSuccess},
		{31, ColorWarning},
		{70, ColorWarning},
		{71, ColorDanger},
		{100, ColorDanger},
	}
	for _, c := range cases {
		got := metricUsageStyle(c.pct).Render("x")
		want := lipgloss.NewStyle().Foreground(c.want).Bold(true).Render("x")
		if got != want {
			t.Fatalf("pct=%.0f: cor errada", c.pct)
		}
	}
}

func TestTinySidebarFitsPanelHeight(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	a := &App{
		width:           120,
		height:          16,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
	}
	panelH := a.projectPanelHeight()
	side := a.renderProjectSidebar()
	if lipgloss.Height(side) > panelH {
		t.Fatalf("sidebar height %d exceeds panel %d", lipgloss.Height(side), panelH)
	}
	plain := stripANSI(side)
	if strings.Contains(plain, "RESUMO") {
		t.Fatalf("tiny sidebar should drop RESUMO: %q", plain)
	}
	got := stripANSI(a.renderProject())
	if !strings.Contains(got, "SCOPE") {
		t.Fatal("tiny project view should keep nav groups")
	}
}

func TestOverviewCompactOnShortTerminal(t *testing.T) {
	project := core.Project{
		Name: "app", Path: "/p", Status: core.StatusRunning, Health: core.HealthHealthy,
		Git: &core.GitInfo{IsRepo: true, Branch: "main", LastCommit: "abc1234"},
	}
	a := &App{
		width: 100, height: 18, tab: TabOverview,
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
	}
	got := stripANSI(a.renderOverviewTab(&project))
	if strings.Contains(got, "NOTAS") || strings.Contains(got, "AÇÕES RÁPIDAS") {
		t.Fatalf("short overview should omit right rail: %q", got)
	}
	if !strings.Contains(got, "GIT") && !strings.Contains(got, "Git") {
		// git box title may vary — at least project name should show
		if !strings.Contains(got, "app") {
			t.Fatalf("compact overview empty: %q", got)
		}
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
	if lipgloss.Width(row) > a.width-8 {
		t.Fatalf("container row width %d exceeds content width", lipgloss.Width(row))
	}
}

func TestRenderContainersMainShowsBottomBoxes(t *testing.T) {
	project := core.Project{
		Path: "/tmp/digiliza",
		Name: "digiliza",
		Containers: []core.Container{
			{ID: "abc", Name: "laradock-workspace-1", Image: "laradock/workspace", Status: "running", State: "Up 2 hours", Ports: "0.0.0.0:80->80/tcp", CPU: 12.6, Memory: 400 << 20},
			{ID: "def", Name: "laradock-nginx-1", Image: "nginx", Status: "running", State: "Up 2 hours"},
		},
		Metrics: core.ProjectMetrics{CPUPercent: 12.6, MemoryMB: 400},
	}
	a := &App{
		width:                   120,
		height:                  40,
		view:                    ViewProject,
		tab:                     TabContainers,
		containerSubview:        containerSubviewList,
		containerPreviewLogs:    "2026-07-20 INFO boot ok\n2026-07-20 WARN slow query",
		containerPreviewStats:   "CPU (%): 12.6%\nMemory: 400MiB / 16GiB\nNet I/O: 1.2MB / 800KB",
		containerPreviewVolumes: []string{"laradock_data", "laradock_mysql"},
		containerCPUHistory:     []float64{4, 8, 12, 10, 14},
		containerMemHistory:     []float64{10, 12, 11, 13, 12},
		containerNetHistory:     []float64{100, 120, 90, 140, 110},
		containerPreviewID:      "abc",
		selectedProject:         &project,
		snapshot:                core.Snapshot{Projects: []core.Project{project}},
	}
	got := stripANSI(a.renderContainersTab(&project))
	for _, want := range []string{"CONTAINERS", "LISTA", "LOGS", "STATS · ALL", "PORTAS", "laradock-workspace-1", "CPU ", "MEM ", "NET "} {
		if !strings.Contains(got, want) {
			t.Fatalf("containers main missing %q in:\n%s", want, got)
		}
	}
	a.containerStatsMode = 1
	got = stripANSI(a.renderContainersBottom(120, 20))
	if !strings.Contains(got, "STATS · CPU") {
		t.Fatalf("cpu focus title missing: %s", got)
	}
	if !strings.Contains(got, "S-U") || !strings.Contains(got, "compose") {
		t.Fatalf("actions should list compose shortcuts:\n%s", truncate(got, 400))
	}
}

func TestParseDockerStatsSample(t *testing.T) {
	cpu, mem, net := parseDockerStatsSample("CPU (%): 12.6%\nMemory: 400MiB / 16GiB (3.34%)\nNet I/O: 1.2MB / 800KB")
	if cpu < 12 || cpu > 13 {
		t.Fatalf("cpu=%v", cpu)
	}
	if mem < 3 || mem > 4 {
		t.Fatalf("mem=%v", mem)
	}
	if net < 1000 { // ~1.2MB + 800KB
		t.Fatalf("net=%v", net)
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
	for _, tab := range []containerDetailTab{containerDetailTabCompose, containerDetailTabFile} {
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

func TestContainerStatsDashboard(t *testing.T) {
	project := core.Project{Path: "/projects/app", Name: "app"}
	a := &App{
		width: 120, height: 36,
		view: ViewProject, tab: TabContainers,
		containerSubview:    containerSubviewDetail,
		containerDetailTab:  containerDetailTabStats,
		containerDetailName: "laradock-workspace-1",
		containerDetailID:   "abc123",
		containerDetailStats: dockerStatsSample{
			CPU: 12.5, MemPct: 4.8, MemLabel: "959MiB / 19GiB (4.8%)",
			NetRX: 1000, NetTX: 500, NetLabel: "1MB / 500KB",
			BlkR: 2000, BlkW: 1000, BlkLabel: "2MB / 1MB", PIDs: 39,
			Raw: "CPU (%): 12.5%\nMemory: 959MiB / 19GiB (4.8%)",
		},
		containerDetailCPUHist:   []float64{4, 8, 12, 10, 14, 12.5},
		containerDetailMemHist:   []float64{3, 4, 5, 4.5, 4.8},
		containerDetailNetHist:   []float64{800, 900, 1000, 1500},
		containerDetailBlkHist:   []float64{1000, 2000, 2500, 3000},
		containerDetailPIDHist:   []float64{30, 35, 39},
		containerDetailStatsLive: true,
		selectedProject:          &project,
		snapshot:                 core.Snapshot{Projects: []core.Project{project}},
	}
	got := stripANSI(a.renderProject())
	for _, want := range []string{"laradock-workspace-1", "CPU", "MEMÓRIA", "REDE", "BLOCK", "I/O", "SAÚDE", "live", "12.50%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stats dashboard missing %q in:\n%s", want, got)
		}
	}
	// must not fall back to the old plain-text stats body
	if strings.Contains(got, "CPU (%):") && !strings.Contains(got, "MEMÓRIA") {
		t.Fatal("renderProject still using old text stats screen")
	}
}

func TestParseDockerStatsFull(t *testing.T) {
	s := parseDockerStatsFull("CPU (%): 0.41%\nMemory: 959.7MiB / 19.25GiB (4.87%)\nNet I/O: 10.9MB / 5.21MB\nBlock I/O: 469MB / 538MB\nPIDs: 39")
	if s.CPU < 0.4 || s.MemPct < 4.8 || s.PIDs != 39 || s.NetRX < 10000 {
		t.Fatalf("%+v", s)
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

func TestDashboardHeaderAlwaysShowsMetrics(t *testing.T) {
	projects := make([]core.Project, 20)
	for i := range projects {
		projects[i] = core.Project{Path: fmt.Sprintf("/p/%d", i), Name: fmt.Sprintf("app-%d", i)}
	}
	host := core.HostMetrics{CPUPercent: 35, MemoryPercent: 96, DiskPercent: 72}
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {200, 60}, {240, 80}} {
		a := &App{
			width: size.w, height: size.h, view: ViewDashboard,
			snapshot: core.Snapshot{Projects: projects, HostMetrics: host},
		}
		raw := a.renderDashboard()
		if lipgloss.Height(raw) > size.h {
			t.Fatalf("%dx%d: dashboard height %d exceeds terminal", size.w, size.h, lipgloss.Height(raw))
		}
		got := stripANSI(raw)
		for _, want := range []string{"CPU 35%", "RAM 96%", "DISK 72%"} {
			if !strings.Contains(got, want) {
				t.Fatalf("%dx%d missing %q\n%s", size.w, size.h, want, got)
			}
		}
		// Métricas na 1ª linha útil (antes de SYSTEM/PROJECTS).
		lines := strings.Split(got, "\n")
		found := false
		for _, line := range lines {
			if strings.Contains(line, "CPU 35%") {
				found = true
				break
			}
			if strings.Contains(line, "SYSTEM OVERVIEW") || strings.Contains(line, "PROJECTS") {
				t.Fatalf("%dx%d: metrics appear after SYSTEM/PROJECTS\n%s", size.w, size.h, got)
			}
		}
		if !found {
			t.Fatalf("%dx%d: metrics never found", size.w, size.h)
		}
	}
}
