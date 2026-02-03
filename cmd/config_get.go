package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current configuration",
	Long: `Muestra toda la configuración actual de go-secret.

Incluye:
  - Proyecto por defecto
  - Separador de carpetas
  - Configuración de clipboard
  - Configuración de auditoría
  - Configuración de sesión
  - Proyectos recientes
  - Ubicaciones guardadas

Ejemplo:
  go-secret config get`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigGet()
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
}

func runConfigGet() error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	// Mostrar configuración
	fmt.Println("Configuración de go-secret")
	fmt.Println("═══════════════════════════════════════")

	// Configuración básica
	fmt.Println("\n📋 Configuración Básica:")
	fmt.Printf("  Proyecto por defecto: %s\n", getValueOrNone(cfg.ProjectID))
	fmt.Printf("  Separador de carpetas: %s\n", getValueOrNone(cfg.FolderSeparator))

	// Configuración de clipboard
	fmt.Println("\n📋 Portapapeles:")
	fmt.Printf("  Auto-clear: %s\n", boolToYesNo(cfg.Clipboard.AutoClear))
	fmt.Printf("  Timeout: %d segundos\n", cfg.Clipboard.TimeoutSeconds)

	// Configuración de auditoría
	fmt.Println("\n📝 Auditoría:")
	fmt.Printf("  Habilitada: %s\n", boolToYesNo(cfg.Audit.Enabled))
	if cfg.Audit.FilePath != "" {
		fmt.Printf("  Archivo: %s\n", cfg.Audit.FilePath)
	}
	fmt.Printf("  Tamaño máximo: %d MB\n", cfg.Audit.MaxSizeMB)
	fmt.Printf("  Retención: %d días\n", cfg.Audit.MaxAgeDays)

	// Configuración de sesión
	fmt.Println("\n⏰ Sesión:")
	fmt.Printf("  Lock on timeout: %s\n", boolToYesNo(cfg.Session.LockOnTimeout))
	if cfg.Session.InactivityTimeout == 0 {
		fmt.Printf("  Inactivity timeout: deshabilitado\n")
	} else {
		fmt.Printf("  Inactivity timeout: %d minutos\n", cfg.Session.InactivityTimeout)
	}

	// Proyectos recientes
	fmt.Println("\n🕐 Proyectos Recientes:")
	if len(cfg.RecentProjects) == 0 {
		fmt.Println("  (ninguno)")
	} else {
		for i, p := range cfg.RecentProjects {
			if i >= 5 {
				fmt.Printf("  ... y %d más\n", len(cfg.RecentProjects)-5)
				break
			}
			fmt.Printf("  %d. %s\n", i+1, p)
		}
	}

	// Ubicaciones guardadas
	fmt.Println("\n📍 Ubicaciones Guardadas:")
	if len(cfg.SecretLocations) == 0 {
		fmt.Println("  (ninguna)")
	} else {
		for _, loc := range cfg.SecretLocations {
			fmt.Printf("  • %s\n", loc)
		}
	}

	// Plantillas
	fmt.Printf("\n📝 Plantillas: %d configuradas\n", len(cfg.Templates))

	// Ruta del archivo de configuración
	cfgPath, _ := config.GetConfigPath()
	fmt.Printf("\n📁 Archivo de configuración: %s\n", cfgPath)

	return nil
}

func getValueOrNone(value string) string {
	if value == "" {
		return "(no configurado)"
	}
	return value
}

func boolToYesNo(value bool) string {
	if value {
		return "Sí"
	}
	return "No"
}
