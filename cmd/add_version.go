package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
	"golang.org/x/term"
)

var (
	addVersionValue     string
	addVersionFromFile  string
	addVersionFromStdin bool
)

var addVersionCmd = &cobra.Command{
	Use:   "add-version <secret-name>",
	Short: "Add a new version to an existing secret",
	Long: `Añade una nueva versión a un secreto existente en GCP Secret Manager.

El valor puede proporcionarse de varias formas:
  • Usando --value (no recomendado, visible en historial de shell)
  • Usando --from-file para leer desde un archivo
  • Usando --from-stdin para leer desde entrada estándar
  • Modo interactivo (por defecto) que solicita el valor de forma segura

Ejemplos:
  go-secret add-version database-password
  go-secret add-version api-key --from-file ./new-key.txt
  echo "new-secret-value" | go-secret add-version my-secret --from-stdin
  go-secret add-version app/config/db --value "new-connection-string"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAddVersion(args[0])
	},
}

func init() {
	rootCmd.AddCommand(addVersionCmd)
	addVersionCmd.Flags().StringVarP(&addVersionValue, "value", "v", "", "Valor de la nueva versión (no recomendado)")
	addVersionCmd.Flags().StringVarP(&addVersionFromFile, "from-file", "f", "", "Leer valor desde archivo")
	addVersionCmd.Flags().BoolVarP(&addVersionFromStdin, "from-stdin", "s", false, "Leer valor desde stdin")
}

func runAddVersion(secretName string) error {
	ctx := context.Background()

	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	// Determinar el proyecto a usar
	proj := projectID
	if proj == "" {
		proj = cfg.ProjectID
	}
	if proj == "" {
		return fmt.Errorf("no se especificó project ID. Usa --project o configura un proyecto por defecto")
	}

	// Obtener el valor de la nueva versión
	var value []byte
	switch {
	case addVersionFromStdin:
		// Leer desde stdin
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error leyendo desde stdin: %w", err)
		}
		value = []byte(strings.Join(lines, "\n"))

	case addVersionFromFile != "":
		// Leer desde archivo
		fileData, err := os.ReadFile(addVersionFromFile)
		if err != nil {
			return fmt.Errorf("error leyendo archivo: %w", err)
		}
		value = fileData

	case addVersionValue != "":
		// Usar valor de flag (no recomendado)
		value = []byte(addVersionValue)

	default:
		// Modo interactivo (por defecto)
		fmt.Printf("Ingresa el valor de la nueva versión (entrada oculta): ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Nueva línea después de la entrada
		if err != nil {
			return fmt.Errorf("error leyendo valor: %w", err)
		}
		value = password
	}

	if len(value) == 0 {
		return fmt.Errorf("el valor de la versión no puede estar vacío")
	}

	// Crear cliente GCP
	client, err := gsm.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Añadir la nueva versión
	version, err := client.AddSecretVersion(ctx, secretName, value)
	if err != nil {
		// Registrar error en audit log
		if cfg.Audit.Enabled {
			auditCfg := audit.Config{
				Enabled:    cfg.Audit.Enabled,
				FilePath:   cfg.Audit.FilePath,
				MaxSizeMB:  cfg.Audit.MaxSizeMB,
				MaxAgeDays: cfg.Audit.MaxAgeDays,
			}
			auditLogger, _ := audit.NewLogger(auditCfg)
			if auditLogger != nil {
				defer func() { _ = auditLogger.Close() }()
				auditLogger.SetUser(client.UserEmail())
				auditLogger.LogVersionAdd(proj, secretName, "", audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error añadiendo versión: %w", err)
	}

	// Registrar en audit log si está habilitado
	if cfg.Audit.Enabled {
		auditCfg := audit.Config{
			Enabled:    cfg.Audit.Enabled,
			FilePath:   cfg.Audit.FilePath,
			MaxSizeMB:  cfg.Audit.MaxSizeMB,
			MaxAgeDays: cfg.Audit.MaxAgeDays,
		}
		auditLogger, err := audit.NewLogger(auditCfg)
		if err == nil {
			defer func() { _ = auditLogger.Close() }()
			auditLogger.SetUser(client.UserEmail())
			auditLogger.LogVersionAdd(proj, secretName, version.Name, audit.ResultSuccess, "")
		}
	}

	fmt.Printf("✓ Nueva versión añadida al secreto '%s'\n", secretName)
	fmt.Printf("  Versión: %s\n", version.Name)
	fmt.Printf("  Estado: %s\n", version.State)
	fmt.Printf("  Creada: %s\n", version.CreateTime)

	return nil
}
