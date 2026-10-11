package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const disambiguateHelp = `Usage: mdhop disambiguate --name <basename> [--target <path>] [--file <path>] [--scan] [--vault <path>] [--format json|text]

Rewrite ambiguous basename links to full paths.

Options:
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --name <basename>   Required. Basename link name to rewrite.
  --target <path>     Optional. Required when the basename has multiple candidates.
  --file <path>       Optional. Limit rewriting to one vault-relative file.
  --scan              Optional. Scan files without requiring an existing DB; useful before build.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop disambiguate --name a --target Notes/a.md --format json
  mdhop disambiguate --name a --scan --format json
  mdhop disambiguate --name a --file Notes/Design.md --target Notes/a.md

`

func runDisambiguate(args []string) error {
	fs := flag.NewFlagSet("disambiguate", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, disambiguateHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	name := fs.String("name", "", "basename to disambiguate")
	target := fs.String("target", "", "target file path (required if multiple candidates)")
	scan := fs.Bool("scan", false, "scan all files without DB")
	var files multiString
	fs.Var(&files, "file", "limit rewriting to these source files")
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
		}, locations)
	} else {
		result, err = core.Disambiguate(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		}, locations)
	}
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		return printRewrittenJSON(os.Stdout, result.Rewritten)
	default:
		return printRewrittenText(os.Stdout, result.Rewritten)
	}
}
