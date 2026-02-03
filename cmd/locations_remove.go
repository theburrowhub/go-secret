package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var locationsRemoveForce bool

var locationsRemoveCmd = &cobra.Command{
	Use:   "remove <index>",
	Short: "Remove a saved location",
	Long: `Removes a location from the saved list by its index.

Use 'go-secret locations list' to see the indices.

Examples:
  go-secret locations remove 2
  go-secret locations remove 1 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return runLocationsRemove(index)
	},
}

func init() {
	locationsCmd.AddCommand(locationsRemoveCmd)
	locationsRemoveCmd.Flags().BoolVarP(&locationsRemoveForce, "force", "f", false, "Remove without confirmation")
}

func runLocationsRemove(index int) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Validate index
	if index < 1 || index > len(cfg.SecretLocations) {
		return fmt.Errorf("index out of range: %d (must be between 1 and %d)", index, len(cfg.SecretLocations))
	}

	// Adjust index (1-based to 0-based)
	idx := index - 1
	location := cfg.SecretLocations[idx]

	// Confirm removal if not using --force
	if !locationsRemoveForce {
		fmt.Printf("Remove location '%s'? (type 'yes' to confirm): ", location)

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Removal cancelled.")
			return nil
		}
	}

	// Remove location
	cfg.RemoveSecretLocation(location)

	// Save configuration
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	fmt.Printf("✓ Location '%s' removed successfully\n", location)
	fmt.Printf("  Remaining locations: %d\n", len(cfg.SecretLocations))

	return nil
}
