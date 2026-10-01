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
  --file <path>       Required, repeatable. Vault-relative file or directory. A trailing / or disk directory enables directory mode.
  --rm                Optional. Also delete files from disk.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Output fields:
  deleted    Nodes removed from the index.
  phantomed  Deleted files kept as phantom nodes because references remain.

Examples:
  mdhop delete --file Notes/Obsolete.md --rm --format json
  mdhop delete --file Notes/archive/ --rm --format json
  mdhop delete --file Notes/Obsolete.md

`

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, deleteHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	rm := fs.Bool("rm", false, "remove files from disk before updating index")
	var files multiString
	fs.Var(&files, "file", "file to delete (can be specified multiple times)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}

	result, err := core.Delete(*vault, core.DeleteOptions{Files: files, RemoveFiles: *rm})
	if err != nil {
		return err
	}

	if *format == "json" {
		return printDeleteJSON(os.Stdout, result)
	}
	return printDeleteText(os.Stdout, result)
}
