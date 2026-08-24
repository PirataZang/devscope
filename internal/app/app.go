package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/ui"
)

// redirectLogOutput moves the standard logger away from stderr and into a
// file. Background collectors (docker, scanner, health...) call log.Printf
// on failure; if that lands on stderr it writes straight onto the terminal
// the TUI's alt screen is using, outside Bubble Tea's control. Bubble Tea's
// renderer tracks how many lines it painted to reposition the cursor on the
// next frame — a stray line from a raw log write throws that count off,
// which is what causes panels to look stacked/duplicated mid-session.
func redirectLogOutput() {
	path := config.LogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			log.SetOutput(f)
			return
		}
	}
	log.SetOutput(io.Discard)
}

func Run(cfgFile string, debug bool) error {
	redirectLogOutput()

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
