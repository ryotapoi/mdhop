package main

import (
	"flag"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

const inspectHelp = `Usage: mdhop inspect --file <path> [options]

Return indexed attributes and an optional body preview for one note.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --file <path>       Indexed note by vault-relative path (required).
  --fields <list>     Comma-separated tags,meta. Default: both.
  --include-head <N>  Include the first N body lines; N must be positive.
  --vault <path>      Vault root directory. Default: ".".
  --format json|text  Output format. Default: text.

Unselected fields are omitted; selected empty tags/head are [] and meta is {}.
--include-head is independent of --fields and reads the note body. Head skips
frontmatter and leading blank lines. Tags and meta use the index only; tags
are leaf tags, while query outgoing may include indexed parent tags.

Example:
  mdhop inspect --file Plan.md --fields tags --include-head 5 --format json

`

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, inspectHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	file := fs.String("file", "", "indexed note path")
	fields := fs.String("fields", "", "comma-separated tags,meta")
	head := fs.Int("include-head", 0, "first N body lines")
	format := fs.String("format", "text", "output format (json or text)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	opts := core.InspectOptions{}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "fields":
			opts.Fields = strings.Split(*fields, ",")
			for i := range opts.Fields {
				opts.Fields[i] = strings.TrimSpace(opts.Fields[i])
			}
		case "include-head":
			opts.IncludeHead = head
		}
	})
	result, err := core.Inspect(*vault, *file, opts, locations)
	if err != nil {
		return err
	}
	if *format == "json" {
		return printInspectJSON(os.Stdout, result)
	}
	return printInspectText(os.Stdout, result)
}
