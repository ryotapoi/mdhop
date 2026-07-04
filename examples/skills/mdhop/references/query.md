# mdhop Read-Only Command Guide

This file is a thin orientation guide for read-only commands. It intentionally does not repeat full flag tables, JSON examples, filter grammar, or field definitions. For exact syntax, output fields, defaults, and examples, run `mdhop <command> --help`.

## Relationship Queries

- `mdhop query`: start from a note, tag, phantom, or auto-detected name and return relationships such as backlinks, outgoing links, tags, two-hop related notes, snippets, head lines, or entry metadata.
- `mdhop resolve`: resolve one link text from one source note and inspect the resolved target.

Use `query` when you need context around a node. Use `resolve` when you need a single link's target.

```bash
mdhop query --help
mdhop resolve --help
```

## Vault-Wide Search

- `mdhop search`: find existing notes without an entry node. Use for frontmatter filters, including `&&` and `||` expressions, path filters, sort/page workflows, computed fields, samples, and count-only checks.

Use `search` instead of `query` when the task starts from a condition rather than a known note/tag/phantom.

```bash
mdhop search --help
```

## Structure and Diagnostics

- `mdhop reachable`: split notes into reachable and unreachable sets from an entry note; optionally include routes.
- `mdhop graph`: export the induced graph as JSON or Graphviz dot.
- `mdhop stats`: return index counts.
- `mdhop diagnose`: report basename conflicts, asset basename conflicts, phantoms, and optional broken anchors.

Use these commands for map-level checks, graph analysis, and structural health checks.

```bash
mdhop reachable --help
mdhop graph --help
mdhop stats --help
mdhop diagnose --help
```

## Frontmatter Checks

- `mdhop meta-check`: check whether selected frontmatter values resolve as vault paths or wikilinks.
- `mdhop meta-validate`: validate frontmatter against required keys and `mdhop.yaml` type declarations.

Use `meta-check` for reference integrity. Use `meta-validate` for schema/type conformance.

```bash
mdhop meta-check --help
mdhop meta-validate --help
```

## Agent Use

- Prefer `--format json`.
- Use `--fields` to keep output small and task-specific.
- Use command-specific help as the source of truth for valid field names, `--where` grammar, route fields, issue/violation fields, and examples.
