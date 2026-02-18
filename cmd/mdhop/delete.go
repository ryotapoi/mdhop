package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	rm := fs.Bool("rm", false, "remove files from disk before updating index")
	var files multiString
	fs.Var(&files, "file", "file to delete (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}
	result, err := core.Delete(*vault, core.DeleteOptions{Files: files, RemoveFiles: *rm})
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		return printDeleteJSON(os.Stdout, result)
	default:
		printDeleteText(os.Stdout, result)
		return nil
	}
}
