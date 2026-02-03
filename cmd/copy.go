package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/clipboard"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

var (
	copyVersion    string
	copyNoAutoClear bool
)

var copyCmd = &cobra.Command{
	Use:   "copy <secret-name>",
	Short: "Copy secret value to clipboard",
	Long: `Copia el valor de un secreto al portapapeles del sistema.

Por defecto copia la versión más reciente (latest).
Usa --version para especificar una versión diferente.

El portapapeles se limpiará automáticamente después del tiempo configurado
(por defecto 30 segundos) a menos que uses --no-auto-clear.

ADVERTENCIA: El valor del secreto estará disponible en el portapapeles.
Asegúrate de estar en un entorno seguro.

Ejemplos:
  go-secret copy database-password
  go-secret copy api-key --version 3
  go-secret copy app/config/secret --no-auto-clear`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCopy(args[0])
	},
}

func init() {
	rootCmd.AddCommand(copyCmd)
	copyCmd.Flags().StringVarP(&copyVersion, "version", "v", "latest", "Versión del secreto a copiar (número o 'latest')")
	copyCmd.Flags().BoolVar(&copyNoAutoClear, "no-auto-clear", false, "No limpiar el portapapeles automáticamente")
}

func runCopy(secretName string) error {
	ctx := context.Background()

	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	// Determinar el proyecto a usar
	proj := projectID
	if proj == "" {
		proj = cfg.ProjectID
	}
	if proj == "" {
		return fmt.Errorf("no se especificó project ID. Usa --project o configura un proyecto por defecto")
	}

	// Crear cliente GCP
	client, err := gcp.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer client.Close()

	// Acceder al valor del secreto
	payload, err := client.AccessSecretVersion(ctx, secretName, copyVersion)
	if err != nil {
		// Registrar error en audit log
		if cfg.Audit.Enabled {
			auditCfg := audit.Config{
				Enabled:    cfg.Audit.Enabled,
				FilePath:   cfg.Audit.FilePath,
				MaxSizeMB:  cfg.Audit.MaxSizeMB,
				MaxAgeDays: cfg.Audit.MaxAgeDays,
			}
			auditLogger, _ := audit.NewLogger(auditCfg)
			if auditLogger != nil {
				defer auditLogger.Close()
				auditLogger.SetUser(client.UserEmail())
				auditLogger.LogSecretCopy(proj, secretName, copyVersion, audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error accediendo al secreto: %w", err)
	}

	// Copiar al portapapeles
	if err := clipboard.WriteText(string(payload)); err != nil {
		return fmt.Errorf("error copiando al portapapeles: %w", err)
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
			defer auditLogger.Close()
			auditLogger.SetUser(client.UserEmail())
			auditLogger.LogSecretCopy(proj, secretName, copyVersion, audit.ResultSuccess, "")
		}
	}

	fmt.Printf("✓ Secreto copiado al portapapeles\n")

	// Auto-clear si está configurado
	autoClear := cfg.Clipboard.AutoClear && !copyNoAutoClear
	if autoClear {
		timeout := cfg.Clipboard.TimeoutSeconds
		if timeout <= 0 {
			timeout = 30 // Valor por defecto
		}

		fmt.Printf("  El portapapeles se limpiará en %d segundos...\n", timeout)

		// Esperar y limpiar
		time.Sleep(time.Duration(timeout) * time.Second)

		if err := clipboard.Clear(); err != nil {
			fmt.Printf("⚠️  Error limpiando portapapeles: %v\n", err)
		} else {
			fmt.Println("🔒 Portapapeles limpiado")

			// Registrar limpieza en audit log
			if cfg.Audit.Enabled {
				auditCfg := audit.Config{
					Enabled:    cfg.Audit.Enabled,
					FilePath:   cfg.Audit.FilePath,
					MaxSizeMB:  cfg.Audit.MaxSizeMB,
					MaxAgeDays: cfg.Audit.MaxAgeDays,
				}
				auditLogger, err := audit.NewLogger(auditCfg)
				if err == nil {
					defer auditLogger.Close()
					auditLogger.SetUser(client.UserEmail())
					auditLogger.LogClipboardClear()
				}
			}
		}
	}

	return nil
}
