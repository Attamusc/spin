package shell

import (
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
