package core

import (
	"os"
	"sort"
)

// StatusResult lists differences between the current disk contents and the
// indexed note and asset records.
type StatusResult struct {
	Untracked []string
	Modified  []string
	Deleted   []string
}

// Status compares the current build inputs with the indexed note and asset
// records. It does not update the index or the vault.
func Status(vaultPath string, locations ...Locations) (*StatusResult, error) {
	db, err := openDBChecked(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cfg, err := LoadConfig(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT path, mtime FROM nodes WHERE type IN ('note', 'asset')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexed := make(map[string]int64)
	for rows.Next() {
		var path string
		var mtime int64
		if err := rows.Scan(&path, &mtime); err != nil {
			return nil, err
		}
		indexed[NormalizePath(path)] = mtime
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	noteFiles, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	noteFiles = filterIndexFiles(vaultPath, noteFiles, locations)
	assetFiles = filterIndexFiles(vaultPath, assetFiles, locations)
	allDiskFiles := append(noteFiles, assetFiles...)
	diskFiles := append(filterBuildExcludes(noteFiles, cfg.Build.ExcludePaths), filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)...)
	diskPathSet := make(map[string]bool, len(allDiskFiles))
	for _, path := range allDiskFiles {
		diskPathSet[path] = true
	}

	result := &StatusResult{
		Untracked: make([]string, 0),
		Modified:  make([]string, 0),
		Deleted:   make([]string, 0),
	}
	for _, path := range diskFiles {
		if _, ok := indexed[path]; !ok {
			result.Untracked = append(result.Untracked, path)
		}
	}

	diskPaths := newVaultDiskPathResolver(vaultPath)
	for path, indexedMtime := range indexed {
		// Use the spelling returned by the walk to distinguish a case-only
		// rename on case-insensitive filesystems. existingPath below still
		// resolves NFC-equivalent disk paths.
		if !diskPathSet[path] {
			result.Deleted = append(result.Deleted, path)
			continue
		}
		fullPath, err := diskPaths.existingPath(path)
		if os.IsNotExist(err) {
			result.Deleted = append(result.Deleted, path)
			continue
		}
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			result.Deleted = append(result.Deleted, path)
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			result.Deleted = append(result.Deleted, path)
			continue
		}
		if info.ModTime().Unix() != indexedMtime {
			result.Modified = append(result.Modified, path)
		}
	}

	sort.Strings(result.Untracked)
	sort.Strings(result.Modified)
	sort.Strings(result.Deleted)
	return result, nil
}
