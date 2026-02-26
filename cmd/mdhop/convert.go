package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
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
