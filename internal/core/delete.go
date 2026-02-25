package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeleteOptions controls which files to remove from the index.
type DeleteOptions struct {
	Files       []string // vault-relative paths
	RemoveFiles bool     // if true, delete files from disk before updating DB
}

// DeleteResult reports which nodes were deleted or converted to phantom.
type DeleteResult struct {
	Deleted   []string // completely removed nodes
	Phantomed []string // converted to phantom
}

// Delete removes the specified files from the index DB.
// Files with incoming references are converted to phantom nodes.
// Files without incoming references are completely removed.
func Delete(vaultPath string, opts DeleteOptions) (*DeleteResult, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Phase 1: Normalize, deduplicate input paths, and collect node info for validation.
	type nodeInfo struct {
		id      int64
		name    string
		path    string // normalized vault-relative path
		isAsset bool
	}
	seen := make(map[string]bool)
	var nodes []nodeInfo
	for _, f := range opts.Files {
		np := NormalizePath(f)
		if seen[np] {
			continue
		}
		seen[np] = true
		// Try note first, then asset.
		key := noteKey(np)
		var id int64
		var name, path string
		err := db.QueryRow("SELECT id, name, path FROM nodes WHERE node_key = ? AND type = 'note'", key).Scan(&id, &name, &path)
		if err == sql.ErrNoRows {
			// Fallback to asset.
			key = assetKey(np)
			err = db.QueryRow("SELECT id, name, path FROM nodes WHERE node_key = ? AND type = 'asset'", key).Scan(&id, &name, &path)
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("file not registered: %s", f)
			}
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, nodeInfo{id: id, name: name, path: np, isAsset: true})
			continue
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, nodeInfo{id: id, name: name, path: np})
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

	return result, nil
}
