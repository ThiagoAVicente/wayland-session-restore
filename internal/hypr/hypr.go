// Package hypr talks to the Hyprland IPC via hyprctl.
package hypr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Special workspaces (scratchpads) have ids like -98/-99; skip them.
const specialWorkspaceThreshold = -90

type Client struct {
	Class      string `json:"class"`
	Title      string `json:"title"`
	Pid        int    `json:"pid"`
	Mapped     bool   `json:"mapped"`
	Floating   bool   `json:"floating"`
	Pinned     bool   `json:"pinned"`
	Fullscreen int    `json:"fullscreen"`
	At         [2]int `json:"at"`
	Size       [2]int `json:"size"`
	Workspace  struct {
		Id int `json:"id"`
	} `json:"workspace"`
}

func defaultRun(args ...string) ([]byte, error) {
	out, err := exec.Command("hyprctl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

var run = defaultRun

// Clients returns mapped, real-pid clients on regular workspaces.
func Clients() ([]Client, error) {
	out, err := run("-j", "clients")
	if err != nil {
		return nil, err
	}
	var all []Client
	if err := json.Unmarshal(out, &all); err != nil {
		return nil, fmt.Errorf("parse clients: %w", err)
	}
	var cs []Client
	for _, c := range all {
		if c.Mapped && c.Pid > 0 && c.Workspace.Id > specialWorkspaceThreshold {
			cs = append(cs, c)
		}
	}
	return cs, nil
}

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

// DispatchExec launches argv via `hyprctl dispatch exec`, optionally
// prefixed with window rules like "workspace 3 silent;float".
func DispatchExec(argv []string, rules string) error {
	cmd := shJoin(argv)
	if rules != "" {
		cmd = fmt.Sprintf("[%s] %s", rules, cmd)
	}
	_, err := run("dispatch", "exec", cmd)
	return err
}
