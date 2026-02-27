---
name: mdhop-query
description: >
  Navigate and explore link relationships in a Markdown vault using mdhop.
  Finds backlinks, outgoing links, two-hop connections, tags, and related notes.
  Resolves specific links, shows vault statistics, and diagnoses issues like
  basename conflicts or phantom (unresolved) nodes.
  Use this skill whenever: exploring connections between notes, following backlinks
  or outgoing links, checking what a wikilink or markdown link resolves to,
  discovering related notes through two-hop paths, investigating vault health,
  getting vault statistics, or working with any Obsidian-style Markdown vault
  that has mdhop installed. Even if the user doesn't mention "mdhop" by name,
  use this skill when they want to understand link structure in a Markdown vault.
---

# mdhop Query

mdhop pre-indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite, so you can navigate between notes structurally instead of relying on grep. It also tracks assets (images, PDFs, etc.).

## Prerequisites

- `mdhop` binary (install: `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`)
- Index built: run `mdhop build` once in the vault root

## Core Principles

**Always use `--format json`** — JSON is unambiguous and machine-parseable. Text format is for humans reading terminal output; as an agent, always prefer JSON.

**Always use `--fields`** — A full query can return hundreds of backlinks and two-hop entries, wasting tokens. Request only the fields you need for the current step. See the field selection guide below.

**Run from vault root** (or pass `--vault <path>`). All paths are vault-relative (e.g., `Notes/Design.md`).

## Recommended Workflow

When investigating a vault, follow this sequence to work efficiently:

1. **Get the big picture first**: `mdhop stats --format json` — understand vault scale (how many notes, edges, tags, phantoms)
2. **Check for problems**: `mdhop diagnose --format json` — identify basename conflicts and phantom (unresolved) nodes before diving in
3. **Explore specific notes**: `mdhop query --file X.md --fields backlinks,outgoing --format json` — start with direct connections
4. **Go deeper if needed**: add `twohop` field or follow backlinks to related notes

This top-down approach avoids wasting tokens on broad queries before understanding the vault structure.

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

Avoid requesting all fields at once unless you specifically need everything. Each extra field multiplies output size.

## Commands

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

Available fields: `backlinks`, `tags`, `twohop`, `outgoing`, `head`, `snippet`

#### Including note content

When you need to read the actual text of related notes (not just their paths):

```bash
# First N lines of each note (frontmatter excluded, leading blanks skipped)
mdhop query --file X.md --fields backlinks,head --include-head 10 --format json

# Context around each link occurrence (N lines before + after)
mdhop query --file X.md --fields backlinks,snippet --include-snippet 3 --format json
```

#### Filtering results

```bash
# Exclude paths by glob pattern
mdhop query --file X.md --exclude "daily/*" --exclude "templates/*" --format json

# Exclude a tag from the tags list and twohop via entries
mdhop query --file X.md --exclude-tag "#template" --format json

# Cap results to avoid token bloat
mdhop query --file X.md --max-backlinks 20 --max-twohop 10 --format json
```

**Important**: `--exclude-tag` removes the tag node itself from results (tags list and twohop via entries). It does NOT filter out notes that carry that tag from backlinks/outgoing. To exclude notes by location, use `--exclude` with a path glob pattern.

### resolve — Check what a specific link points to

When you need to know exactly where a link resolves:

```bash
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
```

Returns one result with `type`, `name`, `path`, `exists`, and optional `subpath` (heading/block ref).

### stats — Vault overview

Quick vault-level numbers:

```bash
mdhop stats --format json
```

Returns: `notes_total`, `notes_exists`, `edges_total`, `tags_total`, `phantoms_total`, `assets_total`.

### diagnose — Find problems

Detect basename conflicts (ambiguity sources) and phantom nodes (broken references):

```bash
mdhop diagnose --format json
```

## Exploration Strategies

### Follow the backlink chain

Start at a note, look at who links to it, then follow those backlinks deeper. Each hop reveals more context about how concepts are connected.

1. `mdhop query --file X.md --fields backlinks --format json`
2. Pick a relevant backlink, query it: `mdhop query --file <path> --fields backlinks --format json`
3. Repeat until you find what you need

### Discover unexpected connections via two-hop

Two-hop finds notes that share a common link target with your starting note, even without a direct connection. The `via` field tells you what they share; `targets` are the related notes.

```bash
mdhop query --file X.md --fields twohop --format json
```

This is particularly useful when exploring a topic and wanting to find notes you didn't know were related.

### Tag-based discovery

Find all notes sharing a specific tag to understand a topic cluster:

```bash
mdhop query --tag "architecture" --fields backlinks --format json
```

## Error Handling

**Stale index**: If you see an mtime mismatch error, the index is outdated. Rebuild:
```bash
mdhop build
```

**Need to modify files?**: This skill covers read-only queries. If you also need to create, move, or delete files, use the **mdhop-workflow** skill which handles index updates and link rewrites.

## Reference

For detailed flag reference, field definitions, and JSON output examples, see [reference.md](reference.md).
