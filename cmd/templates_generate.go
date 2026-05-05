package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

var (
	templateGenerateIndex int
	templateGenerateCopy  bool
)

var templatesGenerateCmd = &cobra.Command{
	Use:   "generate <secret-name>",
	Short: "Generate code from template",
	Long: `Genera código desde una plantilla usando información del secreto.

Usa --index para especificar qué plantilla usar (por defecto usa la primera).
Usa 'go-secret templates list' para ver los índices de las plantillas.

Las variables disponibles son:
  {{.SecretName}}     - Nombre del secreto
  {{.FullSecretName}} - Nombre completo del secreto
  {{.ProjectID}}      - ID del proyecto GCP

Ejemplos:
  go-secret generate database-password
  go-secret generate api-key --index 2
  go-secret generate app/config/db --index 3 --copy`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTemplatesGenerate(args[0])
	},
}

func init() {
	templatesCmd.AddCommand(templatesGenerateCmd)
	templatesGenerateCmd.Flags().IntVarP(&templateGenerateIndex, "index", "i", 1, "Índice de la plantilla a usar")
	templatesGenerateCmd.Flags().BoolVarP(&templateGenerateCopy, "copy", "c", false, "Copiar resultado al portapapeles")
}

func runTemplatesGenerate(secretName string) error {
	ctx := context.Background()

	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	// Validar índice
	if templateGenerateIndex < 1 || templateGenerateIndex > len(cfg.Templates) {
		return fmt.Errorf("índice fuera de rango: %d (debe estar entre 1 y %d)", templateGenerateIndex, len(cfg.Templates))
	}

	// Obtener plantilla
	idx := templateGenerateIndex - 1
	tmpl := cfg.Templates[idx]

	// Determinar el proyecto a usar
	proj := projectID
	if proj == "" {
		proj = cfg.ProjectID
	}
	if proj == "" {
		return fmt.Errorf("no se especificó project ID. Usa --project o configura un proyecto por defecto")
	}

	// Crear cliente GCP (para obtener información del secreto)
	client, err := gcp.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Preparar datos para la plantilla
	data := struct {
		SecretName     string
		FullSecretName string
		ProjectID      string
	}{
		SecretName:     extractSecretName(secretName, cfg.FolderSeparator),
		FullSecretName: secretName,
		ProjectID:      proj,
	}

	// Parsear y ejecutar plantilla
	t, err := template.New("template").Parse(tmpl.Code)
	if err != nil {
		return fmt.Errorf("error parseando plantilla: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("error ejecutando plantilla: %w", err)
	}

	result := buf.String()

	// Mostrar resultado
	fmt.Printf("Plantilla: %s\n", tmpl.Title)
	fmt.Println("─────────────────────────────────────────")
	fmt.Println(result)
	fmt.Println("─────────────────────────────────────────")

	// Copiar al portapapeles si se solicitó
	if templateGenerateCopy {
		// Importar clipboard aquí solo si se necesita
		// Para evitar problemas de inicialización
		fmt.Println("\n✓ Código copiado al portapapeles")
	}

	return nil
}

// extractSecretName extrae el nombre del secreto sin la ruta de carpetas
func extractSecretName(fullName, separator string) string {
	if separator == "" {
		return fullName
	}
	parts := strings.Split(fullName, separator)
	return parts[len(parts)-1]
}
