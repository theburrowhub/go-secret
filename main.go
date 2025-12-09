package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamuriano/go-secrets/internal/config"
	"github.com/jamuriano/go-secrets/internal/ui"
)

func main() {
	// Parse command line flags
	projectID := flag.String("project", "", "GCP Project ID")
	flag.StringVar(projectID, "p", "", "GCP Project ID (shorthand)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create the model
	model := ui.NewModel(cfg, *projectID)

	// Create and run the program
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

