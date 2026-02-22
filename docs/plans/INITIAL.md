# spin — Local Project Runner with Data Profile Switching

## Overview

`spin` is a Go CLI tool that runs local dev scripts in two modes: **foregrounded dev mode** and **backgrounded live mode**, each pointing at a separate SQLite database. The key insight is that both modes run the same dev script — the only difference is environment variables (DB path, port, etc.) and whether the process is attached to a terminal.

## Motivation

When working on local projects, you want:

- The fast feedback loop of `npm run dev` (or equivalent) at all times — no separate "production" build
- The ability to have a stable "live" instance running in the background while you hack in the foreground
- Data isolation between the two so dev work never touches your real data

## User's environment

- macOS
- Projects live in `~/projects` using GitHub `host/owner/repo` conventions
  - Example: `~/projects/github.com/attamusc/mindtrail`
- Projects are mostly Next.js, some Electron/Tauri apps
- Persistence is SQLite

---

## Directory structure

### Project config (per-project, committed to repo)

```
~/projects/github.com/attamusc/mindtrail/.spin.toml
```

### Data directory (gitignored, lives alongside projects)

```
~/projects/.spin/
  attamusc/mindtrail/
    dev/
      data.db
    live/
      data.db
    logs/
      live.log
    live.pid
```

### Global config

```
~/.config/spin/config.toml
```

```toml
projects_root = "~/projects"
```

The `data_root` is always `{projects_root}/.spin/`. No other global config is needed initially.

---

## Project config format

File: `.spin.toml` in the project root.

```toml
# The db_filename is used to construct the full path:
#   {data_root}/{owner}/{repo}/{profile}/{db_filename}
# This path is injected as the template variable {profile_db_path}
db_filename = "data.db"

[dev]
command = "npm run dev"
port = 3001
# env values support {profile_db_path} template variable
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3001" }

[live]
command = "npm run dev"
port = 3000
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3000" }
```

### Config rules

- `command` is required for both `[dev]` and `[live]`.
- `port` is optional. Used for status display and conflict detection, not enforced.
- `env` is a flat string-to-string map. Values are passed as environment variables to the child process, merged on top of the current shell environment.
- `{profile_db_path}` is the only template variable. It resolves to the absolute path of the SQLite file for the current profile.
- A `pre_hook` field (optional, string) can specify a shell command to run before the main command. Useful for Electron/Tauri apps that need config files written to disk rather than reading env vars.

---

## CLI commands

### `spin dev [project]`

Run a project in **dev mode, foregrounded**.

1. Resolve the project (see name resolution below).
2. Parse `.spin.toml`, extract `[dev]` section.
3. Resolve `{profile_db_path}` → `{data_root}/{owner}/{repo}/dev/{db_filename}`. Create parent directories if they don't exist.
4. Run `pre_hook` if defined.
5. Exec the `command` in the project directory with the resolved `env` vars merged into the current environment.
6. stdin/stdout/stderr pass through directly — the user interacts with the process normally.
7. Ctrl-C sends SIGINT to the child process (standard behavior).

### `spin up [project]`

Run a project in **live mode, backgrounded**.

1. Resolve the project.
2. Check if a live instance is already running (check PID file, verify process is alive). If so, print a message and exit.
3. Parse `.spin.toml`, extract `[live]` section.
4. Resolve `{profile_db_path}` → `{data_root}/{owner}/{repo}/live/{db_filename}`. Create parent directories if they don't exist.
5. Run `pre_hook` if defined.
6. Start the `command` as a detached child process:
   - Working directory: the project directory.
   - stdout and stderr redirected to `{data_root}/{owner}/{repo}/logs/live.log`.
   - The log file is opened in write mode (truncate on each `spin up` invocation so it doesn't grow forever — a future enhancement could add rotation).
7. Write the child PID to `{data_root}/{owner}/{repo}/live.pid`.
8. Print confirmation: project name, PID, port (if configured).

### `spin down [project]`

Stop a backgrounded live instance.

1. Resolve the project.
2. Read PID from `live.pid`.
3. Send SIGTERM. Wait up to 5 seconds (poll every 100ms with `syscall.Kill(pid, 0)`).
4. If still running, send SIGKILL.
5. Remove the PID file.
6. Print confirmation.

If no PID file exists or the process is already dead, print a message and clean up the stale PID file.

### `spin status`

Show all running instances.

1. Scan `{data_root}` for `live.pid` files (walk `{data_root}/*/*/live.pid` pattern).
2. For each, check if the process is still alive. Clean up stale PID files.
3. For alive processes, load the project's `.spin.toml` to get the port.
4. Calculate uptime from PID file modification time (mtime).
5. Print a table:

```
PROJECT              MODE   PORT   PID     UPTIME
attamusc/mindtrail   live   3000   48231   2h13m
attamusc/otherthing  live   8080   48302   45m
```

If no live instances are running, print "No live instances running."

Note: dev mode instances are foregrounded and transient, so they don't appear in status. Only backgrounded live instances are tracked.

### `spin logs [project]`

Tail the log file for a backgrounded instance.

1. Resolve the project.
2. Construct log path: `{data_root}/{owner}/{repo}/logs/live.log`.
3. Check if log file exists. If not, print a message and exit.
4. Tail the file using a polling approach (read, sleep 100ms, check for new bytes). Display the last ~50 lines of existing content, then follow new writes.
5. Ctrl-C exits the tail, does not affect the running process.

### `spin list`

Show all discoverable projects.

1. Recursively scan `{projects_root}` for directories containing `.spin.toml`. Skip hidden directories (`.git`, `.spin`, `.jj`, etc.).
2. Derive `owner/repo` from the path (strip `{projects_root}/{host}/` prefix).
3. Print:

```
attamusc/mindtrail    ~/projects/github.com/attamusc/mindtrail
attamusc/otherthing   ~/projects/github.com/attamusc/otherthing
```

If no projects found, print a helpful message.

### `spin init`

Generate a `.spin.toml` in the current directory with sensible defaults. Non-interactive — detect what we can, use TODO comments for the rest.

1. Check if `.spin.toml` already exists in CWD. If so, print a message and exit.
2. Detect the project type by inspecting files in the current directory:
   - `package.json` with `scripts.dev` → Node.js project, command = `npm run dev`
   - `src-tauri/tauri.conf.json` → Tauri app, command = `npm run tauri dev`
   - `package.json` with `electron` in `devDependencies` → Electron app, command = `npm run dev`
   - Fallback: `command = "# TODO: set your dev command"`
3. Assign default ports: dev = 3001, live = 3000 (these are just defaults the user will edit).
4. Write `.spin.toml` to the current directory.
5. Print what was generated.
6. Check if `.spin/` is in a `.gitignore` up the tree; if not, remind the user to add it.

### DB utility commands (phase 4)

These are lower priority but part of the design:

- `spin snapshot [project] [profile] [name]` — Copy the DB file to `{data_root}/{owner}/{repo}/snapshots/{name}.db`. Run `sqlite3 {db} "PRAGMA wal_checkpoint(TRUNCATE)"` first if a WAL file exists.
- `spin restore [project] [profile] [name]` — Copy a snapshot back to the profile's DB path. Refuse if the profile's process is running.
- `spin clone [project] [source-profile] [target-profile]` — Copy one profile's DB to another. Same WAL checkpoint safety.
- `spin reset [project] [profile]` — Delete the profile's DB file. The next `spin dev` or `spin up` will start fresh (the app is responsible for creating/migrating the DB on startup).

---

## Name resolution

When the user provides a `[project]` argument, resolve it in this order:

1. **No argument + CWD detection**: If no project argument is given, walk up from the current working directory looking for `.spin.toml`. If found, derive the project identity from the path relative to `projects_root`. Error if CWD is not within `projects_root`.

2. **Full `owner/repo` match**: If the argument contains a `/`, look for `{projects_root}/*/{argument}/.spin.toml` (the `*` matches any host like `github.com`).

3. **Short name match**: If the argument has no `/`, scan all discovered projects and match against the `repo` portion. If exactly one match, use it. If multiple matches, print them and ask the user to be more specific.

4. **No match**: Print an error with suggestions (did you mean...? or run `spin list`).

---

## Process management details

### Backgrounding on macOS

Do NOT use `fork()` directly from Go (it's unreliable with the Go runtime). Instead:

- Use `os/exec` with `Cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}` to create a new session.
- Start the process, capture its PID, then return. The Go CLI process exits; the child continues.
- stdout/stderr of the child are redirected to the log file via `Cmd.Stdout` and `Cmd.Stderr` set to the opened file descriptor before `Start()`.

### PID file lifecycle

- Written immediately after successful `Cmd.Start()`.
- Contains just the PID as a string.
- On `spin down`, `spin status`, and `spin up` (to check for existing), validate the PID is alive with `syscall.Kill(pid, 0)`.
- Clean up stale PID files whenever they're detected (process no longer exists).

### Signal handling in dev mode

When running `spin dev`, the CLI should forward signals to the child process:

- Trap SIGINT and SIGTERM in the Go process.
- Forward them to the child process.
- Wait for the child to exit and propagate its exit code.

This ensures Ctrl-C behaves naturally — the dev server gets the signal, cleans up, and exits.

---

## Technology choices

- **Language**: Go
- **Module path**: `github.com/attamusc/spin`
- **CLI framework**: [cobra](https://github.com/spf13/cobra) (v1.10.2) — well-established, supports subcommands, auto-generates help text, best-in-class shell completions
- **Config parsing**: [BurntSushi/toml](https://github.com/BurntSushi/toml) (v1.6.0) — TOML v1.1.0 spec support, mirrors `encoding/json` API, zero external deps
- **Color output**: [fatih/color](https://github.com/fatih/color) — added in Phase 5 for colorized terminal output, handles `NO_COLOR` and terminal detection
- **No other runtime dependencies**: the binary is self-contained

### Project structure

```
spin/
  cmd/
    root.go          # cobra root command, global flags, config loading
    dev.go           # spin dev
    up.go            # spin up
    down.go          # spin down
    status.go        # spin status
    logs.go          # spin logs
    list.go          # spin list
    init_cmd.go      # spin init (named init_cmd to avoid collision with Go's init())
    snapshot.go      # spin snapshot (phase 4)
    restore.go       # spin restore (phase 4)
    clone.go         # spin clone (phase 4)
    reset.go         # spin reset (phase 4)
  internal/
    config/
      global.go      # global config loading (~/.config/spin/config.toml)
      global_test.go
      project.go     # .spin.toml parsing, validation, template resolution
      project_test.go
    resolve/
      resolve.go     # project name resolution (CWD, owner/repo, short name)
      resolve_test.go
    process/
      foreground.go  # exec with env injection, signal forwarding
      foreground_test.go
      background.go  # detached process start, log redirection
      background_test.go
      pidfile.go     # PID file read/write/validate utilities
      pidfile_test.go
    db/
      snapshot.go    # WAL checkpoint, file copy operations (phase 4)
      snapshot_test.go
    discovery/
      scan.go        # recursive scan for .spin.toml files
      scan_test.go
  main.go            # cobra Execute() entry point
  go.mod
  go.sum
```

---

## Go type definitions

### Global config (`internal/config/global.go`)

```go
type GlobalConfig struct {
    ProjectsRoot string `toml:"projects_root"`
    DataRoot     string // computed: {ProjectsRoot}/.spin/
}
```

- Load from `~/.config/spin/config.toml`
- If file doesn't exist, default `ProjectsRoot` to `~/projects`
- Expand `~` to the user's home directory
- Derive `DataRoot` as `{ProjectsRoot}/.spin/` (computed, not stored in config)
- Return an error if `ProjectsRoot` doesn't exist on disk

### Project config (`internal/config/project.go`)

```go
type ProjectConfig struct {
    DBFilename string        `toml:"db_filename"`
    Dev        ProfileConfig `toml:"dev"`
    Live       ProfileConfig `toml:"live"`
}

type ProfileConfig struct {
    Command string            `toml:"command"`
    Port    int               `toml:"port,omitempty"`
    Env     map[string]string `toml:"env"`
    PreHook string            `toml:"pre_hook,omitempty"`
}
```

### Resolved project (`internal/resolve/resolve.go`)

```go
type ResolvedProject struct {
    Host   string        // e.g., "github.com"
    Owner  string        // e.g., "attamusc"
    Repo   string        // e.g., "mindtrail"
    Name   string        // "owner/repo" for display
    Dir    string        // absolute path to project directory
    Config ProjectConfig
}
```

### PID file utilities (`internal/process/pidfile.go`)

```go
func WritePID(path string, pid int) error
func ReadPID(path string) (int, error)
func IsProcessAlive(pid int) bool        // uses syscall.Kill(pid, 0)
func CleanStalePIDFile(path string) error
func PIDFilePath(dataRoot, owner, repo string) string
```

### Discovery (`internal/discovery/scan.go`)

```go
type DiscoveredProject struct {
    Host  string // e.g., "github.com"
    Owner string // e.g., "attamusc"
    Repo  string // e.g., "mindtrail"
    Name  string // "owner/repo"
    Dir   string // absolute path
}

func Scan(projectsRoot string) ([]DiscoveredProject, error)
```

---

## Build phases

### Phase 1: Foundation + Foreground dev mode

**Goal**: `spin dev` works from a project directory. Immediately useful.

#### Step 1.1: Project initialization

- Create `go.mod` with module `github.com/attamusc/spin`
- `go get github.com/spf13/cobra` and `github.com/BurntSushi/toml`
- Create `main.go` with cobra `Execute()` entry point
- Create `cmd/root.go` with the root command (no-op, prints help)
- Create `.gitignore` (Go binary, IDE files, etc.)
- Verify: `go build` produces a binary, `./spin --help` prints help text

#### Step 1.2: Global config (`internal/config/global.go`)

- Implement `GlobalConfig` struct and `Load() (*GlobalConfig, error)`
- Handle missing config file (default to `~/projects`)
- Expand `~` in paths
- Compute `DataRoot` from `ProjectsRoot`
- Return error if `ProjectsRoot` doesn't exist on disk

Tests (`global_test.go`):
- Default when no config file exists
- Parsing a valid config file
- `~` expansion
- Invalid/missing `projects_root` directory

#### Step 1.3: Project config (`internal/config/project.go`)

- Implement `ProjectConfig` / `ProfileConfig` structs
- `LoadProject(dir string) (*ProjectConfig, error)` — parse `.spin.toml` from a directory
- Validate: `command` is required in both `[dev]` and `[live]`
- `ResolveProfileDBPath(dataRoot, owner, repo, profile, dbFilename string) string` — returns absolute path, creates parent directories
- `ResolveEnv(env map[string]string, profileDBPath string) map[string]string` — replaces `{profile_db_path}` in all env values

Tests (`project_test.go`):
- Valid config parsing with all fields
- Missing required `command` field
- Template variable resolution in env values
- Directory creation for DB path

#### Step 1.4: CWD-based project resolution (`internal/resolve/resolve.go`)

Only the no-argument CWD detection case. Full name resolution comes in Phase 3.

- Walk up from CWD looking for `.spin.toml`
- Derive project identity from path relative to `projects_root` (strip `{projects_root}/{host}/` prefix)
- Error if CWD is not within `projects_root`
- Error if no `.spin.toml` found walking up to `projects_root`

Tests (`resolve_test.go`):
- CWD is the project root (contains `.spin.toml`)
- CWD is a subdirectory of the project
- CWD is outside `projects_root`
- No `.spin.toml` found

#### Step 1.5: Foreground process execution (`internal/process/foreground.go`)

- Accept: command string, working directory, env map, pre_hook (optional)
- If `pre_hook` is set, run it via `sh -c` in the project directory. Fail if non-zero exit.
- Run main command via `sh -c` (to support shell syntax like pipes, `&&`, etc.)
- Set working directory, merge env vars on top of `os.Environ()`
- Connect stdin/stdout/stderr directly to the parent process
- Trap SIGINT and SIGTERM, forward to child process
- Wait for child to exit, propagate its exit code

Tests (`foreground_test.go`):
- Simple command execution
- Environment variable injection
- Signal forwarding (SIGINT)
- Pre-hook execution and failure handling
- Exit code propagation

#### Step 1.6: `spin dev` command (`cmd/dev.go`)

Wire everything together:
1. Load global config
2. Resolve project (CWD-based only, optional `[project]` arg ignored until Phase 3)
3. Parse `.spin.toml`, extract `[dev]` section
4. Resolve `{profile_db_path}` for the "dev" profile
5. Create data directories if needed
6. Run pre_hook if defined
7. Execute the command in foreground

---

### Phase 2: Background process management

**Goal**: `spin up`, `spin down`, `spin status`, `spin logs` work.

#### Step 2.1: PID file utilities (`internal/process/pidfile.go`)

- `WritePID`, `ReadPID`, `IsProcessAlive`, `CleanStalePIDFile`, `PIDFilePath`
- `IsProcessAlive` uses `syscall.Kill(pid, 0)` — returns `true` if no error

Tests (`pidfile_test.go`):
- Write and read PID round-trip
- Detect alive vs dead processes
- Clean stale PID files
- Handle missing/corrupt PID files

#### Step 2.2: Background process start (`internal/process/background.go`)

- Accept: command string, working directory, env map, log file path, PID file path, pre_hook
- Run pre_hook if defined (same as foreground)
- Open log file in **write** mode (truncate on each `spin up`)
- Start command via `os/exec` with:
  - `Cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}` (new session, detached)
  - `Cmd.Stdout` and `Cmd.Stderr` → log file
  - `Cmd.Dir` → working directory
  - Merged env vars
- Call `Cmd.Start()` (non-blocking — do NOT call `Wait()`)
- Write PID to PID file
- Return PID for confirmation message
- The Go CLI process then exits; the child continues independently

Tests (`background_test.go`):
- Process starts and runs independently
- Log file is created and written to
- PID file is created with correct PID
- Process survives after parent exits

#### Step 2.3: `spin up` command (`cmd/up.go`)

1. Load global config, resolve project
2. Check for existing live instance (read PID file, validate process alive)
   - If alive: print "already running" and exit
   - If stale PID file: clean up and continue
3. Parse `.spin.toml`, extract `[live]` section
4. Resolve `{profile_db_path}` for "live" profile
5. Create data directories and log directory
6. Start background process
7. Print confirmation: project name, PID, port

#### Step 2.4: `spin down` command (`cmd/down.go`)

1. Load global config, resolve project
2. Read PID from `live.pid`
   - No PID file → print "not running", exit
3. Check process alive
   - Dead → clean up PID file, print "already stopped"
4. Send SIGTERM
5. Poll every 100ms for up to 5 seconds
6. If still alive → send SIGKILL
7. Remove PID file
8. Print confirmation

#### Step 2.5: `spin status` command (`cmd/status.go`)

1. Load global config
2. Walk `{data_root}/*/*/live.pid` for PID files
3. For each: validate process, clean stale PIDs, load `.spin.toml` for port
4. Calculate uptime from PID file mtime
5. Print formatted table or "No live instances running."

#### Step 2.6: `spin logs` command (`cmd/logs.go`)

1. Load global config, resolve project
2. Check log file exists at `{data_root}/{owner}/{repo}/logs/live.log`
3. Tail using polling: read last ~50 lines, then poll every 100ms for new bytes
4. Ctrl-C exits tail only

Implementation: simple polling loop (no `fsnotify` dependency). Read existing file, track offset, sleep 100ms, read new bytes, repeat.

#### Step 2.7: Port conflict detection

In `spin up`, after resolving project config:
- Scan all live PID files to find running instances
- If any running instance has the same port configured, print a **warning** (not a hard error — port config is informational)

---

### Phase 3: Discovery + Init

**Goal**: `spin list`, `spin init`, and flexible name resolution.

#### Step 3.1: Project discovery (`internal/discovery/scan.go`)

- Recursively scan `{projects_root}` for directories containing `.spin.toml`
- Skip hidden directories (`.git`, `.spin`, `.jj`, `node_modules`, etc.)
- Extract host, owner, repo from each path
- Return `[]DiscoveredProject`

Tests (`scan_test.go`):
- Finds projects in nested directories
- Skips hidden directories
- Handles empty projects root

#### Step 3.2: `spin list` command (`cmd/list.go`)

1. Load global config
2. Run project discovery
3. Print formatted list with name and path
4. If empty, print helpful message

#### Step 3.3: Full name resolution (`internal/resolve/resolve.go`)

Extend the CWD-only resolver from Step 1.4:

1. **No argument + CWD detection** — already implemented
2. **Full `owner/repo` match** — scan `{projects_root}/*/{argument}/.spin.toml`
3. **Short name match** — scan all projects, match repo portion
   - One match → use it
   - Multiple → print them, ask for specificity
   - None → error with "did you mean?" or `spin list` hint
4. **No match** — error with suggestions

Tests:
- `owner/repo` resolution across multiple hosts
- Short name unique match
- Short name ambiguous match (multiple repos with same name)
- No match with suggestions
- CWD fallback when argument doesn't match

#### Step 3.4: Update all commands with resolver

Update `spin dev`, `spin up`, `spin down`, `spin logs` to accept an optional `[project]` positional argument and use the full resolver.

#### Step 3.5: `spin init` command (`cmd/init_cmd.go`)

Non-interactive, best-effort detection with TODO comments for unknowns.

1. Check if `.spin.toml` already exists → print message and exit
2. Detect project type:
   - `package.json` with `scripts.dev` → `npm run dev`
   - `src-tauri/tauri.conf.json` → `npm run tauri dev`
   - `package.json` with `electron` in `devDependencies` → `npm run dev`
   - Fallback → `# TODO: set your dev command`
3. Write `.spin.toml` with detected values, default ports (dev: 3001, live: 3000)
4. Print what was generated
5. Check `.gitignore` for `.spin/` entry, remind if missing

---

### Phase 4: DB utilities

**Goal**: `spin snapshot`, `spin restore`, `spin clone`, `spin reset`.

#### Step 4.1: DB helpers (`internal/db/snapshot.go`)

Functions:
- `WalCheckpoint(dbPath string) error` — shell out to `sqlite3 {db} "PRAGMA wal_checkpoint(TRUNCATE)"`. Only run if `{db}-wal` file exists.
- `CopyDB(src, dst string) error` — copy DB file plus `-shm` and `-wal` files if they exist.
- `IsProfileRunning(dataRoot, owner, repo, profile string) bool` — check PID file for the profile (only "live" has PID files; "dev" is transient).

Tests (`snapshot_test.go`):
- WAL checkpoint when WAL exists
- WAL checkpoint skip when no WAL
- File copy with and without WAL/SHM files
- Running process detection

#### Step 4.2: `spin snapshot` (`cmd/snapshot.go`)

Usage: `spin snapshot [project] [profile] [name]`

1. Resolve project, validate profile is "dev" or "live"
2. Check DB file exists
3. Run WAL checkpoint
4. Copy to `{data_root}/{owner}/{repo}/snapshots/{name}.db`
5. Print confirmation

#### Step 4.3: `spin restore` (`cmd/restore.go`)

Usage: `spin restore [project] [profile] [name]`

1. Resolve project
2. If profile is "live", check process running → refuse if so
3. Check snapshot exists
4. Copy snapshot to profile's DB path (overwrite)
5. Print confirmation

#### Step 4.4: `spin clone` (`cmd/clone.go`)

Usage: `spin clone [project] [source-profile] [target-profile]`

1. Resolve project
2. If target is "live", check process running → refuse
3. WAL checkpoint the source DB
4. Copy source → target
5. Print confirmation

#### Step 4.5: `spin reset` (`cmd/reset.go`)

Usage: `spin reset [project] [profile]`

1. Resolve project
2. If profile is "live", check process running → refuse
3. Delete DB file (and WAL/SHM if present)
4. Print confirmation (note: app will recreate DB on next start)

---

### Phase 5: Polish

#### Step 5.1: Colorized output

- Add `github.com/fatih/color` dependency
- Color scheme:
  - Green: success messages, running status
  - Yellow: warnings (port conflicts, stale PID files)
  - Red: errors
  - Cyan: project names, commands
- Respect `NO_COLOR` environment variable (handled by `fatih/color`)

#### Step 5.2: Root command enhancement

When `spin` is run with no subcommand:
- Show `spin status` output (running instances)
- Show a brief help summary with common commands
- Not just the bare cobra help text

#### Step 5.3: Shell completions

Cobra supports this natively via `GenBashCompletionV2()`, `GenZshCompletion()`, etc.

- Add `spin completion bash|zsh|fish|powershell` subcommand
- Register custom completion functions for project name arguments that run project discovery

#### Step 5.4: `spin open [project]`

1. Resolve project
2. Determine port (check live first, then dev config)
3. Open `http://localhost:{port}` via `open` command on macOS

#### Step 5.5: Pre-hook verification

Pre-hook support is built into the process execution layer from Phase 1. This step verifies:
- Pre-hook runs before the main command in both dev and live modes
- Pre-hook failures stop execution with a clear error message
- Pre-hook inherits the same env vars as the main command

---

## Testing strategy

### Unit tests

Each `internal/` package gets `_test.go` files alongside the implementation:

- **config**: parsing valid/invalid TOML, template resolution, path construction
- **resolve**: CWD detection, owner/repo match, short name match, edge cases
- **process**: PID file operations, signal handling, env injection
- **discovery**: directory scanning, hidden dir skipping
- **db**: file copy, WAL checkpoint, running process detection

### Integration tests

Create a `testdata/` directory with:
- Sample `.spin.toml` files (valid and invalid)
- A `testdata/projects/` tree simulating the expected directory structure
- Small shell scripts acting as mock dev servers (e.g., `#!/bin/sh\necho started\nsleep 60`)

Integration tests exercise:
- Full `spin dev` → process starts, receives env vars, responds to SIGINT
- Full `spin up` / `spin down` lifecycle
- `spin status` with running and stopped instances
- Name resolution across multiple projects

### Test helpers

- `t.TempDir()` for isolated test directories
- Helper to create project directory structures with `.spin.toml`
- Helper to create and clean up mock global configs

### Running tests

```bash
go test ./...
```

No external test dependencies beyond the standard `testing` package.

---

## Implementation order and dependencies

| Step | Phase | Description | Depends On |
|------|-------|-------------|------------|
| 1.1 | 1 | Project init, go.mod, main.go, root command | — |
| 1.2 | 1 | Global config loading | 1.1 |
| 1.3 | 1 | Project config parsing + template resolution | 1.1 |
| 1.4 | 1 | CWD-based project resolution | 1.2, 1.3 |
| 1.5 | 1 | Foreground process execution | 1.1 |
| 1.6 | 1 | `spin dev` command | 1.2, 1.3, 1.4, 1.5 |
| 2.1 | 2 | PID file utilities | 1.1 |
| 2.2 | 2 | Background process start | 1.5, 2.1 |
| 2.3 | 2 | `spin up` command | 1.2–1.4, 2.1, 2.2 |
| 2.4 | 2 | `spin down` command | 1.2, 1.4, 2.1 |
| 2.5 | 2 | `spin status` command | 1.2, 2.1 |
| 2.6 | 2 | `spin logs` command | 1.2, 1.4 |
| 2.7 | 2 | Port conflict detection | 2.3, 2.5 |
| 3.1 | 3 | Project discovery scanning | 1.2 |
| 3.2 | 3 | `spin list` command | 3.1 |
| 3.3 | 3 | Full name resolution | 1.4, 3.1 |
| 3.4 | 3 | Update all commands with resolver | 3.3 |
| 3.5 | 3 | `spin init` command | 1.3 |
| 4.1 | 4 | DB helper functions | 2.1 |
| 4.2 | 4 | `spin snapshot` | 1.4, 4.1 |
| 4.3 | 4 | `spin restore` | 1.4, 4.1 |
| 4.4 | 4 | `spin clone` | 1.4, 4.1 |
| 4.5 | 4 | `spin reset` | 1.4, 4.1 |
| 5.1 | 5 | Colorized output | All above |
| 5.2 | 5 | Root command enhancement | 2.5 |
| 5.3 | 5 | Shell completions | 3.1 |
| 5.4 | 5 | `spin open` | 1.4 |
| 5.5 | 5 | Pre-hook verification | 1.5, 2.2 |

---

## Example end-to-end session

```bash
# One-time setup
brew install spin  # or `go install github.com/attamusc/spin@latest`

# Onboard a project
cd ~/projects/github.com/attamusc/mindtrail
spin init
# Edit .spin.toml to set correct env var names for your app
echo ".spin/" >> ~/projects/.gitignore

# Daily workflow
spin up                         # live mode, backgrounded on :3000
spin dev                        # dev mode, foregrounded on :3001
# hack, ctrl-c, repeat
spin dev
# done for the day
spin down

# Check what's running
spin status

# Tail live logs
spin logs

# Snapshot before risky migration
spin snapshot live before-migration
# ... something goes wrong ...
spin down
spin restore live before-migration
spin up
```
