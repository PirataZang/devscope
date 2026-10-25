package ui

import (
	"context"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Cloudflare: o domínio do projeto visto de fora.
//
// Dois níveis, como no painel da Cloudflare: a lista de DOMÍNIOS da conta (os
// deste projeto primeiro, cada um com os visitantes da semana) e, com enter, o
// domínio aberto em três abas — 1 tráfego, 2 DNS, 3 segurança. esc volta para
// a lista; esc na lista sai do módulo.
//
// O módulo só aparece no projeto que tem um domínio da conta: o site do Pages
// ligado ao mesmo repositório do GitHub, ou um endereço do domínio escrito nos
// arquivos do projeto (.env, CNAME, package.json, wrangler.toml). A lista de
// domínios da conta é lida uma vez na abertura e a cada 10 min.

const cfdTokenPage = "https://dash.cloudflare.com/profile/api-tokens"

// Abas do domínio aberto.
const (
	cfdTraffic = iota
	cfdDNS
	cfdSecurity
)

var cfdSubNames = [3]string{"tráfego", "DNS", "segurança"}

type cloudflareState struct {
	token   bool // há token nesta máquina
	acct    collectors.CFAccount
	acctOK  bool // a conta já foi lida (com ou sem sucesso)
	acctErr string
	ticking bool

	// Lista de domínios.
	path     string
	links    []collectors.CFLink             // domínios deste projeto
	week     map[string]collectors.CFTraffic // zone ID → últimos 7 dias
	cursor   int
	listBusy bool

	// Domínio aberto.
	open    bool
	zone    collectors.CFZone
	sub     int
	days    int // 7 ou 30
	traffic collectors.CFTraffic
	hourly  []collectors.CFHour
	pages   []collectors.CFCount
	failed  []collectors.CFPathError
	dns     []collectors.CFRecord
	sec     collectors.CFSecurity
	today   time.Time // data UTC da leitura: é ela que diz qual dia é "hoje"
	scroll  int
	loading bool
	err     string

	status string
}

type cfdAccountMsg struct {
	token bool
	acct  collectors.CFAccount
	err   error
}

type cfdAccountTickMsg struct{}

type cfdLinkedMsg struct {
	path, host string
	err        error
}

type cfdListMsg struct {
	path  string
	today time.Time
	links []collectors.CFLink
	week  map[string]collectors.CFTraffic
}

type cfdDetailMsg struct {
	zone    string
	days    int
	traffic collectors.CFTraffic
	hourly  []collectors.CFHour
	pages   []collectors.CFCount
	failed  []collectors.CFPathError
	dns     []collectors.CFRecord
	sec     collectors.CFSecurity
	today   time.Time
	err     error
}

func fetchCloudflareAccount() tea.Cmd {
	return func() tea.Msg {
		token := collectors.CloudflareToken()
		if token == "" {
			return cfdAccountMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		acct, err := collectors.FetchCloudflareAccount(ctx, token)
		return cfdAccountMsg{token: true, acct: acct, err: err}
	}
}

// handleCloudflareAccount guarda a conta e refaz a sidebar: é ela que decide
// se o módulo aparece no projeto aberto.
func (a *App) handleCloudflareAccount(msg cfdAccountMsg) tea.Cmd {
	c := &a.cfd
	c.token, c.acctOK, c.acctErr = msg.token, true, ""
	if msg.err != nil {
		c.acctErr = msg.err.Error()
	} else {
		c.acct = msg.acct
	}
	var cmds []tea.Cmd
	if p := a.currentProject(); p != nil {
		cmds = append(cmds, a.probeModuleCaps(p))
		if a.tab == TabCloudflare {
			cmds = append(cmds, a.loadCloudflare(p))
		}
	}
	if msg.token && !c.ticking {
		c.ticking = true
		cmds = append(cmds, tea.Tick(10*time.Minute, func(time.Time) tea.Msg { return cfdAccountTickMsg{} }))
	}
	return tea.Batch(cmds...)
}

func (a *App) handleCloudflareTick() tea.Cmd {
	a.cfd.ticking = false
	return fetchCloudflareAccount()
}

// loadCloudflare abre o módulo: a lista de domínios com a semana de cada um.
// Domínio que estava aberto continua aberto, e é recarregado junto.
func (a *App) loadCloudflare(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	c := &a.cfd
	if c.path != p.Path {
		*c = cloudflareState{token: c.token, acct: c.acct, acctOK: c.acctOK, acctErr: c.acctErr, ticking: c.ticking, path: p.Path, days: c.days}
	}
	if c.days == 0 {
		c.days = 7
	}
	if !c.acctOK {
		return fetchCloudflareAccount() // a resposta chama loadCloudflare de novo
	}
	if !c.token || c.acctErr != "" {
		return nil
	}
	c.listBusy = true
	path, acct := p.Path, c.acct
	remote := ""
	if p.Git != nil {
		remote = p.Git.Remote
	}
	list := func() tea.Msg {
		if remote == "" {
			remote = collectors.GitRemoteOrigin(path)
		}
		msg := cfdListMsg{path: path, today: time.Now().UTC(), links: collectors.LinkCloudflare(path, remote, acct), week: map[string]collectors.CFTraffic{}}
		token := collectors.CloudflareToken()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var mu sync.Mutex
		var wg sync.WaitGroup
		now := time.Now()
		// ponytail: uma consulta por domínio, em paralelo; com dezenas de
		// domínios, trocar por uma só com zoneTag_in.
		for i, z := range acct.Zones {
			if i == 20 {
				break
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if t, err := collectors.FetchCloudflareTraffic(ctx, token, id, 7, now); err == nil {
					mu.Lock()
					msg.week[id] = t
					mu.Unlock()
				}
			}(z.ID)
		}
		wg.Wait()
		return msg
	}
	if c.open {
		return tea.Batch(list, a.loadCloudflareDetail())
	}
	return list
}

func (a *App) handleCloudflareList(msg cfdListMsg) {
	c := &a.cfd
	if msg.path != c.path {
		return
	}
	c.listBusy = false
	c.links, c.week = msg.links, msg.week
	if !c.open {
		c.today = msg.today
	}
	c.cursor = clampInt(c.cursor, 0, maxInt(0, len(a.cfdRows())-1))
}

// loadCloudflareDetail lê tudo do domínio aberto de uma vez, em paralelo. Só o
// tráfego diário é obrigatório; o resto, se falhar, só some da tela.
func (a *App) loadCloudflareDetail() tea.Cmd {
	c := &a.cfd
	c.loading = true
	zone, days := c.zone, c.days
	return func() tea.Msg {
		token := collectors.CloudflareToken()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msg := cfdDetailMsg{zone: zone.ID, days: days, today: time.Now().UTC()}
		var wg sync.WaitGroup
		run := func(f func()) {
			wg.Add(1)
			go func() { defer wg.Done(); f() }()
		}
		run(func() { msg.traffic, msg.err = collectors.FetchCloudflareTraffic(ctx, token, zone.ID, days, msg.today) })
		run(func() { msg.hourly, _ = collectors.FetchCloudflareHourly(ctx, token, zone.ID, msg.today) })
		run(func() { msg.pages, msg.failed, _ = collectors.FetchCloudflarePaths(ctx, token, zone.ID, msg.today) })
		run(func() { msg.dns, _ = collectors.FetchCloudflareDNS(ctx, token, zone.ID) })
		run(func() { msg.sec = collectors.FetchCloudflareSecurity(ctx, token, zone) })
		wg.Wait()
		sort.SliceStable(msg.dns, func(i, j int) bool { return msg.dns[i].Name < msg.dns[j].Name })
		return msg
	}
}

func (a *App) handleCloudflareDetail(msg cfdDetailMsg) {
	c := &a.cfd
	if !c.open || msg.zone != c.zone.ID || msg.days != c.days {
		return
	}
	c.loading = false
	c.traffic, c.hourly, c.pages, c.failed, c.dns, c.sec, c.today = msg.traffic, msg.hourly, msg.pages, msg.failed, msg.dns, msg.sec, msg.today
	c.err = ""
	if msg.err != nil {
		c.err = msg.err.Error()
	}
}

func (a *App) handleCloudflareLinked(msg cfdLinkedMsg) tea.Cmd {
	if msg.err != nil {
		a.cfd.status = "erro: " + msg.err.Error()
		return nil
	}
	a.cfd.status = msg.host + " vinculado em " + collectors.CloudflareLinkFile
	p := a.currentProject()
	if p == nil || !pathsMatch(p.Path, msg.path) {
		return nil
	}
	return tea.Batch(a.loadCloudflare(p), a.probeModuleCaps(p))
}

// cfdRow é uma linha da lista: o domínio e, se for deste projeto, o vínculo.
type cfdRow struct {
	zone collectors.CFZone
	link *collectors.CFLink
}

// cfdRows: os domínios deste projeto primeiro, depois o resto da conta.
func (a *App) cfdRows() []cfdRow {
	var mine, rest []cfdRow
	for _, z := range a.cfd.acct.Zones {
		row := cfdRow{zone: z}
		for i := range a.cfd.links {
			if a.cfd.links[i].Zone.ID == z.ID {
				row.link = &a.cfd.links[i]
				break
			}
		}
		if row.link != nil {
			mine = append(mine, row)
		} else {
			rest = append(rest, row)
		}
	}
	return append(mine, rest...)
}

// host é o endereço do site: o que o projeto usa, ou o próprio domínio.
func (r cfdRow) host() string {
	if r.link != nil {
		return r.link.Host
	}
	return r.zone.Name
}

func (a *App) cfdOpenRow() cfdRow {
	for _, r := range a.cfdRows() {
		if r.zone.ID == a.cfd.zone.ID {
			return r
		}
	}
	return cfdRow{zone: a.cfd.zone}
}

// cfdDashPage é a página da Cloudflare que corresponde ao que está na tela.
func cfdDashPage(z collectors.CFZone, sub int, open bool) string {
	base := collectors.CloudflareDashURL(z)
	if !open {
		return base
	}
	switch sub {
	case cfdTraffic:
		return base + "/analytics/traffic"
	case cfdDNS:
		return base + "/dns/records"
	}
	return base + "/security"
}

func (a *App) handleCloudflareKeys(msg tea.KeyMsg, p *core.Project) (tea.Cmd, bool) {
	c := &a.cfd
	key := msg.String()
	if key == "r" {
		c.status, c.acctOK = "", false
		return a.loadCloudflare(p), true
	}
	if !c.token || c.acctErr != "" {
		if key == "o" {
			openBrowser(cfdTokenPage)
			c.status = "abrindo a página de tokens da Cloudflare"
			return nil, true
		}
		return nil, false
	}
	if c.open {
		return a.handleCloudflareDetailKeys(key, p)
	}

	rows := a.cfdRows()
	if len(rows) == 0 {
		return nil, false
	}
	c.cursor = clampInt(c.cursor, 0, len(rows)-1)
	row := rows[c.cursor]
	switch key {
	case "up", "k":
		c.cursor = (c.cursor - 1 + len(rows)) % len(rows)
	case "down", "j":
		c.cursor = (c.cursor + 1) % len(rows)
	case "enter":
		c.open, c.zone, c.sub, c.scroll, c.status = true, row.zone, cfdTraffic, 0, ""
		c.traffic, c.hourly, c.pages, c.failed, c.dns, c.sec, c.err = collectors.CFTraffic{}, nil, nil, nil, nil, collectors.CFSecurity{}, ""
		return a.loadCloudflareDetail(), true
	case "w":
		openBrowser("https://" + row.host())
		c.status = "abrindo " + row.host()
	case "o":
		openBrowser(cfdDashPage(row.zone, 0, false))
		c.status = "abrindo " + row.zone.Name + " na Cloudflare"
	case "v":
		if row.link != nil {
			c.status = row.zone.Name + " já é deste projeto (" + row.link.From + ")"
			return nil, true
		}
		host, path := row.zone.Name, p.Path
		return func() tea.Msg {
			return cfdLinkedMsg{path: path, host: host, err: collectors.SaveCloudflareLink(path, host)}
		}, true
	default:
		return nil, false
	}
	return nil, true
}

func (a *App) handleCloudflareDetailKeys(key string, p *core.Project) (tea.Cmd, bool) {
	c := &a.cfd
	row := a.cfdOpenRow()
	switch key {
	case "esc":
		c.open, c.status = false, ""
	case "1", "2", "3":
		c.sub, c.scroll = int(key[0]-'1'), 0
	case "left", "h", "right", "l":
		if c.sub != cfdTraffic {
			return nil, true
		}
		if c.days == 7 {
			c.days = 30
		} else {
			c.days = 7
		}
		return a.loadCloudflareDetail(), true
	case "up", "k":
		c.scroll = maxInt(0, c.scroll-1)
	case "down", "j":
		c.scroll++
	case "w", "enter":
		openBrowser("https://" + row.host())
		c.status = "abrindo " + row.host()
	case "o":
		openBrowser(cfdDashPage(c.zone, c.sub, true))
		c.status = "abrindo " + cfdSubNames[c.sub] + " de " + c.zone.Name + " na Cloudflare"
	default:
		return nil, false
	}
	return nil, true
}
