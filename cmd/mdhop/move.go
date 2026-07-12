package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

const moveHelp = `Usage: mdhop move --from <path> (--to <path>|--to-template <template>) [--dry-run] [--vault <path>] [--format json|text]

Move a registered file or directory and rewrite links needed to preserve meaning.

Options:
  --from <path>       Required. Vault-relative source file or directory. A trailing / or disk directory enables directory mode.
  --to <path>         Vault-relative destination file or directory. Mutually exclusive with --to-template.
  --to-template <template>
                      Destination template for note moves. Supports {field}, {field|fallback}, {dateField:year}, {dateField:month}, {dateField:day}, date-part fallback such as {dateField:year|2099}, and {basename}.
  --dry-run           Optional with --to-template. Report the expanded move plan without changing disk or DB.
  --vault <path>      Optional. Vault root directory. Default: ".".
  --format json|text  Optional. Output format. Default: text.

Behavior notes:
  The source file fails stale detection when its mtime does not match the DB record; external files rewritten as collateral are not stale-checked.
  If --from is missing on disk and --to already exists, the move is treated as already completed and only link rewrites plus DB updates are performed.
  Existing --to paths on disk fail to prevent overwrites.
  Directory moves fail when --from and --to contain each other, such as --from sub --to sub/inner.
  --to-template is incompatible with --to. In directory mode it expands every registered source note under --from, prevalidates every destination, and moves notes as one batch.
  Template fields are read from indexed source-note frontmatter. Missing fields without fallback, fields with multiple values, invalid date extraction, placeholder values containing /, and empty or vault-escaping expanded destinations fail before Move changes files.
  When meta.link_keys is configured, frontmatter raw path values cannot be rewritten; move fails before changing anything if existing raw path values would resolve differently.

Output fields:
  from       Source path for single-file moves.
  to         Destination path for single-file moves.
  moved      Directory-mode moves as an array of from/to pairs.
  rewritten  Files whose links were rewritten.

Examples:
  mdhop move --from Notes/Old.md --to Notes/New.md --format json
  mdhop move --from Notes/Project.md --to-template "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}" --format json
  mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
  mdhop move --from OldDir/ --to NewDir/ --format json

`

func runMove(args []string) error {
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	fs.Usage = commandUsage(fs, moveHelp)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	from := fs.String("from", "", "source file path (vault-relative)")
	to := fs.String("to", "", "destination file path (vault-relative)")
	toTemplate := fs.String("to-template", "", "destination template expanded from source frontmatter")
	dryRun := fs.Bool("dry-run", false, "show the --to-template move plan without making changes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if *to == "" && *toTemplate == "" {
		return fmt.Errorf("--to or --to-template is required")
	}
	if *to != "" && *toTemplate != "" {
		return fmt.Errorf("--to and --to-template cannot be used together")
	}

	fromIsDir := isDirArg(*vault, *from)
	if *dryRun && *toTemplate == "" {
		return fmt.Errorf("--dry-run is only supported with --to-template")
	}

	if *toTemplate != "" {
		opts := core.MoveTemplateOptions{
			From:      *from,
			Template:  *toTemplate,
			Directory: fromIsDir,
		}
		plan, err := core.PlanMoveTemplate(*vault, opts)
		if err != nil {
			return err
		}
		if *dryRun {
			return printMoveTemplatePlan(*format, fromIsDir, plan, nil)
		}
		result, err := core.MoveTemplate(*vault, opts)
		if err != nil {
			return err
		}
		return printMoveTemplatePlan(*format, fromIsDir, plan, result)
	}

	if fromIsDir {
		toIsFile := strings.HasSuffix(strings.ToLower(*to), ".md")
		if toIsFile {
			return fmt.Errorf("--to looks like a file path, use trailing / for directory move")
		}
		fromDir := core.NormalizePath(strings.TrimSuffix(*from, "/"))
		toDir := core.NormalizePath(strings.TrimSuffix(*to, "/"))
		result, err := core.MoveDir(*vault, core.MoveDirOptions{
			FromDir: fromDir,
			ToDir:   toDir,
		})
		if err != nil {
			return err
		}
		switch *format {
		case "json":
			return printMoveDirJSON(os.Stdout, result)
		default:
			printMoveDirText(os.Stdout, result)
			return nil
		}
	}

	toIsDir := strings.HasSuffix(*to, "/")
	if toIsDir {
		return fmt.Errorf("cannot use directory destination for single file move")
	}

	result, err := core.Move(*vault, core.MoveOptions{
		From: *from,
		To:   *to,
	})
	if err != nil {
		return err
	}
	normalizedFrom := core.NormalizePath(*from)
	normalizedTo := core.NormalizePath(*to)
	switch *format {
	case "json":
		return printMoveJSON(os.Stdout, normalizedFrom, normalizedTo, result)
	default:
		printMoveText(os.Stdout, normalizedFrom, normalizedTo, result)
		return nil
	}
}

func printMoveTemplatePlan(format string, isDir bool, plan *core.MoveTemplatePlanResult, result *core.MoveDirResult) error {
	rewritten := []core.RewrittenLink(nil)
	if result != nil {
		rewritten = result.Rewritten
	}
	if isDir {
		dirResult := &core.MoveDirResult{Moved: plan.Moved, Rewritten: rewritten}
		switch format {
		case "json":
			return printMoveDirJSON(os.Stdout, dirResult)
		default:
			printMoveDirText(os.Stdout, dirResult)
			return nil
		}
	}
	if len(plan.Moved) != 1 {
		return fmt.Errorf("expected one --to-template move, got %d", len(plan.Moved))
	}
	moveResult := &core.MoveResult{Rewritten: rewritten}
	switch format {
	case "json":
		return printMoveJSON(os.Stdout, plan.Moved[0].From, plan.Moved[0].To, moveResult)
	default:
		printMoveText(os.Stdout, plan.Moved[0].From, plan.Moved[0].To, moveResult)
		return nil
	}
}
