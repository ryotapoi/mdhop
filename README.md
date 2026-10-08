# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

A CLI tool that indexes link relationships in Markdown repositories into SQLite. It parses wikilinks, markdown links, tags, and frontmatter in Obsidian Vault-compatible directories, enabling fast navigation to related notes without relying on grep. Designed for both Coding Agents (Claude Code, Codex, etc.) and CLI users.

[日本語版 README](README.ja.md) · [Changelog](CHANGELOG.md) · [日本語の変更履歴](CHANGELOG.ja.md)

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

After upgrading an existing vault to v0.20.0 or v0.21.0, run `mdhop build`. v0.20.0 added stored reference link destinations and refreshed the number sort format changed in v0.19.5. v0.21.0 adds stored table context and an index interpretation version; older indexes require a rebuild.

## Quick Start

```bash
# Navigate to your vault directory
cd /path/to/vault

# Build the index (.mdhop/index.sqlite is created)
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
| `init-meta` | Generate frontmatter type declarations for `mdhop.yaml` |

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

## Configuration (mdhop.yaml)

Place `mdhop.yaml` at the vault root to configure build exclusions, query hide / via selection, search exclusions, and frontmatter handling.

```yaml
build:
  exclude_paths:
    - "daily/*"
    - "templates/*"

exclude:
  paths:
    - "daily/*"
  tags:
    - "#daily"

query:
  hide:
    paths: ["archive/*"]
  via:
    exclude: {paths: [], tags: []}

meta:
  link_keys:        # frontmatter keys whose raw path values become link edges
    - related
    - sources
```

## Migrating to v0.21.0

- Query removes `--fields`. Select relationships with `--relations backlinks,outgoing,twohop`; read entry tags / metadata / head with `mdhop inspect --file Notes/Design.md --include-head 5 --format json`. Query head belongs to returned notes; snippets show occurrences proving each relationship.
- `--max-backlinks`, `--max-twohop`, and `--max-via-per-target` are removed. The default returns all targets and all via nodes. For pagination, explicitly select one relation, for example `--relations backlinks --limit 20 --offset 20`. This is not a one-to-one replacement for the old caps. Removed flags have no aliases.
- Query removes `--exclude`, `--exclude-tag`, and `--no-exclude`. Use `--hide-path` / `--hide-tag` for display, `--via*` / `--exclude-via*` for twohop discovery, and `--no-config-hide` / `--no-config-via` to ignore configured conditions.
- Legacy config `exclude` falls back to via exclusion only when the **`query.via.exclude` key is absent**. The explicit empty value above disables fallback. It never supplies hide conditions; via include is independent. CLI conditions remain when no-config flags are used. Search retains its existing exclude / fields / head behavior.
- JSON now returns `entry`, selected `backlinks` / `outgoing` / `2hoplink` arrays, and `page`. Each twohop target has all `relation` nodes and `hidden_relation`. Update consumers of the old via→targets shape, standalone tags, or entry preview / metadata. Selected empty arrays and omitted relations differ.
- Tag / asset / phantom entries retain backlinks and have empty outgoing / twohop. Query outgoing includes indexed parent tags; inspect shows leaf tags.

Run `mdhop <command> --help` for flags and examples. This describes the v0.21.0 contract; it does not announce a published release or tag.

## Documentation

- [Purpose and requirements](docs/requirements.md)
- [Design decisions](decisions/)
- [Verification procedure](docs/verification.md)
- [Versioning and release procedure](docs/release.md)

## License

[MIT License](LICENSE)

Dependency copyright notices and license texts are in [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt). Include both files when redistributing binaries. See the [update and distribution procedure](docs/licensing.md).
