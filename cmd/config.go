package cmd

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long: `Comandos para gestionar la configuración de go-secret.

Permite ver y modificar configuraciones como el proyecto por defecto,
separador de carpetas, configuración de seguridad, y gestionar proyectos
recientes.

Comandos disponibles:
  get      - Muestra la configuración actual
  set      - Establece valores de configuración
  projects - Gestiona proyectos recientes`,
}

func init() {
	rootCmd.AddCommand(configCmd)
}
