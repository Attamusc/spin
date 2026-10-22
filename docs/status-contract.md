# Observational status contract (v1)

Specification only; not implemented by this change. Source baseline:
`5201701c49da5cd6fa6e7783f0f2a10a2ca955b4`.

## Command and safety boundary

Use **`spin status --format json`**, consistent with `spin list --format plain`.
Do not add `--json`. Omitted format and `--format plain` select the existing
human table style; reject other values and positional arguments.

Status observes recorded live-mode PID files, not every configured project or
foreground dev process. The plain command and root dashboard must use the same
non-mutating collector. No PID deletion, file/directory/config creation, process
control, hooks, subprocesses, VCS, or network operations are permitted in this
observation path. A PID existence probe (`kill(pid, 0)`) is permitted; sending an
actual signal is not. No database/profile-path resolution is needed.

The result is a best-effort observation, not an atomic snapshot. `live` means
only that the recorded PID exists at probe time: PID reuse, zombie processes,
application identity, readiness, health, and listening ports are NOT verified.

## JSON shape

Emit exactly one JSON object plus a trailing newline on stdout, with no color,
headers, usage, or human messages. All fields below are required; nullable fields
must be encoded as `null`, never omitted. Empty arrays are `[]`, never `null`.
Use Go's JSON encoder, not string interpolation. Object key order is not a
consumer contract. Consumers must check `schema_version` and ignore unknown fields.

```json
{
  "schema_version": 1,
  "complete": true,
  "projects_root": "/home/me/projects",
  "data_root": "/home/me/projects/.spin",
  "instances": [
    {
      "owner": "me",
      "repo": "app",
      "mode": "live",
      "pid_file": "/home/me/projects/.spin/me/app/live.pid",
      "state": "live",
      "pid": 12345,
      "host": "github.com",
      "project_path": "/home/me/projects/github.com/me/app",
      "port": 3000,
      "uptime_seconds": 42,
      "uptime_estimated": true
    }
  ],
  "diagnostics": []
}
```

Root paths are absolute, lexically cleaned (relative configured paths are
resolved against invocation CWD); do not resolve symlinks to claim identity.
Root paths are both `null` when global configuration cannot be loaded/validated.
Instance fields:

- `owner`, `repo`: data-directory components, not a globally unique project ID.
- `mode`: always `"live"` in v1.
- `pid_file`: absolute observed file path; stable instance key within this data root.
- `state`: `live | stale | malformed | unreadable | unknown`, as below.
- `pid`: positive integer when parsed successfully; otherwise `null`.
- `host`, `project_path`: unique project match, otherwise both `null`.
- `port`: configured live port in 1..65535, or `null`; never evidence of a listener.
- `uptime_seconds`: nonnegative integer, floored seconds since PID-file mtime,
  only for `live` with a readable, non-future mtime; otherwise `null`.
- `uptime_estimated`: true exactly when `uptime_seconds` is non-null. This is
  PID-file age, not measured process uptime. No zero default on stat failure.

Sort instances by `(owner, repo, pid_file)` using Go string order. Sort diagnostics
by `(path, code, message)`; a null path sorts before strings. No timestamp is
required in the envelope. Example clean empty result:

```json
{"schema_version":1,"complete":true,"projects_root":"/home/me/projects","data_root":"/home/me/projects/.spin","instances":[],"diagnostics":[]}
```

## Collection and uncertainty

Scan only `{data_root}/{owner}/{repo}/live.pid`. A missing data root is a normal
first-use empty result, without creating it. A missing projects root remains a
global-config validation error, consistent with `LoadGlobal`. Missing PID files
are normal absences, not stopped-instance rows. If a previously discovered PID
file disappears during a read, omit it; do not invent a stale PID.

| Evidence | State / treatment |
|---|---|
| Positive PID, probe succeeds | `live` |
| Positive PID, probe returns ESRCH | `stale`; retain file unchanged |
| Empty, nonnumeric, overflow, zero, negative PID | `malformed`, PID null; never probe zero/negative values |
| PID file cannot be read, or is not a regular file | `unreadable`, PID null |
| Positive PID, probe returns EPERM or any other non-ESRCH error | `unknown`, retain PID; do not equate permission failure with death |

Do not recurse through symlink owner/repo directories or read symlink/nonregular
PID/config files. Report an `unsupported_entry` diagnostic for such entries at
expected scan positions; ordinary unrelated files are ignored. This bounds the
scan and avoids following aliases outside roots. A symlink configured root is
allowed, but its spelling does not prove canonical identity.

Find project candidates at `{projects_root}/{host}/{owner}/{repo}` across
non-hidden real host directories; a candidate has a `.spin.toml` entry (even if
unreadable/malformed). Do not choose the first host. Exactly one candidate sets
host/path, then `LoadProject` supplies the port. Zero candidates leaves both null
and emits `project_missing`. Multiple candidates leave host/path/port null and
emit `project_ambiguous` listing sorted candidate paths in the human message.
If any host/candidate lookup fails unexpectedly, uniqueness is unproven: leave
host/path/port null, emit `project_scan_failed`. No storage migration: existing
data paths omit host, so duplicate host/owner/repo names cannot be disambiguated.

Unreadable/malformed project config preserves a uniquely matched host/path but
sets port null with `project_config_error`. Missing/zero configured port is a
normal null; negative/out-of-range port emits `invalid_port`. Never expose env,
commands, pre-hooks, or raw config contents in JSON or diagnostics.

## Diagnostics and exits

Each diagnostic is `{ "code": string, "path": string|null, "message": string }`.
Paths identify the failing operation (PID file, config, directory); path null is
allowed for home/CWD resolution failures. Messages are human-readable, not parsed
by consumers, and must not include config contents. Emit corresponding readable
messages on stderr as well, without contaminating stdout.

Stable v1 codes: `global_config_error`, `scan_failed`, `unsupported_entry`,
`pid_malformed`, `pid_unreadable`, `pid_probe_failed`, `pid_stat_failed`,
`pid_mtime_future`, `project_missing`, `project_ambiguous`, `project_scan_failed`,
`project_config_error`, `invalid_port`.

- Exit **0**: complete observation (`complete: true`, no diagnostics). Both live
  and stale records are valid evidence; stale alone is not an error.
- Exit **1**: incomplete observation (`complete: false`, at least one diagnostic).
  Preserve all successfully observed records, including malformed/unreadable/
  unknown rows. Failed data-root/owner/repo directory reads emit `scan_failed`;
  continue accessible siblings. Global-config failure emits the envelope with
  null roots, empty instances, and `global_config_error`. This is not a clean
  empty result. Metadata uncertainty also makes the observation incomplete.
- Invalid argv/format exits **1**, stderr usage/error, no JSON required. Output
  write failure exits **1**, stderr error where possible; cannot promise a full
  JSON object when stdout fails. Do not emit a second object or success message.

Plain status shows all collected states (add a STATE column), null metadata as
`-`, and never says "No live instances running" for an incomplete empty scan.
A complete empty scan may retain that message. The root dashboard may keep only
live rows under its existing heading, but must show diagnostics/incompleteness,
render its help even on collection failure, and return exit 1 on incomplete
observation. Plain status remains stdout data; dashboard/help remain stderr.
Cobra error/usage handling must avoid duplicate diagnostics after JSON emission.

## Affected implementation paths

Source inspection (not runtime verification) found:

- `cmd/status.go`: `scanRunningInstances` currently removes stale PIDs, suppresses
  read/config errors, and returns live rows only. `findProjectDir` chooses first
  host. Replace with an observational result carrying states/diagnostics;
  separate collection from plain/JSON rendering.
- `cmd/root.go`: dashboard shares that scan and suppresses global config errors;
  adopt the result and incomplete semantics without mutating. Completion helpers
  are not a new status API and are outside this change.
- `cmd/up.go`: `checkPortConflicts` shares the scan. Consume only `live` rows with
  known ports, warn about incomplete observations on stderr, and preserve its
  advisory (non-blocking) nature. Do not treat stale/unknown rows as running.
  Keep explicit own-project stale cleanup in `runUp`.
- `cmd/down.go`, `internal/process/pidfile.go`: preserve lifecycle cleanup and
  existing tests. `ReadPID` currently accepts zero/negative numbers and
  `IsProcessAlive` collapses every error into false. Status needs positive-PID
  validation and a probe retaining errno; do not silently alter lifecycle policy
  as part of this contract. A small internal observational helper is sufficient.
- `internal/config/global.go`: **LoadGlobal/LoadGlobalFrom do not write defaults**;
  they read config, apply in-memory defaults, stat projects root, and compute
  `.spin`. A missing config is allowed only if default `~/projects` exists.
- `internal/config/project.go`: `LoadProject` is read-only but validates both
  dev/live commands. Reuse this validation; `ResolveProfileDBPath` creates
  directories and must never enter status/dashboard collection.
- `cmd/list.go`: precedent for `--format`, not its permissive unknown-format
  fallback. `main.go` currently maps returned errors to process exit 1.

## Fixture matrix and implementation checklist

Use isolated `t.TempDir()` roots and HOME, injected probe/clock/filesystem errors
where needed. Permission tests must not rely solely on chmod when run as root.
Tests may create fixtures/processes; the product observation path must not.

| Fixture | Required assertions |
|---|---|
| No config, existing default projects root; absent `.spin`; empty `.spin` | Defaults only in memory, exit 0, arrays `[]`, no created paths |
| Missing projects root; root is a file; unreadable root; malformed/unreadable global config | Exit 1, error envelope, not a clean empty scan |
| Current positive PID; completed/reaped child PID | Live vs stale; stale file survives repeated JSON/plain/dashboard observations |
| Empty/text/overflow/zero/negative PID | Malformed row/null PID, diagnostic, no invalid probe |
| Unreadable/nonregular PID; EPERM/other probe error | Unreadable vs unknown, exit 1, no deletion |
| Unreadable data root/owner/repo, accessible sibling | Sorted partial rows retained, scan diagnostic, exit 1 |
| Known port; absent/zero/invalid port; malformed/unreadable config | Nullable metadata and exact diagnostics without suppressing PID row |
| Missing project; same owner/repo under two hosts; failed host lookup | No invented host/path/port; ambiguity and uncertainty surfaced |
| Failed stat; future mtime; fixed valid mtime | Null vs estimated floored uptime, no negative/default-zero age |
| Names/paths with quotes, backslashes, tabs, newlines, Unicode; shuffled creation order | JSON round-trip escaping and stable array order |
| Symlink directories/PID/config | Bounded scan, unsupported diagnostic, no traversal |
| CLI invalid format/args, stdout write failure; root help on failure | Exit/stream contract; no duplicate object, color, or Cobra stdout usage |
| Shared `up` port check and explicit `up`/`down` cleanup | Live-only known-port warnings; lifecycle cleanup remains explicit |

Implementation sequence:
1. Add failing collector/CLI fixtures for this contract; retain lifecycle tests.
2. Implement errno-aware read-only collection and deterministic diagnostics.
3. Wire JSON/plain/dashboard and advisory port-conflict caller; document CLI help.
4. Compare filesystem tree, contents and mtimes before/after repeated observation
   (including stale/malformed files and absent config/data directories). Assert no
   subprocess/hook/lifecycle helper is called. Include actual CLI subprocess
   tests for JSON stdout/stderr/exit, not only struct serialization tests.
5. Run `make test`, `make vet`, and scoped CLI fixtures; independently review the
   read-only boundary. Existing passing tests alone do not establish the contract.

## Baseline evidence and parent-ticket refinements

Executed before writing this specification, in this checkout:

- `make test` → exit **0**; expanded argv `go test ./... -count=1`.
  Output: main `[no test files]`; `ok` for cmd, internal/config, db, discovery,
  process, resolve, shell, template, vcs. Process tests took 1.527s; other packages
  0.002–0.015s. This is baseline regression evidence, not status-contract proof.
- `make vet` → exit **0**; output `go vet ./...`, no diagnostics.
- `jj log -r @- --no-graph` identified baseline `5201701c`; `jj status` initially
  clean. Initialized colocated jj metadata because checkout had only `.git`;
  no source content changed by initialization. No install/push/global changes.

Proposed precise refinements (parent owns tickets; not edited here):

- **TODO-b53c2f69:** make this document the implementation contract; use only
  `status --format json`, schema version 1, required nullable fields, errno-aware
  PID states, host ambiguity, deterministic partial-result/exit-1 semantics.
  Explicitly include plain status, dashboard, and `up`'s shared port-check caller;
  keep lifecycle cleanup unchanged. Require fixture matrix and actual CLI tests.
- **TODO-6f70ee74:** require independent verification of schema/streams/exits and
  before/after filesystem evidence across JSON/plain/dashboard, including stale
  files and missing defaults. Test unknown liveness, inaccessible siblings,
  duplicate hosts, config failures, escaping and uptime uncertainty. Treat PID
  liveness as existence only; never report readiness/identity as verified. Gate
  completion on runtime CLI evidence, not source inspection or baseline tests.

Only writable deliverable: `docs/status-contract.md`. No product implementation,
commit, ticket transition, or roadmap edit is part of this discovery task.
