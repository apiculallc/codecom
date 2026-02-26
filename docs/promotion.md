# codecom Launch Copy And Promotion

## Product Positioning
- Headline: `codecom: safe Codex session migration for moved repos`
- One-liner: `A terminal-first tool to scan, navigate, and safely move Codex sessions by rewriting cwd with Git-backed auditability.`
- Category: developer tooling, local-first workflow maintenance.

## Primary Audience
- Developers who frequently move/rename repos.
- Developers working in large monorepos with many project roots.

## Core Promise
- Find sessions fast across project folders.
- Batch move sessions to new project roots using deterministic path mapping.
- Keep changes auditable and reversible via Git commits in `.codex`.

## Key Messages
- Safety first: no writes unless user runs TUI move flow.
- Mandatory confirmations for risky operations.
- One move commit per changed session file for granular revert.
- Local workflow: scan data on disk; session discovery is fast and practical.

## Messaging Pillars
- Trust: explicit confirmation, strict validation, transparent commit metadata.
- Productivity: two-panel source/target flow, select-all, live recompute.
- Practicality: single binary, Linux/macOS terminal UX, JSONL and ASCII scan outputs.

## Launch Assets
- One animated GIF:
  1. scan sessions
  2. two-panel navigation
  3. batch move with confirmation
  4. resulting Git commits
- Release notes with:
  - supported commands (`tui`, `scan`)
  - safety model
  - known limitations and non-goals

## Suggested HN Post Draft
- Title: `Show HN: codecom – safe Codex session migration for moved repos`
- Body:
  - Problem: after repo moves, Codex session organization and cwd mapping become hard to manage.
  - What codecom does: scan sessions, browse source/target trees, batch move cwd safely.
  - Safety model: confirmation gates, strict validation, Git audit trail in `.codex`.
  - Scope: terminal-first, local session data workflows.
  - Ask: feedback from developers with heavy repo churn and monorepo setups.

## Website Copy Blocks
- Hero:
  - `Move Codex sessions safely when your repos move.`
  - `codecom gives you a two-panel terminal workflow with Git-backed session cwd migration.`
- Benefits:
  - `Discover sessions by real folder structure`
  - `Batch move with strict validation`
  - `Audit and revert with granular commits`
- CTA:
  - `Install`
  - `Watch Demo`
  - `Run First Scan`

## Success Signals
- Primary: GitHub stars.
- Secondary: quality user feedback (issues/discussions with concrete workflows).

