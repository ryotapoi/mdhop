package main

import (
	"flag"
	"fmt"
	"strings"
)

// multiString implements flag.Value for repeated flags.
type multiString []string

func (m *multiString) String() string { return strings.Join(*m, ",") }
func (m *multiString) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// parseFlags rejects positional arguments and explicit empty location paths before processing.
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional argument: %q", fs.Arg(0))
	}
	var locationErr error
	fs.Visit(func(f *flag.Flag) {
		if locationErr == nil && (f.Name == "db" || f.Name == "config") && f.Value.String() == "" {
			locationErr = fmt.Errorf("--%s requires a nonempty path", f.Name)
		}
	})
	return locationErr
}
