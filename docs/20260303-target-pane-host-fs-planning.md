# Add Real Host-Filesystem Browsing to the Target Pane

## Summary
Replace the current target-pane data source from “session-derived cwd list” to a real host-OS directory browser, rooted at the current user’s home directory by default.

This change keeps the left/source side session-driven, but makes the right/target side truthfully represent valid destination paths that already exist on disk. That aligns the TUI with the v1 move contract:

- source root comes from known session contexts
- target root must be an existing host OS path
- target browsing must work even when a folder has never appeared in Codex session context

## Goal
Make the right/target pane browse actual directories from the host filesystem so the user can choose valid destination roots outside the set of currently known session paths.

## Success Criteria
- The target pane displays real filesystem directories under the chosen starting root.
- The default starting root for the target pane is the current user’s home directory.
- Hidden directories remain excluded in v1, matching existing product decisions.
- Target navigation is scrollable and behaves like the rest of the TUI.
- `Left`/`Right`, `Tab`, `Up/Down`, `PgUp/PgDn`, `Home/End` continue to work with the new target tree.
- No write behavior is introduced.
- Source pane behavior remains session-derived for now.
- Full test suite passes with new target-pane unit tests.

## Chosen Defaults / Assumptions
- Default target browser root: current user’s home directory.
- Hidden directories: excluded, no toggle.
- Files: excluded; directories only.
- Symlinks: display only if the symlink target is a directory and `os.ReadDir` surfaces it as directory-like through `DirEntry.IsDir()`. Do not add custom symlink resolution logic in this phase.
- Source pane remains based on discovered session paths only.
- Target pane does not yet persist last browsed path in config.
- No special-case exclusions beyond hidden directories in this phase.

## Public / Internal Interface Changes

### `internal/tui`
Add explicit separation between source-tree data and target-filesystem data.

#### New internal types
- `type treeNode struct`
  - `Path string`
  - `Name string`
  - `Depth int`
  - `Expanded bool`
  - `HasChildren bool`
  - `KnownSessionCount int`
  - `Orphan bool`
- `type targetBrowser struct`
  - `Root string`
  - `Expanded map[string]struct{}`
  - `Visible []treeNode`

#### Model changes
Extend `tui.Model` with:
- target browser root path
- target expanded-directory state
- target visible nodes list
- optional cached known-session counts by absolute path prefix

Suggested fields:
- `targetRoot string`
- `targetExpanded map[string]struct{}`
- `targetNodes []treeNode`
- `knownSessionCounts map[string]int`

### `internal/app/run_tui.go`
No CLI/interface change required.
`RunTUI` still:
- ensures config
- scans sessions
- constructs `tui.Model`

But `tui.NewModel(...)` will now also derive:
- source session tree projection
- target host-filesystem tree rooted at home

### No CLI flag additions in this phase
Do not add `--target-root` or config-driven target root yet.

## Implementation Plan

### 1. Introduce a real tree projection abstraction in `internal/tui`
Create tree-building helpers inside `internal/tui` rather than overloading the current flat list logic.

Add files:
- `internal/tui/tree.go`
- optionally `internal/tui/fs_browser.go`

Responsibilities:
- Build source tree from session-derived cwd paths
- Build target tree from host filesystem directories
- Flatten tree nodes into visible rows based on expanded state

Keep this logic local to `internal/tui` for now. Do not create a reusable abstraction unless both panes genuinely share enough behavior after implementation.

### 2. Rework the source pane into a real source folder tree
Current source pane is a flat sorted path list. That is already below spec.

Change it to:
- represent the directory hierarchy implied by discovered session cwd paths
- show folders as nested nodes
- carry known-session counts
- preserve current source selection behavior
- bottom session pane still shows sessions under current selected source folder

Rules:
- alphabetical ordering
- count marker shown as `[N]` only for `N > 1`
- hide `[0]` and `[1]`
- orphan folders remain red if the selected/known path does not exist

Selection rule:
- current source root is the selected node’s absolute path

This is necessary because the target pane will become a true folder tree; keeping source flat would leave the two panes inconsistent.

### 3. Build the target pane from host OS directories
Replace `targetFolders []string` as the target-pane backing data.

Initialization:
- determine user home via `os.UserHomeDir()`
- target browser root = home
- initial visible tree should include the root node plus its first-level child directories
- root itself should be selectable

Directory loading rules:
- use `os.ReadDir(path)`
- include only directories
- exclude names beginning with `.`
- sort alphabetically by directory name
- do not prewalk the entire filesystem
- lazily materialize children when a node is expanded

Expansion model:
- `Right` on a collapsed directory expands it
- `Left` on an expanded directory collapses it
- `Left` on a collapsed child moves focus to its parent node
- `Right` on an already-expanded directory moves focus to first child if present
- root remains expand/collapse-safe

This gives MC-style tree navigation without having to build a whole recursive filesystem snapshot up front.

### 4. Keep top-pane focus semantics while refining arrow behavior
Current behavior:
- `Left` jumps to source pane
- `Right` jumps to target pane

Keep that cross-pane behavior only when the active pane is not already one of the top panes, or when a pane-switch intent is unambiguous.

Decision-complete behavior:
- If active pane is `Sessions`:
  - `Left` -> `Source`
  - `Right` -> `Target`
- If active pane is `Source`:
  - `Right` -> `Target`
  - `Left` -> no-op
- If active pane is `Target`:
  - `Left`:
    - first try tree-collapse/parent navigation inside target tree
    - if already at target root with no parent/collapse action available, switch to `Source`
  - `Right`:
    - expand or descend in target tree if possible
    - otherwise no-op

`Tab` order remains:
- `Source -> Sessions -> Target`

`Shift+Tab` reverse remains:
- `Source <- Sessions <- Target`

This keeps the recent pane-focus improvement while adding meaningful tree semantics in the target pane.

### 5. Add target known-session count overlay
Even though target is now real host filesystem, the spec says known session folder counts should be shown.

Overlay behavior:
- compute session-derived counts for known folders from scanned records
- when rendering target filesystem nodes:
  - if that exact absolute path is known and has `N > 1`, render `[N]`
  - otherwise render no count marker
- do not compute recursive subtree totals for arbitrary filesystem branches in this phase
- use exact-path counts only

This is the simplest interpretation consistent with existing behavior and avoids expensive prefix aggregation over the whole browsable filesystem.

### 6. Preserve sessions pane semantics
The bottom pane remains:
- sessions for current source folder only
- sorted as already defined by `sessionindex`
- selectable with `Space`
- `A` selects all visible/current-source sessions

No target-pane selection state is added.
Target selection is just the currently focused target directory node.

### 7. Update target root accessor for future move flow
Add an explicit accessor on the model:
- `CurrentTargetFolder() string`

Behavior:
- returns selected target node absolute path if valid
- returns empty string only if target tree is unexpectedly empty

This will be the path consumed later by `internal/move`.

### 8. Keep refresh behavior scoped for this phase
Do not fully implement `F5` end-to-end here unless necessary for the tree model.

However, the TUI model should be structured so refresh can later rebuild:
- source session tree
- target filesystem tree
- session pane projection

If any small refactor is needed to make rebuilding clean, do it now in plan scope:
- centralize projection building in helper constructors
- do not add background async scanning in this phase

## Rendering Details

### Source pane row format
- tree indentation with two spaces per depth
- current row marker via existing selected row style
- folder name
- count suffix `[N]` only for `N > 1`
- orphan rows red

Example:
- `repo [3]`
- `  app`
- `  docs [2]`

### Target pane row format
- tree indentation with two spaces per depth
- folder name only, plus optional known-session count if exact path matches scanned data and `N > 1`
- active row styled using existing target-pane selection style
- no orphan coloring in target pane for normal existing directories, because by definition they are read from the real filesystem and exist

### Sessions pane
No structural change beyond continuing to respond to the selected source folder node.

## Edge Cases / Failure Modes
- If `os.UserHomeDir()` fails:
  - fallback target root to `/`
- If target root directory cannot be read:
  - show target pane with the root node and an inline status message such as `target root unreadable`
  - do not crash
- If a directory becomes unreadable during browsing:
  - keep node visible
  - treat expansion as empty
  - update status bar with a short read failure message
- If source tree is empty:
  - target pane still works
  - sessions pane remains empty
- If target pane root contains many directories:
  - lazy expansion avoids loading the whole tree
- If long path names exceed pane width:
  - keep existing truncation behavior at render time

## Tests

### Unit tests for tree building
Add tests for:
- source tree projection from multiple nested session paths
- target tree visible-node flattening with expansion state
- hidden directories excluded from target nodes
- exact-path count marker logic (`N > 1` only)

### Model behavior tests
Add/adjust tests for:
- target pane initializes to home-root tree data
- `CurrentTargetFolder()` returns selected OS path
- `Tab` order remains `Source -> Sessions -> Target`
- `Right` from sessions jumps to target
- `Right` in target expands node or descends into child
- `Left` in target collapses or moves to parent
- `Left` from target root returns focus to source
- sessions pane still filters by current source node
- selection logic in sessions pane still works after source-tree conversion

### Rendering tests
Add view-level assertions for:
- source and target tree indentation appears
- target pane shows real folder names from temp-dir filesystem fixture
- footer and overall height remain bounded
- count markers appear only for `N > 1`

### Filesystem fixture tests
Use `t.TempDir()` to create a fake host tree such as:
- `home/projects/app1`
- `home/projects/app2`
- `home/archive`
- `.hidden` (must be excluded)

Build model against:
- session records under `/tmp/...` for source-tree tests
- explicit temp home root for target-tree tests

To keep this decision-complete, `NewModel` should accept an optional target-root override in tests via a new constructor:
- `NewModelWithTargetRoot(records []sessionindex.SessionRecord, targetRoot string) Model`

Production:
- `NewModel(records)` calls `NewModelWithTargetRoot(records, detectedHomeRoot)`

## Important Type / Constructor Additions
Add:
- `func NewModelWithTargetRoot(records []sessionindex.SessionRecord, targetRoot string) Model`
- `func (m Model) CurrentTargetFolder() string`

Internal helper additions:
- source tree builder
- target filesystem tree builder
- target expand/collapse helpers
- parent lookup helper for target tree nodes

## Explicit Non-Goals For This Phase
- No move implementation
- No refresh/rebuild command wiring beyond keeping the model ready for it
- No config option for target starting root
- No hidden-directory toggle
- No filesystem watcher
- No bookmarks/favorites/recent targets
- No recursive filesystem count aggregation
- No copy/fork behavior

## Acceptance Criteria
- Opening the TUI shows:
  - left pane: source folder tree derived from session contexts
  - right pane: actual host OS folder tree rooted at home
  - bottom pane: sessions for current source folder
- User can navigate target directories that never appeared in any session context.
- Target pane no longer depends on inferred session cwd values.
- TUI remains read-only.
- Existing navigation, scrolling, and selection features continue to work.
- `go test ./...` passes.
