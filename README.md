# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

A CLI tool that indexes link relationships in Markdown repositories into SQLite. It parses wikilinks, markdown links, tags, and frontmatter in Obsidian Vault-compatible directories, enabling fast navigation to related notes without relying on grep. Designed for both Coding Agents (Claude Code, Codex, etc.) and CLI users.

[日本語版 README](README.ja.md)

## Features

- **Pre-indexed, instant responses** — Indexes the entire vault into SQLite. Queries return in milliseconds
- **Backlinks / Two-Hop Links / Tags** — Retrieve related information from any starting note in a single call
- **Wikilink / Markdown link / Tag / Frontmatter support** — Obsidian-compatible link parsing
- **Fully local** — No external services required. Pure Go + SQLite
- **Optimized for Coding Agents** — `--fields` and `--include-snippet` return only the minimal context needed

## Installation

```bash
go install github.com/ryotapoi/mdhop/cmd/mdhop@latest
```

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
| `query` | Return Backlinks / Two-Hop / Tags etc. for a node |
| `search` | Find notes vault-wide by frontmatter metadata, path, or isolation filters |
| `reachable` | List notes reachable / unreachable from an entry note via links |
| `graph` | Export the link graph as JSON or Graphviz dot |
| `stats` | Show vault statistics (note count, link count, etc.) |
| `diagnose` | Detect basename conflicts, phantom nodes, and broken heading anchors |
| `meta-check` | Check that frontmatter path/wikilink values resolve to real targets |
| `meta-validate` | Check frontmatter against required keys, profiles, and declared `meta.types` |
| `init-meta` | Generate frontmatter type declarations for `mdhop.yaml` |

Common options: `--vault <path>` (defaults to current directory), `--format json|text`, `--fields <comma-separated>`

Run `mdhop <command> --help` for command-specific details.

## Agent Skill Example

An up-to-date Codex/Claude-style skill is available under [`examples/skills/mdhop`](examples/skills/mdhop). It is a thin agent entry point for choosing the right command and then relying on `mdhop <command> --help` for exact flags, output fields, and examples.

```bash
mdhop stats --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
```

## Configuration (mdhop.yaml)

Place `mdhop.yaml` at the vault root to configure exclusion patterns for build and query, and frontmatter handling.

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

meta:
  link_keys:        # frontmatter keys whose raw path values become link edges
    - related
    - sources
```

## Documentation

- [Command specification and behavior](docs/specs/overview.md)
- [Use cases and workflows](docs/specs/stories.md)
- [Design concepts](docs/rules/01-concept.md)
- [Data model](docs/rules/03-data-model.md)

## License

[MIT License](LICENSE)
