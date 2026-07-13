# wayland-session-restore

![vibecoded](https://img.shields.io/badge/vibecoded-100%25-ff69b4?style=for-the-badge)

Session restore for Hyprland. X11 had session management; Wayland killed it and nothing replaced it. This tool snapshots your session periodically via Hyprland IPC (clients, workspaces, terminal cwds via /proc, launch commands) and relaunches + repositions everything on startup.

WIP — see docs/plans/.
