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
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --dry-run           Optional. Report changes without writing files.
  --file <path>       Optional, repeatable. Limit rewriting to specific vault-relative files.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.
  skipped    Links or files skipped because they could not be simplified safely.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop simplify --dry-run --format json
  mdhop simplify --file Notes/Design.md --format json
  mdhop simplify

`

func runSimplify(args []string) error {
	fs := flag.NewFlagSet("simplify", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, simplifyHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	dryRun := fs.Bool("dry-run", false, "show what would be simplified without making changes")
	var files multiString
	fs.Var(&files, "file", "limit simplification to these source files")
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

	result, err := core.Simplify(*vault, core.SimplifyOptions{
		DryRun: *dryRun,
		Files:  files,
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
