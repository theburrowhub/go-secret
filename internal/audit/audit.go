package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of audit event
type EventType string

const (
	// Secret operations
	EventSecretList    EventType = "SECRET_LIST"
	EventSecretAccess  EventType = "SECRET_ACCESS"
	EventSecretReveal  EventType = "SECRET_REVEAL"
	EventSecretCopy    EventType = "SECRET_COPY"
	EventSecretCreate  EventType = "SECRET_CREATE"
	EventSecretDelete  EventType = "SECRET_DELETE"
	EventVersionAdd    EventType = "VERSION_ADD"
	EventVersionList   EventType = "VERSION_LIST"

	// Configuration operations
	EventConfigChange  EventType = "CONFIG_CHANGE"
	EventProjectSwitch EventType = "PROJECT_SWITCH"

	// Session events
	EventSessionStart EventType = "SESSION_START"
	EventSessionEnd   EventType = "SESSION_END"

	// Security events
	EventClipboardClear EventType = "CLIPBOARD_CLEAR"
)

// EventResult represents the result of an operation
type EventResult string

const (
	ResultSuccess EventResult = "SUCCESS"
	ResultFailure EventResult = "FAILURE"
)

// Event represents an audit log entry
type Event struct {
	Timestamp  string            `json:"timestamp"`
	EventType  EventType         `json:"event_type"`
	Result     EventResult       `json:"result"`
	User       string            `json:"user,omitempty"`
	ProjectID  string            `json:"project_id,omitempty"`
	SecretName string            `json:"secret_name,omitempty"`
	Version    string            `json:"version,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// Logger handles audit logging operations
type Logger struct {
	mu         sync.Mutex
	file       *os.File
	enabled    bool
	filePath   string
	maxSizeMB  int
	maxAgeDays int
	userEmail  string
}

// Config holds audit logger configuration
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	FilePath   string `yaml:"file_path,omitempty"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

// DefaultConfig returns default audit configuration
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		FilePath:   "", // Will be set to default path
		MaxSizeMB:  10,
		MaxAgeDays: 90,
	}
}

// GetDefaultLogPath returns the default audit log path based on OS
func GetDefaultLogPath() (string, error) {
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
	default:
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	logDir := filepath.Join(configDir, "go-secrets", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", err
	}

	return filepath.Join(logDir, "audit.log"), nil
}

// NewLogger creates a new audit logger
func NewLogger(cfg Config) (*Logger, error) {
	logger := &Logger{
		enabled:    cfg.Enabled,
		maxSizeMB:  cfg.MaxSizeMB,
		maxAgeDays: cfg.MaxAgeDays,
	}

	if !cfg.Enabled {
		return logger, nil
	}

	// Determine log file path
	logPath := cfg.FilePath
	if logPath == "" {
		var err error
		logPath, err = GetDefaultLogPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get default log path: %w", err)
		}
	}
	logger.filePath = logPath

	// Create log directory if needed
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file with secure permissions
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	logger.file = file

	// Perform rotation check on startup
	if err := logger.rotateIfNeeded(); err != nil {
		// Log rotation error but don't fail
		fmt.Fprintf(os.Stderr, "audit: rotation check failed: %v\n", err)
	}

	return logger, nil
}

// Log writes an audit event
func (l *Logger) Log(event Event) error {
	if !l.enabled || l.file == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not provided
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Set user if not provided and we have one stored
	if event.User == "" && l.userEmail != "" {
		event.User = l.userEmail
	}

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Write with newline
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// Sync to ensure durability
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	// Check if rotation is needed
	if err := l.rotateIfNeeded(); err != nil {
		// Log rotation error but don't fail the audit write
		fmt.Fprintf(os.Stderr, "audit: rotation failed: %v\n", err)
	}

	return nil
}

// LogSecretAccess logs a secret access event
func (l *Logger) LogSecretAccess(projectID, secretName, version string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventSecretAccess,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Version:    version,
		Error:      errMsg,
	})
}

// LogSecretReveal logs a secret reveal event
func (l *Logger) LogSecretReveal(projectID, secretName, version string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventSecretReveal,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Version:    version,
		Error:      errMsg,
	})
}

// LogSecretCopy logs a secret copy to clipboard event
func (l *Logger) LogSecretCopy(projectID, secretName, version string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventSecretCopy,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Version:    version,
		Error:      errMsg,
	})
}

// LogSecretCreate logs a secret creation event
func (l *Logger) LogSecretCreate(projectID, secretName string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventSecretCreate,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Error:      errMsg,
	})
}

// LogSecretDelete logs a secret deletion event
func (l *Logger) LogSecretDelete(projectID, secretName string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventSecretDelete,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Error:      errMsg,
	})
}

// LogVersionAdd logs a version addition event
func (l *Logger) LogVersionAdd(projectID, secretName, version string, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType:  EventVersionAdd,
		Result:     result,
		ProjectID:  projectID,
		SecretName: secretName,
		Version:    version,
		Error:      errMsg,
	})
}

// LogSecretList logs a secret listing event
func (l *Logger) LogSecretList(projectID string, count int, result EventResult, errMsg string) {
	_ = l.Log(Event{
		EventType: EventSecretList,
		Result:    result,
		ProjectID: projectID,
		Details:   map[string]string{"count": fmt.Sprintf("%d", count)},
		Error:     errMsg,
	})
}

// LogConfigChange logs a configuration change event
func (l *Logger) LogConfigChange(setting, oldValue, newValue string) {
	_ = l.Log(Event{
		EventType: EventConfigChange,
		Result:    ResultSuccess,
		Details: map[string]string{
			"setting":   setting,
			"old_value": oldValue,
			"new_value": newValue,
		},
	})
}

// LogProjectSwitch logs a project switch event
func (l *Logger) LogProjectSwitch(oldProject, newProject string) {
	_ = l.Log(Event{
		EventType: EventProjectSwitch,
		Result:    ResultSuccess,
		ProjectID: newProject,
		Details: map[string]string{
			"previous_project": oldProject,
		},
	})
}

// LogSessionStart logs session start
func (l *Logger) LogSessionStart(projectID string) {
	_ = l.Log(Event{
		EventType: EventSessionStart,
		Result:    ResultSuccess,
		ProjectID: projectID,
	})
}

// LogSessionEnd logs session end
func (l *Logger) LogSessionEnd(projectID string) {
	_ = l.Log(Event{
		EventType: EventSessionEnd,
		Result:    ResultSuccess,
		ProjectID: projectID,
	})
}

// LogClipboardClear logs clipboard clear event
func (l *Logger) LogClipboardClear() {
	_ = l.Log(Event{
		EventType: EventClipboardClear,
		Result:    ResultSuccess,
	})
}

// rotateIfNeeded checks if log rotation is needed and performs it
func (l *Logger) rotateIfNeeded() error {
	if l.file == nil {
		return nil
	}

	info, err := l.file.Stat()
	if err != nil {
		return err
	}

	// Check size (convert MB to bytes)
	maxBytes := int64(l.maxSizeMB) * 1024 * 1024
	if info.Size() < maxBytes {
		return nil
	}

	// Perform rotation
	return l.rotate()
}

// rotate performs log file rotation
func (l *Logger) rotate() error {
	// Close current file
	if err := l.file.Close(); err != nil {
		return err
	}

	// Generate rotated file name with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.filePath, timestamp)

	// Rename current log to rotated name
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		return err
	}

	// Open new log file
	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	l.file = file

	// Clean up old rotated files
	go l.cleanupOldLogs()

	return nil
}

// cleanupOldLogs removes log files older than maxAgeDays
func (l *Logger) cleanupOldLogs() {
	logDir := filepath.Dir(l.filePath)
	baseName := filepath.Base(l.filePath)
	cutoff := time.Now().AddDate(0, 0, -l.maxAgeDays)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Check if it's a rotated log file
		if len(name) <= len(baseName) || name[:len(baseName)] != baseName {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logDir, name))
		}
	}
}

// Close closes the audit logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// IsEnabled returns whether audit logging is enabled
func (l *Logger) IsEnabled() bool {
	return l.enabled
}

// SetUser sets the user email to be included in all audit events
func (l *Logger) SetUser(userEmail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.userEmail = userEmail
}

// GetUser returns the current user email
func (l *Logger) GetUser() string {
	return l.userEmail
}

// GetFilePath returns the audit log file path
func (l *Logger) GetFilePath() string {
	return l.filePath
}

// ReadRecentLogs reads the most recent log entries (up to maxLines)
func (l *Logger) ReadRecentLogs(maxLines int) ([]string, error) {
	if l.filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	
	// Remove empty last line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Return last maxLines entries (most recent at top)
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	// Reverse to show most recent first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines, nil
}

// FormatLogEntry formats a JSON log entry for display
func FormatLogEntry(jsonLine string) string {
	var event Event
	if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
		return jsonLine // Return raw if can't parse
	}

	// Format: [TIME] TYPE USER SECRET RESULT
	timestamp := event.Timestamp
	if len(timestamp) > 19 {
		timestamp = timestamp[:19] // Trim to YYYY-MM-DDTHH:MM:SS
	}
	timestamp = strings.Replace(timestamp, "T", " ", 1)

	user := event.User
	if len(user) > 25 {
		user = user[:22] + "..."
	}
	if user == "" {
		user = "-"
	}

	secret := event.SecretName
	if len(secret) > 30 {
		secret = secret[:27] + "..."
	}
	if secret == "" {
		secret = "-"
	}

	result := "✓"
	if event.Result == ResultFailure {
		result = "✗"
	}

	return fmt.Sprintf("%s %s %-12s %-25s %-30s", timestamp, result, event.EventType, user, secret)
}

