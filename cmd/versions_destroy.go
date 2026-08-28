package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/sources"
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

	// Confirmar destrucción si no se usa --force (before loading registry for UX)
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

	_, reg, uc, err := loadRegistry(ctx)
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

	// Destruir la versión
	if err := p.DestroyVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error destruyendo versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' destruida exitosamente\n", version, secretName)

	return nil
}
