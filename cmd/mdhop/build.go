package main

import (
	"flag"

	"github.com/ryotapoi/mdhop/internal/core"
)

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return core.Build(*vault)
}
