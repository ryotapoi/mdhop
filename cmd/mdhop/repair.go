package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runRepair(args []string) error {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	dryRun := fs.Bool("dry-run", false, "show what would be repaired without making changes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.Repair(*vault, core.RepairOptions{
		DryRun: *dryRun,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if err := printRepairJSON(os.Stdout, result); err != nil {
			return err
		}
	default:
		printRepairText(os.Stdout, result)
	}
	if !*dryRun && len(result.Rewritten) > 0 {
		fmt.Fprintln(os.Stderr, "hint: run 'mdhop build' to create or update the index")
	}
	return nil
}
