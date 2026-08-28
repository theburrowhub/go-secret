// internal/providers/vault/auth.go
package vault

import (
	"context"
	"fmt"

	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveAuth returns a Vault token for the given source according to the
// configured auth method.
func resolveAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	switch sc.Auth.Method {
	case "", "token":
		return resolveTokenAuth(sc)
	case "approle":
		return resolveAppRoleAuth(ctx, sc)
	case "oidc":
		return resolveOIDCAuth(ctx, sc)
	default:
		return "", fmt.Errorf("unknown vault auth method %q", sc.Auth.Method)
	}
}
