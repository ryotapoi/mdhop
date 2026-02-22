# mdhop Query Reference

Detailed reference for read-only mdhop commands: `query`, `resolve`, `stats`, `diagnose`.

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

Available fields for `--fields`: `backlinks`, `tags`, `twohop`, `outgoing`, `head`, `snippet`

| Field | Description |
|-------|-------------|
| `backlinks` | Notes that link to the entry point |
| `tags` | Tags the entry note has |
| `twohop` | Related notes via shared targets (A→X and B→X). Returns `via` node and its `targets` |
| `outgoing` | Outgoing links from the entry note |
| `head` | First N lines of the note (requires `--include-head`) |
| `snippet` | Lines around each link occurrence (requires `--include-snippet`) |

Each node in backlinks/outgoing/twohop includes a `type` field (`note`, `phantom`, or `tag`). Notes include `name`, `path`, `exists`. Phantoms and tags include `name`.

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
| `type` | `note`, `phantom`, `tag`, or `url` |
| `name` | Display name (basename for notes, `#`-prefixed for tags) |
| `path` | Vault-relative path (notes only) |
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

Available fields for `--fields`: `notes_total`, `notes_exists`, `edges_total`, `tags_total`, `phantoms_total`

| Field | Description |
|-------|-------------|
| `notes_total` | Total number of note nodes |
| `notes_exists` | Notes that exist on disk |
| `edges_total` | Total link occurrences |
| `tags_total` | Total unique tags |
| `phantoms_total` | Total phantom (unresolved) nodes |

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
  "phantoms_total": 12
}
```

## diagnose

Detect issues in the vault index.

### Fields

Available fields for `--fields`: `basename_conflicts`, `phantoms`

| Field | Description |
|-------|-------------|
| `basename_conflicts` | Files sharing the same basename (potential ambiguity source) |
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
