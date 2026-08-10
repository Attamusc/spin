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
spin go                               # interactive fzf picker

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
| `spin go [project]` | Navigate to a project directory (fzf picker when no arg) |
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
| `spin status` | Show all running live instances |
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

1. **No argument** — walk up from CWD looking for `.spin.toml` (process commands) or list all projects (navigation commands)
2. **`owner/repo`** — match against `{projects_root}/*/owner/repo`
3. **Short name** — match the repo portion across all discovered projects

Multiple matches produce a disambiguation list. No matches produce suggestions.

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
- `port` is used for status display, conflict detection, and `spin open` — not enforced
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
- `default_owner` is used to expand bare repo names (e.g., `spin go mindtrail` → `attamusc/mindtrail`).
- `[templates]` maps names to directories for `spin new --template <name>`.

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

Projects are discovered by scanning `projects_root` at depth 3 (`host/owner/repo`). All directories at this depth are considered projects — `.spin.toml` is only required for process management commands.

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
make lint           # vet + staticcheck
make fmt            # format source files
make help           # show all targets
```

Requires Go 1.25+.

## License

MIT
