package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	var files multiString
	fs.Var(&files, "file", "file to add (can be specified multiple times)")
	autoDisambiguate := fs.Bool("auto-disambiguate", false,
		"rewrite existing links when basename collision occurs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	_, err := core.Add(*vault, core.AddOptions{
		Files:            files,
		AutoDisambiguate: *autoDisambiguate,
	})
	return err
}
