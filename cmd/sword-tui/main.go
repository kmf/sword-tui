package main

import (
	"fmt"
	"os"
	"sword-tui/internal/cache"
	"sword-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
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
