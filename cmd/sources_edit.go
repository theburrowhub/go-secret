package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an existing source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		idx := -1
		for i, s := range cfg.Sources {
			if s.ID == args[0] {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("source %q not found", args[0])
		}
		sc := &cfg.Sources[idx]

		r := bufio.NewReader(os.Stdin)
		ask := func(q, def string) string {
			fmt.Printf("%s [%s]: ", q, def)
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return def
			}
			return line
		}
		sc.DisplayName = ask("Display name", sc.DisplayName)
		sc.FolderSeparator = ask("Folder separator", sc.FolderSeparator)
		switch sc.Provider {
		case "gsm":
			sc.ProjectID = ask("GCP Project ID", sc.ProjectID)
		case "vault":
			sc.Address = ask("Vault address", sc.Address)
			sc.Auth.Method = ask("Auth method", sc.Auth.Method)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q updated\n", sc.ID)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesEditCmd) }
