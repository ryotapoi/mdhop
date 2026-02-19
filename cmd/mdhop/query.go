package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	file := fs.String("file", "", "note entry (vault-relative path)")
	tag := fs.String("tag", "", "tag entry")
	phantom := fs.String("phantom", "", "phantom entry")
	name := fs.String("name", "", "auto-detect entry")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	includeHead := fs.Int("include-head", 0, "include first N lines of note")
	includeSnippet := fs.Int("include-snippet", 0, "include N context lines around links")
	maxBacklinks := fs.Int("max-backlinks", 100, "max backlinks")
	maxTwoHop := fs.Int("max-twohop", 100, "max twohop entries")
	maxViaPerTarget := fs.Int("max-via-per-target", 10, "max via entries per twohop target")
	var excludePaths multiString
	var excludeTags multiString
	fs.Var(&excludePaths, "exclude", "exclude paths matching glob (repeatable)")
	fs.Var(&excludeTags, "exclude-tag", "exclude tag (repeatable)")
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

	var cfgExclude core.ExcludeConfig
	if !*noExclude {
		cfg, err := core.LoadConfig(*vault)
		if err != nil {
			return err
		}
		cfgExclude = cfg.Exclude
	}
	ef, err := core.NewExcludeFilter(cfgExclude, excludePaths, excludeTags)
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
