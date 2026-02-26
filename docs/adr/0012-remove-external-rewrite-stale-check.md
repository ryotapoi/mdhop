# ADR 0012: Remove stale check for external rewrite targets in move

## Status

Accepted

## Context

`mdhop move` rewrites links in external files (incoming and collateral rewrites) when a file is moved. Previously, all external rewrite target files were checked for stale mtime (DB mtime vs disk mtime) before proceeding.

Obsidian and iCloud sync frequently touch file mtimes without changing content. This caused `mdhop move` to fail with stale errors during consecutive moves, because external files rewritten by a previous move would have their mtime updated by Obsidian between operations.

## Considered Options

- **Keep stale check**: Safe but blocks consecutive moves in Obsidian environments. Users must run `build` between every move.
- **Remove stale check**: Allows consecutive moves. If an external file was genuinely edited (line offsets shifted), the string replacement silently no-ops but DB edges are updated — a minor inconsistency recoverable by `build`.
- **Remove stale check + return replacement success**: `applyFileRewrites` would report whether each replacement actually matched, and DB updates would be skipped on miss. More robust but expands scope significantly.

## Decision

We will remove the stale check for external rewrite targets (incoming and collateral) in both `Move` and `MoveDir`. The moved file's own stale check is preserved.

## Consequences

- Consecutive moves work without intermediate `build` in Obsidian environments.
- If an external file's content is genuinely edited between `build` and `move` (line offsets shift), the link replacement may no-op while the DB edge is updated with the new raw_link. This is recoverable by running `build`.
- The moved file's own stale check still catches the most important case (user edited the file being moved).
- Option C (replacement success tracking) remains available as a future enhancement if the DB inconsistency proves problematic in practice.
