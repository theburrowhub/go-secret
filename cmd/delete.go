package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/cli"
)

var (
	deleteForce bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <secret-name>",
	Short: "Delete a secret",
	Long: `Elimina un secreto y todas sus versiones de GCP Secret Manager.

⚠️  ADVERTENCIA: Esta acción es irreversible.
Todas las versiones del secreto serán destruidas permanentemente.

Por defecto, solicita confirmación antes de eliminar.
Usa --force para omitir la confirmación (útil para scripts).

Ejemplos:
  go-secret delete old-api-key
  go-secret delete database-password --force
  go-secret delete app/config/deprecated --project my-gcp-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDelete(args[0])
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Eliminar sin confirmación")
}

func runDelete(secretName string) error {
	ctx := context.Background()

	// Initialize GCP client using helper
	cfg, client, proj, err := cli.InitGCPClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer client.Close()

	// Confirm action using helper
	message := fmt.Sprintf("⚠️  Estás a punto de eliminar el secreto: %s\nEsta acción NO SE PUEDE DESHACER. Todas las versiones serán destruidas.", secretName)
	confirmed, err := cli.ConfirmAction(message, deleteForce)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Eliminación cancelada.")
		return nil
	}

	// Initialize audit logger
	auditLog, err := cli.NewAuditLogger(cfg, client)
	if err != nil {
		return fmt.Errorf("error inicializando audit logger: %w", err)
	}
	if auditLog != nil {
		defer auditLog.Close()
	}

	// Eliminar el secreto
	if err := client.DeleteSecret(ctx, secretName); err != nil {
		auditLog.LogSecretDelete(proj, secretName, audit.ResultFailure, err.Error())
		return fmt.Errorf("error eliminando secreto: %w", err)
	}

	// Log successful operation
	auditLog.LogSecretDelete(proj, secretName, audit.ResultSuccess, "")

	fmt.Printf("✓ Secreto '%s' eliminado exitosamente\n", secretName)

	return nil
}
