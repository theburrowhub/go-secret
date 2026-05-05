package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
)

var (
	auditLogsCount  int
	auditLogsRaw    bool
	auditLogsFilter string
)

var auditLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View audit logs",
	Long: `Muestra los logs de auditoría más recientes.

Los logs incluyen todas las operaciones realizadas con secretos:
  - Listado de secretos
  - Acceso y revelación de valores
  - Creación y eliminación
  - Cambios de configuración
  - Eventos de sesión

Ejemplos:
  go-secret audit logs
  go-secret audit logs --count 100
  go-secret audit logs --filter SECRET_REVEAL
  go-secret audit logs --raw`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuditLogs()
	},
}

func init() {
	auditCmd.AddCommand(auditLogsCmd)
	auditLogsCmd.Flags().IntVarP(&auditLogsCount, "count", "n", 50, "Número de entradas a mostrar")
	auditLogsCmd.Flags().BoolVarP(&auditLogsRaw, "raw", "r", false, "Mostrar JSON sin formato")
	auditLogsCmd.Flags().StringVarP(&auditLogsFilter, "filter", "f", "", "Filtrar por tipo de evento")
}

func runAuditLogs() error {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	if !cfg.Audit.Enabled {
		fmt.Println("⚠️  La auditoría no está habilitada")
		fmt.Println("Usa 'go-secret config set audit.enabled true' para habilitarla")
		return nil
	}

	// Crear logger para leer logs
	auditCfg := audit.Config{
		Enabled:    cfg.Audit.Enabled,
		FilePath:   cfg.Audit.FilePath,
		MaxSizeMB:  cfg.Audit.MaxSizeMB,
		MaxAgeDays: cfg.Audit.MaxAgeDays,
	}
	auditLogger, err := audit.NewLogger(auditCfg)
	if err != nil {
		return fmt.Errorf("error creando logger de auditoría: %w", err)
	}
	defer func() { _ = auditLogger.Close() }()

	// Leer logs recientes
	lines, err := auditLogger.ReadRecentLogs(auditLogsCount)
	if err != nil {
		return fmt.Errorf("error leyendo logs: %w", err)
	}

	if len(lines) == 0 {
		fmt.Println("No hay logs de auditoría disponibles")
		return nil
	}

	// Filtrar si se especificó
	if auditLogsFilter != "" {
		filtered := make([]string, 0)
		for _, line := range lines {
			var event audit.Event
			if err := json.Unmarshal([]byte(line), &event); err == nil {
				if strings.Contains(string(event.EventType), strings.ToUpper(auditLogsFilter)) {
					filtered = append(filtered, line)
				}
			}
		}
		lines = filtered
	}

	if len(lines) == 0 {
		fmt.Printf("No se encontraron logs que coincidan con el filtro: %s\n", auditLogsFilter)
		return nil
	}

	// Mostrar logs
	if auditLogsRaw {
		// Modo raw: mostrar JSON tal como está
		for _, line := range lines {
			fmt.Println(line)
		}
	} else {
		// Modo formateado: tabla
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "TIMESTAMP\tR\tEVENTO\tUSUARIO\tSECRETO")
		_, _ = fmt.Fprintln(w, "---------\t-\t------\t-------\t-------")

		for _, line := range lines {
			var event audit.Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				// Si no se puede parsear, mostrar línea tal cual
				_, _ = fmt.Fprintln(w, line)
				continue
			}

			// Formatear timestamp
			timestamp := event.Timestamp
			if len(timestamp) > 19 {
				timestamp = timestamp[:19]
			}
			timestamp = strings.Replace(timestamp, "T", " ", 1)

			// Formatear usuario
			user := event.User
			if len(user) > 20 {
				user = user[:17] + "..."
			}
			if user == "" {
				user = "-"
			}

			// Formatear secreto
			secret := event.SecretName
			if len(secret) > 25 {
				secret = secret[:22] + "..."
			}
			if secret == "" {
				secret = "-"
			}

			// Resultado
			result := "✓"
			if event.Result == audit.ResultFailure {
				result = "✗"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%-16s\t%-20s\t%s\n",
				timestamp, result, event.EventType, user, secret)
		}

		_ = w.Flush()
		fmt.Printf("\nMostrando %d de %d entradas más recientes\n", len(lines), auditLogsCount)
	}

	// Mostrar ruta del archivo
	fmt.Printf("\nArchivo de log: %s\n", auditLogger.GetFilePath())

	return nil
}
