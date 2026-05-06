package cmd

import "github.com/spf13/cobra"

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Manage secret sources (GSM projects and Vault mounts)",
}

func init() { rootCmd.AddCommand(sourcesCmd) }
