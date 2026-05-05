package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSourceConfigYAMLRoundTrip(t *testing.T) {
	cfg := &Config{
		DefaultSource: "gsm-prod",
		Sources: []SourceConfig{
			{
				ID:              "gsm-prod",
				Provider:        "gsm",
				Enabled:         true,
				ProjectID:       "my-project",
				FolderSeparator: "/",
				SecretLocations: []string{"global"},
			},
			{
				ID:       "vault-corp",
				Provider: "vault",
				Enabled:  true,
				Address:  "https://vault.corp.io",
				Auth:     VaultAuthConfig{Method: "oidc", Role: "developer", OIDCPort: 8250},
				Mounts:   []VaultMount{{Path: "secret", Version: 2}},
			},
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.DefaultSource != "gsm-prod" {
		t.Fatalf("default_source: got %q, want gsm-prod", got.DefaultSource)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("sources: got %d, want 2", len(got.Sources))
	}
	if got.Sources[1].Auth.Method != "oidc" {
		t.Fatalf("vault auth method: got %q, want oidc", got.Sources[1].Auth.Method)
	}
}
