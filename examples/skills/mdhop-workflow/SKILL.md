---
name: mdhop-workflow
description: >
  Manage the mdhop index when creating, editing, moving, or deleting Markdown files in a vault.
  Use this skill when you have created a new file, edited a file, moved or renamed a file,
  deleted a file, or need to rebuild the vault index. Also covers querying link relationships.
---

# mdhop Workflow

mdhop is a CLI tool that pre-indexes link relationships (wikilinks, markdown links, tags, frontmatter) in a Markdown vault into SQLite, enabling fast structured queries without grep.

## Prerequisites

- Go installed, mdhop available via `go install github.com/ryotapoi/mdhop/cmd/mdhop@latest`

## Basics

- Run commands from the vault root directory (or use `--vault <path>`)
- All paths are vault-relative (e.g., `Notes/Design.md`)
- Use `--format json` for machine-readable output (recommended for agents)

## Initial Setup

Build the index for the first time:

```bash
mdhop build
```

This scans all `*.md` files and non-markdown asset files (images, PDFs, etc.) and creates `.mdhop/index.sqlite`. Hidden files/directories are excluded from asset scanning.

Add `.mdhop/` to your `.gitignore` — the index is machine-local and should be rebuilt per environment.

## Configuration (mdhop.yaml)

Optional. Place `mdhop.yaml` in the vault root:

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
- `exclude.paths` / `exclude.tags`: Filter query results (does not affect the index itself)

## Agent Guidelines

### Use mdhop for file operations

Always use `mdhop move` or `mdhop delete --rm` instead of raw file system operations (mv, rm). Raw operations break links in other files. mdhop handles disk operations and link rewrites atomically.

### Operation order

1. Write/Edit the file content first
2. Then run the appropriate mdhop command (add/update/move/delete)
3. Never run mdhop commands before the file content is finalized

### After auto-disambiguate

If `mdhop add` reports rewritten links, inform the user which files were changed.

### Error recovery

- **Stale mtime error**: Another process (e.g., Obsidian, cloud sync) modified the file after the last index update. Run `mdhop build` to rebuild.
- **Ambiguous link error on add**: Auto-disambiguate handles most cases. If it fails (phantom with multiple candidates), run `mdhop disambiguate --name <basename> --target <path>` first.
- **Ambiguous link error on build**: Run `mdhop diagnose` to identify conflicts, then `mdhop disambiguate --name <name> --target <path> --scan` for each.
- **Non-`.md` files in directory move/delete**: Asset files (images, PDFs, etc.) are now handled automatically — they are moved or deleted alongside `.md` files. No manual intervention needed.
- **Broken path links or vault-escape links**: Run `mdhop repair --dry-run --format json` to preview, then `mdhop repair` to fix. This rewrites broken path links and vault-escape links to basename links (no DB required; can be run before `build`). Vault-escape links are always basename-ified. For broken path links with multiple candidates (reported in `skipped`), use `mdhop disambiguate --name <basename> --target <path>` to resolve individually. After repair, run `mdhop build` to create or update the index.

## Index Update Rules

After modifying vault files, update the index with the appropriate command:

### File created

```bash
mdhop add --file Notes/NewNote.md
```

Multiple files: `mdhop add --file A.md --file B.md`

When adding a file causes a basename conflict, existing ambiguous links are automatically rewritten to full paths (auto-disambiguate). Disable with `--no-auto-disambiguate`.

### File edited

```bash
mdhop update --file Notes/Design.md
```

Multiple files: `mdhop update --file A.md --file B.md`

If the file has been deleted from disk, update treats it as a delete.

### File deleted

```bash
mdhop delete --file Notes/OldNote.md --rm
```

Omit `--rm` if the file is already deleted from disk and you only need to update the index.

If other files still link to the deleted file, it becomes a phantom node.

To delete all registered files (notes and assets) under a directory:

```bash
mdhop delete --file Notes/archive/ --rm
```

With `--rm`, empty directories are cleaned up recursively after deletion.

### File moved or renamed

```bash
mdhop move --from Notes/OldName.md --to Notes/NewName.md
```

This command:
1. Moves the file on disk (creates target directory if needed)
2. Rewrites links in other files that referenced the old path
3. Updates the index

### Directory moves

To move all files (notes and assets) under a directory at once:

```bash
mdhop move --from OldDir/ --to NewDir/
```

This moves all files atomically — link rewrites are computed against the final state, avoiding intermediate ambiguity issues. Prefer this over sequential single-file moves when renaming or relocating directories.

### Rebuild from scratch

```bash
mdhop build
```

Use when the index is out of sync or after bulk changes.

## Strict Mode

mdhop uses strict mode by default. Ambiguous links (basename links that match multiple files) cause errors. When this happens:

```bash
mdhop disambiguate --name ambiguous-basename
```

If multiple candidates exist, specify the target: `mdhop disambiguate --name a --target Notes/a.md`

This rewrites ambiguous basename links to use full paths.

## Phantom Nodes

Phantom nodes represent links to files that don't exist yet. This is normal — writing `[[Future Topic]]` before creating the file is a common pattern. When you later create the file and run `mdhop add`, the phantom is automatically promoted to a real note.

To check current phantoms: `mdhop diagnose --fields phantoms --format json`

## Querying

After updating the index, you can query link relationships:

```bash
mdhop query --file Notes/Design.md --format json
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
mdhop stats --format json
mdhop diagnose --format json
```

## Reference

See [reference.md](reference.md) for detailed command options, field definitions, and output examples.
