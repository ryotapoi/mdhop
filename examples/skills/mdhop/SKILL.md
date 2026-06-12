---
name: mdhop
description: Use mdhop for Markdown vault search, link queries, metadata filters, reachability checks, graph export, diagnostics, frontmatter checks, and link-safe file operations.
---

# mdhop

Use `mdhop` to work with an Obsidian-style Markdown vault through its SQLite link index. Prefer it when you need structural navigation, metadata search, diagnostics, or file operations that must keep links consistent.

## Rules

- Use `--format json` for agent-facing output.
- Use `--fields` on `query` to avoid pulling unnecessary backlinks, two-hop results, snippets, or metadata.
- Run commands from the vault root, or pass `--vault <path>`.
- Treat paths as vault-relative.
- Do not use raw `mv`, `rm`, or `cp` for indexed vault files. Use `mdhop move`, `mdhop delete --rm`, and write-then-`mdhop add`.
- Finish editing file contents before `mdhop add` or `mdhop update`; the index should reflect the final file state.
- After `repair`, `simplify`, `convert`, or `disambiguate --scan`, run `mdhop build`.

## Start Here

```bash
mdhop stats --format json
mdhop diagnose --format json
mdhop search --where "status=active" --fields meta --format json
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
```

## Examples

### Find Notes

```bash
# Find notes by frontmatter metadata.
mdhop search --where "status=active" --sort "-priority" --fields meta --format json

# Find notes missing a metadata key.
mdhop search --where "priority NOT EXISTS" --format json

# Find stale notes (relative date comparison).
mdhop search --where "updated<today-90d" --format json

# Rank by computed fields (line count, link counts) and output a single meta key.
mdhop search --sort -lines --limit 10 --fields lines,outgoing_count,meta.status --format json

# Find notes by path.
mdhop search --path "projects/*" --format json

# Find isolated notes for cleanup.
mdhop search --no-tags --format json
mdhop search --no-outgoing --format json
mdhop search --no-incoming --format json
```

### Explore Relationships

```bash
# Backlinks and outgoing links for one note.
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json

# Notes that use a tag.
mdhop query --tag architecture --fields backlinks --format json

# Unresolved references.
mdhop query --phantom MissingConcept --fields backlinks --format json

# Shared-target discovery.
mdhop query --file Notes/Design.md --fields twohop --format json
```

### Check Reachability and Structure

```bash
# Notes reachable / unreachable from an entry note via links.
mdhop reachable --from index.md --path "docs/*" --format json

# Shortest route to each reachable note.
mdhop reachable --from index.md --path "docs/*" --route --format json

# Export the link graph (induced subgraph) for analysis or visualization.
mdhop graph --path "docs/*" --format json
mdhop graph --path "docs/*" --format dot

# Diagnose only a subtree: phantoms and conflicts referenced from it.
mdhop diagnose --path "projects/*" --format json

# Opt in to broken heading anchor detection ([[note#heading]] fragments).
mdhop diagnose --fields anchors --format json
```

### Validate Frontmatter

```bash
# Do frontmatter reference values resolve to real targets?
mdhop meta-check --key sources --kind path --format json
mdhop meta-check --key related --kind wikilink --format json

# Does frontmatter conform to the schema (required keys, meta.types)?
mdhop meta-validate --require type --require status --require updated --format json
```

### Read Context

```bash
# First lines of related notes.
mdhop query --file Notes/Design.md --fields backlinks,head --include-head 10 --format json

# Link-adjacent snippets.
mdhop query --file Notes/Design.md --fields backlinks,snippet --include-snippet 3 --format json

# Entry note metadata.
mdhop query --file Notes/Design.md --fields meta --format json
```

### Maintain Files

```bash
# Register a new note after writing it.
mdhop add --file Notes/NewNote.md --format json

# Re-index edited notes.
mdhop update --file Notes/Design.md --format json

# Move files or directories while rewriting links.
mdhop move --from Notes/Old.md --to Notes/New.md --format json
mdhop move --from OldDir/ --to NewDir/ --format json

# Delete from disk and index.
mdhop delete --file Notes/Obsolete.md --rm --format json

# Preview and apply repair.
mdhop repair --dry-run --format json
mdhop repair --path "docs/**" --dry-run --format json
mdhop repair
mdhop build
```

## References

Read only the reference needed for the task:

- Query/search/reference output details: [references/query.md](references/query.md)
- File operations and rewrite behavior: [references/commands.md](references/commands.md)
