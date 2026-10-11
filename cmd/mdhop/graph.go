package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const graphHelp = `Usage: mdhop graph [--path <glob>...] [--exclude <glob>...] [--include-phantoms] [--vault <path>] [--format json|dot]

Export an induced link graph for existing notes and assets.

Options:
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --path <glob>       Optional, repeatable. Include node paths matching any glob.
  --exclude <glob>    Optional, repeatable. Exclude node paths matching the glob.
  --include-phantoms  Optional. Include phantom nodes referenced from included notes.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|dot   Optional. Output format. Default: json. This command has no text format and no --fields.

Output:
  json  Object with nodes[] and edges[] for machine processing.
  dot   Graphviz digraph for visualization.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop graph --path "docs/*" --format json
  mdhop graph --format dot --include-phantoms

`

func runGraph(args []string) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, graphHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "json", "output format (json or dot)")
	includePhantoms := fs.Bool("include-phantoms", false, "include phantom nodes referenced from in-set notes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict nodes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude nodes matching glob (repeatable)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}

	// graph has no text format; validate before opening the DB.
	if *format != "json" && *format != "dot" {
		return fmt.Errorf("invalid format: %s (graph supports json or dot)", *format)
	}

	result, err := core.Graph(*vault, core.GraphOptions{
		Path:            pathPatterns,
		Exclude:         excludePaths,
		IncludePhantoms: *includePhantoms,
	}, locations)
	if err != nil {
		return err
	}

	switch *format {
	case "dot":
		return printGraphDot(os.Stdout, result)
	default:
		return printGraphJSON(os.Stdout, result)
	}
}
