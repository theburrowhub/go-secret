package cmd

import (
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Code template management",
	Long: `Comandos para gestionar plantillas de código para generación.

Las plantillas permiten generar código personalizado con información
de secretos usando variables como {{.SecretName}}, {{.FullSecretName}}
y {{.ProjectID}}.

Comandos disponibles:
  list     - Lista todas las plantillas
  create   - Crea una nueva plantilla
  edit     - Edita una plantilla existente
  delete   - Elimina una plantilla
  generate - Genera código desde una plantilla`,
}

func init() {
	rootCmd.AddCommand(templatesCmd)
}
