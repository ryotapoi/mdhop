package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

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
  --where <expr>              Optional, repeatable. Metadata filter using =,!=,~,>,<,>=,<=, EXISTS/NOT EXISTS, coalesce(...), &&, ||, and today±N d/w/m/y dates. Multiple --where flags are ANDed; use " || " inside one expression for OR.
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
  mdhop query --file Notes/Design.md --where "status=active || status=review" --format json

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
	fields := fs.String("fields", "", "comma-separated fields to output")
	includeHead := fs.Int("include-head", 0, "include first N lines of note")
	includeSnippet := fs.Int("include-snippet", 0, "include N context lines around links")
	maxBacklinks := fs.Int("max-backlinks", core.DefaultMaxBacklinks, "max backlinks")
	maxTwoHop := fs.Int("max-twohop", core.DefaultMaxTwoHop, "max twohop entries")
	maxViaPerTarget := fs.Int("max-via-per-target", core.DefaultMaxViaPerTarget, "max via entries per twohop target")
	var pathPatterns multiString
	var excludePaths multiString
	var excludeTags multiString
	var whereExprs multiString
	fs.Var(&pathPatterns, "path", "include result paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude paths matching glob (repeatable)")
	fs.Var(&excludeTags, "exclude-tag", "exclude tag (repeatable)")
	fs.Var(&whereExprs, "where", "frontmatter filter (repeatable)")
	noExclude := fs.Bool("no-exclude", false, "disable config file exclusions")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validQueryFieldsCLI, "query"); err != nil {
		return err
	}

	var cfg core.Config
	if !*noExclude || len(whereExprs) > 0 {
		var err error
		cfg, err = core.LoadConfig(*vault)
		if err != nil {
			return err
		}
	}
	var cfgExclude core.ExcludeConfig
	if !*noExclude {
		cfgExclude = cfg.Exclude
	}
	ef, err := core.NewExcludeFilter(cfgExclude, excludePaths, excludeTags)
	if err != nil {
		return err
	}

	wc, err := core.ParseWhere(whereExprs, cfg.Meta)
	if err != nil {
		return err
	}

	entry := core.EntrySpec{
		File:    *file,
		Tag:     *tag,
		Phantom: *phantom,
		Name:    *name,
	}

	opts := core.QueryOptions{
		Fields:          fieldList,
		IncludeHead:     *includeHead,
		IncludeSnippet:  *includeSnippet,
		MaxBacklinks:    *maxBacklinks,
		MaxTwoHop:       *maxTwoHop,
		MaxViaPerTarget: *maxViaPerTarget,
		Exclude:         ef,
		Where:           wc,
		Path:            pathPatterns,
	}

	result, err := core.Query(*vault, entry, opts)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printQueryJSON(os.Stdout, result)
	default:
		return printQueryText(os.Stdout, result)
	}
}
