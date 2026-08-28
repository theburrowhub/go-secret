package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/sources"
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

	// Deshabilitar la versión
	if err := p.DisableVersion(ctx, secretName, version); err != nil {
		return fmt.Errorf("error deshabilitando versión: %w", err)
	}

	fmt.Printf("✓ Versión %s del secreto '%s' deshabilitada exitosamente\n", version, secretName)

	return nil
}
