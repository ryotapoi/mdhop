package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const mentionsHelp = `Usage: mdhop mentions (--file <path>|--phantom <name>) [options]

Find notes that write a name without linking to it. Reads every note body, so
it is a separate command from query.

Entry options:
  --file <path>     Note entry by vault-relative path. Returns incoming (notes
                    writing the entry name) and outgoing (known note and phantom
                    names written in the entry body).
  --phantom <name>  Phantom entry. Returns incoming only.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --partial        Also match inside Latin words.
  --path <glob>    Return only notes whose path matches any glob (repeatable).
  --hide-path <glob>  Hide notes whose path matches a glob (repeatable).
  --no-phantom     Do not return phantom names (outgoing only).
  --where <expr>   Return only notes whose frontmatter matches (repeatable, AND).
                   Same syntax as query --where.
  --limit <N>      Return at most N mentions per direction (N > 0). Default: all.
  --vault <path>   Vault root directory. Default: ".".
  --format json|text  Output format. Default: text.

Names are note basenames without extension and phantom names, compared
case-insensitively after NFC normalization. By default a match must not start
or end inside a Latin word ("git" does not match "github"); digits, kana, kanji,
and symbols are not boundaries ("ios" matches "iOS26"). Frontmatter, code
fences, inline code, wikilinks, Markdown links (including their text),
reference definitions, and URLs are not searched. The entry itself, nodes
linked with the entry in either direction, and notes hidden by [query.hide]
are not returned. Notes sharing a name are all returned.

--path, --hide-path, and --no-phantom narrow both directions before --limit.
--hide-path adds to [query.hide] paths; a note matching both --path and
--hide-path is hidden. Phantoms have no path, so only --no-phantom removes
them. Globs follow query: case-sensitive, * crosses /, and [] character
classes are unsupported. Quote globs to prevent shell expansion.

--where also narrows both directions before --limit and combines with the
other options by AND. As in query, it never returns phantoms, and a note
without the key fails key=value, key!=value, comparisons, and ~, but passes
key NOT EXISTS.

Each note appears once with its first mention: matched_text and location are
taken from the file before NFC normalization, and excerpt is the physical line
of the match start. location has the query --include-location form. Bodies are
read from disk as they are now; notes deleted since the last build are skipped.

The JSON fields are entry, incoming, outgoing (absent for --phantom), and
total, which counts each direction after filtering and before --limit.

Example:
  mdhop mentions --file Plan.md --format json
`

func runMentions(args []string) error {
	fs := flag.NewFlagSet("mentions", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, mentionsHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	file := fs.String("file", "", "note entry path")
	phantom := fs.String("phantom", "", "phantom entry name")
	partial := fs.Bool("partial", false, "match inside Latin words")
	limit := fs.Int("limit", 0, "maximum mentions per direction")
	format := fs.String("format", "text", "output format (json or text)")
	noPhantom := fs.Bool("no-phantom", false, "exclude phantom names")
	var pathPatterns, hidePaths, whereExprs multiString
	fs.Var(&pathPatterns, "path", "include note path glob (repeatable)")
	fs.Var(&hidePaths, "hide-path", "hide note path glob (repeatable)")
	fs.Var(&whereExprs, "where", "metadata filter expression (repeatable, AND)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	cfg, err := core.LoadConfig(*vault, locations)
	if err != nil {
		return err
	}
	filter, err := core.NewQueryFilter(cfg, core.QueryFilterOptions{Hide: core.ExcludeConfig{Paths: hidePaths}})
	if err != nil {
		return err
	}
	wc, err := core.ParseWhere(whereExprs, cfg.Meta)
	if err != nil {
		return err
	}
	opts := core.MentionsOptions{Partial: *partial, Filter: filter, Path: pathPatterns, NoPhantom: *noPhantom, Where: wc}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "limit" {
			opts.Limit = limit
		}
	})
	result, err := core.Mentions(*vault, core.MentionsEntry{File: *file, Phantom: *phantom}, opts, locations)
	if err != nil {
		return err
	}
	if *format == "json" {
		return printMentionsJSON(os.Stdout, result)
	}
	return printMentionsText(os.Stdout, result)
}
