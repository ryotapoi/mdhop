package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runDisambiguate(args []string) error {
	fs := flag.NewFlagSet("disambiguate", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	name := fs.String("name", "", "basename to disambiguate")
	target := fs.String("target", "", "target file path (required if multiple candidates)")
	scan := fs.Bool("scan", false, "scan all files without DB")
	var files multiString
	fs.Var(&files, "file", "limit rewriting to these source files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if *scan {
		_, err := core.DisambiguateScan(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		})
		return err
	}
	_, err := core.Disambiguate(*vault, core.DisambiguateOptions{
		Name:   *name,
		Target: *target,
		Files:  files,
	})
	return err
}
