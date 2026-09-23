package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var deleteAssetWalk = filepath.Walk
var deleteAssetRemove = os.Remove
var deleteEmptyDirs = CleanupEmptyDirs

// DeleteOptions controls which files to remove from the index.
type DeleteOptions struct {
	Files       []string // vault-relative file or directory paths
	RemoveFiles bool     // if true, delete files from disk before updating DB
}

// DeleteResult reports which nodes were deleted or converted to phantom.
type DeleteResult struct {
	Deleted   []string // completely removed nodes
	Phantomed []string // converted to phantom
}

// Delete removes registered files from the index and can expand directory paths.
// Files with incoming references are converted to phantom nodes; other files are removed.
func Delete(vaultPath string, opts DeleteOptions) (*DeleteResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	files, directories, err := expandDeletePaths(vaultPath, db, opts.Files)
	if err != nil {
		return nil, err
	}

	// Phase 1: Normalize, deduplicate input paths, and collect node info for validation.
	type nodeInfo struct {
		id      int64
		name    string
		path    string // normalized vault-relative path
		isAsset bool
	}
	seen := make(map[string]bool)
	var nodes []nodeInfo
	for _, f := range files {
		np := NormalizePath(f)
		if seen[np] {
			continue
		}
		seen[np] = true
		// Try note first, then asset.
		if id, ok := rm.pathToID[np]; ok {
			nodes = append(nodes, nodeInfo{id: id, name: basename(np), path: np})
			continue
		}
		if id, ok := rm.assetPathToID[np]; ok {
			nodes = append(nodes, nodeInfo{id: id, name: filepath.Base(np), path: np, isAsset: true})
			continue
		}
		return nil, fmt.Errorf("%w: %s", ErrFileNotRegistered, f)
	}

	// Phase 2: disk operations.
	if opts.RemoveFiles {
		// Vault containment check + os.Remove for each file.
		vaultAbs, err := filepath.Abs(vaultPath)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			targetAbs, err := filepath.Abs(filepath.Join(vaultPath, n.path))
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(vaultAbs, targetAbs)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			if rel == ".." || strings.HasPrefix(rel, "../") {
				return nil, fmt.Errorf("path escapes vault: %s", n.path)
			}
			if err := os.Remove(targetAbs); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
	} else {
		// Check that files no longer exist on disk.
		for _, n := range nodes {
			diskPath := filepath.Join(vaultPath, n.path)
			if _, err := os.Stat(diskPath); err == nil {
				return nil, fmt.Errorf("file still exists on disk: %s (delete the file first, then run delete)", n.path)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}

	// Phase 3: DB transaction.
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &DeleteResult{}

	for _, n := range nodes {
		phantomized, err := removeOrPhantomize(tx, n.id, n.name)
		if err != nil {
			return nil, err
		}
		if phantomized {
			result.Phantomed = append(result.Phantomed, n.path)
		} else {
			result.Deleted = append(result.Deleted, n.path)
		}
	}

	if err := cleanupOrphanedNodes(tx); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if opts.RemoveFiles && len(directories) > 0 {
		if err := cleanupDeletedDirectories(vaultPath, directories, result); err != nil {
			return nil, fmt.Errorf("post-delete cleanup failed after registered files and database updates completed: %w", err)
		}
	}

	return result, nil
}

func expandDeletePaths(vaultPath string, db dbExecer, inputs []string) (files, directories []string, err error) {
	for _, input := range inputs {
		if !isDeleteDirectoryArg(vaultPath, input) {
			files = append(files, input)
			continue
		}

		dirPrefix := NormalizePath(strings.TrimSuffix(input, "/"))
		notes, err := listDirNodesByType(db, dirPrefix, NodeTypeNote)
		if err != nil {
			return nil, nil, err
		}
		assets, err := listDirNodesByType(db, dirPrefix, NodeTypeAsset)
		if err != nil {
			return nil, nil, err
		}
		if len(notes) == 0 && len(assets) == 0 {
			return nil, nil, fmt.Errorf("no files registered under directory: %s", input)
		}
		files = append(files, notes...)
		files = append(files, assets...)
		directories = append(directories, dirPrefix)
	}
	return files, directories, nil
}

func isDeleteDirectoryArg(vaultPath, path string) bool {
	if strings.HasSuffix(path, "/") {
		return true
	}
	info, err := os.Stat(filepath.Join(vaultPath, path))
	return err == nil && info.IsDir()
}

func cleanupDeletedDirectories(vaultPath string, directories []string, result *DeleteResult) error {
	for _, dir := range directories {
		absDir := filepath.Join(vaultPath, dir)
		if err := deleteAssetWalk(absDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return fmt.Errorf("walk %s: %w", path, walkErr)
			}
			if info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			if err := deleteAssetRemove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", path, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	allPaths := append(append([]string(nil), result.Deleted...), result.Phantomed...)
	return deleteEmptyDirs(vaultPath, allPaths)
}
