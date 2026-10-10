// Package config loads escowx settings from config.yaml, creating the file
// with sensible defaults the first time the program runs.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PlaceholderContact is the contact written to a fresh config. NWS asks that
// the User-Agent identify a real way to reach you, so the UI nags until it
// is changed.
const PlaceholderContact = "you@example.com"

type Location struct {
	Name    string  `yaml:"name"`
	Lat     float64 `yaml:"lat"`
	Lon     float64 `yaml:"lon"`
	Primary bool    `yaml:"primary,omitempty"`
}

type Refresh struct {
	Alerts   time.Duration `yaml:"alerts"`
	Forecast time.Duration `yaml:"forecast"`
}

type Config struct {
	Contact   string     `yaml:"contact"`
	Refresh   Refresh    `yaml:"refresh"`
	LogFile   string     `yaml:"log_file"`
	Bell      bool       `yaml:"bell"`
	Notify    bool       `yaml:"notify"`
	Locations []Location `yaml:"locations"`

	// path is where the config was loaded from; relative paths in the
	// config (log_file) are resolved against its directory.
	path string
}

const defaultYAML = `# escowx configuration.
# Created automatically with defaults; edit freely and restart escowx.

# api.weather.gov requires a User-Agent that identifies you. Put an email
# address or website here so NWS can reach you if your traffic misbehaves.
# Can also be overridden with --contact or the ESCOWX_CONTACT env var.
contact: ` + PlaceholderContact + `

refresh:
  alerts: 60s     # active alerts
  forecast: 10m   # observations + forecasts

# Mode changes are appended to a log. The default location is
# $XDG_STATE_HOME/escowx/escowx.log (~/.local/state/escowx/escowx.log).
# Uncomment to override; relative paths are relative to this file.
# log_file: ~/escowx.log

bell: true     # ring the terminal bell when a location enters TAKE COVER
notify: true   # best-effort desktop notification (notify-send / osascript)

# The location marked primary is shown in the large panel. If none is marked,
# the first one is used. Coordinates are rounded to 4 decimals for the API.
locations:
  - { name: Escondido,     lat: 33.1192, lon: -117.0864, primary: true }
  - { name: San Marcos,    lat: 33.1434, lon: -117.1661 }
  - { name: Valley Center, lat: 33.2184, lon: -117.0342 }
  - { name: Ramona,        lat: 33.0417, lon: -116.8681 }
  - { name: Poway,         lat: 32.9628, lon: -117.0359 }
  - { name: Vista,         lat: 33.2000, lon: -117.2425 }
  - { name: Oceanside,     lat: 33.1959, lon: -117.3795 }
  - { name: San Diego,     lat: 32.7157, lon: -117.1611 }
  - { name: Julian,        lat: 33.0787, lon: -116.6020 }
`

// DefaultPath is $XDG_CONFIG_HOME/escowx/config.yaml (~/.config/escowx on
// Linux, the platform equivalent elsewhere).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "escowx", "config.yaml"), nil
}

// defaultLogPath is $XDG_STATE_HOME/escowx/escowx.log, falling back to
// ~/.local/state, which is where the XDG spec puts logs.
func defaultLogPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "escowx.log"
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "escowx", "escowx.log")
}

// Load reads the config at path, writing the defaults there first if the
// file does not exist. created reports whether that happened.
func Load(path string) (cfg *Config, created bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, false, fmt.Errorf("creating config dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultYAML), 0o644); err != nil {
			return nil, false, fmt.Errorf("writing default config: %w", err)
		}
		data, created = []byte(defaultYAML), true
	} else if err != nil {
		return nil, false, err
	}
	cfg, err = Parse(data)
	if err != nil {
		return nil, created, fmt.Errorf("%s: %w", path, err)
	}
	cfg.path = path
	return cfg, created, nil
}

// Parse decodes and validates config YAML, filling in defaults for any
// omitted settings.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{
		Contact: PlaceholderContact,
		Refresh: Refresh{Alerts: time.Minute, Forecast: 10 * time.Minute},
		Bell:    true,
		Notify:  true,
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if len(cfg.Locations) == 0 {
		return nil, errors.New("no locations configured")
	}
	if cfg.Refresh.Alerts < 15*time.Second {
		cfg.Refresh.Alerts = 15 * time.Second
	}
	if cfg.Refresh.Forecast < time.Minute {
		cfg.Refresh.Forecast = time.Minute
	}
	primary := -1
	for i, l := range cfg.Locations {
		if l.Name == "" {
			return nil, fmt.Errorf("location %d has no name", i+1)
		}
		if l.Lat < -90 || l.Lat > 90 || l.Lon < -180 || l.Lon > 180 {
			return nil, fmt.Errorf("location %q has invalid coordinates", l.Name)
		}
		if l.Primary && primary < 0 {
			primary = i
		}
		cfg.Locations[i].Primary = false
	}
	if primary < 0 {
		primary = 0
	}
	// Keep the primary location first so index 0 always means "primary".
	p := cfg.Locations[primary]
	p.Primary = true
	cfg.Locations = append([]Location{p}, append(cfg.Locations[:primary:primary], cfg.Locations[primary+1:]...)...)
	return cfg, nil
}

// LogPath returns the mode-change log path: the XDG state default when
// log_file is unset, otherwise log_file with ~ expanded and relative paths
// resolved against the config file's directory.
func (c *Config) LogPath() string {
	p := c.LogFile
	if p == "" {
		return defaultLogPath()
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(c.path), p)
}
