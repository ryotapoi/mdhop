package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.Build(*vault)
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return nil
}
