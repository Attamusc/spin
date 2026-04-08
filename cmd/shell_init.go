package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/shell"
	"github.com/spf13/cobra"
)

var shellInitCmd = &cobra.Command{
	Use:   "shell-init <shell>",
	Short: "Print shell integration script",
	Long: `Print shell integration code for the given shell.

Add this to your shell config to enable directory navigation, fzf project
picker, and enhanced tab completion.

Zsh:
  eval "$(spin shell-init zsh)"

The integration provides:
  - Shell wrapper that cd's into project directories on 'spin go' / 'spin new'
  - Ctrl+Space fzf widget for interactive project selection
  - Full tab completion for all spin commands, flags, and project names`,
	ValidArgs: []string{"zsh"},
	Args:      cobra.ExactArgs(1),
	RunE:      runShellInit,
}

func init() {
	rootCmd.AddCommand(shellInitCmd)
}

func runShellInit(cmd *cobra.Command, args []string) error {
	shellName := args[0]

	switch shellName {
	case "zsh":
		// Find spin binary path for the template
		spinPath, err := os.Executable()
		if err != nil {
			// Fall back to "spin" (rely on PATH)
			spinPath = "spin"
		}

		output, err := shell.GenerateZsh(shell.ZshConfig{
			SpinPath: spinPath,
		})
		if err != nil {
			return fmt.Errorf("failed to generate zsh init: %w", err)
		}

		fmt.Print(output)

		return nil

	default:
		return fmt.Errorf("unsupported shell %q. Supported: zsh", shellName)
	}
}
