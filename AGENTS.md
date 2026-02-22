# spin — Local Project Runner

## Project Overview

`spin` is a Go CLI tool that runs local dev scripts in two modes: foregrounded dev mode and backgrounded live mode, each with a separate SQLite database for data isolation. It targets macOS.

## Project Structure

```
spin/
  main.go                    # Entry point — calls cmd.Execute()
  cmd/                       # Cobra command definitions (one file per subcommand)
    root.go                  # Root command, runRoot dashboard, completeProjectNames helper
    dev.go                   # spin dev — foreground process
    up.go                    # spin up — background process
    down.go                  # spin down — stop background process
    status.go                # spin status — running instances table + shared helpers
    logs.go                  # spin logs — tail live log file
    list.go                  # spin list — discover projects
    init_cmd.go              # spin init — generate .spin.toml
    open.go                  # spin open — open project in browser
    snapshot.go              # spin snapshot
    restore.go               # spin restore
    clone.go                 # spin clone
    reset.go                 # spin reset
    completion.go            # spin completion — shell completions
  internal/                  # Internal packages (each has _test.go files)
    config/                  # Global + project config loading
      global.go              # ~/.config/spin/config.toml parsing
      project.go             # .spin.toml parsing, template resolution
    resolve/                 # Project name resolution (CWD, owner/repo, short name)
      resolve.go
    process/                 # Process management
      foreground.go          # Foreground exec with signal forwarding
      background.go          # Detached background process with log redirection
      pidfile.go             # PID file read/write/validate/clean
      util.go                # sleepMs helper (mockable in tests)
    discovery/               # Project scanning
      scan.go                # Recursive .spin.toml discovery
    db/                      # Database utilities
      snapshot.go            # WAL checkpoint, copy, remove, snapshot helpers
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
- PID files are validated with `syscall.Kill(pid, 0)` and stale ones are cleaned up automatically.
- The `{profile_db_path}` template variable in `.spin.toml` env values is the only supported template.
- Projects are discovered by scanning `projects_root` for `.spin.toml` files in `host/owner/repo` directory structure.

## Version Control

This project uses **Jujutsu (`jj`)** colocated with git. Use `jj` commands, not `git`:

- `jj status` / `jj diff` / `jj log` for inspection
- `jj commit -m "message"` to finalize and start a new change
- `jj describe -m "message"` to update the current change's description without finalizing
