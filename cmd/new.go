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
	Use: "new <name>", Short: "Create a new project directory",
	Long: `Create owner/repo (or repo under default_owner), optionally applying a
named --template, then initialize a jj/git repository. Without a template,
create docs/plans/. Print the project path for shell navigation.`,
	Args: cobra.ExactArgs(1), RunE: runNew,
}

func init() {
	newCmd.Flags().StringP("template", "t", "", "template name from config")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) (err error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if strings.Contains(args[0], "://") {
		return fmt.Errorf("expected repo or owner/repo, not a URL")
	}
	host, owner, repo, err := parseRepoArg(args[0], cfg.DefaultHost, cfg.DefaultOwner, "")
	if err != nil {
		return err
	}
	owner, repo = strings.ToLower(owner), strings.ToLower(repo)
	target, err := config.ProjectPath(cfg.ProjectsRoot, host, owner, repo)
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("template")
	source := cfg.Templates[name]
	if name != "" {
		if info, e := os.Stat(source); e != nil || !info.IsDir() {
			return fmt.Errorf("template %q is not a configured directory", name)
		}
	}
	root, err := os.OpenRoot(cfg.ProjectsRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	rel := filepath.Join(host, owner, repo)
	if err := root.MkdirAll(filepath.Dir(rel), 0755); err != nil {
		return err
	}
	if err := root.Mkdir(rel, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cleanup := root.RemoveAll(rel); cleanup != nil {
				err = fmt.Errorf("%w (cleanup failed: %v)", err, cleanup)
			}
		}
	}()
	if name != "" {
		err = template.Apply(source, target)
	} else {
		err = root.MkdirAll(filepath.Join(rel, "docs", "plans"), 0755)
	}
	if err != nil {
		return err
	}
	if initErr := vcs.Init(target); initErr != nil {
		color.New(color.FgYellow).Fprintf(cmd.ErrOrStderr(), "Warning: VCS init failed: %v\n", initErr)
	}
	color.New(color.FgGreen).Fprintf(cmd.ErrOrStderr(), "Created %s/%s\n", owner, repo)
	_, err = fmt.Fprintln(cmd.OutOrStdout(), target)
	return err
}
