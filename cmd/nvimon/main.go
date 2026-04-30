package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/prods/nvimon/internal/collector"
	"github.com/prods/nvimon/internal/config"
	"github.com/prods/nvimon/internal/model"
	"github.com/prods/nvimon/internal/transport/httpapi"
	"github.com/prods/nvimon/internal/tui"
)

func main() {
	once := flag.Bool("once", false, "collect one snapshot and print a summary")
	jsonOutput := flag.Bool("json", false, "print snapshot as JSON in non-interactive mode")
	configPath := flag.String("config", config.DefaultPath(), "path to config file")
	remoteSnapshot := flag.String("remote-snapshot", "", "fetch a single snapshot from a remote agent URL")
	remoteToken := flag.String("remote-token", "", "token to use with --remote-snapshot")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	c := collector.NewLocalCollector(cfg.RefreshInterval)

	if *once || *remoteSnapshot != "" || !term.IsTerminal(int(os.Stdout.Fd())) {
		snapshot, err := collectSingleSnapshot(context.Background(), c, cfg, *remoteSnapshot, *remoteToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "collect snapshot: %v\n", err)
			os.Exit(1)
		}
		if err := printSnapshotBuffer(os.Stdout, snapshot, *jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "print snapshot: %v\n", err)
			os.Exit(1)
		}
		return
	}

	program := tea.NewProgram(
		tui.NewFromConfig(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run tui: %v\n", err)
		os.Exit(1)
	}
}

func collectSingleSnapshot(ctx context.Context, local collector.Collector, cfg config.Config, remoteURL, remoteToken string) (model.HostSnapshot, error) {
	if remoteURL == "" {
		return local.Collect(ctx)
	}

	client := httpapi.Client{
		BaseURL:   remoteURL,
		AuthToken: remoteToken,
		HTTPClient: &http.Client{
			Timeout: cfg.Timeouts.Request,
		},
	}
	return client.Snapshot(ctx)
}

func printSnapshotBuffer(out io.Writer, snapshot model.HostSnapshot, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(snapshot)
	}

	_, err := fmt.Fprintf(
		out,
		"nvimon snapshot\nhost=%s backend=%s cpu=%s ram=%s/%s gpus=%d errors=%d\n",
		snapshot.Hostname,
		snapshot.GPUBackend,
		model.FormatPercent(snapshot.CPUUsedPct),
		model.FormatBytes(snapshot.RAMUsedBytes),
		model.FormatBytes(snapshot.RAMTotalBytes),
		snapshot.GPUCount(),
		len(snapshot.CollectorErrors),
	)
	return err
}
