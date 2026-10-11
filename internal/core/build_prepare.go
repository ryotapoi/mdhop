package core

import (
	"os"
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
func prepareBuild(vaultPath string, locations ...Locations) (*preparedBuild, error) {
	return prepareBuildWithout(vaultPath, "", locations...)
}

// prepareBuildWithout omits the legacy config that migration will remove.
func prepareBuildWithout(vaultPath, omittedAsset string, locations ...Locations) (*preparedBuild, error) {
	// Pass 0: collect .md files.
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}
	indexFiles := newIndexFileMatcher(vaultPath, locations)
	files, err = indexFiles.filter(files)
	if err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	// Pass 0.5: collect asset files.
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles, err = indexFiles.filter(assetFiles)
	if err != nil {
		return nil, err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)
	if omittedAsset != "" {
		kept := assetFiles[:0]
		for _, path := range assetFiles {
			if path != omittedAsset {
				kept = append(kept, path)
			}
		}
		assetFiles = kept
	}

	// Build resolve maps for notes and assets.
	rm := newResolveMaps(files, assetFiles)
	diskPaths := newVaultDiskPathResolver(vaultPath)

	// Read all files, parse links, stat for mtime, and validate.
	parsed := make([]preparedNote, 0, len(files))
	var userErrors []error
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

		// Collect link validation errors up to maxBuildErrors.
		for _, link := range pr.Links {
			if err := validateParsedLink(rel, link, rm); err != nil {
				userErrors = append(userErrors, err)
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
