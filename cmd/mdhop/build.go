package main

import (
	"flag"

	"github.com/ryotapoi/mdhop/internal/core"
)

const buildHelp = `Usage: mdhop build [--vault <path>]

Build the SQLite index for an Obsidian-style Markdown vault.

Options:
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  No stdout output on success. Creates or replaces the vault index under .mdhop/.
  Warnings, if any, are written to stderr.

Examples:
  mdhop build
  mdhop build --vault ~/Notes

`

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, buildHelp)
	vault := fs.String("vault", ".", "vault root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.Build(*vault)
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	return nil
}
