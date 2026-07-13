// Package config loads the hsr TOML configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Exclude         []*regexp.Regexp
	LaunchOverrides map[string][]string
	Interval        int
}

type raw struct {
	Exclude         []string            `toml:"exclude"`
	LaunchOverrides map[string][]string `toml:"launch_overrides"`
	Snapshot        struct {
		Interval int `toml:"interval"`
	} `toml:"snapshot"`
}

// DefaultPath returns $XDG_CONFIG_HOME/hyprland-session-restore/config.toml.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "hyprland-session-restore", "config.toml")
}

// Load reads the config at path; a missing file yields defaults.
func Load(path string) (*Config, error) {
	cfg := &Config{LaunchOverrides: map[string][]string{}, Interval: 60}
	var r raw
	if _, err := toml.DecodeFile(path, &r); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	for _, p := range r.Exclude {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("config exclude %q: %w", p, err)
		}
		cfg.Exclude = append(cfg.Exclude, re)
	}
	if r.LaunchOverrides != nil {
		cfg.LaunchOverrides = r.LaunchOverrides
	}
	if r.Snapshot.Interval > 0 {
		cfg.Interval = r.Snapshot.Interval
	}
	return cfg, nil
}

// IsExcluded reports whether a window class matches any exclude regex.
func (c *Config) IsExcluded(class string) bool {
	for _, re := range c.Exclude {
		if re.MatchString(class) {
			return true
		}
	}
	return false
}
