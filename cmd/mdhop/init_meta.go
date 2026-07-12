package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryotapoi/mdhop/internal/core"
)

const initMetaHelp = `Usage: mdhop init-meta (--preset|--scan) [--write] [--no-comment] [--vault <path>]

Generate mdhop.yaml meta type definitions from presets, a vault scan, or both.

Options:
  --preset        Required unless --scan is set. Include recommended preset type definitions.
  --scan          Required unless --preset is set. Infer type definitions from vault frontmatter.
  --write         Optional. Write to mdhop.yaml instead of stdout.
  --no-comment    Optional. Omit explanatory comments from generated YAML.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  YAML is written to stdout by default. With --write, mdhop.yaml is updated in place.

Examples:
  mdhop init-meta --preset --scan
  mdhop init-meta --preset --scan --write
  mdhop init-meta --scan --no-comment

`

func runInitMeta(args []string) error {
	fs := flag.NewFlagSet("init-meta", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, initMetaHelp)
	vault := fs.String("vault", ".", "vault root directory")
	preset := fs.Bool("preset", false, "include recommended type definitions")
	scan := fs.Bool("scan", false, "scan vault and infer types from frontmatter")
	write := fs.Bool("write", false, "write to mdhop.yaml (default: stdout)")
	noComment := fs.Bool("no-comment", false, "omit comments from output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := core.InitMeta(*vault, core.InitMetaOptions{
		Preset:    *preset,
		Scan:      *scan,
		NoComment: *noComment,
	})
	if err != nil {
		return err
	}

	if *write {
		configPath := filepath.Join(*vault, "mdhop.yaml")
		// Atomic write: temp file + rename
		tmpPath := configPath + ".tmp"
		if err := os.WriteFile(tmpPath, []byte(result.YAML), 0644); err != nil {
			return fmt.Errorf("write temp file: %w", err)
		}
		if err := os.Rename(tmpPath, configPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename: %w", err)
		}
		// Report to stderr
		if len(result.Added) > 0 {
			fmt.Fprintf(os.Stderr, "added %d type(s) to %s\n", len(result.Added), configPath)
		}
		if len(result.Skipped) > 0 {
			fmt.Fprintf(os.Stderr, "skipped %d existing type(s)\n", len(result.Skipped))
		}
	} else {
		fmt.Print(result.YAML)
	}

	return nil
}
