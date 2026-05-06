package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/sources"
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

	// Obtener secreto
	secret, err := p.Get(ctx, secretName)
	if err != nil {
		return fmt.Errorf("error obteniendo secreto: %w", err)
	}

	// Obtener versiones si se solicitó
	var versions []sources.Version
	if getVersions {
		versions, err = p.ListVersions(ctx, secretName)
		if err != nil {
			return fmt.Errorf("error listando versiones: %w", err)
		}
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
			auditLogger.SetUser(p.UserEmail())
			auditLogger.SetSource(p.ID(), p.Kind())
			auditLogger.LogSecretAccess(p.ID(), secretName, "", audit.ResultSuccess, "")
		}
	}

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

func outputGetTable(secret *sources.Secret, versions []sources.Version) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	_, _ = fmt.Fprintln(w, "CAMPO\tVALOR")
	_, _ = fmt.Fprintln(w, "-----\t-----")
	_, _ = fmt.Fprintf(w, "Nombre\t%s\n", secret.Name)
	_, _ = fmt.Fprintf(w, "Fuente\t%s\n", secret.SourceID)
	_, _ = fmt.Fprintf(w, "Creado\t%s\n", secret.CreateTime)
	_, _ = fmt.Fprintf(w, "Replicación\t%s\n", secret.Replication)

	if len(secret.Labels) > 0 {
		_, _ = fmt.Fprintf(w, "Etiquetas\t")
		first := true
		for k, v := range secret.Labels {
			if !first {
				_, _ = fmt.Fprintf(w, ", ")
			}
			_, _ = fmt.Fprintf(w, "%s=%s", k, v)
			first = false
		}
		_, _ = fmt.Fprintf(w, "\n")
	}

	_ = w.Flush()

	if len(versions) > 0 {
		fmt.Println("\nVersiones:")
		fmt.Println()
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "VERSIÓN\tESTADO\tCREADA")
		_, _ = fmt.Fprintln(w, "-------\t------\t------")

		for _, v := range versions {
			state := v.State
			// Formatear estado para mejor legibilidad
			switch v.State {
			case "STATE_ENABLED", "ENABLED":
				state = "✓ ENABLED"
			case "STATE_DISABLED", "DISABLED":
				state = "○ DISABLED"
			case "STATE_DESTROYED", "DESTROYED":
				state = "✕ DESTROYED"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, state, v.CreateTime)
		}
		_ = w.Flush()
		fmt.Printf("\nTotal: %d versiones\n", len(versions))
	}

	return nil
}

func outputGetJSON(secret *sources.Secret, versions []sources.Version) error {
	fmt.Printf("{\n")
	fmt.Printf("  \"name\": \"%s\",\n", secret.Name)
	fmt.Printf("  \"source_id\": \"%s\",\n", secret.SourceID)
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

func outputGetYAML(secret *sources.Secret, versions []sources.Version) error {
	fmt.Printf("name: %s\n", secret.Name)
	fmt.Printf("source_id: %s\n", secret.SourceID)
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
