package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/sources"
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

	// Habilitar la versión
	if err := p.EnableVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error habilitando versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' habilitada exitosamente\n", version, secretName)

	return nil
}
