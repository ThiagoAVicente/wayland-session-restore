// Package snapshot captures the current Hyprland session to a JSON state file.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ThiagoAVicente/wayland-session-restore/internal/config"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/hypr"
)

type SessionClient struct {
	Class      string   `json:"class"`
	Title      string   `json:"title"`
	Workspace  int      `json:"workspace"`
	Floating   bool     `json:"floating"`
	At         [2]int   `json:"at"`
	Size       [2]int   `json:"size"`
	Pinned     bool     `json:"pinned"`
	Fullscreen int      `json:"fullscreen"`
	Cmdline    []string `json:"cmdline"`
	Cwd        string   `json:"cwd,omitempty"`
}

type Session struct {
	Version int             `json:"version"`
	Clients []SessionClient `json:"clients"`
}

// StatePath returns $XDG_STATE_HOME/hyprland-session-restore/session.json.
func StatePath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "hyprland-session-restore", "session.json")
}

// Take builds a Session from clients, resolving cmdline and cwd per pid.
// Clients matching cfg excludes or lacking a cmdline are skipped.
func Take(clients []hypr.Client, cfg *config.Config, cmdline func(pid int) []string, cwd func(pid int) string) *Session {
	s := &Session{Version: 1, Clients: []SessionClient{}}
	for _, c := range clients {
		if cfg.IsExcluded(c.Class) {
			continue
		}
		argv := cmdline(c.Pid)
		if len(argv) == 0 {
			continue
		}
		s.Clients = append(s.Clients, SessionClient{
			Class:      c.Class,
			Title:      c.Title,
			Workspace:  c.Workspace.Id,
			Floating:   c.Floating,
			At:         c.At,
			Size:       c.Size,
			Pinned:     c.Pinned,
			Fullscreen: c.Fullscreen,
			Cmdline:    argv,
			Cwd:        cwd(c.Pid),
		})
	}
	return s
}

// Write persists the session atomically (tmp file + rename).
func Write(s *Session, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}
