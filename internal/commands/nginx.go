package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/nginxutil"
	"github.com/spf13/cobra"
)

// nginxCmd é o "Publicar projeto" da aba Nginx sem a TUI: o mesmo plano, as
// mesmas checagens e o mesmo Publish — só a saída é texto.
func nginxCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nginx",
		Short: "Publica projetos no nginx-main (uma .inc por projeto em sites/)",
	}

	var (
		port                  int
		dist, profile, slug   string
		bodySize, readTimeout string
		extras                []string
		dryRun                bool
	)
	publish := &cobra.Command{
		Use:          "publish <projeto>",
		Short:        "Detecta porta e build, escreve sites/<projeto>.inc, testa, recarrega e confere a URL",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := loadNginxHub()
			plan := nginxutil.DetectPlan(projectDirArg(h, args[0]), h)
			flags := map[string]string{"slug": slug, "perfil": profile, "dist": dist, "body": bodySize, "timeout": readTimeout}
			if port > 0 {
				flags["porta"] = fmt.Sprint(port)
			}
			if len(extras) > 0 {
				flags["extras"] = strings.Join(extras, " ")
			}
			for _, field := range nginxutil.PlanFields {
				if v := flags[field]; v != "" {
					if err := plan.Set(field, v); err != nil {
						return err
					}
				}
			}

			out := cmd.OutOrStdout()
			checks := nginxutil.Checks(plan, h)
			printPlan(out, plan, h, checks)
			if c, blocked := nginxutil.Blocked(checks); blocked {
				return fmt.Errorf("não publicado: %s", c.Text)
			}
			if dryRun {
				fmt.Fprintln(out, "\n--dry-run: nada foi escrito")
				return nil
			}
			fmt.Fprintln(out)
			res := nginxutil.Publish(h, plan, func(s nginxutil.Step) { printStep(out, s) })
			return printResult(out, res)
		},
	}
	publish.Flags().IntVar(&port, "port", 0, "porta do back-end no host")
	publish.Flags().StringVar(&dist, "dist", "", "pasta do build do front, relativa ao projeto")
	publish.Flags().StringVar(&profile, "profile", "", "saas | front | api")
	publish.Flags().StringVar(&slug, "slug", "", "prefixo público (padrão: nome da pasta)")
	publish.Flags().StringVar(&bodySize, "body-size", "", "client_max_body_size")
	publish.Flags().StringVar(&readTimeout, "timeout", "", "proxy_read_timeout")
	publish.Flags().StringSliceVar(&extras, "extra", nil, "rota fora do prefixo que redireciona para dentro dele (ex.: /portal/)")
	publish.Flags().BoolVar(&dryRun, "dry-run", false, "só mostra a .inc e as checagens")

	unpublish := &cobra.Command{
		Use:          "unpublish <projeto>",
		Short:        "Remove sites/<projeto>.inc (a anterior fica em .inc.bak), testa e recarrega",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := loadNginxHub()
			plan := nginxutil.DetectPlan(projectDirArg(h, args[0]), h)
			out := cmd.OutOrStdout()
			res := nginxutil.Unpublish(h, plan.Slug, func(s nginxutil.Step) { printStep(out, s) })
			return printResult(out, res)
		},
	}

	root.AddCommand(publish, unpublish)
	return root
}

func loadNginxHub() nginxutil.Hub {
	prefs, _ := config.LoadUserPrefs()
	return nginxutil.LoadHub(prefs.Nginx)
}

// projectDirArg: nome solto é uma pasta dentro de ~/projetos; com barra (ou
// ".") é um caminho.
func projectDirArg(h nginxutil.Hub, arg string) string {
	if arg == "." || strings.ContainsRune(arg, filepath.Separator) {
		abs, err := filepath.Abs(arg)
		if err == nil {
			return abs
		}
	}
	return filepath.Join(h.HostRoot, arg)
}

func printPlan(out io.Writer, p nginxutil.Plan, h nginxutil.Hub, checks []nginxutil.Check) {
	fmt.Fprintf(out, "projeto  %s → %s\n", p.Slug, p.URL(h))
	fmt.Fprintf(out, "perfil   %s\n", p.Profile)
	if p.Profile.HasAPI() {
		fmt.Fprintf(out, "porta    %s · %s\n", firstOr(p.Get("porta"), "—"), firstOr(p.PortFrom, "não detectada"))
	}
	if p.Profile.HasFront() {
		fmt.Fprintf(out, "dist     %s · %s\n", firstOr(p.Dist, "—"), firstOr(p.DistFrom, "não detectado"))
	}
	fmt.Fprintln(out)
	for _, c := range checks {
		fmt.Fprintf(out, "%s %s\n", checkGlyph(c.Level), c.Text)
	}

	inc := nginxutil.Render(p, h)
	old, err := os.ReadFile(h.IncPath(p.Slug))
	if err != nil {
		fmt.Fprintf(out, "\nsites/%s.inc (nova)\n%s", p.Slug, inc)
		return
	}
	fmt.Fprintf(out, "\nsites/%s.inc (já existe — será sobrescrita)\n", p.Slug)
	for _, d := range nginxutil.LineDiff(string(old), inc) {
		fmt.Fprintf(out, "%c %s\n", d.Op, d.Text)
	}
}

func checkGlyph(l nginxutil.Level) string {
	switch l {
	case nginxutil.CheckFail:
		return "✗"
	case nginxutil.CheckWarn:
		return "⚠"
	}
	return "✓"
}

func printStep(out io.Writer, s nginxutil.Step) {
	switch s.State {
	case nginxutil.StepOK:
		fmt.Fprintf(out, "✓ %s\n", s.Name)
	case nginxutil.StepFail:
		fmt.Fprintf(out, "✗ %s — %s\n", s.Name, s.Detail)
	}
}

func printResult(out io.Writer, res nginxutil.Result) error {
	if res.Test != "" && res.Err != nil {
		fmt.Fprintf(out, "\nnginx -t:\n%s\n", res.Test)
	}
	if res.Err != nil {
		return res.Err
	}
	if res.URL != "" {
		fmt.Fprintf(out, "\nno ar: %s\n", res.URL)
	}
	return nil
}

func firstOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
