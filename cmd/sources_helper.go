// cmd/sources_helper.go
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
)

// resolveActiveSource returns the source id to use for a write operation,
// applying:
//  1. --source flag (preferred)
//  2. --project flag (deprecated, only matches gsm sources by ProjectID)
//  3. cfg.DefaultSource
//  4. "" (caller may run an interactive prompt)
func resolveActiveSource(cfg *config.Config) string {
	if sourceID != "" {
		return sourceID
	}
	if projectID != "" {
		fmt.Fprintln(os.Stderr, "warning: --project is deprecated; prefer --source <id>")
		for _, s := range cfg.Sources {
			if s.Provider == "gsm" && s.ProjectID == projectID {
				return s.ID
			}
		}
		// Fall through: not found.
	}
	return cfg.DefaultSource
}

// loadRegistry centralizes the boilerplate every cmd needs to talk to backends.
func loadRegistry(ctx context.Context) (*config.Config, *sources.Registry, *sources.UnifiedClient, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load config: %w", err)
	}
	reg, err := sources.LoadFromConfig(ctx, cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load sources: %w", err)
	}
	return cfg, reg, sources.NewUnifiedClient(reg), nil
}
