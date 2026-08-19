package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigFile(t *testing.T) {
	path := writeConfig(t, `
interval_seconds: 5
timeout_seconds: 3
refresh_seconds: 2
theme: solarized
concurrency:
  max_concurrent_pings: 16
  max_hosts: 50
ping:
  protocol: tcp
  tcp_port: 8443
themes:
  solarized:
    title_background: "#002b36"
    title_foreground: "#93a1a1"
`)
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	if cfg.IntervalSeconds != 5 {
		t.Errorf("IntervalSeconds = %d, want 5", cfg.IntervalSeconds)
	}
	if cfg.ThemeName != "solarized" {
		t.Errorf("ThemeName = %q, want solarized", cfg.ThemeName)
	}
	if cfg.Concurrency.MaxHosts != 50 {
		t.Errorf("MaxHosts = %d, want 50", cfg.Concurrency.MaxHosts)
	}
	if cfg.Ping.Protocol != "tcp" || cfg.Ping.TCPPort != 8443 {
		t.Errorf("Ping = %+v, want protocol tcp port 8443", cfg.Ping)
	}
	inline, ok := cfg.Themes["solarized"]
	if !ok {
		t.Fatalf("Themes missing solarized: %+v", cfg.Themes)
	}
	if inline.TitleBackground != "#002b36" {
		t.Errorf("TitleBackground = %q, want #002b36", inline.TitleBackground)
	}
}

func TestLoadConfigFileEmptyAndMinimal(t *testing.T) {
	for name, content := range map[string]string{
		"empty":   "",
		"minimal": "interval_seconds: 5\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfigFile(writeConfig(t, content)); err != nil {
				t.Fatalf("LoadConfigFile: %v", err)
			}
		})
	}
}

func TestMergeSettingsMaxHosts(t *testing.T) {
	fileCfg := Config{Concurrency: ConcurrencyConfig{MaxHosts: 50}}

	settings, err := MergeSettings(Defaults(), fileCfg, CLIOverrides{MaxHosts: -1}, "")
	if err != nil {
		t.Fatalf("MergeSettings: %v", err)
	}
	if settings.MaxHosts != 50 {
		t.Errorf("MaxHosts = %d, want config value 50 when flag unset", settings.MaxHosts)
	}

	settings, err = MergeSettings(Defaults(), fileCfg, CLIOverrides{MaxHosts: 0}, "")
	if err != nil {
		t.Fatalf("MergeSettings: %v", err)
	}
	if settings.MaxHosts != 0 {
		t.Errorf("MaxHosts = %d, want explicit CLI 0 (unlimited) to win", settings.MaxHosts)
	}
}

func TestFindConfigPathExplicitPathIsAuthoritative(t *testing.T) {
	// Missing explicit path must fail rather than fall back to defaults.
	if path, ok := FindConfigPath(filepath.Join(t.TempDir(), "nope.yaml")); ok {
		t.Errorf("FindConfigPath returned %q for a missing explicit path", path)
	}
	existing := writeConfig(t, "interval_seconds: 5\n")
	path, ok := FindConfigPath(existing)
	if !ok || path != existing {
		t.Errorf("FindConfigPath = %q ok=%v, want %q", path, ok, existing)
	}
}

func TestMergeSettingsPrecedence(t *testing.T) {
	fileCfg := Config{IntervalSeconds: 5, ThemeName: "file-theme"}
	cli := CLIOverrides{IntervalSeconds: 7}
	settings, err := MergeSettings(Defaults(), fileCfg, cli, "")
	if err != nil {
		t.Fatalf("MergeSettings: %v", err)
	}
	if settings.Interval != 7*time.Second {
		t.Errorf("Interval = %v, want 7s (CLI wins)", settings.Interval)
	}
	if settings.ThemeName != "file-theme" {
		t.Errorf("ThemeName = %q, want file-theme", settings.ThemeName)
	}
	if settings.Timeout != 2*time.Second {
		t.Errorf("Timeout = %v, want default 2s", settings.Timeout)
	}
}
