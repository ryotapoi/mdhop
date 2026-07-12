package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const convertHelp = `Usage: mdhop convert --to <wikilink|markdown> [--dry-run] [--file <path>...] [--vault <path>] [--format json|text]

Convert between wikilink and Markdown link syntax.

Options:
  --to <wikilink|markdown>  Required. Target link syntax.
  --dry-run                 Optional. Report changes without writing files.
  --file <path>             Optional, repeatable. Limit conversion to specific vault-relative files.
  --vault <path>            Optional. Vault root directory. Default: ".".
  --format json|text        Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were converted.

Examples:
  mdhop convert --to wikilink --dry-run --format json
  mdhop convert --to markdown --file Notes/Design.md --format json
  mdhop convert --to wikilink

`

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, convertHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	toFormat := fs.String("to", "", "target format: wikilink or markdown (required)")
	dryRun := fs.Bool("dry-run", false, "show what would be converted without making changes")
	var files multiString
	fs.Var(&files, "file", "file to convert (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *toFormat != "wikilink" && *toFormat != "markdown" {
		return fmt.Errorf("--to is required and must be 'wikilink' or 'markdown'")
	}

	result, err := core.Convert(*vault, core.ConvertOptions{
		ToFormat: *toFormat,
		DryRun:   *dryRun,
		Files:    files,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if err := printConvertJSON(os.Stdout, result); err != nil {
			return err
		}
	default:
		printRewrittenText(os.Stdout, result.Rewritten)
	}
	if !*dryRun && len(result.Rewritten) > 0 {
		fmt.Fprintln(os.Stderr, "hint: run 'mdhop build' to create or update the index")
	}
	return nil
}
