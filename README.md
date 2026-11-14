# spin

A local project runner and manager for macOS (arm64 and amd64) and Fedora 44 (x86_64). Navigate between projects, clone repos, scaffold from templates, and run dev/live instances with isolated SQLite databases — all from one tool.

## Why

When working on local projects you want:

- Fast navigation between dozens of repos without remembering paths
- Git cloning that puts repos in a consistent `host/owner/repo` directory structure
- The feedback loop of `npm run dev` (or equivalent) at all times
- A stable "live" instance running in the background while you hack in the foreground
- Data isolation between dev and live so your work never touches real data
- Shell integration with fzf-powered project picking and tab completion

`spin` gives you all of this — one config file, two modes, zero build step differences.

## Install

```bash
go install github.com/attamusc/spin@latest
```

Or clone and build locally:

```bash
git clone https://github.com/attamusc/spin.git
cd spin
make build          # compiles to dist/spin
make install        # installs to /usr/local/bin (PREFIX=/usr/local)
```

## Quick start

```bash
# Navigate to a project (cd's into the directory)
spin go attamusc/mindtrail            # by owner/repo
spin go mindtrail                     # by short name
spin go                               # list project names; Ctrl+Space opens fzf

# Clone a repo into projects_root/github.com/owner/repo
spin clone attamusc/some-repo

# Create a new project with VCS init
spin new my-project
spin new my-project --template tauri

# Set up process management
cd ~/projects/github.com/you/your-app
spin init                              # generates .spin.toml with detected defaults
# Edit .spin.toml to configure env vars, ports, etc.

# Daily workflow
spin up                                # start live mode (backgrounded, :3000)
spin dev                               # start dev mode (foregrounded, :3001)
# hack, ctrl-c, repeat
spin down                              # stop live instance
```

## Shell integration

Add to your `.zshrc` (after `compinit`):

```bash
eval "$(spin shell-init zsh)"
```

This provides:

- **Shell wrapper** — `spin go`, `spin clone`, and `spin new` automatically `cd` into the project directory
- **Ctrl+Space** — fzf widget for interactive project selection
- **Tab completion** — full completion for all commands, flags, and project names
- **Completion cache** — stale-while-revalidate project list cache (30 min TTL, async refresh)

## Commands

### Project navigation

| Command | Description |
|---------|-------------|
| `spin go [project]` | Navigate to a project directory (list names when no arg; Ctrl+Space for fzf) |
| `spin clone <repo>` | Clone a git repo into the projects directory |
| `spin new <name>` | Create a new project directory with VCS init |
| `spin list` | List all projects (`*` marks configured ones) |
| `spin update [project]` | Fetch latest changes for all or one project |

### Process management

| Command | Description |
|---------|-------------|
| `spin` | Dashboard — show running instances and common commands |
| `spin dev [project]` | Run in dev mode (foregrounded) |
| `spin up [project]` | Start live mode (backgrounded) |
| `spin down [project]` | Stop a live instance |
| `spin status [--format plain\|json]` | Observe recorded live-mode PID files without changing them |
| `spin logs [project]` | Tail the live instance log file |
| `spin init` | Generate a `.spin.toml` in the current directory |
| `spin open [project]` | Open the project in the browser |

### Database operations

| Command | Description |
|---------|-------------|
| `spin db copy <src> <dst>` | Copy one profile's database to another |
| `spin db snapshot <profile> <name>` | Create a named snapshot of a profile's database |
| `spin db restore <profile> <name>` | Restore a database from a named snapshot |
| `spin db reset <profile>` | Delete a profile's database |

Database copy/snapshot uses SQLite's native page-backup API, including committed WAL data and preserving implicit ROWIDs; it does not copy WAL/SHM files or require the `sqlite3` executable. Restore preserves the destination inode and requires SQLite exclusive access. Copy/restore/reset also reject a running spin-managed target (dev or live), and serialize with lifecycle operations using per-profile advisory locks. Stop external users before destructive operations: idle rollback-journal connections cannot always be detected. The state tree is trusted local storage, not a sandbox against concurrent hostile filesystem mutation.

### Shell & completions

| Command | Description |
|---------|-------------|
| `spin shell-init zsh` | Print shell integration script (wrapper + completions) |
| `spin completion <shell>` | Generate standalone shell completions (bash/zsh/fish/powershell) |

### Command flags

| Command | Flag | Description |
|---------|------|-------------|
| `spin clone` | `--host <host>` | Git host (default: from config or `github.com`) |
| `spin new` | `--template, -t <name>` | Template name from config's `[templates]` map |
| `spin list` | `--configured, -c` | Show only projects with `.spin.toml` |
| `spin list` | `--format plain` | One project name per line (for scripting) |

### Project resolution

When a `[project]` argument is given, it's resolved in order:

1. **No argument** — use the containing `host/owner/repo` project and its root `.spin.toml` (process commands); `spin go` lists all project names
2. **`owner/repo`** — match against `{projects_root}/*/owner/repo`
3. **Short name** — match the repo portion across all discovered projects

Exact names take precedence over case-insensitive substring matches. Multiple matches produce a disambiguation list, including the host for cross-host collisions. No matches produce suggestions; navigation does not automatically clone.

Incomplete scans retain accessible rows in `list` and argument-free `go`, and bulk `update` still fetches accessible repositories; these commands return nonzero to report incompleteness. Completion and the cached picker can suggest accessible candidates. Named navigation (`spin go <name>`), single-project update, and configured resolution all fail closed when an unreadable host could hide another match.

## Project config

Each project gets a `.spin.toml` in its root:

```toml
db_filename = "data.db"

[dev]
command = "npm run dev"
port = 3001
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3001" }

[live]
command = "npm run dev"
port = 3000
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3000" }
```

- `{profile_db_path}` resolves to the full path of the profile's SQLite database
- `port` is used for status display, conflict detection, and `spin open`; it does not enforce which port the application listens on. Zero means unset; configured ports must be 1..65535. `dev` and `up` validate only their selected profile, and `open` validates the port it selects. Status retains project identity and reports `invalid_port` for an invalid live port.

`spin status --format json` emits a schema-versioned observation with explicit
nullable metadata and diagnostics. Exit 0 means complete (including empty or
stale results); exit 1 means incomplete. Plain status and the dashboard are also
read-only: stale PID files are retained. PID existence does not verify process
identity, readiness, or listening ports; uptime is estimated from PID-file mtime.
See [the status contract](docs/status-contract.md) for the JSON schema and states.
- `pre_hook` (optional) runs a shell command before the main command starts
- Both profiles run the same dev command — only the environment differs

## Global config

Lives at `~/.config/spin/config.toml`:

```toml
projects_root = "~/projects"          # where projects live (default: ~/projects)
default_host  = "github.com"          # default git host for clone/new
default_owner = "attamusc"            # default owner for short-name resolution

[templates]
  default = "~/dotfiles/templates/project"
  tauri   = "~/dotfiles/templates/tauri-app"
```

- `projects_root` defaults to `~/projects`. Data is stored in `{projects_root}/.spin/`.
- `default_host` defaults to `github.com`. Used by `spin clone` and `spin new`.
- `default_owner` supplies the owner for bare names in `spin clone` and `spin new`. Navigation searches local repositories instead.
- `[templates]` maps names to directories for `spin new --template <name>`. A template root may be a symlink. Its contents copy as regular files/directories, preserving file permissions and excluding `.git`/`.jj`; linked entries and special files are rejected. If VCS initialization fails, `spin new` warns and retains the scaffolded directory.
- Project, snapshot, and DB filenames must be single path components: no slashes, backslashes, or control characters, and not equal to `.` or `..`. State descendants (including `.spin` itself) must not be symlinks; a configured projects-root symlink is allowed. Repositories aliased outside that root are not supported.
- Same `owner/repo` names on multiple hosts are ambiguous. Configured commands reject these collisions rather than sharing PID/log/DB state. Existing state is not moved.

## Directory structure

```
~/projects/                                 # projects_root
  .spin/                                    # data_root (gitignored)
    attamusc/mindtrail/
      dev/data.db                           # dev profile database
      live/data.db                          # live profile database
      logs/live.log                         # live process logs
      live.pid                              # live process PID
      snapshots/before-migration.db         # named snapshots
  github.com/attamusc/mindtrail/            # project directory
    .spin.toml                              # project config
```

Projects are discovered by scanning `projects_root` at depth 3 (`host/owner/repo`). All directories at this depth are considered projects — `.spin.toml` is required for process management and database commands.

## Version control

VCS operations prefer [Jujutsu](https://github.com/jj-vcs/jj) (`jj`) and fall back to `git` if jj is not found.

- `spin clone` uses `jj git clone --colocate` (or `git clone`)
- `spin new` initializes with `jj git init --colocate` (or `git init`)
- `spin update` fetches with `jj git fetch` (or `git fetch`)

## Development

```bash
make build          # compile to dist/spin
make test           # run all tests
make test-race      # run tests with race detector
make lint           # vet + pinned staticcheck; no silent skips
make vuln           # pinned govulncheck
make fmt            # format source files
make help           # show all targets
```

Requires Go 1.25.5+ for the application. Lint/vulnerability tooling uses a Go 1.26 toolchain and pinned `go run` versions (downloaded into Go's cache, not globally installed). Use current patch releases. CI tests Linux/macOS with Go 1.25 and 1.26, including race and executable zsh fixtures.

## License

MIT
