package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/metrics"
	"github.com/devscope/devscope/internal/scanner"
	"github.com/spf13/cobra"
)

func scanCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan projects and print snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ctx := context.Background()
			s := scanner.New(cfg.Scan.Paths, cfg.Scan.MaxDepth, cfg.Scan.Ignore)
			projects, err := s.Scan(ctx)
			if err != nil {
				return err
			}
			projects = s.MergeDiscovered(ctx, projects)
			projects = collectors.FilterNestedProjects(projects)
			projects = collectors.EnrichSnapshot(ctx, cfg, projects)

			host := metrics.NewHostCollector().Collect()
			snap := core.Snapshot{
				Projects:     projects,
				HostMetrics:  host,
				ScannedAt:    time.Now(),
				ScanPaths:    cfg.Scan.Paths,
				ProjectCount: len(projects),
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(snap)
			}

			fmt.Printf("DevScope scan — %d projects\n", len(projects))
			for _, p := range projects {
				fmt.Printf("  %-24s %-10s %-10s %s\n", p.Name, p.Status, p.Framework.Name, p.Path)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return cmd
}

func watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch project status in the terminal (no TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ctx := context.Background()
			s := scanner.New(cfg.Scan.Paths, cfg.Scan.MaxDepth, cfg.Scan.Ignore)

			for {
				projects, err := s.FastScan(ctx)
				if err != nil {
					return err
				}
				projects = s.MergeDiscovered(ctx, projects)
				projects = collectors.EnrichSnapshot(ctx, cfg, projects)

				fmt.Print("\033[H\033[2J")
				fmt.Printf("DevScope watch — %s — %d projects\n\n", time.Now().Format("15:04:05"), len(projects))
				fmt.Printf("%-20s %-10s %-8s %-8s %s\n", "NAME", "STATUS", "CPU", "RAM", "PORTS")
				for _, p := range projects {
					cpu := "-"
					if p.Metrics.CPUPercent > 0 {
						cpu = fmt.Sprintf("%.0f%%", p.Metrics.CPUPercent)
					}
					ram := "-"
					if p.Metrics.MemoryMB > 0 {
						ram = fmt.Sprintf("%dM", p.Metrics.MemoryMB)
					}
					ports := collectors.FormatPortsShort(p.Ports, 2)
					fmt.Printf("%-20s %-10s %-8s %-8s %s\n", p.Name, p.Status, cpu, ram, ports)
				}
				time.Sleep(cfg.Refresh.MetricsInterval)
			}
		},
	}
}
