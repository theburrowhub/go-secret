package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/clipboard"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
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
  {{.ProjectID}}      - ID del proyecto GCP (vacío para otros backends)
  {{.SourceID}}       - ID de la fuente del secreto
  {{.Provider}}       - Tipo de backend ("gsm", "vault", etc.)

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

	cfg, reg, uc, err := loadRegistry(ctx)
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

	chosenSource := p.ID()

	// Build merged template list: per-source templates first, then global ones
	// (global templates with same Title as a per-source one are skipped).
	sourceTemplates := []config.Template{}
	for _, s := range cfg.Sources {
		if s.ID == chosenSource {
			sourceTemplates = s.Templates
			break
		}
	}
	templates := append([]config.Template{}, sourceTemplates...)
	seen := map[string]bool{}
	for _, t := range sourceTemplates {
		seen[t.Title] = true
	}
	for _, t := range cfg.Templates {
		if !seen[t.Title] {
			templates = append(templates, t)
		}
	}

	// Validar índice
	if templateGenerateIndex < 1 || templateGenerateIndex > len(templates) {
		return fmt.Errorf("índice fuera de rango: %d (debe estar entre 1 y %d)", templateGenerateIndex, len(templates))
	}

	// Obtener plantilla
	tmpl := templates[templateGenerateIndex-1]

	// Determine ProjectID from source config (only populated for GSM sources).
	projectIDVal := ""
	for _, sc := range cfg.Sources {
		if sc.ID == chosenSource && sc.Provider == "gsm" {
			projectIDVal = sc.ProjectID
			break
		}
	}

	// Preparar datos para la plantilla
	data := struct {
		SecretName     string
		FullSecretName string
		ProjectID      string
		SourceID       string
		Provider       string
	}{
		SecretName:     extractSecretName(secretName, p.FolderSeparator()),
		FullSecretName: secretName,
		ProjectID:      projectIDVal,
		SourceID:       chosenSource,
		Provider:       p.Kind(),
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
		if err := clipboard.WriteText(result); err != nil {
			return fmt.Errorf("error copiando al portapapeles: %w", err)
		}
		fmt.Println("\n✓ Código copiado al portapapeles")

		// Auto-clear si está configurado
		if cfg.Clipboard.AutoClear && cfg.Clipboard.TimeoutSeconds > 0 {
			timeout := cfg.Clipboard.TimeoutSeconds
			fmt.Printf("  El portapapeles se limpiará en %d segundos...\n", timeout)
			time.Sleep(time.Duration(timeout) * time.Second)
			if err := clipboard.Clear(); err != nil {
				fmt.Printf("⚠️  Error limpiando portapapeles: %v\n", err)
			} else {
				fmt.Println("🔒 Portapapeles limpiado")
			}
		}
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
