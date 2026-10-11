package main

import (
	"flag"

	"github.com/ryotapoi/mdhop/internal/core"
)

const buildHelp = `Usage: mdhop build [--vault <path>] [--db <path>] [--config <path>]

Build the SQLite index for an Obsidian-style Markdown vault.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  No stdout output on success. Creates or replaces the selected index DB after completing the build.
  Warnings, if any, are written to stderr.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.
An explicit config must exist; unreadable or invalid configs are errors.
Legacy mdhop.yaml and .mdhop/ are not used or migrated; use mdhop migrate.
Use mdhop paths to inspect effective locations. Rebuild after cache removal or
moving the vault; use --db for separate indexes of the same vault.

The temporary DB is built beside the selected DB and published only on success;
failure preserves the old index. An external DB requires no writes to the vault.
On local Ubuntu/macOS filesystems, readers already connected retain the old index,
and readers connecting after replacement see the new completed index.
Serialize build with other index writes, including update and migrate.
Separate CLI calls and edited note contents are not guaranteed the same snapshot.

Examples:
  mdhop build
  mdhop build --vault ~/Notes
  mdhop build --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml

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
