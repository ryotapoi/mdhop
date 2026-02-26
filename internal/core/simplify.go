package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SimplifyOptions controls the simplify operation.
type SimplifyOptions struct {
	DryRun bool
	Files  []string
}

// SimplifyResult reports the outcome of the simplify operation.
type SimplifyResult struct {
	Rewritten []RewrittenLink
	Skipped   []SkippedLink
}

// Simplify rewrites path links to basename links when the basename is unique
// or can be resolved via root-priority. It works by scanning files directly
// (no DB required).
func Simplify(vaultPath string, opts SimplifyOptions) (*SimplifyResult, error) {
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)

	sort.Strings(files)

	// Build resolve maps.
	nm := buildNoteResolveMaps(files)
	am := buildAssetResolveMaps(assetFiles)

	// Build file set for validation.
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	// Validate --file flags.
	fileScope := make(map[string]bool)
	for _, f := range opts.Files {
		np := NormalizePath(f)
		if !fileSet[np] {
			return nil, fmt.Errorf("file not found: %s", np)
		}
		fileScope[np] = true
	}

	// Determine scan files.
	scanFiles := files
	if len(fileScope) > 0 {
		scanFiles = nil
		for _, f := range files {
			if fileScope[f] {
				scanFiles = append(scanFiles, f)
			}
		}
	}

	result := &SimplifyResult{}
	var rewrites []rewriteEntry
	skippedSet := make(map[string]bool) // "file\x00rawLink" dedup

	for _, sourcePath := range scanFiles {
		fullPath := filepath.Join(vaultPath, sourcePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		links := parseLinks(string(content))

		for _, lo := range links {
			if lo.linkType != "wikilink" && lo.linkType != "markdown" {
				continue
			}
			if lo.target == "" {
				continue // self-link
			}
			if lo.isBasename {
				continue // already basename
			}

			// Skip vault-escape links.
			if isLinkEscaping(sourcePath, lo) {
				continue
			}

			// Resolve to vault-relative path.
			resolved := resolveToVaultRelative(sourcePath, lo)
			lower := strings.ToLower(resolved)

			// Determine namespace: note first, then asset.
			var resolvedPath string
			isAsset := false

			if actual, ok := nm.pathSetLower[lower]; ok {
				resolvedPath = actual
			} else if actual, ok := nm.pathSetLower[lower+".md"]; ok {
				resolvedPath = actual
			} else if actual, ok := am.pathSetLower[lower]; ok {
				resolvedPath = actual
				isAsset = true
			} else {
				continue // broken link, skip
			}

			// Check if simplification is possible.
			var basenameTarget string
			var canSimplify bool
			var skippedCandidates []string

			if isAsset {
				// Asset namespace collision check: if a note has the same basename key,
				// simplifying would change resolution from asset to note.
				abk := assetBasenameKey(resolvedPath)
				if nm.basenameCounts[abk] > 0 {
					continue // namespace conflict, skip silently
				}

				canSimplify, skippedCandidates = canSimplifyAsset(resolvedPath, am)
				basenameTarget = filepath.Base(resolvedPath)
			} else {
				canSimplify, skippedCandidates = canSimplifyNote(resolvedPath, nm)
				basenameTarget = filepath.Base(resolvedPath)
			}

			if !canSimplify {
				if len(skippedCandidates) > 0 {
					key := sourcePath + "\x00" + lo.rawLink
					if !skippedSet[key] {
						skippedSet[key] = true
						bn := filepath.Base(resolvedPath)
						if !isAsset {
							bn = basename(resolvedPath)
						}
						sorted := make([]string, len(skippedCandidates))
						copy(sorted, skippedCandidates)
						sort.Strings(sorted)
						result.Skipped = append(result.Skipped, SkippedLink{
							File:       sourcePath,
							RawLink:    lo.rawLink,
							Basename:   bn,
							Candidates: sorted,
						})
					}
				}
				continue
			}

			newRawLink := rewriteRawLink(lo.rawLink, lo.linkType, basenameTarget)
			if newRawLink == lo.rawLink {
				continue
			}

			rewrites = append(rewrites, rewriteEntry{
				rawLink:    lo.rawLink,
				linkType:   lo.linkType,
				lineStart:  lo.lineStart,
				sourcePath: sourcePath,
				newRawLink: newRawLink,
			})
		}
	}

	// Build rewritten result entries.
	for _, re := range rewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	if opts.DryRun || len(rewrites) == 0 {
		return result, nil
	}

	// Apply disk rewrites.
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	_, _, applyErr := applyFileRewrites(vaultPath, groups)
	if applyErr != nil {
		return nil, applyErr
	}

	return result, nil
}

// canSimplifyNote checks if a note path link can be simplified to basename.
// Returns (canSimplify, candidatesIfSkipped).
// candidatesIfSkipped is non-nil only when the link is ambiguous and should be reported as skipped.
func canSimplifyNote(resolvedPath string, nm noteResolveMaps) (bool, []string) {
	bk := basenameKey(resolvedPath)
	count := nm.basenameCounts[bk]
	if count == 1 {
		return true, nil
	}
	// count > 1: check root-priority.
	if rootPath, ok := nm.rootBasenameToPath[bk]; ok {
		if resolvedPath == rootPath {
			return true, nil // link points to root file, basename resolves there
		}
		// Link points to non-root file, but basename would resolve to root.
		// This is an intentional path link, skip silently.
		return false, nil
	}
	// No root file — ambiguous, report as skipped.
	return false, collectNoteBasenameFiles(bk, nm)
}

// canSimplifyAsset checks if an asset path link can be simplified to basename.
func canSimplifyAsset(resolvedPath string, am assetResolveMaps) (bool, []string) {
	abk := assetBasenameKey(resolvedPath)
	count := am.basenameCounts[abk]
	if count == 1 {
		return true, nil
	}
	// count > 1: check root-priority.
	if rootPath, ok := am.rootBasenameToPath[abk]; ok {
		if resolvedPath == rootPath {
			return true, nil
		}
		return false, nil
	}
	return false, collectAssetBasenameFiles(abk, am)
}

// collectNoteBasenameFiles returns all note paths matching a basename key.
func collectNoteBasenameFiles(bk string, nm noteResolveMaps) []string {
	var paths []string
	for lower, actual := range nm.pathSetLower {
		if basenameKey(actual) == bk && strings.HasSuffix(lower, ".md") {
			paths = append(paths, actual)
		}
	}
	return paths
}

// collectAssetBasenameFiles returns all asset paths matching an asset basename key.
func collectAssetBasenameFiles(abk string, am assetResolveMaps) []string {
	var paths []string
	for _, actual := range am.pathSetLower {
		if assetBasenameKey(actual) == abk {
			paths = append(paths, actual)
		}
	}
	return paths
}
