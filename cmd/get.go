package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/cli"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

var (
	getOutput   string
	getVersions bool
)

var getCmd = &cobra.Command{
	Use:   "get <secret-name>",
	Short: "Get detailed secret information",
	Long: `Muestra información detallada de un secreto específico.

Incluye metadatos como fecha de creación, configuración de replicación,
y opcionalmente todas las versiones del secreto.

Ejemplos:
  go-secret get database-password
  go-secret get database-password --versions
  go-secret get app/config/api-key --project my-gcp-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGet(args[0])
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&getOutput, "output", "o", "table", "Formato de salida: table, json, yaml")
	getCmd.Flags().BoolVarP(&getVersions, "versions", "v", false, "Mostrar todas las versiones del secreto")
}

func runGet(secretName string) error {
	ctx := context.Background()

	// Initialize GCP client using helper
	cfg, client, proj, err := cli.InitGCPClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer client.Close()

	// Initialize audit logger
	auditLog := cli.NewAuditLogger(cfg, client)
	if auditLog != nil {
		defer auditLog.Close()
	}

	// Obtener secreto
	secret, err := client.GetSecret(ctx, secretName)
	if err != nil {
		return fmt.Errorf("error obteniendo secreto: %w", err)
	}

	// Obtener versiones si se solicitó
	var versions []gcp.SecretVersion
	if getVersions {
		versions, err = client.ListSecretVersions(ctx, secretName)
		if err != nil {
			return fmt.Errorf("error listando versiones: %w", err)
		}
	}

	// Log successful operation
	auditLog.LogSecretRead(proj, secretName, "", audit.ResultSuccess, "")

	// Mostrar resultados según formato
	switch getOutput {
	case "json":
		return outputGetJSON(secret, versions)
	case "yaml":
		return outputGetYAML(secret, versions)
	default:
		return outputGetTable(secret, versions)
	}
}

func outputGetTable(secret *gcp.Secret, versions []gcp.SecretVersion) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "CAMPO\tVALOR")
	fmt.Fprintln(w, "-----\t-----")
	fmt.Fprintf(w, "Nombre\t%s\n", secret.Name)
	fmt.Fprintf(w, "Nombre completo\t%s\n", secret.FullName)
	fmt.Fprintf(w, "Creado\t%s\n", secret.CreateTime)
	fmt.Fprintf(w, "Replicación\t%s\n", secret.Replication)

	if len(secret.Labels) > 0 {
		fmt.Fprintf(w, "Etiquetas\t")
		first := true
		for k, v := range secret.Labels {
			if !first {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%s=%s", k, v)
			first = false
		}
		fmt.Fprintf(w, "\n")
	}

	w.Flush()

	if len(versions) > 0 {
		fmt.Println("\nVersiones:")
		fmt.Println()
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "VERSIÓN\tESTADO\tCREADA")
		fmt.Fprintln(w, "-------\t------\t------")

		for _, v := range versions {
			state := v.State
			// Formatear estado para mejor legibilidad
			switch v.State {
			case "STATE_ENABLED":
				state = "✓ ENABLED"
			case "STATE_DISABLED":
				state = "○ DISABLED"
			case "STATE_DESTROYED":
				state = "✕ DESTROYED"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, state, v.CreateTime)
		}
		w.Flush()
		fmt.Printf("\nTotal: %d versiones\n", len(versions))
	}

	return nil
}

func outputGetJSON(secret *gcp.Secret, versions []gcp.SecretVersion) error {
	fmt.Printf("{\n")
	fmt.Printf("  \"name\": \"%s\",\n", secret.Name)
	fmt.Printf("  \"full_name\": \"%s\",\n", secret.FullName)
	fmt.Printf("  \"created\": \"%s\",\n", secret.CreateTime)
	fmt.Printf("  \"replication\": \"%s\"", secret.Replication)

	if len(secret.Labels) > 0 {
		fmt.Printf(",\n  \"labels\": {\n")
		i := 0
		for k, v := range secret.Labels {
			comma := ","
			if i == len(secret.Labels)-1 {
				comma = ""
			}
			fmt.Printf("    \"%s\": \"%s\"%s\n", k, v, comma)
			i++
		}
		fmt.Printf("  }")
	}

	if len(versions) > 0 {
		fmt.Printf(",\n  \"versions\": [\n")
		for i, v := range versions {
			comma := ","
			if i == len(versions)-1 {
				comma = ""
			}
			fmt.Printf("    {\"version\": \"%s\", \"state\": \"%s\", \"created\": \"%s\"}%s\n",
				v.Name, v.State, v.CreateTime, comma)
		}
		fmt.Printf("  ]")
	}

	fmt.Printf("\n}\n")
	return nil
}

func outputGetYAML(secret *gcp.Secret, versions []gcp.SecretVersion) error {
	fmt.Printf("name: %s\n", secret.Name)
	fmt.Printf("full_name: %s\n", secret.FullName)
	fmt.Printf("created: %s\n", secret.CreateTime)
	fmt.Printf("replication: %s\n", secret.Replication)

	if len(secret.Labels) > 0 {
		fmt.Printf("labels:\n")
		for k, v := range secret.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if len(versions) > 0 {
		fmt.Printf("versions:\n")
		for _, v := range versions {
			fmt.Printf("  - version: %s\n", v.Name)
			fmt.Printf("    state: %s\n", v.State)
			fmt.Printf("    created: %s\n", v.CreateTime)
		}
	}

	return nil
}
