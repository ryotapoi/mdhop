package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const updateHelp = `Usage: mdhop update --file <path> [--file <path>...] [--vault <path>] [--format json|text]

Re-index registered files after editing them. If a registered file is missing on disk, it is handled like delete.

Options:
  --file <path>       Required, repeatable. Vault-relative registered file path to update.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  updated    Files re-indexed from disk.
  deleted    Registered nodes removed because no references remain.
  phantomed  Missing files kept as phantom nodes because references remain.

Examples:
  mdhop update --file Notes/Design.md --format json
  mdhop update --file Notes/A.md --file Notes/B.md --format json

`

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, updateHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	var files multiString
	fs.Var(&files, "file", "file to update (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	result, err := core.Update(*vault, core.UpdateOptions{Files: files})
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	switch *format {
	case "json":
		return printUpdateJSON(os.Stdout, result)
	default:
		printUpdateText(os.Stdout, result)
		return nil
	}
}
