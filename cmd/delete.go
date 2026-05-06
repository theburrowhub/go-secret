package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/sources"
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

	// Confirmar eliminación si no se usa --force (before loading registry for UX)
	if !deleteForce {
		fmt.Printf("⚠️  Estás a punto de eliminar el secreto: %s\n", secretName)
		fmt.Println("Esta acción NO SE PUEDE DESHACER. Todas las versiones serán destruidas.")
		fmt.Print("\n¿Estás seguro? (escribe 'yes' para confirmar): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error leyendo confirmación: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Eliminación cancelada.")
			return nil
		}
	}

	cfg, reg, uc, err := loadRegistry(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = reg.Close() }()

	p, err := uc.Resolve(ctx, secretName, sourceID)
	if err != nil {
		if errors.Is(err, sources.ErrAmbiguousSecret) {
			return fmt.Errorf("%w. Use --source <id>", err)
		}
		return err
	}

	// Eliminar el secreto
	if err := p.Delete(ctx, secretName); err != nil {
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
				auditLogger.SetUser(p.UserEmail())
				auditLogger.LogSecretDelete(p.ID(), secretName, audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error eliminando secreto: %w", err)
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
			auditLogger.SetUser(p.UserEmail())
			auditLogger.LogSecretDelete(p.ID(), secretName, audit.ResultSuccess, "")
		}
	}

	fmt.Printf("✓ Secreto '%s' eliminado exitosamente\n", secretName)

	return nil
}
