# Changelog

This changelog was reconstructed from the project's [GitHub Releases](https://github.com/ryotapoi/mdhop/releases), Git tags, commit history, and completed version groups in the backlog. Versions without a GitHub Release are recorded here even when no tag was created.

## [Unreleased]

## [v0.20.0] - 2026-10-04

### Added

- Indexed Markdown reference links in full, collapsed, and shortcut forms, including image references. Defined links now appear in outgoing links, backlinks, two-hop results, reachable notes, and graph output; unresolved destinations become phantom nodes. `resolve --from ... --link ...` reads the indexed reference occurrence.

### Fixed

- Preserved balanced parentheses in Markdown self-link fragments during `convert`, including round trips.
- Restored original modification times as well as contents and permissions when a failed `move` rolls back rewritten files, so rollback alone does not trigger a stale-index error on retry.

### Changed

- Detached returned head lines from the full note buffer, reducing retained memory when a search returns heads from large notes.
- Protected reference definitions from changes in meaning during `add` and `move`: operations that would redirect or make an indexed reference ambiguous now fail before changing files or the index. Reference definitions are not rewritten automatically.

### Upgrade

- Run `mdhop build` for each existing vault after upgrading. The index schema now stores reference destinations; older indexes cannot be read or updated until rebuilt.

## [v0.19.6] - 2026-10-02

### Changed

- Simplified redundant tests and CLI output-capture setup while retaining coverage of distinct behavior. CLI behavior is unchanged.

### Documentation

- Aligned command help, specifications, requirements, and internal command maps with the v0.19 fixes, including positional arguments, snippet errors, frontmatter links, and write safety.

## [v0.19.5] - 2026-10-02

### Fixed

- Preserved tags between a wikilink and a later Markdown link on the same line, and applied tag exclusions to two-hop targets from tag and phantom starting points.
- Parsed `--where` expressions using the first operator so values can contain `=` or `!=`; preserved trailing spaces in LIKE patterns.
- Reported each invalid frontmatter list value in `meta-validate`, returned an error for snippet locations beyond the current source, and kept long-decimal numeric sorting and comparisons accurate. Run `mdhop build` after upgrading an existing vault index: the stored number sort format changed.
- Saved preset types when an existing `mdhop.yaml` has `meta.types: null`; rejected unexpected positional arguments in commands; returned an error when `init-meta` could not write YAML to stdout.

### Documentation

- Clarified that `simplify` rewrites quoted frontmatter wikilinks, but leaves raw `frontmatter_path` values and tags unchanged.

## [v0.19.4] - 2026-10-01

### Fixed

- Ignored links and tags inside inline code delimited by multiple backticks, and resolved Unicode path links consistently during build and later lookup.
- Prevented `add` and `move` from promoting unrelated phantom path links with the same basename.
- Preserved padded relative Markdown links, case-insensitive relative targets, and existing references through case-only basename collisions when moving notes.
- Kept asset links intact during `disambiguate --scan` and link conversion; recognized dotted note basenames during scanning and preserved note links containing parentheses after conversion.
- Reported the actual destination of a completed template move in text and JSON output.

## [v0.19.3] - 2026-10-01

### Fixed

- Isolated temporary databases for concurrent builds and temporary configuration files for `init-meta --write`, preventing runs from interfering with one another or existing `.tmp` files.
- Rejected vault-escaping `add` paths, Markdown writes through external symlinks, move destinations outside the vault, and directory moves that would overwrite unregistered assets.
- Resolved normalized Unicode paths to on-disk files for content queries and diagnostics, and used the actual disk path when deleting NFD-named files.
- Preserved other keys in flow-style frontmatter when using `set`, and kept quoted YAML keys valid and adjacent comments or blank lines intact.
- Refreshed frontmatter metadata and index entries after link rewrites, and pointed disambiguated phantom edges at their real targets without requiring a rebuild.

## [v0.19.2] - 2026-10-01

### Changed

- Simplified overlapping SQL and formatter tests and shared internal node-upsert and stale-check logic. CLI behavior is unchanged.
- Made CLI test output capture drain stdout and stderr concurrently to avoid hanging on large output. CLI behavior is unchanged.

## [v0.19.1] - 2026-09-24

### Fixed

- Rollback now attempts to restore the file whose write failed in `set`, rewrite, and move operations.
- `add` ambiguity errors now identify the conflicting basename.
- Content queries now distinguish missing files from other filesystem stat errors.
- `delete --rm` now prevents removal outside the vault when a parent path traverses a symlink.

### Changed

- Raised the minimum Go version from 1.22 to 1.27.1.

### Documentation

- Clarified the source-relative link preservation exception for `move` in the requirements and command overview.

## [v0.19.0] - 2026-09-23

### Added

- Added `set --list <json-array>` to replace a frontmatter key with a complete string list while updating the index. List order, duplicates, empty strings, and empty lists are preserved.

## [v0.18.0] - 2026-09-21

### Added

- Added `status` to list untracked, modified, and deleted note and asset paths without changing the vault or index.
- Added `query --link-key` to restrict direct backlinks and outgoing links to links recorded under the specified frontmatter key.
- Added 1-based locations to `meta-check` and `meta-validate` diagnostics for direct navigation to the reported issue.

### Fixed

- Excluded non-HTTP external URIs from internal-link indexing.
- Corrected code-fence parsing to respect fence marker and length, so enclosed links, tags, and headings are not parsed.
- Made text and DOT output write failures cause a nonzero CLI exit.
- Allowed `meta-check` to resolve existing files excluded from the build.
- Made build temporary-database cleanup failures preserve the existing index instead of proceeding.

### Changed

- Refactored internal build, query, move, and output paths and stabilized focused regression coverage; no CLI contract change is intended.

### Documentation

- Aligned the `resolve` asset output contract and refreshed current command documentation and test plans.

## [v0.17.1] - 2026-09-14

### Added

- Added `meta-check --kind auto` to validate mixed frontmatter references in one pass. Values are classified as paths or wikilinks individually, while URLs and empty values remain allowed.

### Changed

- Aligned frontmatter wikilink extraction with Obsidian property links. Only wikilinks in quoted YAML scalar or list-item values are indexed; bare values and block scalar contents no longer create frontmatter link edges, phantom nodes, or rewrite targets.

### Fixed

- Validated all frontmatter rewrite candidates before writing files, updating the database, or moving paths. Unsupported YAML decode/source mappings now fail without partial changes, and dry-run reports the same validation result as execution.
- Directory `delete --rm` now reports unexpected failures while removing unregistered assets or empty directories as errors instead of reporting success after partial completion. Normal missing-file and non-empty-directory handling is preserved.

### Documentation

- Synchronized command specifications, the test plan, and the example agent skill with the current CLI behavior, keeping detailed command usage in `mdhop <command> --help`.

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
