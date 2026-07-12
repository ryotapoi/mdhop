package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const addHelp = `Usage: mdhop add --file <path> [--file <path>...] [--no-auto-disambiguate] [--vault <path>] [--format json|text]

Add newly created files to the index. Use this after writing the files.

Options:
  --file <path>              Required, repeatable. Vault-relative file path to add.
  --no-auto-disambiguate     Optional. Disable automatic rewriting when a new basename collision would otherwise be made safe.
  --vault <path>             Optional. Vault root directory. Default: ".".
  --format json|text         Optional. Output format. Default: text.

Behavior notes:
  Registered files passed to --file fail.
  Files being added fail when they contain ambiguous basename links.
  When basename collisions occur, existing basename links are automatically rewritten to full paths where their meaning can be preserved; --no-auto-disambiguate disables this.
  Existing basename links to phantom nodes fail even with auto-disambiguation when the added files contain multiple files with that basename, because there is no safe rewrite target.
  When meta.link_keys is configured, frontmatter raw path values cannot be rewritten; add fails before changing anything if existing raw path values would resolve differently.

Output fields:
  added      Files added as real nodes.
  promoted   Phantom nodes promoted to real files.
  rewritten  Files whose links were rewritten during automatic disambiguation.

Examples:
  mdhop add --file Notes/NewNote.md --format json
  mdhop add --file Notes/A.md --file Notes/B.md --format json
  mdhop add --file Notes/NewNote.md --no-auto-disambiguate

`

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, addHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	var files multiString
	fs.Var(&files, "file", "file to add (can be specified multiple times)")
	noAutoDisambiguate := fs.Bool("no-auto-disambiguate", false,
		"disable automatic link rewriting when basename collision occurs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	result, err := core.Add(*vault, core.AddOptions{
		Files:            files,
		AutoDisambiguate: !*noAutoDisambiguate,
	})
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	switch *format {
	case "json":
		return printAddJSON(os.Stdout, result)
	default:
		printAddText(os.Stdout, result)
		return nil
	}
}
