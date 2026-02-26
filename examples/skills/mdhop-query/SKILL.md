---
name: mdhop-query
description: >
  Queries link relationships in a Markdown vault using mdhop. Finds related notes via
  backlinks, outgoing links, two-hop connections, and tags. Resolves specific links,
  shows vault statistics, and diagnoses basename conflicts or phantom nodes.
  Use when exploring note connections, following backlinks, checking link targets,
  discovering related notes, or investigating vault health.
---

# mdhop Query

mdhop indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite for fast structured queries. Also tracks assets (images, PDFs, etc.).

## Prerequisites

- `mdhop` available via `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`
- Index built with `mdhop build` (run once in vault root)

## Basics

- Run from vault root (or use `--vault <path>`)
- All paths are vault-relative (e.g., `Notes/Design.md`)
- Use `--format json` for machine-readable output
- Use `--fields <comma-separated>` to limit output fields

## Key Workflows

### Find related notes

```bash
mdhop query --file Notes/Design.md --format json
```

Returns backlinks, tags, two-hop connections, and outgoing links for the given note. You can also query asset files (e.g., `--file image.png`).

You can also query by tag, phantom, or name:

```bash
mdhop query --tag architecture --format json
mdhop query --phantom MissingConcept --format json
mdhop query --name Design --format json
```

### Include note content

```bash
# Include first 10 lines of each note (frontmatter excluded)
mdhop query --file Notes/Design.md --include-head 10 --format json

# Include 3 lines before/after each link occurrence
mdhop query --file Notes/Design.md --include-snippet 3 --format json
```

### Exclude specific paths or tags from results

```bash
mdhop query --file Notes/Design.md --exclude "daily/*" --exclude-tag "#template" --format json
```

### Resolve a specific link

```bash
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
```

### Check vault statistics

```bash
mdhop stats --format json
```

### Diagnose issues

```bash
mdhop diagnose --format json
```

Reports basename conflicts (for both notes and assets) and phantom (unresolved) nodes.

## Exploration Patterns

### Deep exploration via backlinks

1. Query the target note: `mdhop query --file X.md --fields backlinks --format json`
2. Pick a relevant backlink from the result
3. Query that note to continue exploring: `mdhop query --file <backlink_path> --format json`

### Discover connections via two-hop

Two-hop finds notes that share a common target with your entry point, even without a direct link. The `via` field shows the shared target; `targets` are the related notes worth exploring.

```bash
mdhop query --file X.md --fields twohop --format json
```

### Tag-based discovery

Find all notes sharing a specific tag:

```bash
mdhop query --tag "resource/knowledge-management" --fields backlinks --format json
```

## Token Efficiency

For large vaults, always use `--fields` to request only the data you need. A full query can return hundreds of backlinks and two-hop entries.

```bash
# Prefer this (returns only what you need):
mdhop query --file X.md --fields backlinks,tags --format json
```

Use `--max-backlinks` and `--max-twohop` to cap results when you only need a sample.

## Why `--format json`?

JSON output is structured and unambiguous, making it easier to parse programmatically. Text format is human-friendly but harder to process reliably.

## Stale Index

If you see a stale detection error (mtime mismatch), rebuild the index:

```bash
mdhop build
```

## Need to Update the Index?

If you also create, edit, move, or delete Markdown files, use the **mdhop-workflow** skill instead. It covers both index management and querying.

## Reference

See [reference.md](reference.md) for detailed command options, field definitions, and output examples.
