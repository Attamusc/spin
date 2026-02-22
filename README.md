# spin

A local project runner with data profile switching. Run the same dev script in two isolated modes — **foregrounded dev** and **backgrounded live** — each with its own SQLite database.

## Why

When working on local projects you want:

- The fast feedback loop of `npm run dev` (or equivalent) at all times
- A stable "live" instance running in the background while you hack in the foreground
- Data isolation between the two so dev work never touches your real data

`spin` gives you exactly that — one config file, two modes, zero build step differences.

## Install

```bash
go install github.com/attamusc/spin@latest
```

Or clone and build locally:

```bash
git clone https://github.com/attamusc/spin.git
cd spin
make install   # installs to /usr/local/bin
```

## Quick start

```bash
# Set up a project
cd ~/projects/github.com/you/your-app
spin init                       # generates .spin.toml with detected defaults
# Edit .spin.toml to configure env vars, ports, etc.

# Daily workflow
spin up                         # start live mode (backgrounded, :3000)
spin dev                        # start dev mode (foregrounded, :3001)
# hack, ctrl-c, repeat
spin down                       # stop live instance
```

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

Optional. Lives at `~/.config/spin/config.toml`:

```toml
projects_root = "~/projects"
```

Defaults to `~/projects` if not set. Data is stored in `{projects_root}/.spin/`.

## Commands

| Command | Description |
|---------|-------------|
| `spin` | Show running instances and common commands |
| `spin dev [project]` | Run in dev mode (foregrounded) |
| `spin up [project]` | Start live mode (backgrounded) |
| `spin down [project]` | Stop a live instance |
| `spin status` | Show all running live instances |
| `spin logs [project]` | Tail the live instance log file |
| `spin list` | List all discoverable projects |
| `spin init` | Generate a `.spin.toml` in the current directory |
| `spin open [project]` | Open the project in the browser |
| `spin snapshot [profile] [name]` | Snapshot a profile's database |
| `spin restore [profile] [name]` | Restore a database from a snapshot |
| `spin clone [src] [dst]` | Copy one profile's database to another |
| `spin reset [profile]` | Delete a profile's database |
| `spin completion [shell]` | Generate shell completions (bash/zsh/fish/powershell) |

### Project resolution

When a `[project]` argument is given, it's resolved in order:

1. **No argument** — walk up from CWD looking for `.spin.toml`
2. **`owner/repo`** — match against `{projects_root}/*/owner/repo`
3. **Short name** — match the `repo` portion across all discovered projects

If there are multiple matches, you'll get a list. If there are no matches, you'll get suggestions.

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

## Shell completions

```bash
# Bash
source <(spin completion bash)

# Zsh
spin completion zsh > "${fpath[1]}/_spin"

# Fish
spin completion fish | source
```

Project names are tab-completed for commands that accept a `[project]` argument.

## Development

```bash
make build          # compile to dist/spin
make test           # run all tests
make test-race      # run tests with race detector
make lint           # vet + staticcheck
make fmt            # format source files
make help           # show all targets
```

## License

MIT
