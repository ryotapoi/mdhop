package main

import (
	"flag"
	"fmt"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	var files multiString
	fs.Var(&files, "file", "file to delete (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	_, err := core.Delete(*vault, core.DeleteOptions{Files: files})
	return err
}
