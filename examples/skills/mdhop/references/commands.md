# mdhop Command Guide

This file is a thin orientation guide for commands that build or mutate the index, files, or links. It intentionally does not repeat full flag tables, JSON examples, or detailed behavior rules. For exact syntax, output fields, defaults, and examples, run `mdhop <command> --help`.

## Index Lifecycle

- `mdhop build`: create or rebuild the vault index. Use before query/search workflows or after scan-based rewrite commands.
- `mdhop add`: register newly written files. Use after creating files, not before their content is final.
- `mdhop update`: refresh registered files after editing. Use when file contents changed and the index must match disk.
- `mdhop set`: update one frontmatter key in one file and refresh the index. Use `--date <expr>` for relative dates such as `today-90d`; use it instead of hand-editing a single scalar metadata value.

Help:

```bash
mdhop build --help
mdhop add --help
mdhop update --help
mdhop set --help
```

## File-Safe Mutations

- `mdhop move`: move or rename a registered file or directory while preserving link meaning. Note moves can use `--to-template` to expand the destination from indexed frontmatter, including directory dry-run planning.
- `mdhop delete`: remove registered files from the index, optionally from disk with `--rm`.

Use these instead of raw `mv` or `rm` for indexed vault content. Check help before destructive operations.

```bash
mdhop move --help
mdhop delete --help
```

## Link Rewrite Tools

- `mdhop disambiguate`: rewrite ambiguous basename links to full paths.
- `mdhop repair`: repair broken path links and vault-escape links when safe.
- `mdhop simplify`: shorten path links to basename links when unambiguous.
- `mdhop convert`: convert between wikilink and Markdown link syntax.

Most rewrite tools can run in preview-style workflows with `--dry-run` when the command supports it. After scan-based rewrite work, rebuild the index.

```bash
mdhop disambiguate --help
mdhop repair --help
mdhop simplify --help
mdhop convert --help
```

## Metadata Setup

- `mdhop init-meta`: generate `mdhop.yaml` metadata type declarations from presets, a vault scan, or both.

```bash
mdhop init-meta --help
```

## Agent Use

- Prefer `--format json` when the command returns machine-readable output.
- Treat paths as vault-relative unless the command help says otherwise.
- For full output field definitions, copy-pasteable examples, required flags, and safety notes, use the command-specific help as the source of truth.
