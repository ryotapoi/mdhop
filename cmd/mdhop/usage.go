package main

import (
	"flag"
	"fmt"
)

func commandUsage(fs *flag.FlagSet, help string) func() {
	return func() {
		fmt.Fprint(fs.Output(), help)
	}
}
