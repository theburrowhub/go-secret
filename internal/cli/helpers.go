package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/gcp"
	"golang.org/x/term"
)

// InitGCPClient initializes configuration and GCP client with the provided or configured project ID.
// It returns the config, client, and project ID being used.
func InitGCPClient(ctx context.Context, projectID string) (*config.Config, *gcp.Client, string, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error loading configuration: %w", err)
	}

	// Determine which project to use
	proj := projectID
	if proj == "" {
		proj = cfg.ProjectID
	}
	if proj == "" {
		return nil, nil, "", fmt.Errorf("no project ID specified. Use --project or configure a default project")
	}

	// Create GCP client
	client, err := gcp.NewClient(ctx, proj)
	if err != nil {
		return nil, nil, "", fmt.Errorf("error creating GCP client: %w", err)
	}

	return cfg, client, proj, nil
}

// AuditLogger wraps audit logger initialization and provides helper methods.
type AuditLogger struct {
	logger *audit.Logger
}

// NewAuditLogger creates a new audit logger if audit is enabled in config.
// Returns nil if audit is disabled or if there's an error (errors are silently ignored).
func NewAuditLogger(cfg *config.Config, client *gcp.Client) *AuditLogger {
	if !cfg.Audit.Enabled {
		return nil
	}

	auditCfg := audit.Config{
		Enabled:    cfg.Audit.Enabled,
		FilePath:   cfg.Audit.FilePath,
		MaxSizeMB:  cfg.Audit.MaxSizeMB,
		MaxAgeDays: cfg.Audit.MaxAgeDays,
	}

	logger, err := audit.NewLogger(auditCfg)
	if err != nil {
		return nil
	}

	if client != nil {
		logger.SetUser(client.UserEmail())
	}

	return &AuditLogger{logger: logger}
}

// Close closes the audit logger.
func (a *AuditLogger) Close() {
	if a != nil && a.logger != nil {
		a.logger.Close()
	}
}

// LogSecretCreate logs a secret creation event.
func (a *AuditLogger) LogSecretCreate(projectID, secretName string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretCreate(projectID, secretName, result, details)
	}
}

// LogSecretDelete logs a secret deletion event.
func (a *AuditLogger) LogSecretDelete(projectID, secretName string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretDelete(projectID, secretName, result, details)
	}
}

// LogSecretRead logs a secret read event (used for generic read access).
func (a *AuditLogger) LogSecretRead(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretAccess(projectID, secretName, version, result, details)
	}
}

// LogVersionAdd logs a version addition event.
func (a *AuditLogger) LogVersionAdd(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogVersionAdd(projectID, secretName, version, result, details)
	}
}

// LogVersionEnable logs a version enable event (using generic event logging).
func (a *AuditLogger) LogVersionEnable(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		// Use generic Log since there's no specific method for enable
		_ = a.logger.Log(audit.Event{
			EventType:  "VERSION_ENABLE",
			Result:     result,
			ProjectID:  projectID,
			SecretName: secretName,
			Version:    version,
			Error:      details,
		})
	}
}

// LogVersionDisable logs a version disable event (using generic event logging).
func (a *AuditLogger) LogVersionDisable(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		_ = a.logger.Log(audit.Event{
			EventType:  "VERSION_DISABLE",
			Result:     result,
			ProjectID:  projectID,
			SecretName: secretName,
			Version:    version,
			Error:      details,
		})
	}
}

// LogVersionDestroy logs a version destroy event (using generic event logging).
func (a *AuditLogger) LogVersionDestroy(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		_ = a.logger.Log(audit.Event{
			EventType:  "VERSION_DESTROY",
			Result:     result,
			ProjectID:  projectID,
			SecretName: secretName,
			Version:    version,
			Error:      details,
		})
	}
}

// LogTemplateCreate logs a template creation event (using generic event logging).
func (a *AuditLogger) LogTemplateCreate(templateTitle string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		_ = a.logger.Log(audit.Event{
			EventType: "TEMPLATE_CREATE",
			Result:    result,
			Details:   map[string]string{"title": templateTitle},
			Error:     details,
		})
	}
}

// LogTemplateDelete logs a template deletion event (using generic event logging).
func (a *AuditLogger) LogTemplateDelete(templateTitle string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		_ = a.logger.Log(audit.Event{
			EventType: "TEMPLATE_DELETE",
			Result:    result,
			Details:   map[string]string{"title": templateTitle},
			Error:     details,
		})
	}
}

// LogConfigChange logs a configuration change event.
func (a *AuditLogger) LogConfigChange(field, oldValue, newValue string) {
	if a != nil && a.logger != nil {
		a.logger.LogConfigChange(field, oldValue, newValue)
	}
}

// LogSecretCopy logs a secret copy to clipboard event.
func (a *AuditLogger) LogSecretCopy(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretCopy(projectID, secretName, version, result, details)
	}
}

// LogSecretReveal logs a secret reveal event.
func (a *AuditLogger) LogSecretReveal(projectID, secretName, version string, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretReveal(projectID, secretName, version, result, details)
	}
}

// LogSecretList logs a secret listing event.
func (a *AuditLogger) LogSecretList(projectID string, count int, result audit.EventResult, details string) {
	if a != nil && a.logger != nil {
		a.logger.LogSecretList(projectID, count, result, details)
	}
}

// ReadSecretValue reads a secret value from various sources: stdin, file, direct value, or interactive prompt.
func ReadSecretValue(fromStdin bool, fromFile, value, promptMessage string) ([]byte, error) {
	switch {
	case fromStdin:
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading from stdin: %w", err)
		}
		return []byte(strings.Join(lines, "\n")), nil

	case fromFile != "":
		fileData, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}
		return fileData, nil

	case value != "":
		return []byte(value), nil

	default:
		// Interactive mode with hidden input
		if promptMessage != "" {
			fmt.Print(promptMessage)
		} else {
			fmt.Print("Enter secret value (hidden input): ")
		}
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return nil, fmt.Errorf("error reading value: %w", err)
		}
		return password, nil
	}
}

// ConfirmAction prompts the user for confirmation unless force is true.
// Returns true if the action should proceed, false otherwise.
func ConfirmAction(message string, force bool) (bool, error) {
	if force {
		return true, nil
	}

	fmt.Println(message)
	fmt.Print("\nAre you sure? (type 'yes' to confirm): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("error reading confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes", nil
}

// ParseLabels converts a slice of "key=value" strings into a map.
func ParseLabels(labelStrings []string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, label := range labelStrings {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s (use key=value)", label)
		}
		labels[parts[0]] = parts[1]
	}
	return labels, nil
}

// ReadInput reads a single line of input from stdin with a prompt.
func ReadInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}
	return strings.TrimSpace(input), nil
}
