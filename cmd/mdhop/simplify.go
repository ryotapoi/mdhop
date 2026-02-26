package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runSimplify(args []string) error {
	fs := flag.NewFlagSet("simplify", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	dryRun := fs.Bool("dry-run", false, "show what would be simplified without making changes")
	var files multiString
	fs.Var(&files, "file", "limit simplification to these source files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.Simplify(*vault, core.SimplifyOptions{
		DryRun: *dryRun,
		Files:  files,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if err := printSimplifyJSON(os.Stdout, result); err != nil {
			return err
		}
	default:
		printSimplifyText(os.Stdout, result)
	}
	if !*dryRun && len(result.Rewritten) > 0 {
		fmt.Fprintln(os.Stderr, "hint: run 'mdhop build' to create or update the index")
	}
	return nil
}
