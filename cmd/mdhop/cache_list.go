package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/ryotapoi/mdhop/internal/core"
)

const cacheListHelp = `Usage: mdhop cache-list [--format json|text]

List existing default cache folders without specifying a vault.

Options:
  --format json|text  Output format. Default: text.

Reads only the current <cache>/mdhop/vaults/ directory. <cache> is absolute
XDG_CACHE_HOME, or ~/.cache if unset, empty, or relative. No vault, DB, config,
or positional arguments are accepted. Only 64-character lowercase hex real
directories are listed (not symlink folders), in ascending cache_path order.

Size is the sum of filesystem allocated blocks times 512 bytes for the folder
and all descendant file, directory, and symlink entries. Symlink targets are not
followed; hardlinks count per entry. Includes DB, SQLite auxiliary files, temporary
files, and origin.json. Measured at listing time, with no snapshot guarantee;
shared extents or compression can differ from space reclaimed on deletion.

Text shows the absolute cache path, then size in IEC units and original vault.
An absent origin record displays unknown (original vault is unknown).
JSON is an array of {cache_path, size_bytes, vault}; bytes are unrounded,
and an absent origin is null. Empty output is empty text or []. Invalid or
unreadable origin records are errors; recorded vaults need not still exist.

Listing creates or updates no directory, DB, or origin record. External DBs and
past cache roots are not tracked. Cache folders are regenerable: remove a whole
folder only while mdhop is not using it, then run build when needed.

Examples:
  mdhop cache-list
  mdhop cache-list --format json

`

func runCacheList(args []string) error {
	fs := flag.NewFlagSet("cache-list", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, cacheListHelp)
	format := fs.String("format", "text", "output format (json or text)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	entries, err := core.ListCaches()
	if err != nil {
		return err
	}
	if *format == "json" {
		return encodeJSON(os.Stdout, entries)
	}
	return printCacheListText(os.Stdout, entries)
}

func formatCacheSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	value := float64(size) / 1024
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[unit]
}

func printCacheListText(w io.Writer, entries []core.CacheEntry) error {
	output := &textErrorWriter{w: w}
	for i, entry := range entries {
		if i > 0 {
			fmt.Fprintln(output)
		}
		vault := "unknown"
		if entry.Vault != nil {
			vault = *entry.Vault
		}
		fmt.Fprintf(output, "%s\nsize: %s  original: %s\n", entry.CachePath, formatCacheSize(entry.SizeBytes), vault)
	}
	return output.err
}
