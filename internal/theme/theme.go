package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme defines the color scheme for the application
type Theme struct {
	Name string

	// Text colors
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Muted     color.Color
	Error     color.Color
	Success   color.Color
	Warning   color.Color

	// UI element colors
	Border       color.Color
	BorderActive color.Color
	Background   color.Color
	Highlight    color.Color
	Shadow       color.Color
}

// keep lipgloss imported for the Color constructor used below
var _ = lipgloss.Color

// Available themes
var (
	CatppuccinMocha = Theme{
		Name:         "Catppuccin Mocha",
		Primary:      lipgloss.Color("#cdd6f4"),
		Secondary:    lipgloss.Color("#a6adc8"),
		Accent:       lipgloss.Color("#f5c2e7"),
		Muted:        lipgloss.Color("#6c7086"),
		Error:        lipgloss.Color("#f38ba8"),
		Success:      lipgloss.Color("#a6e3a1"),
		Warning:      lipgloss.Color("#f9e2af"),
		Border:       lipgloss.Color("#45475a"),
		BorderActive: lipgloss.Color("#89b4fa"),
		Background:   lipgloss.Color("#313244"),
		Highlight:    lipgloss.Color("#45475a"),
		Shadow:       lipgloss.Color("#11111b"),
	}

	CatppuccinLatte = Theme{
		Name:         "Catppuccin Latte",
		Primary:      lipgloss.Color("#4c4f69"),
		Secondary:    lipgloss.Color("#5c5f77"),
		Accent:       lipgloss.Color("#ea76cb"),
		Muted:        lipgloss.Color("#9ca0b0"),
		Error:        lipgloss.Color("#d20f39"),
		Success:      lipgloss.Color("#40a02b"),
		Warning:      lipgloss.Color("#df8e1d"),
		Border:       lipgloss.Color("#dce0e8"),
		BorderActive: lipgloss.Color("#1e66f5"),
		Background:   lipgloss.Color("#e6e9ef"),
		Highlight:    lipgloss.Color("#ccd0da"),
		Shadow:       lipgloss.Color("#bcc0cc"),
	}

	Dracula = Theme{
		Name:         "Dracula",
		Primary:      lipgloss.Color("#f8f8f2"),
		Secondary:    lipgloss.Color("#6272a4"),
		Accent:       lipgloss.Color("#ff79c6"),
		Muted:        lipgloss.Color("#6272a4"),
		Error:        lipgloss.Color("#ff5555"),
		Success:      lipgloss.Color("#50fa7b"),
		Warning:      lipgloss.Color("#f1fa8c"),
		Border:       lipgloss.Color("#44475a"),
		BorderActive: lipgloss.Color("#bd93f9"),
		Background:   lipgloss.Color("#282a36"),
		Highlight:    lipgloss.Color("#44475a"),
		Shadow:       lipgloss.Color("#11121a"),
	}

	RosePineMoon = Theme{
		Name:         "Rosé Pine Moon",
		Primary:      lipgloss.Color("#e0def4"),
		Secondary:    lipgloss.Color("#908caa"),
		Accent:       lipgloss.Color("#ebbcba"),
		Muted:        lipgloss.Color("#6e6a86"),
		Error:        lipgloss.Color("#eb6f92"),
		Success:      lipgloss.Color("#9ccfd8"),
		Warning:      lipgloss.Color("#f6c177"),
		Border:       lipgloss.Color("#403d52"),
		BorderActive: lipgloss.Color("#c4a7e7"),
		Background:   lipgloss.Color("#2a273f"),
		Highlight:    lipgloss.Color("#393552"),
		Shadow:       lipgloss.Color("#16121f"),
	}

	RosePineDawn = Theme{
		Name:         "Rosé Pine Dawn",
		Primary:      lipgloss.Color("#575279"),
		Secondary:    lipgloss.Color("#797593"),
		Accent:       lipgloss.Color("#d7827e"),
		Muted:        lipgloss.Color("#9893a5"),
		Error:        lipgloss.Color("#b4637a"),
		Success:      lipgloss.Color("#56949f"),
		Warning:      lipgloss.Color("#ea9d34"),
		Border:       lipgloss.Color("#f2e9e1"),
		BorderActive: lipgloss.Color("#907aa9"),
		Background:   lipgloss.Color("#faf4ed"),
		Highlight:    lipgloss.Color("#f2e9e1"),
		Shadow:       lipgloss.Color("#dfdad9"),
	}

	SolarizedDark = Theme{
		Name:         "Solarized Dark",
		Primary:      lipgloss.Color("#839496"),
		Secondary:    lipgloss.Color("#586e75"),
		Accent:       lipgloss.Color("#d33682"),
		Muted:        lipgloss.Color("#586e75"),
		Error:        lipgloss.Color("#dc322f"),
		Success:      lipgloss.Color("#859900"),
		Warning:      lipgloss.Color("#b58900"),
		Border:       lipgloss.Color("#073642"),
		BorderActive: lipgloss.Color("#268bd2"),
		Background:   lipgloss.Color("#002b36"),
		Highlight:    lipgloss.Color("#073642"),
		Shadow:       lipgloss.Color("#00161c"),
	}

	SolarizedLight = Theme{
		Name:         "Solarized Light",
		Primary:      lipgloss.Color("#657b83"),
		Secondary:    lipgloss.Color("#93a1a1"),
		Accent:       lipgloss.Color("#d33682"),
		Muted:        lipgloss.Color("#93a1a1"),
		Error:        lipgloss.Color("#dc322f"),
		Success:      lipgloss.Color("#859900"),
		Warning:      lipgloss.Color("#b58900"),
		Border:       lipgloss.Color("#eee8d5"),
		BorderActive: lipgloss.Color("#268bd2"),
		Background:   lipgloss.Color("#fdf6e3"),
		Highlight:    lipgloss.Color("#eee8d5"),
		Shadow:       lipgloss.Color("#ddd6c1"),
	}

	BruEspresso = Theme{
		Name:         "Bru Espresso",
		Primary:      lipgloss.Color("#f5e8c7"),
		Secondary:    lipgloss.Color("#dbcbb1"),
		Accent:       lipgloss.Color("#ed8649"),
		Muted:        lipgloss.Color("#8a7f6f"),
		Error:        lipgloss.Color("#fa5750"),
		Success:      lipgloss.Color("#75b938"),
		Warning:      lipgloss.Color("#dbb32d"),
		Border:       lipgloss.Color("#3a332b"),
		BorderActive: lipgloss.Color("#4695f7"),
		Background:   lipgloss.Color("#1c1814"),
		Highlight:    lipgloss.Color("#322b23"),
		Shadow:       lipgloss.Color("#0d0a07"),
	}

	BruLatte = Theme{
		Name:         "Bru Latte",
		Primary:      lipgloss.Color("#3a2f22"),
		Secondary:    lipgloss.Color("#61574b"),
		Accent:       lipgloss.Color("#c25d1e"),
		Muted:        lipgloss.Color("#8a7f6f"),
		Error:        lipgloss.Color("#d2212d"),
		Success:      lipgloss.Color("#489100"),
		Warning:      lipgloss.Color("#ad8900"),
		Border:       lipgloss.Color("#e0d4b0"),
		BorderActive: lipgloss.Color("#0072d4"),
		Background:   lipgloss.Color("#faf3e0"),
		Highlight:    lipgloss.Color("#ece3cc"),
		Shadow:       lipgloss.Color("#d1c39a"),
	}

	JoziNights = Theme{
		Name:         "Jozi Nights",
		Primary:      lipgloss.Color("#a9b1d6"),
		Secondary:    lipgloss.Color("#c0caf5"),
		Accent:       lipgloss.Color("#f92aad"),
		Muted:        lipgloss.Color("#8089b3"),
		Error:        lipgloss.Color("#f92aad"),
		Success:      lipgloss.Color("#54e484"),
		Warning:      lipgloss.Color("#e0b401"),
		Border:       lipgloss.Color("#3b4261"),
		BorderActive: lipgloss.Color("#b141f1"),
		Background:   lipgloss.Color("#1b1e2e"),
		Highlight:    lipgloss.Color("#24283b"),
		Shadow:       lipgloss.Color("#0b0d18"),
	}

	JoziMorning = Theme{
		Name:         "Jozi Morning",
		Primary:      lipgloss.Color("#343b58"),
		Secondary:    lipgloss.Color("#1f2335"),
		Accent:       lipgloss.Color("#d6197a"),
		Muted:        lipgloss.Color("#6172af"),
		Error:        lipgloss.Color("#d6197a"),
		Success:      lipgloss.Color("#16864a"),
		Warning:      lipgloss.Color("#c49000"),
		Border:       lipgloss.Color("#b0b3be"),
		BorderActive: lipgloss.Color("#4f46e5"),
		Background:   lipgloss.Color("#d5d6db"),
		Highlight:    lipgloss.Color("#bfc1cc"),
		Shadow:       lipgloss.Color("#9fa1ad"),
	}

	JoziMidnight = Theme{
		Name:         "Jozi Midnight",
		Primary:      lipgloss.Color("#a9b1d6"),
		Secondary:    lipgloss.Color("#c0caf5"),
		Accent:       lipgloss.Color("#f92aad"),
		Muted:        lipgloss.Color("#8089b3"),
		Error:        lipgloss.Color("#f92aad"),
		Success:      lipgloss.Color("#54e484"),
		Warning:      lipgloss.Color("#e0b401"),
		Border:       lipgloss.Color("#232433"),
		BorderActive: lipgloss.Color("#b141f1"),
		Background:   lipgloss.Color("#101014"),
		Highlight:    lipgloss.Color("#1a1b26"),
		Shadow:       lipgloss.Color("#040406"),
	}
)

// AllThemes returns a list of all available themes
func AllThemes() []Theme {
	return []Theme{
		CatppuccinMocha,
		CatppuccinLatte,
		Dracula,
		RosePineMoon,
		RosePineDawn,
		SolarizedDark,
		SolarizedLight,
		BruEspresso,
		BruLatte,
		JoziNights,
		JoziMorning,
		JoziMidnight,
	}
}

// GetTheme returns a theme by name, defaulting to Catppuccin Mocha if not found
func GetTheme(name string) Theme {
	themes := map[string]Theme{
		"catppuccin-mocha": CatppuccinMocha,
		"catppuccin-latte": CatppuccinLatte,
		"dracula":          Dracula,
		"rosepine-moon":    RosePineMoon,
		"rosepine-dawn":    RosePineDawn,
		"solarized-dark":   SolarizedDark,
		"solarized-light":  SolarizedLight,
		"bru-espresso":     BruEspresso,
		"bru-latte":        BruLatte,
		"jozi-nights":      JoziNights,
		"jozi-morning":     JoziMorning,
		"jozi-midnight":    JoziMidnight,
	}

	if theme, ok := themes[name]; ok {
		return theme
	}
	return CatppuccinMocha
}
