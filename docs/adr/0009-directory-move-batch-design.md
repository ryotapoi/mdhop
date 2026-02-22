# ADR 0009: Directory move as independent batch function

## Status

Accepted

## Context

When moving a directory (e.g., `sub/` to `newdir/`), all `.md` files under it must be relocated. A naive approach would loop over each file and call the existing `Move()` function sequentially. However, this causes a fundamental problem: each intermediate move triggers link rewriting against the current state, but the basename ambiguity landscape changes with every step. A basename that is unique after moving file 1 might become ambiguous after moving file 2, leading to incorrect or inconsistent rewrites.

The core constraint is that link rewriting must be computed against the **final state** — after all files have been moved — not against intermediate states.

## Considered Options

- **Option A: Sequential `Move()` calls**: Loop through files and call `Move()` for each. Simple to implement but produces incorrect rewrites due to intermediate-state ambiguity.
- **Option B: Independent `MoveDir()` function**: A new function that collects all moves first, computes the post-move state once, then performs all rewrites and disk operations atomically. Shares low-level helpers with `Move()` but has its own control flow.
- **Option C: Refactor `Move()` to accept batches**: Generalize `Move()` to handle multiple files simultaneously. Would require significant restructuring of the existing function and could introduce regressions.

## Decision

We will implement `MoveDir()` as an independent function (Option B). It shares low-level helpers (`rewriteRawLink`, `applyFileRewrites`, `buildMapsFromDB`, etc.) with `Move()` but has its own 6-phase algorithm that handles batch operations atomically.

For directory delete, we use CLI-layer expansion only — the existing `Delete()` already supports multiple files, so no core-layer changes are needed.

## Consequences

- Correct link rewriting: all rewrites are computed against the final post-move state, eliminating intermediate-state ambiguity issues.
- Code duplication: `MoveDir()` and `Move()` share similar phases (incoming rewrite, collateral, outgoing, disk ops, DB transaction) but cannot be trivially unified due to batch-specific complexity (moved-set-internal links, batch rollback).
- Maintenance cost: bug fixes in rewrite logic may need to be applied to both `Move()` and `MoveDir()`. The shared helpers mitigate this partially.
- Simpler delete: directory delete requires no core changes, keeping the blast radius small.
