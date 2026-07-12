package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const disambiguateHelp = `Usage: mdhop disambiguate --name <basename> [--target <path>] [--file <path>] [--scan] [--vault <path>] [--format json|text]

Rewrite ambiguous basename links to full paths.

Options:
  --name <basename>   Required. Basename link name to rewrite.
  --target <path>     Optional. Required when the basename has multiple candidates.
  --file <path>       Optional. Limit rewriting to one vault-relative file.
  --scan              Optional. Scan files without requiring an existing DB; useful before build.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  rewritten  Files whose links were rewritten.

Examples:
  mdhop disambiguate --name a --target Notes/a.md --format json
  mdhop disambiguate --name a --scan --format json
  mdhop disambiguate --name a --file Notes/Design.md --target Notes/a.md

`

func runDisambiguate(args []string) error {
	fs := flag.NewFlagSet("disambiguate", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, disambiguateHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	name := fs.String("name", "", "basename to disambiguate")
	target := fs.String("target", "", "target file path (required if multiple candidates)")
	scan := fs.Bool("scan", false, "scan all files without DB")
	var files multiString
	fs.Var(&files, "file", "limit rewriting to these source files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	var result *core.DisambiguateResult
	var err error
	if *scan {
		result, err = core.DisambiguateScan(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		})
	} else {
		result, err = core.Disambiguate(*vault, core.DisambiguateOptions{
			Name:   *name,
			Target: *target,
			Files:  files,
		})
	}
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		return printDisambiguateJSON(os.Stdout, result)
	default:
		printDisambiguateText(os.Stdout, result)
		return nil
	}
}
