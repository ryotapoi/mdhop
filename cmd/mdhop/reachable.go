package main

import (
	"flag"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const reachableHelp = `Usage: mdhop reachable --from <path> [--path <glob>...] [--exclude <glob>...] [--route] [--fields <list>] [--vault <path>] [--format json|text]

Split notes into reachable and unreachable sets from an entry note by following links.

Options:
  --from <path>       Required. Vault-relative entry note path.
  --path <glob>       Optional, repeatable. Include target note paths matching any glob.
  --exclude <glob>    Optional, repeatable. Exclude target note paths matching the glob.
  --route             Optional. Include shortest routes to reachable notes.
  --fields <list>     Optional. Comma-separated fields: reachable,unreachable.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  from         Normalized entry path; always included in JSON.
  reachable    Notes reachable from --from within the target set.
  unreachable  Notes in the target set not reachable from --from.
  routes       Shortest routes to reachable notes; only with --route.

Examples:
  mdhop reachable --from index.md --path "docs/*" --route --format json
  mdhop reachable --from index.md --fields unreachable --format json

`

func runReachable(args []string) error {
	fs := flag.NewFlagSet("reachable", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, reachableHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	from := fs.String("from", "", "entry note path (vault-relative)")
	route := fs.Bool("route", false, "include shortest routes for reachable notes")
	var pathPatterns multiString
	var excludePaths multiString
	fs.Var(&pathPatterns, "path", "restrict target notes to paths matching glob (repeatable)")
	fs.Var(&excludePaths, "exclude", "exclude target notes matching glob (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateFormat(*format); err != nil {
		return err
	}

	fieldList := parseFields(*fields)
	if err := validateFields(fieldList, validReachableFieldsCLI, "reachable"); err != nil {
		return err
	}

	result, err := core.Reachable(*vault, core.ReachableOptions{
		From:    *from,
		Path:    pathPatterns,
		Exclude: excludePaths,
		Route:   *route,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printReachableJSON(os.Stdout, result, fieldList)
	default:
		return printReachableText(os.Stdout, result, fieldList)
	}
}
