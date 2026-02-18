package main

import (
	"flag"
	"fmt"
	"os"

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
