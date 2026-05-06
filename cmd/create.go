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
	"github.com/theburrowhub/go-secret/internal/sources"
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

	cfg, reg, _, err := loadRegistry(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = reg.Close() }()

	target := resolveActiveSource(cfg)
	if target == "" {
		picked, err := sources.PromptForSource(reg.Active())
		if err != nil {
			return fmt.Errorf("select source: %w", err)
		}
		target = picked
	}

	p, err := reg.Get(target)
	if err != nil {
		return fmt.Errorf("source %q not found: %w", target, err)
	}

	// Obtain the secret value.
	var value []byte
	switch {
	case createFromStdin:
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
		fileData, err := os.ReadFile(createFromFile)
		if err != nil {
			return fmt.Errorf("error leyendo archivo: %w", err)
		}
		value = fileData

	case createValue != "":
		value = []byte(createValue)

	default:
		fmt.Printf("Ingresa el valor del secreto (entrada oculta): ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("error leyendo valor: %w", err)
		}
		value = password
	}

	if len(value) == 0 {
		return fmt.Errorf("el valor del secreto no puede estar vacío")
	}

	// Parse labels.
	labels := make(map[string]string)
	for _, label := range createLabels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("formato de etiqueta inválido: %s (usa key=value)", label)
		}
		labels[parts[0]] = parts[1]
	}

	// Create the secret via the provider.
	createErr := p.Create(ctx, secretName, value, sources.CreateOpts{
		Labels:   labels,
		Location: createLocation,
	})
	if createErr != nil {
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
				auditLogger.SetUser(p.UserEmail())
				auditLogger.SetSource(p.ID(), p.Kind())
				auditLogger.LogSecretCreate(target, secretName, audit.ResultFailure, createErr.Error())
			}
		}
		return fmt.Errorf("error creando secreto: %w", createErr)
	}

	// Add the initial version with the value.
	version, versionErr := p.AddVersion(ctx, secretName, value)
	if versionErr != nil {
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
				auditLogger.SetUser(p.UserEmail())
				auditLogger.SetSource(p.ID(), p.Kind())
				auditLogger.LogVersionAdd(target, secretName, "", audit.ResultFailure, versionErr.Error())
			}
		}
		return fmt.Errorf("error añadiendo versión inicial: %w", versionErr)
	}

	// Save location in config if specified.
	if createLocation != "" {
		cfg.AddSecretLocation(createLocation)
		_ = cfg.Save()
	}

	// Audit log success.
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
			auditLogger.SetUser(p.UserEmail())
			auditLogger.SetSource(p.ID(), p.Kind())
			auditLogger.LogSecretCreate(target, secretName, audit.ResultSuccess, "")
			auditLogger.LogVersionAdd(target, secretName, version.Name, audit.ResultSuccess, "")
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
