package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

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
		return fmt.Errorf("search: sample must be > 0")
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
