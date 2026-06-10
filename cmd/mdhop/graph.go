package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runGraph(args []string) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "json", "output format (json or dot)")
	includePhantoms := fs.Bool("include-phantoms", false, "include phantom nodes referenced from in-set notes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict nodes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude nodes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// graph has no text format; validate before opening the DB.
	if *format != "json" && *format != "dot" {
		return fmt.Errorf("invalid format: %s (graph supports json or dot)", *format)
	}

	result, err := core.Graph(*vault, core.GraphOptions{
		Path:            pathPatterns,
		Exclude:         excludePaths,
		IncludePhantoms: *includePhantoms,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "dot":
		return printGraphDot(os.Stdout, result)
	default:
		return printGraphJSON(os.Stdout, result)
	}
}
