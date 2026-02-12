package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	var files multiString
	fs.Var(&files, "file", "file to update (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	_, err := core.Update(*vault, core.UpdateOptions{Files: files})
	return err
}
