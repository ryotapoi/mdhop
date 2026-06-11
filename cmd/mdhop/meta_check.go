package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runMetaCheck(args []string) error {
	fs := flag.NewFlagSet("meta-check", flag.ContinueOnError)
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
