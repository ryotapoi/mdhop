package main

import (
	"flag"
	"github.com/ryotapoi/mdhop/internal/core"
)

const migrateHelp = `Usage: mdhop migrate [--vault <path>]

Migrate <vault>/mdhop.yaml to mdhop.toml and rebuild the default cache index.
After a successful rebuild, remove the old YAML and <vault>/.mdhop/.
With TOML only, use that file unchanged; without either config, use defaults.

Options:
  --vault <path>  Vault root directory. Default: current directory.

--db, --config, and positional arguments are not accepted.
Both YAML and TOML present is an error; neither file is changed.
Unknown or unconvertible YAML settings are errors. YAML merge keys are unsupported.
Conversion, save, or rebuild failure retains the old YAML and .mdhop/.
A new TOML is rolled back on rebuild failure. Rollback or cleanup failure reports
remaining paths; inspect them before retrying. After cleanup failure, the new
cache index is already published. No stdout output on success; warnings use stderr.

Examples:
  mdhop migrate
  mdhop migrate --vault ~/Notes

`

func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, migrateHelp)
	vault := fs.String("vault", ".", "vault root directory")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	result, err := core.Migrate(*vault)
	if err != nil {
		return err
	}
	printWarnings(result.Warnings)
	return nil
}
