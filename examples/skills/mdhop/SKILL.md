---
name: mdhop
description: >
  Navigate, query, and manage Markdown vaults with mdhop. Explores link
  relationships (backlinks, outgoing, two-hop, tags), filters notes by
  frontmatter metadata (--where), resolves links, shows vault statistics,
  diagnoses issues, and manages file operations (add, move, delete, rename)
  with automatic link rewrites. Also handles link format conversion, broken
  link repair, and frontmatter metadata type setup (init-meta).
  Use this skill whenever: exploring connections between notes, querying
  backlinks or outgoing links, filtering notes by frontmatter fields (status,
  priority, dates), checking what a link resolves to, getting vault statistics,
  investigating vault health, adding/moving/deleting/renaming files, fixing
  broken links, converting link formats, or setting up metadata types. Even if
  the user doesn't mention "mdhop" by name, use this skill for any navigation
  or file operation in an Obsidian-style Markdown vault that has mdhop installed
  — raw mv/rm/cp will break links.
---

# mdhop

mdhop pre-indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite, so you can navigate between notes structurally instead of relying on grep. It tracks assets (images, PDFs, etc.) and stores frontmatter metadata with type-safe querying. When files are created, moved, renamed, or deleted, mdhop handles both the disk operation and the link rewrites atomically.

## Prerequisites

- `mdhop` binary (install: `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`)
- Index built: run `mdhop build` once in the vault root
- Add `.mdhop/` to `.gitignore`

## Core Principles

**Always use `--format json`** — JSON is unambiguous and machine-parseable. Text format is for humans reading terminal output; as an agent, always prefer JSON.

**Always use `--fields`** — A full query can return hundreds of backlinks and two-hop entries, wasting tokens. Request only the fields you need for the current step.

**Never use raw `mv`, `rm`, or `cp` on vault files.** These break links in other notes. Always use `mdhop move`, `mdhop delete --rm`, and write-then-`mdhop add` instead.

**Finalize content before running mdhop commands.** Write or edit the file first, then run `mdhop add` or `mdhop update`. Running mdhop on partially-written files means the index won't reflect the final content.

**Run from vault root** (or pass `--vault <path>`). All paths are vault-relative (e.g., `Notes/Design.md`).

## Recommended Query Workflow

When investigating a vault, follow this sequence to work efficiently:

1. **Get the big picture first**: `mdhop stats --format json` — understand vault scale
2. **Check for problems**: `mdhop diagnose --format json` — identify conflicts and phantoms
3. **Explore specific notes**: `mdhop query --file X.md --fields backlinks,outgoing --format json`
4. **Go deeper if needed**: add `twohop` field or follow backlinks to related notes
5. **Filter by metadata**: add `--where` to narrow results by frontmatter fields

## Field Selection Guide

Choose fields based on what you're trying to learn:

| Goal | Fields to request |
|------|-------------------|
| Who links to this note? | `backlinks` |
| What does this note link to? | `outgoing` |
| What tags does this note have? | `tags` |
| Find indirectly related notes | `twohop` |
| Read note content | `backlinks,head` + `--include-head N` |
| See link context | `backlinks,snippet` + `--include-snippet N` |
| Get frontmatter metadata | `meta` |

Avoid requesting all fields at once. Each extra field multiplies output size.

## Query Commands

### query — Find related notes

The primary exploration command. Takes one entry point and returns its relationships.

```bash
# By file path (most common)
mdhop query --file Notes/Design.md --fields backlinks,tags --format json

# By tag
mdhop query --tag architecture --fields backlinks --format json

# By phantom (unresolved reference)
mdhop query --phantom MissingConcept --fields backlinks --format json

# By name (auto-detects type: #-prefixed = tag, otherwise note/phantom)
mdhop query --name Design --fields backlinks,outgoing --format json
```

Available fields: `backlinks`, `tags`, `twohop`, `outgoing`, `head`, `snippet`, `meta`

#### Including note content

```bash
# First N lines of each note (frontmatter excluded, leading blanks skipped)
mdhop query --file X.md --fields backlinks,head --include-head 10 --format json

# Context around each link occurrence (N lines before + after)
mdhop query --file X.md --fields backlinks,snippet --include-snippet 3 --format json
```

#### Including frontmatter metadata

The `meta` field returns the entry note's frontmatter key-value pairs. It's opt-in — only included when explicitly requested in `--fields`:

```bash
mdhop query --file X.md --fields backlinks,meta --format json
```

#### Filtering by frontmatter metadata (--where)

`--where` filters result nodes (backlinks, outgoing, twohop targets) by their frontmatter metadata. The entry node itself is never filtered.

```bash
# Notes with status=active that link to X
mdhop query --file X.md --where "status=active" --fields backlinks --format json

# Notes with priority greater than 1
mdhop query --file X.md --where "priority>1" --fields backlinks --format json

# Notes that have a "due" key (any value)
mdhop query --file X.md --where "due" --fields backlinks --format json

# Combine filters (AND): active notes with high priority
mdhop query --file X.md --where "status=active" --where "priority>1" --format json
```

Operators: `=`, `!=`, `~` (LIKE pattern with `%`/`_`), `>`, `<`, `>=`, `<=`, and EXISTS (key name only).

Multiple `--where` with the same key: OR. Different keys: AND.

Type-safe comparisons (>, <, >=, <=) work when keys are declared in `mdhop.yaml`'s `meta.types` — dates, numbers, and semver compare correctly instead of lexicographically. Without type declarations, all comparisons are string-based.

Phantom, tag, and asset nodes have no metadata, so they are always excluded when `--where` is used.

#### Filtering by path and tag

```bash
# Exclude paths by glob pattern
mdhop query --file X.md --exclude "daily/*" --exclude "templates/*" --format json

# Exclude a tag from the tags list and twohop via entries
mdhop query --file X.md --exclude-tag "#template" --format json

# Cap results to avoid token bloat
mdhop query --file X.md --max-backlinks 20 --max-twohop 10 --format json
```

`--exclude-tag` removes the tag node itself from results (tags list and twohop via entries). It does NOT filter out notes that carry that tag from backlinks/outgoing.

### resolve — Check what a specific link points to

```bash
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
```

Returns one result with `type`, `name`, `path`, `exists`, and optional `subpath`.

### stats — Vault overview

```bash
mdhop stats --format json
```

Returns: `notes_total`, `notes_exists`, `edges_total`, `tags_total`, `phantoms_total`, `assets_total`.

### diagnose — Find problems

```bash
mdhop diagnose --format json
```

Returns: `basename_conflicts`, `asset_basename_conflicts`, `phantoms`.

## File Operation Commands

### Choosing the Right Command

| What happened? | Command | Notes |
|----------------|---------|-------|
| Created a new .md file | `mdhop add --file <path>` | Auto-disambiguates if basename conflicts arise |
| Edited an existing file | `mdhop update --file <path>` | If file was deleted from disk, treated as delete |
| Deleted a file | `mdhop delete --file <path> --rm` | Omit `--rm` if already deleted from disk |
| Deleted a directory | `mdhop delete --file <dir>/ --rm` | Trailing `/` triggers directory mode |
| Moved or renamed a file | `mdhop move --from <old> --to <new>` | Handles disk move + link rewrites |
| Moved a directory | `mdhop move --from <old>/ --to <new>/` | Atomic bulk move |
| Links are broken | `mdhop repair` | Preview with `--dry-run --format json` |
| Want shorter link paths | `mdhop simplify` | Inverse of disambiguate |
| Convert wikilink ↔ markdown | `mdhop convert --to wikilink\|markdown` | |
| Index is stale or corrupted | `mdhop build` | Full rebuild from scratch |
| Set up frontmatter types | `mdhop init-meta` | Generates `mdhop.yaml` `meta.types` |

### Key Workflows

**Adding a new file:**
```bash
# 1. Write the file content first
# 2. Register it
mdhop add --file Notes/NewNote.md --format json
```

**Moving or renaming:**
```bash
mdhop move --from Notes/OldName.md --to Notes/NewName.md --format json
# Directory move (atomic, preferred over sequential)
mdhop move --from OldDir/ --to NewDir/ --format json
```

**Deleting:**
```bash
mdhop delete --file Notes/Obsolete.md --rm --format json
```

**Repairing broken links:**
```bash
mdhop repair --dry-run --format json   # preview
mdhop repair                            # apply
mdhop build                             # rebuild index
```

**Setting up frontmatter metadata:**
```bash
# Scan vault to infer types, review output
mdhop init-meta --scan

# Write to mdhop.yaml and rebuild
mdhop init-meta --scan --write
mdhop build
```

For a quick start without scanning, `mdhop init-meta --preset --write` gives recommended type definitions. Use `--no-comment` for machine-readable output.

### When to Rebuild

After these commands, always run `mdhop build`:
- `mdhop repair`, `mdhop simplify`, `mdhop convert`, `mdhop disambiguate --scan`

Incremental commands (`add`, `update`, `delete`, `move`) update the index automatically.

## Configuration (mdhop.yaml)

Optional file in vault root:

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
  types:
    date: date
    created: date
    due: date
    priority: number
    version: semver
    severity:
      ordered: [low, medium, high, critical]
```

- `build.exclude_paths`: Files excluded from indexing entirely (links become phantoms)
- `exclude.paths` / `exclude.tags`: Filter query results only (files are still indexed)
- `meta.types`: Declare frontmatter key types for type-safe `--where` comparisons
  - `string` (default), `number`, `date`, `semver`, `ordered: [val1, val2, ...]`

## Exploration Strategies

### Follow the backlink chain

1. `mdhop query --file X.md --fields backlinks --format json`
2. Pick a relevant backlink, query it
3. Repeat until you find what you need

### Two-hop discovery

```bash
mdhop query --file X.md --fields twohop --format json
```

Finds notes sharing a common link target — useful for discovering unexpected connections.

### Tag-based discovery

```bash
mdhop query --tag "architecture" --fields backlinks --format json
```

### Metadata-driven filtering

```bash
# Active, high-priority notes related to a topic
mdhop query --tag "project" --where "status=active" --where "priority>1" --fields backlinks --format json

# Notes with upcoming deadlines that link to a spec
mdhop query --file Spec.md --where "due" --fields backlinks,meta --format json
```

## Error Handling

| Error | Cause | Fix |
|-------|-------|-----|
| Stale mtime | File changed since last index | `mdhop build` |
| Ambiguous link (on add) | Basename conflict | Usually auto-resolved; if not: `mdhop disambiguate` |
| Ambiguous link (on build) | Multiple files share basename | `mdhop diagnose` → `mdhop disambiguate --name <name> --target <path> --scan` |
| Broken path links | External file moves | `mdhop repair --dry-run` → `mdhop repair` → `mdhop build` |

## Reference

For detailed flag reference, all output fields, and JSON output examples:
- Query commands (query, resolve, stats, diagnose, --where): [references/query.md](references/query.md)
- File operation commands (add, update, delete, move, repair, simplify, convert, disambiguate, init-meta): [references/commands.md](references/commands.md)
