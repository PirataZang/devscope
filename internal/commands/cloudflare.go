package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/spf13/cobra"
)

// cloudflareCmd guarda o token que o módulo Cloudflare da TUI usa. O token é
// conferido antes de ser salvo: um token que não lista domínios não serve.
func cloudflareCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cloudflare",
		Short: "Conecta o DevScope à sua conta da Cloudflare (só leitura)",
	}
	root.AddCommand(
		&cobra.Command{
			Use:          "login",
			Short:        "Confere e salva um API Token da Cloudflare",
			Args:         cobra.NoArgs,
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Crie um token em https://dash.cloudflare.com/profile/api-tokens")
				fmt.Fprintln(out, "(modelo \"Read all resources\": Zone, DNS, Analytics, SSL, Pages e Security Center com Read).")
				fmt.Fprint(out, "\nToken: ")
				token, err := readSecret(cmd.InOrStdin())
				fmt.Fprintln(out)
				if err != nil {
					return err
				}
				if token == "" {
					return errors.New("nenhum token digitado")
				}
				acct, err := fetchAccount(token)
				if err != nil {
					return err
				}
				if err := collectors.SaveCloudflareToken(token); err != nil {
					return err
				}
				fmt.Fprintf(out, "✓ token salvo em %s\n", collectors.CloudflareTokenPath())
				printAccount(out, acct)
				return nil
			},
		},
		&cobra.Command{
			Use:          "status",
			Short:        "Mostra os domínios que o token enxerga",
			Args:         cobra.NoArgs,
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				out := cmd.OutOrStdout()
				token := collectors.CloudflareToken()
				if token == "" {
					fmt.Fprintln(out, "nenhum token da Cloudflare. Conecte com devscope cloudflare login")
					return nil
				}
				from := collectors.CloudflareTokenPath()
				if os.Getenv("CLOUDFLARE_API_TOKEN") != "" {
					from = "CLOUDFLARE_API_TOKEN"
				}
				fmt.Fprintf(out, "token: %s\n", from)
				acct, err := fetchAccount(token)
				if err != nil {
					return err
				}
				printAccount(out, acct)
				return nil
			},
		},
		&cobra.Command{
			Use:          "logout",
			Short:        "Apaga o token salvo",
			Args:         cobra.NoArgs,
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if err := collectors.SaveCloudflareToken(""); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "✓ token da Cloudflare apagado")
				return nil
			},
		},
	)
	return root
}

// readSecret lê sem eco no terminal; com a entrada redirecionada, lê a linha.
func readSecret(in io.Reader) (string, error) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(f.Fd()) {
		b, err := term.ReadPassword(f.Fd())
		return strings.TrimSpace(string(b)), err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func fetchAccount(token string) (collectors.CFAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return collectors.FetchCloudflareAccount(ctx, token)
}

func printAccount(out io.Writer, acct collectors.CFAccount) {
	fmt.Fprintf(out, "%d domínio(s):\n", len(acct.Zones))
	for _, z := range acct.Zones {
		fmt.Fprintf(out, "  %-30s %s · %s\n", z.Name, z.Status, z.Plan)
	}
	for _, p := range acct.Pages {
		repo := "sem repositório"
		if p.Repo != "" {
			repo = "GitHub " + p.Repo
		}
		fmt.Fprintf(out, "  Pages %-24s %s\n", p.Name, repo)
	}
	if len(acct.Zones) == 0 {
		fmt.Fprintln(out, "  o token não enxerga nenhum domínio: confira a permissão Zone Read")
	}
}
