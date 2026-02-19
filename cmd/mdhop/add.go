package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	var files multiString
	fs.Var(&files, "file", "file to add (can be specified multiple times)")
	noAutoDisambiguate := fs.Bool("no-auto-disambiguate", false,
		"disable automatic link rewriting when basename collision occurs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	result, err := core.Add(*vault, core.AddOptions{
		Files:            files,
		AutoDisambiguate: !*noAutoDisambiguate,
	})
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		return printAddJSON(os.Stdout, result)
	default:
		printAddText(os.Stdout, result)
		return nil
	}
}
