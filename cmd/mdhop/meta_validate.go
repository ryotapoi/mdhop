package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const metaValidateHelp = `Usage: mdhop meta-validate [--require <key>...] [--path <glob>...] [--exclude <glob>...] [--vault <path>] [--format json|text]

Validate frontmatter against required keys and mdhop.yaml meta type declarations.

Options:
  --require <key>     Optional, repeatable. Require a non-empty value for this key; overrides mdhop.yaml meta.profiles for this run only.
  --path <glob>       Optional, repeatable. Include source notes whose paths match any glob.
  --exclude <glob>    Optional, repeatable. Exclude source notes whose paths match the glob.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  Fails if there is no --require, no mdhop.yaml meta.profiles, and no non-string meta.types declaration.

Output fields:
  violations[]  One item per schema violation.
  source_path   Note containing the violation.
  key           Frontmatter key.
  value         Invalid value when applicable.
  reason        missing, type, or enum.

Examples:
  mdhop meta-validate --require type --require status --format json
  mdhop meta-validate --format json

`

func runMetaValidate(args []string) error {
	fs := flag.NewFlagSet("meta-validate", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, metaValidateHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	var require multiString
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&require, "require", "frontmatter key that must hold a non-empty value; overrides mdhop.yaml meta.profiles for this run only, not persisted (repeatable)")
	fs.Var(&pathPatterns, "path", "restrict source notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude source notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	result, err := core.MetaValidate(*vault, core.MetaValidateOptions{
		Require: require,
		Path:    pathPatterns,
		Exclude: excludePaths,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printMetaValidateJSON(os.Stdout, result)
	default:
		return printMetaValidateText(os.Stdout, result)
	}
}
