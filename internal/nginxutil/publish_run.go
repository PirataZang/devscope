package nginxutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// runCmd roda um comando e devolve a saída combinada. Os testes trocam por um
// falso — é assim que o "nginx -t falhou" é exercitado sem docker.
var runCmd = func(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// smokeTries/smokeDelay: o reload troca os workers em segundo plano; o
// primeiro GET logo depois ainda pode cair num worker velho.
var (
	smokeTries = 3
	smokeDelay = 400 * time.Millisecond
)

type StepState int

const (
	StepRunning StepState = iota
	StepOK
	StepFail
)

// Step é um passo da publicação, mandado ao vivo para a tela.
type Step struct {
	Index  int
	Name   string
	State  StepState
	Detail string
}

// Result é o card final.
type Result struct {
	File     string // sites/<slug>.inc
	Test     string // saída do nginx -t
	TestLine int    // linha da .inc que o nginx -t apontou (0 = nenhuma)
	URL      string
	Err      error
}

// PublishSteps são os nomes dos passos, na ordem — a tela desenha a lista
// inteira antes de começar.
func PublishSteps(p Plan) []string {
	steps := []string{"escrever sites/" + p.Slug + ".inc", "nginx -t", "nginx -s reload", "GET /" + p.Slug + " → 301"}
	if p.Profile.HasFront() {
		steps = append(steps, "GET /"+p.Slug+"/ → 200 html")
	}
	if p.Profile.HasAPI() {
		steps = append(steps, "GET /"+p.Slug+"/api/ → back-end")
	}
	return steps
}

// Publish escreve a .inc, testa, recarrega e confere de fora. nginx -t
// recusando desfaz a escrita: o hub volta exatamente como estava e nada é
// recarregado.
func Publish(h Hub, p Plan, progress func(Step)) Result {
	names := PublishSteps(p)
	res := Result{File: "sites/" + p.Slug + ".inc", URL: p.URL(h)}
	step := func(i int, st StepState, detail string) {
		if progress != nil {
			progress(Step{Index: i, Name: names[i], State: st, Detail: detail})
		}
	}

	step(0, StepRunning, "")
	restore, err := writeWithBackup(h.IncPath(p.Slug), Render(p, h))
	if err != nil {
		step(0, StepFail, err.Error())
		res.Err = err
		return res
	}
	step(0, StepOK, "")

	if !testAndReload(h, p.Slug, &res, restore, func(i int, st StepState, d string) { step(1+i, st, d) }) {
		return res
	}

	base := h.PublicURL + "/" + p.Slug
	checks := []func() string{func() string { return expectRedirect(base, "/"+p.Slug+"/") }}
	if p.Profile.HasFront() {
		checks = append(checks, func() string {
			problem := expectHTML(base + "/")
			if problem != "" && p.BuildPending {
				problem += " — o build ainda não foi feito"
			}
			return problem
		})
	}
	if p.Profile.HasAPI() {
		checks = append(checks, func() string { return expectBackend(base + "/api/") })
	}
	var failed []string
	for i, check := range checks {
		step(3+i, StepRunning, "")
		if problem := retrySmoke(check); problem != "" {
			step(3+i, StepFail, problem)
			failed = append(failed, names[3+i]+": "+problem)
			continue
		}
		step(3+i, StepOK, "")
	}
	if len(failed) > 0 {
		res.Err = fmt.Errorf("publicado, mas o smoke test falhou — %s", strings.Join(failed, "; "))
	}
	return res
}

// Unpublish tira a .inc do projeto, testa e recarrega. A anterior fica em
// .inc.bak (fora do include *.inc).
func Unpublish(h Hub, slug string, progress func(Step)) Result {
	names := []string{"remover sites/" + slug + ".inc", "nginx -t", "nginx -s reload"}
	res := Result{File: "sites/" + slug + ".inc"}
	step := func(i int, st StepState, detail string) {
		if progress != nil {
			progress(Step{Index: i, Name: names[i], State: st, Detail: detail})
		}
	}
	path := h.IncPath(slug)
	step(0, StepRunning, "")
	old, err := os.ReadFile(path)
	if err != nil {
		res.Err = fmt.Errorf("%s não está publicado", slug)
		step(0, StepFail, res.Err.Error())
		return res
	}
	if err := atomicWrite(path+".bak", old); err != nil {
		res.Err = err
		step(0, StepFail, err.Error())
		return res
	}
	if err := os.Remove(path); err != nil {
		res.Err = err
		step(0, StepFail, err.Error())
		return res
	}
	step(0, StepOK, "a anterior ficou em "+slug+".inc.bak")
	restore := func() { _ = atomicWrite(path, old) }
	testAndReload(h, slug, &res, restore, func(i int, st StepState, d string) { step(1+i, st, d) })
	return res
}

var reTestLine = regexp.MustCompile(`\.inc:(\d+)`)

// testAndReload: nginx -t e, só se passar, o reload. Recusa desfaz.
func testAndReload(h Hub, slug string, res *Result, restore func(), step func(int, StepState, string)) bool {
	step(0, StepRunning, "")
	out, err := runCmd("docker", "exec", h.Container, "nginx", "-t")
	res.Test = out
	if err != nil {
		restore()
		if strings.Contains(out, "/"+slug+".inc:") {
			if m := reTestLine.FindStringSubmatch(out[strings.Index(out, "/"+slug+".inc:"):]); m != nil {
				res.TestLine, _ = strconv.Atoi(m[1])
			}
		}
		res.Err = fmt.Errorf("nginx -t recusou — a .inc foi desfeita e nada foi recarregado")
		step(0, StepFail, testError(out))
		return false
	}
	step(0, StepOK, "")

	step(1, StepRunning, "")
	if out, err := runCmd("docker", "exec", h.Container, "nginx", "-s", "reload"); err != nil {
		res.Err = fmt.Errorf("reload falhou: %s", firstNonEmpty(lastLine(out), err.Error()))
		step(1, StepFail, res.Err.Error())
		return false
	}
	step(1, StepOK, "")
	return true
}

// writeWithBackup grava path atomicamente; o conteúdo anterior vira path.bak.
// restore devolve o hub ao estado de antes — .inc e .bak.
func writeWithBackup(path, content string) (restore func(), err error) {
	bak := path + ".bak"
	oldInc, incErr := os.ReadFile(path)
	oldBak, bakErr := os.ReadFile(bak)
	restore = func() {
		if incErr == nil {
			_ = atomicWrite(path, oldInc)
		} else {
			_ = os.Remove(path)
		}
		if bakErr == nil {
			_ = atomicWrite(bak, oldBak)
		} else {
			_ = os.Remove(bak)
		}
	}
	if incErr == nil {
		if err := atomicWrite(bak, oldInc); err != nil {
			return nil, err
		}
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		restore()
		return nil, err
	}
	return restore, nil
}

// atomicWrite: temporário na mesma pasta + rename. O nome do temporário não
// termina em .inc, então o include *.inc nunca vê um arquivo pela metade.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// ─── smoke test ─────────────────────────────────────────────────────────────

var smokeClient = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func retrySmoke(check func() string) string {
	problem := ""
	for i := 0; i < smokeTries; i++ {
		if i > 0 {
			time.Sleep(smokeDelay)
		}
		if problem = check(); problem == "" {
			return ""
		}
	}
	return problem
}

func smokeGet(url string) (*http.Response, string) {
	resp, err := smokeClient.Get(url)
	if err != nil {
		return nil, "sem resposta: " + err.Error()
	}
	resp.Body.Close()
	return resp, ""
}

func expectRedirect(url, to string) string {
	resp, problem := smokeGet(url)
	if problem != "" {
		return problem
	}
	if resp.StatusCode != http.StatusMovedPermanently || !strings.HasSuffix(resp.Header.Get("Location"), to) {
		return fmt.Sprintf("veio %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	return ""
}

func expectHTML(url string) string {
	resp, problem := smokeGet(url)
	if problem != "" {
		return problem
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		return fmt.Sprintf("veio %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return ""
}

func expectBackend(url string) string {
	resp, problem := smokeGet(url)
	if problem != "" {
		return problem
	}
	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout {
		return fmt.Sprintf("%d — container parado ou porta errada?", resp.StatusCode)
	}
	return ""
}

// ─── diff ───────────────────────────────────────────────────────────────────

// DiffLine é uma linha do diff da republicação: ' ' igual, '+' nova, '-' saiu.
type DiffLine struct {
	Op   byte
	Text string
}

// LineDiff compara duas versões da .inc por LCS de linhas.
// ponytail: O(n·m) — a .inc tem dezenas de linhas; para arquivos grandes, Myers.
func LineDiff(old, new string) []DiffLine {
	a, b := splitLines(old), splitLines(new)
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []DiffLine
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, DiffLine{' ', a[i]})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, DiffLine{'-', a[i]})
			i++
		default:
			out = append(out, DiffLine{'+', b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, DiffLine{'-', a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, DiffLine{'+', b[j]})
	}
	return out
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// testError é a linha do nginx -t que diz o que está errado ([emerg]), não o
// "test failed" do fim.
func testError(out string) string {
	for _, l := range splitLines(out) {
		if strings.Contains(l, "[emerg]") {
			return strings.TrimSpace(l)
		}
	}
	return lastLine(out)
}

func lastLine(s string) string {
	lines := splitLines(strings.TrimSpace(s))
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}
