package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jenkinsutil"
)

type jenkinsSubTab int

const (
	jenkinsTabOverview jenkinsSubTab = iota
	jenkinsTabPipelines
	jenkinsTabBuilds
	jenkinsTabSettings
)

type jenkinsFocus int

const (
	jenkinsFocusNav jenkinsFocus = iota
	jenkinsFocusTable
	jenkinsFocusDetail
	jenkinsFocusLogs
)

const (
	jenkinsSetURL = iota
	jenkinsSetUser
	jenkinsSetToken
	jenkinsSetFolder
	jenkinsSetRefresh
)

type jenkinsLoadedMsg struct {
	cfg     jenkinsutil.ProjectConfig
	info    jenkinsutil.ServerInfo
	jobs    []jenkinsutil.Job
	builds  []jenkinsutil.Build
	console string
	queue   int
	err     string
	gen     int
}

type jenkinsActionMsg struct {
	out string
	err string
	gen int
}

type jenkinsTickMsg struct {
	gen int
}

func (a *App) enterJenkinsTab(_ *core.Project) {
	a.tab = TabJenkins
	a.jenkinsOpen = false
}

func (a *App) openJenkinsClient(p *core.Project) tea.Cmd {
	a.jenkinsOpen = true
	a.jenkinsSubTab = jenkinsTabPipelines
	a.jenkinsFocus = jenkinsFocusTable
	a.jenkinsCursor = 0
	a.jenkinsScroll = 0
	a.jenkinsBuildCursor = 0
	a.jenkinsBuildScroll = 0
	a.jenkinsLogScroll = 0
	a.jenkinsLogHScroll = 0
	a.jenkinsErr = ""
	a.jenkinsStatus = ""
	a.jenkinsBuildDetail = false
	a.jenkinsEditing = false
	a.jenkinsGen++
	a.loadJenkinsSettingsDraft(p)
	return a.refreshJenkins(p)
}

func (a *App) leaveJenkinsTab() tea.Cmd {
	a.jenkinsOpen = false
	a.jenkinsEditing = false
	a.jenkinsBuildDetail = false
	a.tab = TabJenkins
	a.jenkinsGen++
	return nil
}

func (a *App) loadJenkinsSettingsDraft(p *core.Project) {
	cfg := a.jenkinsCfg
	if p != nil {
		cfg = jenkinsutil.LoadProject(p.Path)
	}
	a.jenkinsCfg = cfg
	a.jenkinsEditURL = cfg.URL
	a.jenkinsEditUser = cfg.User
	a.jenkinsEditToken = cfg.Token
	a.jenkinsEditFolder = cfg.Folder
	a.jenkinsEditRefresh = strconv.Itoa(cfg.RefreshSec)
	if a.jenkinsEditRefresh == "0" || a.jenkinsEditRefresh == "" {
		a.jenkinsEditRefresh = "5"
	}
	a.jenkinsSetField = jenkinsSetURL
	a.jenkinsSetCursor = len([]rune(a.jenkinsEditURL))
}

func (a *App) refreshJenkins(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	a.jenkinsLoading = true
	path := p.Path
	jobName := a.jenkinsSelectedJobName()
	buildNum := a.jenkinsSelectedBuildNum()
	gen := a.jenkinsGen
	return func() tea.Msg {
		cfg := jenkinsutil.LoadProject(path)
		c := jenkinsutil.NewClient(cfg)
		info := c.Ping()
		if info.Err != "" && !cfg.Configured() {
			return jenkinsLoadedMsg{cfg: cfg, info: info, err: info.Err, gen: gen}
		}
		jobs, err := c.ListJobs()
		msg := jenkinsLoadedMsg{cfg: cfg, info: info, jobs: jobs, gen: gen}
		if err != nil {
			msg.err = err.Error()
			return msg
		}
		running := 0
		for _, j := range jobs {
			if j.Status == "running" {
				running++
			}
		}
		info.BusyExec = running
		msg.info = info
		q, _ := c.QueueDepth()
		msg.queue = q

		if jobName == "" && len(jobs) > 0 {
			jobName = jobs[0].FullName
		}
		if jobName != "" {
			_, builds, jerr := c.GetJob(jobName)
			if jerr == nil {
				msg.builds = builds
				if buildNum <= 0 && len(builds) > 0 {
					buildNum = builds[0].Number
				}
				if buildNum > 0 {
					console, cerr := c.BuildConsole(jobName, buildNum)
					if cerr == nil {
						msg.console = console
					} else if msg.err == "" {
						msg.err = cerr.Error()
					}
				}
			} else if msg.err == "" {
				msg.err = jerr.Error()
			}
		}
		return msg
	}
}

func (a *App) scheduleJenkinsTick() tea.Cmd {
	sec := a.jenkinsCfg.RefreshSec
	if sec <= 0 {
		sec = 5
	}
	gen := a.jenkinsGen
	return tea.Tick(time.Duration(sec)*time.Second, func(t time.Time) tea.Msg {
		return jenkinsTickMsg{gen: gen}
	})
}

func (a *App) handleJenkinsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case jenkinsLoadedMsg:
		if m.gen != a.jenkinsGen {
			return a, nil
		}
		a.jenkinsLoading = false
		a.jenkinsCfg = m.cfg
		a.jenkinsInfo = m.info
		a.jenkinsJobs = m.jobs
		a.jenkinsBuilds = m.builds
		a.jenkinsConsole = m.console
		a.jenkinsQueue = m.queue
		if m.err != "" {
			a.jenkinsErr = m.err
		} else {
			a.jenkinsErr = ""
		}
		if a.jenkinsCursor >= len(a.jenkinsJobs) {
			a.jenkinsCursor = maxInt(0, len(a.jenkinsJobs)-1)
		}
		if a.jenkinsBuildCursor >= len(a.jenkinsBuilds) {
			a.jenkinsBuildCursor = maxInt(0, len(a.jenkinsBuilds)-1)
		}
		if a.jenkinsOpen {
			return a, a.scheduleJenkinsTick()
		}
	case jenkinsActionMsg:
		if m.gen != a.jenkinsGen {
			return a, nil
		}
		a.jenkinsLoading = false
		if m.err != "" {
			a.jenkinsErr = m.err
			a.jenkinsStatus = ""
		} else {
			a.jenkinsErr = ""
			a.jenkinsStatus = truncate(m.out, 60)
		}
		return a, a.refreshJenkins(a.currentProject())
	case jenkinsTickMsg:
		if m.gen != a.jenkinsGen || !a.jenkinsOpen || a.jenkinsEditing {
			return a, nil
		}
		return a, a.refreshJenkins(a.currentProject())
	}
	return a, nil
}

func (a *App) jenkinsSelectedJobName() string {
	if a.jenkinsJobFocus != "" {
		return a.jenkinsJobFocus
	}
	if a.jenkinsCursor >= 0 && a.jenkinsCursor < len(a.jenkinsJobs) {
		return a.jenkinsJobs[a.jenkinsCursor].FullName
	}
	return ""
}

func (a *App) jenkinsSelectedBuildNum() int {
	if a.jenkinsBuildFocus > 0 {
		return a.jenkinsBuildFocus
	}
	if a.jenkinsBuildCursor >= 0 && a.jenkinsBuildCursor < len(a.jenkinsBuilds) {
		return a.jenkinsBuilds[a.jenkinsBuildCursor].Number
	}
	if a.jenkinsCursor >= 0 && a.jenkinsCursor < len(a.jenkinsJobs) {
		return a.jenkinsJobs[a.jenkinsCursor].LastBuild
	}
	return 0
}

func (a *App) jenkinsSelectedJob() (jenkinsutil.Job, bool) {
	if a.jenkinsCursor < 0 || a.jenkinsCursor >= len(a.jenkinsJobs) {
		return jenkinsutil.Job{}, false
	}
	return a.jenkinsJobs[a.jenkinsCursor], true
}

func (a *App) jenkinsSelectedBuild() (jenkinsutil.Build, bool) {
	if a.jenkinsBuildCursor < 0 || a.jenkinsBuildCursor >= len(a.jenkinsBuilds) {
		return jenkinsutil.Build{}, false
	}
	return a.jenkinsBuilds[a.jenkinsBuildCursor], true
}

func (a *App) renderJenkinsLanding(p *core.Project) string {
	w, h := a.moduleSize()
	cfg := jenkinsutil.LoadProject(p.Path)
	status := "offline"
	if cfg.Configured() {
		status = "configured"
	}
	ctx := a.renderModuleContext(p, w, "JENKINS", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(6, bodyH*35/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("CI/CD — pipelines, builds e console"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	if !cfg.Configured() {
		openLines = append(openLines, "", StyleWarning.Render("configure URL/user/token em Settings"))
	} else {
		openLines = append(openLines, "", StyleMuted.Render("server  ")+StyleNormal.Render(cfg.Host()))
		openLines = append(openLines, StyleMuted.Render("user    ")+StyleNormal.Render(cfg.User))
	}
	featLines := []string{
		StyleMuted.Render("overview · saúde do server"),
		StyleMuted.Render("pipelines · trigger / stop / console"),
		StyleMuted.Render("builds · logs com scroll"),
		StyleMuted.Render("config em .devscope/jenkins.json"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("JENKINS", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	details := []string{
		StyleMuted.Render("Config  ") + StyleNormal.Render(boolLabel(cfg.Configured())),
		StyleMuted.Render("Host    ") + StyleMuted.Render(firstNonEmpty(cfg.Host(), "—")),
		StyleMuted.Render("Auth    ") + StyleMuted.Render("Basic token"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir console"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderJenkinsTab(p *core.Project) string {
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
	if a.jenkinsBuildDetail {
		return lipgloss.JoinVertical(lipgloss.Left,
			a.renderJenkinsHeader(p, w),
			a.renderJenkinsBuildDetail(w, maxInt(10, h-4)),
			a.renderStatusBar(a.jenkinsHints()),
		)
	}
	header := a.renderJenkinsHeader(p, w)
	nav := a.renderJenkinsNav(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(nav)
	bodyH := maxInt(10, h-headerH-2)

	var body string
	switch a.jenkinsSubTab {
	case jenkinsTabOverview:
		body = a.renderJenkinsOverview(p, w, bodyH)
	case jenkinsTabBuilds:
		body = a.renderJenkinsBuildsView(p, w, bodyH)
	case jenkinsTabSettings:
		body = a.renderJenkinsSettings(p, w, bodyH)
	default:
		body = a.renderJenkinsPipelinesView(p, w, bodyH)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, nav, body, a.renderStatusBar(a.jenkinsHints()))
}

func (a *App) jenkinsHints() string {
	if a.jenkinsEditing {
		return "tab campo  ←→ cursor  enter salvar  t testar  esc cancelar"
	}
	if a.jenkinsBuildDetail {
		return "↑↓←→ scroll  pgup/pgdown  esc voltar  r refresh"
	}
	base := "0-3 aba  tab painel  r refresh  b build  x stop  enter logs  esc"
	switch a.jenkinsSubTab {
	case jenkinsTabOverview:
		base = "1 pipelines  2 builds  3 settings  r refresh  esc"
	case jenkinsTabSettings:
		base = "e editar  t testar  r refresh  esc"
	case jenkinsTabBuilds:
		base = "↑↓ build  enter logs  r rebuild  tab painel  esc"
	}
	if a.jenkinsLoading {
		base = "carregando…  " + base
	}
	if a.jenkinsStatus != "" {
		return truncate(a.jenkinsStatus, 36) + "  ·  " + base
	}
	if a.jenkinsErr != "" {
		return StyleUnhealthy.Render(truncate(a.jenkinsErr, 40)) + "  ·  " + base
	}
	return base
}

func (a *App) renderJenkinsHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabJenkins)).Bold(true)
	name := "project"
	if p != nil {
		name = p.Name
	}
	host := a.jenkinsCfg.Host()
	if host == "" {
		host = "—"
	}
	left := accent.Render("devscope") + StyleMuted.Render(" › jenkins") +
		StyleMuted.Render("  ") + StyleNormal.Render(host) +
		StyleMuted.Render("  Projeto: ") + StyleNormal.Render(name)

	badge := StyleMuted.Render("○ Offline")
	if a.jenkinsInfo.Connected {
		badge = StyleHealthy.Render("● Connected")
	}
	ver := a.jenkinsInfo.Version
	if ver == "" {
		ver = "—"
	}
	right := badge + StyleMuted.Render(fmt.Sprintf("  v%s  queue:%d  running:%d", ver, a.jenkinsQueue, a.jenkinsInfo.BusyExec))
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderJenkinsNav(width int) string {
	names := []string{"Overview", "Pipelines", "Builds", "Settings"}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf(" %d:%s ", i, n)
		if jenkinsSubTab(i) == a.jenkinsSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	line := strings.Join(parts, StyleMuted.Render("│"))
	pad := width - lipgloss.Width(stripANSI(line))
	if pad < 0 {
		pad = 0
	}
	return line + strings.Repeat(" ", pad)
}

func (a *App) renderJenkinsOverview(p *core.Project, width, height int) string {
	rightW := a.moduleRightWidth(width)
	centerW := maxInt(36, width-rightW-1)
	success, fail, running := 0, 0, 0
	for _, j := range a.jenkinsJobs {
		switch j.Status {
		case "success":
			success++
		case "failure":
			fail++
		case "running":
			running++
		}
	}
	chartH := maxInt(7, height*28/100)
	sumH := maxInt(7, height*32/100)
	listH := maxInt(5, height-chartH-sumH)
	lines := []string{
		StyleMuted.Render("Server     ") + jenkinsStatusLabel(a.jenkinsInfo.Connected),
		StyleMuted.Render("Version    ") + StyleNormal.Render(firstNonEmpty(a.jenkinsInfo.Version, "—")),
		StyleMuted.Render("Mode       ") + StyleNormal.Render(firstNonEmpty(a.jenkinsInfo.Mode, "—")),
		StyleMuted.Render("Node       ") + StyleNormal.Render(firstNonEmpty(a.jenkinsInfo.NodeName, "—")),
		StyleMuted.Render("User       ") + StyleNormal.Render(firstNonEmpty(a.jenkinsCfg.User, "—")),
		StyleMuted.Render("Jobs       ") + StyleNormal.Render(fmt.Sprintf("%d", len(a.jenkinsJobs))),
		StyleMuted.Render("Running    ") + StyleWarning.Render(fmt.Sprintf("%d", running)),
		StyleMuted.Render("Queue      ") + StyleNormal.Render(fmt.Sprintf("%d", a.jenkinsQueue)),
		StyleMuted.Render("Success    ") + StyleHealthy.Render(fmt.Sprintf("%d", success)) +
			StyleMuted.Render("  Fail ") + StyleUnhealthy.Render(fmt.Sprintf("%d", fail)),
	}
	evLines := make([]string, 0, listH-2)
	if len(a.jenkinsBuilds) == 0 {
		evLines = append(evLines, StyleMuted.Render("(sem builds recentes — abra Pipelines)"))
	} else {
		n := minInt(listH-2, len(a.jenkinsBuilds))
		for i := 0; i < n; i++ {
			b := a.jenkinsBuilds[i]
			evLines = append(evLines, jenkinsBuildRow(b, width/2))
		}
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderJenkinsActivityPane(centerW, chartH, false),
		renderApiTitledBox("OVERVIEW", fitExactLines(lines, sumH-2), centerW, sumH, false),
		renderApiTitledBox("RECENT BUILDS", fitExactLines(evLines, listH-2), centerW, listH, false),
	)
	details := []string{
		StyleHealthy.Render(fmt.Sprintf("ok     %d", success)),
		StyleUnhealthy.Render(fmt.Sprintf("fail   %d", fail)),
		StyleWarning.Render(fmt.Sprintf("run    %d", running)),
		StyleMuted.Render(fmt.Sprintf("queue  %d", a.jenkinsQueue)),
	}
	actions := moduleActionLines(
		[2]string{"1", "pipelines"},
		[2]string{"2", "builds"},
		[2]string{"r", "refresh"},
	)
	right := a.renderModuleRightRail(rightW, height, details, actions)
	return lipgloss.JoinHorizontal(lipgloss.Top, center, right)
}

func jenkinsStatusLabel(ok bool) string {
	if ok {
		return StyleHealthy.Render("● Connected")
	}
	return StyleMuted.Render("○ Offline")
}

func (a *App) renderJenkinsPipelinesView(p *core.Project, width, height int) string {
	if height < 10 {
		height = 10
	}
	leftW, rightW, centerW := jenkinsSplit(width)
	bottomH := height * 32 / 100
	if bottomH < 5 {
		bottomH = 5
	}
	if bottomH > height-6 {
		bottomH = height - 6
	}
	tableH := height - bottomH
	actW := centerW / 2
	logW := centerW - actW
	left := a.renderJenkinsSideNav(leftW, height)
	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderJenkinsJobTable(centerW, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderJenkinsActivityPane(actW, bottomH, true),
			a.renderJenkinsLogsPane(logW, bottomH),
		),
	)
	right := a.renderJenkinsInspector(p, rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (a *App) renderJenkinsBuildsView(p *core.Project, width, height int) string {
	if height < 10 {
		height = 10
	}
	leftW, rightW, centerW := jenkinsSplit(width)
	bottomH := height * 32 / 100
	if bottomH < 5 {
		bottomH = 5
	}
	if bottomH > height-6 {
		bottomH = height - 6
	}
	tableH := height - bottomH
	actW := centerW / 2
	logW := centerW - actW
	left := a.renderJenkinsSideNav(leftW, height)
	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderJenkinsBuildTable(centerW, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderJenkinsActivityPane(actW, bottomH, true),
			a.renderJenkinsLogsPane(logW, bottomH),
		),
	)
	right := a.renderJenkinsInspector(p, rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func jenkinsSplit(width int) (leftW, rightW, centerW int) {
	leftW = maxInt(16, width*14/100)
	if leftW > 22 {
		leftW = 22
	}
	rightW = maxInt(22, width*24/100)
	if rightW > 34 {
		rightW = 34
	}
	centerW = width - leftW - rightW
	if centerW < 28 {
		shrink := 28 - centerW
		take := shrink / 2
		leftW = maxInt(14, leftW-take)
		rightW = maxInt(18, rightW-(shrink-take))
		centerW = width - leftW - rightW
	}
	return
}

func (a *App) renderJenkinsSideNav(width, height int) string {
	focus := a.jenkinsFocus == jenkinsFocusNav
	statsH := height * 36 / 100
	if statsH < 5 {
		statsH = 5
	}
	if statsH > height-6 {
		statsH = height - 6
	}
	navH := height - statsH
	items := []string{"Overview", "Pipelines", "Builds", "Settings"}
	lines := make([]string, 0, navH-2)
	for i, name := range items {
		mark := "  "
		style := StyleMuted
		if jenkinsSubTab(i) == a.jenkinsSubTab {
			mark = "▸ "
			if focus {
				style = StyleSelected
			} else {
				style = StyleNormal
			}
		}
		badge := ""
		switch jenkinsSubTab(i) {
		case jenkinsTabPipelines:
			badge = fmt.Sprintf(" %d", len(a.jenkinsJobs))
		case jenkinsTabBuilds:
			badge = fmt.Sprintf(" %d", len(a.jenkinsBuilds))
		}
		lines = append(lines, style.Render(truncate(mark+name+badge, width-2)))
	}
	running := a.jenkinsInfo.BusyExec
	stats := []string{
		StyleHealthy.Render(truncate(fmt.Sprintf("Jobs %d", len(a.jenkinsJobs)), width-2)),
		StyleWarning.Render(truncate(fmt.Sprintf("Run  %d", running), width-2)),
		StyleMuted.Render(truncate(fmt.Sprintf("Q    %d", a.jenkinsQueue), width-2)),
	}
	title := "NAV"
	if focus {
		title = "> NAV"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox(title, fitExactLines(lines, navH-2), width, navH, focus),
		renderApiTitledBox("QUICK STATS", fitExactLines(stats, statsH-2), width, statsH, false),
	)
}

func (a *App) renderJenkinsJobTable(width, height int) string {
	focus := a.jenkinsFocus == jenkinsFocusTable
	n := len(a.jenkinsJobs)
	a.jenkinsScroll = ensureVisible(a.jenkinsCursor, a.jenkinsScroll, height-3, n)
	header := fmt.Sprintf("%-3s %-28s %-10s %-8s", "ST", "JOB", "STATUS", "LAST")
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum job — configure folder/URL)"))
	} else {
		start := a.jenkinsScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			j := a.jenkinsJobs[i]
			prefix := "  "
			style := StyleMuted
			if i == a.jenkinsCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			row := fmt.Sprintf("%s%-28s %-10s #%-7d",
				prefix,
				truncate(j.FullName, 28),
				truncate(j.Status, 10),
				j.LastBuild,
			)
			lines = append(lines, jenkinsStatusDot(j.Status)+" "+style.Render(truncate(row, width-4)))
		}
	}
	title := "PIPELINES"
	if focus {
		title = "> PIPELINES"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderJenkinsBuildTable(width, height int) string {
	focus := a.jenkinsFocus == jenkinsFocusTable
	n := len(a.jenkinsBuilds)
	a.jenkinsBuildScroll = ensureVisible(a.jenkinsBuildCursor, a.jenkinsBuildScroll, height-3, n)
	job := a.jenkinsSelectedJobName()
	header := fmt.Sprintf("%-3s %-8s %-10s %-10s %s", "ST", "BUILD", "RESULT", "DURATION", "WHEN")
	lines := []string{
		StyleMuted.Render(truncate("job: "+firstNonEmpty(job, "—"), width-2)),
		StyleMuted.Render(truncate(header, width-2)),
	}
	if n == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum build)"))
	} else {
		start := a.jenkinsBuildScroll
		end := minInt(start+height-4, n)
		for i := start; i < end; i++ {
			b := a.jenkinsBuilds[i]
			st := jenkinsutil.BuildStatus(b)
			prefix := "  "
			style := StyleMuted
			if i == a.jenkinsBuildCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			row := fmt.Sprintf("%s#%-7d %-10s %-10s %s",
				prefix,
				b.Number,
				truncate(st, 10),
				jenkinsutil.FormatDuration(b.Duration),
				jenkinsutil.FormatAgo(b.Timestamp),
			)
			lines = append(lines, jenkinsStatusDot(st)+" "+style.Render(truncate(row, width-4)))
		}
	}
	title := "BUILDS"
	if focus {
		title = "> BUILDS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

// renderJenkinsActivityPane draws a duration sparkline + status row for recent builds
// (Jenkins equivalent of Ngrok's live-requests visual).
func (a *App) renderJenkinsActivityPane(width, height int, compact bool) string {
	innerW := maxInt(8, width-2)
	viewH := maxInt(1, height-2)
	builds := a.jenkinsBuildsForChart(innerW)
	lines := make([]string, 0, viewH)

	if len(builds) == 0 {
		lines = append(lines,
			StyleMuted.Render(strings.Repeat("·", minInt(innerW, 24))),
			StyleMuted.Render("(sem builds para gráfico)"),
		)
	} else {
		bars := jenkinsDurationSparkline(builds)
		status := jenkinsStatusSparkline(builds)
		lines = append(lines,
			StyleMuted.Render("DUR  ")+bars,
			StyleMuted.Render("ST   ")+status,
		)
		ok, fail, run := 0, 0, 0
		var maxDur int64
		for _, b := range builds {
			switch jenkinsutil.BuildStatus(b) {
			case "success":
				ok++
			case "failure":
				fail++
			case "running":
				run++
			}
			if b.Duration > maxDur {
				maxDur = b.Duration
			}
		}
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("n=%d  ", len(builds)))+
			StyleHealthy.Render(fmt.Sprintf("ok %d ", ok))+
			StyleUnhealthy.Render(fmt.Sprintf("fail %d ", fail))+
			StyleWarning.Render(fmt.Sprintf("run %d", run)))
		if !compact && maxDur > 0 {
			lines = append(lines, StyleMuted.Render("pico ")+StyleNormal.Render(jenkinsutil.FormatDuration(maxDur))+
				StyleMuted.Render("  (esq→dir cronológico)"))
		}
		// mini histogram rows when there's vertical room
		if viewH >= 6 {
			hist := jenkinsDurationBars(builds, minInt(3, viewH-4))
			lines = append(lines, hist...)
		}
	}

	title := "ACTIVITY"
	return renderApiTitledBox(title, fitExactLines(lines, viewH), width, height, false)
}

func (a *App) jenkinsBuildsForChart(maxBars int) []jenkinsutil.Build {
	if maxBars < 4 {
		maxBars = 4
	}
	// leave room for "DUR  " / "ST   " prefix
	n := maxBars - 5
	if n < 4 {
		n = 4
	}
	src := a.jenkinsBuilds
	if len(src) == 0 {
		return nil
	}
	if len(src) > n {
		src = src[:n]
	}
	// API returns newest-first; chart reads left→right chronological
	out := make([]jenkinsutil.Build, len(src))
	for i := range src {
		out[len(src)-1-i] = src[i]
	}
	return out
}

func jenkinsDurationSparkline(builds []jenkinsutil.Build) string {
	glyphs := []rune("▁▂▃▄▅▆▇█")
	var max int64
	for _, b := range builds {
		d := b.Duration
		if b.Building && d <= 0 {
			d = 1
		}
		if d > max {
			max = d
		}
	}
	if max <= 0 {
		max = 1
	}
	var b strings.Builder
	for _, build := range builds {
		d := build.Duration
		if build.Building && d <= 0 {
			d = max / 2
		}
		idx := int(float64(d) / float64(max) * float64(len(glyphs)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(glyphs) {
			idx = len(glyphs) - 1
		}
		st := jenkinsutil.BuildStatus(build)
		ch := string(glyphs[idx])
		switch st {
		case "success":
			b.WriteString(StyleHealthy.Render(ch))
		case "failure":
			b.WriteString(StyleUnhealthy.Render(ch))
		case "running", "unstable":
			b.WriteString(StyleWarning.Render(ch))
		default:
			b.WriteString(StyleMuted.Render(ch))
		}
	}
	return b.String()
}

func jenkinsStatusSparkline(builds []jenkinsutil.Build) string {
	var b strings.Builder
	for _, build := range builds {
		st := jenkinsutil.BuildStatus(build)
		ch := "●"
		switch st {
		case "success":
			b.WriteString(StyleHealthy.Render(ch))
		case "failure":
			b.WriteString(StyleUnhealthy.Render(ch))
		case "running", "unstable":
			b.WriteString(StyleWarning.Render(ch))
		default:
			b.WriteString(StyleMuted.Render("○"))
		}
	}
	return b.String()
}

func jenkinsDurationBars(builds []jenkinsutil.Build, rows int) []string {
	if rows < 1 || len(builds) == 0 {
		return nil
	}
	var max int64
	for _, b := range builds {
		if b.Duration > max {
			max = b.Duration
		}
	}
	if max <= 0 {
		max = 1
	}
	out := make([]string, 0, rows)
	for r := rows - 1; r >= 0; r-- {
		threshold := float64(r+1) / float64(rows)
		var line strings.Builder
		line.WriteString(StyleMuted.Render("     "))
		for _, build := range builds {
			pct := float64(build.Duration) / float64(max)
			if build.Building && build.Duration <= 0 {
				pct = 0.5
			}
			if pct >= threshold-1e-9 {
				st := jenkinsutil.BuildStatus(build)
				switch st {
				case "success":
					line.WriteString(StyleHealthy.Render("█"))
				case "failure":
					line.WriteString(StyleUnhealthy.Render("█"))
				case "running", "unstable":
					line.WriteString(StyleWarning.Render("█"))
				default:
					line.WriteString(StyleMuted.Render("█"))
				}
			} else {
				line.WriteString(StyleMuted.Render(" "))
			}
		}
		out = append(out, line.String())
	}
	return out
}

func (a *App) renderJenkinsLogsPane(width, height int) string {
	focus := a.jenkinsFocus == jenkinsFocusLogs
	raw := strings.Split(strings.ReplaceAll(a.jenkinsConsole, "\r\n", "\n"), "\n")
	if len(raw) == 1 && raw[0] == "" {
		raw = []string{"(sem console — selecione um build)"}
	}
	viewH := maxInt(1, height-2)
	maxScroll := maxInt(0, len(raw)-viewH)
	if a.jenkinsLogScroll > maxScroll {
		a.jenkinsLogScroll = maxScroll
	}
	if a.jenkinsLogScroll < 0 {
		a.jenkinsLogScroll = 0
	}
	start := a.jenkinsLogScroll
	end := minInt(start+viewH, len(raw))
	lines := make([]string, 0, viewH)
	for i := start; i < end; i++ {
		line := raw[i]
		if a.jenkinsLogHScroll > 0 && a.jenkinsLogHScroll < len(line) {
			line = line[a.jenkinsLogHScroll:]
		} else if a.jenkinsLogHScroll >= len(line) {
			line = ""
		}
		lines = append(lines, StyleMuted.Render(truncate(line, width-2)))
	}
	title := "LOGS"
	if focus {
		title = "> LOGS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewH), width, height, focus)
}

func (a *App) renderJenkinsInspector(p *core.Project, width, height int) string {
	focus := a.jenkinsFocus == jenkinsFocusDetail
	detH := height * 55 / 100
	if detH < 6 {
		detH = 6
	}
	if detH > height-5 {
		detH = height - 5
	}
	actH := height - detH
	details := []string{StyleMuted.Render("(nada selecionado)")}
	if a.jenkinsSubTab == jenkinsTabBuilds {
		if b, ok := a.jenkinsSelectedBuild(); ok {
			st := jenkinsutil.BuildStatus(b)
			details = []string{
				StyleMuted.Render("Build   ") + StyleNormal.Render(fmt.Sprintf("#%d", b.Number)),
				StyleMuted.Render("Status  ") + jenkinsStatusStyled(st),
				StyleMuted.Render("Dur     ") + StyleNormal.Render(jenkinsutil.FormatDuration(b.Duration)),
				StyleMuted.Render("When    ") + StyleMuted.Render(jenkinsutil.FormatAgo(b.Timestamp)),
				StyleMuted.Render("Job     ") + StyleMuted.Render(truncate(b.FullName, width-12)),
			}
		}
	} else if j, ok := a.jenkinsSelectedJob(); ok {
		details = []string{
			StyleMuted.Render("Job     ") + StyleNormal.Render(truncate(j.Name, width-12)),
			StyleMuted.Render("Full    ") + StyleMuted.Render(truncate(j.FullName, width-12)),
			StyleMuted.Render("Status  ") + jenkinsStatusStyled(j.Status),
			StyleMuted.Render("Last    ") + StyleNormal.Render(fmt.Sprintf("#%d", j.LastBuild)),
			StyleMuted.Render("Queue   ") + StyleNormal.Render(boolLabel(j.InQueue)),
		}
		if j.Description != "" {
			details = append(details, StyleMuted.Render(truncate(j.Description, width-2)))
		}
	}
	actions := moduleActionLines(
		[2]string{"b", "trigger"},
		[2]string{"x", "stop last"},
		[2]string{"enter", "ver logs"},
		[2]string{"r", "refresh"},
	)
	title := "DETAILS"
	if focus {
		title = "> DETAILS"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox(title, fitExactLines(details, detH-2), width, detH, focus),
		renderApiTitledBox("AÇÕES", fitExactLines(actions, actH-2), width, actH, false),
	)
}

func (a *App) renderJenkinsSettings(p *core.Project, width, height int) string {
	rightW := a.moduleRightWidth(width)
	centerW := maxInt(36, width-rightW-1)
	tokenShown := jenkinsutil.MaskToken(a.jenkinsEditToken)
	if a.jenkinsEditing && a.jenkinsSetField == jenkinsSetToken {
		tokenShown = a.jenkinsEditToken
	}
	lines := []string{
		a.renderJenkinsSetField("URL", a.jenkinsEditURL, jenkinsSetURL),
		a.renderJenkinsSetField("User", a.jenkinsEditUser, jenkinsSetUser),
		a.renderJenkinsSetField("Token", tokenShown, jenkinsSetToken),
		a.renderJenkinsSetField("Folder", a.jenkinsEditFolder, jenkinsSetFolder),
		a.renderJenkinsSetField("Refresh", a.jenkinsEditRefresh+"s", jenkinsSetRefresh),
		"",
		StyleMuted.Render("arquivo  ") + StyleMuted.Render(".devscope/jenkins.json"),
	}
	if p != nil {
		lines = append(lines, StyleMuted.Render("path     ")+StyleMuted.Render(truncate(p.Path, width-18)))
	}
	if !a.jenkinsEditing {
		lines = append(lines, "", StyleMuted.Render("e editar · t testar conexão · enter salvar após editar"))
	}
	center := renderApiTitledBox("SETTINGS", fitExactLines(lines, height-2), centerW, height, a.jenkinsEditing)
	details := []string{
		StyleMuted.Render("Config  ") + StyleNormal.Render(boolLabel(a.jenkinsCfg.Configured())),
		StyleMuted.Render("Host    ") + StyleMuted.Render(firstNonEmpty(a.jenkinsCfg.Host(), "—")),
		jenkinsStatusLabel(a.jenkinsInfo.Connected),
	}
	actions := moduleActionLines(
		[2]string{"e", "editar"},
		[2]string{"t", "testar"},
		[2]string{"enter", "salvar"},
	)
	right := a.renderModuleRightRail(rightW, height, details, actions)
	return lipgloss.JoinHorizontal(lipgloss.Top, center, right)
}

func (a *App) renderJenkinsSetField(label, value string, field int) string {
	prefix := StyleMuted.Render(fmt.Sprintf("%-9s ", label))
	if !a.jenkinsEditing || a.jenkinsSetField != field {
		return prefix + StyleNormal.Render(value)
	}
	runes := []rune(a.jenkinsSetText())
	cur := a.jenkinsSetCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	if field == jenkinsSetToken && a.jenkinsSetField == field {
		// show raw while editing
	} else if field == jenkinsSetRefresh {
		shown = string(runes[:cur]) + "█" + string(runes[cur:])
	}
	return prefix + StyleSelected.Render(shown)
}

func (a *App) renderJenkinsBuildDetail(width, height int) string {
	job := a.jenkinsSelectedJobName()
	num := a.jenkinsSelectedBuildNum()
	title := fmt.Sprintf("BUILD LOG  %s #%d", job, num)
	raw := strings.Split(strings.ReplaceAll(a.jenkinsConsole, "\r\n", "\n"), "\n")
	if len(raw) == 1 && raw[0] == "" {
		raw = []string{"(console vazio)"}
	}
	viewH := maxInt(1, height-2)
	maxScroll := maxInt(0, len(raw)-viewH)
	if a.jenkinsLogScroll > maxScroll {
		a.jenkinsLogScroll = maxScroll
	}
	start := a.jenkinsLogScroll
	end := minInt(start+viewH, len(raw))
	lines := make([]string, 0, viewH)
	for i := start; i < end; i++ {
		line := raw[i]
		if a.jenkinsLogHScroll > 0 && a.jenkinsLogHScroll < len([]rune(line)) {
			r := []rune(line)
			line = string(r[a.jenkinsLogHScroll:])
		} else if a.jenkinsLogHScroll >= len([]rune(line)) {
			line = ""
		}
		lines = append(lines, StyleMuted.Render(truncate(line, width-2)))
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewH), width, height, true)
}

func (a *App) handleJenkinsKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.jenkinsEditing {
		return a.updateJenkinsSettings(msg, p)
	}
	if a.jenkinsBuildDetail {
		return a.handleJenkinsBuildDetailKeys(msg, p)
	}

	switch msg.String() {
	case "esc":
		return a, a.leaveJenkinsTab()
	case "tab":
		a.jenkinsFocus = (a.jenkinsFocus + 1) % 4
	case "0":
		a.jenkinsSubTab = jenkinsTabOverview
	case "1", "p":
		a.jenkinsSubTab = jenkinsTabPipelines
		a.jenkinsFocus = jenkinsFocusTable
	case "2":
		a.jenkinsSubTab = jenkinsTabBuilds
		a.jenkinsFocus = jenkinsFocusTable
	case "3", "s":
		a.jenkinsSubTab = jenkinsTabSettings
	case "up", "k":
		return a, a.jenkinsMove(-1)
	case "down", "j":
		return a, a.jenkinsMove(1)
	case "pgup":
		a.jenkinsLogScroll = maxInt(0, a.jenkinsLogScroll-5)
	case "pgdown":
		a.jenkinsLogScroll += 5
	case "l":
		a.jenkinsFocus = jenkinsFocusLogs
	case "r":
		if a.jenkinsSubTab == jenkinsTabBuilds {
			return a, a.jenkinsRebuild(p)
		}
		return a, a.refreshJenkins(p)
	case "b":
		return a, a.jenkinsTrigger(p)
	case "x":
		return a, a.jenkinsStop(p)
	case "e":
		if a.jenkinsSubTab == jenkinsTabSettings {
			a.jenkinsEditing = true
			a.loadJenkinsSettingsDraft(p)
			return a, nil
		}
	case "t":
		if a.jenkinsSubTab == jenkinsTabSettings {
			return a, a.jenkinsTestConnection(p)
		}
	case "enter":
		if a.jenkinsSubTab == jenkinsTabSettings {
			a.jenkinsEditing = true
			a.loadJenkinsSettingsDraft(p)
			return a, nil
		}
		if a.jenkinsSubTab == jenkinsTabPipelines {
			if j, ok := a.jenkinsSelectedJob(); ok {
				a.jenkinsJobFocus = j.FullName
				a.jenkinsBuildFocus = j.LastBuild
				a.jenkinsSubTab = jenkinsTabBuilds
				a.jenkinsBuildCursor = 0
				a.jenkinsFocus = jenkinsFocusTable
				return a, a.refreshJenkins(p)
			}
		}
		if a.jenkinsSubTab == jenkinsTabBuilds {
			if b, ok := a.jenkinsSelectedBuild(); ok {
				a.jenkinsBuildFocus = b.Number
				a.jenkinsJobFocus = b.FullName
				if a.jenkinsJobFocus == "" {
					a.jenkinsJobFocus = a.jenkinsSelectedJobName()
				}
				a.jenkinsBuildDetail = true
				a.jenkinsLogScroll = 0
				a.jenkinsLogHScroll = 0
				return a, a.refreshJenkins(p)
			}
		}
	}
	return a, nil
}

func (a *App) handleJenkinsBuildDetailKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.jenkinsBuildDetail = false
		a.jenkinsSubTab = jenkinsTabBuilds
		return a, nil
	case "up", "k":
		a.jenkinsLogScroll = maxInt(0, a.jenkinsLogScroll-1)
	case "down", "j":
		a.jenkinsLogScroll++
	case "left", "h":
		a.jenkinsLogHScroll = maxInt(0, a.jenkinsLogHScroll-4)
	case "right", "l":
		a.jenkinsLogHScroll += 4
	case "pgup":
		a.jenkinsLogScroll = maxInt(0, a.jenkinsLogScroll-maxInt(4, a.height-8))
	case "pgdown":
		a.jenkinsLogScroll += maxInt(4, a.height-8)
	case "home":
		a.jenkinsLogScroll = 0
	case "r":
		return a, a.refreshJenkins(p)
	}
	return a, nil
}

func (a *App) jenkinsMove(delta int) tea.Cmd {
	if a.jenkinsFocus == jenkinsFocusNav {
		next := int(a.jenkinsSubTab) + delta
		if next < 0 {
			next = int(jenkinsTabSettings)
		}
		if next > int(jenkinsTabSettings) {
			next = 0
		}
		a.jenkinsSubTab = jenkinsSubTab(next)
		return nil
	}
	if a.jenkinsFocus == jenkinsFocusLogs {
		a.jenkinsLogScroll = maxInt(0, a.jenkinsLogScroll+delta)
		return nil
	}
	if a.jenkinsSubTab == jenkinsTabBuilds {
		n := len(a.jenkinsBuilds)
		if n == 0 {
			return nil
		}
		a.jenkinsBuildCursor = clampInt(a.jenkinsBuildCursor+delta, 0, n-1)
		if b, ok := a.jenkinsSelectedBuild(); ok {
			a.jenkinsBuildFocus = b.Number
			a.jenkinsJobFocus = b.FullName
		}
		return a.refreshJenkins(a.currentProject())
	}
	n := len(a.jenkinsJobs)
	if n == 0 {
		return nil
	}
	a.jenkinsCursor = clampInt(a.jenkinsCursor+delta, 0, n-1)
	if j, ok := a.jenkinsSelectedJob(); ok {
		a.jenkinsJobFocus = j.FullName
		a.jenkinsBuildFocus = j.LastBuild
	}
	return a.refreshJenkins(a.currentProject())
}

func (a *App) jenkinsTrigger(p *core.Project) tea.Cmd {
	job := a.jenkinsSelectedJobName()
	if job == "" || p == nil {
		a.jenkinsStatus = "nenhum job selecionado"
		return nil
	}
	path := p.Path
	gen := a.jenkinsGen
	a.jenkinsLoading = true
	return func() tea.Msg {
		c := jenkinsutil.NewClient(jenkinsutil.LoadProject(path))
		if err := c.TriggerBuild(job); err != nil {
			return jenkinsActionMsg{err: err.Error(), gen: gen}
		}
		return jenkinsActionMsg{out: "build disparado: " + job, gen: gen}
	}
}

func (a *App) jenkinsStop(p *core.Project) tea.Cmd {
	job := a.jenkinsSelectedJobName()
	num := a.jenkinsSelectedBuildNum()
	if job == "" || num <= 0 || p == nil {
		a.jenkinsStatus = "nenhum build para parar"
		return nil
	}
	path := p.Path
	gen := a.jenkinsGen
	a.jenkinsLoading = true
	return func() tea.Msg {
		c := jenkinsutil.NewClient(jenkinsutil.LoadProject(path))
		if err := c.StopBuild(job, num); err != nil {
			return jenkinsActionMsg{err: err.Error(), gen: gen}
		}
		return jenkinsActionMsg{out: fmt.Sprintf("stop #%d %s", num, job), gen: gen}
	}
}

func (a *App) jenkinsRebuild(p *core.Project) tea.Cmd {
	return a.jenkinsTrigger(p)
}

func (a *App) jenkinsTestConnection(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	cfg := jenkinsutil.ProjectConfig{
		URL:   a.jenkinsEditURL,
		User:  a.jenkinsEditUser,
		Token: a.jenkinsEditToken,
	}
	gen := a.jenkinsGen
	return func() tea.Msg {
		info := jenkinsutil.NewClient(cfg).Ping()
		if info.Err != "" {
			return jenkinsActionMsg{err: info.Err, gen: gen}
		}
		return jenkinsActionMsg{out: "ok " + info.Version, gen: gen}
	}
}

func (a *App) updateJenkinsSettings(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.jenkinsEditing = false
		a.loadJenkinsSettingsDraft(p)
		return a, nil
	case "tab", "down", "j":
		a.jenkinsSetFocusField(a.jenkinsSetField + 1)
		return a, nil
	case "shift+tab", "up", "k":
		a.jenkinsSetFocusField(a.jenkinsSetField - 1)
		return a, nil
	case "left", "h":
		if a.jenkinsSetCursor > 0 {
			a.jenkinsSetCursor--
		}
		return a, nil
	case "right", "l":
		if a.jenkinsSetCursor < len([]rune(a.jenkinsSetText())) {
			a.jenkinsSetCursor++
		}
		return a, nil
	case "home":
		a.jenkinsSetCursor = 0
		return a, nil
	case "end":
		a.jenkinsSetCursor = len([]rune(a.jenkinsSetText()))
		return a, nil
	case "backspace":
		runes := []rune(a.jenkinsSetText())
		if a.jenkinsSetCursor > 0 && len(runes) > 0 {
			runes = append(runes[:a.jenkinsSetCursor-1], runes[a.jenkinsSetCursor:]...)
			a.setJenkinsSetText(string(runes))
			a.jenkinsSetCursor--
		}
		return a, nil
	case "delete":
		runes := []rune(a.jenkinsSetText())
		if a.jenkinsSetCursor < len(runes) {
			runes = append(runes[:a.jenkinsSetCursor], runes[a.jenkinsSetCursor+1:]...)
			a.setJenkinsSetText(string(runes))
		}
		return a, nil
	case "enter":
		return a, a.saveJenkinsSettings(p)
	case "t":
		return a, a.jenkinsTestConnection(p)
	default:
		if msg.Type == tea.KeyRunes {
			ch := string(msg.Runes)
			if a.jenkinsSetField == jenkinsSetRefresh {
				for _, r := range msg.Runes {
					if r < '0' || r > '9' {
						return a, nil
					}
				}
			}
			runes := []rune(a.jenkinsSetText())
			runes = append(runes[:a.jenkinsSetCursor], append([]rune(ch), runes[a.jenkinsSetCursor:]...)...)
			a.setJenkinsSetText(string(runes))
			a.jenkinsSetCursor += len([]rune(ch))
		}
	}
	return a, nil
}

func (a *App) jenkinsSetText() string {
	switch a.jenkinsSetField {
	case jenkinsSetURL:
		return a.jenkinsEditURL
	case jenkinsSetUser:
		return a.jenkinsEditUser
	case jenkinsSetToken:
		return a.jenkinsEditToken
	case jenkinsSetFolder:
		return a.jenkinsEditFolder
	case jenkinsSetRefresh:
		return a.jenkinsEditRefresh
	default:
		return ""
	}
}

func (a *App) setJenkinsSetText(s string) {
	switch a.jenkinsSetField {
	case jenkinsSetURL:
		a.jenkinsEditURL = s
	case jenkinsSetUser:
		a.jenkinsEditUser = s
	case jenkinsSetToken:
		a.jenkinsEditToken = s
	case jenkinsSetFolder:
		a.jenkinsEditFolder = s
	case jenkinsSetRefresh:
		a.jenkinsEditRefresh = s
	}
}

func (a *App) jenkinsSetFocusField(field int) {
	if field < jenkinsSetURL {
		field = jenkinsSetRefresh
	}
	if field > jenkinsSetRefresh {
		field = jenkinsSetURL
	}
	a.jenkinsSetField = field
	a.jenkinsSetCursor = len([]rune(a.jenkinsSetText()))
}

func (a *App) saveJenkinsSettings(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	sec, _ := strconv.Atoi(a.jenkinsEditRefresh)
	if sec <= 0 {
		sec = 5
	}
	cfg := jenkinsutil.ProjectConfig{
		URL:        a.jenkinsEditURL,
		User:       a.jenkinsEditUser,
		Token:      a.jenkinsEditToken,
		Folder:     a.jenkinsEditFolder,
		RefreshSec: sec,
	}
	if err := jenkinsutil.SaveProject(p.Path, cfg); err != nil {
		a.jenkinsErr = err.Error()
		return nil
	}
	a.jenkinsCfg = cfg
	a.jenkinsEditing = false
	a.jenkinsStatus = "config salva"
	return a.refreshJenkins(p)
}

func jenkinsStatusDot(status string) string {
	switch status {
	case "success":
		return StyleHealthy.Render("●")
	case "failure":
		return StyleUnhealthy.Render("●")
	case "running":
		return StyleWarning.Render("●")
	case "unstable":
		return StyleWarning.Render("●")
	default:
		return StyleMuted.Render("○")
	}
}

func jenkinsStatusStyled(status string) string {
	switch status {
	case "success":
		return StyleHealthy.Render(status)
	case "failure":
		return StyleUnhealthy.Render(status)
	case "running", "unstable":
		return StyleWarning.Render(status)
	default:
		return StyleMuted.Render(status)
	}
}

func jenkinsBuildRow(b jenkinsutil.Build, maxW int) string {
	st := jenkinsutil.BuildStatus(b)
	return jenkinsStatusDot(st) + " " + StyleMuted.Render(fmt.Sprintf("#%-5d %-8s %s",
		b.Number, truncate(st, 8), jenkinsutil.FormatAgo(b.Timestamp)))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
