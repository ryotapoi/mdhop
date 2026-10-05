package main

import (
	"flag"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

const queryHelp = `Usage: mdhop query (--file <path>|--tag <name>|--phantom <name>|--name <name>) [options]

Return backlinks, outgoing links, and notes sharing an outgoing target with one entry.

Entry options:
  --file <path>             Note entry by vault-relative path.
  --tag <name>              Tag entry. Leading # is optional.
  --phantom <name>          Phantom entry.
  --name <name>             Auto-detect note, phantom, or tag. Ambiguous names fail.

Options:
  --relations <list>        Comma-separated backlinks,outgoing,twohop. Default: all three.
  --limit <N>               Return at most N targets (N > 0); requires one explicit relation.
  --offset <N>              Skip N targets (N >= 0); requires one explicit relation.
  --via <type:value>        Select one two-hop via node by exact type and identifier.
  --via-path <glob>         Include via paths matching a glob (repeatable).
  --via-tag <tag>           Include via tags (repeatable).
  --exclude-via-path <glob> Exclude via paths matching a glob (repeatable).
  --exclude-via-tag <tag>   Exclude via tags (repeatable).
  --hide-path <glob>        Hide matching note targets and note via nodes (repeatable).
  --hide-tag <tag>          Hide matching tag targets and via nodes (repeatable).
  --no-config-hide          Ignore configured query hide conditions.
  --no-config-via           Ignore configured query via conditions and legacy exclude fallback.
  --link-key <key>          Restrict direct backlinks and outgoing links to a frontmatter key.
  --path <glob>             Include result target paths matching any glob (repeatable).
  --where <expr>            Metadata filter (repeatable; expressions are ANDed).
  --include-head <N>        Include the first N body lines of returned notes (N > 0).
  --include-snippet <N>     Include source link lines with N lines of context (N >= 0).
  --vault <path>            Vault root directory. Default: ".".
  --format json|text        Output format. Default: text.

Relations select output sections. Via conditions select two-hop paths; hide conditions
remove visible note/tag targets and via identifiers. Hidden via nodes can still
discover targets. --via accepts note/asset vault-relative paths, phantom names,
or tag names (with optional #); it does not restrict backlinks or outgoing.
Via include/exclude paths apply to note/asset nodes, and tags to tag nodes.
Repeatable patterns combine with configured patterns; excludes take precedence.
When query.via.exclude is absent, top-level exclude supplies via exclusions.
Globs are case-sensitive, * crosses /, and [] character classes are unsupported.
Quote globs to prevent shell expansion. --path and --where filter targets only;
--link-key filters direct links only.

The JSON fields are entry, selected backlinks/outgoing/2hoplink arrays, and page.
Unselected relation fields are absent; selected empty relations are []. Page
next_offset is null at the end. A missing limit returns all remaining targets.
--include-head previews returned notes only. --include-snippet 0 includes just
the link line; outgoing snippets come from the entry, backlinks from the target,
and two-hop snippets from the target's link to each visible via node.

Examples:
  mdhop query --file Plan.md --format json
  mdhop query --file Plan.md --relations twohop --via note:topics/Design.md
  mdhop query --file Plan.md --relations backlinks --limit 20 --offset 20
  mdhop query --file Plan.md --hide-path 'archive/*' --exclude-via-tag '#private'

`

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, queryHelp)
	vault := fs.String("vault", ".", "vault root directory")
	file := fs.String("file", "", "note entry (vault-relative path)")
	tag := fs.String("tag", "", "tag entry")
	phantom := fs.String("phantom", "", "phantom entry")
	name := fs.String("name", "", "auto-detect entry")
	format := fs.String("format", "text", "output format (json or text)")
	relations := fs.String("relations", "", "comma-separated relations")
	head := fs.Int("include-head", 0, "returned note body lines")
	snippet := fs.Int("include-snippet", 0, "link context lines")
	limit := fs.Int("limit", 0, "max targets for one relation")
	offset := fs.Int("offset", 0, "targets to skip for one relation")
	linkKey := fs.String("link-key", "", "frontmatter key for direct link results")
	noConfigHide := fs.Bool("no-config-hide", false, "ignore configured query hide")
	noConfigVia := fs.Bool("no-config-via", false, "ignore configured query via")
	var via, viaPaths, viaTags, excludeViaPaths, excludeViaTags multiString
	var hidePaths, hideTags, pathPatterns, whereExprs multiString
	fs.Var(&via, "via", "typed two-hop via node")
	fs.Var(&viaPaths, "via-path", "include via path glob (repeatable)")
	fs.Var(&viaTags, "via-tag", "include via tag (repeatable)")
	fs.Var(&excludeViaPaths, "exclude-via-path", "exclude via path glob (repeatable)")
	fs.Var(&excludeViaTags, "exclude-via-tag", "exclude via tag (repeatable)")
	fs.Var(&hidePaths, "hide-path", "hide note path glob (repeatable)")
	fs.Var(&hideTags, "hide-tag", "hide tag (repeatable)")
	fs.Var(&pathPatterns, "path", "include result path glob (repeatable)")
	fs.Var(&whereExprs, "where", "frontmatter filter (repeatable)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	var selected []string
	var pageLimit, pageOffset, includeHead, includeSnippet *int
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "relations":
			selected = []string{}
			if *relations != "" {
				selected = strings.Split(*relations, ",")
				for i := range selected {
					selected[i] = strings.TrimSpace(selected[i])
				}
			}
		case "include-head":
			includeHead = head
		case "include-snippet":
			includeSnippet = snippet
		case "limit":
			pageLimit = limit
		case "offset":
			pageOffset = offset
		}
	})

	var cfg core.Config
	if !*noConfigHide || !*noConfigVia || len(whereExprs) > 0 {
		var err error
		cfg, err = core.LoadConfig(*vault)
		if err != nil {
			return err
		}
	}
	filter, err := core.NewQueryFilter(cfg, core.QueryFilterOptions{
		Hide:         core.ExcludeConfig{Paths: hidePaths, Tags: hideTags},
		ViaInclude:   core.ExcludeConfig{Paths: viaPaths, Tags: viaTags},
		ViaExclude:   core.ExcludeConfig{Paths: excludeViaPaths, Tags: excludeViaTags},
		NoConfigHide: *noConfigHide,
		NoConfigVia:  *noConfigVia,
		Via:          via,
	})
	if err != nil {
		return err
	}
	wc, err := core.ParseWhere(whereExprs, cfg.Meta)
	if err != nil {
		return err
	}

	result, err := core.Query(*vault, core.EntrySpec{
		File: *file, Tag: *tag, Phantom: *phantom, Name: *name,
	}, core.QueryOptions{
		Relations:      selected,
		IncludeHead:    includeHead,
		IncludeSnippet: includeSnippet,
		Filter:         filter,
		Limit:          pageLimit,
		Offset:         pageOffset,
		Where:          wc,
		LinkKey:        *linkKey,
		Path:           pathPatterns,
	})
	if err != nil {
		return err
	}
	if *format == "json" {
		return printQueryJSON(os.Stdout, result)
	}
	return printQueryText(os.Stdout, result)
}
