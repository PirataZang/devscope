package collectors

import (
	"net"
	"os"
	"runtime"
	"testing"
)

func TestProcAddr(t *testing.T) {
	for hex, want := range map[string]string{
		"00000000": "*", "0100007F": "127.0.0.1", "0101A8C0": "192.168.1.1",
		"00000000000000000000000000000000": "*", "00000000000000000000000001000000": "::1",
	} {
		if got := procAddr(hex); got != want {
			t.Errorf("%s: %q, queria %q", hex, got, want)
		}
	}
}

func TestParseLsofListeners(t *testing.T) {
	raw := "p100\ncnode\nn*:3000\nn[::1]:3001\np200\ncpostgres\nn127.0.0.1:5432\n"
	got := parseLsofListeners(raw)
	if len(got) != 3 || got[0].PID != 100 || got[0].Addr != "*" || got[1].Addr != "::1" ||
		got[2].Process != "postgres" || got[2].Port != 5432 {
		t.Fatalf("%+v", got)
	}
}

func TestDedupListenersMergesIPv4AndIPv6(t *testing.T) {
	got := dedupListeners([]Listener{
		{Port: 8080, Addr: "127.0.0.1", PID: 2, Process: "docker-proxy"},
		{Port: 3000, Addr: "*", PID: 1, Process: "node"},
		{Port: 8080, Addr: "*", PID: 3, Process: "docker-proxy"},
	})
	if len(got) != 2 || got[0].Port != 3000 || got[1].Addr != "*" {
		t.Fatalf("%+v", got)
	}
}

// Abre uma porta de verdade e confere que o DevScope acha o próprio processo
// nela — com pasta de trabalho, que é o que casa com o projeto.
func TestListListenersFindsOwnProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("leitura via /proc")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	wd, _ := os.Getwd()
	for _, l := range ListListeners() {
		if l.Port == port {
			if l.PID != os.Getpid() || l.Addr != "127.0.0.1" || l.Cwd != wd {
				t.Fatalf("dono errado: %+v", l)
			}
			return
		}
	}
	t.Fatalf("porta %d não apareceu", port)
}
