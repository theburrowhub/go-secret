// internal/providers/vault/auth_approle.go
package vault

import (
	"context"
	"errors"

	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveAppRoleAuth authenticates against Vault using the AppRole auth method.
// Stub implementation — filled in Task 14.
func resolveAppRoleAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	return "", errors.New("approle auth not implemented yet")
}
