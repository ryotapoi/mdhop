---
name: mdhop
description: Use mdhop for Markdown vault search, link queries, metadata filters, reachability checks, graph export, diagnostics, frontmatter checks and writes, and link-safe file operations.
---

# mdhop

Use `mdhop` to work with an Obsidian-style Markdown vault through its SQLite link index. Prefer it when you need structural navigation, metadata search, diagnostics, or file operations that must keep links consistent.

## Rules

- Use `--format json` for agent-facing output.
- Use `--relations` on `query` and `--fields` on `inspect`, `search`, `resolve`, `reachable`, `stats`, and `diagnose` to select only the data you need.
- Run commands from the vault root, or pass `--vault <path>`.
- Treat note paths and configuration globs as vault-relative. `--vault`, `--db`, and `--config` accept absolute paths or paths relative to the current directory.
- Keep the same vault, DB, and config selection across related commands. Use `--db` to separate indexes built with different configurations of the same vault.
- Serialize index writes such as `build`, `update`, and `migrate`; concurrent reads during a build can use the completed index.
- Do not use raw `mv`, `rm`, or `cp` for indexed vault files. Use `mdhop move`, `mdhop delete --rm`, and write-then-`mdhop add`.
- Do not hand-edit a single frontmatter key. Use `mdhop set` so the index stays in sync.
- Finish editing file contents before `mdhop add` or `mdhop update`; the index should reflect the final file state.
- After `repair`, `simplify`, `convert`, or `disambiguate --scan`, run `mdhop build`.
- For exact flags, output fields, and examples, run `mdhop <command> --help`.

## Start Here

Use `mdhop paths --format json` to inspect effective vault, config, and DB locations
without reading or creating config or DB files. The default config is
`<vault>/mdhop.toml`; a missing default means no config. `--config` selects only
that file, which must exist and contain valid TOML.

The default index is stored in `<cache>/mdhop/vaults/<vault-hash>/index.sqlite`.
Cache uses absolute `XDG_CACHE_HOME`, otherwise `~/.cache`; the hash identifies
the symlink-resolved absolute vault root. Run `mdhop build` if the index is
missing, the cache was removed, or the vault moved.

```bash
mdhop stats --format json
mdhop diagnose --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --relations backlinks,outgoing --format json
```

For independent locations, pass the same selection to each command:

```bash
mdhop build --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml
mdhop query --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml --file Index.md --format json
```

### Migrate Legacy Configuration and Index

Normal commands do not read `mdhop.yaml` or use the old `.mdhop/` index. Run
`mdhop migrate --help`, then `mdhop migrate --vault <path>` to convert legacy
YAML to TOML and rebuild the default cache index. `migrate` accepts neither
`--db` nor `--config`. Note frontmatter remains YAML.

Migration removes the old YAML and `.mdhop/` only after a successful rebuild.
If YAML and TOML both exist, preserve both and move one outside the vault before
retrying. Conversion, save, or rebuild failure retains the legacy files; cleanup
failure can leave some legacy paths after publishing the new index. Inspect
reported remaining paths before retrying.

### Check Index Drift Without Changing It

Use `status` before deciding whether to update or rebuild. It only lists untracked,
modified, and deleted indexed files; it does not change the vault or index.

```bash
mdhop status --format json
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

Use `mdhop query` when you have an entry note, tag, phantom, or name and need backlinks, outgoing links, or two-hop related notes. By default it returns all three relations. Use `mdhop inspect` for one indexed note's tags, metadata, or body preview.

```bash
mdhop query --file Notes/Design.md --relations backlinks,outgoing --format json
mdhop query --tag architecture --relations backlinks --format json
mdhop query --file Notes/Design.md --relations backlinks,outgoing --link-key sources --format json
mdhop query --file Notes/Design.md --relations backlinks --limit 20 --offset 20 --format json
mdhop query --file Notes/Design.md --relations twohop --via tag:architecture --format json
mdhop query --file Notes/Design.md --hide-path 'archive/*' --exclude-via-tag '#private' --format json
mdhop query --file Notes/Design.md --relations twohop --include-head 5 --include-snippet 0 --format json
mdhop inspect --file Notes/Design.md --fields tags,meta --include-head 5 --format json
```

`--limit` and `--offset` require exactly one explicit relation. The JSON `page.next_offset` tells you whether another page exists. `--via*` selects or filters two-hop paths; `--exclude-via*` removes paths before finding targets. `--hide*` removes matching targets from display and hides via identifiers while preserving targets found through them. Use `--link-key <key>` only for direct backlinks and outgoing links. Head lines and snippets are read only when requested; `--include-snippet 0` returns the link line itself. Run `mdhop query --help` for filters and `mdhop inspect --help` for note attributes.

### Resolve One Link

Use `mdhop resolve` when you need to know exactly what one link text resolves to from one source note.

For Markdown reference links such as `[Spec][guide]`, pass the exact indexed
link text. `resolve` reads the saved edge and definition snapshot, so run
`mdhop update --file <source>` after editing the source or `mdhop build` to
refresh the whole index. After upgrading from an older index schema, run
`mdhop build` to regenerate it.

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

Use `mdhop meta-check` to verify that frontmatter reference values point to real paths or wikilinks. Use `mdhop meta-validate` to check required keys and declared types from the selected TOML config.

```bash
mdhop meta-check --key sources --kind path --format json
mdhop meta-validate --require type --require status --format json
```

Issues and violations include source locations (`line` in JSON and `location` in text).
After upgrading, rebuild an existing index with `mdhop build` before relying on line data.
Run `mdhop meta-check --help` or `mdhop meta-validate --help` for issue/violation fields and filtering.

### Maintain the Index and Files

Use `mdhop build` to create the index, `add` after creating files, `update` after editing files, `set` for one scalar or one whole string list, `move` for link-safe moves, and `delete` for index/disk removal.

```bash
mdhop build
mdhop add --file Notes/NewNote.md --format json
mdhop update --file Notes/Design.md --format json
mdhop set --file Notes/Design.md --key reviewed --value 2026-07-04 --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop set --file Notes/Design.md --key aliases --list '["design","proposal"]' --format json
mdhop move --from Notes/Old.md --to Notes/New.md --format json
mdhop move --from Notes/Project.md --to-template "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}" --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
mdhop delete --file Notes/Obsolete.md --rm --format json
```

Use `--list` with a JSON string array to replace one key with a whole string list. `[]` writes an empty list. `--list` does not add or remove one item. Run `mdhop set --help` for how duplicates, empty strings, and existing scalars are handled.

Run the matching `mdhop <command> --help` before performing file-changing operations, especially `move` and `delete --rm`.

Reference definitions are not rewritten automatically. `add` and `move` fail
before changing files or the index if an indexed reference would change target
or become ambiguous; edit its definition first and refresh the index.

### Repair or Rewrite Links

Use `mdhop disambiguate` to rewrite ambiguous basename links, `repair` for broken path or vault-escape links, `simplify` to shorten safe path links, and `convert` to switch link syntax.
These commands leave Markdown reference links and their definitions unchanged.

```bash
mdhop repair --dry-run --format json
mdhop simplify --dry-run --format json
mdhop convert --to wikilink --dry-run --format json
```

Run `mdhop disambiguate --help`, `mdhop repair --help`, `mdhop simplify --help`, or `mdhop convert --help` for exact safety notes and output fields.

### Initialize Metadata Schema

Use `mdhop init-meta` to scaffold TOML `meta.types` from presets, a vault scan, or both. It prints TOML by default; `--write` updates the selected config, preserving existing settings, explicit types, and file permissions. A missing default config can be created, but an explicit `--config` must already exist. No index DB is opened.

```bash
mdhop init-meta --preset --scan
```

Run `mdhop init-meta --help` for write and comment options.
