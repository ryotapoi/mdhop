package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const metaCheckHelp = `Usage: mdhop meta-check --key <name> [--key <name>...] [--kind path|wikilink] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Check whether frontmatter values resolve to real vault paths or wikilinks.

Options:
  --key <name>          Required, repeatable. Frontmatter key to inspect.
  --kind path|wikilink  Optional. Interpret values as raw paths or wikilinks. Default: path.
  --path <glob>         Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>      Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>        Optional. Vault root directory. Default: ".".
  --format json|text    Optional. Output format. Default: text.

Output fields:
  issues[]     One item per unresolved value.
  source_path  Note containing the frontmatter value.
  key          Frontmatter key.
  value        Unresolved value.
  reason       not_found, ambiguous, vault_escape, or not_wikilink.

Examples:
  mdhop meta-check --key sources --kind path --format json
  mdhop meta-check --key related --kind wikilink --format json
  mdhop meta-check --key sources --path "docs/*" --format json

`

func runMetaCheck(args []string) error {
	fs := flag.NewFlagSet("meta-check", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, metaCheckHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	kind := fs.String("kind", "path", "value interpretation (path or wikilink)")
	var keys multiString
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&keys, "key", "frontmatter key to check (repeatable, required)")
	fs.Var(&pathPatterns, "path", "restrict source notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude source notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.MetaCheck(*vault, core.MetaCheckOptions{
		Keys:    keys,
		Kind:    core.MetaValueKind(*kind),
		Path:    pathPatterns,
		Exclude: excludePaths,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printMetaCheckJSON(os.Stdout, result)
	default:
		return printMetaCheckText(os.Stdout, result)
	}
}
