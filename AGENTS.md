# spin — Local Project Runner & Manager

## Project Overview

`spin` is a Go CLI tool for managing local development projects. It handles project navigation, git cloning, project creation from templates, bulk repo updates, process management (foreground dev mode / background live mode with separate SQLite databases), and shell integration with fzf-powered project picking. It supports macOS (arm64 and amd64) and Fedora 44 (x86_64).

## Project Structure

```
spin/
  main.go                    # Entry point — calls cmd.Execute()
  cmd/                       # Cobra command definitions (one file per subcommand)
    root.go                  # Root command, runRoot dashboard, completeProjectNames/completeAllProjectNames
    go.go                    # spin go — navigate to project directory (prints path to stdout)
    clone.go                 # spin clone — git clone repo into projects_root
    new.go                   # spin new — create project from template, init VCS
    list.go                  # spin list — discover all projects (--configured, --format plain, alias: ls)
    update.go                # spin update — bulk fetch all repos (parallel, 8 goroutines)
    dev.go                   # spin dev — foreground process
    up.go                    # spin up — background process
    down.go                  # spin down — stop background process
    status.go                # spin status — running instances table + shared helpers
    logs.go                  # spin logs — tail live log file
    init_cmd.go              # spin init — generate .spin.toml
    open.go                  # spin open — open project in browser
    db.go                    # spin db — parent command for database operations
    db_copy.go               # spin db copy — copy database between profiles
    db_snapshot.go           # spin db snapshot — snapshot a profile database
    db_restore.go            # spin db restore — restore a database snapshot
    db_reset.go              # spin db reset — reset a profile database
    shell_init.go            # spin shell-init — emit shell integration script
    completion.go            # spin completion — shell completions
  internal/                  # Internal packages (each has _test.go files)
    config/                  # Global + project config loading
      global.go              # ~/.config/spin/config.toml parsing (default_host, default_owner, templates)
      project.go             # .spin.toml parsing, template resolution
    resolve/                 # Project name resolution (CWD, owner/repo, short name)
      resolve.go             # Resolve (requires .spin.toml) and ResolveAny (directory-only)
    process/                 # Process management
      foreground.go          # Foreground exec with signal forwarding
      background.go          # Detached background process with log redirection
      pidfile.go             # PID file read/write/validate/clean
      util.go                # sleepMs helper (mockable in tests)
    discovery/               # Project scanning
      scan.go                # Scan (.spin.toml only) and ScanAll (all dirs at depth 3)
    db/                      # Database utilities
      snapshot.go            # WAL checkpoint, copy, remove, snapshot helpers
    vcs/                     # Version control operations (jj-first, git fallback)
      vcs.go                 # Clone, Init, Fetch, IsRepo, IsJJRepo, IsGitRepo
    shell/                   # Shell integration script generation
      zsh.go                 # Zsh template: wrapper, fzf widget, cache, completions
    template/                # Project template application
      template.go            # Recursive copy, skips .git/.jj, preserves permissions
  docs/plans/INITIAL.md      # Original implementation plan (reference only)
```

## Tech Stack

- **Language**: Go 1.25+
- **CLI framework**: `github.com/spf13/cobra`
- **Config parsing**: `github.com/BurntSushi/toml`
- **Color output**: `github.com/fatih/color`
- **Version control**: Jujutsu (`jj`), colocated with git
- **No other runtime dependencies**

## Code Conventions

- All `internal/` packages must have corresponding `_test.go` files with unit tests.
- Tests use `t.TempDir()` for isolation — no shared test state.
- Commands in `cmd/` follow a pattern: cobra command var, `init()` to register, `runX()` function.
- Commands that accept a `[project]` argument use `resolve.Resolve(arg, projectsRoot)` and register `ValidArgsFunction: completeProjectNames`.
- Navigation commands (`go`, `clone`, `new`) use `resolve.ResolveAny` (no `.spin.toml` required) and `completeAllProjectNames`.
- Color scheme: green = success, yellow = warnings, cyan = project names/commands, red = errors. Uses `fatih/color` which respects `NO_COLOR`.
- Output goes to stderr (status messages), stdout is reserved for data.
- Module path: `github.com/attamusc/spin`

## Build & Test

```bash
make build          # Compile to dist/spin
make test           # go test ./... -count=1
make test-race      # Tests with race detector
make lint           # go vet + staticcheck
make fmt            # go fmt ./...
```

## Key Design Decisions

- Background processes use `Setsid: true` for session detachment — do NOT use `fork()` from Go.
- Stopping background processes sends signals to the process group (`-pid`) since `sh -c` may spawn children.
- Background processes use a goroutine with `cmd.Wait()` to reap children — do NOT use `cmd.Process.Release()` (causes zombies in tests).
- Lifecycle commands validate PID files with `syscall.Kill(pid, 0)` and explicitly clean stale files. Status and the dashboard never clean files: their shared read-only collector retains stale/unknown/malformed records and reports uncertainty. `spin status --format json` follows `docs/status-contract.md`; PID existence is not identity/readiness, and uptime is estimated PID-file age.
- The `{profile_db_path}` template variable in `.spin.toml` env values is the only supported template.
- Projects are discovered by scanning `projects_root` at depth 3 (`host/owner/repo`). `ScanAll` finds all directories; `Scan` filters to `.spin.toml` only.
- Shell integration uses `eval "$(spin shell-init zsh)"` pattern. The shell wrapper intercepts `go`, `new`, `clone` commands to `cd` into the path printed on stdout.
- VCS operations prefer `jj` (Jujutsu) and fall back to `git` if jj is not found.
- Commands that produce a path for shell `cd` (`go`, `new`, `clone`) print the path as the last line of stdout. Informational output goes to stderr.
- The completion cache uses stale-while-revalidate: serves from cache file, refreshes async in background if older than 30 minutes.

## Version Control

This project uses **Jujutsu (`jj`)** colocated with git. Use `jj` commands, not `git`:

- `jj status` / `jj diff` / `jj log` for inspection
- `jj commit -m "message"` to finalize and start a new change
- `jj describe -m "message"` to update the current change's description without finalizing
