// internal/providers/vault/auth_token.go
package vault

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveTokenAuth returns a Vault token using the following resolution order:
//  1. VAULT_TOKEN env var
//  2. OS keyring entry "go-secret:vault:<source-id>"
//  3. ~/.vault-token file
func resolveTokenAuth(sc config.SourceConfig) (string, error) {
	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		return v, nil
	}
	if v, ok := keyringGet(sc.ID); ok {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		data, err := os.ReadFile(filepath.Join(home, ".vault-token"))
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("no vault token available (set VAULT_TOKEN, run `go-secret sources login %s`, or write ~/.vault-token)", sc.ID)
}
