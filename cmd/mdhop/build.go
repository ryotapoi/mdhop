package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

const buildHelp = `Usage: mdhop build [--vault <path>] [--db <path>] [--config <path>] [--full]

Build the SQLite index, automatically detecting added, changed, and deleted files.
Unchanged notes are not read or parsed. Use --full to read and regenerate everything.
Markdown destinations are source-relative; a leading / starts at the vault root.
Run build to replace indexes created with older link interpretation rules.

Options:
  --full          Read and parse every note; ignore the existing index.
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  No stdout output on success. Creates or updates the selected index DB after completing the build.
  Warnings, if any, are written to stderr.

Detection compares each file's nanosecond mtime and size with saved values. Moves
are deletion plus addition. Same mtime and size edits/restores are not detected;
use --full, including on filesystems with coarse timestamps. status retains its
separate whole-second mtime rule.
A missing/old/incomplete index, changed canonical root or index-effective
build/meta settings, or a completed index mutation causes full regeneration.
Query/search-only settings do not. Detected input changes during build fail and
preserve the old index. This is not a filesystem snapshot or lock; edits after
the final check and changes restored to the same signature may go undetected.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.
An explicit config must exist; unreadable or invalid configs are errors.
Legacy mdhop.yaml and .mdhop/ are not used or migrated; use mdhop migrate.
Default builds record the canonical vault root in origin.json beside the DB.
An explicit --db never creates or updates this record. Use cache-list to compare caches.
Rebuild after cache removal or
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
	full := fs.Bool("full", false, "regenerate the full index")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	var emptyDB bool
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "db" && *db == "" {
			emptyDB = true
		}
	})
	if emptyDB {
		return fmt.Errorf("--db requires a nonempty path")
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}
	var result *core.BuildResult
	var err error
	if *full {
		result, err = core.BuildFull(*vault, locations)
	} else {
		result, err = core.Build(*vault, locations)
	}
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	return nil
}
