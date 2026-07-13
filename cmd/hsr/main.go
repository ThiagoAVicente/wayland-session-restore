// Command hsr snapshots and restores Hyprland sessions.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ThiagoAVicente/wayland-session-restore/internal/config"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/hypr"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/procfs"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/restore"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/snapshot"
)

const version = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `hsr %s - Hyprland session snapshot & restore

Usage:
  hsr [flags] snapshot            capture current session
  hsr [flags] restore [--dry-run] relaunch saved session
  hsr [flags] watch [--interval N] snapshot every N seconds

Flags:
  --config PATH      config file (default %s)
  --state-file PATH  state file (default %s)
  --version
`, version, config.DefaultPath(), snapshot.StatePath())
}

func takeAndWrite(cfg *config.Config, stateFile string) error {
	clients, err := hypr.Clients()
	if err != nil {
		return err
	}
	sess := snapshot.Take(clients, cfg,
		func(pid int) []string { return procfs.Cmdline(procfs.Root, pid) },
		func(pid int) string { return procfs.DeepestCwd(procfs.Root, pid) },
	)
	return snapshot.Write(sess, stateFile)
}

func run() error {
	global := flag.NewFlagSet("hsr", flag.ExitOnError)
	global.Usage = usage
	configPath := global.String("config", config.DefaultPath(), "config file")
	stateFile := global.String("state-file", snapshot.StatePath(), "state file")
	showVersion := global.Bool("version", false, "print version")
	global.Parse(os.Args[1:])

	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if global.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	cmd, rest := global.Arg(0), global.Args()[1:]
	switch cmd {
	case "snapshot":
		return takeAndWrite(cfg, *stateFile)

	case "restore":
		fs := flag.NewFlagSet("restore", flag.ExitOnError)
		dryRun := fs.Bool("dry-run", false, "print commands instead of dispatching")
		fs.Parse(rest)
		raw, err := os.ReadFile(*stateFile)
		if err != nil {
			return fmt.Errorf("no snapshot at %s: %w", *stateFile, err)
		}
		var sess snapshot.Session
		if err := json.Unmarshal(raw, &sess); err != nil {
			return fmt.Errorf("parse %s: %w", *stateFile, err)
		}
		n, err := restore.Restore(&sess, cfg, *dryRun, hypr.DispatchExec)
		if err != nil {
			return err
		}
		fmt.Printf("restored %d clients\n", n)
		return nil

	case "watch":
		fs := flag.NewFlagSet("watch", flag.ExitOnError)
		interval := fs.Int("interval", cfg.Interval, "seconds between snapshots")
		fs.Parse(rest)
		for {
			if err := takeAndWrite(cfg, *stateFile); err != nil {
				fmt.Fprintln(os.Stderr, "snapshot:", err)
			}
			time.Sleep(time.Duration(*interval) * time.Second)
		}

	default:
		usage()
		os.Exit(2)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hsr:", err)
		os.Exit(1)
	}
}
