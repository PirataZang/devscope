package hub

import (
	"fmt"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/devscopeutil"
)

// ctrState é o que o vigia guarda de cada container entre uma leitura e outra.
type ctrState struct {
	id, state, health         string
	project, projectID, short string
}

func (c ctrState) ok() bool {
	return c.state == "running" && !strings.EqualFold(c.health, "unhealthy")
}

// downMark lembra a última queda de um container: repetir em menos de 10
// minutos sobe o count do mesmo aviso em vez de abrir outro.
type downMark struct {
	at     time.Time
	count  int
	open   bool // caiu e ainda não voltou
	health bool // caiu pelo healthcheck: a volta é "resolved"
}

// watcher compara o estado de agora com a leitura anterior (a cada 5 s) e
// gera os Event do Contrato. A primeira leitura só serve de referência.
type watcher struct {
	started  bool
	prev     map[string]ctrState
	prevProj map[string]string
	downs    map[string]*downMark
	diskHigh bool
	sslSent  map[string]time.Time
}

func newWatcher() *watcher {
	return &watcher{downs: map[string]*downMark{}, sslSent: map[string]time.Time{}}
}

// step devolve os avisos novos e se algum projeto ou container mudou de
// estado (o que pede um snapshot fora de hora).
func (w *watcher) step(snap core.Snapshot, now time.Time) (evs []Event, changed bool) {
	cur := map[string]ctrState{}
	curProj := map[string]string{}
	for _, p := range snap.Projects {
		curProj[p.ID], _, _, _ = projectState(p)
		for _, c := range p.Containers {
			cur["ctr:"+c.Name] = ctrState{id: c.ID, state: c.Status, health: c.Health,
				project: p.Name, projectID: p.ID, short: shortName(p, c)}
		}
	}
	disk := snap.HostMetrics.DiskPercent
	if !w.started {
		w.started = true
		w.prev, w.prevProj = cur, curProj
		w.diskHigh = disk > 90
		for _, p := range snap.Projects {
			for _, s := range p.SSL {
				if s.DaysLeft < 7 {
					w.sslSent["ssl:"+s.Domain] = now
				}
			}
		}
		return nil, false
	}

	for key, c := range cur {
		old, ok := w.prev[key]
		if !ok {
			continue
		}
		changed = changed || old != c
		switch m := w.downs[key]; {
		case old.ok() && !c.ok():
			evs = append(evs, w.down(key, c, now))
		case !old.ok() && c.ok() && m != nil && m.open:
			m.open = false
			evs = append(evs, upEvent(key, c, m, now))
		}
	}
	for id, st := range curProj {
		changed = changed || w.prevProj[id] != st
	}
	w.prev, w.prevProj = cur, curProj

	// Disco: avisa ao passar de 90% e só de novo depois de cair 5 pontos.
	m := snap.HostMetrics
	switch {
	case !w.diskHigh && disk > 90:
		w.diskHigh = true
		evs = append(evs, Event{Key: "disk", Kind: "disk", Title: fmt.Sprintf("disco em %d%%", pct(disk)),
			Detail: fmt.Sprintf("%.0f de %.0f GB usados", m.DiskUsedGB, m.DiskTotalGB), At: now})
	case w.diskHigh && disk < 85:
		w.diskHigh = false
		evs = append(evs, Event{Key: "disk", Kind: "resolved", Title: fmt.Sprintf("disco voltou para %d%%", pct(disk)), At: now})
	}

	for _, p := range snap.Projects {
		for _, s := range p.SSL {
			key := "ssl:" + s.Domain
			if s.DaysLeft >= 7 || now.Sub(w.sslSent[key]) < 24*time.Hour {
				continue
			}
			w.sslSent[key] = now
			title := fmt.Sprintf("SSL de %s vence em %d dias", s.Domain, s.DaysLeft)
			if s.DaysLeft < 0 {
				title = fmt.Sprintf("SSL de %s venceu", s.Domain)
			}
			evs = append(evs, Event{Key: key, Kind: "ssl", ProjectID: p.ID, Project: p.Name, Title: title, At: now})
		}
	}
	return evs, changed
}

func (w *watcher) down(key string, c ctrState, now time.Time) Event {
	m := w.downs[key]
	if m == nil || now.Sub(m.at) >= 10*time.Minute {
		m = &downMark{}
		w.downs[key] = m
	}
	m.count++
	m.at, m.open, m.health = now, true, c.state == "running"

	ev := Event{Key: key, Kind: "container_down", ProjectID: c.projectID, Project: c.project,
		Container: c.short, Count: m.count, At: now}
	if m.health {
		ev.Title, ev.Detail = c.short+" está unhealthy", "o healthcheck está falhando"
		return ev
	}
	ev.Title, ev.Detail = c.short+" caiu", c.state
	if code, oom, err := dockerExitState(c.id); err == nil {
		ev.ExitCode = &code
		ev.Detail = fmt.Sprintf("saiu com código %d", code)
		if c.state == "restarting" {
			ev.Detail = fmt.Sprintf("reiniciando sem parar (último código %d)", code)
		}
		if oom {
			ev.Detail += " (sem memória)"
		}
	}
	return ev
}

func upEvent(key string, c ctrState, m *downMark, now time.Time) Event {
	ev := Event{Key: key, Kind: "container_up", ProjectID: c.projectID, Project: c.project,
		Container: c.short, Title: c.short + " voltou", Count: m.count, At: now}
	if m.health {
		ev.Kind, ev.Title = "resolved", c.short+" saudável de novo"
	}
	return ev
}

// ciWatcher lê os 5 últimos runs de cada projeto com gh, a cada 60 s, e avisa
// de cada run que terminou desde a leitura anterior.
type ciWatcher struct{ seen map[string]map[string]bool }

func newCIWatcher() *ciWatcher { return &ciWatcher{seen: map[string]map[string]bool{}} }

func (w *ciWatcher) step(a *Agent, projects []core.Project) []Event {
	var evs []Event
	for _, p := range projects {
		if !ciCapable(p.Path) {
			continue
		}
		runs, err := ghaListRuns(p.Path, 5)
		if err != nil {
			continue
		}
		a.setCI(p.ID, runs)
		prev, known := w.seen[p.ID]
		cur := map[string]bool{}
		for i := len(runs) - 1; i >= 0; i-- { // do mais antigo para o mais novo
			r := runs[i]
			if r.Status != "completed" {
				continue
			}
			cur[r.ID] = true
			if !known || prev[r.ID] {
				continue
			}
			ev := Event{Key: "ci:" + p.ID + ":" + r.Workflow + ":" + r.Branch, ProjectID: p.ID, Project: p.Name,
				Detail: devscopeutil.MaskLine(r.DisplayTitle), At: a.now()}
			switch r.Conclusion {
			case "success":
				ev.Kind, ev.Title = "ci_ok", fmt.Sprintf("CI passou: %s (%s)", r.Workflow, r.Branch)
			case "failure", "timed_out", "startup_failure":
				ev.Kind, ev.Title = "ci_failed", fmt.Sprintf("CI falhou: %s (%s)", r.Workflow, r.Branch)
			default: // cancelado ou pulado não é aviso
				continue
			}
			evs = append(evs, ev)
		}
		w.seen[p.ID] = cur
	}
	return evs
}
