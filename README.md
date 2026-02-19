# mdhop

A CLI tool that indexes link relationships in Markdown repositories into SQLite. It parses wikilinks, markdown links, tags, and frontmatter in Obsidian Vault-compatible directories, enabling fast navigation to related notes without relying on grep. Designed for both Coding Agents (Claude Code, Codex, etc.) and CLI users.

[日本語版 README](README.ja.md)

## Features

- **Pre-indexed, instant responses** — Indexes the entire vault into SQLite. Queries return in milliseconds
- **Backlinks / 2-Hop Links / Tags** — Retrieve related information from any starting note in a single call
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
| `delete` | Remove files from the index |
| `move` | Reflect file moves and update links |
| `disambiguate` | Rewrite ambiguous basename links to full paths |
| `resolve` | Resolve a link to its target |
| `query` | Return Backlinks / 2-Hop / Tags etc. for a node |
| `stats` | Show vault statistics (note count, link count, etc.) |
| `diagnose` | Detect basename conflicts and phantom nodes |

Common options: `--vault <path>` (defaults to current directory), `--format json|text`, `--fields <comma-separated>`

Run `mdhop <command> --help` for command-specific details.

## Configuration (mdhop.yaml)

Place `mdhop.yaml` at the vault root to configure exclusion patterns for build and query.

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
```

## Documentation

- [Command specification and behavior](docs/external/overview.md)
- [Use cases and workflows](docs/external/stories.md)
- [Design concepts](docs/architecture/01-concept.md)
- [Data model](docs/architecture/03-data-model.md)

## License

[MIT License](LICENSE)
