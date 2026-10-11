package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const statusHelp = `Usage: mdhop status [--vault <path>] [--format json|text]

List untracked, modified, and deleted files without changing the vault or index.
Modified compares the indexed and disk mtimes at whole-second precision; content
changes within the same second are not detected.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop status
  mdhop status --format json

`

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, statusHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
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

	result, err := core.Status(*vault, locations)
	if err != nil {
		return err
	}
	if *format == "json" {
		return printStatusJSON(os.Stdout, result)
	}
	return printStatusText(os.Stdout, result)
}
