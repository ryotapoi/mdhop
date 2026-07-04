package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runReachable(args []string) error {
	fs := flag.NewFlagSet("reachable", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, reachableHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	from := fs.String("from", "", "entry note path (vault-relative)")
	route := fs.Bool("route", false, "include shortest routes for reachable notes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict target notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude target notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validReachableFieldsCLI, "reachable"); err != nil {
		return err
	}

	result, err := core.Reachable(*vault, core.ReachableOptions{
		From:    *from,
		Path:    pathPatterns,
		Exclude: excludePaths,
		Route:   *route,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printReachableJSON(os.Stdout, result, fieldList)
	default:
		return printReachableText(os.Stdout, result, fieldList)
	}
}
