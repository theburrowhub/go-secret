package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
)

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set configuration values",
	Long: `Establece valores de configuración.

Claves disponibles:
  project             - Proyecto GCP por defecto
  separator           - Separador de carpetas (default: /)
  clipboard.auto      - Auto-clear del clipboard (true/false)
  clipboard.timeout   - Timeout en segundos para limpiar clipboard
  audit.enabled       - Habilitar auditoría (true/false)
  audit.max-size      - Tamaño máximo del archivo de log en MB
  audit.max-age       - Retención de logs en días
  session.lock        - Lock automático en timeout (true/false)
  session.timeout     - Timeout de inactividad en minutos (0 = deshabilitado)

Ejemplos:
  go-secret config set project my-gcp-project
  go-secret config set separator /
  go-secret config set clipboard.auto true
  go-secret config set clipboard.timeout 60
  go-secret config set audit.enabled true
  go-secret config set session.timeout 15`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigSet(args[0], args[1])
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
}

func runConfigSet(key, value string) error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	oldValue := ""
	setting := key

	// Procesar según la clave
	switch strings.ToLower(key) {
	case "project", "project-id", "projectid":
		oldValue = cfg.ProjectID
		cfg.ProjectID = value
		// Añadir a proyectos recientes
		cfg.AddRecentProject(value)

	case "separator", "folder-separator":
		oldValue = cfg.FolderSeparator
		cfg.FolderSeparator = value

	case "clipboard.auto", "clipboard.autoclear":
		oldValue = strconv.FormatBool(cfg.Clipboard.AutoClear)
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("valor inválido para clipboard.auto: debe ser true o false")
		}
		cfg.Clipboard.AutoClear = boolVal

	case "clipboard.timeout", "clipboard.timeout-seconds":
		oldValue = strconv.Itoa(cfg.Clipboard.TimeoutSeconds)
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("valor inválido para clipboard.timeout: debe ser un número")
		}
		if intVal < 0 {
			return fmt.Errorf("clipboard.timeout debe ser mayor o igual a 0")
		}
		cfg.Clipboard.TimeoutSeconds = intVal

	case "audit.enabled", "audit.enable":
		oldValue = strconv.FormatBool(cfg.Audit.Enabled)
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("valor inválido para audit.enabled: debe ser true o false")
		}
		cfg.Audit.Enabled = boolVal

	case "audit.max-size", "audit.maxsize":
		oldValue = strconv.Itoa(cfg.Audit.MaxSizeMB)
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("valor inválido para audit.max-size: debe ser un número")
		}
		if intVal <= 0 {
			return fmt.Errorf("audit.max-size debe ser mayor que 0")
		}
		cfg.Audit.MaxSizeMB = intVal

	case "audit.max-age", "audit.maxage":
		oldValue = strconv.Itoa(cfg.Audit.MaxAgeDays)
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("valor inválido para audit.max-age: debe ser un número")
		}
		if intVal <= 0 {
			return fmt.Errorf("audit.max-age debe ser mayor que 0")
		}
		cfg.Audit.MaxAgeDays = intVal

	case "session.lock", "session.lock-on-timeout":
		oldValue = strconv.FormatBool(cfg.Session.LockOnTimeout)
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("valor inválido para session.lock: debe ser true o false")
		}
		cfg.Session.LockOnTimeout = boolVal

	case "session.timeout", "session.inactivity-timeout":
		oldValue = strconv.Itoa(cfg.Session.InactivityTimeout)
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("valor inválido para session.timeout: debe ser un número")
		}
		if intVal < 0 {
			return fmt.Errorf("session.timeout debe ser mayor o igual a 0 (0 = deshabilitado)")
		}
		cfg.Session.InactivityTimeout = intVal

	default:
		return fmt.Errorf("clave desconocida: %s", key)
	}

	// Guardar configuración
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
	}

	// Registrar en audit log si está habilitado
	if cfg.Audit.Enabled {
		auditCfg := audit.Config{
			Enabled:    cfg.Audit.Enabled,
			FilePath:   cfg.Audit.FilePath,
			MaxSizeMB:  cfg.Audit.MaxSizeMB,
			MaxAgeDays: cfg.Audit.MaxAgeDays,
		}
		auditLogger, err := audit.NewLogger(auditCfg)
		if err == nil {
			defer func() { _ = auditLogger.Close() }()
			auditLogger.LogConfigChange(setting, oldValue, value)
		}
	}

	fmt.Printf("✓ Configuración actualizada\n")
	fmt.Printf("  %s: %s → %s\n", key, oldValue, value)

	return nil
}
