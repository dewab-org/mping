package theme

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"mping/internal/config"
)

// LoadThemeFiles reads .theme files from the provided directories.
func LoadThemeFiles(dirs []string) map[string]config.ThemeConfig {
	out := map[string]config.ThemeConfig{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".theme" {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".theme")
			if _, exists := out[name]; exists {
				continue
			}
			cfg, err := parseThemeFile(filepath.Join(dir, e.Name()))
			if err == nil {
				out[name] = cfg
			}
		}
	}
	return out
}

// ResolveTheme picks a Theme based on name, custom list, or fallback to config theme.
func ResolveTheme(name string, cfgTheme config.ThemeConfig, custom map[string]config.ThemeConfig) Theme {
	if custom != nil {
		if t, ok := custom[name]; ok {
			return FromConfig(t)
		}
	}
	return FromConfig(cfgTheme)
}

func parseThemeFile(path string) (config.ThemeConfig, error) {
	cfg := config.ThemeConfig{}
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.Trim(line, "\"")
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := normalizeThemeKey(strings.TrimSpace(parts[0]))
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			switch key {
			case "title_background":
				cfg.TitleBackground = val
			case "title_foreground":
				cfg.TitleForeground = val
			case "status_background":
				cfg.StatusBackground = val
			case "status_foreground":
				cfg.StatusForeground = val
			case "header_background":
				cfg.HeaderBackground = val
			case "header_foreground":
				cfg.HeaderForeground = val
			case "row_foreground":
				cfg.RowForeground = val
			case "ok_text_success":
				cfg.OKTextSuccess = val
			case "ok_text_failure":
				cfg.OKTextFailure = val
			case "modal_border_background":
				cfg.ModalBorderBackground = val
			case "modal_border_foreground":
				cfg.ModalBorderForeground = val
			case "button_ok_background":
				cfg.ButtonOKBackground = val
			case "button_ok_foreground":
				cfg.ButtonOKForeground = val
			case "button_cancel_background":
				cfg.ButtonCancelBackground = val
			case "button_cancel_foreground":
				cfg.ButtonCancelForeground = val
			case "main_bg":
				cfg.RowForeground = val
			case "main_fg":
				cfg.RowForeground = val
			}
		}
	}
	return cfg, nil
}

func normalizeThemeKey(k string) string {
	k = strings.TrimSpace(strings.ToLower(k))
	k = strings.TrimPrefix(k, "theme[")
	k = strings.TrimSuffix(k, "]")
	k = strings.Trim(k, "\"")
	return k
}
