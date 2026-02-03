package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/cli"
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

	// Initialize GCP client using helper
	cfg, client, proj, err := cli.InitGCPClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer client.Close()

	// Read secret value using helper
	value, err := cli.ReadSecretValue(createFromStdin, createFromFile, createValue, "Ingresa el valor del secreto (entrada oculta): ")
	if err != nil {
		return err
	}

	if len(value) == 0 {
		return fmt.Errorf("el valor del secreto no puede estar vacío")
	}

	// Parse labels using helper
	labels, err := cli.ParseLabels(createLabels)
	if err != nil {
		return err
	}

	// Initialize audit logger
	auditLog := cli.NewAuditLogger(cfg, client)
	if auditLog != nil {
		defer auditLog.Close()
	}

	// Crear el secreto
	if err := client.CreateSecret(ctx, secretName, labels, createLocation); err != nil {
		auditLog.LogSecretCreate(proj, secretName, audit.ResultFailure, err.Error())
		return fmt.Errorf("error creando secreto: %w", err)
	}

	// Añadir la primera versión con el valor
	version, err := client.AddSecretVersion(ctx, secretName, value)
	if err != nil {
		auditLog.LogVersionAdd(proj, secretName, "", audit.ResultFailure, err.Error())
		return fmt.Errorf("error añadiendo versión inicial: %w", err)
	}

	// Guardar ubicación en configuración si se especificó y no está guardada
	if createLocation != "" {
		cfg.AddSecretLocation(createLocation)
		_ = cfg.Save()
	}

	// Log successful operations
	auditLog.LogSecretCreate(proj, secretName, audit.ResultSuccess, "")
	auditLog.LogVersionAdd(proj, secretName, version.Name, audit.ResultSuccess, "")

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
