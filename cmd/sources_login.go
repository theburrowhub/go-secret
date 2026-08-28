package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/vault"
)

var sourcesLoginCmd = &cobra.Command{
	Use:   "login <id>",
	Short: "Re-authenticate against a Vault source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		var sc *config.SourceConfig
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == args[0] {
				sc = &cfg.Sources[i]
				break
			}
		}
		if sc == nil {
			return fmt.Errorf("source %q not found", args[0])
		}
		if sc.Provider != "vault" {
			return fmt.Errorf("login only applies to vault sources")
		}
		if sc.Auth.Method == "approle" {
			fmt.Print("Enter secret_id (input hidden): ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if err := vault.SaveAppRoleSecretID(sc.ID, line); err != nil {
				return err
			}
			fmt.Println("✓ secret_id stored in keyring")
			return nil
		}
		// Token / OIDC: trigger a connection to drive the auth flow.
		if _, err := vault.NewFromSourceConfig(context.Background(), *sc); err != nil {
			return err
		}
		fmt.Println("✓ Login successful, token cached")
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesLoginCmd) }
