package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var (
	templateCreateTitle string
	templateCreateCode  string
)

var templatesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new template",
	Long: `Crea una nueva plantilla de código.

Puedes especificar el título y código usando flags, o de forma interactiva.

Variables disponibles:
  {{.SecretName}}     - Nombre del secreto
  {{.FullSecretName}} - Nombre completo del secreto
  {{.ProjectID}}      - ID del proyecto GCP

Ejemplos:
  go-secret templates create
  go-secret templates create --title "Python Env" --code 'import os\nPWD = os.getenv("{{.SecretName}}")'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplatesCreate()
	},
}

func init() {
	templatesCmd.AddCommand(templatesCreateCmd)
	templatesCreateCmd.Flags().StringVarP(&templateCreateTitle, "title", "t", "", "Título de la plantilla")
	templatesCreateCmd.Flags().StringVarP(&templateCreateCode, "code", "c", "", "Código de la plantilla")
}

func runTemplatesCreate() error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	title := templateCreateTitle
	code := templateCreateCode

	// Modo interactivo si no se proporcionan flags
	reader := bufio.NewReader(os.Stdin)

	if title == "" {
		fmt.Print("Título de la plantilla: ")
		titleInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error leyendo título: %w", err)
		}
		title = strings.TrimSpace(titleInput)
	}

	if title == "" {
		return fmt.Errorf("el título no puede estar vacío")
	}

	if code == "" {
		fmt.Println("Código de la plantilla (termina con una línea que solo contenga 'EOF'):")
		var lines []string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error leyendo código: %w", err)
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "EOF" {
				break
			}
			lines = append(lines, strings.TrimRight(line, "\n"))
		}
		code = strings.Join(lines, "\n")
	}

	if code == "" {
		return fmt.Errorf("el código no puede estar vacío")
	}

	// Añadir la nueva plantilla
	cfg.Templates = append(cfg.Templates, config.Template{
		Title: title,
		Code:  code,
	})

	// Guardar configuración
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
	}

	fmt.Printf("✓ Plantilla '%s' creada exitosamente\n", title)
	fmt.Printf("  Total de plantillas: %d\n", len(cfg.Templates))

	return nil
}
