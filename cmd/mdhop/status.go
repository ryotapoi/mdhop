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
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Examples:
  mdhop status
  mdhop status --format json

`

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, statusHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.Status(*vault)
	if err != nil {
		return err
	}
	if *format == "json" {
		return printStatusJSON(os.Stdout, result)
	}
	return printStatusText(os.Stdout, result)
}
