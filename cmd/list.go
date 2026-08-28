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
	listFilter string
	listOutput string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets in the project",
	Long: `Lists all secrets available across configured sources.

You can filter results using the --filter flag to search by name.
Use --source to restrict listing to a single backend.

Examples:
  go-secret list
  go-secret list --filter database
  go-secret list --source my-gsm
  go-secret list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listFilter, "filter", "f", "", "Filter secrets by name")
	listCmd.Flags().StringVarP(&listOutput, "output", "o", "table", "Output format: table, json, yaml")
}

func runList() error {
	ctx := context.Background()
	cfg, reg, uc, err := loadRegistry(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = reg.Close() }()

	var (
		secrets  []sources.Secret
		listErr  error
		listProv sources.Provider
	)
	if sourceID != "" {
		p, err := reg.Get(sourceID)
		if err != nil {
			return err
		}
		listProv = p
		secrets, listErr = p.List(ctx)
	} else {
		secrets, listErr = uc.List(ctx)
	}
	if listErr != nil {
		var pe *sources.PartialError
		if errors.As(listErr, &pe) {
			fmt.Fprintf(os.Stderr, "warning: partial failure: %v\n", pe)
		} else {
			return listErr
		}
	}

	// audit logging
	if cfg.Audit.Enabled {
		auditCfg := audit.Config{
			Enabled:    cfg.Audit.Enabled,
			FilePath:   cfg.Audit.FilePath,
			MaxSizeMB:  cfg.Audit.MaxSizeMB,
			MaxAgeDays: cfg.Audit.MaxAgeDays,
		}
		if auditLogger, auditErr := audit.NewLogger(auditCfg); auditErr == nil {
			defer func() { _ = auditLogger.Close() }()
			if listProv != nil {
				auditLogger.SetUser(listProv.UserEmail())
				auditLogger.SetSource(listProv.ID(), listProv.Kind())
			}
			auditLogger.LogSecretList(sourceID, len(secrets), audit.ResultSuccess, "")
		}
	}

	switch listOutput {
	case "json":
		return outputListJSON(secrets)
	case "yaml":
		return outputListYAML(secrets)
	default:
		return outputListTable(secrets)
	}
}

func outputListTable(secrets []sources.Secret) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "PROVIDER\tNAME\tCREATED\tREPLICATION")
	_, _ = fmt.Fprintln(w, "--------\t----\t-------\t-----------")
	for _, s := range secrets {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.SourceID, s.Name, s.CreateTime, s.Replication)
	}
	_ = w.Flush()
	fmt.Printf("\nTotal: %d secrets\n", len(secrets))
	return nil
}

func outputListJSON(secrets []sources.Secret) error {
	fmt.Println("[")
	for i, s := range secrets {
		comma := ","
		if i == len(secrets)-1 {
			comma = ""
		}
		fmt.Printf("  {\"source_id\": %q, \"name\": %q, \"created\": %q, \"replication\": %q}%s\n",
			s.SourceID, s.Name, s.CreateTime, s.Replication, comma)
	}
	fmt.Println("]")
	return nil
}

func outputListYAML(secrets []sources.Secret) error {
	for _, s := range secrets {
		fmt.Printf("- source_id: %s\n", s.SourceID)
		fmt.Printf("  name: %s\n", s.Name)
		fmt.Printf("  created: %s\n", s.CreateTime)
		fmt.Printf("  replication: %s\n", s.Replication)
	}
	return nil
}
