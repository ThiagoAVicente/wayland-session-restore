# wayland-session-restore

![vibecoded](https://img.shields.io/badge/vibecoded-100%25-ff69b4?style=for-the-badge)
![ci](https://github.com/ThiagoAVicente/wayland-session-restore/actions/workflows/ci.yml/badge.svg)

Session restore for Hyprland. X11 had session management; Wayland killed it and nothing replaced it. Reboot and you lose everything: which apps were open, on which workspaces, which directories your terminals were in. `hsr` snapshots your session periodically via Hyprland IPC and brings it all back on login — single static binary, one config file.

## How it works

**Snapshot**: `hyprctl -j clients` gives the list of windows, their workspaces, and geometry. `/proc/PID/cmdline` gives the launch command for each window's process. For terminals, the terminal process itself keeps its launch cwd, so instead `hsr` walks the deepest-descendant chain in `/proc/*/cwd` to capture the cwd of the shell or editor actually running inside the terminal.

**Restore**: each window is relaunched via `hyprctl dispatch exec "[workspace N silent;float;move X Y;size W H] cmd"`. Commands are wrapped as `sh -c 'cd DIR && exec CMD'` so that terminals reopen in the right directory — since a terminal spawns its shell in its own cwd, `cd`-ing before `exec`-ing the terminal binary restores that context.

## Install

```sh
go install github.com/ThiagoAVicente/wayland-session-restore/cmd/hsr@latest
```

or clone and build locally:

```sh
git clone https://github.com/ThiagoAVicente/wayland-session-restore
cd wayland-session-restore
go build -o ~/.local/bin/hsr ./cmd/hsr
```

## Usage

```
hsr [flags] snapshot            capture current session
hsr [flags] restore [--dry-run] relaunch saved session
hsr [flags] watch [--interval N] snapshot every N seconds

Flags:
  --config PATH      config file (default ~/.config/hyprland-session-restore/config.toml)
  --state-file PATH  state file (default XDG state dir)
  --version
```

Use `hsr restore --dry-run` to preview the exact `hyprctl dispatch exec` commands without launching anything.

## Autostart

Add to your `hyprland.conf`:

```
exec-once = hsr restore
exec-once = hsr watch
```

This restores the last snapshot on login and then keeps snapshotting in the background for next time.

### Alternative: systemd timer

Instead of `hsr watch`, you can use the bundled systemd user units to snapshot on a timer:

```sh
cp systemd/hyprland-session-restore.{service,timer} ~/.config/systemd/user/
systemctl --user enable --now hyprland-session-restore.timer
```

Note the service's `ExecStart` assumes `hsr` is installed at `~/.local/bin/hsr`; edit the unit if you installed it elsewhere.

## Configuration

See [`examples/config.toml`](examples/config.toml) for a full annotated example. Place your copy at `~/.config/hyprland-session-restore/config.toml`.

| Key | Type | Description |
|---|---|---|
| `exclude` | list of regex | Case-insensitive regexes matched against window class. Matching windows are skipped at **both** snapshot and restore time. |
| `launch_overrides` | table of class → argv list | Override the command used to relaunch windows of a given class, instead of the command line captured at snapshot time. |
| `snapshot.interval` | int (seconds) | How often `hsr watch` takes a snapshot. |

## Limitations

- **Hyprland only.** Session snapshotting relies on Hyprland's IPC (`hyprctl`); there's no generic Wayland protocol for window enumeration or placement, so this won't work on other compositors.
- **Apps must be relaunchable from their cmdline.** Single-instance apps, apps requiring a running daemon, or apps that don't accept meaningful CLI args may not restore correctly.
- **Window content isn't restored.** Browser tabs, editor buffers, etc. are not captured — use the app's own session restore feature for that (e.g. Firefox's "restore previous session").
- **Exact tiled layout isn't reproduced.** Windows land on the correct workspace and floating geometry is preserved, but Hyprland's dynamic tiling order/splits are not recreated.

## License

MIT
