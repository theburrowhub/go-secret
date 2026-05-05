// internal/providers/vault/client.go
package vault

import (
	"context"
	"errors"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
)

// Client is the Vault sources.Provider implementation. Supports KV v1 and v2,
// possibly multiple mounts within the same source.
type Client struct {
	id              string
	displayName     string
	folderSeparator string
	api             *vaultapi.Client
	mounts          []mountInfo
}

type mountInfo struct {
	Path    string
	Version int // 1 or 2
}

// NewFromSourceConfig instantiates a Vault Client from a SourceConfig.
// Authentication happens here; the returned Client carries the Vault token
// in the api client.
func NewFromSourceConfig(ctx context.Context, sc config.SourceConfig) (*Client, error) {
	if sc.Address == "" {
		return nil, fmt.Errorf("vault source %q missing address", sc.ID)
	}
	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	tok, err := resolveAuth(ctx, sc)
	if err != nil {
		return nil, err
	}
	api.SetToken(tok)

	c := &Client{
		id:              sc.ID,
		displayName:     sc.DisplayName,
		folderSeparator: sc.FolderSeparator,
		api:             api,
	}
	if c.folderSeparator == "" {
		c.folderSeparator = "/"
	}
	for _, m := range sc.Mounts {
		c.mounts = append(c.mounts, mountInfo{Path: m.Path, Version: m.Version})
	}
	if len(c.mounts) == 0 {
		return nil, fmt.Errorf("vault source %q has no mounts configured", sc.ID)
	}
	return c, nil
}

// ID implements sources.Provider.
func (c *Client) ID() string { return c.id }

// Kind implements sources.Provider.
func (c *Client) Kind() string { return "vault" }

// DisplayName implements sources.Provider.
func (c *Client) DisplayName() string {
	if c.displayName != "" {
		return c.displayName
	}
	return c.id
}

// FolderSeparator implements sources.Provider.
func (c *Client) FolderSeparator() string { return c.folderSeparator }

// UserEmail implements sources.Provider. Populated in Task 15 for OIDC.
func (c *Client) UserEmail() string { return "" }

// Close implements sources.Provider.
func (c *Client) Close() error { return nil }

// Capabilities implements sources.Provider.
func (c *Client) Capabilities() sources.Capabilities {
	supportsVersions := false
	for _, m := range c.mounts {
		if m.Version == 2 {
			supportsVersions = true
			break
		}
	}
	return sources.Capabilities{
		SupportsVersions: supportsVersions,
		SupportsLabels:   true,
	}
}

// List implements sources.Provider. Full implementation in Task 12.
func (c *Client) List(ctx context.Context) ([]sources.Secret, error) {
	return nil, errors.New("vault.Client.List not implemented")
}

// Get implements sources.Provider. Full implementation in Task 12.
func (c *Client) Get(ctx context.Context, name string) (*sources.Secret, error) {
	return nil, errors.New("vault.Client.Get not implemented")
}

// Reveal implements sources.Provider. Full implementation in Task 12.
func (c *Client) Reveal(ctx context.Context, name, version string) ([]byte, error) {
	return nil, errors.New("vault.Client.Reveal not implemented")
}

// ListVersions implements sources.Provider. Full implementation in Task 13.
func (c *Client) ListVersions(ctx context.Context, name string) ([]sources.Version, error) {
	return nil, errors.New("vault.Client.ListVersions not implemented")
}

// Create implements sources.Provider. Full implementation in Task 12.
func (c *Client) Create(ctx context.Context, name string, value []byte, opts sources.CreateOpts) error {
	return errors.New("vault.Client.Create not implemented")
}

// Delete implements sources.Provider. Full implementation in Task 12.
func (c *Client) Delete(ctx context.Context, name string) error {
	return errors.New("vault.Client.Delete not implemented")
}

// AddVersion implements sources.Provider. Full implementation in Task 13.
func (c *Client) AddVersion(ctx context.Context, name string, value []byte) (*sources.Version, error) {
	return nil, errors.New("vault.Client.AddVersion not implemented")
}

// EnableVersion implements sources.Provider. Requires KV v2.
func (c *Client) EnableVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("EnableVersion not supported until KV v2 implemented")
}

// DisableVersion implements sources.Provider. Requires KV v2.
func (c *Client) DisableVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("DisableVersion not supported until KV v2 implemented")
}

// DestroyVersion implements sources.Provider. Requires KV v2.
func (c *Client) DestroyVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("DestroyVersion not supported until KV v2 implemented")
}

// Compile-time assertion that *Client satisfies sources.Provider.
var _ sources.Provider = (*Client)(nil)
