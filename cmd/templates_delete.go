package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var templateDeleteForce bool

var templatesDeleteCmd = &cobra.Command{
	Use:   "delete <index>",
	Short: "Delete a template",
	Long: `Elimina una plantilla por su índice.

Usa 'go-secret templates list' para ver los índices de las plantillas.

Ejemplos:
  go-secret templates delete 3
  go-secret templates delete 1 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("índice inválido: %s", args[0])
		}
		return runTemplatesDelete(index)
	},
}

func init() {
	templatesCmd.AddCommand(templatesDeleteCmd)
	templatesDeleteCmd.Flags().BoolVarP(&templateDeleteForce, "force", "f", false, "Eliminar sin confirmación")
}

func runTemplatesDelete(index int) error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	// Validar índice
	if index < 1 || index > len(cfg.Templates) {
		return fmt.Errorf("índice fuera de rango: %d (debe estar entre 1 y %d)", index, len(cfg.Templates))
	}

	// Ajustar índice (de 1-based a 0-based)
	idx := index - 1
	template := cfg.Templates[idx]

	// Confirmar eliminación si no se usa --force
	if !templateDeleteForce {
		fmt.Printf("¿Eliminar plantilla '%s'? (escribe 'yes' para confirmar): ", template.Title)

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

	// Eliminar la plantilla
	cfg.Templates = append(cfg.Templates[:idx], cfg.Templates[idx+1:]...)

	// Guardar configuración
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
	}

	fmt.Printf("✓ Plantilla '%s' eliminada exitosamente\n", template.Title)
	fmt.Printf("  Total de plantillas restantes: %d\n", len(cfg.Templates))

	return nil
}
