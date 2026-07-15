package commands

import (
	"fmt"

	"github.com/devscope/devscope/internal/app"
	"github.com/devscope/devscope/pkg/version"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "devscope",
	Short: "htop dos projetos — visualize todos os projetos na sua VPS",
	Long: `DevScope é uma TUI que agrupa containers, serviços, git, deploy,
health e métricas sob a abstração de Projeto.

Escaneia automaticamente diretórios e mostra tudo em uma interface fluida.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.Run(cfgFile, debug)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/devscope/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("devscope", version.String())
		},
	})

	rootCmd.AddCommand(scanCmd())
	rootCmd.AddCommand(watchCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
