package core

import (
	"os"
	"sort"
)

// scanRewriteOptions describes one disk-only markdown rewrite operation.
// Commands provide a preparation callback for command-specific file validation
// and link rewriting.
type scanRewriteOptions struct {
	DryRun  bool
	Prepare func(files, excludePaths []string) (scanRewritePlan, error)
}

// scanRewritePlan selects already-collected source files and decides their
// rewrites. Its files must be drawn from the list supplied to Prepare.
type scanRewritePlan struct {
	Files   []string
	Rewrite func(sourcePath string, content []byte) ([]rewriteEntry, error)
}

// scanAndRewrite collects eligible markdown files, applies build exclusions,
// gathers command-specific rewrites, and applies them unless dry-run is set.
func scanAndRewrite(vaultPath string, opts scanRewriteOptions) ([]rewriteEntry, error) {
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}
	excludePaths := cfg.Build.ExcludePaths
	if err := validateGlobPatterns(excludePaths); err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, excludePaths)
	sort.Strings(files)

	plan, err := opts.Prepare(files, excludePaths)
	if err != nil {
		return nil, err
	}

	diskPaths := newVaultDiskPathResolver(vaultPath)
	var rewrites []rewriteEntry
	for _, sourcePath := range plan.Files {
		fullPath, err := diskPaths.existingPath(sourcePath)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		entries, err := plan.Rewrite(sourcePath, content)
		if err != nil {
			return nil, err
		}
		rewrites = append(rewrites, entries...)
	}

	if len(rewrites) == 0 {
		return rewrites, nil
	}
	prepared, err := prepareFileRewrites(vaultPath, rewrites)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return rewrites, nil
	}

	if _, _, rollbackFailures, err := applyPreparedFileRewrites(prepared); err != nil {
		return nil, wrapRollbackFailures(err, rollbackFailures)
	}

	return rewrites, nil
}
