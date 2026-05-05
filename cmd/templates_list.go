package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all templates",
	Long: `Muestra todas las plantillas de código disponibles.

Las plantillas se usan para generar código personalizado con información
de secretos.

Ejemplo:
  go-secret templates list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplatesList()
	},
}

func init() {
	templatesCmd.AddCommand(templatesListCmd)
}

func runTemplatesList() error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	if len(cfg.Templates) == 0 {
		fmt.Println("No hay plantillas configuradas")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "#\tTÍTULO\tVISTA PREVIA")
	_, _ = fmt.Fprintln(w, "-\t------\t------------")

	for i, t := range cfg.Templates {
		// Reemplazar saltos de línea para mostrar en una línea
		preview := ""
		lines := 0
		for _, ch := range t.Code {
			if ch == '\n' {
				lines++
				if lines > 1 {
					preview += "..."
					break
				}
				preview += " "
			} else {
				preview += string(ch)
			}
		}
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}

		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, t.Title, preview)
	}

	_ = w.Flush()
	fmt.Printf("\nTotal: %d plantillas\n", len(cfg.Templates))
	fmt.Println("\nVariables disponibles:")
	fmt.Println("  {{.SecretName}}     - Nombre del secreto")
	fmt.Println("  {{.FullSecretName}} - Nombre completo del secreto")
	fmt.Println("  {{.ProjectID}}      - ID del proyecto GCP")

	return nil
}
