// internal/providers/vault/auth_oidc.go
package vault

import (
	"context"
	"errors"

	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveOIDCAuth authenticates against Vault using the OIDC auth method.
// Stub implementation — filled in Task 15.
func resolveOIDCAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	return "", errors.New("oidc auth not implemented yet")
}
