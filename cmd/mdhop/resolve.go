package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runResolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	from := fs.String("from", "", "source file (vault-relative path)")
	link := fs.String("link", "", "link text to resolve")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if *link == "" {
		return fmt.Errorf("--link is required")
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	parsedFields := parseFields(*fields)
	if err := validateResolveFields(parsedFields); err != nil {
		return err
	}

	result, err := core.Resolve(*vault, *from, *link)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printResolveJSON(os.Stdout, result, parsedFields)
	default:
		return printResolveText(os.Stdout, result, parsedFields)
	}
}
