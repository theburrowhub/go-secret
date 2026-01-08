package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for go-secret.

To load completions:

Bash:
  $ source <(go-secret completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ go-secret completion bash > /etc/bash_completion.d/go-secret
  # macOS:
  $ go-secret completion bash > $(brew --prefix)/etc/bash_completion.d/go-secret

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ go-secret completion zsh > "${fpath[1]}/_go-secret"

  # You may need to start a new shell for this setup to take effect.

Fish:
  $ go-secret completion fish | source

  # To load completions for each session, execute once:
  $ go-secret completion fish > ~/.config/fish/completions/go-secret.fish

PowerShell:
  PS> go-secret completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> go-secret completion powershell > go-secret.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			_ = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			_ = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
