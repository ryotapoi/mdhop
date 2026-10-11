package core

import (
	"path/filepath"
	"sort"
	"strings"
)

// RepairOptions controls the repair operation.
type RepairOptions struct {
	DryRun  bool
	Path    []string // include globs for source notes (empty = all)
	Exclude []string // exclude globs for source notes
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

// Repair repairs broken and escaping body links. Markdown repairs name an
// explicit vault-root path; wikilink repairs use basename form.
// It works by scanning files directly (no DB required).
// Escaping wikilinks retain the existing unconditional basename repair policy.
// Markdown and non-escaping broken links skip ambiguous repair candidates.
func Repair(vaultPath string, opts RepairOptions, locations ...Locations) (*RepairResult, error) {
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	result := &RepairResult{}
	rewrites, err := scanAndRewrite(vaultPath, scanRewriteOptions{
		DryRun: opts.DryRun,
		Prepare: func(files, excludePaths []string) (scanRewritePlan, error) {
			diskPaths := newVaultDiskPathResolver(vaultPath)
			pathSetLower := make(map[string]bool, len(files))
			basenameMap := make(map[string][]string)
			for _, f := range files {
				pathSetLower[strings.ToLower(f)] = true
				key := strings.TrimSuffix(strings.ToLower(filepath.Base(f)), ".md")
				basenameMap[key] = append(basenameMap[key], f)
			}
			assetFiles, err := collectAssetFiles(vaultPath)
			if err != nil {
				return scanRewritePlan{}, err
			}
			assetBasenames := make(map[string][]string)
			for _, f := range filterBuildExcludes(assetFiles, excludePaths) {
				key := strings.ToLower(filepath.Base(f))
				assetBasenames[key] = append(assetBasenames[key], f)
			}
			var scanFiles []string
			for _, f := range files {
				if pathMatchesFilters(f, opts.Path, opts.Exclude) {
					scanFiles = append(scanFiles, f)
				}
			}
			skippedSet := make(map[string]bool) // "file\x00rawLink" dedup
			return scanRewritePlan{Files: scanFiles, Rewrite: func(sourcePath string, content []byte) ([]rewriteEntry, error) {
				links := parseLinks(string(content)).Links
				var entries []rewriteEntry
				for _, lo := range links {
					// Body links only. repair targets broken/escaping path links by
					// rewriting them to basename form, but frontmatter wikilinks are
					// routinely authored as full vault-relative paths and "broken"
					// detection there would produce noisy rewrites that change YAML
					// semantics.
					if !isBodyPathLinkType(lo.linkType) {
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
						if linkTargetExistsRaw(diskPaths, sourcePath, lo) {
							continue
						}
					} else {
						continue // normal path link → skip
					}

					// Extract basename preserving original case. parseLinks already stripped .md via normalizeBasename.
					bn := filepath.Base(lo.target)
					bk := strings.ToLower(bn) // lookup key (don't use basenameKey — it strips all extensions)
					candidates := basenameMap[bk]
					if lo.linkType == LinkTypeMarkdown && len(candidates) == 0 {
						candidates = assetBasenames[bk]
					}

					if (!escaping || lo.linkType == LinkTypeMarkdown) && len(candidates) >= 2 {
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

					// Markdown names the chosen path; wikilinks retain basename repair.
					targetPath := bn + ".md"
					if lo.linkType == LinkTypeMarkdown && filepath.Ext(bn) != "" {
						targetPath = bn
					}
					if lo.linkType == LinkTypeMarkdown && len(candidates) == 1 {
						targetPath = candidates[0]
					}
					newRawLink := rewriteRawLink(lo.rawLink, lo.linkType, targetPath, lo.inTable)
					if newRawLink == lo.rawLink {
						continue
					}

					entries = append(entries, rewriteEntry{
						rawLink:    lo.rawLink,
						linkType:   lo.linkType,
						lineStart:  lo.lineStart,
						sourcePath: sourcePath,
						newRawLink: newRawLink,
					})
				}
				return entries, nil
			}}, nil
		},
	}, locations...)
	if err != nil {
		return nil, err
	}

	// Build rewritten result entries.
	for _, re := range rewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
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
func linkTargetExistsRaw(diskPaths *vaultDiskPathResolver, sourcePath string, lo linkOccur) bool {
	resolved := resolveToVaultRelative(sourcePath, lo)

	if _, err := diskPaths.existingPath(resolved); err == nil {
		return true
	}
	if !strings.HasSuffix(strings.ToLower(resolved), ".md") {
		if _, err := diskPaths.existingPath(resolved + ".md"); err == nil {
			return true
		}
	}
	return false
}

// isBodyPathLinkType reports whether linkType is a body link whose target
// resolves to a vault path. Repair only rewrites body links; other rewrite
// operations may include frontmatter wikilinks via isPathLinkType.
func isBodyPathLinkType(linkType LinkType) bool {
	switch linkType {
	case LinkTypeWikilink, LinkTypeMarkdown:
		return true
	}
	return false
}
