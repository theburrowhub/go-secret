package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var locationsAddCmd = &cobra.Command{
	Use:   "add <location>",
	Short: "Add a new location",
	Long: `Adds a new GCP location to the saved list.

Locations are GCP regions where secrets can be replicated.

Common locations:
  us-central1, us-east1, us-west1
  europe-west1, europe-west2
  asia-east1, asia-southeast1

Examples:
  go-secret locations add us-central1
  go-secret locations add europe-west1`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLocationsAdd(args[0])
	},
}

func init() {
	locationsCmd.AddCommand(locationsAddCmd)
}

func runLocationsAdd(location string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Check if already exists
	for _, loc := range cfg.SecretLocations {
		if loc == location {
			fmt.Printf("Location '%s' is already in the list\n", location)
			return nil
		}
	}

	// Add location
	cfg.AddSecretLocation(location)

	// Save configuration
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	fmt.Printf("✓ Location '%s' added successfully\n", location)
	fmt.Printf("  Total locations: %d\n", len(cfg.SecretLocations))

	return nil
}
