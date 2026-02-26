# codecom Technical Architecture (v1)

## Scope
- In: `scan` and `tui`.
- In: batch `cwd` move with strict validation and Git-backed commits.
- In: orphaned marker (`cwd` path missing) computed for all sessions.
- Out: session copy/fork behavior, heuristics/suggestions, MCP server.

## Runtime Model
- Single terminal app binary.
- Default Codex data root: `~/.codex`.
- Override root via `--codex-dir`; config resolves under that root.

## Proposed Package Layout
```text
cmd/codecom/
  main.go

internal/app/
  run_tui.go
  run_scan.go

internal/config/
  config.go

internal/sessionindex/
  scan.go
  parse_jsonl.go
  models.go
  orphan_check.go
  sort.go

internal/move/
  planner.go
  validator.go
  rewriter.go
  confirm.go

internal/gitops/
  repo.go
  preflight_snapshot.go
  commit_move.go
  undo.go

internal/tui/
  model.go
  keys.go
  panels.go
  session_list.go
  dialogs.go
  statusbar.go

internal/output/
  ascii_scan.go
  jsonl_scan.go
```

## Data Flow
1. Load config (`codecom.toml`) and CLI flags.
2. Scan sessions from `sessions/YYYY/MM/DD/*.jsonl`.
3. Parse only needed fields from JSONL lines:
   - session id
   - session file path
   - `cwd` from `session_meta` and `turn_context`
   - latest total/last tokens
4. Compute orphan status for all sessions (worker pool).
5. Build folder trees and session list projections for TUI.

## Move Flow (Safety-Critical)
1. User picks source root (left), target root (right), and sessions.
2. Planner builds remap using relative suffix from source root.
3. Validator enforces:
   - each selected session cwd is under source root
   - mapped destination path exists
   - batch fails if any violation
4. Confirmation dialog:
   - summary + sample mappings (truncate if >10)
   - optional “do not show again” setting
   - forced confirmation when batch size >10
5. Git preflight:
   - auto-commit pre-existing dirty state:
     - `codecom: snapshot pre-existing local changes`
   - abort if preflight commit fails
6. Rewrite:
   - update `session_meta.payload.cwd`
   - update all `turn_context.payload.cwd`
7. Commit:
   - one commit per changed session file
   - machine-readable JSON in commit message body
   - include trailer for undo targeting
8. Live recompute in memory and refresh UI counts.

## Parsing Strategy
- Stream files line-by-line.
- Use high-performance JSON parse path extraction.
- Tolerant parsing:
  - skip malformed lines
  - keep session record
  - count warnings (logs only; included in JSONL output)

## TUI Structure
- Top area: two folder trees
  - left: source
  - right: target
- Bottom area: session rows for current source folder only
- Status bar: scan/orphan progress and operation status
- Colors:
  - red for orphaned folder/session paths
  - marker with count for known session-bearing directories (`[N]`, hide 0/1)

## Keybindings (initial)
- `F6`: move (confirmation)
- `F5`: refresh (full rescan)
- `Space`: toggle row selection
- `A`: select all rows in current source folder
- `U`: undo last codecom cwd-change commit
- `Y`: copy detailed error report to clipboard

## CLI Commands
- `codecom` (defaults to `tui`)
- `codecom tui [--codex-dir <path>]`
- `codecom scan --json|--ascii [--codex-dir <path>]`

## Output Contract
- `scan --json`: JSONL session records to stdout.
- `scan --ascii`: table rows to stdout.
- Version header, warnings, errors to stderr.
- Exit codes:
  - scan: `0` unless fatal scan failure
  - tui: non-zero on startup/runtime fatal errors

