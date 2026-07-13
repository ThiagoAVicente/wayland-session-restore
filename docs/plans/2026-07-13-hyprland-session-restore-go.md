# Hyprland Session Restore — Go Implementation Plan

> **For agentic workers:** Execute task-by-task; each task = tests first, verify fail, implement, verify pass, commit.

**Goal:** Single static binary `hsr` that snapshots the Hyprland session (windows, workspaces, terminal cwds, launch commands) periodically and restores it on login.

**Architecture:** Go module with packages: `internal/hypr` (hyprctl JSON wrapper, injectable runner), `internal/procfs` (/proc cmdline + deepest-descendant cwd), `internal/config` (TOML config, exclude regexes), `internal/snapshot` (capture → atomic JSON state file), `internal/restore` (state → `hyprctl dispatch exec` with workspace/float rules), `cmd/hsr` (subcommands snapshot/restore/watch). Terminals get their cwd back because restore wraps commands in `sh -c 'cd CWD && exec CMD'` — terminals spawn their shell in their own cwd.

**Tech Stack:** Go ≥1.22. Single dep: `github.com/BurntSushi/toml`. Tests: stdlib `testing`.

**Non-goals:** other compositors, D-Bus, window content, exact tiling layout (workspace + floating geometry only), event-socket watching.

## State file schema

`~/.local/state/hyprland-session-restore/session.json` (respects `$XDG_STATE_HOME`):

```json
{
  "version": 1,
  "clients": [
    {"class": "foot", "title": "foot", "workspace": 1, "floating": false,
     "at": [21, 625], "size": [928, 554], "pinned": false, "fullscreen": 0,
     "cmdline": ["foot"], "cwd": "/home/vcnt/Projects/ss"}
  ]
}
```

## Config file

`~/.config/hyprland-session-restore/config.toml` (respects `$XDG_CONFIG_HOME`), all keys optional. Exclusion applies at BOTH snapshot and restore time:

```toml
# Case-insensitive regexes matched against window class.
exclude = ["^Rofi$", "^steam_", "polkit"]

[launch_overrides]
firefox = ["firefox"]

[snapshot]
interval = 60   # seconds, for `hsr watch`
```

## Tasks

- **G1 scaffold:** `go.mod` (module `github.com/ThiagoAVicente/wayland-session-restore`), `cmd/hsr/main.go` stub, builds clean.
- **G2 config:** `internal/config` — Load with defaults, `(?i)` regexes, IsExcluded, DefaultPath (XDG).
- **G3 procfs:** `internal/procfs` — Cmdline(procRoot, pid), DeepestCwd(procRoot, pid) walking ppid tree from /proc/*/stat, newest child on ties.
- **G4 hypr:** `internal/hypr` — Client struct, Clients() filtering unmapped/pid<=0/special workspaces (id <= -90), DispatchExec(argv, rules) building `[rules] cmd` with POSIX shell quoting; runner injectable for tests.
- **G5 snapshot:** `internal/snapshot` — Take(cfg), atomic Write, StatePath (XDG_STATE_HOME).
- **G6 restore:** `internal/restore` — BuildLaunch (override > cmdline; sh -c cd wrap; rules `workspace N silent[;float;move X Y;size W H][;pin][;fullscreen]`), Restore(session, cfg, dryRun).
- **G7 cli:** `cmd/hsr` — subcommands snapshot / restore [--dry-run] / watch [--interval], global --config, --state-file.
- **G8 docs:** full README (vibecoded badge), examples/config.toml, systemd units, CI build workflow.
