package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateOptions controls which files to re-parse and update in the index.
type UpdateOptions struct {
	Files []string // vault-relative paths
}

// UpdateResult reports the outcome for each processed file.
type UpdateResult struct {
	Updated   []string // files whose content was re-parsed
	Deleted   []string // files completely removed (disk-absent, no references)
	Phantomed []string // files converted to phantom (disk-absent, has references)
}

// Update re-parses the specified files and updates the existing index DB in-place.
// Files that no longer exist on disk are treated like delete (phantom conversion
// or complete removal depending on incoming references).
func Update(vaultPath string, opts UpdateOptions) (*UpdateResult, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Normalize and deduplicate input paths, collect node info for validation.
	type fileInfo struct {
		id   int64
		name string
		path string // normalized vault-relative path
	}
	seen := make(map[string]bool)
	var files []fileInfo
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
		files = append(files, fileInfo{id: id, name: name, path: np})
	}

	// Check disk existence for each file.
	type classifiedFile struct {
		fileInfo
		existsOnDisk bool
		diskMtime    int64
	}
	var classified []classifiedFile
	for _, fi := range files {
		info, err := os.Stat(filepath.Join(vaultPath, fi.path))
		if os.IsNotExist(err) {
			classified = append(classified, classifiedFile{fileInfo: fi, existsOnDisk: false})
		} else if err != nil {
			return nil, err
		} else {
			classified = append(classified, classifiedFile{
				fileInfo:     fi,
				existsOnDisk: true,
				diskMtime:    info.ModTime().Unix(),
			})
		}
	}

	// Build in-memory maps from DB (mirrors build's Pass 1).
	pathToID, pathSet, basenameCounts, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Adjust maps to reflect post-update vault state.
	for _, cf := range classified {
		if !cf.existsOnDisk {
			// Only adjust maps if the file was present in them.
			if _, ok := pathToID[cf.path]; ok {
				delete(pathToID, cf.path)
				rel := strings.ToLower(cf.path)
				delete(pathSet, rel)
				noExt := strings.TrimSuffix(cf.path, filepath.Ext(cf.path))
				delete(pathSet, strings.ToLower(noExt))
				bk := basenameKey(cf.path)
				basenameCounts[bk]--
				if basenameCounts[bk] <= 0 {
					delete(basenameCounts, bk)
				}
			}
		} else {
			// Ensure present in maps (normally already there for registered notes).
			if _, ok := pathToID[cf.path]; !ok {
				pathToID[cf.path] = cf.id
				rel := strings.ToLower(cf.path)
				pathSet[rel] = cf.path
				noExt := strings.TrimSuffix(cf.path, filepath.Ext(cf.path))
				pathSet[strings.ToLower(noExt)] = cf.path
				bk := basenameKey(cf.path)
				basenameCounts[bk]++
			}
		}
	}

	// Rebuild basenameToPath from adjusted basenameCounts.
	basenameToPath := make(map[string]string)
	for bk, count := range basenameCounts {
		if count == 1 {
			// Find the path with this basename key.
			for p := range pathToID {
				if basenameKey(p) == bk {
					basenameToPath[bk] = p
					break
				}
			}
		}
	}

	// Pre-mutation: read and validate disk-present files.
	type parsedFile struct {
		cf    classifiedFile
		links []linkOccur
	}
	var toUpdate []parsedFile
	for _, cf := range classified {
		if !cf.existsOnDisk {
			continue
		}
		content, err := os.ReadFile(filepath.Join(vaultPath, cf.path))
		if err != nil {
			return nil, err
		}
		links := parseLinks(string(content))

		// Check for ambiguous links and vault escape (same logic as build's detectAmbiguousLinks).
		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(cf.path, link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, cf.path)
			}
			if link.isBasename && basenameCounts[strings.ToLower(link.target)] > 1 {
				return nil, fmt.Errorf("ambiguous link: %s in %s", link.target, cf.path)
			}
		}

		toUpdate = append(toUpdate, parsedFile{cf: cf, links: links})
	}

	// Begin transaction.
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &UpdateResult{}

	// Phase A: update disk-present files.
	for _, pf := range toUpdate {
		// Delete all outgoing edges.
		if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", pf.cf.id); err != nil {
			return nil, err
		}

		// Update mtime and exists_flag.
		if _, err := tx.Exec("UPDATE nodes SET exists_flag=1, mtime=? WHERE id=?", pf.cf.diskMtime, pf.cf.id); err != nil {
			return nil, err
		}

		// Re-resolve links and create new edges.
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.cf.path, link, pathSet, basenameToPath, pathToID)
			if err != nil {
				return nil, err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, pf.cf.id, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return nil, err
			}
		}

		result.Updated = append(result.Updated, pf.cf.path)
	}

	// Phase B: handle disk-absent files (same logic as delete).
	for _, cf := range classified {
		if cf.existsOnDisk {
			continue
		}

		// Check incoming edges (excluding self-links).
		var incomingCount int
		if err := tx.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ? AND source_id != ?", cf.id, cf.id).Scan(&incomingCount); err != nil {
			return nil, err
		}

		if incomingCount > 0 {
			// Phantom conversion: has incoming references.
			// Delete all outgoing edges.
			if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", cf.id); err != nil {
				return nil, err
			}

			// Check if a phantom with the same name already exists.
			phantomKey := fmt.Sprintf("phantom:name:%s", strings.ToLower(cf.name))
			var existingPhantomID int64
			err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&existingPhantomID)
			if err == nil {
				// Existing phantom found: reassign incoming edges and delete the note node.
				if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", existingPhantomID, cf.id); err != nil {
					return nil, err
				}
				if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", cf.id); err != nil {
					return nil, err
				}
			} else if err == sql.ErrNoRows {
				// No existing phantom: convert note to phantom in-place.
				if _, err := tx.Exec(
					"UPDATE nodes SET type='phantom', node_key=?, path=NULL, exists_flag=0, mtime=NULL WHERE id=?",
					phantomKey, cf.id,
				); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
			result.Phantomed = append(result.Phantomed, cf.path)
		} else {
			// Complete deletion: no incoming references.
			if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ? OR target_id = ?", cf.id, cf.id); err != nil {
				return nil, err
			}
			if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", cf.id); err != nil {
				return nil, err
			}
			result.Deleted = append(result.Deleted, cf.path)
		}
	}

	// Orphan cleanup: remove tags/phantoms not referenced by any edge.
	if _, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom') AND id NOT IN (SELECT DISTINCT target_id FROM edges)"); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

// buildMapsFromDB constructs in-memory maps from existing DB note nodes,
// mirroring build's Pass 1 structure.
func buildMapsFromDB(db *sql.DB) (pathToID map[string]int64, pathSet map[string]string, basenameCounts map[string]int, err error) {
	pathToID = make(map[string]int64)
	pathSet = make(map[string]string)
	basenameCounts = make(map[string]int)

	rows, err := db.Query("SELECT id, path FROM nodes WHERE type='note' AND exists_flag=1")
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, nil, nil, err
		}
		pathToID[path] = id

		rel := strings.ToLower(path)
		pathSet[rel] = path
		noExt := strings.TrimSuffix(path, filepath.Ext(path))
		pathSet[strings.ToLower(noExt)] = path

		bk := basenameKey(path)
		basenameCounts[bk]++
	}

	return pathToID, pathSet, basenameCounts, rows.Err()
}
