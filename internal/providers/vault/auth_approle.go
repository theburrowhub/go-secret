// internal/providers/vault/auth_approle.go
package vault

import (
	"context"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveAppRoleAuth authenticates against Vault using the AppRole auth method.
// It reads the secret_id from the OS keyring (seeded by SaveAppRoleSecretID),
// POSTs to auth/approle/login, and caches the resulting client token back into
// the keyring for subsequent calls.
func resolveAppRoleAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	if sc.Auth.AppRoleRoleID == "" {
		return "", fmt.Errorf("approle: missing role_id in config for source %q", sc.ID)
	}
	secretID, ok := keyringGetSecretID(sc.ID)
	if !ok {
		return "", fmt.Errorf("approle: no secret_id in keyring for source %q (run `go-secret sources login %s`)", sc.ID, sc.ID)
	}

	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return "", err
	}
	resp, err := api.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]interface{}{
		"role_id":   sc.Auth.AppRoleRoleID,
		"secret_id": secretID,
	})
	if err != nil {
		return "", fmt.Errorf("approle login: %w", err)
	}
	if resp == nil || resp.Auth == nil || resp.Auth.ClientToken == "" {
		return "", fmt.Errorf("approle login: no token in response")
	}
	_ = keyringSet(sc.ID, resp.Auth.ClientToken)
	return resp.Auth.ClientToken, nil
}

// SaveAppRoleSecretID persists the AppRole secret_id into the OS keyring (or
// in-memory fallback).  Called by `go-secret sources login` (Task 23) after
// the user supplies the secret_id interactively.
func SaveAppRoleSecretID(sourceID, secretID string) error {
	return keyringSetSecretID(sourceID, secretID)
}
