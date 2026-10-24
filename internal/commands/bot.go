package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/hub"
	"github.com/devscope/devscope/internal/ui"
	"github.com/spf13/cobra"
)

const (
	defaultBot   = "devscope_vps_bot"
	telegramBlue = lipgloss.Color("#2AABEE")
	serviceName  = "devscope-bot"
)

// unitPath é variável para os testes não lerem o systemd da máquina.
var unitPath = "/etc/systemd/system/" + serviceName + ".service"

// exitError carrega o código de saída até o main: 130 no Ctrl+C, 3 para o
// systemd não reiniciar uma máquina sem pareamento.
type exitError struct{ code int }

func (e *exitError) Error() string { return "exit " + strconv.Itoa(e.code) }

// ExitCode é o código de saída do processo para o erro de Execute.
func ExitCode(err error) int {
	var e *exitError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}

func botCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bot",
		Short: "Conecta esta máquina ao bot do Telegram (@" + defaultBot + ")",
		Args:  cobra.NoArgs,
		RunE:  botAction((*botUI).pair),
	}
	root.AddCommand(
		&cobra.Command{Use: "status", Short: "Mostra o pareamento e a conexão com o hub", Args: cobra.NoArgs, RunE: botAction((*botUI).status)},
		&cobra.Command{Use: "disconnect", Short: "Desconecta esta máquina do bot e apaga a key", Args: cobra.NoArgs, RunE: botAction((*botUI).disconnect)},
		&cobra.Command{Use: "install", Short: "Instala o serviço systemd que mantém a máquina sempre conectada", Args: cobra.NoArgs, RunE: botAction((*botUI).install)},
		&cobra.Command{Use: "uninstall", Short: "Remove o serviço systemd", Args: cobra.NoArgs, RunE: botAction((*botUI).uninstall)},
		&cobra.Command{Use: "run", Short: "Mantém a máquina conectada, sem tela (usado pelo serviço)", Args: cobra.NoArgs, RunE: botService},
	)
	for _, c := range append(root.Commands(), root) {
		c.SilenceErrors, c.SilenceUsage = true, true
	}
	return root
}

func botAction(fn func(*botUI, context.Context, *config.Config) error) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		u := newBotUI()
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return u.finish(ctx, err)
		}
		return u.finish(ctx, fn(u, ctx, cfg))
	}
}

// ─── terminal ───────────────────────────────────────────────────────────────

// botUI é o terminal do `devscope bot`. Entrada, saída, relógio e largura
// ficam aqui para o teste golden rodar sem terminal de verdade.
type botUI struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	color  bool
	tty    bool // stdout é terminal: a contagem se atualiza no lugar
	inTTY  bool // stdin é terminal: a resposta já aparece ecoada
	width  int
	now    func() time.Time
	lines  chan string
}

func newBotUI() *botUI {
	tty := term.IsTerminal(os.Stdout.Fd())
	width := 80
	if w, _, err := term.GetSize(os.Stdout.Fd()); tty && err == nil && w > 0 {
		width = w
	}
	return &botUI{
		in: os.Stdin, out: os.Stdout, errOut: os.Stderr,
		color: tty && os.Getenv("NO_COLOR") == "", tty: tty, inTTY: term.IsTerminal(os.Stdin.Fd()),
		width: width, now: time.Now,
	}
}

func (u *botUI) paint(c lipgloss.TerminalColor, s string) string {
	if !u.color {
		return s
	}
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

func (u *botUI) check() string        { return u.paint(ui.ColorSuccess, "✓") }
func (u *botUI) link(s string) string { return u.paint(telegramBlue, s) }
func (u *botUI) cmd(s string) string  { return u.paint(ui.ColorPrimary, s) }
func (u *botUI) warn(s string) string { return u.paint(ui.ColorWarning, s) }
func (u *botUI) gray(s string) string { return u.paint(ui.ColorMuted, s) }

func (u *botUI) printf(format string, a ...any) { fmt.Fprintf(u.out, format, a...) }

// finish traduz o erro em saída: Ctrl+C sai com 130, o resto mostra a
// mensagem do hub e sai com 1.
func (u *botUI) finish(ctx context.Context, err error) error {
	var ee *exitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ee):
		return err
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		fmt.Fprintln(u.out)
		return &exitError{130}
	}
	msg := err.Error()
	var he *hub.Error
	if errors.As(err, &he) && he.Code == hub.CodeRateLimited && he.RetryAfter > 0 {
		msg += " — tente de novo em " + he.RetryAfter.Round(time.Second).String()
	}
	fmt.Fprintf(u.errOut, "%s %s\n", u.paint(ui.ColorDanger, "✗"), msg)
	return &exitError{1}
}

func (u *botUI) readLine(ctx context.Context) (string, error) {
	if u.lines == nil {
		u.lines = make(chan string)
		go func() {
			sc := bufio.NewScanner(u.in)
			for sc.Scan() {
				u.lines <- sc.Text()
			}
			close(u.lines)
		}()
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case l, ok := <-u.lines:
		if !ok {
			return "", io.EOF
		}
		return l, nil
	}
}

// ask lê s/n. Enter vazio (ou fim da entrada) fica com o padrão.
func (u *botUI) ask(ctx context.Context, def bool) (bool, error) {
	l, err := u.readLine(ctx)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if !u.inTTY { // ninguém viu a resposta ser digitada
		u.printf("%s\n", l)
	}
	a := strings.ToLower(strings.TrimSpace(l))
	switch {
	case a == "":
		return def, nil
	case def:
		return !strings.HasPrefix(a, "n"), nil
	default:
		return strings.HasPrefix(a, "s") || strings.HasPrefix(a, "y"), nil
	}
}

// ─── devscope bot ───────────────────────────────────────────────────────────

func (u *botUI) pair(ctx context.Context, cfg *config.Config) error {
	key, err := hub.LoadKey()
	if err != nil {
		return err
	}
	st := hub.LoadState()
	if key != "" && st.Owner != nil && st.Revoked == "" {
		u.statusBlock(ctx, cfg, key, st)
		st = hub.LoadState() // o statusBlock marca a key se o hub a recusou
		u.printf("\nParear com outra conta? Isso desconecta %s. [s/N] ", st.Owner.Who())
		if yes, err := u.ask(ctx, false); err != nil || !yes {
			return err
		}
		u.printf("\n")
	}

	verb := "encontrada"
	// Key revogada não volta: pareia de novo com uma nova.
	if key == "" || st.Revoked != "" {
		if err := hub.ClearLocal(); err != nil {
			return err
		}
		if key, err = hub.NewKey(); err != nil {
			return err
		}
		if err := hub.SaveKey(key); err != nil {
			return err
		}
		verb = "criada"
	}
	u.printf("%s key desta máquina %s  %s\n", u.check(), verb, u.gray("("+tilde(hub.KeyPath())+")"))

	c := hub.NewClient(cfg.Bot.HubURL, key)
	name := cfg.Bot.Name()
	id, err := c.Register(ctx, name, runtime.GOOS)
	if err != nil {
		return err
	}
	if err := hub.UpdateState(func(s *hub.State) { s.MachineID = id }); err != nil {
		return err
	}
	u.printf("%s hub: %s\n", u.check(), c.Host())

	for {
		code, err := c.Pair(ctx)
		if err != nil {
			return err
		}
		u.showCode(code)
		req, err := u.waitPair(ctx, c, code)
		if err != nil {
			return err
		}
		if req == nil {
			u.printf("código venceu. Gerar outro? [S/n] ")
			if again, err := u.ask(ctx, true); err != nil || !again {
				return err
			}
			continue
		}

		who := req.Chat.Who()
		if req.Chat.Username != "" && req.Chat.Name != "" {
			who += " (" + req.Chat.Name + ")"
		}
		u.printf("\n%s quer conectar a %s. Confirmar? [s/N] ", who, name)
		accept, err := u.ask(ctx, false)
		if err != nil {
			return err
		}
		owner, err := c.PairConfirm(ctx, req.RequestID, accept)
		if err != nil {
			return err
		}
		if !accept {
			u.printf("pedido recusado. Nada foi conectado.\n")
			return nil
		}
		if owner.Name == "" {
			owner.Name = req.Chat.Name
		}
		bot := firstNonEmpty(code.Bot, defaultBot)
		err = hub.UpdateState(func(s *hub.State) {
			s.Owner, s.Bot, s.PairedAt, s.LastSeen = &owner, bot, u.now(), u.now()
			s.Revoked, s.RevokedAt = "", time.Time{}
		})
		if err != nil {
			return err
		}
		u.connected(owner)
		return nil
	}
}

// showCode mostra o link, o código e o QR. A legenda fica ao lado do QR com
// 80 colunas ou mais, embaixo com menos, e sem QR abaixo de 45.
func (u *botUI) showCode(code hub.PairCode) {
	u.printf("\nAbra no celular:\n  %s\nou mande para o bot:  %s\n\n", u.link(code.Link), u.cmd("/start "+code.Code))
	legend := []string{
		"QR do mesmo link",
		"código vale até " + code.ExpiresAt.In(u.now().Location()).Format("15:04"),
		"uso único",
	}
	qr, err := hub.QR(code.Link)
	if err != nil || u.width < 45 {
		u.printf("%s\n", u.gray(strings.Join(legend[1:], " · ")))
		return
	}
	for i, l := range qr {
		if u.width >= 80 && i < len(legend) {
			l += "    " + u.gray(legend[i])
		}
		u.printf("%s\n", l)
	}
	if u.width < 80 {
		for _, l := range legend {
			u.printf("%s\n", u.gray(l))
		}
	}
}

// waitPair segura o pair/wait enquanto a contagem anda. Só a última linha se
// atualiza no lugar, e só quando é terminal. nil sem erro = código venceu.
func (u *botUI) waitPair(ctx context.Context, c *hub.Client, code hub.PairCode) (*hub.PairRequest, error) {
	type result struct {
		w   hub.PairWait
		err error
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan result, 1)
	go func() {
		for {
			w, err := c.PairWait(ctx, code.Code, 50)
			if err != nil || w.Request != nil || w.Expired {
				ch <- result{w, err}
				return
			}
		}
	}()
	line := func() string {
		left := max(0, code.ExpiresAt.Sub(u.now())).Truncate(time.Second)
		return u.gray("aguardando o Telegram…") + fmt.Sprintf(" %02d:%02d", int(left.Minutes()), int(left.Seconds())%60)
	}
	u.printf("\n%s", line())
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-ch:
			u.printf("\n")
			if r.err != nil {
				return nil, r.err
			}
			return r.w.Request, nil
		case <-t.C:
			if u.tty {
				u.printf("\r%s\x1b[K", line())
			}
		}
	}
}

func (u *botUI) connected(owner hub.Owner) {
	u.printf("%s conectado ao chat %s\n", u.check(), u.gray(owner.IDMasked))
	u.printf("  avisos:   container, CI com falha, disco > 90%%\n")
	u.printf("  ações:    %s\n            libere com %s\n", u.warn("só leitura"), u.cmd("devscope bot actions on"))
	if serviceInstalled() {
		u.printf("  conexão:  sempre ligado (serviço %s)\n", serviceName)
		return
	}
	u.printf("  conexão:  enquanto o devscope estiver aberto\n            sempre ligado: %s\n", u.cmd("devscope bot install"))
}

// ─── devscope bot status ────────────────────────────────────────────────────

func (u *botUI) status(ctx context.Context, cfg *config.Config) error {
	key, st, ok := hub.Paired()
	switch {
	case key == "":
		u.printf("esta máquina não está pareada. Conecte com %s\n", u.cmd("devscope bot"))
	case st.Revoked != "":
		u.printf("%s o hub recusou esta máquina em %s: %s\n  pareie de novo com %s\n",
			u.warn("⚠"), st.RevokedAt.In(u.now().Location()).Format("02/01 15:04"), st.Revoked, u.cmd("devscope bot"))
	case !ok:
		u.printf("o pareamento não terminou. Rode %s\n", u.cmd("devscope bot"))
	default:
		u.statusBlock(ctx, cfg, key, st)
	}
	return nil
}

func (u *botUI) statusBlock(ctx context.Context, cfg *config.Config, key string, st hub.State) {
	c := hub.NewClient(cfg.Bot.HubURL, key)
	// Lista de avisos vazia: não muda nada no hub e confere rede e key.
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := c.Events(pctx, nil)
	cancel()
	answer := u.gray("respondeu")
	if err != nil {
		answer = u.warn("não respondeu") + u.gray(": "+err.Error())
		if hub.IsCode(err, hub.CodeUnauthorized) {
			_ = hub.UpdateState(func(s *hub.State) { s.Revoked, s.RevokedAt = err.Error(), u.now() })
		}
	}
	u.printf("bot:      %s\n", u.link("@"+firstNonEmpty(st.Bot, defaultBot)))
	u.printf("hub:      %s · %s\n", c.Host(), answer)
	u.printf("máquina:  %s\n", cfg.Bot.Name())
	u.printf("dono:     %s · %s\n", st.Owner.Who(), u.gray(st.Owner.IDMasked))
	u.printf("conexão:  %s\n", connectionLabel())
	u.printf("ações:    %s\n", u.warn("só leitura"))
	u.printf("contato:  %s\n", u.lastSeen(st.LastSeen))
}

func connectionLabel() string {
	s := "com a TUI aberta"
	if serviceInstalled() {
		s = "serviço: parado"
		if out, _ := exec.Command("systemctl", "is-active", serviceName).Output(); strings.TrimSpace(string(out)) == "active" {
			s = "serviço: ativo"
		}
	}
	if pid := hub.LockHolder(); pid != 0 {
		s += fmt.Sprintf(" · conectada agora (PID %d)", pid)
	}
	return s
}

func (u *botUI) lastSeen(t time.Time) string {
	if t.IsZero() {
		return "ainda não falou com o hub"
	}
	d := u.now().Sub(t)
	var ago string
	switch {
	case d < time.Minute:
		ago = "agora há pouco"
	case d < time.Hour:
		ago = fmt.Sprintf("há %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		ago = fmt.Sprintf("há %d h", int(d.Hours()))
	default:
		ago = fmt.Sprintf("há %d dias", int(d.Hours()/24))
	}
	return ago + u.gray(" ("+t.In(u.now().Location()).Format("02/01 15:04")+")")
}

// ─── devscope bot disconnect ────────────────────────────────────────────────

func (u *botUI) disconnect(ctx context.Context, cfg *config.Config) error {
	key, err := hub.LoadKey()
	if err != nil {
		return err
	}
	if key == "" {
		u.printf("esta máquina não está pareada.\n")
		return nil
	}
	who := "do bot"
	if st := hub.LoadState(); st.Owner != nil {
		who = "de " + st.Owner.Who()
	}
	u.printf("Desconectar esta máquina %s? A key é apagada e o bot deixa de vê-la. [s/N] ", who)
	if yes, err := u.ask(ctx, false); err != nil || !yes {
		return err
	}
	// unauthorized: o hub já não conhece a key, falta só a limpeza local.
	if err := hub.NewClient(cfg.Bot.HubURL, key).Disconnect(ctx); err != nil && !hub.IsCode(err, hub.CodeUnauthorized) {
		return err
	}
	if err := hub.ClearLocal(); err != nil {
		return err
	}
	u.printf("%s desconectada: key e estado local apagados\n", u.check())
	if serviceInstalled() {
		return u.uninstall(ctx, cfg)
	}
	return nil
}

// ─── devscope bot install / uninstall ───────────────────────────────────────

func serviceInstalled() bool {
	_, err := os.Stat(unitPath)
	return err == nil
}

func systemdRunning() bool {
	_, err := os.Stat("/run/systemd/system")
	return runtime.GOOS == "linux" && err == nil
}

func (u *botUI) install(_ context.Context, _ *config.Config) error {
	if !systemdRunning() {
		u.printf("o devscope bot install só sabe usar o systemd, no Linux.\n")
		u.printf("Aqui, mantenha %s rodando por conta própria: tmux, screen, launchd ou o agendador do sistema.\n", u.cmd("devscope bot run"))
		return nil
	}
	usr, err := serviceUser()
	if err != nil {
		return err
	}
	// Com sudo o HOME pode ser o do root: a key é a de quem chamou o sudo.
	if os.Geteuid() == 0 && os.Getenv("SUDO_USER") != "" {
		if err := os.Setenv("HOME", usr.HomeDir); err != nil {
			return err
		}
	}
	if _, _, ok := hub.Paired(); !ok {
		return errors.New("esta máquina não está pareada: rode devscope bot primeiro")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	unit := renderUnit(usr.Username, exe, cfgFile)
	u.warnDockerGroup(usr)

	if os.Geteuid() != 0 {
		u.printf("Sem root eu não mexo no systemd. Rode:\n\n")
		u.printf("sudo tee %s > /dev/null <<'EOF'\n%sEOF\n", unitPath, unit)
		u.printf("sudo systemctl daemon-reload\nsudo systemctl enable --now %s\n", serviceName)
		return nil
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "--now", serviceName); err != nil {
		return err
	}
	u.printf("%s serviço %s instalado e ligado: a máquina volta online sozinha depois de reiniciar\n", u.check(), serviceName)
	return nil
}

func (u *botUI) uninstall(_ context.Context, _ *config.Config) error {
	if !serviceInstalled() {
		u.printf("o serviço %s não está instalado.\n", serviceName)
		return nil
	}
	if os.Geteuid() != 0 {
		u.printf("Para remover o serviço, rode:\n\nsudo systemctl disable --now %s\nsudo rm %s\nsudo systemctl daemon-reload\n", serviceName, unitPath)
		return nil
	}
	if err := systemctl("disable", "--now", serviceName); err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	u.printf("%s serviço %s removido\n", u.check(), serviceName)
	return nil
}

func systemctl(args ...string) error {
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// serviceUser é quem roda o serviço: o usuário atual, ou quem chamou o sudo.
func serviceUser() (*user.User, error) {
	if name := os.Getenv("SUDO_USER"); os.Geteuid() == 0 && name != "" {
		return user.Lookup(name)
	}
	return user.Current()
}

func (u *botUI) warnDockerGroup(usr *user.User) {
	g, err := user.LookupGroup("docker")
	if err != nil || usr.Uid == "0" {
		return
	}
	if ids, _ := usr.GroupIds(); slices.Contains(ids, g.Gid) {
		return
	}
	u.printf("%s %s não está no grupo docker: o serviço não vai enxergar os containers.\n  %s\n\n",
		u.warn("⚠"), usr.Username, u.cmd("sudo usermod -aG docker "+usr.Username))
}

func renderUnit(username, exe, cfg string) string {
	start := systemdQuote(exe)
	if cfg != "" {
		if abs, err := filepath.Abs(cfg); err == nil {
			cfg = abs
		}
		start += " --config " + systemdQuote(cfg)
	}
	return fmt.Sprintf(`[Unit]
Description=DevScope bot: esta máquina conectada ao hub do Telegram
After=network-online.target docker.service
Wants=network-online.target

[Service]
User=%s
ExecStart=%s bot run
Restart=always
RestartSec=5
# 3 = máquina sem pareamento ou revogada: reiniciar não resolve
RestartPreventExitStatus=3

[Install]
WantedBy=multi-user.target
`, username, start)
}

func systemdQuote(s string) string {
	if strings.ContainsAny(s, " \t\"'\\") {
		return strconv.Quote(s)
	}
	return s
}

// ─── devscope bot run ───────────────────────────────────────────────────────

// botService é a máquina conectada sem tela, em mode service. Loga em stderr,
// que o journal guarda.
func botService(*cobra.Command, []string) error {
	log.SetOutput(os.Stderr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Printf("bot: config: %v", err)
		return &exitError{1}
	}
	key, st, ok := hub.Paired()
	if !ok {
		why := "não está pareada"
		if st.Revoked != "" {
			why = "foi recusada pelo hub (" + st.Revoked + ")"
		}
		log.Printf("bot: esta máquina %s: rode devscope bot", why)
		return &exitError{3}
	}

	// A TUI aberta pode estar com o lock: espera ela fechar em vez de sair e
	// deixar o systemd reiniciar a cada 5 s.
	var release func()
	for logged := false; ; logged = true {
		if release, err = hub.AcquireLock(); err == nil {
			break
		}
		var le *hub.LockedError
		if !errors.As(err, &le) {
			log.Printf("bot: %v", err)
			return &exitError{1}
		}
		if !logged {
			log.Printf("bot: %v; espero ele fechar", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
	defer release()

	store := core.NewStateStore(cfg.Scan.Paths)
	manager := collectors.NewManager(store, cfg)
	manager.QuickScan(ctx)
	manager.StartWithContext(ctx)
	log.Printf("bot: %s conectada a %s", cfg.Bot.Name(), hub.NewClient(cfg.Bot.HubURL, key).Host())
	if err := hub.Connect(ctx, key, store, cfg, "service"); err != nil {
		log.Printf("bot: %v", err)
		if hub.IsCode(err, hub.CodeUpgrade) {
			return &exitError{1}
		}
		return &exitError{3}
	}
	return nil
}

func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
