package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestMigrate_LegacyToSources(t *testing.T) {
	cfg := &Config{
		ProjectID:       "active-proj",
		RecentProjects:  []string{"active-proj", "other-proj"},
		FolderSeparator: "/",
		SecretLocations: []string{"global", "europe-west1"},
	}
	migrated := MigrateLegacy(cfg)

	if migrated.DefaultSource != "gsm-active-proj" {
		t.Fatalf("default_source: got %q, want gsm-active-proj", migrated.DefaultSource)
	}
	if len(migrated.Sources) != 2 {
		t.Fatalf("sources count: got %d, want 2", len(migrated.Sources))
	}
	var active, other *SourceConfig
	for i := range migrated.Sources {
		s := &migrated.Sources[i]
		if s.ID == "gsm-active-proj" {
			active = s
		}
		if s.ID == "gsm-other-proj" {
			other = s
		}
	}
	if active == nil || other == nil {
		t.Fatalf("missing migrated sources: %+v", migrated.Sources)
	}
	if !active.Enabled {
		t.Fatal("active project source should be Enabled")
	}
	if other.Enabled {
		t.Fatal("inactive project source should be Disabled by default")
	}
	if active.ProjectID != "active-proj" {
		t.Fatalf("project_id: got %q", active.ProjectID)
	}
	if active.FolderSeparator != "/" {
		t.Fatalf("folder_separator: got %q", active.FolderSeparator)
	}
	if len(active.SecretLocations) != 2 {
		t.Fatalf("secret_locations: got %v", active.SecretLocations)
	}

	// Legacy fields should be cleared after migration.
	if migrated.ProjectID != "" || len(migrated.RecentProjects) != 0 ||
		migrated.FolderSeparator != "" || len(migrated.SecretLocations) != 0 {
		t.Fatal("legacy fields not cleared after migration")
	}
}

func TestMigrate_NoOpWhenSourcesAlreadyPresent(t *testing.T) {
	cfg := &Config{
		Sources: []SourceConfig{{ID: "x", Provider: "gsm", Enabled: true}},
	}
	migrated := MigrateLegacy(cfg)
	if len(migrated.Sources) != 1 || migrated.Sources[0].ID != "x" {
		t.Fatalf("sources mutated: %+v", migrated.Sources)
	}
}

func TestMigrate_SanitizeProjectIDInSourceID(t *testing.T) {
	cfg := &Config{
		ProjectID:      "My Project With Spaces",
		RecentProjects: []string{"My Project With Spaces"},
	}
	migrated := MigrateLegacy(cfg)
	if migrated.Sources[0].ID != "gsm-my-project-with-spaces" {
		t.Fatalf("sanitized id: got %q", migrated.Sources[0].ID)
	}
}

func TestLoad_PerformsMigrationAndPersists(t *testing.T) {
	// On macOS GetConfigPath uses ~/Library/Application Support and ignores
	// XDG_CONFIG_HOME, so the test cannot control the config path. CI runs on
	// Linux where XDG_CONFIG_HOME is respected.
	if runtime.GOOS == "darwin" {
		t.Skip("TestLoad_PerformsMigrationAndPersists requires Linux config path (XDG_CONFIG_HOME); skipping on darwin")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	legacy := `
project_id: legacy-proj
recent_projects: ["legacy-proj"]
folder_separator: "/"
clipboard:
  auto_clear: true
  timeout_seconds: 30
audit:
  enabled: true
  max_size_mb: 10
  max_age_days: 90
session:
  inactivity_timeout: 15
  lock_on_timeout: true
`
	path := filepath.Join(dir, "go-secrets", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(cfg.Sources))
	}
	if cfg.DefaultSource != "gsm-legacy-proj" {
		t.Fatalf("default: got %q", cfg.DefaultSource)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "default_source: gsm-legacy-proj") {
		t.Fatalf("config not persisted with migrated shape:\n%s", saved)
	}
	if strings.Contains(string(saved), "project_id: legacy-proj") {
		t.Fatal("legacy project_id should be removed after migration")
	}
}
