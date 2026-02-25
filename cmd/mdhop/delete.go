package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

// isDirArg returns true if the argument refers to a directory
// (trailing slash or existing directory on disk).
func isDirArg(vaultPath, arg string) bool {
	if strings.HasSuffix(arg, "/") {
		return true
	}
	info, err := os.Stat(filepath.Join(vaultPath, arg))
	return err == nil && info.IsDir()
}

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	vault := fs.String("vault", ".", "vault root directory")
	format := fs.String("format", "text", "output format (json or text)")
	rm := fs.Bool("rm", false, "remove files from disk before updating index")
	var files multiString
	fs.Var(&files, "file", "file to delete (can be specified multiple times)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("--file is required")
	}

	// Expand directory arguments to individual files.
	var hasDirArg bool
	var expanded []string
	for _, f := range files {
		if isDirArg(*vault, f) {
			hasDirArg = true
			dirPrefix := core.NormalizePath(strings.TrimSuffix(f, "/"))
			notes, err := core.ListDirNotes(*vault, dirPrefix)
			if err != nil {
				return err
			}
			assets, err := core.ListDirAssets(*vault, dirPrefix)
			if err != nil {
				return err
			}
			if len(notes) == 0 && len(assets) == 0 {
				return fmt.Errorf("no files registered under directory: %s", f)
			}
			expanded = append(expanded, notes...)
			expanded = append(expanded, assets...)
		} else {
			expanded = append(expanded, f)
		}
	}

	result, err := core.Delete(*vault, core.DeleteOptions{Files: expanded, RemoveFiles: *rm})
	if err != nil {
		return err
	}

	// Clean up after --rm with directory mode.
	if *rm && hasDirArg {
		// Remove any remaining unregistered files on disk (D5: disk-based deletion).
		for _, f := range files {
			if !isDirArg(*vault, f) {
				continue
			}
			dirPrefix := core.NormalizePath(strings.TrimSuffix(f, "/"))
			absDir := filepath.Join(*vault, dirPrefix)
			// Walk to delete remaining files (assets added after build, etc.).
			_ = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil // best-effort
				}
				if info.IsDir() {
					if strings.HasPrefix(info.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				// Only remove non-.md files (D5: disk-based deletion for assets only).
				if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
					return nil
				}
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return nil // best-effort
				}
				return nil
			})
		}

		// Collect all deleted/phantomed files for directory cleanup.
		var allPaths []string
		allPaths = append(allPaths, result.Deleted...)
		allPaths = append(allPaths, result.Phantomed...)
		if err := core.CleanupEmptyDirs(*vault, allPaths); err != nil {
			return err
		}
	}

	switch *format {
	case "json":
		return printDeleteJSON(os.Stdout, result)
	default:
		printDeleteText(os.Stdout, result)
		return nil
	}
}
