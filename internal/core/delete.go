package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// DeleteOptions controls which files to remove from the index.
type DeleteOptions struct {
	Files []string // vault-relative paths
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

	// Normalize, deduplicate input paths, and collect node info for validation.
	type nodeInfo struct {
		id   int64
		name string
		path string // normalized vault-relative path
	}
	seen := make(map[string]bool)
	var nodes []nodeInfo
	for _, f := range opts.Files {
		np := normalizePath(f)
		if seen[np] {
			continue
		}
		seen[np] = true
		key := noteKey(np)
		var id int64
		var name, path string
		err := db.QueryRow("SELECT id, name, path FROM nodes WHERE node_key = ? AND type = 'note'", key).Scan(&id, &name, &path)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not registered: %s", f)
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, nodeInfo{id: id, name: name, path: np})
	}

	// Check that files no longer exist on disk.
	for _, n := range nodes {
		diskPath := filepath.Join(vaultPath, n.path)
		if _, err := os.Stat(diskPath); err == nil {
			return nil, fmt.Errorf("file still exists on disk: %s (delete the file first, then run delete)", n.path)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

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
