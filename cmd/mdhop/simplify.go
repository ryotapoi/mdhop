package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const simplifyHelp = `Usage: mdhop simplify [--dry-run] [--file <path>...] [--vault <path>] [--format json|text]

Shorten path links to basename links when the shortened form remains unambiguous.

Options:
  --dry-run           Optional. Report changes without writing files.
  --file <path>       Optional, repeatable. Limit rewriting to specific vault-relative files.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links or files skipped because they could not be simplified safely.

Examples:
  mdhop simplify --dry-run --format json
  mdhop simplify --file Notes/Design.md --format json
  mdhop simplify

`

func runSimplify(args []string) error {
	fs := flag.NewFlagSet("simplify", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, simplifyHelp)
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
		fmt.Fprintln(os.Stderr, buildIndexHint)
	}
	return nil
}
