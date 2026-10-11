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
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --dry-run           Optional. Report changes without writing files.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links skipped because no safe repair target was available.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop repair --dry-run --format json
  mdhop repair --path "docs/*" --exclude "docs/archive/*" --dry-run --format json
  mdhop repair

`

func runRepair(args []string) error {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, repairHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	dryRun := fs.Bool("dry-run", false, "show what would be repaired without making changes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict source notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude source notes matching glob (repeatable)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.Repair(*vault, core.RepairOptions{
		DryRun:  *dryRun,
		Path:    pathPatterns,
		Exclude: excludePaths,
	}, locations)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if err := printRewriteResultJSON(os.Stdout, result.Rewritten, result.Skipped); err != nil {
			return err
		}
	default:
		if err := printRewriteResultText(os.Stdout, result.Rewritten, result.Skipped); err != nil {
			return err
		}
	}
	if !*dryRun && len(result.Rewritten) > 0 {
		fmt.Fprintln(os.Stderr, buildIndexHint)
	}
	return nil
}
