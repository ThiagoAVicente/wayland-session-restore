package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenNoFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Exclude) != 0 || len(cfg.LaunchOverrides) != 0 || cfg.Interval != 60 {
		t.Fatalf("bad defaults: %+v", cfg)
	}
}

func TestLoadFullConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte(`
exclude = ["^Rofi$", "steam"]
[launch_overrides]
firefox = ["firefox"]
[snapshot]
interval = 30
`), 0o644)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Exclude) != 2 {
		t.Fatalf("want 2 exclude regexes, got %d", len(cfg.Exclude))
	}
	if got := cfg.LaunchOverrides["firefox"]; len(got) != 1 || got[0] != "firefox" {
		t.Fatalf("bad override: %v", got)
	}
	if cfg.Interval != 30 {
		t.Fatalf("interval = %d", cfg.Interval)
	}
}

func TestIsExcludedCaseInsensitive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte(`exclude = ["^rofi$"]`), 0o644)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsExcluded("Rofi") {
		t.Error("Rofi should be excluded")
	}
	if cfg.IsExcluded("firefox") {
		t.Error("firefox should not be excluded")
	}
}

func TestBadRegexErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte(`exclude = ["["]`), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("want error for invalid regex")
	}
}

func TestDefaultPathRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if got := DefaultPath(); got != "/x/hyprland-session-restore/config.toml" {
		t.Fatalf("got %q", got)
	}
}
