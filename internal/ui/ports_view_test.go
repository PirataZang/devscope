package ui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func portsSnapshot() core.Snapshot {
	return core.Snapshot{
		Projects: []core.Project{
			{Name: "api", Path: "/p/api", Ports: []int{9000}, PortConflicts: map[int][]string{9000: {"web"}},
				Containers: []core.Container{{Name: "api-db-1", Ports: "0.0.0.0:5432->5432/tcp"}}},
			{Name: "web", Path: "/p/web"},
			{Name: "web-admin", Path: "/p/web/admin"},
		},
		OrphanContainers: []core.Container{
			{Name: "lopis_redis", Ports: "0.0.0.0:6379->6379/tcp", ProjectPath: "/outros/lopis"},
			{Name: "solto", Ports: "0.0.0.0:7000->7000/tcp"},
		},
	}
}

// Cada porta vai para o dono certo, e a lista sai na ordem dos grupos:
// projetos, composes fora da lista, containers soltos, programas, sistema.
func TestMatchPortOwners(t *testing.T) {
	rows := matchPortOwners([]collectors.Listener{
		{Port: 6000, Process: "x", PID: 0},
		{Port: 1716, Process: "kdeconnectd", PID: 9, Cwd: "/home"},
		{Port: 7000, Process: "docker-proxy", PID: 13},
		{Port: 6379, Process: "docker-proxy", PID: 14},
		{Port: 5432, Process: "docker-proxy", PID: 10},
		{Port: 3000, Process: "node", PID: 11, Cwd: "/p/web/admin/src"},
		{Port: 9000, Process: "php", PID: 12, Cwd: "/tmp"},
	}, portsSnapshot())
	var got []string
	for _, r := range rows {
		got = append(got, strings.TrimSpace(strings.Join([]string{
			":" + strconv.Itoa(r.Port), r.project, r.container, strings.Join(r.conflict, ",")}, " ")))
	}
	want := []string{
		":5432 api api-db-1", ":9000 api  web", // declarada no compose de api, disputada com web
		":3000 web-admin", // pasta do processo, o projeto mais específico
		":6379 lopis lopis_redis",
		":7000  solto",
		":1716", ":6000",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("donos:\n%s\nqueria:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPortsViewGroupsByProject(t *testing.T) {
	a := responsiveApp(140, 40)
	a.snapshot = portsSnapshot()
	a.dashboardSubview = dashboardSubviewPorts
	a.handlePortsLoaded(portsLoadedMsg{list: []collectors.Listener{
		{Port: 5432, Addr: "*", Process: "docker-proxy", PID: 10},
		{Port: 9000, Addr: "127.0.0.1", Process: "php", PID: 12, Cwd: "/tmp", Cmdline: "php artisan serve"},
		{Port: 6379, Addr: "*", Process: "docker-proxy", PID: 14},
		{Port: 6000, Addr: "*"},
	}})
	got := stripANSI(a.View())
	for _, want := range []string{
		"4 abertas", "3 em projetos", "3 na rede", "⚠ 1 disputada",
		"api  2 portas · /p/api", "lopis  1 porta · compose fora da lista de projetos",
		"SISTEMA · outro usuário", "▣ :5432", "api-db-1", "local", "⚠ também em web",
		"projeto  api", "x encerrar",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
}

func TestPortsViewKeys(t *testing.T) {
	a := responsiveApp(140, 40)
	a.snapshot = portsSnapshot()
	a.dashboardSubview = dashboardSubviewPorts
	a.handlePortsLoaded(portsLoadedMsg{list: []collectors.Listener{
		{Port: 5432, Addr: "*", Process: "docker-proxy", PID: 10},
		{Port: 6379, Addr: "*", Process: "docker-proxy", PID: 14},
		{Port: 6000, Addr: "*"},
	}})
	press := func(k tea.KeyMsg) { a.updateDashboard(k) }

	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !a.portsConfirm || !strings.Contains(stripANSI(a.View()), "docker stop") {
		t.Fatal("x no container confirma com docker stop")
	}
	press(tea.KeyMsg{Type: tea.KeyEsc})

	press(tea.KeyMsg{Type: tea.KeyDown})
	press(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(a.portsStatus, "lista de projetos") {
		t.Fatalf("compose fora da lista não abre projeto, explica: %q", a.portsStatus)
	}

	press(tea.KeyMsg{Type: tea.KeyDown})
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.portsConfirm || !strings.Contains(a.portsStatus, "outro usuário") {
		t.Fatalf("processo de outro usuário não se encerra daqui: %q", a.portsStatus)
	}
	press(tea.KeyMsg{Type: tea.KeyEsc})
	if a.dashboardSubview != dashboardSubviewList {
		t.Fatal("esc volta para a lista de projetos")
	}
}
