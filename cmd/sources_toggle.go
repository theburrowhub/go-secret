package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Toggle a source's enabled flag and persist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == args[0] {
				cfg.Sources[i].Enabled = !cfg.Sources[i].Enabled
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Printf("✓ Source %q is now %s\n", args[0], onOff(cfg.Sources[i].Enabled))
				return nil
			}
		}
		return fmt.Errorf("source %q not found", args[0])
	},
}

func onOff(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func init() { sourcesCmd.AddCommand(sourcesToggleCmd) }
