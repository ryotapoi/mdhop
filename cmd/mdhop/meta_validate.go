package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runMetaValidate(args []string) error {
	fs := flag.NewFlagSet("meta-validate", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	var require multiString
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&require, "require", "frontmatter key that must hold a non-empty value (repeatable)")
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
