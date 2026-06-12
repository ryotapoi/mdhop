# mdhop Query Reference

Detailed reference for read-only commands: `query`, `search`, `resolve`, `reachable`, `graph`, `stats`, `diagnose`, `meta-check`, `meta-validate`.

## Common Options

- `--vault <path>`: Vault root directory (default: current directory)
- `--format json|text`: Output format (default: text)
- `--fields <comma-separated>`: Limit output fields

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

Available fields for `--fields`: `backlinks`, `tags`, `twohop`, `outgoing`, `head`, `snippet`, `meta`

| Field | Description |
|-------|-------------|
| `backlinks` | Notes that link to the entry point |
| `tags` | Tags the entry note has |
| `twohop` | Related notes via shared targets (A→X and B→X). Returns `via` node and its `targets` |
| `outgoing` | Outgoing links from the entry note |
| `head` | First N lines of the note (requires `--include-head`) |
| `snippet` | Lines around each link occurrence (requires `--include-snippet`) |
| `meta` | Frontmatter metadata of the entry note (opt-in: only included when explicitly listed in `--fields`) |

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

### Metadata Filter (--where)

| Flag | Description |
|------|-------------|
| `--where <expr>` | Filter result nodes by frontmatter metadata (repeatable) |

Operators:

| Operator | Syntax | Description |
|----------|--------|-------------|
| `=` | `key=value` | Exact match |
| `!=` | `key!=value` | Not equal |
| `~` | `key~pattern` | LIKE pattern (`%` and `_` wildcards) |
| `>` | `key>value` | Greater than |
| `<` | `key<value` | Less than |
| `>=` | `key>=value` | Greater than or equal |
| `<=` | `key<=value` | Less than or equal |
| EXISTS | `key` | Key exists (any value) |
| NOT EXISTS | `key NOT EXISTS` | Key does not exist |

**Logic:**
- Multiple `--where` with the same key: OR (match any)
- Multiple `--where` with different keys: AND (match all)
- Within a single `--where`, ` && ` separator: AND (even for the same key). Example: `--where "created>=2025-01-01 && created<=2025-03-31"`
- Filters apply to: backlinks, outgoing, twohop targets
- Entry node is never filtered
- Phantom, tag, and asset nodes are always excluded (no metadata)

**Values must not be quoted** — write `created>=2025-02-01`, not `created>='2025-02-01'`. Embedded quotes cause silent zero-match.

**Type-safe comparisons:** When keys are declared in `mdhop.yaml`'s `meta.types` (e.g., `date`, `number`, `semver`), comparison operators (>, <, >=, <=) use normalized sort values. Without type declarations, comparisons are lexicographic.

**Relative dates:** comparison values may use `today` or `today±Nd/w/m/y` (e.g., `updated<today-90d`, `reviewed>today-1y`). They expand to an absolute date at run time using the local date and compare as dates. The left-hand key must be declared `date` in `meta.types`; an undeclared key is stored as a string and the date guard skips it, so it never matches. Example: `--where "updated<today-90d"` finds notes not updated in the last 90 days (with `updated: date` declared).

### Path Filter and Exclude Options

| Flag | Description |
|------|-------------|
| `--path <glob>` | Include only result nodes matching the glob (repeatable, OR-joined) |
| `--exclude <glob>` | Exclude paths matching the glob pattern (repeatable) |
| `--exclude-tag <tag>` | Exclude a specific tag (repeatable, `#` prefix recommended) |
| `--no-exclude` | Ignore exclusions defined in `mdhop.yaml` |

CLI `--exclude`/`--exclude-tag` flags are merged with `mdhop.yaml` `exclude` settings.

`--path` behavior:
- Applies to: backlinks, outgoing, twohop targets, snippet
- Nodes without a path (phantom, tag) are never filtered out
- Not applied to twohop via nodes (so targets inside the range stay reachable through via nodes outside it)

#### Exclude Behavior

- Applies to: backlinks, outgoing, tags, twohop (both via and targets), snippet
- Entry node itself is never excluded
- Path glob: `*` matches any character including `/`. `?` matches a single character. Case-sensitive. `[...]` character classes are not supported (causes error)
- Tag exclude: exact match, case-insensitive
- twohop: if a via node matches an excluded tag/path, the entire via entry is removed

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

# With metadata filter
mdhop query --file Notes/Design.md --where "status=active" --fields backlinks,meta --format json

# Find backlinks missing a metadata key
mdhop query --file Notes/Design.md --where "priority NOT EXISTS" --fields backlinks --format json

# Date range filter (same-key AND via && syntax)
mdhop query --tag project --where "created>=2024-01-01 && created<=2024-03-31" --fields backlinks --format json
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
  ],
  "meta": {
    "status": ["active"],
    "priority": ["2"],
    "date": ["2024-01-15"]
  }
}
```

## search

Entry-point-free vault-wide note search. Returns notes matching metadata conditions, path filters, and supports sorting and pagination.

**Key difference from `query`**: `query` starts from a specific entry (note, tag, phantom) and returns relationships. `search` finds notes across the entire vault without an entry point.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--where <expr>` | string (repeatable) | — | Frontmatter filter (same syntax as query `--where`, including relative dates) |
| `--sort <key>` | string | — | Sort by meta key or computed field: `key` (ascending) or `-key` (descending). Default: path order |
| `--limit <N>` | int | 0 | Maximum results (0 = unlimited) |
| `--offset <N>` | int | 0 | Skip first N results |
| `--path <glob>` | string (repeatable) | — | Include only paths matching glob (OR-joined) |
| `--exclude <glob>` | string (repeatable) | — | Exclude paths matching glob |
| `--no-exclude` | bool | false | Disable config file exclusions |
| `--include-head <N>` | int | 0 | Include first N lines of each note |
| `--fields <list>` | string | — | Available: `meta` (all meta), `meta.<key>` (a single key), `lines`, `outgoing_count`, `incoming_count` (computed, opt-in) |
| `--no-tags` | bool | false | Include only notes with no tag edges |
| `--no-outgoing` | bool | false | Include only notes with no outgoing edges, including tag edges |
| `--no-incoming` | bool | false | Include only notes with no incoming edges |
| `--format json\|text` | string | text | Output format |

### Sorting

- No `--sort`: results ordered by path (ascending)
- `--sort priority`: ascending by meta key `priority`
- `--sort -priority`: descending by meta key `priority`
- `--sort -lines` / `--sort outgoing_count` / `--sort incoming_count`: sort by a computed field
- Meta-key sorting uses the normalized `sort_value` column — type-safe when key is declared in `mdhop.yaml`
- Notes without the sort key appear last (nulls last)

### Computed Fields and Meta Keys

`--fields` and `--sort` accept computed fields in addition to `meta`:

| Field | Description |
|-------|-------------|
| `lines` | Note line count (persisted at index time; counts the whole file including frontmatter) |
| `outgoing_count` | Number of outgoing link edges (including tag edges) |
| `incoming_count` | Number of incoming link edges |
| `meta.<key>` | Output only the given frontmatter key (e.g., `meta.status`); `meta` alone outputs all keys |

- Computed fields are opt-in: they appear in output only when listed in `--fields`
- `--fields` only adds these opt-in fields; `type` / `name` / `path` / `exists` are always in the output and must not be passed to `--fields` (unknown field error)
- `lines` is persisted, so it is available for `--sort` without recomputation; the edge counts are aggregated at query time

### Path Filter

- `--path "sub/*"`: include only matching paths (OR-joined if multiple)
- Uses SQLite GLOB: `*` matches any character including `/`, `?` matches single character
- Case-sensitive
- Empty (no `--path`) = all paths included

### Scope

- Only returns existing notes (`type=note`, `exists=true`)
- Never returns phantoms, tags, or assets
- `--no-tags`, `--no-outgoing`, and `--no-incoming` are AND-combined with other search filters
- `--no-outgoing` is edge-based and counts tag edges as outgoing

### Examples

```bash
# All active notes
mdhop search --where "status=active" --format json

# Sorted by due date, top 10
mdhop search --where "due" --sort "due" --limit 10 --format json

# Notes in daily/ directory
mdhop search --path "daily/*" --format json

# Paginated with metadata and content preview
mdhop search --where "status=draft" --fields meta --include-head 5 --limit 20 --offset 0 --format json

# Combine metadata filters: active AND high priority
mdhop search --where "status=active" --where "priority>1" --sort "-priority" --format json

# Notes missing a metadata key
mdhop search --where "priority NOT EXISTS" --format json

# Notes with no tags
mdhop search --no-tags --format json

# Notes with no outgoing edges, including tag edges
mdhop search --no-outgoing --format json

# Notes with no incoming edges
mdhop search --no-incoming --format json

# Stale notes: not updated in the last 90 days
mdhop search --where "updated<today-90d" --format json

# Biggest notes with computed fields and a single meta key
mdhop search --sort -lines --limit 10 --fields lines,outgoing_count,meta.status --format json
```

### JSON Output Example

```json
{
  "total": 42,
  "items": [
    {
      "type": "note",
      "name": "ProjectAlpha",
      "path": "projects/ProjectAlpha.md",
      "exists": true,
      "lines": 320,
      "outgoing_count": 12,
      "incoming_count": 5,
      "meta": {
        "status": ["active"]
      },
      "head": ["# Project Alpha", "High-priority project for Q1."]
    }
  ]
}
```

- `total`: count of all matching notes before `--limit`/`--offset` (for pagination)
- `meta`: present when `--fields meta` (all keys) or `--fields meta.<key>` (only the listed keys) is specified
- `lines` / `outgoing_count` / `incoming_count`: present only when the matching computed field is listed in `--fields`
- `head`: only present when `--include-head N` is specified

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

## reachable

List notes reachable / unreachable from an entry note by following links (BFS over the index).

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--from <path>` | Yes | Entry note (vault-relative path). Assets or unregistered paths are errors |
| `--path <glob>` | No | Restrict the target note set to matching paths (repeatable, OR-joined). Default: all notes |
| `--exclude <glob>` | No | Exclude notes from the target set (repeatable) |
| `--route` | No | Also output the shortest route to each reachable note |
| `--fields <list>` | No | Available: `reachable`, `unreachable` |
| `--format json\|text` | No | Output format |

### Behavior

- Traverses outgoing links only: `wikilink`, `markdown`, `frontmatter_wikilink`, `frontmatter_path`. Tag edges are not traversed (sharing a tag does not count as reachable)
- The entry note itself is reachable at 0 hops when it is inside the target set
- Notes outside the target set still relay traversal but appear in neither list
- `from` is always included in JSON output regardless of `--fields`; `routes` appears only with `--route` (route connectors may lie outside the target set)
- `--path` / `--exclude` are CLI-only; `mdhop.yaml` `exclude` settings do not apply

### Examples

```bash
# Which docs notes are orphaned from the index page?
mdhop reachable --from docs/index.md --path "docs/*" --format json

# Show how each note is reached.
mdhop reachable --from docs/index.md --path "docs/*" --route --format json
```

### JSON Output Example

```json
{
  "from": "docs/index.md",
  "reachable": ["docs/index.md", "docs/sub.md", "docs/leaf.md"],
  "unreachable": ["docs/orphan.md"],
  "routes": {
    "docs/leaf.md": ["docs/index.md", "docs/sub.md", "docs/leaf.md"]
  }
}
```

## graph

Export the link graph as an induced subgraph for external visualization or analysis (similarity, clustering, etc. are left to the consumer).

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--path <glob>` | No | Restrict the node set to matching paths (repeatable, OR-joined). Default: all |
| `--exclude <glob>` | No | Exclude nodes from the node set (repeatable) |
| `--include-phantoms` | No | Include phantom nodes referenced from in-set notes (default: excluded) |
| `--format json\|dot` | No | Output format (default: json). No `text`, no `--fields` |

### Behavior

- Nodes are existing notes and assets matching the filters; tag nodes are never exported (tag edges drop out too)
- Edges are link occurrences whose both endpoints are in the node set (induced subgraph), one edge per occurrence — aggregate weights on the consumer side
- Node `id` is an export-scoped reference key used by `edges[].source/target`; it is not stable across builds
- dot labels: path for notes/assets, `(phantom) <name>` for phantoms
- `--path` / `--exclude` are CLI-only; `mdhop.yaml` `exclude` settings do not apply

### Examples

```bash
mdhop graph --path "docs/*" --format json
mdhop graph --format dot --include-phantoms > vault.dot
```

### JSON Output Example

```json
{
  "nodes": [
    {"id": 1, "type": "note", "name": "a", "path": "docs/a.md"},
    {"id": 7, "type": "phantom", "name": "Ghost", "path": ""}
  ],
  "edges": [
    {"source": 1, "target": 7, "link_type": "wikilink"}
  ]
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

Available fields for `--fields`: `basename_conflicts`, `asset_basename_conflicts`, `phantoms`, `anchors`

| Field | Description |
|-------|-------------|
| `basename_conflicts` | Note files sharing the same basename (potential ambiguity source) |
| `asset_basename_conflicts` | Asset files sharing the same basename |
| `phantoms` | Nodes referenced by links but not present on disk |
| `anchors` | Broken heading anchors: `[[note#heading]]` / `[text](note.md#fragment)` whose fragment is not a heading in the (existing) target note. **Opt-in** — unlike other fields, an empty `--fields` does not enable it (it reads target notes from disk). Reported as `broken_anchors` |

Anchor matching uses Obsidian-compatible normalization (strip `#`, drop punctuation/symbols, collapse whitespace; case and accents preserved). Block references (`#^id`) are not checked. Targets that are phantoms or assets are out of scope (a missing note is the phantom detector's job).

### Path Filter

| Flag | Description |
|------|-------------|
| `--path <glob>` | Restrict to source notes matching the glob (repeatable, OR-joined) |
| `--exclude <glob>` | Exclude source notes matching the glob (repeatable) |

With filters, `phantoms` returns only phantoms referenced from the filtered notes, and the conflict fields return only conflict groups targeted by basename-style links written in the filtered notes (i.e., actual resolution risks). CLI-only; `mdhop.yaml` `exclude` settings do not apply to diagnose.

### Example

```bash
mdhop diagnose --format json

# Diagnose only one subtree.
mdhop diagnose --path "projects/*" --format json

# Opt in to broken heading anchor detection.
mdhop diagnose --fields anchors --format json
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

With `--fields anchors`:

```json
{
  "broken_anchors": [
    {
      "source_path": "docs/guide.md",
      "raw_link": "[[spec#Old Section]]",
      "target_path": "docs/spec.md",
      "fragment": "Old Section"
    }
  ]
}
```

## meta-check

Check that the values of given frontmatter keys resolve to a real path or wikilink target in the vault. This checks *reference existence* (whether the value points at something real), complementing `meta-validate` (schema conformance).

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--key <name>` | Yes | Frontmatter key to check (repeatable) |
| `--kind path\|wikilink` | No | How to interpret values (default: `path`) |
| `--path <glob>` | No | Restrict source notes to matching paths (repeatable, OR-joined) |
| `--exclude <glob>` | No | Exclude source notes matching the glob (repeatable) |
| `--format json\|text` | No | Output format |

### Behavior

- `--kind path` (default) interprets values as raw paths, using markdown-link resolution: `./` / `../` note-relative, a value containing `/` vault-relative, a bare name basename-resolved
- With `--kind path`, values ending in `/` are treated as directory references; an existing directory is valid, and a missing directory is reported as `not_found`
- `--kind wikilink` interprets values as `[[...]]`
- List and scalar values are already expanded per value in the `meta` table, so there is no list/scalar distinction
- URL values (containing `://`) and empty values are allowed (not reported)
- `reason` is one of: `not_found`, `ambiguous` (basename resolves to multiple notes), `vault_escape`, `not_wikilink` (`--kind wikilink` but the value is not `[[...]]`)
- `--path` / `--exclude` are CLI-only; `mdhop.yaml` `exclude` settings do not apply

### Examples

```bash
# Do the sources: paths in docs/ all resolve?
mdhop meta-check --key sources --kind path --path "docs/*" --format json

# Are the related: wikilinks real?
mdhop meta-check --key related --kind wikilink --format json
```

### JSON Output Example

```json
{
  "issues": [
    {
      "source_path": "docs/index.md",
      "key": "sources",
      "value": "./missing.md",
      "reason": "not_found"
    }
  ]
}
```

## meta-validate

Check that frontmatter conforms to the declared schema: required keys are present, and keys typed in `mdhop.yaml`'s `meta.types` hold values that parse as their type or belong to their ordered list. This checks *schema conformance*, complementing `meta-check` (reference existence).

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--require <key>` | No | Key that must hold a non-empty value (repeatable) |
| `--path <glob>` | No | Restrict source notes to matching paths (repeatable, OR-joined) |
| `--exclude <glob>` | No | Exclude source notes matching the glob (repeatable) |
| `--format json\|text` | No | Output format |

At least one of `--require` or a non-string `meta.types` declaration must be present; otherwise there is nothing to check and the command errors.

### Behavior

- `--require <key>` reports `missing` for every note lacking a non-empty value for the key. Empty / null frontmatter values are dropped at index time, so `key:` with no value is reported as `missing` (same defect as an absent key)
- Keys declared `date` / `number` / `semver` in `meta.types` whose values fail to parse are reported as `type`; keys declared `ordered` whose values fall outside the list are reported as `enum`
- Type/enum checks always run (driven by `meta.types`), with or without `--require`; `--require` only adds the missing-key check
- `string` and undeclared keys carry no type/enum constraint and are not checked
- Type/enum detection reads the `value_type` stored at index time, so rebuild after changing `meta.types` (the same "rebuild the DB on change" assumption every read command relies on)
- `--path` / `--exclude` are CLI-only; `mdhop.yaml` `exclude` settings do not apply

### Examples

```bash
# Every doc must declare type, status, updated.
mdhop meta-validate --require type --require status --require updated --path "docs/*" --format json

# Just type/enum conformance from meta.types (no required keys).
mdhop meta-validate --format json
```

### JSON Output Example

```json
{
  "violations": [
    {"source_path": "docs/a.md", "key": "status", "value": "", "reason": "missing"},
    {"source_path": "docs/b.md", "key": "updated", "value": "someday", "reason": "type"},
    {"source_path": "docs/c.md", "key": "severity", "value": "urgent", "reason": "enum"}
  ]
}
```
