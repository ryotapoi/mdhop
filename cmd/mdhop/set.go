package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, setHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	file := fs.String("file", "", "file to update")
	key := fs.String("key", "", "frontmatter key to set")
	value := fs.String("value", "", "frontmatter value to write")
	date := fs.String("date", "", "relative date expression to write as YYYY-MM-DD")
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
	valueSpecified := false
	dateSpecified := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "value":
			valueSpecified = true
		case "date":
			dateSpecified = true
		}
	})
	if valueSpecified == dateSpecified {
		return fmt.Errorf("exactly one of --value or --date is required")
	}
	writeValue := *value
	if valueSpecified {
		if writeValue == "" {
			return fmt.Errorf("--value cannot be empty")
		}
	} else {
		if *date == "" {
			return fmt.Errorf("--date cannot be empty")
		}
		expanded, ok := core.ExpandRelativeDate(*date, time.Now())
		if !ok {
			return fmt.Errorf("--date must use relative date syntax such as today, today-90d, or today+1y")
		}
		writeValue = expanded
	}
	result, err := core.Set(*vault, core.SetOptions{
		File:  *file,
		Key:   *key,
		Value: writeValue,
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
