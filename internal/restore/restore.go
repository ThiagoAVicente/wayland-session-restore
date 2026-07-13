// Package restore relaunches a saved session via Hyprland dispatch rules.
package restore

import (
	"fmt"
	"strings"

	"github.com/ThiagoAVicente/wayland-session-restore/internal/config"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/snapshot"
)

// shQuote quotes s for POSIX sh when needed.
func shQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$&|;<>(){}*?#~`!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shQuote(a)
	}
	return strings.Join(parts, " ")
}

// BuildLaunch returns the argv and hyprland exec rules for a saved client.
// Launch overrides take precedence over the captured cmdline; a saved cwd
// wraps the command in `sh -c 'cd ... && exec ...'` so terminals reopen
// where the user was working.
func BuildLaunch(c snapshot.SessionClient, cfg *config.Config) ([]string, string) {
	argv := c.Cmdline
	if o, ok := cfg.LaunchOverrides[c.Class]; ok {
		argv = o
	}
	if c.Cwd != "" {
		argv = []string{"sh", "-c", fmt.Sprintf("cd %s && exec %s", shQuote(c.Cwd), shJoin(argv))}
	}
	rules := fmt.Sprintf("workspace %d silent", c.Workspace)
	if c.Floating {
		rules += fmt.Sprintf(";float;move %d %d;size %d %d", c.At[0], c.At[1], c.Size[0], c.Size[1])
	}
	if c.Pinned {
		rules += ";pin"
	}
	if c.Fullscreen != 0 {
		rules += ";fullscreen"
	}
	return argv, rules
}

// Restore dispatches every non-excluded client. With dryRun it only prints.
// Returns the number of clients restored (or printed).
func Restore(sess *snapshot.Session, cfg *config.Config, dryRun bool, dispatch func(argv []string, rules string) error) (int, error) {
	n := 0
	for _, c := range sess.Clients {
		if cfg.IsExcluded(c.Class) {
			continue
		}
		argv, rules := BuildLaunch(c, cfg)
		if dryRun {
			fmt.Printf("[%s] %s\n", rules, shJoin(argv))
		} else if err := dispatch(argv, rules); err != nil {
			return n, fmt.Errorf("restore %s: %w", c.Class, err)
		}
		n++
	}
	return n, nil
}
