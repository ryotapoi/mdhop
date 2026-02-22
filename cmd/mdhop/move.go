package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runMove(args []string) error {
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	from := fs.String("from", "", "source file path (vault-relative)")
	to := fs.String("to", "", "destination file path (vault-relative)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if *to == "" {
		return fmt.Errorf("--to is required")
	}

	fromIsDir := isDirArg(*vault, *from)

	if fromIsDir {
		// Directory mode.
		toIsFile := strings.HasSuffix(strings.ToLower(*to), ".md")
		if toIsFile {
			return fmt.Errorf("--to looks like a file path, use trailing / for directory move")
		}
		fromDir := core.NormalizePath(strings.TrimSuffix(*from, "/"))
		toDir := core.NormalizePath(strings.TrimSuffix(*to, "/"))
		result, err := core.MoveDir(*vault, core.MoveDirOptions{
			FromDir: fromDir,
			ToDir:   toDir,
		})
		if err != nil {
			return err
		}
		switch *format {
		case "json":
			return printMoveDirJSON(os.Stdout, result)
		default:
			printMoveDirText(os.Stdout, result)
			return nil
		}
	}

	// Single file mode.
	toIsDir := strings.HasSuffix(*to, "/")
	if toIsDir {
		return fmt.Errorf("cannot use directory destination for single file move")
	}

	result, err := core.Move(*vault, core.MoveOptions{
		From: *from,
		To:   *to,
	})
	if err != nil {
		return err
	}
	normalizedFrom := core.NormalizePath(*from)
	normalizedTo := core.NormalizePath(*to)
	switch *format {
	case "json":
		return printMoveJSON(os.Stdout, normalizedFrom, normalizedTo, result)
	default:
		printMoveText(os.Stdout, normalizedFrom, normalizedTo, result)
		return nil
	}
}
