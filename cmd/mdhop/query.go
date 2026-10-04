package main

import (
	"flag"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

const queryHelp = `Usage: mdhop query (--file <path>|--tag <name>|--phantom <name>|--name <name>) [options]

Return backlinks, outgoing links, and notes sharing a link target with one entry.

Entry options:
  --file <path>             Note entry by vault-relative path.
  --tag <name>              Tag entry. Leading # is optional.
  --phantom <name>          Phantom entry.
  --name <name>             Auto-detect note, phantom, or tag. Ambiguous names fail.

Options:
  --relations <list>        Comma-separated backlinks,outgoing,twohop. Default: all three.
  --limit <N>               Return at most N targets; requires one explicit relation.
  --offset <N>              Skip N targets; requires one explicit relation.
  --via <type:value>        Select one typed two-hop via node.
  --via-path <glob>         Include via paths matching a glob (repeatable).
  --via-tag <tag>           Include via tags (repeatable).
  --exclude-via-path <glob> Exclude via paths matching a glob (repeatable).
  --exclude-via-tag <tag>   Exclude via tags (repeatable).
  --hide-path <glob>        Hide matching note targets and via nodes (repeatable).
  --hide-tag <tag>          Hide matching tag targets and via nodes (repeatable).
  --no-config-hide          Ignore configured query hide conditions.
  --no-config-via           Ignore configured query via conditions.
  --link-key <key>          Restrict direct backlinks and outgoing links to a frontmatter key.
  --path <glob>             Include result paths matching any glob (repeatable).
  --where <expr>            Metadata filter (repeatable; expressions are ANDed).
  --vault <path>            Vault root directory. Default: ".".
  --format json|text        Output format. Default: text.

The JSON fields are entry, selected backlinks/outgoing/2hoplink arrays, and page.

Examples:
  mdhop query --file Plan.md --format json
  mdhop query --file Plan.md --relations twohop --via note:topics/Design.md
  mdhop query --file Plan.md --relations backlinks --limit 20 --offset 20

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
	var pageLimit, pageOffset *int
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
		Relations: selected,
		Filter:    filter,
		Limit:     pageLimit,
		Offset:    pageOffset,
		Where:     wc,
		LinkKey:   *linkKey,
		Path:      pathPatterns,
	})
	if err != nil {
		return err
	}
	if *format == "json" {
		return printQueryJSON(os.Stdout, result)
	}
	return printQueryText(os.Stdout, result)
}
