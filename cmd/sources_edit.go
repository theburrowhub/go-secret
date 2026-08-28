package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an existing source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		idx := -1
		for i, s := range cfg.Sources {
			if s.ID == args[0] {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("source %q not found", args[0])
		}
		sc := &cfg.Sources[idx]

		r := bufio.NewReader(os.Stdin)
		ask := func(q, def string) string {
			if def != "" {
				fmt.Printf("%s [%s]: ", q, def)
			} else {
				fmt.Printf("%s: ", q)
			}
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return def
			}
			return line
		}
		askYesNo := func(q string, def bool) bool {
			defStr := "n"
			if def {
				defStr = "y"
			}
			ans := strings.ToLower(ask(q+" (y/n)", defStr))
			return ans == "y" || ans == "yes" || ans == "true"
		}
		askInt := func(q string, def int) int {
			defStr := strconv.Itoa(def)
			val := ask(q, defStr)
			n, err := strconv.Atoi(val)
			if err != nil {
				return def
			}
			return n
		}

		// Common fields
		sc.DisplayName = ask("Display name", sc.DisplayName)
		sc.FolderSeparator = ask("Folder separator", sc.FolderSeparator)
		sc.Enabled = askYesNo("Enabled?", sc.Enabled)

		switch sc.Provider {
		case "gsm":
			sc.ProjectID = ask("GCP Project ID", sc.ProjectID)
			locStr := strings.Join(sc.SecretLocations, ",")
			locStr = ask("Secret locations (comma-separated)", locStr)
			if locStr == "" {
				sc.SecretLocations = nil
			} else {
				parts := strings.Split(locStr, ",")
				out := parts[:0]
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						out = append(out, p)
					}
				}
				sc.SecretLocations = out
			}
		case "vault":
			sc.Address = ask("Vault address", sc.Address)
			sc.Auth.Method = ask("Auth method (token|approle|oidc)", sc.Auth.Method)
			switch sc.Auth.Method {
			case "oidc":
				sc.Auth.Role = ask("OIDC role", sc.Auth.Role)
				port := sc.Auth.OIDCPort
				if port == 0 {
					port = 8250
				}
				sc.Auth.OIDCPort = askInt("OIDC port", port)
			case "approle":
				sc.Auth.AppRoleRoleID = ask("AppRole role_id", sc.Auth.AppRoleRoleID)
			}
			editMounts(r, ask, askInt, &sc.Mounts)
		default:
			return fmt.Errorf("unknown provider %q", sc.Provider)
		}

		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q updated\n", sc.ID)
		return nil
	},
}

func editMounts(r *bufio.Reader, ask func(string, string) string, askInt func(string, int) int, mounts *[]config.VaultMount) {
	for {
		fmt.Println("\nMounts:")
		if len(*mounts) == 0 {
			fmt.Println("  (none)")
		}
		for i, m := range *mounts {
			fmt.Printf("  [%d] %s (KV v%d)\n", i, m.Path, m.Version)
		}
		action := strings.ToLower(ask("Mounts action (a=add, e=edit, r=remove, d=done)", "d"))
		switch action {
		case "a":
			path := ask("Mount path", "secret")
			version := askInt("KV version (1|2)", 2)
			*mounts = append(*mounts, config.VaultMount{Path: path, Version: version})
		case "e":
			idx := askInt("Index to edit", 0)
			if idx < 0 || idx >= len(*mounts) {
				fmt.Println("Invalid index")
				continue
			}
			(*mounts)[idx].Path = ask("Mount path", (*mounts)[idx].Path)
			(*mounts)[idx].Version = askInt("KV version (1|2)", (*mounts)[idx].Version)
		case "r":
			idx := askInt("Index to remove", 0)
			if idx < 0 || idx >= len(*mounts) {
				fmt.Println("Invalid index")
				continue
			}
			*mounts = append((*mounts)[:idx], (*mounts)[idx+1:]...)
		case "d", "done", "":
			return
		default:
			fmt.Println("Unknown action")
		}
	}
}

func init() { sourcesCmd.AddCommand(sourcesEditCmd) }
