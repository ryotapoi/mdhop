package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ryotapoi/mdhop/internal/core"
)

const resolveHelp = `Usage: mdhop resolve --from <path> --link <link text> [--fields <list>] [--vault <path>] [--format json|text]

Resolve one link as written from a source note.
Exact indexed link text uses the source snapshot, including table aliases and
reference definitions. Occurrences with different indexed targets or fragments
are ambiguous. Reference links require exact indexed text; other links fall back
to normalized matching. Rebuild or update the index after editing source notes.
Markdown destinations decode backslash escapes, HTML entities, and percent escapes.

Options:
  --from <path>       Required. Vault-relative source note path.
  --link <link text>  Required. Link text, such as '[[Spec]]', '[Spec](Spec.md)', or '[Spec][guide]'.
  --fields <list>     Optional. Comma-separated output fields.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Fields:
  type     Target type: note, phantom, tag, url, or asset.
  name     Display name.
  path     Vault-relative path for note and asset targets.
  exists   Existence flag for note and asset targets.
  subpath  Heading or block fragment such as #Heading or #^block.

Examples:
  mdhop resolve --from Notes/Design.md --link '[[Spec]]' --format json
  mdhop resolve --from Notes/Design.md --link '[Spec](Spec.md)' --fields type,path --format json
  mdhop resolve --from Notes/Design.md --link '[Spec][guide]' --format json
  mdhop resolve --from Notes/Design.md --link '#architecture'

`

func runResolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, resolveHelp)
	vault := fs.String("vault", ".", "vault root directory")
	from := fs.String("from", "", "source file (vault-relative path)")
	link := fs.String("link", "", "link text to resolve")
	format := fs.String("format", "text", "output format (json or text)")
	fields := fs.String("fields", "", "comma-separated fields to output")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if *link == "" {
		return fmt.Errorf("--link is required")
	}
	if err := validateFormat(*format); err != nil {
		return err
	}

	parsedFields := parseFields(*fields)
	if err := validateFields(parsedFields, validResolveFields, "resolve"); err != nil {
		return err
	}

	result, err := core.Resolve(*vault, *from, *link)
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return printResolveJSON(os.Stdout, result, parsedFields)
	default:
		return printResolveText(os.Stdout, result, parsedFields)
	}
}
