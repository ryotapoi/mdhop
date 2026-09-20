package core

import (
	"fmt"
	"os"
	"strings"
)

type preparedBuild struct {
	notes       []preparedNote
	assets      []preparedAsset
	resolveMaps *resolveMaps
	metaTypes   map[string]MetaTypeInfo
}

type preparedNote struct {
	path  string
	mtime int64
	lines int
	links []linkOccur
	meta  []FrontmatterEntry
}

type preparedAsset struct {
	path  string
	mtime int64
}

// prepareBuild collects and validates all Build inputs before database work begins.
func prepareBuild(vaultPath string) (*preparedBuild, error) {
	// Pass 0: collect .md files.
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

	// Pass 0.5: collect asset files.
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)

	// Build resolve maps for notes and assets.
	rm := newResolveMaps(files, assetFiles)
	diskPaths := newVaultDiskPathResolver(vaultPath)

	// Read all files, parse links, stat for mtime, and validate.
	parsed := make([]preparedNote, 0, len(files))
	var userErrors []string
	for _, rel := range files {
		fullPath, err := diskPaths.existingPath(rel)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		pr := parseLinksWithLinkKeys(string(content), cfg.Meta.LinkKeys)

		// Validate links: collect user errors (ambiguous, vault-escape) up to maxBuildErrors.
		for _, link := range pr.Links {
			if !isPathLinkType(link.linkType) {
				continue
			}
			if link.isRelative && escapesVault(rel, link.target) {
				userErrors = append(userErrors, fmt.Sprintf("link escapes vault: %s in %s", link.rawLink, rel))
			} else if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
				userErrors = append(userErrors, fmt.Sprintf("link escapes vault: %s in %s", link.rawLink, rel))
			} else if link.isBasename && isAmbiguousBasenameLink(link.target, rm) {
				candidates := ambiguousCandidates(link.target, rm)
				userErrors = append(userErrors, fmt.Sprintf("ambiguous link: %s in %s (candidates: %s)", link.target, rel, strings.Join(candidates, ", ")))
			} else {
				continue
			}
			if len(userErrors) >= maxBuildErrors {
				break
			}
		}
		if len(userErrors) >= maxBuildErrors {
			break
		}

		parsed = append(parsed, preparedNote{
			path:  rel,
			mtime: info.ModTime().Unix(),
			lines: countLines(string(content)),
			links: pr.Links,
			meta:  pr.Meta,
		})
	}
	if len(userErrors) > 0 {
		return nil, formatBuildErrors(userErrors)
	}

	// Stat asset files for mtime.
	assetInfos := make([]preparedAsset, 0, len(assetFiles))
	for _, rel := range assetFiles {
		fullPath, err := diskPaths.existingPath(rel)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		assetInfos = append(assetInfos, preparedAsset{path: rel, mtime: info.ModTime().Unix()})
	}

	return &preparedBuild{
		notes:       parsed,
		assets:      assetInfos,
		resolveMaps: rm,
		metaTypes:   cfg.Meta.Types,
	}, nil
}
