# ADR 0004: Root-priority rule for ambiguous basename links

## Status

Accepted

## Context

In strict mode, when multiple files share the same basename (e.g., `A.md` at root and `sub/A.md`), a basename link `[[A]]` was treated as ambiguous and rejected. However, in vault-relative path notation, a root file's path is just its basename (e.g., `A`), which is identical to the basename link form. This meant that even when there was a clear, unambiguous resolution available (the root file), the system would error.

Additionally, the rewrite system used source-relative paths (`./A`, `../A`) for root files but vault-relative paths (`sub/A`) for subdirectory files, creating an inconsistency.

## Considered Options

- **Option A**: Keep all basename-ambiguous links as errors, require explicit path for all
- **Option B**: Root-priority rule — when a root file exists, `[[basename]]` resolves to it
- **Option C**: Obsidian-compatible shortest-path resolution (deferred to compat mode)

## Decision

We will adopt root-priority (Option B): when multiple files share a basename and one is at the vault root, `[[basename]]` resolves to the root file without requiring explicit path disambiguation. All rewritten links use vault-relative paths uniformly.

This is based on the property that a root file's vault-relative path (without extension) is identical to its basename, so `[[basename]]` is already the correct vault-relative link to it.

## Consequences

- Basename links to root files remain short and readable even when collisions exist
- All rewritten links are now vault-relative (no more `./` or `../` prefixes), simplifying the rewrite logic
- The root file gets special treatment, which may be surprising if users expect all collisions to be treated equally
- Case-sensitive filesystems with two root files differing only in case are explicitly out of scope
- Subdirectory-only collisions (e.g., `sub1/A.md` vs `sub2/A.md`) still require explicit path disambiguation
