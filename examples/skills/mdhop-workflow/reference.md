# mdhop Command Reference

Detailed reference for all mdhop commands: index management and querying.

## Common Options

- `--vault <path>`: Vault root directory (default: current directory)
- `--format json|text`: Output format (default: text)
- `--fields <comma-separated>`: Limit output fields (available for query, resolve, stats, diagnose)

---

# Index Commands

## build

Scan all `*.md` files and non-markdown asset files, then create or rebuild the index from scratch.

```bash
mdhop build
```

- Creates `.mdhop/index.sqlite`
- Registers `.md` files as notes and non-`.md` files as assets (hidden files/directories excluded)
- Errors if ambiguous links exist (strict mode)
- Respects `mdhop.yaml` `build.exclude_paths` — excluded files are not indexed; links to them become phantom nodes

## add

Register new files in the index.

```bash
mdhop add --file Notes/NewNote.md
mdhop add --file A.md --file B.md
```

| Flag | Required | Description |
|------|----------|-------------|
| `--file <path>` | Yes | File to add (repeatable) |
| `--no-auto-disambiguate` | No | Disable automatic link rewriting when basename conflicts arise |
| `--format json\|text` | No | Output format |

**Behavior:**
- Errors if the file is already registered
- Errors if the new file contains ambiguous links
- When adding causes a basename conflict, existing basename links in other files are automatically rewritten to full paths (auto-disambiguate). Use `--no-auto-disambiguate` to disable
- If existing basename links reference a phantom and the new file creates multiple candidates for that basename, it errors even with auto-disambiguate on (cannot safely determine rewrite target)

**Output fields:** `added`, `promoted`, `rewritten`

## update

Update index entries for modified files.

```bash
mdhop update --file Notes/Design.md
mdhop update --file A.md --file B.md
```

| Flag | Required | Description |
|------|----------|-------------|
| `--file <path>` | Yes | File to update (repeatable) |
| `--format json\|text` | No | Output format |

**Behavior:**
- If the file has been deleted from disk, treats it as a delete (phantom or full removal depending on references)
- Errors if updated content contains ambiguous links

**Output fields:** `updated`, `deleted`, `phantomed`

## delete

Remove files from the index.

```bash
mdhop delete --file Notes/OldNote.md
mdhop delete --file Notes/OldNote.md --rm
mdhop delete --file A.md --file B.md
mdhop delete --file Notes/archive/ --rm
```

| Flag | Required | Description |
|------|----------|-------------|
| `--file <path>` | Yes | File or directory to delete (repeatable) |
| `--rm` | No | Also delete the file from disk |
| `--format json\|text` | No | Output format |

**Behavior:**
- Errors if the file is not registered (with `--rm`, the file is not deleted from disk either)
- If other files still link to the deleted file, it becomes a phantom node
- If no references remain, the node is fully removed
- Directory mode: trailing `/` or a disk directory deletes all registered files (notes and assets) under that directory
  - Errors if no files are registered under the directory
  - With `--rm`, also removes unregistered non-`.md` files on disk (hidden files/directories are ignored)
  - With `--rm`, empty directories are cleaned up recursively after deletion

**Output fields:** `deleted`, `phantomed`

## move

Move or rename a file or directory, updating links and the index.

```bash
mdhop move --from Notes/OldName.md --to Notes/NewName.md
mdhop move --from OldDir/ --to NewDir/
```

| Flag | Required | Description |
|------|----------|-------------|
| `--from <path>` | Yes | Current file or directory path |
| `--to <path>` | Yes | New file or directory path |
| `--format json\|text` | No | Output format |

**Behavior (single file):**
- Works for both notes (`.md`) and assets (non-`.md` files)
- Moves the file on disk (creates target directory if needed)
- If `--from` is missing on disk but `--to` exists, treats as already-moved (rewrites links + updates DB only)
- Errors if `--to` already exists on disk (no overwrite)
- Rewrites links in other files:
  - Basename links (`[[a]]`) that remain unambiguous after the move are kept as-is
  - Links that would become ambiguous or resolve differently are rewritten to full paths
  - Path-based links (`[[path/to/a]]`) are always rewritten
  - Relative links in the moved file are adjusted for the new location
- Errors if source file or affected files have stale mtime (mtime mismatch with DB)

**Output fields (single file):** `from`, `to`, `rewritten`

**Behavior (directory):**
- Trailing `/` or a disk directory triggers directory mode
- `--to` ending with `.md` in directory mode is an error
- All registered files (notes and assets) under the directory are moved at once
- Unregistered non-`.md` files on disk are also moved (hidden files/directories are ignored)
- Link rewrites are computed against the final state (all files moved simultaneously)
- Links between files within the moved set (including relative links) are correctly adjusted
- Disk state must be consistent: all files must be either normal (not yet moved) or already-moved (cannot mix)
- Source and destination directories must not overlap (`--from sub --to sub/inner` is an error)

**Output fields (directory):** `moved[]` (array of `from`/`to`), `rewritten`

### JSON Output Example (directory)

```json
{
  "moved": [
    {"from": "OldDir/A.md", "to": "NewDir/A.md"},
    {"from": "OldDir/B.md", "to": "NewDir/B.md"}
  ],
  "rewritten": [
    {"file": "Other.md", "old_link": "[[OldDir/A]]", "new_link": "[[NewDir/A]]"}
  ]
}
```

## disambiguate

Rewrite ambiguous basename links to use full paths.

```bash
mdhop disambiguate --name a
mdhop disambiguate --name a --target Notes/a.md
mdhop disambiguate --name a --file Notes/Specific.md
mdhop disambiguate --name a --scan
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name <basename>` | Yes | Basename to disambiguate |
| `--target <path>` | No | Target file when multiple candidates exist |
| `--file <path>` | No | Only rewrite links in this specific file |
| `--scan` | No | Scan all files without using DB (initial rescue) |
| `--format json\|text` | No | Output format |

**Behavior:**
- If `--name` is unique (one candidate), rewrites automatically
- If multiple candidates exist, `--target` is required
- `--scan` respects `build.exclude_paths`
- Also handles broken path links pointing to phantom nodes (e.g., after `repair` leaves multi-candidate links unresolved)

**Output fields:** `rewritten`

## repair

Fix broken path links and vault-escape links by rewriting them to basename links.

```bash
mdhop repair
mdhop repair --dry-run --format json
```

| Flag | Required | Description |
|------|----------|-------------|
| `--dry-run` | No | Show what would be repaired without making changes |
| `--format json\|text` | No | Output format |

**Behavior:**
- DB not required (file-scan based). Can be run before `build`
- Finds broken path links (target does not exist) and vault-escape links (wikilink/markdown)
- Vault-escape links are always basename-ified regardless of candidate count (escape resolution is top priority; use `disambiguate` afterwards if ambiguous)
- Broken path links are rewritten to basename if the basename has 0 or 1 candidate note
- Skips broken path links where the basename has 2+ candidates (reported in `skipped`)
- Skips links whose target file exists on disk (e.g., excluded by `build.exclude_paths`)
- Skips basename links (already in basename form)
- `--dry-run` shows the result without modifying disk
- After repair, run `mdhop build` to create or update the index
- If build fails with ambiguous links after repair, use `disambiguate` to resolve them
- URL links, tag links, and frontmatter links are not affected

**Output fields:** `rewritten`, `skipped`

### JSON Output Example

```json
{
  "rewritten": [
    {"file": "A.md", "old": "[[old/path/X]]", "new": "[[X]]"}
  ],
  "skipped": [
    {
      "file": "A.md",
      "raw_link": "[[old/M]]",
      "basename": "M",
      "candidates": ["dir1/M.md", "dir2/M.md"]
    }
  ]
}
```

## simplify

Shorten redundant path links to basename form. Inverse of `disambiguate`.

```bash
mdhop simplify
mdhop simplify --dry-run --format json
mdhop simplify --file Notes/A.md
```

| Flag | Required | Description |
|------|----------|-------------|
| `--dry-run` | No | Show what would be simplified without making changes |
| `--file <path>` | No | Limit to specific file (repeatable) |
| `--format json\|text` | No | Output format |

**Behavior:**
- DB not required (file-scan based)
- Shortens path links (relative and absolute) to basename when:
  - The basename is unique across the vault, OR
  - The basename has multiple candidates but one is in the vault root (root-priority rule)
- Basename links are skipped (already short)
- Broken links and vault-escape links are skipped (use `repair` first)
- Asset path links are only shortened when no note has the same basename (namespace conflict detection)
- `build.exclude_paths` is respected
- URL links, tag links, and frontmatter links are not affected
- After simplify, run `mdhop build` to update the index

**Output fields:** `rewritten`, `skipped`

### JSON Output Example

```json
{
  "rewritten": [
    {"file": "A.md", "old_link": "[[sub/B]]", "new_link": "[[B]]"}
  ],
  "skipped": [
    {
      "file": "A.md",
      "raw_link": "[[dir1/M]]",
      "basename": "M",
      "reason": "ambiguous",
      "candidates": ["dir1/M.md", "dir2/M.md"]
    }
  ]
}
```

## convert

Convert between wikilink and markdown link formats.

```bash
mdhop convert --to wikilink
mdhop convert --to markdown
mdhop convert --to wikilink --dry-run --format json
mdhop convert --to markdown --file A.md
```

| Flag | Required | Description |
|------|----------|-------------|
| `--to <format>` | Yes | Target format: `wikilink` or `markdown` |
| `--dry-run` | No | Show what would be converted without making changes |
| `--file <path>` | No | Limit to specific file (repeatable) |
| `--format json\|text` | No | Output format |

**Behavior:**
- DB not required (file-scan based). Can run before `build`
- Converts wikilink (`[[...]]`) ↔ markdown link (`[...](...)`)
- URL links, tags, and frontmatter links are not affected
- `build.exclude_paths` is respected (excluded files are not scanned)
- After convert, run `mdhop build` to create or update the index

**Output fields:** `rewritten`

### JSON Output Example

```json
{
  "rewritten": [
    {"file": "A.md", "old_link": "[[B]]", "new_link": "[B](B.md)"},
    {"file": "A.md", "old_link": "[[C#Heading]]", "new_link": "[C](C.md#Heading)"}
  ]
}
```

---

# Query Commands

## query

Query link relationships for a given entry point.

### Entry Point (one required)

| Flag | Description |
|------|-------------|
| `--file <path>` | Note entry point (vault-relative path) |
| `--tag <name>` | Tag entry point (`#` prefix optional) |
| `--phantom <name>` | Phantom (unresolved) node entry point |
| `--name <name>` | Auto-detect type: `#tag` → tag, otherwise note/phantom. Errors if ambiguous (root-priority exception applies) |

### Fields

Available fields for `--fields`: `backlinks`, `tags`, `twohop`, `outgoing`, `head`, `snippet`

| Field | Description |
|-------|-------------|
| `backlinks` | Notes that link to the entry point |
| `tags` | Tags the entry note has |
| `twohop` | Related notes via shared targets (A→X and B→X). Returns `via` node and its `targets` |
| `outgoing` | Outgoing links from the entry note |
| `head` | First N lines of the note (requires `--include-head`) |
| `snippet` | Lines around each link occurrence (requires `--include-snippet`) |

Each node in backlinks/outgoing/twohop includes a `type` field (`note`, `phantom`, `tag`, or `asset`). Notes and assets include `name`, `path`, `exists`. Phantoms and tags include `name`.

### Content Options

| Flag | Description |
|------|-------------|
| `--include-head <N>` | Include first N lines of each note (frontmatter excluded, leading blank lines skipped) |
| `--include-snippet <N>` | Include N lines before and after each link (2N+1 lines total) |

### Limit Options

| Flag | Default | Description |
|------|---------|-------------|
| `--max-backlinks <N>` | 100 | Maximum backlinks returned |
| `--max-twohop <N>` | 100 | Maximum two-hop entries returned |
| `--max-via-per-target <N>` | 10 | Maximum via nodes per two-hop target |

### Exclude Options

| Flag | Description |
|------|-------------|
| `--exclude <glob>` | Exclude paths matching the glob pattern (repeatable) |
| `--exclude-tag <tag>` | Exclude a specific tag (repeatable, `#` prefix recommended) |
| `--no-exclude` | Ignore exclusions defined in `mdhop.yaml` |

CLI `--exclude`/`--exclude-tag` flags are merged with `mdhop.yaml` `exclude` settings.

#### Exclude Behavior

- Applies to: backlinks, outgoing, tags, twohop (both via and targets), snippet
- Entry node itself is never excluded
- Path glob: `*` matches any character including `/`. `?` matches a single character. Case-sensitive. `[...]` character classes are not supported (causes error)
- Tag exclude: exact match, case-insensitive
- Twohop: if a via node matches an excluded tag/path, the entire via entry is removed

### Examples

```bash
# Full query with JSON output
mdhop query --file Notes/Design.md --format json

# Only backlinks and tags
mdhop query --file Notes/Design.md --fields backlinks,tags --format json

# With content
mdhop query --file Notes/Design.md --include-head 10 --include-snippet 3 --format json

# Tag query
mdhop query --tag architecture --format json

# With exclusions
mdhop query --file Notes/Design.md --exclude "daily/*" --exclude-tag "#template" --format json
```

### JSON Output Example

```json
{
  "backlinks": [
    {"type": "note", "name": "Spec", "path": "Notes/Spec.md", "exists": true}
  ],
  "tags": [
    {"type": "tag", "name": "#architecture"}
  ],
  "twohop": [
    {
      "via": {"type": "note", "name": "Spec", "path": "Notes/Spec.md", "exists": true},
      "targets": [
        {"type": "note", "name": "Plan", "path": "Notes/Plan.md", "exists": true}
      ]
    }
  ],
  "outgoing": [
    {"type": "note", "name": "Spec", "path": "Notes/Spec.md", "exists": true},
    {"type": "phantom", "name": "FutureIdea"}
  ]
}
```

## resolve

Resolve a specific link from a given source file.

### Required Flags

| Flag | Description |
|------|-------------|
| `--from <path>` | Source file (vault-relative) |
| `--link <link>` | Link to resolve (e.g., `[[Spec]]`, `[text](spec.md)`) |

### Fields

Available fields for `--fields`: `type`, `name`, `path`, `exists`, `subpath`

| Field | Description |
|-------|-------------|
| `type` | `note`, `phantom`, `tag`, `asset`, or `url` |
| `name` | Display name (basename for notes/assets, `#`-prefixed for tags) |
| `path` | Vault-relative path (notes and assets only) |
| `exists` | Whether the note file exists on disk |
| `subpath` | Heading (`#Heading`) or block reference (`#^block`) if present |

### Resolution Rules

- The link must actually exist in the source file
- Resolution always returns exactly one result (ambiguous = error)
- `[[Note]]`: basename search across vault. Multiple matches → error (root-priority exception: if one match is in vault root, it wins)
- `[[#Heading]]`: resolves to the source file itself
- `[[path/to/Note]]`: vault-root-relative
- `[[./Note]]`, `[[../Note]]`: relative to source file
- Markdown links: `/`-prefixed → vault-root-relative; `./`/`../`-prefixed → relative to source; contains `/` → path resolution; no `/` → basename resolution
- Paths that escape outside the vault are errors in strict mode

### Example

```bash
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
```

```json
{
  "type": "note",
  "name": "Spec",
  "path": "Notes/Spec.md",
  "exists": true
}
```

## stats

Show vault statistics.

### Fields

Available fields for `--fields`: `notes_total`, `notes_exists`, `edges_total`, `tags_total`, `phantoms_total`, `assets_total`

| Field | Description |
|-------|-------------|
| `notes_total` | Total number of note nodes |
| `notes_exists` | Notes that exist on disk |
| `edges_total` | Total link occurrences |
| `tags_total` | Total unique tags |
| `phantoms_total` | Total phantom (unresolved) nodes |
| `assets_total` | Total asset (non-.md) nodes |

### Example

```bash
mdhop stats --format json
```

```json
{
  "notes_total": 150,
  "notes_exists": 148,
  "edges_total": 1200,
  "tags_total": 45,
  "phantoms_total": 12,
  "assets_total": 30
}
```

## diagnose

Detect issues in the vault index.

### Fields

Available fields for `--fields`: `basename_conflicts`, `asset_basename_conflicts`, `phantoms`

| Field | Description |
|-------|-------------|
| `basename_conflicts` | Note files sharing the same basename (potential ambiguity source) |
| `asset_basename_conflicts` | Asset files sharing the same basename |
| `phantoms` | Nodes referenced by links but not present on disk |

### Example

```bash
mdhop diagnose --format json
```

```json
{
  "basename_conflicts": [
    {
      "name": "README",
      "paths": ["README.md", "docs/README.md"]
    }
  ],
  "phantoms": [
    {"name": "FutureIdea"},
    {"name": "MissingRef"}
  ]
}
```
