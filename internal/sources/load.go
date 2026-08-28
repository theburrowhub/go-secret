// internal/sources/load.go
package sources

import (
	"context"
	"fmt"

	"github.com/theburrowhub/go-secret/internal/config"
)

// Constructors injected at runtime to avoid an import cycle (config + sources
// shouldn't depend on the concrete provider packages directly).
var (
	newGSMProvider   func(ctx context.Context, sc config.SourceConfig) (Provider, error)
	newVaultProvider func(ctx context.Context, sc config.SourceConfig) (Provider, error)
)

// RegisterProviderConstructors is called once from main (or cmd/root.go) to
// wire the real GSM and Vault factories.
func RegisterProviderConstructors(
	gsm func(ctx context.Context, sc config.SourceConfig) (Provider, error),
	vault func(ctx context.Context, sc config.SourceConfig) (Provider, error),
) {
	newGSMProvider = gsm
	newVaultProvider = vault
}

// LoadFromConfig instantiates providers per SourceConfig and registers them
// with their initial enabled flag.
func LoadFromConfig(ctx context.Context, cfg *config.Config) (*Registry, error) {
	r := NewRegistry()
	for _, sc := range cfg.Sources {
		var (
			p   Provider
			err error
		)
		switch sc.Provider {
		case "gsm":
			if newGSMProvider == nil {
				return nil, fmt.Errorf("gsm provider constructor not registered")
			}
			p, err = newGSMProvider(ctx, sc)
		case "vault":
			if newVaultProvider == nil {
				return nil, fmt.Errorf("vault provider constructor not registered")
			}
			p, err = newVaultProvider(ctx, sc)
		default:
			return nil, fmt.Errorf("unknown provider %q for source %q", sc.Provider, sc.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", sc.ID, err)
		}
		r.Register(p, sc.Enabled)
	}
	return r, nil
}
