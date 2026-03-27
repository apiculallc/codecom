# Codex Session Path Migration Format

## Goal
Migrate Codex session history from one folder prefix to another (for example, `/home/bofh/...` to `/home/devel/...`) so `codex resume` can discover sessions correctly.

## Key Fact
`codex resume` uses local state only:
- Rollout JSONL files under `~/.codex/sessions/YYYY/MM/DD/*.jsonl`
- Local SQLite index at `~/.codex/state_5.sqlite`

It is not reading resume history from a remote API.

## Required Inputs
- `old_prefix`: source path prefix (example: `/home/bofh`)
- `new_prefix`: target path prefix (example: `/home/devel`)
- `codex_home`: usually `~/.codex` (or `$CODEX_HOME` if set)

## JSONL Rewrite Rules
For each rollout file:
1. Rewrite `session_meta.payload.cwd` from `old_prefix` to `new_prefix`.
2. Rewrite every `turn_context.payload.cwd` from `old_prefix` to `new_prefix`.
3. Keep JSONL valid (one valid JSON object per line).

Notes:
- Files with only bootstrap metadata and no real user-event history are weak resume candidates.
- Resume listing logic expects sessions to have a non-empty first user message in index metadata.

## SQLite Rewrite Rules
Update the local `threads` table so index data matches migrated files.

Typical fields to update:
- `cwd`
- `rollout_path` (only if file paths moved)

Template SQL:

```sql
UPDATE threads
SET cwd = REPLACE(cwd, :old_prefix, :new_prefix)
WHERE cwd LIKE :old_prefix || '%';

UPDATE threads
SET rollout_path = REPLACE(rollout_path, :old_prefix, :new_prefix)
WHERE rollout_path LIKE :old_prefix || '%';
```

Practical one-liner example:

```sql
UPDATE threads
SET cwd = REPLACE(cwd, '/home/bofh', '/home/devel')
WHERE cwd LIKE '/home/bofh/%';
```

## Validation Queries
Check migrated rows:

```sql
SELECT id, cwd, rollout_path
FROM threads
WHERE cwd LIKE '/home/devel/%'
ORDER BY updated_at DESC
LIMIT 50;
```

Check remaining old paths:

```sql
SELECT id, cwd, rollout_path
FROM threads
WHERE cwd LIKE '/home/bofh/%'
   OR rollout_path LIKE '/home/bofh/%'
ORDER BY updated_at DESC;
```

## Resume Behavior Constraints
- Default `codex resume` filters to current `cwd`.
- `codex resume --all` disables `cwd` filtering.
- Only interactive sources are listed (`cli`, `vscode`).
- Sessions with empty `first_user_message` are excluded by DB-backed listing.

## Migration Checklist
1. Rewrite JSONL `cwd` fields.
2. Rewrite SQLite `threads.cwd` (and `threads.rollout_path` if needed).
3. Verify no old prefix remains in both JSONL and SQLite.
4. Run `codex resume --all` to confirm visibility.
5. Run `codex resume` inside target folder to confirm `cwd`-filtered visibility.
