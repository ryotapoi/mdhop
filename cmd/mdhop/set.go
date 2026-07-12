package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ryotapoi/mdhop/internal/core"
)

const setHelp = `Usage: mdhop set --file <path> --key <name> (--value <value>|--date <expr>) [--vault <path>] [--format json|text]

Set one frontmatter document key and update the index.

Options:
  --file <path>       Required. Vault-relative Markdown file to edit.
  --key <name>        Required. Frontmatter key to set.
  --value <value>     YAML scalar value to write exactly as provided.
  --date <expr>       Relative date expression to expand and write as YYYY-MM-DD. Mutually exclusive with --value.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Exactly one of --value or --date is required.
  --date supports today, today-90d, today+1d, today-2w, today+3m, and today+1y.
  Files without frontmatter get a new frontmatter block at the start of the file.
  Missing keys are inserted before the closing ---.
  List values, multi-line values, and duplicate keys are rejected.

Output fields:
  file     Edited file path.
  key      Updated key.
  value    Written value.
  created  Whether the key was newly inserted.

Examples:
  mdhop set --file Notes/Design.md --key reviewed --value 2026-07-04 --format json
  mdhop set --file Notes/Design.md --key reviewed --date today-90d
  mdhop set --file Notes/Design.md --key status --value active

`

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, setHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	file := fs.String("file", "", "file to update")
	key := fs.String("key", "", "frontmatter key to set")
	value := fs.String("value", "", "frontmatter value to write")
	date := fs.String("date", "", "relative date expression to write as YYYY-MM-DD")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}
	if *key == "" {
		return fmt.Errorf("--key is required")
	}
	valueSpecified := false
	dateSpecified := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "value":
			valueSpecified = true
		case "date":
			dateSpecified = true
		}
	})
	if valueSpecified == dateSpecified {
		return fmt.Errorf("exactly one of --value or --date is required")
	}
	writeValue := *value
	if valueSpecified {
		if writeValue == "" {
			return fmt.Errorf("--value cannot be empty")
		}
	} else {
		if *date == "" {
			return fmt.Errorf("--date cannot be empty")
		}
		expanded, ok := core.ExpandRelativeDate(*date, time.Now())
		if !ok {
			return fmt.Errorf("--date must use relative date syntax such as today, today-90d, or today+1y")
		}
		writeValue = expanded
	}
	result, err := core.Set(*vault, core.SetOptions{
		File:  *file,
		Key:   *key,
		Value: writeValue,
	})
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	switch *format {
	case "json":
		return printSetJSON(os.Stdout, result)
	default:
		printSetText(os.Stdout, result)
		return nil
	}
}
