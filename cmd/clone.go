package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/vcs"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repo>",
	Short: "Clone a git repo into the projects directory",
	Long: `Clone a git repo into the correct projects_root/host/owner/repo path.

Repo formats:
  owner/repo          Clone from default host (github.com)
  repo                Clone from default host using default owner
  https://github.com/owner/repo  Full URL

Uses jj (preferred) or git for cloning. The owner and repo names
are lowercased for consistency.`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

func init() {
	cloneCmd.Flags().String("host", "", "git host (default: from config or github.com)")
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	hostFlag, _ := cmd.Flags().GetString("host")

	host, owner, repo, err := parseRepoArg(args[0], globalCfg.DefaultHost, globalCfg.DefaultOwner, hostFlag)
	if err != nil {
		return err
	}

	// Case-normalize
	owner = strings.ToLower(owner)
	repo = strings.ToLower(repo)

	targetDir := filepath.Join(globalCfg.ProjectsRoot, host, owner, repo)

	// Check if target already exists
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		return fmt.Errorf("directory already exists: %s", targetDir)
	}

	// Construct clone URL
	cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Fprintf(os.Stderr, "Cloning %s into %s...\n", cyan(owner+"/"+repo), targetDir)

	if err := vcs.Clone(cloneURL, targetDir); err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	color.New(color.FgGreen).Fprintf(os.Stderr, "Cloned %s/%s\n", owner, repo)

	// Print path to stdout (for shell integration)
	fmt.Println(targetDir)

	return nil
}

// parseRepoArg parses a repo argument into host, owner, repo components.
// Supports: "owner/repo", "repo" (with default owner), full URL.
func parseRepoArg(arg, defaultHost, defaultOwner, hostFlag string) (host, owner, repo string, err error) {
	// Try as full URL first
	if strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") {
		u, parseErr := url.Parse(arg)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("invalid URL %q: %w", arg, parseErr)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return "", "", "", fmt.Errorf("URL %q must contain owner/repo", arg)
		}
		host = u.Host
		owner = parts[0]
		repo = strings.TrimSuffix(parts[1], ".git")
		return host, owner, repo, nil
	}

	// Determine host
	host = defaultHost
	if hostFlag != "" {
		host = hostFlag
	}

	// owner/repo format
	if strings.Contains(arg, "/") {
		parts := strings.SplitN(arg, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", "", fmt.Errorf("invalid repo format %q, expected owner/repo", arg)
		}
		return host, parts[0], strings.TrimSuffix(parts[1], ".git"), nil
	}

	// Short name (just repo) — requires default owner
	if defaultOwner == "" {
		return "", "", "", fmt.Errorf("cannot resolve short name %q: set default_owner in config", arg)
	}

	return host, defaultOwner, arg, nil
}
