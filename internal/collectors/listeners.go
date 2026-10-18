package collectors

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// Listener é uma porta TCP aberta para conexão e quem está nela.
type Listener struct {
	Port    int
	Addr    string // "*" = todas as interfaces; "127.0.0.1"/"::1" = só local
	PID     int    // 0 = processo de outro usuário: o kernel não deixa ver
	Process string
	Cmdline string
	Cwd     string // casa o processo com o projeto pela pasta de trabalho
}

// ListListeners responde "quem está usando a porta?". No Linux lê /proc
// direto (nenhum processo criado); onde não há /proc (macOS), cai no lsof.
func ListListeners() []Listener {
	var out []Listener
	if _, err := os.Stat("/proc/net/tcp"); err == nil {
		out = procListeners()
	} else {
		out = lsofListeners()
	}
	return dedupListeners(out)
}

// dedupListeners junta o mesmo serviço escutando em IPv4 e IPv6 (docker-proxy
// sobe um processo para cada) numa linha só.
func dedupListeners(in []Listener) []Listener {
	seen := map[string]int{}
	var out []Listener
	for _, l := range in {
		key := strconv.Itoa(l.Port) + " " + l.Process
		if i, ok := seen[key]; ok {
			if l.Addr == "*" {
				out[i].Addr = "*"
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func procListeners() []Listener {
	byInode := map[string]Listener{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		parseProcNetListen(f, byInode)
	}
	if len(byInode) == 0 {
		return nil
	}
	owners := socketOwners(byInode)
	out := make([]Listener, 0, len(byInode))
	for inode, l := range byInode {
		if pid, ok := owners[inode]; ok {
			l.PID = pid
			l.Process = readTrim(fmt.Sprintf("/proc/%d/comm", pid))
			l.Cmdline = strings.TrimSpace(strings.ReplaceAll(readTrim(fmt.Sprintf("/proc/%d/cmdline", pid)), "\x00", " "))
			l.Cwd, _ = os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
		}
		out = append(out, l)
	}
	return out
}

func parseProcNetListen(path string, out map[string]Listener) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // cabeçalho
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 || fields[3] != "0A" { // 0A = LISTEN
			continue
		}
		ip, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil || port == 0 {
			continue
		}
		out[fields[9]] = Listener{Port: int(port), Addr: procAddr(ip)}
	}
}

// procAddr traduz o endereço hexadecimal do /proc só no que importa para a
// tela: aberto para a rede ou só local.
func procAddr(hex string) string {
	switch hex {
	case "00000000", "00000000000000000000000000000000":
		return "*"
	case "0100007F":
		return "127.0.0.1"
	case "00000000000000000000000001000000":
		return "::1"
	}
	if len(hex) == 8 { // IPv4, bytes em little-endian
		var b [4]uint64
		for i := range b {
			b[i], _ = strconv.ParseUint(hex[6-2*i:8-2*i], 16, 8)
		}
		return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
	}
	return "ipv6"
}

// socketOwners varre /proc/*/fd atrás de "socket:[inode]". Processo de outro
// usuário dá permissão negada e fica sem dono — a tela diz isso.
func socketOwners(want map[string]Listener) map[string]int {
	owners := map[string]int{}
	procs, _ := filepath.Glob("/proc/[0-9]*/fd")
	for _, fdDir := range procs {
		pid, err := strconv.Atoi(filepath.Base(filepath.Dir(fdDir)))
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			if _, ok := want[inode]; ok {
				if _, seen := owners[inode]; !seen {
					owners[inode] = pid
				}
			}
		}
		if len(owners) == len(want) {
			break
		}
	}
	return owners
}

// lsofListeners: `-F pcn` sai uma linha por campo — p<pid>, c<comando>,
// n<endereço:porta>. Sem pasta de trabalho: o casamento com projeto fica só
// pela porta declarada.
func lsofListeners() []Listener {
	raw, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn").Output()
	if err != nil {
		return nil
	}
	return parseLsofListeners(string(raw))
}

func parseLsofListeners(raw string) []Listener {
	var out []Listener
	pid, comm := 0, ""
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			comm = line[1:]
		case 'n':
			i := strings.LastIndex(line, ":")
			port, err := strconv.Atoi(line[i+1:])
			if i < 0 || err != nil {
				continue
			}
			addr := strings.Trim(line[1:i], "[]")
			if addr == "" || addr == "*" || addr == "0.0.0.0" || addr == "::" {
				addr = "*"
			}
			out = append(out, Listener{Port: port, Addr: addr, PID: pid, Process: comm})
		}
	}
	return out
}

// StopListener encerra o processo com SIGTERM — o mesmo que um ctrl+c educado,
// dá chance de o servidor fechar conexão e gravar o que precisa.
func StopListener(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("processo de outro usuário — encerre com sudo")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
