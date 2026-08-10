package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateZsh_DefaultSpinCmd(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	// Should use "spin" when no SpinPath given
	if !strings.Contains(output, "command spin ") {
		t.Error("expected 'command spin' in output when SpinPath is empty")
	}
}

func TestGenerateZsh_CustomSpinPath(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{SpinPath: "/usr/local/bin/spin"})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	if !strings.Contains(output, "command /usr/local/bin/spin ") {
		t.Error("expected custom spin path in output")
	}

	// Should NOT contain bare "command spin " (without the path)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue // skip comments
		}
		if strings.Contains(trimmed, "command spin ") && !strings.Contains(trimmed, "command /usr/local/bin/spin") {
			t.Errorf("found bare 'command spin' instead of custom path in: %s", trimmed)
		}
	}
}

func TestGenerateZsh_ContainsShellWrapper(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	required := []string{
		"spin()",
		"builtin cd",
		`case "$1" in`,
		"go|new|clone|git-clone)",
	}
	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("expected shell wrapper to contain %q", s)
		}
	}
}

func TestGenerateZsh_ContainsFzfWidget(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	required := []string{
		"_spin_fzf_widget()",
		"zle -N _spin_fzf_widget",
		`bindkey '^ ' _spin_fzf_widget`,
		"fzf",
		"--format plain",
	}
	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("expected fzf widget to contain %q", s)
		}
	}
}

func TestGenerateZsh_ContainsCompletion(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	// Should contain the project completion helper (used by fzf widget)
	if !strings.Contains(output, "_spin_complete_projects()") {
		t.Error("expected _spin_complete_projects() in output")
	}

	// Should NOT contain the old compdef registration (cobra handles this now)
	if strings.Contains(output, "_spin_go_completion") {
		t.Error("unexpected _spin_go_completion — cobra completion handles subcommand routing now")
	}
}

func TestGenerateZsh_ContainsHeader(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	if !strings.HasPrefix(output, "# spin shell integration") {
		t.Error("expected output to start with header comment")
	}

	if !strings.Contains(output, `eval "$(spin shell-init zsh)"`) {
		t.Error("expected eval instruction in header")
	}
}

func TestGenerateZsh_PassthroughCommands(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	// These commands should pass through without cd interception
	passthrough := []string{"help", "completion", "shell-init"}
	for _, cmd := range passthrough {
		if !strings.Contains(output, cmd) {
			t.Errorf("expected passthrough list to contain %q", cmd)
		}
	}
}

func TestGenerateZsh_ContainsCacheFunctions(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	required := []string{
		"_SPIN_CACHE_TTL",
		"_spin_cache_file()",
		"_spin_cache_age()",
		"_spin_refresh_cache()",
		"_spin_refresh_cache_async()",
		"_spin_cached_projects()",
		"projects.cache",
	}
	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("expected cache code to contain %q", s)
		}
	}
}

func TestGenerateZsh_CacheInvalidation(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	// The shell wrapper should invalidate cache on directory-changing ops
	if !strings.Contains(output, `rm -f "$cache_file"`) {
		t.Error("expected cache invalidation (rm -f) in shell wrapper")
	}
}

func TestGenerateZsh_CacheAgeSupportsBSDAndGNUStat(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	for _, tc := range []struct {
		name       string
		statScript string
	}{
		{
			name: "BSD stat",
			statScript: `#!/bin/sh
if [ "$1" = "-f" ] && [ "$2" = "%m" ]; then
  printf '1234\n'
  exit 0
fi
printf 'unsupported stat form\n' >&2
exit 1
`,
		},
		{
			name: "GNU stat",
			statScript: `#!/bin/sh
if [ "$1" = "-c" ] && [ "$2" = "%Y" ]; then
  printf '1234\n'
  exit 0
fi
printf 'unsupported stat form\n' >&2
exit 1
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runCacheAge(t, output, tc.statScript, true)
			if got != "766\n" {
				t.Fatalf("_spin_cache_age output = %q, want one numeric value %q", got, "766\n")
			}
		})
	}
}

func TestGenerateZsh_CacheAgeWithoutCachePreservesSentinel(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	got := runCacheAge(t, output, "#!/bin/sh\nexit 1\n", false)
	if got != "999999\n" {
		t.Fatalf("_spin_cache_age output = %q, want %q", got, "999999\n")
	}
}

func TestGenerateZsh_CacheAgeIsNumericWhenStatFails(t *testing.T) {
	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	got := runCacheAge(t, output, "#!/bin/sh\nprintf 'stat failed\\n' >&2\nexit 1\n", true)
	if got != "2000\n" {
		t.Fatalf("_spin_cache_age output = %q, want one numeric value %q", got, "2000\n")
	}
}

func TestGenerateZsh_Syntax(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not found in PATH")
	}

	output, err := GenerateZsh(ZshConfig{})
	if err != nil {
		t.Fatalf("GenerateZsh() error: %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "spin.zsh")
	if err := os.WriteFile(scriptPath, []byte(output), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(zshPath, "-n", scriptPath)
	if syntaxOutput, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh -n failed: %v\n%s", err, syntaxOutput)
	}
}

func runCacheAge(t *testing.T, generated, statScript string, cacheExists bool) string {
	t.Helper()

	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not found in PATH")
	}

	start := strings.Index(generated, "_spin_cache_file()")
	end := strings.Index(generated, "_spin_refresh_cache()")
	if start == -1 || end == -1 || end <= start {
		t.Fatal("generated shell does not contain the expected cache functions")
	}

	cacheRoot := t.TempDir()
	if cacheExists {
		cacheDir := filepath.Join(cacheRoot, "spin")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cacheDir, "projects.cache"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	binDir := t.TempDir()
	for name, contents := range map[string]string{
		"stat": statScript,
		"date": "#!/bin/sh\nprintf '2000\\n'\n",
	} {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
			t.Fatal(err)
		}
	}

	script := fmt.Sprintf("%s\n_spin_cache_age\n", generated[start:end])
	cmd := exec.Command(zshPath, "-f", "-c", script)
	cmd.Env = []string{
		"HOME=" + cacheRoot,
		"PATH=" + binDir,
		"XDG_CACHE_HOME=" + cacheRoot,
	}
	combined, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running _spin_cache_age failed: %v\n%s", err, combined)
	}
	return string(combined)
}
