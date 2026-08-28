package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tPROVIDER\tENABLED\tDETAIL\tDEFAULT")
		_, _ = fmt.Fprintln(w, "--\t--------\t-------\t------\t-------")
		for _, s := range cfg.Sources {
			detail := s.ProjectID
			if s.Provider == "vault" {
				detail = s.Address
			}
			def := ""
			if s.ID == cfg.DefaultSource {
				def = "*"
			}
			enabled := "no"
			if s.Enabled {
				enabled = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Provider, enabled, detail, def)
		}
		_ = w.Flush()
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesListCmd) }
