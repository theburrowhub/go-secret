package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var locationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved locations",
	Long: `Shows all saved GCP locations.

These locations can be used when creating secrets to specify
replication regions.

Example:
  go-secret locations list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLocationsList()
	},
}

func init() {
	locationsCmd.AddCommand(locationsListCmd)
}

func runLocationsList() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	if len(cfg.SecretLocations) == 0 {
		fmt.Println("No saved locations")
		fmt.Println("\nCommon GCP locations:")
		fmt.Println("  us-central1, us-east1, us-west1")
		fmt.Println("  europe-west1, europe-west2")
		fmt.Println("  asia-east1, asia-southeast1")
		return nil
	}

	fmt.Println("Saved Locations:")
	for i, loc := range cfg.SecretLocations {
		fmt.Printf("  %d. %s\n", i+1, loc)
	}

	fmt.Printf("\nTotal: %d locations\n", len(cfg.SecretLocations))

	return nil
}
