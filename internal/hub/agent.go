package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/devscopeutil"
)

// Intervalos do Contrato. São variáveis para os testes não esperarem minutos.
var (
	pollWait      = 50
	snapshotEvery = 30 * time.Second
	watchEvery    = 5 * time.Second
	ciEvery       = 60 * time.Second
	tuiRecheck    = 5 * time.Second
)

// ─── indicador da TUI ───────────────────────────────────────────────────────

const (
	indicatorOff int32 = iota
	indicatorOnline
	indicatorOffline
)

var indicator atomic.Int32

// Indicator é o que a barra de status da TUI mostra: "" quando não há agente
// nesta TUI, "bot ●" conectado e "bot ○" sem conexão.
func Indicator() (text string, online bool) {
	switch indicator.Load() {
	case indicatorOnline:
		return "bot ●", true
	case indicatorOffline:
		return "bot ○", false
	}
	return "", false
}

// ─── lock ───────────────────────────────────────────────────────────────────

// LockedError diz que outro processo vivo já está conectado ao hub.
type LockedError struct{ PID int }

func (e *LockedError) Error() string {
	return fmt.Sprintf("outro devscope já está conectado ao hub (PID %d)", e.PID)
}

// AcquireLock grava o PID em bot.lock: é o que garante uma conexão por
// máquina. O PID vai num arquivo temporário e o link só funciona se o lock
// não existir, então dois processos nunca ganham juntos. Lock de PID morto é
// tomado.
func AcquireLock() (release func(), err error) {
	if err := ensureDir(); err != nil {
		return nil, err
	}
	tmp := fmt.Sprintf("%s.%d", LockPath(), os.Getpid())
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(tmp)
	for range 2 {
		if err := os.Link(tmp, LockPath()); err == nil {
			return func() {
				if LockHolder() == os.Getpid() {
					_ = os.Remove(LockPath())
				}
			}, nil
		} else if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if pid := LockHolder(); pid != 0 {
			return nil, &LockedError{PID: pid}
		}
		_ = os.Remove(LockPath())
	}
	return nil, errors.New("não consegui pegar o " + LockPath())
}

// LockHolder devolve o PID vivo que segura o lock, ou 0.
func LockHolder() int {
	b, err := os.ReadFile(LockPath())
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid <= 0 || !pidAlive(pid) {
		return 0
	}
	return pid
}

func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// No Windows o FindProcess já falha com processo inexistente; o sinal 0
	// só existe nos unix.
	if runtime.GOOS == "windows" {
		return true
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// ─── agente ─────────────────────────────────────────────────────────────────

// Agent é a máquina conectada: responde as consultas do hub, manda o
// snapshot e os avisos do vigia. Lê sempre do StateStore de quem o subiu.
type Agent struct {
	client *Client
	store  *core.StateStore
	cfg    *config.Config
	mode   string // "tui" | "service"
	now    func() time.Time

	cancel  context.CancelCauseFunc
	snapNow chan struct{}

	ciMu sync.Mutex
	ci   map[string]ciEntry

	qMu   sync.Mutex
	queue []Event

	touched atomic.Int64
}

func newAgent(c *Client, store *core.StateStore, cfg *config.Config, mode string) *Agent {
	return &Agent{
		client: c, store: store, cfg: cfg, mode: mode, now: time.Now,
		snapNow: make(chan struct{}, 1),
		ci:      map[string]ciEntry{},
	}
}

// Run fica conectado até ctx acabar (devolve nil) ou o hub recusar a máquina
// de vez: key revogada, sem dono ou protocolo velho.
func (a *Agent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	a.cancel = cancel
	a.queue = LoadState().Queue

	var wg sync.WaitGroup
	for _, loop := range []func(context.Context){a.snapshotLoop, a.watchLoop, a.ciLoop} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx)
		}()
	}
	a.pollLoop(ctx)
	cancel(nil)
	loopsDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(loopsDone)
	}()
	// ponytail: espera no máximo 1 s; um gh pendurado no vigia de CI não
	// segura o fechamento da TUI, e a goroutine dele morre sozinha depois.
	select {
	case <-loopsDone:
	case <-time.After(time.Second):
	}

	var e *Error
	if !errors.As(context.Cause(ctx), &e) {
		return nil
	}
	indicator.Store(indicatorOffline)
	if e.Code != CodeUpgrade {
		_ = UpdateState(func(st *State) {
			st.Revoked = e.Message
			st.RevokedAt = a.now()
		})
	}
	return e
}

// fail para o agente quando o erro não se resolve tentando de novo.
func (a *Agent) fail(err error) bool {
	if !IsCode(err, CodeUnauthorized) && !IsCode(err, CodeNotPaired) && !IsCode(err, CodeUpgrade) {
		return false
	}
	a.cancel(err)
	return true
}

// actionsEnabled vai para o poll e o snapshot. Ações chegam na fase 2; até lá
// a máquina se declara só leitura, não importa o config.
func (a *Agent) actionsEnabled() bool { return false }

func (a *Agent) pollLoop(ctx context.Context) {
	var b backoff
	for ctx.Err() == nil {
		cmds, err := a.client.Poll(ctx, a.mode, a.actionsEnabled(), pollWait)
		if err != nil {
			if ctx.Err() != nil || a.fail(err) {
				return
			}
			indicator.Store(indicatorOffline)
			log.Printf("bot: %v", err)
			sleep(ctx, b.next(err))
			continue
		}
		b.reset()
		indicator.Store(indicatorOnline)
		a.touch()
		for _, c := range cmds {
			go a.handle(ctx, c)
		}
	}
}

// touch grava a hora do último contato, no máximo uma vez por minuto: é o que
// `devscope bot status` mostra.
func (a *Agent) touch() {
	now := a.now()
	if now.Unix()-a.touched.Load() < 60 {
		return
	}
	a.touched.Store(now.Unix())
	_ = UpdateState(func(st *State) { st.LastSeen = now })
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// handle responde um comando antes do deadline dele. Um handler que trava
// (docker ou gh pendurado) não segura a resposta: ela sai como timeout.
func (a *Agent) handle(ctx context.Context, c Command) {
	deadline := c.Deadline
	if deadline.IsZero() {
		deadline = a.now().Add(20 * time.Second)
	}
	cctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	res := Result{CommandID: c.ID}
	data, err := a.dispatch(cctx, c)
	if err != nil {
		res.Error, res.Message = errorCode(err), devscopeutil.MaskLine(err.Error())
	} else {
		res.OK, res.Data = true, data
	}
	if err := a.client.Result(ctx, res); err != nil && ctx.Err() == nil && !a.fail(err) {
		log.Printf("bot: resposta de %s: %v", c.Type, err)
	}
}

func (a *Agent) dispatch(ctx context.Context, c Command) (any, error) {
	h, ok := handlers[c.Type]
	if !ok {
		if actionTypes[c.Type] {
			return nil, cmdErr("actions_disabled", "as ações estão desligadas nesta máquina")
		}
		return nil, cmdErr("unknown_command", "comando desconhecido: "+c.Type)
	}
	var args Args
	if len(c.Args) > 0 {
		if err := json.Unmarshal(c.Args, &args); err != nil {
			return nil, cmdErr("bad_request", "argumentos inválidos para "+c.Type)
		}
	}
	type reply struct {
		data any
		err  error
	}
	ch := make(chan reply, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- reply{err: fmt.Errorf("falha interna em %s: %v", c.Type, r)}
			}
		}()
		d, err := h(ctx, a, args)
		ch <- reply{d, err}
	}()
	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, cmdErr("timeout", "a máquina não respondeu a tempo")
	}
}

func (a *Agent) snapshotLoop(ctx context.Context) {
	// O coletor de métricas acabou de subir: sem esperar a primeira leitura,
	// o hub veria a máquina com 0% de tudo pelos primeiros 30 s.
	for i := 0; i < 25 && ctx.Err() == nil && a.store.Get().HostMetrics.Uptime == 0; i++ {
		sleep(ctx, 200*time.Millisecond)
	}
	t := time.NewTicker(snapshotEvery)
	defer t.Stop()
	for {
		snap := a.store.Get()
		err := a.client.Snapshot(ctx, Snapshot{
			Host:           a.host(snap.HostMetrics),
			Projects:       a.summaries(snap.Projects),
			ActionsEnabled: a.actionsEnabled(),
			GH:             ghaAvailable(),
			Mode:           a.mode,
			At:             a.now(),
		})
		if err != nil {
			a.fail(err) // offline: o próximo tique tenta de novo
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-a.snapNow:
		}
	}
}

func (a *Agent) watchLoop(ctx context.Context) {
	w := newWatcher()
	t := time.NewTicker(watchEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		evs, changed := w.step(a.store.Get(), a.now())
		if changed {
			select {
			case a.snapNow <- struct{}{}:
			default:
			}
		}
		a.sendEvents(ctx, evs)
	}
}

func (a *Agent) ciLoop(ctx context.Context) {
	w := newCIWatcher()
	t := time.NewTicker(ciEvery)
	defer t.Stop()
	for {
		a.sendEvents(ctx, w.step(a, a.store.Get().Projects))
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sendEvents manda os avisos novos junto com os que ficaram na fila. Sem
// conexão eles voltam para a fila de bot.json (até 100, por até 24 h) e saem
// depois com delayed: true.
func (a *Agent) sendEvents(ctx context.Context, evs []Event) {
	a.qMu.Lock()
	defer a.qMu.Unlock()
	queued := pruneQueue(a.queue, a.now())
	if len(evs) == 0 && len(queued) == 0 {
		return
	}
	err := a.client.Events(ctx, append(append([]Event{}, queued...), evs...))
	switch {
	case err == nil:
		a.touch()
		queued = nil
	case ctx.Err() != nil || IsCode(err, CodeOffline) || IsCode(err, CodeRateLimited):
		for i := range evs {
			evs[i].Delayed = true
		}
		queued = pruneQueue(append(queued, evs...), a.now())
	default:
		// O hub recusou o lote: tentar de novo só repetiria a recusa.
		if !a.fail(err) {
			log.Printf("bot: avisos recusados pelo hub: %v", err)
		}
		queued = nil
	}
	if len(a.queue) == 0 && len(queued) == 0 {
		return
	}
	a.queue = queued
	_ = UpdateState(func(st *State) { st.Queue = queued })
}

func pruneQueue(q []Event, now time.Time) []Event {
	out := make([]Event, 0, len(q))
	for _, e := range q {
		if now.Sub(e.At) < 24*time.Hour {
			out = append(out, e)
		}
	}
	if len(out) > 100 {
		out = out[len(out)-100:]
	}
	return out
}

// Connect roda o agente até ctx acabar e se despede com /v1/bye, para o hub
// marcar a máquina offline sem gerar aviso.
func Connect(ctx context.Context, key string, store *core.StateStore, cfg *config.Config, mode string) error {
	a := newAgent(NewClient(cfg.Bot.HubURL, key), store, cfg, mode)
	if err := a.Run(ctx); err != nil {
		return err
	}
	bctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = a.client.Bye(bctx)
	return nil
}

// StartTUI sobe o agente junto da TUI quando a máquina está pareada e ninguém
// mais segura o lock (o serviço, ou outra TUI aberta antes). Confere de novo a
// cada tuiRecheck: quem pareia com a TUI já aberta, ou fecha a TUI que estava
// conectada, não precisa reabrir as outras. Sem key ou sem dono só lê os
// arquivos locais, sem chamada de rede. Nada vai para o terminal: os erros vão
// para o log, que a TUI já redireciona.
func StartTUI(store *core.StateStore, cfg *config.Config) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		var lastErr string
		for {
			if key, _, ok := Paired(); ok {
				if release, err := AcquireLock(); err != nil {
					if err.Error() != lastErr { // o log não ganha a mesma linha a cada 5 s
						log.Printf("bot: %v", err)
						lastErr = err.Error()
					}
				} else {
					lastErr = ""
					indicator.Store(indicatorOffline)
					err := Connect(ctx, key, store, cfg, "tui")
					release()
					indicator.Store(indicatorOff)
					if err != nil {
						log.Printf("bot: %v", err)
					}
					// Versão velha não melhora tentando de novo; key recusada já
					// marcou o estado e o Paired acima espera um pareamento novo.
					if IsCode(err, CodeUpgrade) {
						return
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(tuiRecheck):
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
