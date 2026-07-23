package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const searchHelp = `Usage: mdhop search [--where <expr>...] [--path <glob>...] [options]

Search existing notes without an entry node.

Options:
  --where <expr>       Optional, repeatable. Metadata filter using the same syntax as query. Multiple --where flags are ANDed; use " || " inside one expression for OR.
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
  mdhop search --where "status=active || status=review" --format json
  mdhop search --where "updated<today-90d" --count --format json
  mdhop search --where "status=active" --sample 10 --format json

`

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, searchHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output (meta, meta.<key>, lines, outgoing_count, incoming_count)")
	sortKey := fs.String("sort", "", "sort by meta key or computed field (lines, outgoing_count, incoming_count; prefix with - for desc)")
	includeHead := fs.Int("include-head", 0, "include first N lines of each note")
	limit := fs.Int("limit", 0, "max results (0 = unlimited)")
	offset := fs.Int("offset", 0, "skip first N results")
	sample := fs.Int("sample", 0, "randomly return N results from filtered candidates")
	count := fs.Bool("count", false, "return only the filtered result count")
	noTags := fs.Bool("no-tags", false, "only notes with no tag edges")
	noOutgoing := fs.Bool("no-outgoing", false, "only notes with no outgoing edges")
	noIncoming := fs.Bool("no-incoming", false, "only notes with no incoming edges")
	var whereExprs multiString
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&whereExprs, "where", "frontmatter filter (repeatable)")
	fs.Var(&pathPatterns, "path", "include paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude paths matching glob (repeatable)")
	noExclude := fs.Bool("no-exclude", false, "disable config file exclusions")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}
	sampleSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "sample" {
			sampleSet = true
		}
	})
	if sampleSet && *sample <= 0 {
		return fmt.Errorf("sample must be > 0")
	}

	fieldList := parseFields(*fields)
	if err := validateSearchFields(fieldList); err != nil {
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
	ef, err := core.NewExcludeFilter(cfgExclude, excludePaths, nil)
	if err != nil {
		return err
	}

	wc, err := core.ParseWhere(whereExprs, cfg.Meta)
	if err != nil {
		return err
	}

	opts := core.SearchOptions{
		Fields:      fieldList,
		Where:       wc,
		Exclude:     ef,
		Path:        pathPatterns,
		Sort:        *sortKey,
		Limit:       *limit,
		Offset:      *offset,
		Sample:      *sample,
		Count:       *count,
		IncludeHead: *includeHead,
		NoTags:      *noTags,
		NoOutgoing:  *noOutgoing,
		NoIncoming:  *noIncoming,
	}

	result, err := core.Search(*vault, opts)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if *count {
			return printSearchCountJSON(os.Stdout, result.Total)
		}
		return printSearchJSON(os.Stdout, result, fieldList)
	default:
		if *count {
			return printSearchCountText(os.Stdout, result.Total)
		}
		return printSearchText(os.Stdout, result, fieldList)
	}
}
