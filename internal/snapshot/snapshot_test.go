package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ThiagoAVicente/wayland-session-restore/internal/config"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/hypr"
)

func fakeClient() hypr.Client {
	var c hypr.Client
	c.Class = "foot"
	c.Title = "foot"
	c.Pid = 123
	c.Floating = true
	c.At = [2]int{10, 20}
	c.Size = [2]int{800, 600}
	c.Workspace.Id = 2
	return c
}

func TestTake(t *testing.T) {
	snap := Take(
		[]hypr.Client{fakeClient()},
		&config.Config{},
		func(pid int) []string { return []string{"foot"} },
		func(pid int) string { return "/home/x/proj" },
	)
	if snap.Version != 1 || len(snap.Clients) != 1 {
		t.Fatalf("bad snapshot: %+v", snap)
	}
	c := snap.Clients[0]
	if c.Class != "foot" || c.Workspace != 2 || !c.Floating ||
		c.At != [2]int{10, 20} || c.Size != [2]int{800, 600} ||
		c.Cwd != "/home/x/proj" || len(c.Cmdline) != 1 {
		t.Fatalf("bad client: %+v", c)
	}
}

func TestTakeSkipsClientsWithoutCmdline(t *testing.T) {
	snap := Take(
		[]hypr.Client{fakeClient()},
		&config.Config{},
		func(pid int) []string { return nil },
		func(pid int) string { return "" },
	)
	if len(snap.Clients) != 0 {
		t.Fatalf("want 0 clients, got %+v", snap.Clients)
	}
}

func TestTakeRespectsExclude(t *testing.T) {
	cfg := &config.Config{Exclude: []*regexp.Regexp{regexp.MustCompile("(?i)^foot$")}}
	snap := Take(
		[]hypr.Client{fakeClient()},
		cfg,
		func(pid int) []string { return []string{"foot"} },
		func(pid int) string { return "" },
	)
	if len(snap.Clients) != 0 {
		t.Fatalf("want 0 clients, got %+v", snap.Clients)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state", "session.json")
	if err := Write(&Session{Version: 1}, target); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil || s.Version != 1 {
		t.Fatalf("bad state file: %v %+v", err, s)
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestStatePathRespectsXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/x")
	if got := StatePath(); got != "/x/hyprland-session-restore/session.json" {
		t.Fatalf("got %q", got)
	}
}
