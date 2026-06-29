// Command pharos is a terminal UI for managing a small fleet of Linux servers
// over SSH: health, basic metrics, Docker containers, and interactive shells.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Vrex123/pharos/internal/config"
	"github.com/Vrex123/pharos/internal/tui"
)

// Build metadata, injected by GoReleaser via -ldflags at release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to the pharos config file")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("pharos %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	// A missing config is fine: pharos starts with an empty fleet and the user
	// can add servers from the UI. Only a malformed config aborts startup.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pharos: %v\n", err)
		os.Exit(1)
	}

	if err := tui.Run(cfg, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "pharos: %v\n", err)
		os.Exit(1)
	}
}
