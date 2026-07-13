package restore

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ThiagoAVicente/wayland-session-restore/internal/config"
	"github.com/ThiagoAVicente/wayland-session-restore/internal/snapshot"
)

func client() snapshot.SessionClient {
	return snapshot.SessionClient{
		Class:     "foot",
		Workspace: 3,
		At:        [2]int{10, 20},
		Size:      [2]int{800, 600},
		Cmdline:   []string{"foot"},
		Cwd:       "/home/x/proj",
	}
}

func TestBuildLaunchWrapsCwdInSh(t *testing.T) {
	argv, rules := BuildLaunch(client(), &config.Config{})
	if len(argv) != 3 || argv[0] != "sh" || argv[1] != "-c" {
		t.Fatalf("argv = %v", argv)
	}
	if argv[2] != "cd /home/x/proj && exec foot" {
		t.Fatalf("argv[2] = %q", argv[2])
	}
	if rules != "workspace 3 silent" {
		t.Fatalf("rules = %q", rules)
	}
}

func TestBuildLaunchNoCwd(t *testing.T) {
	c := client()
	c.Cwd = ""
	argv, _ := BuildLaunch(c, &config.Config{})
	if len(argv) != 1 || argv[0] != "foot" {
		t.Fatalf("argv = %v", argv)
	}
}

func TestBuildLaunchFloatingRules(t *testing.T) {
	c := client()
	c.Floating, c.Pinned, c.Fullscreen = true, true, 1
	_, rules := BuildLaunch(c, &config.Config{})
	want := "workspace 3 silent;float;move 10 20;size 800 600;pin;fullscreen"
	if rules != want {
		t.Fatalf("rules = %q, want %q", rules, want)
	}
}

func TestBuildLaunchOverride(t *testing.T) {
	c := client()
	c.Cwd = ""
	cfg := &config.Config{LaunchOverrides: map[string][]string{"foot": {"kitty"}}}
	argv, _ := BuildLaunch(c, cfg)
	if len(argv) != 1 || argv[0] != "kitty" {
		t.Fatalf("argv = %v", argv)
	}
}

func TestRestoreDispatchesAndSkipsExcluded(t *testing.T) {
	rofi := client()
	rofi.Class = "Rofi"
	rofi.Cmdline = []string{"rofi"}
	sess := &snapshot.Session{Version: 1, Clients: []snapshot.SessionClient{client(), rofi}}
	cfg := &config.Config{Exclude: []*regexp.Regexp{regexp.MustCompile("(?i)^rofi$")}}

	var dispatched []string
	n, err := Restore(sess, cfg, false, func(argv []string, rules string) error {
		dispatched = append(dispatched, strings.Join(argv, " "))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(dispatched) != 1 {
		t.Fatalf("n=%d dispatched=%v", n, dispatched)
	}
}

func TestRestoreDryRunDoesNotDispatch(t *testing.T) {
	sess := &snapshot.Session{Version: 1, Clients: []snapshot.SessionClient{client()}}
	n, err := Restore(sess, &config.Config{}, true, func(argv []string, rules string) error {
		t.Fatal("dispatch called in dry run")
		return nil
	})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
