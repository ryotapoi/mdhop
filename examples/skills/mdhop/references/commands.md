# mdhop Command Reference

Detailed reference for file operation commands: `build`, `add`, `update`, `delete`, `move`, `disambiguate`, `repair`, `simplify`, `convert`, `init-meta`.

## Common Options

- `--vault <path>`: Vault root directory (default: current directory)
- `--format json|text`: Output format (default: text)

---

## build

Scan all `*.md` files and non-markdown asset files, then create or rebuild the index from scratch.

```bash
mdhop build
```

- Creates `.mdhop/index.sqlite`
- Registers `.md` files as notes and non-`.md` files as assets (hidden files/directories excluded)
- Stores frontmatter metadata in the `meta` table (type-aware normalization if `meta.types` is configured)
- With `meta.link_keys` configured, raw path values of the declared frontmatter keys (e.g., `related: docs/spec.md`) are indexed as `frontmatter_path` link edges — they appear in backlinks, reachable, and graph. Resolution follows markdown-link rules; URL values and wikilink values are skipped; unresolvable values become phantoms
- Errors if ambiguous links exist (strict mode); `link_keys` raw path values are validated too (vault escape / ambiguous basename)
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
- With `meta.link_keys` configured: raw path values in frontmatter are not link syntax and cannot be rewritten, so `add` errors before changing anything when it would change what an existing raw path value resolves to (fix the frontmatter value manually, then retry)

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
  - Wikilinks inside frontmatter values are rewritten the same way; quoted vs bare YAML style is preserved
  - With `meta.link_keys` configured: raw path values in frontmatter are not rewritable, so `move` errors before changing anything when it would change what a raw path value resolves to (fix the frontmatter value manually, then retry)
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
- Also handles broken path links pointing to phantom nodes

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
- Finds broken path links (target does not exist) and vault-escape links
- Vault-escape links are always basename-ified regardless of candidate count
- Broken path links are rewritten to basename if the basename has 0 or 1 candidate note
- Skips broken path links where the basename has 2+ candidates (reported in `skipped`)
- Skips links whose target file exists on disk
- Skips basename links (already in basename form)
- After repair, run `mdhop build` to create or update the index
- URL links, tag links, and frontmatter values (including wikilinks inside frontmatter) are not affected

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
- Shortens path links to basename when:
  - The basename is unique across the vault, OR
  - The basename has multiple candidates but one is in the vault root (root-priority rule)
- Basename links are skipped (already short)
- Broken links and vault-escape links are skipped (use `repair` first)
- Asset path links are only shortened when no note has the same basename
- Wikilinks inside frontmatter values are also shortened (quoted vs bare YAML style is preserved)
- `build.exclude_paths` is respected
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
- Converts wikilink (`[[...]]`) ↔ markdown link (`[...](...)`) in note bodies
- URL links, tags, and frontmatter values (including wikilinks inside frontmatter) are not affected
- `build.exclude_paths` is respected
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

## init-meta

Generate frontmatter type declarations for `mdhop.yaml`.

```bash
mdhop init-meta --preset
mdhop init-meta --scan
mdhop init-meta --preset --scan --write
```

| Flag | Required | Description |
|------|----------|-------------|
| `--vault <path>` | No | Vault root directory (default: current directory) |
| `--preset` | One of two | Include recommended type definitions |
| `--scan` | One of two | Scan vault frontmatter and infer types |
| `--write` | No | Write directly to `mdhop.yaml` (default: stdout) |
| `--no-comment` | No | Omit inference comments from output |

At least one of `--preset` or `--scan` is required.

**Preset types (15 entries):**
- date (10): `date`, `created`, `modified`, `updated`, `lastmod`, `due`, `deadline`, `scheduled`, `start`, `done`
- number (4): `priority`, `weight`, `order`, `rating`
- semver (1): `version`

**Scan behavior:**
- Parses all `.md` files' frontmatter (DB not required, can run before `build`)
- Infers type when 80%+ of values match a pattern (date, number, semver)
- String keys with 10 or fewer unique values are suggested as `ordered` type candidates (in comments)
- Skips `tags` and `aliases` keys (well-known special keys)
- Comments include sample values and unique value counts for review

**`--preset --scan` combined:** Scan results override preset defaults (data-driven takes priority).

**`--write` behavior:**
- Atomically writes to `mdhop.yaml` (temp file + rename)
- Preserves existing `build` and `exclude` sections
- Does NOT overwrite existing `meta.types` keys (only adds new ones)
- Reports added/skipped counts to stderr

### Output Example (stdout)

```yaml
meta:
  types:
    date: date
    priority: number        # inferred: number (3/3 values)
    status: string          # inferred: string (2 values, e.g. "active", "done")
```
