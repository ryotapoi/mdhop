# ADR 0011: Asset Node Management

## Status

Accepted

## Context

mdhop managed only `.md` files. Non-markdown files (images, PDFs, etc.) referenced via `![[image.png]]` or `[doc](doc.pdf)` became phantom nodes. This caused broken links on move/delete, and directory move/delete rejected directories containing non-`.md` files.

Obsidian vaults commonly contain asset files alongside notes. Users expect link integrity for these files too.

## Considered Options

- **A: Treat assets as a new node type in SQLite** — Add `type='asset'` to the nodes table with separate basename key space. Build scans all non-`.md` files.
- **B: Extend phantom nodes** — Keep assets as phantoms but add `exists_flag=1` when the file exists on disk. Simpler schema but conflates "missing reference" with "binary file."
- **C: Separate asset table** — New `assets` table with its own schema. Clean separation but doubles query complexity for resolution and requires JOINs.

## Decision

We will treat assets as a new node type (`type='asset'`) in the existing nodes table (Option A).

Key design choices:
- **Resolution priority**: note → asset → phantom. Notes always win over same-named assets.
- **Separate basename key spaces**: Note basenames are extension-stripped (`basenameKey("Note.md")` → `"note"`). Asset basenames keep extensions (`assetBasenameKey("image.png")` → `"image.png"`). This prevents cross-type ambiguity.
- **Assets are never sources**: Asset nodes have no outgoing edges. Only notes produce edges.
- **Build-only registration**: Assets are registered in the DB only during `build`. Add/update use existing DB assets for resolution; new assets on disk are phantom until the next build.
- **Disk-based move/delete**: Non-`.md` files are moved/deleted on disk regardless of DB registration. DB is updated only for registered assets.

## Consequences

- Asset links (`![[image.png]]`, `[doc](doc.pdf)`) now resolve to real nodes instead of phantoms.
- Move/delete operations maintain link integrity for assets.
- Directory move/delete no longer rejects directories with non-`.md` files.
- Build time increases slightly due to asset file scanning (mitigated by skipping hidden files/dirs and `.mdhop/`).
- `cleanupOrphanedNodes` removes unreferenced assets during update/add/delete, which may cause `stats.assets_total` to differ between build and subsequent operations.
- Assets added to disk after build remain phantom until the next build (by design — avoids filesystem watching complexity).
