package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
)

var versionsListCmd = &cobra.Command{
	Use:   "list <secret-name>",
	Short: "List all versions of a secret",
	Long: `Muestra todas las versiones de un secreto específico.

Incluye información sobre el número de versión, estado y fecha de creación.

Estados posibles:
  ✓ ENABLED   - Versión activa y accesible
  ○ DISABLED  - Versión deshabilitada pero recuperable
  ✕ DESTROYED - Versión destruida permanentemente

Ejemplos:
  go-secret versions list database-password
  go-secret versions list app/config/api-key --project my-gcp-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVersionsList(args[0])
	},
}

func init() {
	versionsCmd.AddCommand(versionsListCmd)
}

func runVersionsList(secretName string) error {
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

	// Crear cliente GCP
	client, err := gsm.NewClient(ctx, proj)
	if err != nil {
		return fmt.Errorf("error creando cliente GCP: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Listar versiones
	versions, err := client.ListSecretVersions(ctx, secretName)
	if err != nil {
		return fmt.Errorf("error listando versiones: %w", err)
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
			_ = auditLogger.Log(audit.Event{
				EventType:  audit.EventVersionList,
				Result:     audit.ResultSuccess,
				ProjectID:  proj,
				SecretName: secretName,
			})
		}
	}

	if len(versions) == 0 {
		fmt.Println("No se encontraron versiones")
		return nil
	}

	// Mostrar tabla de versiones
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "VERSIÓN\tESTADO\tCREADA")
	_, _ = fmt.Fprintln(w, "-------\t------\t------")

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
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, state, v.CreateTime)
	}

	_ = w.Flush()
	fmt.Printf("\nTotal: %d versiones\n", len(versions))

	return nil
}
