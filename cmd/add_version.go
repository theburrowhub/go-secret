package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/cli"
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

	// Initialize GCP client using helper
	cfg, client, proj, err := cli.InitGCPClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer client.Close()

	// Read secret value using helper
	value, err := cli.ReadSecretValue(addVersionFromStdin, addVersionFromFile, addVersionValue, "Ingresa el valor de la nueva versión (entrada oculta): ")
	if err != nil {
		return err
	}

	if len(value) == 0 {
		return fmt.Errorf("el valor de la versión no puede estar vacío")
	}

	// Initialize audit logger
	auditLog, err := cli.NewAuditLogger(cfg, client)
	if err != nil {
		return fmt.Errorf("error inicializando audit logger: %w", err)
	}
	if auditLog != nil {
		defer auditLog.Close()
	}

	// Añadir la nueva versión
	version, err := client.AddSecretVersion(ctx, secretName, value)
	if err != nil {
		auditLog.LogVersionAdd(proj, secretName, "", audit.ResultFailure, err.Error())
		return fmt.Errorf("error añadiendo versión: %w", err)
	}

	// Log successful operation
	auditLog.LogVersionAdd(proj, secretName, version.Name, audit.ResultSuccess, "")

	fmt.Printf("✓ Nueva versión añadida al secreto '%s'\n", secretName)
	fmt.Printf("  Versión: %s\n", version.Name)
	fmt.Printf("  Estado: %s\n", version.State)
	fmt.Printf("  Creada: %s\n", version.CreateTime)

	return nil
}
