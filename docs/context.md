# codecom Context (Session Summary)

## Objective
Design an open-source, terminal-first Go app (`codecom`) to manage Codex sessions from `.codex`, with emphasis on safe session migration after repo/folder moves.

## Final Product Direction
- Primary audience: developers who move/rename repos, and monorepo users.
- Positioning: safety-first migration toolkit (not generic session browser).
- Core value:
  - fast session discovery across folders
  - safe batch `cwd` migration
  - Git-backed auditability/revert

## Research Findings Captured
- Codex sessions are in `~/.codex/sessions/YYYY/MM/DD/*.jsonl`.
- Filename pattern observed: `rollout-<timestamp>-<id>.jsonl`.
- `cwd` exists in:
  - `session_meta.payload.cwd`
  - repeated `turn_context.payload.cwd`
- Token data (when present) in `event_msg` with `payload.type="token_count"`:
  - `total_token_usage`
  - `last_token_usage`
- Resume/session identity relevance:
  - local history keyed by `session_id`
  - resume behavior depends on known session IDs
  - conclusion: avoid v1 copy/fork semantics that may break continuity assumptions

## Major Scope Decisions (Q/A Outcomes)
- Build as single Go app, terminal-first.
- Use Bubble Tea for TUI.
- v1 commands:
  - `codecom` (default to `tui`)
  - `codecom tui`
  - `codecom scan --json|--ascii`
- Defer `copy` feature.
- `move` means rewriting `cwd`.
- Batch move required.
- UI:
  - two panels: left `source`, right `target`
  - bottom pane for session multi-select
- Select-all in current source folder required.
- Mapping rule:
  - preserve relative suffix under source root
  - example: `~/dev/java/app1` -> `~/projects/java/app1`
- Destination path policy:
  - must already exist on OS
  - do not create missing dirs
  - if any mapping invalid, fail whole batch
- Update both cwd locations during move:
  - `session_meta.payload.cwd`
  - all `turn_context.payload.cwd`

## Safety/Trust Decisions
- Confirmation mandatory for move.
- “Do not show again” setting allowed, but confirmation forced when batch size `>10`.
- Pre-existing dirty `.codex` Git state:
  - auto-commit first with message:
    - `codecom: snapshot pre-existing local changes`
  - abort move if this preflight commit fails
- Move commits:
  - one commit per changed session file
  - include machine-readable JSON in commit message body
  - transparent absolute paths allowed
- Undo:
  - `undo last move` required
  - only undo most recent codecom cwd-change commit
  - never touch unrelated external commits

## Data/Parsing Decisions
- Stream JSONL parsing, high-performance parser approach.
- Tolerant mode:
  - skip malformed lines
  - keep session record
  - warnings logged
- Orphan detection in v1:
  - orphan if `cwd` path does not exist
  - show orphaned paths/folders in red
  - no auto-suggestions
- Orphan checks:
  - for all discovered sessions
  - progressive updates in UI
  - worker pool concurrency default: `min(32, NumCPU*2)`
- Refresh:
  - full rescan (`F5`)
  - live in-memory recompute after moves

## Display/UX Decisions
- Folder tree sorted alphabetically.
- Hidden directories: not shown, no toggle in v1.
- No special exclusions (`/proc`, `/sys`, etc.) configured.
- Known-session folder marker format:
  - show `[N]` count for total sessions under folder
  - suppress `[0]` and `[1]`
- Session row sort:
  - by `cwd` ascending
  - then newest first
- Tokens display in session list:
  - show both total and last
  - missing -> `n/a`
- Copy report key:
  - `y` copies detailed failure list to system clipboard
  - if clipboard fails, show scrollable modal with full text
  - no fallback file creation

## CLI Output Contract
- `scan --json`:
  - JSONL records on stdout
  - include warnings count field
- `scan --ascii`:
  - flat per-session rows
  - include file path
  - path truncation from the left, style like `~/…/java/app1`
- Version/errors/warnings to stderr; data to stdout.
- `scan` exit code:
  - `0` with warnings
  - non-zero only on fatal scan failure

## Config/Paths Decisions
- Default codex dir: `~/.codex`.
- Override flag: `--codex-dir`.
- Config path follows selected codex dir.
- Config file: `<codex-dir>/codecom.toml`.
- Generate config file with defaults on first launch.
- Keep config minimal.

## Launch/OSS Decisions
- Name: `codecom`.
- License: MIT.
- Distribution target: both `go install` and prebuilt releases/Homebrew.
- Launch messaging:
  - safety-first migration narrative
  - explicit statement: writes only in TUI move flow
- Demo asset:
  - one GIF covering discovery + batch move + Git audit
- Week-1 success metrics:
  - GitHub stars
  - qualitative user feedback
- Built-in issue helper idea accepted; include version/hash and Codex CLI version if possible.

## Files Created In This Session
- `docs/promotion.md`
- `docs/architecture.md`
- `AGENTS.md`
- `context.md` (this file)

