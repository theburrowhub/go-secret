package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/vault"
)

var (
	vaultDetectAddress string
	vaultDetectToken   string
	vaultDetectID      string
	vaultDetectName    string
)

var sourcesVaultDetectCmd = &cobra.Command{
	Use:   "vault-detect",
	Short: "Auto-discover a Vault source from VAULT_ADDR + ~/.vault-token",
	Long: `Detects an existing Vault setup using the same env/files the
'vault' CLI uses, lists all KV mounts, and adds a ready-to-use source.

Resolution order for address:
  1. --address flag
  2. VAULT_ADDR env
Resolution order for token:
  1. --token flag
  2. VAULT_TOKEN env
  3. ~/.vault-token file

Examples:
  go-secret sources vault-detect
  go-secret sources vault-detect --address https://vault.corp.io
  go-secret sources vault-detect --id vault-prod --display-name "Vault prod"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Resolve address
		address := vaultDetectAddress
		if address == "" {
			address = os.Getenv("VAULT_ADDR")
		}
		if address == "" {
			return fmt.Errorf("no Vault address: set --address or VAULT_ADDR")
		}

		// Resolve token
		token := vaultDetectToken
		if token == "" {
			token = os.Getenv("VAULT_TOKEN")
		}
		if token == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				data, err := os.ReadFile(filepath.Join(home, ".vault-token"))
				if err == nil {
					token = strings.TrimSpace(string(data))
				}
			}
		}
		if token == "" {
			return fmt.Errorf("no Vault token: set --token, VAULT_TOKEN, or write ~/.vault-token")
		}

		fmt.Printf("Detected VAULT_ADDR=%s\n", address)
		fmt.Println("Reading token from environment/file...")

		// Discover mounts
		mounts, err := vault.DiscoverMounts(ctx, address, token)
		if err != nil {
			return fmt.Errorf("discover mounts: %w", err)
		}
		if len(mounts) == 0 {
			return fmt.Errorf("no KV mounts found at %s", address)
		}

		fmt.Printf("Found %d KV mounts:\n", len(mounts))
		for _, m := range mounts {
			fmt.Printf("  - %s/ (KV v%d)\n", m.Path, m.Version)
		}

		// Generate ID
		id := vaultDetectID
		if id == "" {
			id = vault.SuggestSourceID(address)
		}

		// Load config and check for duplicates
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for _, s := range cfg.Sources {
			if s.ID == id {
				return fmt.Errorf("source %q already exists; use --id to specify a different one", id)
			}
		}

		sc := vault.BuildSourceConfigFromDiscovery(id, address, mounts)
		if vaultDetectName != "" {
			sc.DisplayName = vaultDetectName
		}

		// Cache the discovered token in keyring so subsequent commands work
		// without VAULT_TOKEN env. (Best-effort — keyring may fall back to
		// in-process memory on Linux without secret-service.)
		_ = vault.SaveToken(id, token)

		cfg.Sources = append(cfg.Sources, sc)
		if cfg.DefaultSource == "" {
			cfg.DefaultSource = sc.ID
		}
		if err := cfg.Save(); err != nil {
			return err
		}

		fmt.Printf("✓ Source %q added and enabled\n", sc.ID)
		fmt.Printf("  Run `go-secret list` to see secrets,\n")
		fmt.Printf("  or `go-secret sources login %s` to switch to OIDC/AppRole later.\n", sc.ID)
		return nil
	},
}

func init() {
	sourcesCmd.AddCommand(sourcesVaultDetectCmd)
	sourcesVaultDetectCmd.Flags().StringVar(&vaultDetectAddress, "address", "", "Vault address (default: $VAULT_ADDR)")
	sourcesVaultDetectCmd.Flags().StringVar(&vaultDetectToken, "token", "", "Vault token (default: $VAULT_TOKEN or ~/.vault-token)")
	sourcesVaultDetectCmd.Flags().StringVar(&vaultDetectID, "id", "", "Source ID (default: derived from address hostname)")
	sourcesVaultDetectCmd.Flags().StringVar(&vaultDetectName, "display-name", "", "Display name for the source (default: ID)")
}

