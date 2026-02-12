package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddOptions controls which files to add to the index.
type AddOptions struct {
	Files            []string
	AutoDisambiguate bool
}

// AddResult reports the outcome of the add operation.
type AddResult struct {
	Added    []string // files added as new notes
	Promoted []string // phantom nodes promoted to notes
}

// Add inserts new files into the existing index DB.
func Add(vaultPath string, opts AddOptions) (*AddResult, error) {
	// Step 1: DB existence check.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	// Step 2: auto-disambiguate check.
	if opts.AutoDisambiguate {
		return nil, fmt.Errorf("--auto-disambiguate is not yet implemented")
	}

	// Step 3: normalize and deduplicate input paths.
	type addFile struct {
		path  string
		mtime int64
	}
	seen := make(map[string]bool)
	var files []addFile
	for _, f := range opts.Files {
		np := normalizePath(f)
		if seen[np] {
			continue
		}
		seen[np] = true
		files = append(files, addFile{path: np})
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Step 4: check that no file is already registered.
	for _, f := range files {
		key := noteKey(f.path)
		var id int64
		err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ? AND type = 'note'", key).Scan(&id)
		if err == nil {
			return nil, fmt.Errorf("file already registered: %s", f.path)
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	// Step 5: check disk existence and collect mtime.
	for i := range files {
		info, err := os.Stat(filepath.Join(vaultPath, files[i].path))
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", files[i].path)
		}
		if err != nil {
			return nil, err
		}
		files[i].mtime = info.ModTime().Unix()
	}

	// Step 6: build maps from DB + save oldBasenameCounts copy.
	pathToID, pathSet, basenameCounts, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}
	oldBasenameCounts := make(map[string]int, len(basenameCounts))
	for k, v := range basenameCounts {
		oldBasenameCounts[k] = v
	}

	// Step 7: adjust maps for post-add state.
	for _, f := range files {
		rel := strings.ToLower(f.path)
		pathSet[rel] = f.path
		noExt := strings.TrimSuffix(f.path, filepath.Ext(f.path))
		pathSet[strings.ToLower(noExt)] = f.path
		bk := basenameKey(f.path)
		basenameCounts[bk]++
	}

	// Step 8: check if adding causes existing links to become ambiguous.
	// Build oldBasenameToPath for pattern A detection.
	oldBasenameToPath := make(map[string]string)
	for bk, count := range oldBasenameCounts {
		if count == 1 {
			for p := range pathToID {
				if basenameKey(p) == bk {
					oldBasenameToPath[bk] = p
					break
				}
			}
		}
	}

	for bk, newCount := range basenameCounts {
		if newCount <= 1 {
			continue
		}
		oldCount := oldBasenameCounts[bk]
		if oldCount >= 2 {
			continue // already ambiguous before add
		}

		// Find the target node that existing basename links pointed to.
		var targetID int64
		if oldCount == 1 {
			// Pattern A: existing unique note becomes ambiguous.
			targetID = pathToID[oldBasenameToPath[bk]]
		} else {
			// Pattern B: phantom (oldCount == 0, adding 2+ files with same basename).
			phantomKey := fmt.Sprintf("phantom:name:%s", bk)
			err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&targetID)
			if err == sql.ErrNoRows {
				continue // no phantom, so no existing basename links
			}
			if err != nil {
				return nil, err
			}
		}

		// Check if any existing edges to this target are basename links.
		rows, err := db.Query(
			`SELECT raw_link, link_type FROM edges
			 WHERE target_id = ?
			 AND link_type IN ('wikilink', 'markdown')`, targetID)
		if err != nil {
			return nil, err
		}
		var ambiguousFound bool
		for rows.Next() {
			var rawLink, linkType string
			if err := rows.Scan(&rawLink, &linkType); err != nil {
				rows.Close()
				return nil, err
			}
			if isBasenameRawLink(rawLink, linkType) {
				ambiguousFound = true
				break
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if ambiguousFound {
			return nil, fmt.Errorf("adding files would make existing links ambiguous")
		}
	}

	// Step 9: build basenameToPath (includes both existing and new files).
	basenameToPath := make(map[string]string)
	for bk, count := range basenameCounts {
		if count != 1 {
			continue
		}
		// Search existing notes.
		for p := range pathToID {
			if basenameKey(p) == bk {
				basenameToPath[bk] = p
				break
			}
		}
		// Search new files if not found.
		if _, ok := basenameToPath[bk]; !ok {
			for _, f := range files {
				if basenameKey(f.path) == bk {
					basenameToPath[bk] = f.path
					break
				}
			}
		}
	}

	// Step 10: parse all new files and check for ambiguous links.
	type parsedFile struct {
		file  addFile
		links []linkOccur
	}
	var parsed []parsedFile
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(vaultPath, f.path))
		if err != nil {
			return nil, err
		}
		links := parseLinks(string(content))

		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(f.path, link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, f.path)
			}
			if link.isBasename && basenameCounts[strings.ToLower(link.target)] > 1 {
				return nil, fmt.Errorf("ambiguous link: %s in %s", link.target, f.path)
			}
		}

		parsed = append(parsed, parsedFile{file: f, links: links})
	}

	// Step 11: begin transaction.
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &AddResult{}

	// Step 12: insert all note nodes.
	for _, pf := range parsed {
		name := basename(pf.file.path)
		id, err := upsertNote(tx, pf.file.path, name, pf.file.mtime)
		if err != nil {
			return nil, err
		}
		pathToID[pf.file.path] = id
		result.Added = append(result.Added, pf.file.path)
	}

	// Step 13: phantom → note promotion.
	for _, pf := range parsed {
		bk := basenameKey(pf.file.path)
		phantomKey := fmt.Sprintf("phantom:name:%s", bk)
		var phantomID int64
		err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&phantomID)
		if err == sql.ErrNoRows {
			continue // no phantom to promote
		}
		if err != nil {
			return nil, err
		}

		noteID := pathToID[pf.file.path]

		// Reassign incoming edges from phantom to note.
		if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", noteID, phantomID); err != nil {
			return nil, err
		}

		// Delete phantom node.
		if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
			return nil, err
		}

		result.Promoted = append(result.Promoted, pf.file.path)
	}

	// Step 14: resolve links and create edges.
	for _, pf := range parsed {
		sourceID := pathToID[pf.file.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.file.path, link, pathSet, basenameToPath, pathToID)
			if err != nil {
				return nil, err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, sourceID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return nil, err
			}
		}
	}

	// Step 15: orphan cleanup.
	if _, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom') AND id NOT IN (SELECT DISTINCT target_id FROM edges)"); err != nil {
		return nil, err
	}

	// Step 16: commit.
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

// isBasenameRawLink checks if a raw_link represents a basename link (no path separators).
func isBasenameRawLink(rawLink, linkType string) bool {
	switch linkType {
	case "wikilink":
		// raw_link is like "[[Target]]" or "[[Target|alias]]" or "[[Target#heading]]"
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")
		// Remove alias part.
		if idx := strings.Index(inner, "|"); idx >= 0 {
			inner = inner[:idx]
		}
		// Remove subpath (heading).
		if idx := strings.Index(inner, "#"); idx >= 0 {
			inner = inner[:idx]
		}
		// Empty target means self-link like [[#Heading]], not a basename link.
		if inner == "" {
			return false
		}
		return !strings.Contains(inner, "/")
	case "markdown":
		// raw_link is like "[text](url)" or "[text](url#heading)"
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return false
		}
		url := rawLink[start+2:]
		url = strings.TrimSuffix(url, ")")
		// Remove fragment.
		if idx := strings.Index(url, "#"); idx >= 0 {
			url = url[:idx]
		}
		// Empty url means self-link like [text](#heading), not a basename link.
		if url == "" {
			return false
		}
		return !strings.Contains(url, "/")
	}
	return false
}
