package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

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
		// Directory mode.
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

	// Single file mode.
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
