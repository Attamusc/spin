# Plan: Consolidating pm into spin

## 1. Summary

`spin` is a Go CLI for running local dev projects with profile-isolated SQLite databases. `pm` is a separate Node.js + zsh tool for project navigation, cloning, and creation. This plan consolidates all `pm` functionality into `spin`, eliminating the separate tool entirely.

The core additions are: shell-integrated project navigation (`spin go`), git repo cloning with auto-clone-on-navigate, new project scaffolding from templates, broad project discovery (not just `.spin.toml` projects), bulk repo updates, and a shell integration layer (`spin shell-init`) that enables `cd`-on-navigate and fzf-powered completions.

## 2. Naming Collision Resolution

### Problem

`spin clone` (`cmd/clone.go`) currently copies a SQLite database between dev/live profiles. We also need a command for git-cloning repos from GitHub into the projects root. These collide.

### Options considered

| Option | Git clone command | DB copy command | Tradeoffs |
|---|---|---|---|
| A. Subcommand namespacing | `spin clone <repo>` | `spin db clone <src> <dst>` | Groups DB ops, but `spin db clone` is verbose for a rare operation |
| B. Rename DB clone | `spin clone <repo>` | `spin db copy` or `spin db cp` | "copy" is more accurate for what it does anyway; `clone` is universally understood as git clone |
| C. Add `--repo` flag | `spin clone --repo <url>` | `spin clone dev live` | Ambiguous, poor discoverability |
| D. New verb for git | `spin get <repo>` | `spin clone dev live` | "get" is less standard than "clone" |

### Decision: Option B — Rename DB clone, reclaim `clone` for git

**Rationale:**
- `clone` universally means "git clone" to developers. Fighting that expectation is a UX mistake.
- The current `spin clone dev live` is really a database *copy* operation. Calling it `clone` was always slightly misleading — it copies a file, it doesn't create a linked clone.
- DB operations are infrequent and grouping them under `spin db` (with subcommands `copy`, `snapshot`, `restore`, `reset`) creates a cleaner command surface.
- This is a breaking change, but spin is pre-1.0 with a single user.

**Migration path:**
- Move `clone.go` to `db_copy.go`, register under `spin db copy`.
- Also move `snapshot`, `restore`, `reset` under `spin db` for consistency.
- Create new `spin clone` for git repo cloning.

## 3. New and Modified Commands

### 3.1 `spin go [project]` — Navigate to a project

**Purpose:** Replace `pm <owner/repo>`. Navigate to any project directory, auto-cloning from GitHub if it doesn't exist locally.

**Arguments:**
- `[project]` — optional. Formats: `owner/repo`, `repo` (short name), or empty (interactive picker via fzf).

**Behavior:**
1. If `project` is provided, resolve it against all known projects (not just `.spin.toml` projects — uses new `discovery.ScanAll`).
2. If no local match and `project` is `owner/repo`, prompt to clone from `github.com` (using `default_host` from config). If the input is a short name, use `default_owner` from config to construct `default_owner/repo`.
3. On resolution or clone, print the absolute project path to **stdout** (last line). The shell wrapper intercepts this and `cd`s into it.
4. If no argument, launch fzf with all projects as candidates. Selected project path goes to stdout.

**Why `go` and not `cd` or `jump`:**
- `cd` implies it *is* the builtin, which would be confusing.
- `go` is short, imperative, memorable. `spin go mindtrail` reads naturally.
- Two characters. Easy to type.

**Files:**
- Create: `cmd/go.go`
- Modify: `cmd/root.go` (add to help output)

### 3.2 `spin clone <repo> [--host HOST]` — Clone a git repo

**Purpose:** Clone a GitHub repo into the correct `projects_root/host/owner/repo` path.

**Arguments:**
- `<repo>` — required. Formats: `owner/repo`, `repo` (uses `default_owner`), or full URL.
- `--host` — optional, defaults to `github.com`.

**Behavior:**
1. Parse the repo argument, apply defaults.
2. Compute target: `{projects_root}/{host}/{owner}/{repo}`.
3. If target exists, error with message.
4. Run `jj git clone --colocate <url> <target>` (respects spin's jj-first VCS choice from AGENTS.md).
5. Print the target path to stderr for confirmation.
6. Case-normalize the owner and repo names (lowercase) to match pm behavior.

**Files:**
- Create: `cmd/clone.go` (replaces the current file entirely)

### 3.3 `spin new <name> [--template TMPL]` — Create a new project

**Purpose:** Replace `pm new`. Create a new project directory with VCS init and optional template scaffolding.

**Arguments:**
- `<name>` — required. The repo name (placed under `default_owner`), or `owner/repo`.
- `--template` — optional template name from config's `templates` map.

**Behavior:**
1. Compute target: `{projects_root}/{default_host}/{owner}/{name}`.
2. `mkdir -p` the target.
3. If `--template` specified, copy template directory contents into the target.
4. Otherwise, create minimal scaffolding: `docs/plans/` directory.
5. Run `jj git init --colocate` in the target directory.
6. Print target path to stderr.

**Files:**
- Create: `cmd/new.go`

### 3.4 `spin list` — Broaden to show ALL projects

**Purpose:** Replace `pm list`. Show all projects under `projects_root`, not just `.spin.toml` projects.

**Arguments:**
- `--configured` / `-c` — flag to filter to only `.spin.toml` projects (current behavior).
- `--format plain` — output format for scripting (one path per line, used by completions).

**Behavior (modified):**
1. Default: call `discovery.ScanAll` (new), showing every `host/owner/repo` directory.
2. With `--configured`: call `discovery.Scan` (existing), showing only `.spin.toml` projects.
3. Display table includes a marker column showing whether the project has `.spin.toml`.

**Files:**
- Modify: `cmd/list.go`

### 3.5 `spin update [project]` — Pull latest for repos

**Purpose:** Replace `pm update`. Fetch latest for all or one project.

**Arguments:**
- `[project]` — optional. If omitted, updates all projects.

**Behavior:**
1. For each project directory, run `jj git fetch` (jj equivalent of pull/fetch).
2. Report results: updated, already up-to-date, failed (with reason).
3. Skip directories that are not jj/git repos.

**Files:**
- Create: `cmd/update.go`

### 3.6 `spin db <subcommand>` — Database operations group

**Purpose:** Reorganize existing DB commands under a parent.

**Subcommands:**
- `spin db copy <src-profile> <dst-profile>` — current `spin clone` behavior, moved here.
- `spin db snapshot <profile> <name>` — current `spin snapshot`.
- `spin db restore <profile> <name>` — current `spin restore`.
- `spin db reset <profile>` — current `spin reset`.

**Files:**
- Create: `cmd/db.go` (parent command)
- Move + rename: `cmd/clone.go` -> `cmd/db_copy.go`
- Move: `cmd/snapshot.go` -> `cmd/db_snapshot.go`
- Move: `cmd/restore.go` -> `cmd/db_restore.go`
- Move: `cmd/reset.go` -> `cmd/db_reset.go`

### 3.7 `spin shell-init <shell>` — Shell integration

**Purpose:** Emit shell code that enables `cd`-on-navigate, fzf widget, and completion cache.

**Arguments:**
- `<shell>` — required. Only `zsh` initially.

**Behavior:**
- Outputs a shell script to stdout. User adds `eval "$(spin shell-init zsh)"` to `.zshrc`.
- The emitted script defines:
  - `spin()` shell function wrapper that intercepts `spin go` output (last stdout line = path, `cd` into it).
  - `_spin_fzf_widget` — ZLE widget bound to Ctrl+Space that runs `spin list --format plain | fzf` and navigates.
  - Completion integration for `spin go` using cached project list.

**Files:**
- Create: `cmd/shell_init.go`
- Create: `internal/shell/zsh.go` (template for the zsh script)

### 3.8 Root command updates

- Add `spin go`, `spin new`, `spin update`, `spin db`, `spin shell-init` to the help output in `cmd/root.go`.
- Update `completeProjectNames` to use `discovery.ScanAll` for broader tab completion on navigation commands.

## 4. Config Changes

### `~/.config/spin/config.toml`

```toml
# Existing
projects_root = "~/projects"

# New fields
default_host = "github.com"       # default git host for cloning/new
default_owner = "attamusc"        # default owner for short-name resolution

[templates]
  # Named template directories for `spin new --template`
  default = "~/dotfiles/templates/project"
  tauri = "~/dotfiles/templates/tauri-app"
```

### `GlobalConfig` struct changes (`internal/config/global.go`)

```go
type GlobalConfig struct {
    ProjectsRoot string            `toml:"projects_root"`
    DefaultHost  string            `toml:"default_host"`   // new
    DefaultOwner string            `toml:"default_owner"`  // new
    Templates    map[string]string `toml:"templates"`      // new
    DataRoot     string            // computed
}
```

Defaults: `DefaultHost = "github.com"`, `DefaultOwner = ""` (empty means no short-name-to-clone fallback), `Templates = nil`.

## 5. Internal Package Changes

### 5.1 `internal/discovery/scan.go` — Add `ScanAll`

Currently `Scan` only finds `.spin.toml` projects. Add `ScanAll` that finds every directory at the `host/owner/repo` depth.

```go
// ScanAll finds all directories at depth 3 (host/owner/repo) under projectsRoot,
// regardless of whether they have a .spin.toml.
func ScanAll(projectsRoot string) ([]DiscoveredProject, error)
```

Also add a `HasConfig` bool field to `DiscoveredProject`:

```go
type DiscoveredProject struct {
    Host      string
    Owner     string
    Repo      string
    Name      string // "owner/repo"
    Dir       string // absolute path
    HasConfig bool   // true if .spin.toml exists
}
```

`ScanAll` walks to depth 3 using `os.ReadDir` at each level (not `filepath.WalkDir` — we don't need to go deeper), checks for `.spin.toml` existence to set `HasConfig`, and returns the list.

Update `Scan` to call `ScanAll` and filter to `HasConfig == true`, so the logic is shared.

### 5.2 `internal/resolve/resolve.go` — Support config-optional resolution

Add `ResolveAny` that works like `Resolve` but does not require `.spin.toml`. This is used by `spin go` and other navigation commands.

```go
// ResolvedDir holds project directory info without requiring .spin.toml.
type ResolvedDir struct {
    Host  string
    Owner string
    Repo  string
    Name  string
    Dir   string
}

// ResolveAny resolves a project directory by name without requiring .spin.toml.
// Falls back to default_owner for bare repo names.
func ResolveAny(arg, projectsRoot, defaultOwner string) (*ResolvedDir, error)
```

Internal changes:
- Add `resolveOwnerRepoAny` — like `resolveOwnerRepo` but checks directory existence instead of `.spin.toml` existence.
- Add `resolveShortNameAny` — uses `discovery.ScanAll` instead of `discovery.Scan`.
- Existing `Resolve` / `ResolvedProject` remain unchanged for commands that need `.spin.toml`.

### 5.3 `internal/vcs/vcs.go` — New package for VCS operations

Encapsulate git/jj operations used by `clone`, `new`, and `update`.

```go
package vcs

// Clone clones a git repo using jj into the target directory.
func Clone(url, targetDir string) error

// Init initializes a new jj repo (colocated with git) in the target directory.
func Init(dir string) error

// Fetch runs jj git fetch in the given directory.
func Fetch(dir string) error

// IsRepo returns true if the directory contains a .jj or .git directory.
func IsRepo(dir string) bool
```

All functions exec `jj` as a subprocess. If `jj` is not found, fall back to `git` equivalents.

### 5.4 `internal/shell/zsh.go` — New package for shell script generation

Contains the zsh script as an embedded Go template string. The template receives:
- The path to the `spin` binary (for completions).
- Whether fzf is available (detected at init time or assumed).

### 5.5 `internal/template/template.go` — New package for project templates

```go
package template

// Apply copies the contents of srcDir into dstDir, skipping .git/.jj.
func Apply(srcDir, dstDir string) error
```

Simple recursive copy. No variable substitution — just directory templates.

## 6. Shell Integration Design

### Emitted zsh script (via `spin shell-init zsh`)

```zsh
# spin shell integration
spin() {
  local output
  output=$(command spin "$@")
  local exit_code=$?

  if [[ $exit_code -eq 0 && ( "$1" == "go" || "$1" == "new" ) ]]; then
    local last_line="${output##*$'\n'}"
    if [[ -d "$last_line" ]]; then
      builtin cd "$last_line"
      # Print any preceding output (minus the last line) to stderr
      local preceding="${output%$'\n'*}"
      [[ -n "$preceding" && "$preceding" != "$last_line" ]] && echo "$preceding" >&2
      return 0
    fi
  fi

  # Default: just print everything
  [[ -n "$output" ]] && echo "$output"
  return $exit_code
}

# Ctrl+Space fzf widget
_spin_fzf_widget() {
  local selected
  selected=$(command spin list --format plain | fzf --height=40% --reverse --prompt="project> ")
  if [[ -n "$selected" ]]; then
    BUFFER="spin go $selected"
    zle accept-line
  fi
  zle reset-prompt
}
zle -N _spin_fzf_widget
bindkey '^ ' _spin_fzf_widget
```

### Protocol

Commands that want to trigger `cd` in the parent shell (`go`, `new`) must:
1. Print informational output to **stderr**.
2. Print the target directory path as the **last line of stdout**.
3. The shell wrapper detects this and `cd`s.

Commands that don't participate (`dev`, `up`, `list`, etc.) just work normally — their stdout passes through.

### Completion cache

The fzf widget calls `spin list --format plain` directly. For stale-while-revalidate:
- The shell wrapper can cache the output to a temp file with a timestamp.
- On widget invocation: if cache is <30min old, use it and trigger background refresh. Otherwise, call synchronously.
- This is an enhancement for phase 3, not blocking.

## 7. Phased Implementation Order

### Phase 1: Foundation (no breaking changes) — COMPLETE

1. ~~**`internal/discovery/scan.go`** — Add `HasConfig` field to `DiscoveredProject`, add `ScanAll` function, update `Scan` to use it internally.~~ ✓
2. ~~**`internal/config/global.go`** — Add `DefaultHost`, `DefaultOwner`, `Templates` fields with defaults.~~ ✓
3. ~~**`internal/resolve/resolve.go`** — Add `ResolvedDir` type and `ResolveAny` function (additive, existing API untouched).~~ ✓
4. ~~**`internal/vcs/vcs.go`** — New package with `Clone`, `Init`, `Fetch`, `IsRepo`.~~ ✓
5. ~~**Tests for all of the above.**~~ ✓

### Phase 2: New commands — COMPLETE

6. ~~**`cmd/clone.go`** — New git clone command (temporarily named `git-clone` or registered alongside old clone to avoid collision during development).~~ ✓
7. ~~**`cmd/go.go`** — Navigate command.~~ ✓
8. ~~**`cmd/new.go`** — Project creation command.~~ ✓
9. ~~**`cmd/list.go`** — Modify to use `ScanAll`, add `--configured` and `--format` flags.~~ ✓
10. ~~**`cmd/update.go`** — Bulk fetch command.~~ ✓

### Phase 3: Shell integration — COMPLETE

11. ~~**`internal/shell/zsh.go`** — Zsh script template.~~ ✓
12. ~~**`cmd/shell_init.go`** — Shell init command.~~ ✓
13. ~~**Manual testing** of the full shell integration flow.~~ ✓

### Phase 4: Breaking changes (DB command reorg) — COMPLETE

14. ~~**`cmd/db.go`** — Parent command.~~ ✓
15. ~~**Move** `clone.go` -> `db_copy.go`, `snapshot.go` -> `db_snapshot.go`, `restore.go` -> `db_restore.go`, `reset.go` -> `db_reset.go`.~~ ✓
16. ~~**`cmd/clone.go`** — Replace with git clone (remove temporary naming from step 6).~~ ✓
17. ~~**`cmd/root.go`** — Update help text.~~ ✓

### Phase 5: Polish — COMPLETE

18. ~~**`internal/template/template.go`** — Template application.~~ ✓ Done (5.1)
19. ~~**Completion cache** with stale-while-revalidate in shell init.~~ ✓ Done (5.3)
20. ~~**`cmd/root.go`** — Update `completeProjectNames` to use `ScanAll` for broader tab completion on navigation commands.~~ ✓ Done (5.4)

## 8. Testing Strategy

### Unit tests (all in `_test.go` files per convention)

| Package | What to test |
|---|---|
| `internal/discovery` | `ScanAll` finds all dirs at depth 3; `HasConfig` correctly set; skips hidden dirs; empty root returns empty; respects `hiddenDirs` |
| `internal/resolve` | `ResolveAny` with owner/repo, short name, ambiguous, not-found; `DefaultOwner` fallback; case normalization |
| `internal/config` | New fields parse from TOML; defaults applied when absent; `Templates` map parsed; `~` expansion on template paths |
| `internal/vcs` | `IsRepo` detection (create temp dirs with `.jj`/`.git`); `Clone`/`Init`/`Fetch` require integration or mock — test arg construction |
| `internal/shell` | Zsh template renders without syntax errors; contains expected function names; fzf widget present |
| `internal/template` | `Apply` copies files correctly; skips `.git`/`.jj`; handles empty source dir |

### Integration tests

- `spin go attamusc/mindtrail` on a test projects_root — verify stdout is the correct path.
- `spin list` vs `spin list --configured` — verify counts differ when unconfigured projects exist.
- `spin clone` — requires network; gate behind `SPIN_INTEGRATION_TESTS=1` env var, or mock the `jj` command.
- `spin shell-init zsh` — verify output is valid zsh (pipe through `zsh -n` for syntax check).
- `spin new foo` — verify directory structure, `.jj` directory exists.

### Test isolation

- All tests use `t.TempDir()` per existing convention.
- VCS tests that exec `jj`/`git` should check for binary availability and `t.Skip` if absent.
- No shared state between tests.

## 9. Migration Checklist

### Pre-migration (while pm still works)
- [x] Implement phases 1-3 and verify `spin go`, `spin clone`, `spin new`, `spin list` work
- [x] Add `eval "$(spin shell-init zsh)"` to `.zshrc` **after** the existing pm setup
- [x] Test both coexist: `pm` still works, `spin go` also works
- [x] Verify fzf Ctrl+Space widget works with spin

### Migration (big bang)
- [x] Remove pm shell function and completion sources from `.zshrc` / zsh config
- [x] Delete `~/.local/bin/pm`
- [x] Delete `~/.config/zsh/config/pm-shell-integration.zsh`
- [x] Delete `~/.config/zsh/completions/pm-completions.zsh`
- [x] Delete `~/.local/share/pm/` cache directory
- [x] Implement phase 4 (DB command reorg) — this is the breaking change
- [x] Update `spin` binary in PATH
- [ ] Verify: `spin go mindtrail` navigates correctly
- [ ] Verify: `spin clone attamusc/newrepo` clones to correct path
- [ ] Verify: `spin new myproject` scaffolds correctly
- [ ] Verify: `spin list` shows all projects (not just configured)
- [ ] Verify: `spin db copy dev live` works (replacing old `spin clone dev live`)
- [ ] Verify: `spin dev`, `spin up`, `spin down` still work (regression check)
- [ ] Verify: tab completion works for all new commands
- [ ] Verify: Ctrl+Space fzf widget works

### Post-migration cleanup
- [x] Update AGENTS.md with new commands
- [x] Update root command help text to reflect full command set
