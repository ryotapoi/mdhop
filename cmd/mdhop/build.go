package main

import (
	"flag"

	"github.com/ryotapoi/mdhop/internal/core"
)

const buildHelp = `Usage: mdhop build [--vault <path>]

Build the SQLite index for an Obsidian-style Markdown vault.

Options:
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  No stdout output on success. Creates or replaces the selected index DB after completing the build.
  Warnings, if any, are written to stderr.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop build
  mdhop build --vault ~/Notes

`

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, buildHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}
	result, err := core.Build(*vault, locations)
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	return nil
}
