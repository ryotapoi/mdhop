# ADR 0006: Config file and query exclude filter

## Status

Accepted

## Context

AI coding agents using `mdhop query` receive noisy results from daily notes, templates, and broad tags (e.g. `#daily`) that connect many unrelated notes via twohop. Users need a way to exclude irrelevant paths and tags from query results. This requires both CLI flags for ad-hoc exclusion and a config file for persistent defaults.

Key constraints:
- No external CLI framework (stdlib `flag` only)
- `yaml.v3` is already an indirect dependency
- SQLite GLOB is built-in and supports `*` matching `/`
- Exclusion should happen at the SQL level (not Go post-filtering) for correct interaction with LIMIT

## Considered Options

- **YAML config at vault root (`mdhop.yaml`)**: Simple, discoverable, consistent with `.mdhop/` data dir separation
- **Config inside `.mdhop/` directory**: Keeps config with data, but less discoverable and `.mdhop/` is meant for generated data
- **TOML or JSON config**: TOML requires a new dependency; JSON lacks comments and is less human-friendly
- **Go post-filter instead of SQL WHERE**: Simpler code but breaks LIMIT semantics (would return fewer results than requested)
- **`[...]` character class support in glob**: SQLite GLOB supports it but adds complexity to the Go-side `globMatch` helper for negligible practical benefit

## Decision

We will use `mdhop.yaml` at the vault root as the config file (YAML format, leveraging existing `yaml.v3` dependency). The config supports an `exclude` section with `paths` (glob patterns) and `tags` (exact match, case-insensitive).

Exclusion applies to `query` only (not `stats` or `diagnose`). Filters are applied via SQL WHERE clauses with parameterized queries. The `*` glob matches any character including `/` (SQLite GLOB semantics). Character class `[...]` is not supported (patterns containing `[` produce an error).

CLI flags `--exclude`, `--exclude-tag` (repeatable) and `--no-exclude` are provided. Config and CLI exclusions are merged.

## Consequences

- **Positive**: AI agents get cleaner query results by default; users can configure once in `mdhop.yaml`
- **Positive**: No new dependencies (`yaml.v3` promoted from indirect to direct)
- **Positive**: SQL-level filtering preserves LIMIT correctness
- **Negative**: `mdhop.yaml` is a new file users must know about (mitigated: optional, zero-config by default)
- **Negative**: `[...]` glob character classes are not supported; a future implementation would need to update both SQL and Go glob matching
- **Neutral**: Config file format is committed to YAML; changing later would require migration
