# codecom Conversation Search Design (v1)

## Mission
Add conversation-content search to TUI so users can find sessions by chat text and navigate directly to matched messages.

## Product Flow
1. User presses `Ctrl+F`.
2. User types search terms (supports token terms and quoted phrases).
3. codecom searches conversation content (user + assistant messages).
4. Left pane is filtered to only branches/folders containing matching conversations, while keeping ancestor context.
5. User selects a matching node.
6. Sessions pane shows only matching sessions for the selected source folder.
7. User opens a session; conversation popup auto-scrolls to first hit and highlights all matched lines.

## Locked Decisions
- Dedicated key: `Ctrl+F` for conversation search mode.
- Existing `/` filter remains unchanged (path/name filter for source/target panes).
- v1 match semantics: case-insensitive token + quoted phrase matching.
- v1 scope indexed: user + assistant message text only.
- Tree behavior: keep ancestors + hit folders (pruned tree, not flat list).
- Sessions behavior in active conversation search: show matching sessions only.
- Popup behavior: jump to first hit + highlight all hits.
- Index update strategy: rebuild at startup and on `F5` refresh.
- Index build timing: background task after UI loads (non-blocking).

## Backend Choice
- Primary backend: SQLite sidecar index.
- Sidecar path: `~/.codecom/search/index-v1.sqlite`.
- Keep search implementation modular to allow future backends (fuzzy/stemming/other engines).

## Safety and Policy
- Existing move safety semantics must not change.
- `scan` remains read-only.
- Conversation search must not mutate:
  - session JSONL files
  - `~/.codex/state_5.sqlite`
- New non-move write allowance is explicitly limited to sidecar search artifacts under `~/.codecom/search/`.

## Architecture Additions

### New package
- `internal/search/`
  - `engine.go`: stable interfaces and result models
  - `query.go`: query normalization/parser (tokens + quoted phrases)
  - `sqlite_index.go`: sqlite index build/query implementation
  - `extract.go`: shared conversation text extraction from JSONL

### Interface Contract
Define a backend-facing interface that decouples TUI from storage implementation:
- `Build(ctx, sessions []sessionindex.SessionRecord) error`
- `Search(ctx, q Query) (Result, error)`
- `Close() error`

`Result` should include:
- matched session IDs
- matched folder paths (for left tree pruning)
- per-session line-offset hits (for popup jump/highlight)

### Data indexed per hit
- session id
- session file path
- source folder/cwd (effective cwd)
- line offset in JSONL file
- normalized message text
- speaker (`user`/`assistant`)
- timestamp (optional but useful for ordering/debug)

## TUI Integration

### Model state additions
- Conversation search mode state (`active`, `query`, status text).
- Search backend handle and readiness status (`building`, `ready`, `error`).
- Current search result cache:
  - matched folder set
  - matched session set
  - per-session match offsets

### Event handling
- `Ctrl+F` enters conversation search mode.
- Typing updates query buffer.
- Enter applies/keeps query.
- Esc clears conversation search query and exits search mode.
- `/` continues existing path filter behavior.

### Rendering behavior
- Left source tree is pruned by matched-folder set with ancestor retention.
- Sessions pane uses matched-session set for current source folder.
- Status bar shows index state and active query summary.

### Conversation popup behavior
- If opened from active query and session has hits:
  - initialize popup cursor/offset at first matching rendered line
  - apply highlight style to each matching rendered line
- If no hits for session under active query, fallback to current default popup behavior.

## Index Build and Refresh Lifecycle
- Startup:
  - TUI initializes quickly.
  - Background index build starts from scanned sessions.
  - Status reflects progress and completion/failure.
- `F5`:
  - Existing rescan runs.
  - Search index rebuild is triggered in background from fresh session list.
  - Active query is re-evaluated after rebuild.

## Implementation Constraints
- Keep change set surgical and v1-scoped.
- No speculative features in v1:
  - no stemming
  - no fuzzy ranking engine
  - no incremental watcher
  - no Lucene/Xapian integration yet
- Preserve existing keybindings and move flow behavior.

## Testing Checklist
1. Search query parser handles tokens, phrases, whitespace normalization.
2. SQLite sidecar index builds from representative JSONL conversation formats currently supported by popup parser.
3. Query results correctly map to:
   - matched sessions
   - matched folders
   - matched offsets
4. TUI `Ctrl+F` flow works end-to-end without breaking `/` filter mode.
5. Left tree pruning keeps ancestor context and hides non-matching branches.
6. Sessions pane shows matching-only sessions for selected source folder during active search.
7. Popup opens at first hit and highlights all hit lines.
8. Startup and `F5` trigger background (non-blocking) build/rebuild with status updates.
9. Search path does not modify session JSONL files.
10. Search path does not modify `~/.codex/state_5.sqlite`.
11. Sidecar files are written only under `~/.codecom/search/`.

## Acceptance Criteria
- User can find conversations by content from TUI via `Ctrl+F`.
- Left pane and sessions pane are filtered exactly by conversation matches.
- Opening a matched conversation lands user at first relevant message with visible highlighting.
- Existing move safety and scan behavior remain intact.

## Future Extensions (Post-v1)
- Add alternative `SearchBackend` implementations:
  - fuzzy matcher
  - stemming/linguistic FTS
  - external index engines
- Add ranking controls/sorting by relevance.
- Add incremental indexing if startup/rebuild latency requires it.
