package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new source interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		r := bufio.NewReader(os.Stdin)
		ask := func(q, def string) string {
			if def != "" {
				fmt.Printf("%s [%s]: ", q, def)
			} else {
				fmt.Printf("%s: ", q)
			}
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return def
			}
			return line
		}

		sc := config.SourceConfig{
			Enabled:         true,
			FolderSeparator: "/",
		}
		sc.Provider = ask("Provider (gsm|vault)", "gsm")
		sc.ID = ask("Source ID", "")
		sc.DisplayName = ask("Display name (optional)", "")
		switch sc.Provider {
		case "gsm":
			sc.ProjectID = ask("GCP Project ID", "")
		case "vault":
			sc.Address = ask("Vault address", "")
			sc.Auth.Method = ask("Auth method (token|approle|oidc)", "token")
			if sc.Auth.Method == "approle" {
				sc.Auth.AppRoleRoleID = ask("Role ID", "")
			}
			if sc.Auth.Method == "oidc" {
				sc.Auth.Role = ask("OIDC role", "")
			}
			mountPath := ask("KV mount path", "secret")
			versionStr := ask("KV version (1|2)", "2")
			version := 2
			if versionStr == "1" {
				version = 1
			}
			sc.Mounts = []config.VaultMount{{Path: mountPath, Version: version}}
		default:
			return fmt.Errorf("unknown provider %q", sc.Provider)
		}

		for _, existing := range cfg.Sources {
			if existing.ID == sc.ID {
				return fmt.Errorf("source %q already exists", sc.ID)
			}
		}
		cfg.Sources = append(cfg.Sources, sc)
		if cfg.DefaultSource == "" {
			cfg.DefaultSource = sc.ID
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q added\n", sc.ID)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesAddCmd) }
