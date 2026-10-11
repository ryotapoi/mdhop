package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const deleteHelp = `Usage: mdhop delete --file <path> [--file <path>...] [--rm] [--vault <path>] [--format json|text]

Remove registered files from the index. With --rm, remove them from disk as well.

Options:
  --db <path>      Index DB path. Default: <cache>/mdhop/vaults/<vault-hash>/index.sqlite (see paths).
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --file <path>       Required, repeatable. Vault-relative file or directory. A trailing / or disk directory enables directory mode.
  --rm                Optional. Remove registered files from disk. In directory mode, also remove remaining non-Markdown files.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Without --rm, registered files must already be absent from disk.
  A directory must contain at least one registered file. With --rm, unregistered Markdown files and files in hidden directories remain.

Output fields:
  deleted    Nodes removed from the index.
  phantomed  Deleted files kept as phantom nodes because references remain.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop delete --file Notes/Obsolete.md --rm --format json
  mdhop delete --file Notes/archive/ --rm --format json
  mdhop delete --file Notes/Obsolete.md

`

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, deleteHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	rm := fs.Bool("rm", false, "remove files from disk before updating index")
	var files multiString
	fs.Var(&files, "file", "file to delete (can be specified multiple times)")
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
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}

	result, err := core.Delete(*vault, core.DeleteOptions{Files: files, RemoveFiles: *rm}, locations)
	if err != nil {
		return err
	}

	if *format == "json" {
		return printDeleteJSON(os.Stdout, result)
	}
	return printDeleteText(os.Stdout, result)
}
