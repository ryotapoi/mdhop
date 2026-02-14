package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runMove(args []string) error {
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	from := fs.String("from", "", "source file path (vault-relative)")
	to := fs.String("to", "", "destination file path (vault-relative)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if *to == "" {
		return fmt.Errorf("--to is required")
	}
	_, err := core.Move(*vault, core.MoveOptions{
		From: *from,
		To:   *to,
	})
	return err
}
