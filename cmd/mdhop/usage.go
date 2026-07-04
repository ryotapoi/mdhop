package main

import (
	"flag"
	"fmt"
)

func commandUsage(fs *flag.FlagSet, help string) func() {
	return func() {
		fmt.Fprint(fs.Output(), help)
	}
}

const buildHelp = `Usage: mdhop build [--vault <path>]

Build the SQLite index for an Obsidian-style Markdown vault.

Options:
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  No stdout output on success. Creates or replaces the vault index under .mdhop/.
  Warnings, if any, are written to stderr.

Examples:
  mdhop build
  mdhop build --vault ~/Notes

`

const addHelp = `Usage: mdhop add --file <path> [--file <path>...] [--no-auto-disambiguate] [--vault <path>] [--format json|text]

Add newly created files to the index. Use this after writing the files.

Options:
  --file <path>              Required, repeatable. Vault-relative file path to add.
  --no-auto-disambiguate     Optional. Disable automatic rewriting when a new basename collision would otherwise be made safe.
  --vault <path>             Optional. Vault root directory. Default: ".".
  --format json|text         Optional. Output format. Default: text.

Behavior notes:
  Registered files passed to --file fail.
  Files being added fail when they contain ambiguous basename links.
  When basename collisions occur, existing basename links are automatically rewritten to full paths where their meaning can be preserved; --no-auto-disambiguate disables this.
  Existing basename links to phantom nodes fail even with auto-disambiguation when the added files contain multiple files with that basename, because there is no safe rewrite target.
  When meta.link_keys is configured, frontmatter raw path values cannot be rewritten; add fails before changing anything if existing raw path values would resolve differently.

Output fields:
  added      Files added as real nodes.
  promoted   Phantom nodes promoted to real files.
  rewritten  Files whose links were rewritten during automatic disambiguation.

Examples:
  mdhop add --file Notes/NewNote.md --format json
  mdhop add --file Notes/A.md --file Notes/B.md --format json
  mdhop add --file Notes/NewNote.md --no-auto-disambiguate

`

const updateHelp = `Usage: mdhop update --file <path> [--file <path>...] [--vault <path>] [--format json|text]

Re-index registered files after editing them. If a registered file is missing on disk, it is handled like delete.

Options:
  --file <path>       Required, repeatable. Vault-relative registered file path to update.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  updated    Files re-indexed from disk.
  deleted    Registered nodes removed because no references remain.
  phantomed  Missing files kept as phantom nodes because references remain.

Examples:
  mdhop update --file Notes/Design.md --format json
  mdhop update --file Notes/A.md --file Notes/B.md --format json

`

const setHelp = `Usage: mdhop set --file <path> --key <name> --value <value> [--vault <path>] [--format json|text]

Set one existing frontmatter document key and update the index.

Options:
  --file <path>       Required. Vault-relative Markdown file to edit.
  --key <name>        Required. Frontmatter key to set.
  --value <value>     Required. YAML scalar value to write exactly as provided; relative dates are not expanded.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Files without frontmatter fail; frontmatter is not created.
  Missing keys are inserted before the closing ---.
  List values, multi-line values, and duplicate keys are rejected.

Output fields:
  file     Edited file path.
  key      Updated key.
  value    Written value.
  created  Whether the key was newly inserted.

Examples:
  mdhop set --file Notes/Design.md --key reviewed --value 2026-07-04 --format json
  mdhop set --file Notes/Design.md --key status --value active

`

const deleteHelp = `Usage: mdhop delete --file <path> [--file <path>...] [--rm] [--vault <path>] [--format json|text]

Remove registered files from the index. With --rm, remove them from disk as well.

Options:
  --file <path>       Required, repeatable. Vault-relative file or directory. A trailing / or disk directory enables directory mode.
  --rm                Optional. Also delete registered files from disk.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  deleted    Nodes removed from the index.
  phantomed  Deleted files kept as phantom nodes because references remain.

Examples:
  mdhop delete --file Notes/Obsolete.md --rm --format json
  mdhop delete --file Notes/archive/ --rm --format json
  mdhop delete --file Notes/Obsolete.md

`

const moveHelp = `Usage: mdhop move --from <path> --to <path> [--vault <path>] [--format json|text]

Move a registered file or directory and rewrite links needed to preserve meaning.

Options:
  --from <path>       Required. Vault-relative source file or directory. A trailing / or disk directory enables directory mode.
  --to <path>         Required. Vault-relative destination file or directory.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  The source file fails stale detection when its mtime does not match the DB record; external files rewritten as collateral are not stale-checked.
  If --from is missing on disk and --to already exists, the move is treated as already completed and only link rewrites plus DB updates are performed.
  Existing --to paths on disk fail to prevent overwrites.
  Directory moves fail when --from and --to contain each other, such as --from sub --to sub/inner.
  When meta.link_keys is configured, frontmatter raw path values cannot be rewritten; move fails before changing anything if existing raw path values would resolve differently.

Output fields:
  from       Source path for single-file moves.
  to         Destination path for single-file moves.
  moved      Directory-mode moves as an array of from/to pairs.
  rewritten  Files whose links were rewritten.

Examples:
  mdhop move --from Notes/Old.md --to Notes/New.md --format json
  mdhop move --from OldDir/ --to NewDir/ --format json

`

const disambiguateHelp = `Usage: mdhop disambiguate --name <basename> [--target <path>] [--file <path>] [--scan] [--vault <path>] [--format json|text]

Rewrite ambiguous basename links to full paths.

Options:
  --name <basename>   Required. Basename link name to rewrite.
  --target <path>     Optional. Required when the basename has multiple candidates.
  --file <path>       Optional. Limit rewriting to one vault-relative file.
  --scan              Optional. Scan files without requiring an existing DB; useful before build.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.

Examples:
  mdhop disambiguate --name a --target Notes/a.md --format json
  mdhop disambiguate --name a --scan --format json
  mdhop disambiguate --name a --file Notes/Design.md --target Notes/a.md

`

const simplifyHelp = `Usage: mdhop simplify [--dry-run] [--file <path>...] [--vault <path>] [--format json|text]

Shorten path links to basename links when the shortened form remains unambiguous.

Options:
  --dry-run           Optional. Report changes without writing files.
  --file <path>       Optional, repeatable. Limit rewriting to specific vault-relative files.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links or files skipped because they could not be simplified safely.

Examples:
  mdhop simplify --dry-run --format json
  mdhop simplify --file Notes/Design.md --format json
  mdhop simplify

`

const repairHelp = `Usage: mdhop repair [--dry-run] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Rewrite broken path links and vault-escape links to basename links when safe.

Options:
  --dry-run           Optional. Report changes without writing files.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links skipped because no safe repair target was available.

Examples:
  mdhop repair --dry-run --format json
  mdhop repair --path "docs/*" --exclude "docs/archive/*" --dry-run --format json
  mdhop repair

`

const convertHelp = `Usage: mdhop convert --to <wikilink|markdown> [--dry-run] [--file <path>...] [--vault <path>] [--format json|text]

Convert between wikilink and Markdown link syntax.

Options:
  --to <wikilink|markdown>  Required. Target link syntax.
  --dry-run                 Optional. Report changes without writing files.
  --file <path>             Optional, repeatable. Limit conversion to specific vault-relative files.
  --vault <path>            Optional. Vault root directory. Default: ".".
  --format json|text        Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were converted.

Examples:
  mdhop convert --to wikilink --dry-run --format json
  mdhop convert --to markdown --file Notes/Design.md --format json
  mdhop convert --to wikilink

`

const resolveHelp = `Usage: mdhop resolve --from <path> --link <link text> [--fields <list>] [--vault <path>] [--format json|text]

Resolve one link as written from a source note.

Options:
  --from <path>       Required. Vault-relative source note path.
  --link <link text>  Required. Link text, such as '[[Spec]]' or '[Spec](Spec.md)'.
  --fields <list>     Optional. Comma-separated output fields.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  type     Target type: note, phantom, tag, url, or asset.
  name     Display name.
  path     Vault-relative path for note and asset targets.
  exists   Existence flag for note and asset targets.
  subpath  Heading or block fragment such as #Heading or #^block.

Examples:
  mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
  mdhop resolve --from Notes/Design.md --link '[Spec](Spec.md)' --fields type,path --format json
  mdhop resolve --from Notes/Design.md --link '#architecture'

`

const queryHelp = `Usage: mdhop query (--file <path>|--tag <name>|--phantom <name>|--name <name>) [options]

Return related information for one entry note, tag, phantom, or auto-detected name.

Entry options:
  --file <path>     Note entry by vault-relative path.
  --tag <name>      Tag entry. Leading # is optional.
  --phantom <name>  Phantom entry.
  --name <name>     Auto-detect note, phantom, or tag. Ambiguous names fail.

Options:
  --fields <list>             Optional. Comma-separated fields: backlinks,tags,twohop,outgoing,head,snippet,meta.
  --include-head <N>          Include the first N content lines as head.
  --include-snippet <N>       Include N context lines around matching links as snippet.
  --max-backlinks <N>         Backlinks limit. Default: 100.
  --max-twohop <N>            Two-hop result limit. Default: 100.
  --max-via-per-target <N>    Via entries per two-hop target. Default: 10.
  --path <glob>               Optional, repeatable. Include result paths matching any glob.
  --exclude <glob>            Optional, repeatable. Exclude result paths matching the glob.
  --exclude-tag <tag>         Optional, repeatable. Exclude matching tags.
  --no-exclude                Ignore mdhop.yaml exclude settings.
  --where <expr>              Optional, repeatable. Metadata filter using =,!=,~,>,<,>=,<=, EXISTS/NOT EXISTS, coalesce(...), and today±N d/w/m/y dates.
  --vault <path>              Optional. Vault root directory. Default: ".".
  --format json|text          Optional. Output format. Default: text.

Fields:
  backlinks  Notes linking to the entry.
  outgoing   Links from the entry note.
  twohop     Related notes sharing outgoing targets with the entry; includes via targets.
  tags       Tags on the entry note.
  head       First N content lines, enabled by --include-head.
  snippet    Link-adjacent context, enabled by --include-snippet.
  meta       Entry frontmatter metadata; opt-in via --fields.

Examples:
  mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
  mdhop query --tag architecture --fields backlinks --format json
  mdhop query --file Notes/Design.md --where "status=active" --fields backlinks,meta --format json

`

const searchHelp = `Usage: mdhop search [--where <expr>...] [--path <glob>...] [options]

Search existing notes without an entry node.

Options:
  --where <expr>       Optional, repeatable. Metadata filter using the same syntax as query.
  --path <glob>        Optional, repeatable. Include note paths matching any glob.
  --exclude <glob>     Optional, repeatable. Exclude note paths matching the glob.
  --no-exclude         Ignore mdhop.yaml exclude settings.
  --sort <key|-key>    Sort by metadata key or computed field; prefix - for descending.
  --limit <N>          Limit result count.
  --offset <N>         Skip result rows before returning results.
  --sample <N>         Randomly sample N matches; cannot be combined with --limit, --offset, or --sort.
  --count              Return only the match count; cannot be combined with --fields, --include-head, --sample, --sort, --limit, or --offset.
  --no-tags            Only notes with no tag edges.
  --no-outgoing        Only notes with no outgoing edges.
  --no-incoming        Only notes with no incoming edges.
  --include-head <N>   Include the first N content lines.
  --fields <list>      Optional. meta, meta.<key>, lines, outgoing_count, incoming_count.
  --vault <path>       Optional. Vault root directory. Default: ".".
  --format json|text   Optional. Output format. Default: text.

Fields:
  meta            All frontmatter metadata.
  meta.<key>      One frontmatter key.
  lines           File line count recorded at build/update time.
  outgoing_count  Count of outgoing edges, including tag edges.
  incoming_count  Count of incoming edges.

Examples:
  mdhop search --where "status=active" --sort "-priority" --fields meta --format json
  mdhop search --where "updated<today-90d" --count --format json
  mdhop search --where "status=active" --sample 10 --format json

`

const reachableHelp = `Usage: mdhop reachable --from <path> [--path <glob>...] [--exclude <glob>...] [--route] [--fields <list>] [--vault <path>] [--format json|text]

Split notes into reachable and unreachable sets from an entry note by following links.

Options:
  --from <path>       Required. Vault-relative entry note path.
  --path <glob>       Optional, repeatable. Include target note paths matching any glob.
  --exclude <glob>    Optional, repeatable. Exclude target note paths matching the glob.
  --route             Optional. Include shortest routes to reachable notes.
  --fields <list>     Optional. Comma-separated fields: reachable,unreachable.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  from         Normalized entry path; always included in JSON.
  reachable    Notes reachable from --from within the target set.
  unreachable  Notes in the target set not reachable from --from.
  routes       Shortest routes to reachable notes; only with --route.

Examples:
  mdhop reachable --from index.md --path "docs/*" --route --format json
  mdhop reachable --from index.md --fields unreachable --format json

`

const graphHelp = `Usage: mdhop graph [--path <glob>...] [--exclude <glob>...] [--include-phantoms] [--vault <path>] [--format json|dot]

Export an induced link graph for existing notes and assets.

Options:
  --path <glob>       Optional, repeatable. Include node paths matching any glob.
  --exclude <glob>    Optional, repeatable. Exclude node paths matching the glob.
  --include-phantoms  Optional. Include phantom nodes referenced from included notes.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|dot   Optional. Output format. Default: json. This command has no text format and no --fields.

Output:
  json  Object with nodes[] and edges[] for machine processing.
  dot   Graphviz digraph for visualization.

Examples:
  mdhop graph --path "docs/*" --format json
  mdhop graph --format dot --include-phantoms

`

const statsHelp = `Usage: mdhop stats [--fields <list>] [--vault <path>] [--format json|text]

Show index statistics for the vault.

Options:
  --fields <list>     Optional. Comma-separated fields.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  notes_total     Total note nodes.
  notes_exists    Existing note nodes.
  edges_total     Total edge occurrences.
  tags_total      Tag nodes.
  phantoms_total  Phantom nodes.
  assets_total    Asset nodes.

Examples:
  mdhop stats --format json
  mdhop stats --fields notes_exists,edges_total --format json

`

const diagnoseHelp = `Usage: mdhop diagnose [--path <glob>...] [--exclude <glob>...] [--fields <list>] [--vault <path>] [--format json|text]

Report basename conflicts, asset basename conflicts, phantom references, and optional broken anchors.

Options:
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --fields <list>     Optional. basename_conflicts,asset_basename_conflicts,phantoms,anchors.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  basename_conflicts        Note basename collision groups.
  asset_basename_conflicts  Asset filename collision groups.
  phantoms                  Phantom names referenced from notes.
  anchors                   Broken heading anchors; opt-in and not included when --fields is omitted.

Examples:
  mdhop diagnose --format json
  mdhop diagnose --fields anchors --format json
  mdhop diagnose --path "projects/*" --format json

`

const metaCheckHelp = `Usage: mdhop meta-check --key <name> [--key <name>...] [--kind path|wikilink] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Check whether frontmatter values resolve to real vault paths or wikilinks.

Options:
  --key <name>          Required, repeatable. Frontmatter key to inspect.
  --kind path|wikilink  Optional. Interpret values as raw paths or wikilinks. Default: path.
  --path <glob>         Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>      Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>        Optional. Vault root directory. Default: ".".
  --format json|text    Optional. Output format. Default: text.

Output fields:
  issues[]     One item per unresolved value.
  source_path  Note containing the frontmatter value.
  key          Frontmatter key.
  value        Unresolved value.
  reason       not_found, ambiguous, vault_escape, or not_wikilink.

Examples:
  mdhop meta-check --key sources --kind path --format json
  mdhop meta-check --key related --kind wikilink --format json
  mdhop meta-check --key sources --path "docs/*" --format json

`

const metaValidateHelp = `Usage: mdhop meta-validate [--require <key>...] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Validate frontmatter against required keys and mdhop.yaml meta type declarations.

Options:
  --require <key>     Optional, repeatable. Require a non-empty value for this key; combined with mdhop.yaml meta.profiles for this run only.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Fails if there is no --require, no mdhop.yaml meta.profiles, and no non-string meta.types declaration.

Output fields:
  violations[]  One item per schema violation.
  source_path   Note containing the violation.
  key           Frontmatter key.
  value         Invalid value when applicable.
  reason        missing, type, or enum.

Examples:
  mdhop meta-validate --require type --require status --format json
  mdhop meta-validate --format json

`

const initMetaHelp = `Usage: mdhop init-meta (--preset|--scan) [--write] [--no-comment] [--vault <path>]

Generate mdhop.yaml meta type definitions from presets, a vault scan, or both.

Options:
  --preset        Required unless --scan is set. Include recommended preset type definitions.
  --scan          Required unless --preset is set. Infer type definitions from vault frontmatter.
  --write         Optional. Write to mdhop.yaml instead of stdout.
  --no-comment    Optional. Omit explanatory comments from generated YAML.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  YAML is written to stdout by default. With --write, mdhop.yaml is updated in place.

Examples:
  mdhop init-meta --preset --scan
  mdhop init-meta --preset --scan --write
  mdhop init-meta --scan --no-comment

`
