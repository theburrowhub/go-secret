package sources

import (
	"context"
	"testing"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestLoadFromConfig_GSMOnly_InstantiatesEnabledOnly(t *testing.T) {
	cfg := &config.Config{
		DefaultSource: "a",
		Sources: []config.SourceConfig{
			{ID: "a", Provider: "gsm", Enabled: true, ProjectID: "p1"},
			{ID: "b", Provider: "gsm", Enabled: false, ProjectID: "p2"},
		},
	}
	prevGSM := newGSMProvider
	defer func() { newGSMProvider = prevGSM }()
	newGSMProvider = func(ctx context.Context, sc config.SourceConfig) (Provider, error) {
		return newFakeProvider(sc.ID, "gsm"), nil
	}
	prevVault := newVaultProvider
	defer func() { newVaultProvider = prevVault }()
	newVaultProvider = func(ctx context.Context, sc config.SourceConfig) (Provider, error) {
		t.Fatal("vault should not be instantiated")
		return nil, nil
	}

	reg, err := LoadFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}
	if got := len(reg.All()); got != 2 {
		t.Fatalf("got %d registered, want 2", got)
	}
	if got := len(reg.Active()); got != 1 {
		t.Fatalf("got %d active, want 1", got)
	}
	if reg.Active()[0].ID() != "a" {
		t.Fatalf("active[0]: got %q", reg.Active()[0].ID())
	}
}

func TestLoadFromConfig_UnknownProviderReturnsError(t *testing.T) {
	cfg := &config.Config{
		Sources: []config.SourceConfig{{ID: "x", Provider: "azure", Enabled: true}},
	}
	if _, err := LoadFromConfig(context.Background(), cfg); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
