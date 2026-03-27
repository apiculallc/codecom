# codecom Agent Instructions

## Mission
Build `codecom`, a terminal-first Go app for safe Codex session management:
- fast discovery (`scan`)
- safe batch `cwd` migration (`tui`)

## Ground Rules
- Keep scope to v1 decisions in `docs/architecture.md`.
- Prefer simple, surgical changes.
- Do not add speculative features.
- Preserve behavior predictability over cleverness.

## Commands In Scope
- `codecom` -> default `tui`
- `codecom tui`
- `codecom scan --json`
- `codecom scan --ascii`

## Safety Requirements (Must Not Break)
- No write actions in `scan`.
- TUI write path only via explicit move flow.
- Move must validate all selected sessions before any write.
- Batch move fails fully if any selected row is invalid.
- Preflight dirty snapshot commit required before move writes.
- One commit per changed session file.
- Undo targets only codecom cwd-change commits.

## Move Semantics (Fixed)
- Source root: left panel folder.
- Target root: right panel folder.
- Remap rule: preserve relative suffix from source root.
- Target paths must already exist on OS.
- Rewrite both:
  - `session_meta.payload.cwd`
  - all `turn_context.payload.cwd`

## Parsing And Scan Behavior
- Scan `sessions/YYYY/MM/DD/*.jsonl`.
- Stream parse lines; do not unmarshal full files.
- Skip malformed lines, keep session record, log warnings.
- Compute orphan status for all discovered sessions.
- Orphan means `cwd` path does not exist.

## UI Requirements
- Two folder panels (source/target) + bottom session pane.
- Bottom pane shows sessions for current source folder only.
- Include select-all action for current source folder.
- Show orphaned items in red.
- Show known session folder counts as `[N]`, hide `0` and `1`.
- Provide refresh (`F5`) and move (`F6`).

## Output Contract
- `scan --json`: JSONL data to stdout only.
- `scan --ascii`: table to stdout only.
- Version header and diagnostics to stderr.

## Non-Goals (v1)
- No session copy/fork support.
- No “orphan suggestion” heuristics.
- No MCP server module.
- No hidden directory browsing mode.

## Verification Checklist
1. `scan --json` emits valid JSONL records with required fields.
2. `scan --ascii` renders flat session rows with path truncation style.
3. TUI loads without writes and shows source/target/session panes.
4. Move performs full validation and blocks on any invalid mapping.
5. Move updates both cwd locations in JSONL files.
6. Git commits are created as specified (snapshot + per-file move).
7. Undo only affects last codecom cwd-change commit.

