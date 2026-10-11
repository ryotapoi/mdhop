package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const initMetaHelp = `Usage: mdhop init-meta (--preset|--scan) [--write] [--no-comment] [--vault <path>]

Generate mdhop.toml meta type definitions from presets, a vault scan, or both.

Options:
  --db <path>      Index DB path. Default: <vault>/.mdhop/index.sqlite.
  --config <path>  Read only this config file. Default: <vault>/mdhop.toml; missing default allowed.
  --preset        Required unless --scan is set. Include recommended preset type definitions.
  --scan          Required unless --preset is set. Infer type definitions from vault frontmatter.
  --write         Optional. Write to the selected configuration file instead of stdout.
  --no-comment    Optional. Omit explanatory comments from generated TOML.
  --vault <path>  Optional. Vault root directory. Default: ".".

Output:
  TOML is written to stdout by default. With --write, update --config when specified,
  otherwise <vault>/mdhop.toml. An explicit --config file must already exist;
  a missing explicit file is an error. A missing default file may be created.

Location paths may be absolute or relative to the current directory.
Note paths and configuration globs remain relative to the vault.

Examples:
  mdhop init-meta --preset --scan
  mdhop init-meta --preset --scan --write
  mdhop init-meta --scan --no-comment

`

func runInitMeta(args []string) error {
	fs := flag.NewFlagSet("init-meta", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, initMetaHelp)
	vault := fs.String("vault", ".", "vault root directory")
	db := fs.String("db", "", "index database path (relative to current directory)")
	config := fs.String("config", "", "configuration file path (relative to current directory)")
	preset := fs.Bool("preset", false, "include recommended type definitions")
	scan := fs.Bool("scan", false, "scan vault and infer types from frontmatter")
	write := fs.Bool("write", false, "write to the selected configuration file (default: stdout)")
	noComment := fs.Bool("no-comment", false, "omit comments from output")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	locations := core.Locations{DBPath: *db, ConfigPath: *config}
	if _, err := core.LoadConfig(*vault, locations); err != nil {
		return err
	}

	result, err := core.InitMeta(*vault, core.InitMetaOptions{
		Preset:    *preset,
		Scan:      *scan,
		NoComment: *noComment,
	}, locations)
	if err != nil {
		return err
	}

	if *write {
		configPath := locations.ConfigFile(*vault)
		tmpPath := configPath + ".tmp-" + rand.Text()
		tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		defer os.Remove(tmpPath)
		if _, err := tmp.WriteString(result.TOML); err != nil {
			tmp.Close()
			return fmt.Errorf("write temp file: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("close temp file: %w", err)
		}
		if err := os.Rename(tmpPath, configPath); err != nil {
			return fmt.Errorf("rename: %w", err)
		}
		if len(result.Added) > 0 {
			fmt.Fprintf(os.Stderr, "added %d type(s) to %s\n", len(result.Added), configPath)
		}
		if len(result.Skipped) > 0 {
			fmt.Fprintf(os.Stderr, "skipped %d existing type(s)\n", len(result.Skipped))
		}
	} else {
		_, err := fmt.Print(result.TOML)
		return err
	}

	return nil
}
