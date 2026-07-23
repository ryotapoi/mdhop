package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const repairHelp = `Usage: mdhop repair [--dry-run] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Rewrite broken path links and vault-escape links to basename links when safe.

Options:
  --dry-run           Optional. Report changes without writing files.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links skipped because no safe repair target was available.

Examples:
  mdhop repair --dry-run --format json
  mdhop repair --path "docs/*" --exclude "docs/archive/*" --dry-run --format json
  mdhop repair

`

func runRepair(args []string) error {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, repairHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	dryRun := fs.Bool("dry-run", false, "show what would be repaired without making changes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict source notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude source notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.Repair(*vault, core.RepairOptions{
		DryRun:  *dryRun,
		Path:    pathPatterns,
		Exclude: excludePaths,
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
		fmt.Fprintln(os.Stderr, buildIndexHint)
	}
	return nil
}
