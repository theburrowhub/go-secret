package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		id := args[0]
		filtered := cfg.Sources[:0]
		removed := false
		for _, s := range cfg.Sources {
			if s.ID == id {
				removed = true
				continue
			}
			filtered = append(filtered, s)
		}
		if !removed {
			return fmt.Errorf("source %q not found", id)
		}
		cfg.Sources = filtered
		if cfg.DefaultSource == id {
			cfg.DefaultSource = ""
			if len(cfg.Sources) > 0 {
				cfg.DefaultSource = cfg.Sources[0].ID
			}
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q removed\n", id)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesRemoveCmd) }
