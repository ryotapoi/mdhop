# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

A CLI tool that indexes link relationships in Markdown repositories into SQLite. It parses wikilinks, markdown links, tags, and frontmatter in Obsidian Vault-compatible directories, enabling fast navigation to related notes without relying on grep. Designed for both Coding Agents (Claude Code, Codex, etc.) and CLI users.

English · [日本語](README.ja.md)

## Features

- **Pre-indexed, instant responses** — Indexes the entire vault into SQLite. Queries return in milliseconds
- **Backlinks / Two-Hop Links / Tags** — Retrieve related information from any starting note in a single call
- **Wikilink / Markdown link / Tag / Frontmatter support** — Obsidian-compatible link parsing
- **Markdown reference links** — Resolve full, collapsed, and shortcut references using definitions in the same note; include them in indexed link queries
- **Fully local** — No external services required. Pure Go + SQLite
- **Optimized for Coding Agents** — `query --relations`, `inspect --fields`, and `--include-snippet` return only the minimal context needed

## Installation

```bash
go install github.com/ryotapoi/mdhop/cmd/mdhop@latest
```

For v0.22.0, run `mdhop migrate` to move legacy configuration and indexes to the new locations. See [Migrating legacy locations](#migrating-legacy-locations).

After upgrading an existing vault to v0.20.0 or v0.21.0, run `mdhop build`. v0.20.0 added stored reference link destinations and refreshed the number sort format changed in v0.19.5. v0.21.0 adds stored table context and an index interpretation version; older indexes require a rebuild.

## Quick Start

```bash
# Navigate to your vault directory
cd /path/to/vault

# Build the index (the index is created in the user cache)
mdhop build

# Get related information for a note
mdhop query --file Notes/Design.md

# Explore by tag
mdhop query --tag '#project'

# Resolve a link
mdhop resolve --from Notes/A.md --link '[[B]]'
```

## Commands

| Command | Description |
|---------|-------------|
| `build` | Parse the entire vault and create the index |
| `add` | Add new files to the index |
| `update` | Update existing files in the index |
| `set` | Set one frontmatter key or relative date and update the index |
| `delete` | Remove files from the index |
| `move` | Reflect file moves and update links, including frontmatter-based destination templates |
| `disambiguate` | Rewrite ambiguous basename links to full paths |
| `simplify` | Shorten redundant path links to basename form (inverse of disambiguate) |
| `convert` | Convert link format between wikilink and markdown |
| `repair` | Rewrite broken or vault-escaping path links to basename form |
| `resolve` | Resolve a link to its target |
| `query` | Return backlinks / outgoing / twohop for one node |
| `inspect` | Read indexed tags / metadata and optional head for one note |
| `search` | Find notes vault-wide by frontmatter metadata, path, or isolation filters |
| `reachable` | List notes reachable / unreachable from an entry note via links |
| `graph` | Export the link graph as JSON or Graphviz dot |
| `stats` | Show vault statistics (note count, link count, etc.) |
| `diagnose` | Detect basename conflicts, phantom nodes, and broken heading anchors |
| `status` | Compare the disk with the current index without syncing |
| `meta-check` | Check that frontmatter path/wikilink values resolve to real targets |
| `meta-validate` | Check frontmatter against required keys, profiles, and declared `meta.types` |
| `paths` | Show effective vault, configuration, and index locations |
| `migrate` | Migrate legacy YAML and vault-local index to default locations |
| `init-meta` | Generate frontmatter type declarations for `mdhop.toml` |

`--vault <path>` (defaults to the current directory) is common to commands. Output and field flags vary by command.

Run `mdhop <command> --help` for authoritative command-specific flags, output fields, and examples. Run `mdhop --version` to discover the installed version.

## Agent Skill Example

An up-to-date Codex/Claude-style skill is available under [`examples/skills/mdhop`](examples/skills/mdhop). It is a thin agent entry point for choosing the right command and then relying on `mdhop <command> --help` for exact flags, output fields, and examples.

```bash
mdhop stats --format json
mdhop status --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --relations backlinks,outgoing --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
```

## Vault, index, and configuration paths

Except for the dedicated `migrate` command, all commands that use an index or configuration accept `--vault`, `--db`, and `--config`. `--vault` defaults to the current directory; `--db` defaults to `<cache>/mdhop/vaults/<vault-hash>/index.sqlite`. Absolute paths and paths relative to the current directory are accepted. Vault is always the base for note paths, link interpretation, previews, output paths, and configuration globs; it is never inferred from the DB or configuration location.

`<cache>` uses an absolute, nonempty `XDG_CACHE_HOME`; unset, empty, or relative values fall back to `~/.cache` on every supported OS. `<vault-hash>` is the full lowercase SHA-256 of the absolute vault root after resolving symlinks. Relative and symlink aliases of one vault share its index; moving the vault selects a new index. Configuration does not affect this hash: use separate `--db` paths to keep indexes for different configurations.

Run `mdhop paths --vault <path>` (or add `--format json`) to see the effective absolute `vault`, `config`, and `db` paths, even before the config or DB exists. This command reads neither file and creates nothing. If the cache is deleted or the vault moves, run `mdhop build` again. Normal commands neither use nor migrate or delete the old `<vault>/.mdhop/` directory.

`--config` reads exactly the selected file, with any filename. Otherwise only `<vault>/mdhop.toml` is read; a missing default file means no configuration. Missing explicit files, read failures, and invalid configuration are errors, including when configuration filters are disabled. Files are neither discovered nor merged. `init-meta --write` updates the selected configuration file; an explicitly selected file must already exist.

```sh
mdhop build --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml
mdhop query --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml --file Index.md
```

With an external DB, build and reference commands require no write permission on the vault and create no index or temporary files there. Builds complete a private DB beside the selected DB and publish it only after commit and close. A failed rebuild preserves the previous index. A selected DB inside the vault is excluded from status inputs and directory note mutations; its path is retained when those notes are deleted or moved. `add` rejects the selected DB and its auxiliary files; delete and move reject an already indexed DB resource before changing files or the index, including template moves. On supported Ubuntu and macOS local filesystems, readers already connected retain the old completed index; readers starting after replacement see the new one. Read operations overlapping rebuild see completed generations. This does not promise one generation across separate CLI calls or a snapshot shared with changing note text; preview stale checks still apply. Serialize build with update and other index mutations: concurrent writers are unsupported. DB-free scan and configuration generation commands accept `--db` without opening it.

## Migrating legacy locations

Run `mdhop migrate` inside the vault, or `mdhop migrate --vault ./Notes`. Targets are fixed to the vault-root `mdhop.yaml`, `mdhop.toml`, `.mdhop/`, and that vault's default cache DB. `--db`, `--config`, and positional arguments are rejected.

With YAML, migrate converts and validates all settings, saves TOML, and fully rebuilds the index using it. With TOML only, it uses that file unchanged; with neither file, it uses defaults. Only after a successful rebuild does it delete legacy YAML and `.mdhop/`. The new index excludes the YAML asset and resolves links against the final file set. Success produces no stdout; warnings use stderr.

If YAML and TOML both exist, migration fails without changes. Preserve both, decide which configuration to use, and move one outside the vault before retrying. Unknown fields, invalid values, multiple YAML documents, and YAML merge keys are errors. Symlinks at the fixed targets and cache locations inside the directory scheduled for deletion are also rejected.

Conversion, save, or rebuild failures retain the old YAML, `.mdhop/`, and existing cache index; newly generated TOML is removed on failure. If rollback fails, the error reports the remaining TOML path: preserve the YAML, inspect and move the generated TOML aside, then retry. Cleanup failure is an error with the new cache already published and some old files possibly remaining. Inspect reported paths and fix permissions; if both configuration files remain, preserve and move the old YAML aside before retrying. Normal reference commands can use the new TOML and cache. Serialize migration with build, update, and other writes.

## Configuration (mdhop.toml)

Place `mdhop.toml` at the vault root to configure build exclusions, query hide / via selection, search exclusions, and frontmatter handling.

```toml
[build]
exclude_paths = ["daily/*", "templates/*"]

[exclude]
paths = ["daily/*"]
tags = ["#daily"]

[query.hide]
paths = ["archive/*"]

[query.via]
exclude = { paths = [], tags = [] }

[meta]
link_keys = ["related", "sources"]

[meta.types]
date = "date"
priority = { ordered = ["low", "high"] }

[[meta.profiles]]
path = "notes/*"
require = ["date"]
```

Missing or empty configuration uses defaults. Normal commands ignore the old `mdhop.yaml` and leave it unchanged. Notes continue to use YAML frontmatter.

## Migrating to v0.21.0

- Query removes `--fields`. Select relationships with `--relations backlinks,outgoing,twohop`; read entry tags / metadata / head with `mdhop inspect --file Notes/Design.md --include-head 5 --format json`. Query head belongs to returned notes; snippets show occurrences proving each relationship.
- `--max-backlinks`, `--max-twohop`, and `--max-via-per-target` are removed. The default returns all targets and all via nodes. For pagination, explicitly select one relation, for example `--relations backlinks --limit 20 --offset 20`. This is not a one-to-one replacement for the old caps. Removed flags have no aliases.
- Query removes `--exclude`, `--exclude-tag`, and `--no-exclude`. Use `--hide-path` / `--hide-tag` for display, `--via*` / `--exclude-via*` for twohop discovery, and `--no-config-hide` / `--no-config-via` to ignore configured conditions.
- Legacy config `exclude` falls back to via exclusion only when the **`query.via.exclude` key is absent**. The explicit empty value above disables fallback. It never supplies hide conditions; via include is independent. CLI conditions remain when no-config flags are used. Search retains its existing exclude / fields / head behavior.
- JSON now returns `entry`, selected `backlinks` / `outgoing` / `2hoplink` arrays, and `page`. Each twohop target has all `relation` nodes and `hidden_relation`. Update consumers of the old via→targets shape, standalone tags, or entry preview / metadata. Selected empty arrays and omitted relations differ.
- Tag / asset / phantom entries retain backlinks and have empty outgoing / twohop. Query outgoing includes indexed parent tags; inspect shows leaf tags.

Run `mdhop <command> --help` for flags and examples.

## Documentation

- [Changelog](CHANGELOG.md)
- [Purpose and requirements](docs/requirements.md)
- [Design decisions](decisions/)
- [Verification procedure](docs/verification.md)
- [Versioning and release procedure](docs/release.md)

## License

[MIT License](LICENSE)

Dependency copyright notices and license texts are in [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt). Include both files when redistributing binaries. See the [update and distribution procedure](docs/licensing.md).
