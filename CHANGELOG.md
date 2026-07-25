# Changelog

This changelog was reconstructed from the project's [GitHub Releases](https://github.com/ryotapoi/mdhop/releases) and Git tags. Versions that have a tag but no GitHub Release entry are included from the tag-to-tag commit history.

## [Unreleased]

## [v0.16.6] - 2026-07-26

### Changed

- Command errors are now printed as `error: <subcommand>: <message>`. Individual error messages no longer carry their own command prefix; the subcommand name is attached in one place. Error text changed, but exit codes and successful output are unchanged.
- Aligned duplicated raw-string errors in `resolve` and `query` with their sentinel errors.
- Expanded regression coverage for `init-meta --write`, `meta-check` vault escapes, and diagnose formatter output; split move tests by responsibility. CLI behavior is unchanged.
- Refactored link-type SQL filtering, resolve output-field constants, and formatter normalization. CLI behavior is unchanged.

### Documentation

- Corrected the `diagnose` description in the concept and requirements documents: it reports neither parse failures nor exclusion counts, and the opt-in anchor check was missing.
- Stated the axis that separates core from mutate commands: whether a command rewrites Markdown notes in the vault.
- Finished renaming `reconcile` / `canonicalize` to `disambiguate` / `simplify` in the rules documents.
- Declared `docs/specs/overview.md` as the source of truth for command specifications, with CLI help as its summary.

## [v0.16.3] - 2026-07-12

### Changed

- Refined internal maintainability through focused refactoring and strengthened regression contracts; CLI behavior is unchanged.

## [v0.16.2] - 2026-07-12

### Fixed

- Improved rollback restoration and failure reporting for rewrite and `set` operations.
- Fixed wrapped template source lookup errors.

## [v0.16.1] - 2026-07-05

### Fixed

- Pinned the macOS GitHub Actions runner to `macos-15`, fixing release CI failures caused by the `macos-latest` migration. CLI behavior is unchanged.

## [v0.16.0] - 2026-07-05

### Changed

- Repeated `--where` flags for `query` and `search` are now always combined with AND, including repeated filters for the same metadata key.
- Use `||` inside a single `--where` expression for explicit OR semantics. Existing `!=` exclusion semantics are preserved.
- Updated command help, specifications, requirements, SQL-generation documentation, and regression coverage for the new filter rules.

This is a breaking change for commands that relied on the former implicit same-key OR behavior, such as `--where "status=active" --where "status=review"`.

## [v0.15.0] - 2026-07-05

### Added

- Added `set --date` for relative-date frontmatter writes and automatic frontmatter block creation.
- Added `||` expressions to `--where` filters.
- Completed `move --to-template` behavior, including dry-run planning, directory mode, date-part extraction, fallbacks, and placeholder path validation.

### Changed

- Explicit `meta-validate --require` values now override `meta.profiles` for that invocation.
- Hardened move execution through shared mover paths, rollback-failure reporting, and broader regression coverage.
- Refactored link resolution, resolve-map registration, utility boundaries, and output field constants.

## [v0.14.0] - 2026-07-04

### Added

- Added destination templates to `move`, including template-based file and directory destinations.

### Fixed

- Routed single-file moves through the shared mover and improved rollback-failure reporting.

## [v0.13.0] - 2026-07-04

### Added

- Added `set` for safe single-key frontmatter updates with index refresh.
- Added path-scoped `meta-validate` require profiles.
- Added `search` sampling, count-only output, and `coalesce(key1, key2, ...)` filters.

### Changed

- Centralized query default limits in core constants.
- Expanded per-command `--help` output with fields, behavior notes, and examples.
- Slimmed the example agent skill so exact command details live in `mdhop <command> --help`.

## [v0.12.1] - 2026-06-24

### Fixed

- Fixed Unicode-normalized path handling on Linux for NFD/NFC filenames.
- Fixed moved relative links that could be rewritten with a trailing `/`.

## [v0.12.0] - 2026-06-13

### Added

- Added `repair --path` and `--exclude` filters for selecting source notes.

### Changed

- Normalized indexed paths to NFC for consistent path resolution across filesystems.
- Allowed `meta-check` to accept directory paths.

## [v0.11.0] - 2026-06-11

### Added

- Added relative date values such as `today-90d` to `--where` comparisons.
- Added computed `search` fields and `meta.<key>` output selection.
- Added heading-anchor checking to `diagnose`.
- Added `meta-check` for frontmatter path and wikilink reference validation.
- Added `meta-validate` for frontmatter schema checks.

### Fixed

- Corrected anchor checking for inline-code headings and stale targets.
- Required date-declared keys for relative date comparisons.

## [v0.10.0] - 2026-06-11

### Added

- Added `--path` and `--exclude` source-note filters to `diagnose`.
- Added the `--path` result filter to `query`.
- Added `meta.link_keys` for indexing raw frontmatter path values as link edges.
- Added `reachable` for link reachability checks.
- Added `graph` for JSON and Graphviz subgraph export.

### Fixed

- Fixed asset collection when the vault path is the current directory.

## [v0.9.0] - 2026-06-10

### Changed

- Tightened search and path-filter behavior and aligned glob matching with SQLite GLOB semantics.
- Unified database-side basename resolution with the root-priority rule and improved diagnostics internals.
- Expanded CLI coverage and refreshed the example skill for existing commands.

This was primarily a maintenance and quality release; no new CLI command was introduced.

## [v0.8.0] - 2026-05-09

### Added

- Added `search` isolation filters: `--no-tags`, `--no-outgoing`, and `--no-incoming`.
- Added frontmatter `--where` `NOT EXISTS` filtering, for example `--where "priority NOT EXISTS"`.

### Fixed

- Tightened `search` filter behavior around existing notes and missing metadata.

## [v0.7.1] - 2026-05-09

### Changed

- Enabled Codex Goals in the project's agent workflow.

This tag contains agent-workflow changes only; CLI behavior is unchanged.

## [v0.7.0] - 2026-05-07

### Added

- Added frontmatter wikilink parsing and rewriting across `add`, `update`, `move`, `disambiguate`, and `simplify`.
- Added rendering of `claude -p` stream output as assistant text plus tool names.

### Fixed

- Ignored YAML comments while scanning frontmatter wikilinks.
- Preserved frontmatter wikilinks inside YAML block scalars and scoped block-scalar bodies to deeper-indented lines.
- Rewrote relative frontmatter wikilinks correctly when moving notes.
- Stopped `runner.sh` cleanly on Ctrl+C.

## [v0.6.1] - 2026-05-06

### Changed

- Improved internal type boundaries, query and formatting file organization, Go formatting automation, and agent workflow checks.

This tag contains maintenance and workflow changes only; no documented CLI behavior change was introduced.

## [v0.6.0] - 2026-03-22

### Added

- Added frontmatter metadata storage, metadata types, and sort-value normalization.
- Added `--where` filtering and `--fields meta` to `query`.
- Added `search` for entry-free vault-wide note search.
- Added `init-meta` for frontmatter type scaffolding.
- Added `&&` expressions for same-key AND filtering.
- Added candidate paths to ambiguous-link build errors.

## [v0.5.0] - 2026-03-18

### Changed

- Consolidated shared move, resolve, repair, simplify, database, tag, and node-update helpers.
- Expanded coverage for directory moves, collateral rewrites, asset resolution, phantom nodes, and frontmatter parsing.
- Reorganized the public documentation and agent workflow structure.

This was primarily an internal architecture and maintenance release; no new CLI command was introduced.

## [v0.4.0] - 2026-02-26

### Added

- Added `convert` for converting links between wikilink and Markdown formats.
- Added `simplify` for shortening redundant path links to basename form.

### Fixed

- Removed an external rewrite stale check that could block `move` and `movedir` operations.

## [v0.3.0] - 2026-02-26

### Added

- Added indexing and management of non-Markdown assets, including asset links, resolution, updates, moves, deletes, queries, and statistics.

## [v0.2.0] - 2026-02-25

### Added

- Added `repair` for broken path links and links that escape the vault.

## [v0.1.0] - 2026-02-23

### Added

- Initial public release of the Markdown link indexer and SQLite-backed CLI.
- Added `build`, `add`, `update`, `delete`, `move`, `disambiguate`, `resolve`, `query`, `stats`, and `diagnose`.
- Added strict link resolution with root-priority handling for ambiguous basenames and automatic disambiguation support.
- Added Obsidian-compatible tags, Unicode support, JSON/text output, vault configuration, index exclusion paths, and optional disk removal via `delete --rm`.
- Added directory move and directory delete support, collateral link rewriting, and Coding Agent example skills.
