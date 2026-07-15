package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/ui"
)

func Run(cfgFile string, debug bool) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if debug {
		fmt.Fprintln(os.Stderr, "debug mode enabled")
		fmt.Fprintf(os.Stderr, "scan paths: %v\n", cfg.Scan.Paths)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := core.NewStateStore(cfg.Scan.Paths)
	manager := collectors.NewManager(store, cfg)
	manager.QuickScan(ctx)
	manager.StartWithContext(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	program := ui.NewApp(store, cfg)
	if err := program.Run(); err != nil {
		return err
	}

	cancel()
	return nil
}
