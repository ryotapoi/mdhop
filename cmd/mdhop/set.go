package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ryotapoi/mdhop/internal/core"
)

const setHelp = `Usage: mdhop set --file <path> --key <name> (--value <value>|--date <expr>|--list <json-array>) [--vault <path>] [--format json|text]

Set one frontmatter document key and update the index.

Options:
  --file <path>       Required. Vault-relative Markdown file to edit.
  --key <name>        Required. Frontmatter key to set.
  --value <value>     YAML scalar value to write exactly as provided.
  --date <expr>       Relative date expression to expand and write as YYYY-MM-DD. Mutually exclusive with --value.
  --list <json-array> JSON string array to write as a YAML sequence. Mutually exclusive with --value and --date.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Exactly one of --value, --date, or --list is required.
  --date supports today, today-90d, today+1d, today-2w, today+3m, and today+1y.
  Files without frontmatter get a new frontmatter block at the start of the file.
  Missing keys are inserted before the closing ---.
  --list replaces the whole value, including an existing scalar or sequence.
  It keeps order, duplicates, and empty strings. [] writes an empty sequence.
  Scalar writes reject existing list values. Duplicate keys are rejected.

Output fields:
  file     Edited file path.
  key      Updated key.
  value    JSON string for --value/--date; JSON string array for --list (including []). Text output prints lists as compact JSON arrays.
  created  Whether the key was newly inserted.

Examples:
  mdhop set --file Notes/Design.md --key reviewed --value 2026-07-04 --format json
  mdhop set --file Notes/Design.md --key reviewed --date today-90d
  mdhop set --file Notes/Design.md --key status --value active
  mdhop set --file Notes/Design.md --key aliases --list '["design","proposal"]'

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
	list := fs.String("list", "", "JSON string array to write as a YAML sequence")
	if err := parseFlags(fs, args); err != nil {
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
	listSpecified := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "value":
			valueSpecified = true
		case "date":
			dateSpecified = true
		case "list":
			listSpecified = true
		}
	})
	if (boolToInt(valueSpecified) + boolToInt(dateSpecified) + boolToInt(listSpecified)) != 1 {
		return fmt.Errorf("exactly one of --value, --date, or --list is required")
	}
	writeValue := *value
	var writeList []string
	if valueSpecified {
		if writeValue == "" {
			return fmt.Errorf("--value cannot be empty")
		}
	} else if dateSpecified {
		if *date == "" {
			return fmt.Errorf("--date cannot be empty")
		}
		expanded, ok := core.ExpandRelativeDate(*date, time.Now())
		if !ok {
			return fmt.Errorf("--date must use relative date syntax such as today, today-90d, or today+1y")
		}
		writeValue = expanded
	} else {
		var values []any
		if err := json.Unmarshal([]byte(*list), &values); err != nil || values == nil {
			return fmt.Errorf("--list must be a JSON string array")
		}
		writeList = make([]string, len(values))
		for i, item := range values {
			stringValue, ok := item.(string)
			if !ok {
				return fmt.Errorf("--list must be a JSON string array")
			}
			writeList[i] = stringValue
		}
	}
	result, err := core.Set(*vault, core.SetOptions{
		File:  *file,
		Key:   *key,
		Value: writeValue,
		List:  writeList,
	})
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	switch *format {
	case "json":
		return printSetJSON(os.Stdout, result)
	default:
		return printSetText(os.Stdout, result)
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
