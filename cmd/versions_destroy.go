package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
)

var versionsDestroyForce bool

var versionsDestroyCmd = &cobra.Command{
	Use:   "destroy <secret-name> <version>",
	Short: "Permanently destroy a version",
	Long: `Destruye permanentemente una versión de un secreto.

⚠️  ADVERTENCIA: Esta acción es irreversible.
Una vez destruida, la versión no podrá ser recuperada ni habilitada.

Por defecto, solicita confirmación antes de destruir.
Usa --force para omitir la confirmación (útil para scripts).

Ejemplos:
  go-secret versions destroy database-password 1
  go-secret versions destroy api-key 3 --force`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVersionsDestroy(args[0], args[1])
	},
}

func init() {
	versionsCmd.AddCommand(versionsDestroyCmd)
	versionsDestroyCmd.Flags().BoolVarP(&versionsDestroyForce, "force", "f", false, "Destruir sin confirmación")
}

func runVersionsDestroy(secretName, version string) error {
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

	// Confirmar destrucción si no se usa --force
	if !versionsDestroyForce {
		fmt.Printf("⚠️  Estás a punto de destruir la versión %s del secreto: %s\n", version, secretName)
		fmt.Println("Esta acción NO SE PUEDE DESHACER. La versión será destruida permanentemente.")
		fmt.Print("\n¿Estás seguro? (escribe 'yes' para confirmar): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error leyendo confirmación: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Destrucción cancelada.")
			return nil
		}
	}

	// Crear cliente GCP
	client, err := gsm.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Destruir la versión
	if err := client.DestroySecretVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error destruyendo versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' destruida exitosamente\n", version, secretName)

	return nil
}
