package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Config mirrors the YAML structure used for defaults and overrides.
type Config struct {
	IntervalSeconds int    `yaml:"interval_seconds"`
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
	RefreshSeconds  int    `yaml:"refresh_seconds"`
	ThemeName       string `yaml:"theme"`

	Concurrency ConcurrencyConfig      `yaml:"concurrency"`
	Memory      MemoryConfig           `yaml:"memory"`
	Ping        PingConfig             `yaml:"ping"`
	Theme       ThemeConfig            `yaml:"theme"`
	Themes      map[string]ThemeConfig `yaml:"themes"`
}

type ConcurrencyConfig struct {
	MaxConcurrentPings int `yaml:"max_concurrent_pings"`
	MaxHosts           int `yaml:"max_hosts"`
	PingQueueCapacity  int `yaml:"ping_queue_capacity"`
}

type MemoryConfig struct {
	MaxHostsTracked int `yaml:"max_hosts_tracked"`
}

type PingConfig struct {
	Backend       string   `yaml:"backend"`
	SystemCommand string   `yaml:"system_command"`
	SystemArgs    []string `yaml:"system_args"`
}

type ThemeConfig struct {
	TitleBackground        string `yaml:"title_background"`
	TitleForeground        string `yaml:"title_foreground"`
	StatusBackground       string `yaml:"status_background"`
	StatusForeground       string `yaml:"status_foreground"`
	HeaderBackground       string `yaml:"header_background"`
	HeaderForeground       string `yaml:"header_foreground"`
	RowForeground          string `yaml:"row_foreground"`
	OKTextSuccess          string `yaml:"ok_text_success"`
	OKTextFailure          string `yaml:"ok_text_failure"`
	ModalBorderBackground  string `yaml:"modal_border_background"`
	ModalBorderForeground  string `yaml:"modal_border_foreground"`
	ButtonOKBackground     string `yaml:"button_ok_background"`
	ButtonOKForeground     string `yaml:"button_ok_foreground"`
	ButtonCancelBackground string `yaml:"button_cancel_background"`
	ButtonCancelForeground string `yaml:"button_cancel_foreground"`
}

// Settings is the concrete runtime configuration the rest of the program uses.
type Settings struct {
	Interval           time.Duration
	Timeout            time.Duration
	RefreshInterval    time.Duration
	ThemeName          string
	MaxConcurrentPings int
	MaxHosts           int
	MaxHostsTracked    int
	PingQueueCapacity  int
	Backend            string
	SystemCommand      string
	SystemArgs         []string
	Theme              ThemeConfig
	ConfigPath         string
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{
		IntervalSeconds: 10,
		TimeoutSeconds:  2,
		RefreshSeconds:  1,
		ThemeName:       "default",
		Concurrency: ConcurrencyConfig{
			MaxConcurrentPings: 64,
			MaxHosts:           0,
			PingQueueCapacity:  256,
		},
		Memory: MemoryConfig{
			MaxHostsTracked: 0,
		},
		Ping: PingConfig{
			Backend:       "system",
			SystemCommand: "ping",
		},
		Theme: ThemeConfig{
			TitleBackground:        "blue",
			TitleForeground:        "white",
			StatusBackground:       "blue",
			StatusForeground:       "white",
			HeaderBackground:       "darkblue",
			HeaderForeground:       "white",
			RowForeground:          "white",
			OKTextSuccess:          "green",
			OKTextFailure:          "red",
			ModalBorderBackground:  "blue",
			ModalBorderForeground:  "white",
			ButtonOKBackground:     "green",
			ButtonOKForeground:     "white",
			ButtonCancelBackground: "red",
			ButtonCancelForeground: "white",
		},
	}
}

// LoadConfigFile parses a YAML configuration file from the provided path.
func LoadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// FindConfigPath resolves the configuration file path using the search order.
func FindConfigPath(cliPath string) (string, bool) {
	candidates := []string{}
	if cliPath != "" {
		candidates = append(candidates, cliPath)
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "mping", "config.yaml"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "mping", "config.yaml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".mping", "config.yaml"))
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// MergeSettings builds the final runtime settings given defaults, config, and CLI overrides.
func MergeSettings(defaults Config, fileCfg Config, cli CLIOverrides, configPath string) (Settings, error) {
	merged := defaults

	applyConfig := func(dst *Config, src Config) {
		if src.IntervalSeconds > 0 {
			dst.IntervalSeconds = src.IntervalSeconds
		}
		if src.TimeoutSeconds > 0 {
			dst.TimeoutSeconds = src.TimeoutSeconds
		}
		if src.RefreshSeconds > 0 {
			dst.RefreshSeconds = src.RefreshSeconds
		}
		if src.Concurrency.MaxConcurrentPings > 0 {
			dst.Concurrency.MaxConcurrentPings = src.Concurrency.MaxConcurrentPings
		}
		if src.Concurrency.MaxHosts >= 0 {
			dst.Concurrency.MaxHosts = src.Concurrency.MaxHosts
		}
		if src.Concurrency.PingQueueCapacity > 0 {
			dst.Concurrency.PingQueueCapacity = src.Concurrency.PingQueueCapacity
		}
		if src.Memory.MaxHostsTracked >= 0 {
			dst.Memory.MaxHostsTracked = src.Memory.MaxHostsTracked
		}
		if src.Ping.Backend != "" {
			dst.Ping.Backend = src.Ping.Backend
		}
		if src.Ping.SystemCommand != "" {
			dst.Ping.SystemCommand = src.Ping.SystemCommand
		}
		if len(src.Ping.SystemArgs) > 0 {
			dst.Ping.SystemArgs = src.Ping.SystemArgs
		}
		if src.Theme != (ThemeConfig{}) {
			dst.Theme = src.Theme
		}
	}

	applyConfig(&merged, fileCfg)

	if cli.IntervalSeconds > 0 {
		merged.IntervalSeconds = cli.IntervalSeconds
	}
	if cli.TimeoutSeconds > 0 {
		merged.TimeoutSeconds = cli.TimeoutSeconds
	}
	if cli.RefreshSeconds > 0 {
		merged.RefreshSeconds = cli.RefreshSeconds
	}
	if cli.MaxConcurrentPings > 0 {
		merged.Concurrency.MaxConcurrentPings = cli.MaxConcurrentPings
	}
	if cli.MaxHosts >= 0 {
		merged.Concurrency.MaxHosts = cli.MaxHosts
	}
	if cli.PingQueueCapacity > 0 {
		merged.Concurrency.PingQueueCapacity = cli.PingQueueCapacity
	}
	if cli.Backend != "" {
		merged.Ping.Backend = cli.Backend
	}
	if cli.ThemeName != "" {
		merged.ThemeName = cli.ThemeName
	}

	settings := Settings{
		Interval:           time.Duration(merged.IntervalSeconds) * time.Second,
		Timeout:            time.Duration(merged.TimeoutSeconds) * time.Second,
		RefreshInterval:    time.Duration(merged.RefreshSeconds) * time.Second,
		ThemeName:          merged.ThemeName,
		MaxConcurrentPings: merged.Concurrency.MaxConcurrentPings,
		MaxHosts:           merged.Concurrency.MaxHosts,
		MaxHostsTracked:    merged.Memory.MaxHostsTracked,
		PingQueueCapacity:  merged.Concurrency.PingQueueCapacity,
		Backend:            merged.Ping.Backend,
		SystemCommand:      merged.Ping.SystemCommand,
		SystemArgs:         defaultArgs(merged.Ping.SystemArgs),
		Theme:              merged.Theme,
		ConfigPath:         configPath,
	}

	if settings.Interval <= 0 {
		return Settings{}, errors.New("interval must be positive")
	}
	if settings.Timeout <= 0 {
		return Settings{}, errors.New("timeout must be positive")
	}
	if settings.RefreshInterval <= 0 {
		return Settings{}, errors.New("refresh interval must be positive")
	}
	if settings.MaxConcurrentPings <= 0 {
		return Settings{}, errors.New("max_concurrent_pings must be positive")
	}
	if settings.PingQueueCapacity <= 0 {
		return Settings{}, errors.New("ping_queue_capacity must be positive")
	}

	return settings, nil
}

// CLIOverrides capture the command-line flags that trump config values.
type CLIOverrides struct {
	IntervalSeconds    int
	TimeoutSeconds     int
	RefreshSeconds     int
	MaxConcurrentPings int
	PingQueueCapacity  int
	MaxHosts           int
	Backend            string
	ThemeName          string
}

func defaultArgs(cfgArgs []string) []string {
	if len(cfgArgs) > 0 {
		return cfgArgs
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{"-c", "1"}
	default:
		return []string{"-c", "1"}
	}
}
