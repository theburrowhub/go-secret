package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/config"
)

var configProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage recent projects",
	Long: `Gestiona la lista de proyectos GCP recientes.

Subcomandos:
  list   - Lista proyectos recientes
  add    - Añade un proyecto a la lista
  remove - Elimina un proyecto de la lista
  switch - Cambia al proyecto especificado

Ejemplos:
  go-secret config projects list
  go-secret config projects add my-new-project
  go-secret config projects remove old-project
  go-secret config projects switch another-project`,
}

var configProjectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista proyectos recientes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigProjectsList()
	},
}

var configProjectsAddCmd = &cobra.Command{
	Use:   "add <project-id>",
	Short: "Añade un proyecto a la lista",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigProjectsAdd(args[0])
	},
}

var configProjectsRemoveCmd = &cobra.Command{
	Use:   "remove <index>",
	Short: "Elimina un proyecto de la lista",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("índice inválido: %s", args[0])
		}
		return runConfigProjectsRemove(index)
	},
}

var configProjectsSwitchCmd = &cobra.Command{
	Use:   "switch <project-id>",
	Short: "Cambia al proyecto especificado",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigProjectsSwitch(args[0])
	},
}

func init() {
	configCmd.AddCommand(configProjectsCmd)
	configProjectsCmd.AddCommand(configProjectsListCmd)
	configProjectsCmd.AddCommand(configProjectsAddCmd)
	configProjectsCmd.AddCommand(configProjectsRemoveCmd)
	configProjectsCmd.AddCommand(configProjectsSwitchCmd)
}

func runConfigProjectsList() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	if len(cfg.RecentProjects) == 0 {
		fmt.Println("No hay proyectos recientes")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "#\tPROYECTO\tACTUAL")
	_, _ = fmt.Fprintln(w, "-\t--------\t------")

	for i, p := range cfg.RecentProjects {
		current := ""
		if p == cfg.ProjectID {
			current = "✓"
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, p, current)
	}

	_ = w.Flush()
	fmt.Printf("\nTotal: %d proyectos\n", len(cfg.RecentProjects))

	return nil
}

func runConfigProjectsAdd(projectID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	cfg.AddRecentProject(projectID)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
	}

	fmt.Printf("✓ Proyecto '%s' añadido a la lista\n", projectID)

	return nil
}

func runConfigProjectsRemove(index int) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	if index < 1 || index > len(cfg.RecentProjects) {
		return fmt.Errorf("índice fuera de rango: %d (debe estar entre 1 y %d)", index, len(cfg.RecentProjects))
	}

	idx := index - 1
	projectID := cfg.RecentProjects[idx]

	// Confirmar eliminación
	fmt.Printf("¿Eliminar proyecto '%s' de la lista? (escribe 'yes' para confirmar): ", projectID)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error leyendo confirmación: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" {
		fmt.Println("Eliminación cancelada.")
		return nil
	}

	cfg.RemoveProject(projectID)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
	}

	fmt.Printf("✓ Proyecto '%s' eliminado de la lista\n", projectID)

	return nil
}

func runConfigProjectsSwitch(projectID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error cargando configuración: %w", err)
	}

	oldProject := cfg.ProjectID
	cfg.ProjectID = projectID
	cfg.AddRecentProject(projectID)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error guardando configuración: %w", err)
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
			auditLogger.LogProjectSwitch(oldProject, projectID)
		}
	}

	fmt.Printf("✓ Proyecto cambiado a '%s'\n", projectID)

	return nil
}
