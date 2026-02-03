package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/audit"
	"github.com/theburrowhub/go-secret/internal/cli"
	"github.com/theburrowhub/go-secret/internal/gcp"
)

var (
	listFilter string
	listOutput string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets in the project",
	Long: `Lists all secrets available in the GCP project.

You can filter results using the --filter flag to search by name.

Examples:
  go-secret list
  go-secret list --filter database
  go-secret list --project my-gcp-project
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

	// Initialize GCP client using helper
	cfg, client, proj, err := cli.InitGCPClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer client.Close()

	// List secrets
	secrets, err := client.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("error listing secrets: %w", err)
	}

	// Filter if specified
	if listFilter != "" {
		filtered := make([]gcp.Secret, 0)
		for _, s := range secrets {
			if strings.Contains(strings.ToLower(s.Name), strings.ToLower(listFilter)) {
				filtered = append(filtered, s)
			}
		}
		secrets = filtered
	}

	// Sort by name
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Name < secrets[j].Name
	})

	// Initialize audit logger and log successful operation
	auditLog := cli.NewAuditLogger(cfg, client)
	if auditLog != nil {
		defer auditLog.Close()
		auditLog.LogSecretList(proj, len(secrets), audit.ResultSuccess, "")
	}

	// Show results according to format
	switch listOutput {
	case "json":
		return outputJSON(secrets)
	case "yaml":
		return outputYAML(secrets)
	default:
		return outputTable(secrets, cfg.FolderSeparator)
	}
}

func outputTable(secrets []gcp.Secret, separator string) error {
	if len(secrets) == 0 {
		fmt.Println("No secrets found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tCREATED\tREPLICATION")
	fmt.Fprintln(w, "----\t-------\t-----------")

	for _, s := range secrets {
		// Show folder structure if using separator
		name := s.Name
		if separator != "" && separator != "/" {
			name = strings.ReplaceAll(name, separator, "/")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", name, s.CreateTime, s.Replication)
	}

	w.Flush()
	fmt.Printf("\nTotal: %d secrets\n", len(secrets))
	return nil
}

func outputJSON(secrets []gcp.Secret) error {
	// Simple JSON output implementation
	fmt.Println("[")
	for i, s := range secrets {
		comma := ","
		if i == len(secrets)-1 {
			comma = ""
		}
		fmt.Printf("  {\"name\": \"%s\", \"created\": \"%s\", \"replication\": \"%s\"}%s\n",
			s.Name, s.CreateTime, s.Replication, comma)
	}
	fmt.Println("]")
	return nil
}

func outputYAML(secrets []gcp.Secret) error {
	// Simple YAML output implementation
	fmt.Println("secrets:")
	for _, s := range secrets {
		fmt.Printf("  - name: %s\n", s.Name)
		fmt.Printf("    created: %s\n", s.CreateTime)
		fmt.Printf("    replication: %s\n", s.Replication)
	}
	return nil
}
