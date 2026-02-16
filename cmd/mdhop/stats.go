package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validStatsFieldsCLI, "stats"); err != nil {
		return err
	}

	result, err := core.Stats(*vault, core.StatsOptions{Fields: fieldList})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printStatsJSON(os.Stdout, result, fieldList)
	default:
		return printStatsText(os.Stdout, result, fieldList)
	}
}
