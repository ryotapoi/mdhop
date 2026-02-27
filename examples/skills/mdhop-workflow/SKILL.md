---
name: mdhop-workflow
description: >
  Manages the mdhop index and rewrites links when creating, editing, moving,
  renaming, or deleting Markdown files or assets in a vault. Also handles link
  format conversion (wikilink ↔ markdown), simplifying verbose paths to basename,
  repairing broken links, and resolving basename ambiguity. Use this skill
  whenever you need to add new notes, move or rename files, delete notes,
  update changed files, fix broken links, convert link formats, or clean up
  redundant paths. Even if the user doesn't mention "mdhop" by name, use this
  skill for any file operation in an Obsidian-style Markdown vault that has
  mdhop installed — raw mv/rm/cp will break links.
---

# mdhop Workflow

mdhop indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite. When files are created, moved, renamed, or deleted, mdhop handles both the disk operation and the link rewrites atomically — so other notes' links stay correct.

## Prerequisites

- `mdhop` binary (install: `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`)
- Index built: run `mdhop build` once in the vault root
- Add `.mdhop/` to `.gitignore`

## Core Principles

**Never use raw `mv`, `rm`, or `cp` on vault files.** These break links in other notes. Always use `mdhop move`, `mdhop delete --rm`, and write-then-`mdhop add` instead. This is the most important rule — a single raw file operation can silently corrupt dozens of links.

**Always use `--format json`** for machine-readable output. Text format is for humans reading terminal output.

**Finalize content before running mdhop commands.** Write or edit the file first, then run `mdhop add` or `mdhop update`. Running mdhop on partially-written files means the index won't reflect the final content.

## Choosing the Right Command

| What happened? | Command | Notes |
|----------------|---------|-------|
| Created a new .md file | `mdhop add --file <path>` | Auto-disambiguates if basename conflicts arise |
| Edited an existing file | `mdhop update --file <path>` | If file was deleted from disk, treated as delete |
| Deleted a file | `mdhop delete --file <path> --rm` | Omit `--rm` if already deleted from disk |
| Deleted a directory | `mdhop delete --file <dir>/ --rm` | Trailing `/` triggers directory mode |
| Moved or renamed a file | `mdhop move --from <old> --to <new>` | Handles disk move + link rewrites |
| Moved a directory | `mdhop move --from <old>/ --to <new>/` | Atomic bulk move, preferred over sequential |
| Links are broken after external changes | `mdhop repair` | Preview with `--dry-run --format json` |
| Want shorter link paths | `mdhop simplify` | Inverse of disambiguate |
| Convert wikilink ↔ markdown | `mdhop convert --to wikilink` or `--to markdown` | |
| Index is stale or corrupted | `mdhop build` | Full rebuild from scratch |

## Operation Workflows

### Adding a new file

```bash
# 1. Write the file content
# 2. Register it
mdhop add --file Notes/NewNote.md --format json
```

If adding this file causes a basename conflict (another file shares the same name), mdhop automatically rewrites existing basename links to full paths. When this happens, **tell the user which files were rewritten** — they may want to review those changes.

### Moving or renaming

```bash
# Single file
mdhop move --from Notes/OldName.md --to Notes/NewName.md --format json

# Entire directory (atomic, preferred over sequential moves)
mdhop move --from OldDir/ --to NewDir/ --format json
```

Directory moves are atomic — all files move simultaneously and link rewrites are computed against the final state. Always prefer directory move over looping through files individually.

### Deleting

```bash
# Delete from disk + remove from index
mdhop delete --file Notes/Obsolete.md --rm --format json

# Already deleted from disk, just clean up index
mdhop delete --file Notes/Obsolete.md --format json
```

When other notes still link to the deleted file, it becomes a phantom node (a known "unresolved reference" — this is normal, not an error).

### Repairing broken links

When links are broken (e.g., after files were moved externally without mdhop):

```bash
# 1. Preview what would change
mdhop repair --dry-run --format json

# 2. Check the "skipped" field — multi-candidate links need manual resolution
# 3. Apply if preview looks good
mdhop repair

# 4. Resolve any skipped ambiguous links
mdhop disambiguate --name <basename> --target <path>

# 5. Rebuild the index
mdhop build
```

### Simplifying verbose paths

Shortens `[[path/to/Note]]` to `[[Note]]` when the basename is unambiguous:

```bash
# 1. Preview
mdhop simplify --dry-run --format json

# 2. Apply
mdhop simplify

# 3. Rebuild
mdhop build
```

### Converting link formats

```bash
# Markdown links → wikilinks
mdhop convert --to wikilink --format json

# Wikilinks → markdown links
mdhop convert --to markdown --format json

# Rebuild after conversion
mdhop build
```

## When to Rebuild

Several commands modify files without updating the index. After these, always run `mdhop build`:

- `mdhop repair`
- `mdhop simplify`
- `mdhop convert`
- `mdhop disambiguate --scan`

The incremental commands (`add`, `update`, `delete`, `move`) update the index automatically — no rebuild needed.

## Error Recovery

| Error | Cause | Fix |
|-------|-------|-----|
| Stale mtime | File changed since last index | `mdhop build` |
| Ambiguous link (on add) | New file creates basename conflict | Usually auto-resolved. If phantom with multiple candidates: `mdhop disambiguate --name <name> --target <path>` first |
| Ambiguous link (on build) | Multiple files share a basename and links are ambiguous | `mdhop diagnose` → `mdhop disambiguate --name <name> --target <path> --scan` for each |
| Broken path links / vault-escape | External file moves or manual edits | `mdhop repair --dry-run --format json` → `mdhop repair` → `mdhop build` |

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
```

- `build.exclude_paths`: Files excluded from indexing entirely (links to them become phantoms)
- `exclude.paths` / `exclude.tags`: Filter query results only (files are still indexed)

## Querying (Read-Only)

For exploring link structure without modifying files, use `mdhop query`, `mdhop resolve`, `mdhop stats`, and `mdhop diagnose`. These are covered in detail by the **mdhop-query** skill. Key commands:

```bash
mdhop stats --format json                    # vault overview
mdhop diagnose --format json                 # find conflicts and phantoms
mdhop query --file X.md --fields backlinks,outgoing --format json
```

## Reference

For detailed flag reference, all output fields, and JSON examples, see [reference.md](reference.md).
