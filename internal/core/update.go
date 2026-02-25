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
		np := NormalizePath(f)
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
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Adjust maps to reflect post-update vault state.
	for _, cf := range classified {
		if !cf.existsOnDisk {
			// Only adjust maps if the file was present in them.
			if _, ok := rm.pathToID[cf.path]; ok {
				delete(rm.pathToID, cf.path)
				rel := strings.ToLower(cf.path)
				delete(rm.pathSet, rel)
				noExt := strings.TrimSuffix(cf.path, filepath.Ext(cf.path))
				delete(rm.pathSet, strings.ToLower(noExt))
				bk := basenameKey(cf.path)
				rm.basenameCounts[bk]--
				if rm.basenameCounts[bk] <= 0 {
					delete(rm.basenameCounts, bk)
				}
				if isRootFile(cf.path) {
					delete(rm.rootBasenameToPath, bk)
				}
			}
		} else {
			// Ensure present in maps (normally already there for registered notes).
			if _, ok := rm.pathToID[cf.path]; !ok {
				rm.pathToID[cf.path] = cf.id
				rel := strings.ToLower(cf.path)
				rm.pathSet[rel] = cf.path
				noExt := strings.TrimSuffix(cf.path, filepath.Ext(cf.path))
				rm.pathSet[strings.ToLower(noExt)] = cf.path
				bk := basenameKey(cf.path)
				rm.basenameCounts[bk]++
				if isRootFile(cf.path) {
					rm.rootBasenameToPath[bk] = cf.path
				}
			}
		}
	}

	// Rebuild basenameToPath from adjusted basenameCounts.
	rm.basenameToPath = make(map[string]string)
	for bk, count := range rm.basenameCounts {
		if count == 1 {
			// Find the path with this basename key.
			for p := range rm.pathToID {
				if basenameKey(p) == bk {
					rm.basenameToPath[bk] = p
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

		// Check for ambiguous links and vault escape (same logic as build's inline validation).
		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(cf.path, link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, cf.path)
			}
			if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, cf.path)
			}
			if link.isBasename && isAmbiguousBasenameLink(link.target, rm) {
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
			targetID, subpath, err := resolveLink(tx, pf.cf.path, link, rm)
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

		phantomized, err := removeOrPhantomize(tx, cf.id, cf.name)
		if err != nil {
			return nil, err
		}
		if phantomized {
			result.Phantomed = append(result.Phantomed, cf.path)
		} else {
			result.Deleted = append(result.Deleted, cf.path)
		}
	}

	// Orphan cleanup: remove tags/phantoms not referenced by any edge.
	if err := cleanupOrphanedNodes(tx); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

// buildMapsFromDB constructs in-memory resolveMaps from existing DB nodes,
// mirroring build's Pass 1 structure.
func buildMapsFromDB(db dbExecer) (*resolveMaps, error) {
	rm := &resolveMaps{
		pathToID:                make(map[string]int64),
		pathSet:                 make(map[string]string),
		basenameCounts:          make(map[string]int),
		basenameToPath:          make(map[string]string),
		rootBasenameToPath:      make(map[string]string),
		assetPathToID:           make(map[string]int64),
		assetPathSet:            make(map[string]string),
		assetBasenameCounts:     make(map[string]int),
		assetBasenameToPath:     make(map[string]string),
		assetRootBasenameToPath: make(map[string]string),
	}

	// Load notes.
	rows, err := db.Query("SELECT id, path FROM nodes WHERE type='note' AND exists_flag=1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		rm.pathToID[path] = id

		rel := strings.ToLower(path)
		rm.pathSet[rel] = path
		noExt := strings.TrimSuffix(path, filepath.Ext(path))
		rm.pathSet[strings.ToLower(noExt)] = path

		bk := basenameKey(path)
		rm.basenameCounts[bk]++
		if isRootFile(path) {
			rm.rootBasenameToPath[bk] = path
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load assets.
	arows, err := db.Query("SELECT id, path FROM nodes WHERE type='asset' AND exists_flag=1")
	if err != nil {
		return nil, err
	}
	defer arows.Close()

	for arows.Next() {
		var id int64
		var path string
		if err := arows.Scan(&id, &path); err != nil {
			return nil, err
		}
		rm.assetPathToID[path] = id
		rm.assetPathSet[strings.ToLower(path)] = path

		abk := assetBasenameKey(path)
		rm.assetBasenameCounts[abk]++
		if isRootFile(path) {
			rm.assetRootBasenameToPath[abk] = path
		}
	}
	if err := arows.Err(); err != nil {
		return nil, err
	}

	// Build basenameToPath for notes (unique only).
	for p := range rm.pathToID {
		bk := basenameKey(p)
		if rm.basenameCounts[bk] == 1 {
			rm.basenameToPath[bk] = p
		}
	}

	// Build assetBasenameToPath for assets (unique only).
	for p := range rm.assetPathToID {
		abk := assetBasenameKey(p)
		if rm.assetBasenameCounts[abk] == 1 {
			rm.assetBasenameToPath[abk] = p
		}
	}

	return rm, nil
}
