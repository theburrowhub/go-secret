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
	"github.com/theburrowhub/go-secret/internal/gcp"
	"golang.org/x/term"
)

var (
	createValue      string
	createLocation   string
	createFromFile   string
	createFromStdin  bool
	createLabels     []string
	createInteractive bool
)

var createCmd = &cobra.Command{
	Use:   "create <secret-name>",
	Short: "Create a new secret",
	Long: `Crea un nuevo secreto en GCP Secret Manager.

El valor del secreto puede proporcionarse de varias formas:
  • Usando --value (no recomendado, visible en historial de shell)
  • Usando --from-file para leer desde un archivo
  • Usando --from-stdin para leer desde entrada estándar
  • Modo interactivo (por defecto) que solicita el valor de forma segura

La ubicación puede especificarse usando --location. Si no se especifica,
se usa replicación automática (global).

Ejemplos:
  go-secret create database-password
  go-secret create database-password --value "mysecret123"
  go-secret create api-key --from-file ./api-key.txt
  echo "secret-value" | go-secret create my-secret --from-stdin
  go-secret create app/config/db --location us-central1
  go-secret create prod-cert --from-file cert.pem --label env=prod --label team=backend`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate(args[0])
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createValue, "value", "v", "", "Valor del secreto (no recomendado, usa modo interactivo)")
	createCmd.Flags().StringVarP(&createLocation, "location", "l", "", "Ubicación de replicación (vacío = automática/global)")
	createCmd.Flags().StringVarP(&createFromFile, "from-file", "f", "", "Leer valor desde archivo")
	createCmd.Flags().BoolVarP(&createFromStdin, "from-stdin", "s", false, "Leer valor desde stdin")
	createCmd.Flags().StringSliceVar(&createLabels, "label", []string{}, "Etiquetas en formato key=value (puede repetirse)")
	createCmd.Flags().BoolVarP(&createInteractive, "interactive", "i", true, "Modo interactivo (solicita valor de forma segura)")
}

func runCreate(secretName string) error {
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

	// Obtener el valor del secreto
	var value []byte
	switch {
	case createFromStdin:
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

	case createFromFile != "":
		// Leer desde archivo
		fileData, err := os.ReadFile(createFromFile)
		if err != nil {
			return fmt.Errorf("error leyendo archivo: %w", err)
		}
		value = fileData

	case createValue != "":
		// Usar valor de flag (no recomendado)
		value = []byte(createValue)

	default:
		// Modo interactivo (por defecto)
		fmt.Printf("Ingresa el valor del secreto (entrada oculta): ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Nueva línea después de la entrada
		if err != nil {
			return fmt.Errorf("error leyendo valor: %w", err)
		}
		value = password
	}

	if len(value) == 0 {
		return fmt.Errorf("el valor del secreto no puede estar vacío")
	}

	// Parsear etiquetas
	labels := make(map[string]string)
	for _, label := range createLabels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("formato de etiqueta inválido: %s (usa key=value)", label)
		}
		labels[parts[0]] = parts[1]
	}

	// Crear cliente GCP
	client, err := gcp.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer client.Close()

	// Crear el secreto
	if err := client.CreateSecret(ctx, secretName, labels, createLocation); err != nil {
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
				defer auditLogger.Close()
				auditLogger.SetUser(client.UserEmail())
				auditLogger.LogSecretCreate(proj, secretName, audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error creando secreto: %w", err)
	}

	// Añadir la primera versión con el valor
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
				defer auditLogger.Close()
				auditLogger.SetUser(client.UserEmail())
				auditLogger.LogVersionAdd(proj, secretName, "", audit.ResultFailure, err.Error())
			}
		}
		return fmt.Errorf("error añadiendo versión inicial: %w", err)
	}

	// Guardar ubicación en configuración si se especificó y no está guardada
	if createLocation != "" {
		cfg.AddSecretLocation(createLocation)
		_ = cfg.Save()
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
			defer auditLogger.Close()
			auditLogger.SetUser(client.UserEmail())
			auditLogger.LogSecretCreate(proj, secretName, audit.ResultSuccess, "")
			auditLogger.LogVersionAdd(proj, secretName, version.Name, audit.ResultSuccess, "")
		}
	}

	fmt.Printf("✓ Secreto '%s' creado exitosamente\n", secretName)
	fmt.Printf("  Versión: %s\n", version.Name)
	fmt.Printf("  Estado: %s\n", version.State)
	if createLocation != "" {
		fmt.Printf("  Ubicación: %s\n", createLocation)
	} else {
		fmt.Printf("  Ubicación: automática (global)\n")
	}

	return nil
}
