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

// RewrittenLink records a single link rewrite performed by auto-disambiguate.
type RewrittenLink struct {
	File    string
	OldLink string
	NewLink string
}

// AddResult reports the outcome of the add operation.
type AddResult struct {
	Added     []string        // files added as new notes
	Promoted  []string        // phantom nodes promoted to notes
	Rewritten []RewrittenLink // links rewritten by auto-disambiguate
}

// Add inserts new files into the existing index DB.
func Add(vaultPath string, opts AddOptions) (*AddResult, error) {
	// DB existence check.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	// Normalize and deduplicate input paths.
	type addFile struct {
		path  string
		mtime int64
	}
	seen := make(map[string]bool)
	var files []addFile
	for _, f := range opts.Files {
		np := NormalizePath(f)
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

	// Check that no file is already registered.
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

	// Check disk existence and collect mtime.
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

	// Build maps from DB + save oldBasenameCounts copy.
	pathToID, pathSet, basenameCounts, rootBasenameToPath, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}
	oldBasenameCounts := make(map[string]int, len(basenameCounts))
	for k, v := range basenameCounts {
		oldBasenameCounts[k] = v
	}

	// Adjust maps for post-add state.
	for _, f := range files {
		rel := strings.ToLower(f.path)
		pathSet[rel] = f.path
		noExt := strings.TrimSuffix(f.path, filepath.Ext(f.path))
		pathSet[strings.ToLower(noExt)] = f.path
		bk := basenameKey(f.path)
		basenameCounts[bk]++
		if isRootFile(f.path) {
			rootBasenameToPath[bk] = f.path
		}
	}

	// Check if adding causes existing links to become ambiguous.
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

	var allRewrites []rewriteEntry

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
		var isPatternA bool
		if oldCount == 1 {
			// Pattern A: existing unique note becomes ambiguous.
			oldTarget := oldBasenameToPath[bk]
			if isRootFile(oldTarget) {
				continue // Root-priority: [[A]] still resolves to root A.md → no ambiguity.
			}
			targetID = pathToID[oldTarget]
			isPatternA = true
		} else {
			// Pattern B: phantom (oldCount == 0, adding 2+ files with same basename).
			// Check if any of the new files is at root → root priority resolves.
			hasNewRoot := false
			for _, f := range files {
				if basenameKey(f.path) == bk && isRootFile(f.path) {
					hasNewRoot = true
					break
				}
			}
			if hasNewRoot {
				continue // Root priority: [[A]] resolves to root file → not ambiguous.
			}
			pk := phantomKey(bk)
			err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&targetID)
			if err == sql.ErrNoRows {
				continue // no phantom, so no existing basename links
			}
			if err != nil {
				return nil, err
			}
		}

		// Query edges with source info for potential rewriting.
		rows, err := db.Query(
			`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id
			 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
			 WHERE e.target_id = ?
			 AND e.link_type IN ('wikilink', 'markdown')`, targetID)
		if err != nil {
			return nil, err
		}
		var basenameEdges []rewriteEntry
		for rows.Next() {
			var re rewriteEntry
			if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID); err != nil {
				rows.Close()
				return nil, err
			}
			if isBasenameRawLink(re.rawLink, re.linkType) {
				basenameEdges = append(basenameEdges, re)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		if len(basenameEdges) == 0 {
			continue
		}

		if isPatternA && opts.AutoDisambiguate {
			// Compute new raw links for each edge.
			targetPath := oldBasenameToPath[bk]
			for i := range basenameEdges {
				basenameEdges[i].newRawLink = rewriteRawLink(basenameEdges[i].rawLink, basenameEdges[i].linkType, targetPath)
			}
			allRewrites = append(allRewrites, basenameEdges...)
		} else {
			// Pattern B or auto-disambiguate not enabled → error.
			return nil, fmt.Errorf("adding files would make existing links ambiguous")
		}
	}

	// Stale check for source files that need rewriting.
	if len(allRewrites) > 0 {
		sourceStaleChecked := make(map[int64]bool)
		for _, re := range allRewrites {
			if sourceStaleChecked[re.sourceID] {
				continue
			}
			sourceStaleChecked[re.sourceID] = true
			var dbMtime int64
			err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", re.sourceID).Scan(&dbMtime)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(filepath.Join(vaultPath, re.sourcePath))
			if err != nil {
				return nil, err
			}
			if info.ModTime().Unix() != dbMtime {
				return nil, fmt.Errorf("source file is stale: %s", re.sourcePath)
			}
		}
	}

	// Build basenameToPath (includes both existing and new files).
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

	// Parse all new files and check for ambiguous links.
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
			if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, f.path)
			}
			if link.isBasename && isAmbiguousBasenameLink(link.target, basenameCounts, pathSet) {
				return nil, fmt.Errorf("ambiguous link: %s in %s", link.target, f.path)
			}
		}

		parsed = append(parsed, parsedFile{file: f, links: links})
	}

	// Apply disk rewrites before transaction (so DB rollback is safe).
	// newMtimes maps sourceID → new mtime after file write.
	var newMtimes map[int64]int64
	var backups []rewriteBackup
	if len(allRewrites) > 0 {
		// Group rewrites by source file.
		groups := make(map[string][]rewriteEntry)
		for _, re := range allRewrites {
			groups[re.sourcePath] = append(groups[re.sourcePath], re)
		}
		var applyErr error
		newMtimes, backups, applyErr = applyFileRewrites(vaultPath, groups)
		if applyErr != nil {
			return nil, applyErr
		}
	}

	// Begin transaction.
	tx, err := db.Begin()
	if err != nil {
		// Restore disk changes if transaction start fails.
		restoreBackups(vaultPath, backups)
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
			// Restore disk changes on failure (best-effort).
			restoreBackups(vaultPath, backups)
		}
	}()

	result := &AddResult{}

	// Insert all note nodes.
	for _, pf := range parsed {
		name := basename(pf.file.path)
		id, err := upsertNote(tx, pf.file.path, name, pf.file.mtime)
		if err != nil {
			return nil, err
		}
		pathToID[pf.file.path] = id
		result.Added = append(result.Added, pf.file.path)
	}

	// Phantom → note promotion (root-priority aware).
	// When multiple files share a basename, prefer root file for phantom promotion.
	rootForBasename := make(map[string]string) // bk → root file path
	for _, pf := range parsed {
		bk := basenameKey(pf.file.path)
		if isRootFile(pf.file.path) {
			rootForBasename[bk] = pf.file.path
		}
	}
	promotedBasenames := make(map[string]bool)
	for _, pf := range parsed {
		bk := basenameKey(pf.file.path)
		if promotedBasenames[bk] {
			continue
		}
		// If a root file exists for this basename but this isn't it, skip (root will promote).
		if rp, ok := rootForBasename[bk]; ok && rp != pf.file.path {
			continue
		}

		pk := phantomKey(basename(pf.file.path))
		var phantomID int64
		err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
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

		promotedBasenames[bk] = true
		result.Promoted = append(result.Promoted, pf.file.path)
	}

	// Resolve links and create edges.
	for _, pf := range parsed {
		sourceID := pathToID[pf.file.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.file.path, link, pathSet, basenameToPath, rootBasenameToPath, pathToID)
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

	// Update DB for rewritten edges.
	if len(allRewrites) > 0 {
		for _, re := range allRewrites {
			if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
				return nil, err
			}
			result.Rewritten = append(result.Rewritten, RewrittenLink{
				File:    re.sourcePath,
				OldLink: re.rawLink,
				NewLink: re.newRawLink,
			})
		}
		// Update source mtime in DB.
		mtimeUpdated := make(map[int64]bool)
		for _, re := range allRewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := newMtimes[re.sourceID]
			if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}

	// Orphan cleanup.
	if err := cleanupOrphanedNodes(tx); err != nil {
		return nil, err
	}

	// Commit.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return result, nil
}
