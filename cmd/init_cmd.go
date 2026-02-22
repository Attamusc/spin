package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new .spin.toml config for the current project",
	Long: `Generate a .spin.toml configuration file in the current directory.
Detects the project type (Next.js, Tauri, Electron, etc.) and writes
appropriate defaults. Non-interactive, best-effort detection.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get working directory: %w", err)
	}

	spinToml := filepath.Join(cwd, ".spin.toml")

	// Check if .spin.toml already exists
	if _, err := os.Stat(spinToml); err == nil {
		fmt.Fprintf(os.Stderr, ".spin.toml already exists in %s\n", cwd)
		return nil
	}

	// Detect project type
	devCommand := detectDevCommand(cwd)

	// Write .spin.toml
	content := generateSpinToml(devCommand)
	if err := os.WriteFile(spinToml, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .spin.toml: %w", err)
	}

	color.New(color.FgGreen).Fprintf(os.Stderr, "Created .spin.toml in %s\n\n", cwd)
	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Fprintf(os.Stderr, "Detected command: %s\n", cyan(devCommand))
	fmt.Fprintf(os.Stderr, "Dev port: 3001, Live port: 3000\n")

	// Check .gitignore for .spin/ entry
	checkGitignore(cwd)

	return nil
}

// detectDevCommand inspects the project directory and returns the best-guess
// dev command.
func detectDevCommand(dir string) string {
	// Check for Tauri
	tauriConf := filepath.Join(dir, "src-tauri", "tauri.conf.json")
	if _, err := os.Stat(tauriConf); err == nil {
		return "npm run tauri dev"
	}

	// Check package.json
	pkgPath := filepath.Join(dir, "package.json")
	pkgData, err := os.ReadFile(pkgPath)
	if err == nil {
		var pkg packageJSON
		if err := json.Unmarshal(pkgData, &pkg); err == nil {
			// Check for electron in devDependencies
			if _, ok := pkg.DevDependencies["electron"]; ok {
				return "npm run dev"
			}
			// Check for scripts.dev
			if _, ok := pkg.Scripts["dev"]; ok {
				return "npm run dev"
			}
		}
	}

	return "# TODO: set your dev command"
}

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func generateSpinToml(devCommand string) string {
	return fmt.Sprintf(`db_filename = "data.db"

[dev]
command = %q
port = 3001
env = { DATABASE_URL = "file:{profile_db_path}" }

[live]
command = %q
port = 3000
env = { DATABASE_URL = "file:{profile_db_path}" }
`, devCommand, devCommand)
}

// checkGitignore checks if .spin/ is in .gitignore and reminds the user if not.
func checkGitignore(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	f, err := os.Open(gitignorePath)
	if err != nil {
		color.New(color.FgYellow).Fprintf(os.Stderr, "\nReminder: add '.spin/' to your .gitignore\n")
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ".spin/" || line == ".spin" {
			return // Already present
		}
	}

	color.New(color.FgYellow).Fprintf(os.Stderr, "\nReminder: add '.spin/' to your .gitignore\n")
}
