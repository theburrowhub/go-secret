package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
)

var (
	revealVersion string
)

var revealCmd = &cobra.Command{
	Use:   "reveal <secret-name>",
	Short: "Reveal secret value",
	Long: `Muestra el valor real de un secreto o una versión específica.

Por defecto muestra la versión más reciente (latest).
Usa --version para especificar una versión diferente.

ADVERTENCIA: Este comando muestra datos sensibles en texto plano.
Asegúrate de estar en un entorno seguro.

Ejemplos:
  go-secret reveal database-password
  go-secret reveal database-password --version 3
  go-secret reveal app/config/api-key --version latest`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReveal(args[0])
	},
}

func init() {
	rootCmd.AddCommand(revealCmd)
	revealCmd.Flags().StringVarP(&revealVersion, "version", "v", "latest", "Versión del secreto a revelar (número o 'latest')")
}

func runReveal(secretName string) error {
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
	client, err := gsm.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Acceder al valor del secreto
	payload, err := client.AccessSecretVersion(ctx, secretName, revealVersion)
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
				defer func() { _ = auditLogger.Close() }()
				auditLogger.SetUser(client.UserEmail())
				auditLogger.LogSecretReveal(proj, secretName, revealVersion, audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error accediendo al secreto: %w", err)
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
			auditLogger.SetUser(client.UserEmail())
			auditLogger.LogSecretReveal(proj, secretName, revealVersion, audit.ResultSuccess, "")
		}
	}

	// Mostrar el valor
	fmt.Println(string(payload))

	return nil
}
