package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	file := fs.String("file", "", "file to update")
	key := fs.String("key", "", "frontmatter key to set")
	value := fs.String("value", "", "frontmatter value to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}
	if *key == "" {
		return fmt.Errorf("--key is required")
	}
	if *value == "" {
		return fmt.Errorf("--value is required")
	}
	result, err := core.Set(*vault, core.SetOptions{
		File:  *file,
		Key:   *key,
		Value: *value,
	})
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	switch *format {
	case "json":
		return printSetJSON(os.Stdout, result)
	default:
		printSetText(os.Stdout, result)
		return nil
	}
}
