package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesSetDefaultCmd = &cobra.Command{
	Use:   "set-default <id>",
	Short: "Set the default source for write operations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for _, s := range cfg.Sources {
			if s.ID == args[0] {
				cfg.DefaultSource = args[0]
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Printf("✓ Default source set to %q\n", args[0])
				return nil
			}
		}
		return fmt.Errorf("source %q not found", args[0])
	},
}

func init() { sourcesCmd.AddCommand(sourcesSetDefaultCmd) }
