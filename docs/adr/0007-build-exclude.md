# ADR 0007: Build exclude paths

## Status

Accepted

## Context

While query exclusion (ADR 0006) filters results at query time, some files (daily notes, templates) should not be indexed at all. Including them in the DB creates unnecessary phantom connections and pollutes tag/edge counts. Users need a way to exclude files from the build itself.

Key constraints:
- `mdhop.yaml` already exists for config (ADR 0006)
- Build exclude and query exclude serve different purposes and must be independent
- Excluded files that are linked to should become phantom nodes (not silently dropped)
- `DisambiguateScan` shares file collection logic with `Build` and must respect the same exclusions
- No CLI flags — build exclusion is always config-driven (declarative, not per-invocation)

## Considered Options

- **Reuse `exclude.paths` for both build and query**: Simpler config but conflates two distinct concerns. A user may want to exclude daily notes from query results while still indexing them for completeness.
- **Separate `build.exclude_paths` key**: Independent control. Build exclusion removes files before indexing; query exclusion filters results after indexing.
- **`.mdhopignore` file (gitignore-style)**: More familiar syntax but adds a new file format and parser for minimal benefit.
- **CLI `--exclude` flag on build command**: Per-invocation exclusion is inconsistent with build's declarative nature.

## Decision

Add `build.exclude_paths` to `mdhop.yaml` as a list of glob patterns (same syntax as `exclude.paths`: `*` matches any character including `/`, `[` not supported). Files matching any pattern are filtered out after `collectMarkdownFiles()` and before basename counting and link parsing.

Links pointing to excluded files resolve to phantom nodes (same as links to non-existent files). Tags inside excluded files are not indexed. `DisambiguateScan` applies the same exclusion to ensure consistency with `Build`.

Mutation commands (`add`, `update`, `delete`, `move`) do not reference `build.exclude_paths` — they operate on DB state. Inconsistencies (e.g., `add --file daily/D.md`) are resolved on next `build`.

## Consequences

- **Positive**: Excluded files don't pollute the index, reducing noise in tags, edges, and phantom counts
- **Positive**: Links to excluded files degrade gracefully to phantoms
- **Positive**: Config-driven approach is consistent with the declarative nature of `build`
- **Negative**: `Build` now reads `mdhop.yaml`, so an invalid YAML file causes build failure even without build-specific config (acceptable: query commands already fail on invalid YAML)
- **Negative**: Basename collisions between excluded and non-excluded files are invisible to mdhop (acceptable trade-off: excluded files are outside mdhop's scope)
