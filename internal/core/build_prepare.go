package core

import (
	"os"
	"path/filepath"
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
func prepareBuild(vaultPath string, locations ...Locations) (*preparedBuild, error) {
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
	files, err = filterIndexFiles(vaultPath, files, locations)
	if err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	// Pass 0.5: collect asset files.
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles, err = filterIndexFiles(vaultPath, assetFiles, locations)
	if err != nil {
		return nil, err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)

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

// Exclude only the selected index and its directly associated SQLite/build files.
func filterIndexFiles(vaultPath string, files []string, locations []Locations) ([]string, error) {
	result := make([]string, 0, len(files))
	for _, file := range files {
		excluded, err := isIndexFile(vaultPath, filepath.Join(vaultPath, file), locations)
		if err != nil {
			return nil, err
		}
		if excluded {
			continue
		}
		result = append(result, file)
	}
	return result, nil
}

// isIndexFile identifies only the selected DB and its direct auxiliary files.
// path is a filesystem path, rather than a vault-relative note identifier.
func isIndexFile(vaultPath, path string, locations []Locations) (bool, error) {
	dbp, err := resolveDBPath(vaultPath, locations...)
	if err != nil {
		return false, err
	}
	index := indexResourcePath(dbp)
	absolute := indexResourcePath(path)
	return absolute == index || absolute == index+"-journal" || absolute == index+"-wal" || absolute == index+"-shm" || strings.HasPrefix(absolute, index+".tmp-"), nil
}

// Resolve parent aliases (including a symlink vault root) without following the
// final entry, which build replaces and directory mutations must preserve.
func indexResourcePath(path string) string {
	absolute, _ := filepath.Abs(path)
	if parent, err := filepath.EvalSymlinks(filepath.Dir(absolute)); err == nil {
		absolute = filepath.Join(parent, filepath.Base(absolute))
	}
	return NormalizePath(absolute)
}
