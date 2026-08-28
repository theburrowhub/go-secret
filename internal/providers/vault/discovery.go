package vault

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
)

// DiscoveredMount describes a KV mount found via /sys/mounts.
type DiscoveredMount struct {
	Path    string // e.g. "secret"
	Version int    // 1 or 2
	Type    string // raw type from Vault, e.g. "kv"
}

// DiscoverMounts queries /sys/mounts on the given address using the given
// token and returns all KV mounts found, with version auto-detected from
// the options.version field (defaulting to 1 when absent).
func DiscoverMounts(ctx context.Context, address, token string) ([]DiscoveredMount, error) {
	cfg := vaultapi.DefaultConfig()
	cfg.Address = address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	api.SetToken(token)

	mounts, err := api.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list mounts: %w", err)
	}

	out := []DiscoveredMount{}
	for path, m := range mounts {
		if !strings.HasPrefix(m.Type, "kv") && m.Type != "generic" {
			continue
		}
		path = strings.TrimSuffix(path, "/")
		version := 1
		if m.Options != nil {
			if v, ok := m.Options["version"]; ok {
				if v == "2" {
					version = 2
				}
			}
		}
		// Vault sometimes reports version="2" via the type itself.
		if m.Type == "kv-v2" {
			version = 2
		}
		out = append(out, DiscoveredMount{Path: path, Version: version, Type: m.Type})
	}
	return out, nil
}

// BuildSourceConfigFromDiscovery composes a SourceConfig pre-populated with
// the discovered mounts. The caller decides id/displayName.
func BuildSourceConfigFromDiscovery(id, address string, mounts []DiscoveredMount) config.SourceConfig {
	sc := config.SourceConfig{
		ID:              id,
		Provider:        "vault",
		Enabled:         true,
		Address:         address,
		FolderSeparator: "/",
		Auth:            config.VaultAuthConfig{Method: "token"},
	}
	for _, m := range mounts {
		sc.Mounts = append(sc.Mounts, config.VaultMount{Path: m.Path, Version: m.Version})
	}
	return sc
}

// SuggestSourceID derives a default source ID from a Vault address.
// E.g., "http://localhost:8200" → "vault-localhost".
func SuggestSourceID(address string) string {
	u, err := url.Parse(address)
	host := address
	if err == nil && u.Host != "" {
		host = u.Host
	}
	// Strip port
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	host = strings.ToLower(host)
	host = strings.ReplaceAll(host, ".", "-")
	host = strings.ReplaceAll(host, "_", "-")
	return "vault-" + host
}

// SaveToken caches a Vault token for sourceID in the OS keyring (with
// in-memory fallback) so subsequent operations don't require VAULT_TOKEN
// in the environment.
func SaveToken(sourceID, token string) error {
	return keyringSet(sourceID, token)
}
