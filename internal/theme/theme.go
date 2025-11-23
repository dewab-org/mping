package theme

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	"mping/internal/config"
)

// Theme contains resolved tcell colors for all UI roles.
type Theme struct {
	TitleBackground        tcell.Color
	TitleForeground        tcell.Color
	StatusBackground       tcell.Color
	StatusForeground       tcell.Color
	HeaderBackground       tcell.Color
	HeaderForeground       tcell.Color
	RowForeground          tcell.Color
	OKTextSuccess          tcell.Color
	OKTextFailure          tcell.Color
	ModalBorderBackground  tcell.Color
	ModalBorderForeground  tcell.Color
	ButtonOKBackground     tcell.Color
	ButtonOKForeground     tcell.Color
	ButtonCancelBackground tcell.Color
	ButtonCancelForeground tcell.Color
}

var colorMap = map[string]tcell.Color{
	"black":    tcell.ColorBlack,
	"white":    tcell.ColorWhite,
	"red":      tcell.ColorRed,
	"green":    tcell.ColorGreen,
	"blue":     tcell.ColorBlue,
	"darkblue": tcell.ColorDarkBlue,
	"yellow":   tcell.ColorYellow,
	"cyan":     tcell.ColorDarkCyan,
	"magenta":  tcell.ColorPurple,
	"grey":     tcell.ColorGrey,
	"gray":     tcell.ColorGrey,
}

func lookup(name string, fallback tcell.Color) tcell.Color {
	name = strings.TrimSpace(strings.ToLower(name))
	if c, ok := colorMap[name]; ok {
		return c
	}
	if strings.HasPrefix(name, "#") {
		hex := strings.TrimPrefix(name, "#")
		if len(hex) == 6 {
			if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
				r := int32(v >> 16 & 0xff)
				g := int32(v >> 8 & 0xff)
				b := int32(v & 0xff)
				return tcell.NewRGBColor(r, g, b)
			}
		}
		if len(hex) == 2 {
			if v, err := strconv.ParseUint(hex, 16, 8); err == nil {
				gray := int32(v)
				return tcell.NewRGBColor(gray, gray, gray)
			}
		}
	}
	if strings.Contains(name, " ") {
		parts := strings.Fields(name)
		if len(parts) == 3 {
			r, _ := strconv.Atoi(parts[0])
			g, _ := strconv.Atoi(parts[1])
			b, _ := strconv.Atoi(parts[2])
			return tcell.NewRGBColor(int32(r), int32(g), int32(b))
		}
	}
	return fallback
}

// FromConfig converts the configuration theme names to tcell colors.
func FromConfig(cfg config.ThemeConfig) Theme {
	return Theme{
		TitleBackground:        lookup(cfg.TitleBackground, tcell.ColorBlue),
		TitleForeground:        lookup(cfg.TitleForeground, tcell.ColorWhite),
		StatusBackground:       lookup(cfg.StatusBackground, tcell.ColorBlue),
		StatusForeground:       lookup(cfg.StatusForeground, tcell.ColorWhite),
		HeaderBackground:       lookup(cfg.HeaderBackground, tcell.ColorDarkBlue),
		HeaderForeground:       lookup(cfg.HeaderForeground, tcell.ColorWhite),
		RowForeground:          lookup(cfg.RowForeground, tcell.ColorWhite),
		OKTextSuccess:          lookup(cfg.OKTextSuccess, tcell.ColorGreen),
		OKTextFailure:          lookup(cfg.OKTextFailure, tcell.ColorRed),
		ModalBorderBackground:  lookup(cfg.ModalBorderBackground, tcell.ColorBlue),
		ModalBorderForeground:  lookup(cfg.ModalBorderForeground, tcell.ColorWhite),
		ButtonOKBackground:     lookup(cfg.ButtonOKBackground, tcell.ColorGreen),
		ButtonOKForeground:     lookup(cfg.ButtonOKForeground, tcell.ColorWhite),
		ButtonCancelBackground: lookup(cfg.ButtonCancelBackground, tcell.ColorRed),
		ButtonCancelForeground: lookup(cfg.ButtonCancelForeground, tcell.ColorWhite),
	}
}
