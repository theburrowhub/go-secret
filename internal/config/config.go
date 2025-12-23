package config

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Template represents a code template for secret usage
type Template struct {
	Title string `yaml:"title"`
	Code  string `yaml:"code"`
}

// ClipboardConfig holds clipboard security settings
type ClipboardConfig struct {
	AutoClear      bool `yaml:"auto_clear"`
	TimeoutSeconds int  `yaml:"timeout_seconds"`
}

// AuditConfig holds audit logging settings
type AuditConfig struct {
	Enabled    bool   `yaml:"enabled"`
	FilePath   string `yaml:"file_path,omitempty"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

// SessionConfig holds session security settings
type SessionConfig struct {
	InactivityTimeout int  `yaml:"inactivity_timeout"` // Minutes, 0 = disabled
	LockOnTimeout     bool `yaml:"lock_on_timeout"`
}

// Config holds the application configuration
type Config struct {
	ProjectID        string          `yaml:"project_id"`
	FolderSeparator  string          `yaml:"folder_separator"`
	Templates        []Template      `yaml:"templates"`
	RecentProjects   []string        `yaml:"recent_projects"`
	Clipboard        ClipboardConfig `yaml:"clipboard"`
	Audit            AuditConfig     `yaml:"audit"`
	Session          SessionConfig   `yaml:"session"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ProjectID:       "",
		FolderSeparator: "/",
		Clipboard: ClipboardConfig{
			AutoClear:      true,
			TimeoutSeconds: 30,
		},
		Audit: AuditConfig{
			Enabled:    true,
			FilePath:   "", // Uses default path
			MaxSizeMB:  10,
			MaxAgeDays: 90,
		},
		Session: SessionConfig{
			InactivityTimeout: 15, // 15 minutes default
			LockOnTimeout:     true,
		},
		Templates: []Template{
			{
				Title: "Bash Export",
				Code:  `export {{.SecretName}}=$(gcloud secrets versions access latest --secret="{{.FullSecretName}}" --project="{{.ProjectID}}")`,
			},
			{
				Title: "Helmfile secretRef",
				Code: `- secretRef:
    name: {{.SecretName}}
    key: {{.FullSecretName}}`,
			},
			{
				Title: "Kyverno Policy",
				Code: `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-secret-{{.SecretName}}
spec:
  validationFailureAction: enforce
  rules:
  - name: check-secret
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Secret {{.FullSecretName}} must be referenced"
      pattern:
        spec:
          containers:
          - env:
            - valueFrom:
                secretKeyRef:
                  name: "{{.SecretName}}"`,
			},
			{
				Title: "Go Client",
				Code: `import secretmanager "cloud.google.com/go/secretmanager/apiv1"

// Access the secret
result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/{{.ProjectID}}/secrets/{{.FullSecretName}}/versions/latest",
})`,
			},
		},
		RecentProjects: []string{},
	}
}

// GetConfigPath returns the path to the config file based on OS
func GetConfigPath() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // linux and others
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	appDir := filepath.Join(configDir, "go-secrets")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "config.yaml"), nil
}

// Load reads the config from disk or returns defaults
func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the config to disk
func (c *Config) Save() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// AddRecentProject adds a project to the saved projects list
// Projects are stored in order of most recent use (most recent first)
func (c *Config) AddRecentProject(projectID string) {
	if projectID == "" {
		return
	}
	
	// Remove if already exists (to move it to front)
	filtered := make([]string, 0, len(c.RecentProjects))
	for _, p := range c.RecentProjects {
		if p != projectID {
			filtered = append(filtered, p)
		}
	}

	// Add to front (most recently used)
	c.RecentProjects = append([]string{projectID}, filtered...)
}

// RemoveProject removes a project from the saved list
func (c *Config) RemoveProject(projectID string) {
	filtered := make([]string, 0, len(c.RecentProjects))
	for _, p := range c.RecentProjects {
		if p != projectID {
			filtered = append(filtered, p)
		}
	}
	c.RecentProjects = filtered
}

