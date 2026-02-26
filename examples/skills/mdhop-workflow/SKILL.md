---
name: mdhop-workflow
description: >
  Manages the mdhop index and rewrites links when creating, editing, moving, or deleting
  Markdown files in a vault. Also converts link formats and simplifies redundant paths.
  Use when files are created, edited, moved, renamed, or deleted, when the vault index needs
  rebuilding, when converting between wikilinks and markdown links, or when simplifying
  verbose path links to basename form.
---

# mdhop Workflow

mdhop indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite for fast structured queries without grep. It also tracks assets (images, PDFs, etc.).

## Prerequisites

- `mdhop` available via `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`

## Basics

- Run from vault root (or use `--vault <path>`)
- All paths are vault-relative (e.g., `Notes/Design.md`)
- Use `--format json` for machine-readable output

## Initial Setup

```bash
mdhop build    # creates .mdhop/index.sqlite
```

Add `.mdhop/` to `.gitignore`.

## Configuration (mdhop.yaml)

Optional. Place in vault root:

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

- `build.exclude_paths`: Excluded from indexing (links to them become phantoms)
- `exclude.paths` / `exclude.tags`: Filter query results only

## Agent Guidelines

### Use mdhop for file operations

Always use `mdhop move` or `mdhop delete --rm` instead of raw `mv`/`rm`. Raw operations break links. mdhop handles disk operations and link rewrites atomically.

### Operation order

1. Write/Edit the file content first
2. Then run the appropriate mdhop command
3. Never run mdhop commands before the file content is finalized

### After auto-disambiguate

If `mdhop add` reports rewritten links, inform the user which files were changed.

### Error recovery

- **Stale mtime error**: Run `mdhop build` to rebuild
- **Ambiguous link error on add**: Auto-disambiguate handles most cases. If it fails (phantom with multiple candidates), run `mdhop disambiguate --name <basename> --target <path>` first
- **Ambiguous link error on build**: Run `mdhop diagnose` to identify conflicts, then `mdhop disambiguate --name <name> --target <path> --scan` for each
- **Broken path links or vault-escape links**: Run `mdhop repair` to fix. Preview with `--dry-run --format json`. Multi-candidate links appear in `skipped` — resolve with `disambiguate`. After repair, run `build`

## Index Update Rules

### File created

```bash
mdhop add --file Notes/NewNote.md
mdhop add --file A.md --file B.md       # multiple files
```

Auto-disambiguate rewrites existing basename links to full paths when a basename conflict arises. Disable with `--no-auto-disambiguate`.

### File edited

```bash
mdhop update --file Notes/Design.md
```

If the file has been deleted from disk, update treats it as a delete.

### File deleted

```bash
mdhop delete --file Notes/OldNote.md --rm
mdhop delete --file Notes/archive/ --rm   # directory: all registered files
```

Omit `--rm` if already deleted from disk. Linked-to files become phantoms.

### File moved or renamed

```bash
mdhop move --from Notes/OldName.md --to Notes/NewName.md
mdhop move --from OldDir/ --to NewDir/   # directory: atomic bulk move
```

Moves file on disk, rewrites links in other files, updates index. Directory moves are atomic — prefer over sequential single-file moves.

### Rebuild from scratch

```bash
mdhop build
```

## Link Rewriting Commands

### Disambiguate ambiguous links

```bash
mdhop disambiguate --name ambiguous-basename
mdhop disambiguate --name a --target Notes/a.md   # when multiple candidates
```

Rewrites ambiguous basename links to full paths. mdhop uses strict mode by default — ambiguous links cause errors.

### Repair broken links

```bash
mdhop repair --dry-run --format json   # preview
mdhop repair                           # apply
```

Rewrites broken path links and vault-escape links to basename form. DB not required. After repair, run `build`.

### Simplify redundant path links

```bash
mdhop simplify --dry-run --format json   # preview
mdhop simplify                           # apply
mdhop simplify --file Notes/A.md         # specific file only
```

Inverse of disambiguate: shortens path links (relative and absolute) to basename when the basename is unique or resolvable via root-priority. DB not required. Skips broken/vault-escape links (use `repair` first). After simplify, run `build`.

### Convert link format

```bash
mdhop convert --to wikilink              # markdown → wikilink
mdhop convert --to markdown              # wikilink → markdown
mdhop convert --to wikilink --dry-run --format json   # preview
mdhop convert --to markdown --file A.md  # specific file only
```

Converts between wikilink (`[[...]]`) and markdown link (`[...](...)`) formats. DB not required — can run before `build`. After convert, run `build`.

## Phantom Nodes

Links to non-existent files create phantoms. When you later create the file and run `mdhop add`, the phantom is promoted to a real note.

Check phantoms: `mdhop diagnose --fields phantoms --format json`

## Querying

```bash
mdhop query --file Notes/Design.md --format json
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
mdhop stats --format json
mdhop diagnose --format json
```

## Reference

See [reference.md](reference.md) for detailed command options, field definitions, and output examples.
