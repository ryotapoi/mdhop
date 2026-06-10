package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict source notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude source notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validDiagnoseFieldsCLI, "diagnose"); err != nil {
		return err
	}

	result, err := core.Diagnose(*vault, core.DiagnoseOptions{
		Fields:  fieldList,
		Path:    pathPatterns,
		Exclude: excludePaths,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printDiagnoseJSON(os.Stdout, result, fieldList)
	default:
		return printDiagnoseText(os.Stdout, result, fieldList)
	}
}
