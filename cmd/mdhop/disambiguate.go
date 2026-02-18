package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runDisambiguate(args []string) error {
	fs := flag.NewFlagSet("disambiguate", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	name := fs.String("name", "", "basename to disambiguate")
	target := fs.String("target", "", "target file path (required if multiple candidates)")
	scan := fs.Bool("scan", false, "scan all files without DB")
	var files multiString
	fs.Var(&files, "file", "limit rewriting to these source files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	var result *core.DisambiguateResult
	var err error
	if *scan {
		result, err = core.DisambiguateScan(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		})
	} else {
		result, err = core.Disambiguate(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		})
	}
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		return printDisambiguateJSON(os.Stdout, result)
	default:
		printDisambiguateText(os.Stdout, result)
		return nil
	}
}
