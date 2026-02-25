package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RepairOptions controls the repair operation.
type RepairOptions struct {
	DryRun bool
}

// RepairResult reports the outcome of the repair operation.
type RepairResult struct {
	Rewritten []RewrittenLink
	Skipped   []SkippedLink
}

// SkippedLink reports a broken path link that could not be auto-repaired.
type SkippedLink struct {
	File       string
	RawLink    string
	Basename   string
	Candidates []string
}

// Repair rewrites broken path links and vault-escape links to basename links.
// It works by scanning files directly (no DB required).
// Vault-escape links are always converted to basename (escape resolution is top priority).
// Broken path links are converted when 0-1 candidates exist; 2+ candidates are skipped.
func Repair(vaultPath string, opts RepairOptions) (*RepairResult, error) {
	// Collect all .md files.
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

	sort.Strings(files)

	// Build path set (lowercase) and basename map.
	pathSetLower := make(map[string]bool, len(files))
	basenameMap := make(map[string][]string) // lowercase basename (with ext stripped only .md) → []vault-relative paths
	for _, f := range files {
		pathSetLower[strings.ToLower(f)] = true
		// Use lowercase of filepath.Base minus .md extension as key.
		base := filepath.Base(f)
		key := strings.ToLower(base)
		if strings.HasSuffix(key, ".md") {
			key = key[:len(key)-3]
		}
		basenameMap[key] = append(basenameMap[key], f)
	}

	result := &RepairResult{}
	var rewrites []rewriteEntry
	skippedSet := make(map[string]bool) // "file\x00rawLink" dedup

	for _, sourcePath := range files {
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
				continue // self-link [[#Heading]]
			}
			if lo.isBasename {
				continue // already basename form
			}

			escaping := isLinkEscaping(sourcePath, lo)
			if escaping {
				// vault-escape → always a repair candidate (don't os.Stat outside vault)
			} else if isLinkBrokenForScan(sourcePath, lo, pathSetLower) {
				// Broken path link → protect links to excluded files that exist on disk
				if linkTargetExistsRaw(vaultPath, sourcePath, lo) {
					continue
				}
			} else {
				continue // normal path link → skip
			}

			// Extract basename preserving original case. parseLinks already stripped .md via normalizeBasename.
			bn := filepath.Base(lo.target)
			bk := strings.ToLower(bn) // lookup key (don't use basenameKey — it strips all extensions)
			candidates := basenameMap[bk]

			if !escaping && len(candidates) >= 2 {
				// Broken path link + 2+ candidates → skip, report with dedup
				key := sourcePath + "\x00" + lo.rawLink
				if !skippedSet[key] {
					skippedSet[key] = true
					sorted := make([]string, len(candidates))
					copy(sorted, candidates)
					sort.Strings(sorted)
					result.Skipped = append(result.Skipped, SkippedLink{
						File:       sourcePath,
						RawLink:    lo.rawLink,
						Basename:   bn,
						Candidates: sorted,
					})
				}
				continue
			}

			// vault-escape: always basename-ify regardless of candidate count
			// broken path link: 0-1 candidates → basename-ify
			newRawLink := rewriteRawLink(lo.rawLink, lo.linkType, bn+".md")
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

// isLinkEscaping checks if a link escapes the vault boundary.
// Mirrors build.go's validation logic:
//   - relative links (./  ../) → escapesVault()
//   - path links (not basename, not relative) → pathEscapesVault()
//   - basename links → never escape
func isLinkEscaping(sourcePath string, lo linkOccur) bool {
	if lo.isRelative {
		return escapesVault(sourcePath, lo.target)
	}
	if !lo.isBasename {
		return pathEscapesVault(lo.target)
	}
	return false
}

// linkTargetExistsRaw checks if a link target resolves to an existing file on disk.
// Used to protect broken path links that point to files excluded by build.exclude_paths.
// NOT used for vault-escape links (they point outside the vault, so os.Stat is inappropriate).
func linkTargetExistsRaw(vaultPath, sourcePath string, lo linkOccur) bool {
	target := lo.target

	var resolved string
	if lo.isRelative {
		resolved = NormalizePath(filepath.Join(filepath.Dir(sourcePath), target))
	} else if strings.HasPrefix(target, "/") {
		resolved = strings.TrimPrefix(target, "/")
	} else {
		resolved = target
	}

	full := filepath.Join(vaultPath, resolved)
	if _, err := os.Stat(full); err == nil {
		return true
	}
	if !strings.HasSuffix(strings.ToLower(resolved), ".md") {
		if _, err := os.Stat(full + ".md"); err == nil {
			return true
		}
	}
	return false
}
