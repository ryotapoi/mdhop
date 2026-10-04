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
  --file <path>       Indexed note by vault-relative path (required).
  --fields <list>     Comma-separated tags,meta. Default: both.
  --include-head <N>  Include the first N body lines; N must be positive.
  --vault <path>      Vault root directory. Default: ".".
  --format json|text  Output format. Default: text.

Unselected fields are omitted; selected empty tags/head are [] and meta is {}.
Head skips frontmatter and leading blank lines. Attributes use the index only.

`

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, inspectHelp)
	vault := fs.String("vault", ".", "vault root directory")
	file := fs.String("file", "", "indexed note path")
	fields := fs.String("fields", "", "comma-separated tags,meta")
	head := fs.Int("include-head", 0, "first N body lines")
	format := fs.String("format", "text", "output format (json or text)")
	if err := parseFlags(fs, args); err != nil {
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
	result, err := core.Inspect(*vault, *file, opts)
	if err != nil {
		return err
	}
	if *format == "json" {
		return printInspectJSON(os.Stdout, result)
	}
	return printInspectText(os.Stdout, result)
}
