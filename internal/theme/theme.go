package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color scheme for the application
type Theme struct {
	Name string

	// Text colors
	Primary       lipgloss.Color
	Secondary     lipgloss.Color
	Accent        lipgloss.Color
	Muted         lipgloss.Color
	Error         lipgloss.Color
	Success       lipgloss.Color
	Warning       lipgloss.Color

	// UI element colors
	Border        lipgloss.Color
	BorderActive  lipgloss.Color
	Background    lipgloss.Color
	Highlight     lipgloss.Color
}

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
	}

	if theme, ok := themes[name]; ok {
		return theme
	}
	return CatppuccinMocha
}
