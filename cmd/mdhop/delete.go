package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

// multiString implements flag.Value for repeated flags.
type multiString []string

func (m *multiString) String() string { return strings.Join(*m, ",") }
func (m *multiString) Set(v string) error {
	*m = append(*m, v)
	return nil
}

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
