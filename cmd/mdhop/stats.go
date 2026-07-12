package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

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

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, statsHelp)
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
