package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const diagnoseHelp = `Usage: mdhop diagnose [--path <glob>...] [--exclude <glob>...] [--fields <list>] [--vault <path>] [--format json|text]

Report basename conflicts, asset basename conflicts, phantom references, and optional broken anchors.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --fields <list>     Optional. basename_conflicts,asset_basename_conflicts,phantoms,anchors.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  basename_conflicts        Note basename collision groups.
  asset_basename_conflicts  Asset filename collision groups.
  phantoms                  Phantom names referenced from notes.
  anchors                   Broken heading anchors; opt-in and not included when --fields is omitted.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop diagnose --format json
  mdhop diagnose --fields anchors --format json
  mdhop diagnose --path "projects/*" --format json

`

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, diagnoseHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
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

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validDiagnoseFieldsCLI, "diagnose"); err != nil {
		return err
	}

	result, err := core.Diagnose(*vault, core.DiagnoseOptions{
		Fields:  fieldList,
		Path:    pathPatterns,
		Exclude: excludePaths,
	}, locations)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printDiagnoseJSON(os.Stdout, result, fieldList)
	default:
		return printDiagnoseText(os.Stdout, result, fieldList)
	}
}
