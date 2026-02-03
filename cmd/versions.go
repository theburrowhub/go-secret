package cmd

import (
	"github.com/spf13/cobra"
)

var versionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "Secret version management",
	Long: `Comandos para gestionar versiones de secretos en GCP Secret Manager.

Permite listar, habilitar, deshabilitar y destruir versiones específicas
de secretos existentes.

Comandos disponibles:
  list     - Lista todas las versiones de un secreto
  enable   - Habilita una versión deshabilitada
  disable  - Deshabilita una versión activa
  destroy  - Destruye permanentemente una versión`,
}

func init() {
	rootCmd.AddCommand(versionsCmd)
}
