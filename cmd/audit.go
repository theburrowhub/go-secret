package cmd

import (
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit log management",
	Long: `Comandos para gestionar y visualizar logs de auditoría.

La auditoría registra todas las operaciones realizadas con secretos
para cumplimiento y seguimiento de seguridad.

Comandos disponibles:
  logs - Visualiza los logs de auditoría`,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
