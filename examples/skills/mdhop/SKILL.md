---
name: mdhop
description: Use mdhop for Markdown vault search, link queries, metadata filters, reachability checks, graph export, diagnostics, frontmatter checks, and link-safe file operations.
---

# mdhop

Use `mdhop` to work with an Obsidian-style Markdown vault through its SQLite link index. Prefer it when you need structural navigation, metadata search, diagnostics, or file operations that must keep links consistent.

## Rules

- Use `--format json` for agent-facing output.
- Use `--fields` on `query`, `search`, `resolve`, `reachable`, `stats`, and `diagnose` to avoid pulling unnecessary data.
- Run commands from the vault root, or pass `--vault <path>`.
- Treat paths as vault-relative.
- Do not use raw `mv`, `rm`, or `cp` for indexed vault files. Use `mdhop move`, `mdhop delete --rm`, and write-then-`mdhop add`.
- Do not hand-edit a single frontmatter key. Use `mdhop set` so the index stays in sync.
- Finish editing file contents before `mdhop add` or `mdhop update`; the index should reflect the final file state.
- After `repair`, `simplify`, `convert`, or `disambiguate --scan`, run `mdhop build`.
- For exact flags, output fields, and examples, run `mdhop <command> --help`.

## Start Here

```bash
mdhop stats --format json
mdhop diagnose --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
```

## When To Use Which Command

### Find Notes

Use `mdhop search` when there is no single entry note and you want notes by frontmatter, path, link-count fields, random sample, or count-only output.

```bash
mdhop search --where "status=active" --fields meta --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop search --where "updated<today-90d" --count --format json
```

Run `mdhop search --help` for the full filter, sort, sampling, count, and output-field syntax.

### Explore Relationships

Use `mdhop query` when you have an entry note, tag, phantom, or name and need backlinks, outgoing links, tags, two-hop related notes, snippets, head lines, or entry metadata.

```bash
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
mdhop query --tag architecture --fields backlinks --format json
```

Run `mdhop query --help` for entry options, fields, metadata filters, limits, and include options.

### Resolve One Link

Use `mdhop resolve` when you need to know exactly what one link text resolves to from one source note.

```bash
mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
```

Run `mdhop resolve --help` for supported output fields and examples.

### Check Reachability and Structure

Use `mdhop reachable` for reachable/unreachable note sets from an entry note. Use `mdhop graph` for graph export, `mdhop stats` for counts, and `mdhop diagnose` for conflicts, phantoms, and optional anchor checks.

```bash
mdhop reachable --from index.md --path "docs/*" --route --format json
mdhop graph --path "docs/*" --format json
mdhop diagnose --format json
```

Run `mdhop reachable --help`, `mdhop graph --help`, `mdhop stats --help`, or `mdhop diagnose --help` before composing flags.

### Validate Frontmatter

Use `mdhop meta-check` to verify that frontmatter reference values point to real paths or wikilinks. Use `mdhop meta-validate` to check required keys and declared types from `mdhop.yaml`.

```bash
mdhop meta-check --key sources --kind path --format json
mdhop meta-validate --require type --require status --format json
```

Run `mdhop meta-check --help` or `mdhop meta-validate --help` for issue/violation fields and filtering.

### Maintain the Index and Files

Use `mdhop build` to create the index, `add` after creating files, `update` after editing files, `set` for one frontmatter key, `move` for link-safe moves, and `delete` for index/disk removal.

```bash
mdhop build
mdhop add --file Notes/NewNote.md --format json
mdhop update --file Notes/Design.md --format json
mdhop set --file Notes/Design.md --key reviewed --value 2026-07-04 --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop move --from Notes/Old.md --to Notes/New.md --format json
mdhop move --from Notes/Project.md --to-template "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}" --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
mdhop delete --file Notes/Obsolete.md --rm --format json
```

Run the matching `mdhop <command> --help` before performing file-changing operations, especially `move` and `delete --rm`.

### Repair or Rewrite Links

Use `mdhop disambiguate` to rewrite ambiguous basename links, `repair` for broken path or vault-escape links, `simplify` to shorten safe path links, and `convert` to switch link syntax.

```bash
mdhop repair --dry-run --format json
mdhop simplify --dry-run --format json
mdhop convert --to wikilink --dry-run --format json
```

Run `mdhop disambiguate --help`, `mdhop repair --help`, `mdhop simplify --help`, or `mdhop convert --help` for exact safety notes and output fields.

### Initialize Metadata Schema

Use `mdhop init-meta` to scaffold `mdhop.yaml` `meta.types` from presets, a vault scan, or both.

```bash
mdhop init-meta --preset --scan
```

Run `mdhop init-meta --help` for write and comment options.

## References

Read only the reference needed for the task:

- Query/search/read-only command guide: [references/query.md](references/query.md)
- File operation and rewrite command guide: [references/commands.md](references/commands.md)
