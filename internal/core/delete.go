package core

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
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

	// Normalize input paths and collect node info for validation.
	type nodeInfo struct {
		id   int64
		name string
		path string // normalized vault-relative path
	}
	nodes := make([]nodeInfo, len(opts.Files))
	for i, f := range opts.Files {
		np := normalizePath(f)
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
		nodes[i] = nodeInfo{id: id, name: name, path: np}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &DeleteResult{}

	for _, n := range nodes {
		// Check incoming edges (excluding self-links).
		var incomingCount int
		if err := tx.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ? AND source_id != ?", n.id, n.id).Scan(&incomingCount); err != nil {
			return nil, err
		}

		if incomingCount > 0 {
			// Phantom conversion: has incoming references.
			// Delete all outgoing edges.
			if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", n.id); err != nil {
				return nil, err
			}

			// Check if a phantom with the same name already exists.
			phantomKey := fmt.Sprintf("phantom:name:%s", strings.ToLower(n.name))
			var existingPhantomID int64
			err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&existingPhantomID)
			if err == nil {
				// Existing phantom found: reassign incoming edges and delete the note node.
				if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", existingPhantomID, n.id); err != nil {
					return nil, err
				}
				if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", n.id); err != nil {
					return nil, err
				}
			} else if err == sql.ErrNoRows {
				// No existing phantom: convert note to phantom in-place.
				if _, err := tx.Exec(
					"UPDATE nodes SET type='phantom', node_key=?, path=NULL, exists_flag=0, mtime=NULL WHERE id=?",
					phantomKey, n.id,
				); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
			result.Phantomed = append(result.Phantomed, n.path)
		} else {
			// Complete deletion: no incoming references.
			if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ? OR target_id = ?", n.id, n.id); err != nil {
				return nil, err
			}
			if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", n.id); err != nil {
				return nil, err
			}
			result.Deleted = append(result.Deleted, n.path)
		}
	}

	// Clean up orphaned tags and phantoms (not referenced by any edge).
	if _, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom') AND id NOT IN (SELECT DISTINCT target_id FROM edges)"); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}
