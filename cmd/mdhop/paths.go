package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const pathsHelp = `Usage: mdhop paths [--vault <path>] [--db <path>] [--config <path>] [--format json|text]

Show effective absolute locations without reading or creating configuration or DB files.

Options:
  --vault <path>      Vault root directory. Default: "."; symlinks are resolved.
  --db <path>         Index DB. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite.
  --config <path>     Configuration file. Default: <vault>/mdhop.toml.
  --format json|text  Output format. Default: text.

Cache uses absolute XDG_CACHE_HOME, otherwise ~/.cache.
The vault hash is SHA-256 of the absolute, symlink-resolved vault root.
Relative location paths use the current directory. Missing config and DB files are allowed.
Use --db to keep separate indexes for different configurations of the same vault.

`

func runPaths(args []string) error {
	fs := flag.NewFlagSet("paths", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, pathsHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path")
	config := fs.String("config", "", "configuration file path")
	format := fs.String("format", "text", "output format (json or text)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	paths, err := core.Paths(*vault, core.Locations{DBPath: *db, ConfigPath: *config})
	if err != nil {
		return err
	}
	if *format == "json" {
		return json.NewEncoder(os.Stdout).Encode(paths)
	}
	_, err = fmt.Fprintf(os.Stdout, "vault: %s\nconfig: %s\ndb: %s\n", paths.Vault, paths.Config, paths.DB)
	return err
}
