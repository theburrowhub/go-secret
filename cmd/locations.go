package cmd

import (
	"github.com/spf13/cobra"
)

var locationsCmd = &cobra.Command{
	Use:   "locations",
	Short: "Manage saved GCP locations",
	Long: `Commands to manage saved GCP locations for secret replication.

Locations can be used when creating secrets to specify where
they should be replicated.

Available commands:
  list   - List saved locations
  add    - Add a new location
  remove - Remove a saved location`,
}

func init() {
	rootCmd.AddCommand(locationsCmd)
}
