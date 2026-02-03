package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/ui"
)

var (
	// Version information (set by ldflags)
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"

	// Flags
	projectID string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-secret",
	Short: "A security-first CLI/TUI for GCP Secret Manager",
	Long: `go-secret is a command-line and terminal user interface for managing
Google Cloud Platform Secret Manager secrets.

Features:
  • Full CLI commands for automation and scripting
  • Interactive TUI mode for visual navigation
  • Folder-like organization with configurable separators
  • Real-time filtering and search
  • Version management (view, reveal, add)
  • Code generation from customizable templates
  • Secure memory handling and clipboard auto-clear
  • Comprehensive audit logging for compliance

Run without arguments to launch the TUI, or use any of the available commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&projectID, "project", "p", "", "GCP Project ID")
	
	// Add version flag
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

func runTUI() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create the model
	model := ui.NewModel(cfg, projectID)

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
