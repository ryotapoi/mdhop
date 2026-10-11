package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const convertHelp = `Usage: mdhop convert --to <wikilink|markdown> [--dry-run] [--file <path>...] [--vault <path>] [--format json|text]

Convert between wikilink and Markdown link syntax.
Reference links and their definitions are left unchanged. Markdown self-links
with balanced parentheses in their fragments can be converted. Destinations are
decoded before conversion; links that cannot preserve their meaning as wikilinks
are left unchanged. Table wikilink aliases keep their escaped pipe separator.

Options:
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --to <wikilink|markdown>  Required. Target link syntax.
  --dry-run                 Optional. Report changes without writing files.
  --file <path>             Optional, repeatable. Limit conversion to specific vault-relative files.
  --vault <path>            Optional. Vault root directory. Default: ".".
  --format json|text        Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were converted.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop convert --to wikilink --dry-run --format json
  mdhop convert --to markdown --file Notes/Design.md --format json
  mdhop convert --to wikilink

`

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, convertHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	format := fs.String("format", "text", "output format (json or text)")
	toFormat := fs.String("to", "", "target format: wikilink or markdown (required)")
	dryRun := fs.Bool("dry-run", false, "show what would be converted without making changes")
	var files multiString
	fs.Var(&files, "file", "file to convert (can be specified multiple times)")
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
	if *toFormat != "wikilink" && *toFormat != "markdown" {
		return fmt.Errorf("--to is required and must be 'wikilink' or 'markdown'")
	}

	result, err := core.Convert(*vault, core.ConvertOptions{
		ToFormat: *toFormat,
		DryRun:   *dryRun,
		Files:    files,
	}, locations)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		if err := printRewrittenJSON(os.Stdout, result.Rewritten); err != nil {
			return err
		}
	default:
		if err := printRewrittenText(os.Stdout, result.Rewritten); err != nil {
			return err
		}
	}
	if !*dryRun && len(result.Rewritten) > 0 {
		fmt.Fprintln(os.Stderr, buildIndexHint)
	}
	return nil
}
