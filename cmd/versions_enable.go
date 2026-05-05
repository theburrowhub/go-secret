package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

var versionsEnableCmd = &cobra.Command{
	Use:   "enable <secret-name> <version>",
	Short: "Enable a disabled version",
	Long: `Habilita una versión previamente deshabilitada de un secreto.

Solo se pueden habilitar versiones que estén en estado DISABLED.
Las versiones en estado DESTROYED no pueden ser habilitadas.

Ejemplos:
  go-secret versions enable database-password 3
  go-secret versions enable app/config/api-key 5`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVersionsEnable(args[0], args[1])
	},
}

func init() {
	versionsCmd.AddCommand(versionsEnableCmd)
}

func runVersionsEnable(secretName, version string) error {
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
	defer func() { _ = client.Close() }()

	// Habilitar la versión
	if err := client.EnableSecretVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error habilitando versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' habilitada exitosamente\n", version, secretName)

	return nil
}
