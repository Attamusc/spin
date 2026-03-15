package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/template"
	"github.com/attamusc/spin/internal/vcs"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project directory",
	Long: `Create a new project directory with VCS initialization.

Name formats:
  repo          Create under default_host/default_owner/repo
  owner/repo    Create under default_host/owner/repo

Options:
  --template    Apply a named template from config [templates] section

If no template is specified, a minimal directory structure is created
(docs/plans/). The project is initialized as a jj+git colocated repo.`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().StringP("template", "t", "", "template name from config")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	tmplName, _ := cmd.Flags().GetString("template")

	name := args[0]
	var owner, repo string

	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		if parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid name %q, expected owner/repo", name)
		}
		owner = strings.ToLower(parts[0])
		repo = strings.ToLower(parts[1])
	} else {
		if globalCfg.DefaultOwner == "" {
			return fmt.Errorf("cannot create project with short name %q: set default_owner in config", name)
		}
		owner = globalCfg.DefaultOwner
		repo = strings.ToLower(name)
	}

	targetDir := filepath.Join(globalCfg.ProjectsRoot, globalCfg.DefaultHost, owner, repo)

	// Check if already exists
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		return fmt.Errorf("directory already exists: %s", targetDir)
	}

	// Create directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Apply template if specified
	if tmplName != "" {
		tmplPath, ok := globalCfg.Templates[tmplName]
		if !ok {
			// Clean up the created directory on error
			os.RemoveAll(targetDir)
			var available []string
			for k := range globalCfg.Templates {
				available = append(available, k)
			}
			if len(available) > 0 {
				return fmt.Errorf("template %q not found. Available: %s", tmplName, strings.Join(available, ", "))
			}
			return fmt.Errorf("template %q not found. No templates configured", tmplName)
		}
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			os.RemoveAll(targetDir)
			return fmt.Errorf("template directory does not exist: %s", tmplPath)
		}
		if err := template.Apply(tmplPath, targetDir); err != nil {
			os.RemoveAll(targetDir)
			return fmt.Errorf("failed to apply template %q: %w", tmplName, err)
		}
		fmt.Fprintf(os.Stderr, "Applied template %q\n", tmplName)
	} else {
		// Create minimal scaffolding
		plansDir := filepath.Join(targetDir, "docs", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			return fmt.Errorf("failed to create scaffolding: %w", err)
		}
	}

	// Init VCS
	if err := vcs.Init(targetDir); err != nil {
		color.New(color.FgYellow).Fprintf(os.Stderr, "Warning: VCS init failed: %v\n", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	color.New(color.FgGreen).Fprintf(os.Stderr, "Created %s\n", cyan(owner+"/"+repo))

	// Print path to stdout for shell wrapper
	fmt.Println(targetDir)

	return nil
}
