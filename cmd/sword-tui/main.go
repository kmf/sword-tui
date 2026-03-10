package main

import (
	"flag"
	"fmt"
	"os"
	"sword-tui/internal/cache"
	"sword-tui/internal/ui"
	"sword-tui/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Parse command line flags
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("sword-tui %s (build %s)\n", version.Version, version.BuildNumber)
		os.Exit(0)
	}
	// Initialize cache
	cacheManager, err := cache.NewCache()
	if err != nil {
		fmt.Printf("Warning: Could not initialize cache: %v\n", err)
		// Continue without cache
		cacheManager = nil
	}

	model := ui.NewModel()
	model.SetCache(cacheManager)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
