package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
)

var versionsDisableCmd = &cobra.Command{
	Use:   "disable <secret-name> <version>",
	Short: "Disable an active version",
	Long: `Deshabilita una versión activa de un secreto.

La versión deshabilitada ya no será accesible pero puede ser habilitada
nuevamente más tarde usando el comando 'versions enable'.

Ejemplos:
  go-secret versions disable database-password 2
  go-secret versions disable app/config/api-key 4`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVersionsDisable(args[0], args[1])
	},
}

func init() {
	versionsCmd.AddCommand(versionsDisableCmd)
}

func runVersionsDisable(secretName, version string) error {
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

	// Deshabilitar la versión
	if err := client.DisableSecretVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error deshabilitando versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' deshabilitada exitosamente\n", version, secretName)

	return nil
}
