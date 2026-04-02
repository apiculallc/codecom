# codecom

Safe, terminal-first management for local Codex sessions when your repos move and your `cwd` history does not.

## Demo

![codecom demo](demos/codecom.opt.gif)

## Why This Exists

If you rename a repo, move directories, or consolidate projects into a monorepo, old Codex sessions can become awkward to discover and harder to trust.  
`codecom` is focused on exactly that failure mode:

- fast local session discovery from `~/.codex/sessions/...`
- safe batch `cwd` migration with strict validation
- Git-backed audit trail so changes are inspectable and reversible

This is not a generic dashboard and it is not trying to be your next data platform. It is a careful wrench for a specific job.

## What It Does Today

### Commands

- `codecom` (defaults to `tui`)
- `codecom tui [--codex-dir <path>]`
- `codecom scan --json [--codex-dir <path>]`
- `codecom scan --ascii [--codex-dir <path>]`

### `scan` mode (read-only by contract)

- Scans session JSONL files under `sessions/YYYY/MM/DD/*.jsonl`
- Extracts per-session metadata:
  - `session_id`
  - session file path
  - effective `cwd`
  - orphan status (`cwd` path missing on host)
  - token counts when present
  - warning counts for malformed lines
- Emits data only to `stdout`:
  - JSONL with `--json`
  - tabular text with `--ascii`
- Emits version + warnings to `stderr`
- Never rewrites session files or SQLite state

### `tui` mode (explicit move workflow)

- Three panes:
  - source folder tree (session-derived)
  - target folder tree (real host filesystem, home-rooted by default)
  - sessions for current source folder
- Multi-select sessions (`Space`, `A`)
- Confirmed move flow (`F6`) rewrites both JSONL and SQLite thread paths
- Conversation search (`Ctrl+F`) across message text with token/phrase query parsing

## The Safety Model

`codecom` is intentionally conservative.

1. No writes in `scan`.
2. Move validates the full batch before rewriting anything.
3. Destination paths must already exist.
4. If one mapping is invalid, the entire batch is blocked.
5. If relevant files are dirty, preflight snapshot commit is created first.
6. Move commits are granular (one commit per changed session file, with structured commit body and trailer).
7. Paths in commit metadata are redacted unless safely representable relative to repo root.

In short: it would rather fail loudly than silently improvise.

## Core Use Cases

### 1. Repo move / home prefix migration

Example: `/home/alice/dev/...` moved to `/home/alice/projects/...`.

Pattern:

1. Run `scan` to inspect orphans and impacted sessions.
2. Open `tui`.
3. Choose source root and target root.
4. Select sessions.
5. Confirm move.
6. Inspect Git history in your Codex root.

### 2. Monorepo re-rooting

You split or merge project trees and need session history to follow real paths without manual JSONL surgery.

### 3. Session triage at scale

Use `scan --json` in scripts/CI-style checks to detect orphan drift or summarize active session paths.

## Search Behavior (Power User Notes)

Conversation search is available from TUI via `Ctrl+F`.

- Query supports:
  - plain tokens
  - quoted phrases (`"exact phrase"`)
- Matching is case-insensitive substring matching over normalized message text.
- Multiple clauses are ANDed (session must match all clauses).
- Results drive both source-tree pruning and session-list filtering.
- Opening a hit session jumps to and highlights matched conversation offsets.

Index details:

- SQLite sidecar index at `~/.codecom/search/index-v1.sqlite`
- Built/rebuilt in background at startup and after `F5` refresh
- Session JSONL and Codex SQLite state are not modified by search

## Data Model and Rewrite Scope

During move, `codecom` rewrites:

- JSONL:
  - `session_meta.payload.cwd`
  - all `turn_context.payload.cwd`
- SQLite (`state_5.sqlite`, `threads` table):
  - `cwd`
  - `rollout_path` (when present)

This dual rewrite keeps local file history and local thread index aligned for resume/discovery behavior.

## Operational Patterns That Actually Work

### Pattern A: dry-run with `scan` first

Run `scan --json` and sanity-check counts/orphans before opening TUI. This catches bad assumptions early.

### Pattern B: small controlled batches

Move a constrained folder subset first, inspect commits, then scale up. Faster rollback, clearer blame.

### Pattern C: keep `.codex` in Git

Without Git, you lose one of the main guarantees: auditable, per-session commits with deterministic rollback paths.

### Pattern D: refresh after external changes

If another process modifies sessions while TUI is open, use `F5` to rescan and rebuild search index state.

## Caveats and Current Limits

- No copy/fork semantics in v1.
- Hidden directories are excluded from browsing.
- Target pane is directories-only.
- Move requires destination path to pre-exist (no auto-mkdir).
- Malformed JSONL lines are tolerated and warned, not repaired.
- `scan` exits non-zero only on fatal failures, not routine warnings.
- Undo and clipboard-report keybindings (`U`, `Y`) are present in UI help, but currently status-only placeholders.
- Search index location is currently home-based (`~/.codecom/search/...`) rather than `--codex-dir` scoped.

## Installation and Running

From source:

```bash
go build ./cmd/codecom
./codecom scan --ascii
./codecom
```

Override Codex root:

```bash
./codecom tui --codex-dir /path/to/.codex
./codecom scan --json --codex-dir /path/to/.codex
```

Run tests:

```bash
go test ./...
```

## Output Contracts

### `scan --json`

One JSON object per line with fields:

- `session_id`
- `session_file`
- `cwd`
- `orphan`
- `total_tokens` (optional)
- `last_tokens` (optional)
- `warnings_count`

### `scan --ascii`

Tabular rows with:

- `SESSION_ID`
- truncated `CWD`
- `ORPHAN`
- `LAST` tokens
- `TOTAL` tokens
- `FILE`

## Design Philosophy

Minimal scope, strict semantics, boringly predictable behavior.

Plenty of tools can promise “smart migration.”  
`codecom` is optimized for “I can explain exactly what it changed, and why, to another engineer in under two minutes.”
