package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MoveOptions controls the move operation.
type MoveOptions struct {
	From string // vault-relative old path
	To   string // vault-relative new path
}

// MoveResult reports the outcome of the move operation.
type MoveResult struct {
	Rewritten []RewrittenLink
}

// Move moves a file from one path to another, updating the index and rewriting links.
// If the file has already been moved on disk (from absent, to present), the disk move
// is skipped and only link rewrites + DB updates are performed.
func Move(vaultPath string, opts MoveOptions) (*MoveResult, error) {
	// Phase 0: validation.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	from := normalizePath(opts.From)
	to := normalizePath(opts.To)

	if from == to {
		return nil, fmt.Errorf("source and destination are the same: %s", from)
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Check from is registered as a note in DB.
	fromKey := noteKey(from)
	var nodeID int64
	var dbMtime int64
	err = db.QueryRow("SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'note'", fromKey).Scan(&nodeID, &dbMtime)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file not registered: %s", from)
	}
	if err != nil {
		return nil, err
	}

	// Check to is not already registered in DB.
	toKey := noteKey(to)
	var existingID int64
	err = db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", toKey).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("destination already registered: %s", to)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Determine disk state: from present, to present.
	fromOnDisk := fileExists(filepath.Join(vaultPath, from))
	toOnDisk := fileExists(filepath.Join(vaultPath, to))

	// Determine whether we need to do the disk move.
	var needDiskMove bool
	switch {
	case fromOnDisk && !toOnDisk:
		// Normal: mdhop performs the move.
		needDiskMove = true
	case !fromOnDisk && toOnDisk:
		// Already moved: skip disk move.
		needDiskMove = false
	case fromOnDisk && toOnDisk:
		return nil, fmt.Errorf("destination already exists on disk: %s", to)
	default: // !fromOnDisk && !toOnDisk
		return nil, fmt.Errorf("source file not found on disk: %s", from)
	}

	// Stale check for the moved file.
	if needDiskMove {
		info, err := os.Stat(filepath.Join(vaultPath, from))
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != dbMtime {
			return nil, fmt.Errorf("source file is stale: %s", from)
		}
	} else {
		// Already moved: check that the file at 'to' has the same mtime as DB recorded for 'from'.
		// os.Rename preserves mtime, so a mismatch means the file was edited after the move.
		info, err := os.Stat(filepath.Join(vaultPath, to))
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != dbMtime {
			return nil, fmt.Errorf("moved file is stale: %s", to)
		}
	}

	// Phase 1: build maps and adjust for post-move state.
	pathToID, pathSet, basenameCounts, rootBasenameToPath, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Save pre-move pathSet for Phase 2/2.5 root-priority checks.
	preMovePathSet := make(map[string]string, len(pathSet))
	for k, v := range pathSet {
		preMovePathSet[k] = v
	}

	// Remove from from maps.
	delete(pathToID, from)
	fromLower := strings.ToLower(from)
	delete(pathSet, fromLower)
	fromNoExt := strings.TrimSuffix(from, filepath.Ext(from))
	delete(pathSet, strings.ToLower(fromNoExt))
	basenameCounts[basenameKey(from)]--
	if isRootFile(from) {
		delete(rootBasenameToPath, basenameKey(from))
	}

	// Add to to maps.
	pathToID[to] = nodeID
	toLower := strings.ToLower(to)
	pathSet[toLower] = to
	toNoExt := strings.TrimSuffix(to, filepath.Ext(to))
	pathSet[strings.ToLower(toNoExt)] = to
	basenameCounts[basenameKey(to)]++
	if isRootFile(to) {
		rootBasenameToPath[basenameKey(to)] = to
	}

	// Build basenameToPath (count == 1 only).
	basenameToPath := make(map[string]string)
	// We need to iterate all current notes to build this map.
	// pathToID was already adjusted, so iterate it.
	for p := range pathToID {
		bk := basenameKey(p)
		if basenameCounts[bk] == 1 {
			basenameToPath[bk] = p
		}
	}

	// Phase 2: incoming link rewrite.
	incomingRows, err := db.Query(
		`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id
		 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		 WHERE e.target_id = ? AND e.link_type IN ('wikilink', 'markdown')`, nodeID)
	if err != nil {
		return nil, err
	}
	var incomingRewrites []rewriteEntry
	for incomingRows.Next() {
		var re rewriteEntry
		if err := incomingRows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID); err != nil {
			incomingRows.Close()
			return nil, err
		}
		// Skip self-reference edges (source == moved file); handled in outgoing phase.
		if re.sourcePath == from {
			continue
		}
		if isBasenameRawLink(re.rawLink, re.linkType) {
			// Basename link: determine if rewrite is needed.
			fromBK := basenameKey(from)
			toBK := basenameKey(to)
			if fromBK != toBK {
				// Basename changed → must rewrite.
				re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, to)
				incomingRewrites = append(incomingRewrites, re)
			} else if basenameCounts[toBK] > 1 {
				// Basename unchanged but ambiguous after move.
				// Root priority: skip rewrite if root file exists both before AND after move.
				preRoot := hasRootInPathSet(toBK, preMovePathSet)
				postRoot := hasRootInPathSet(toBK, pathSet)
				if !(preRoot && postRoot) {
					re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, to)
					incomingRewrites = append(incomingRewrites, re)
				}
			}
			// else: basename unchanged and unique → no rewrite needed.
		} else {
			// Path link → always rewrite.
			re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, to)
			incomingRewrites = append(incomingRewrites, re)
		}
	}
	incomingRows.Close()
	if err := incomingRows.Err(); err != nil {
		return nil, err
	}

	// Phase 2.5: vault-wide ambiguity check for the destination basename.
	toBK := basenameKey(to)
	if basenameCounts[toBK] > 1 {
		// Root priority: if root file exists both before AND after move, basename links
		// still resolve to root → no ambiguity.
		preRoot := hasRootInPathSet(toBK, preMovePathSet)
		postRoot := hasRootInPathSet(toBK, pathSet)
		if !(preRoot && postRoot) {
			// The destination basename is ambiguous. Check if there are basename links
			// targeting this basename from files other than the moved file.
			// These links would become ambiguous after the move.
			rows, err := db.Query(
				`SELECT e.raw_link, e.link_type, sn.path
				 FROM edges e
				 JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
				 JOIN nodes tn ON tn.id = e.target_id
				 WHERE tn.name = ? AND e.link_type IN ('wikilink', 'markdown')`,
				basename(to))
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var rawLink, linkType, sourcePath string
				if err := rows.Scan(&rawLink, &linkType, &sourcePath); err != nil {
					rows.Close()
					return nil, err
				}
				if sourcePath == from {
					continue // handled in outgoing phase
				}
				if isBasenameRawLink(rawLink, linkType) {
					// Check if this edge is already being rewritten.
					alreadyHandled := false
					for _, re := range incomingRewrites {
						if re.sourcePath == sourcePath && re.rawLink == rawLink {
							alreadyHandled = true
							break
						}
					}
					if !alreadyHandled {
						rows.Close()
						return nil, fmt.Errorf("move would make existing links ambiguous for basename: %s", basename(to))
					}
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
	}

	// Stale check for incoming rewrite source files.
	if len(incomingRewrites) > 0 {
		sourceStaleChecked := make(map[int64]bool)
		for _, re := range incomingRewrites {
			if sourceStaleChecked[re.sourceID] {
				continue
			}
			sourceStaleChecked[re.sourceID] = true
			var srcMtime int64
			err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", re.sourceID).Scan(&srcMtime)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(filepath.Join(vaultPath, re.sourcePath))
			if err != nil {
				return nil, err
			}
			if info.ModTime().Unix() != srcMtime {
				return nil, fmt.Errorf("source file is stale: %s", re.sourcePath)
			}
		}
	}

	// Phase 3: outgoing link rewrite.
	// Read the file content from its current disk location.
	var movedFilePath string
	if needDiskMove {
		movedFilePath = filepath.Join(vaultPath, from)
	} else {
		movedFilePath = filepath.Join(vaultPath, to)
	}
	movedInfo, err := os.Stat(movedFilePath)
	if err != nil {
		return nil, err
	}
	movedPerm := movedInfo.Mode().Perm()
	movedContent, err := os.ReadFile(movedFilePath)
	if err != nil {
		return nil, err
	}
	outgoingLinks := parseLinks(string(movedContent))

	// Check for ambiguous outgoing links and collect relative link rewrites.
	type outgoingRewrite struct {
		rawLink    string
		newRawLink string
		lineStart  int
	}
	var outgoingRewrites []outgoingRewrite

	for _, link := range outgoingLinks {
		if link.linkType != "wikilink" && link.linkType != "markdown" {
			continue
		}
		// Ambiguity check on post-move maps.
		if link.isBasename && isAmbiguousBasenameLink(link.target, basenameCounts, pathSet) {
			return nil, fmt.Errorf("ambiguous link after move: %s in %s", link.target, to)
		}
		// Relative link rewrite.
		if link.isRelative {
			newRL, err := rewriteOutgoingRelativeLink(link.rawLink, link.linkType, from, to)
			if err != nil {
				return nil, err
			}
			if newRL != link.rawLink {
				outgoingRewrites = append(outgoingRewrites, outgoingRewrite{
					rawLink:    link.rawLink,
					newRawLink: newRL,
					lineStart:  link.lineStart,
				})
			}
		}
	}

	// Phase 4: disk operations.
	result := &MoveResult{}

	// 4.1: apply incoming link rewrites to other files.
	var incomingBackups []rewriteBackup
	var incomingMtimes map[int64]int64
	if len(incomingRewrites) > 0 {
		groups := make(map[string][]rewriteEntry)
		for _, re := range incomingRewrites {
			groups[re.sourcePath] = append(groups[re.sourcePath], re)
		}
		var applyErr error
		incomingMtimes, incomingBackups, applyErr = applyFileRewrites(vaultPath, groups)
		if applyErr != nil {
			return nil, applyErr
		}
	}

	// 4.2: apply outgoing relative link rewrites to the moved file.
	var movedFileBackup *rewriteBackup
	if len(outgoingRewrites) > 0 {
		// Save backup of the file at its current disk location.
		movedFileBackup = &rewriteBackup{path: from, content: movedContent, perm: movedPerm}
		if !needDiskMove {
			movedFileBackup.path = to
		}

		lines := strings.Split(string(movedContent), "\n")
		lineRewrites := make(map[int][]outgoingRewrite)
		for _, ow := range outgoingRewrites {
			lineRewrites[ow.lineStart] = append(lineRewrites[ow.lineStart], ow)
		}
		for lineNum, ows := range lineRewrites {
			if lineNum < 1 || lineNum > len(lines) {
				continue
			}
			idx := lineNum - 1
			for _, ow := range ows {
				lines[idx] = replaceOutsideInlineCode(lines[idx], ow.rawLink, ow.newRawLink)
			}
		}
		movedContent = []byte(strings.Join(lines, "\n"))

		// Write the rewritten content back to the current disk location.
		if err := writeFilePreservePerm(movedFilePath, movedContent, movedPerm); err != nil {
			restoreBackups(vaultPath, incomingBackups)
			return nil, err
		}
	}

	// 4.3: disk move (if needed).
	if needDiskMove {
		toFull := filepath.Join(vaultPath, to)
		toDir := filepath.Dir(toFull)
		if err := os.MkdirAll(toDir, 0o755); err != nil {
			// Rollback: restore incoming and moved file.
			if movedFileBackup != nil {
				_ = writeFilePreservePerm(filepath.Join(vaultPath, movedFileBackup.path), movedFileBackup.content, movedFileBackup.perm)
			}
			restoreBackups(vaultPath, incomingBackups)
			return nil, err
		}
		if err := os.Rename(filepath.Join(vaultPath, from), toFull); err != nil {
			if movedFileBackup != nil {
				_ = writeFilePreservePerm(filepath.Join(vaultPath, movedFileBackup.path), movedFileBackup.content, movedFileBackup.perm)
			}
			restoreBackups(vaultPath, incomingBackups)
			return nil, err
		}
	}

	// Get the mtime of the file at its final location.
	toInfo, err := os.Stat(filepath.Join(vaultPath, to))
	if err != nil {
		// Rollback disk move.
		if needDiskMove {
			_ = os.Rename(filepath.Join(vaultPath, to), filepath.Join(vaultPath, from))
		}
		if movedFileBackup != nil {
			_ = writeFilePreservePerm(filepath.Join(vaultPath, movedFileBackup.path), movedFileBackup.content, movedFileBackup.perm)
		}
		restoreBackups(vaultPath, incomingBackups)
		return nil, err
	}
	toMtime := toInfo.ModTime().Unix()

	// Phase 5: DB transaction.
	tx, err := db.Begin()
	if err != nil {
		if needDiskMove {
			_ = os.Rename(filepath.Join(vaultPath, to), filepath.Join(vaultPath, from))
		}
		if movedFileBackup != nil {
			_ = writeFilePreservePerm(filepath.Join(vaultPath, movedFileBackup.path), movedFileBackup.content, movedFileBackup.perm)
		}
		restoreBackups(vaultPath, incomingBackups)
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
			if needDiskMove {
				_ = os.Rename(filepath.Join(vaultPath, to), filepath.Join(vaultPath, from))
			}
			if movedFileBackup != nil {
				diskPath := movedFileBackup.path
				if needDiskMove {
					// After rollback of Rename, file is back at 'from'.
					diskPath = from
				}
				_ = writeFilePreservePerm(filepath.Join(vaultPath, diskPath), movedFileBackup.content, movedFileBackup.perm)
			}
			restoreBackups(vaultPath, incomingBackups)
		}
	}()

	// 5.1: update node for the moved file.
	newName := basename(to)
	if _, err := tx.Exec(
		"UPDATE nodes SET node_key = ?, name = ?, path = ?, mtime = ? WHERE id = ?",
		toKey, newName, to, toMtime, nodeID); err != nil {
		return nil, err
	}

	// 5.2: delete old outgoing edges.
	if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", nodeID); err != nil {
		return nil, err
	}

	// 5.3: re-parse moved file content and create new edges (using new path).
	newLinks := parseLinks(string(movedContent))
	for _, link := range newLinks {
		targetID, subpath, err := resolveLink(tx, to, link, pathSet, basenameToPath, rootBasenameToPath, pathToID)
		if err != nil {
			return nil, err
		}
		if targetID == 0 {
			continue
		}
		if err := insertEdge(tx, nodeID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
			return nil, err
		}
	}

	// 5.4: update incoming edge raw_links.
	for _, re := range incomingRewrites {
		if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
			return nil, err
		}
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	// 5.5: update source file mtimes for rewritten files.
	if incomingMtimes != nil {
		mtimeUpdated := make(map[int64]bool)
		for _, re := range incomingRewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := incomingMtimes[re.sourceID]
			if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}

	// Add outgoing rewrites to result.
	for _, ow := range outgoingRewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    to,
			OldLink: ow.rawLink,
			NewLink: ow.newRawLink,
		})
	}

	// 5.6: phantom promotion — check if to's basename matches a phantom.
	pk := phantomKey(basename(to))
	var phantomID int64
	err = tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
	if err == nil {
		// Reassign incoming edges from phantom to moved note.
		if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", nodeID, phantomID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
			return nil, err
		}
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// Orphan cleanup.
	if err := cleanupOrphanedNodes(tx); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return result, nil
}

// rewriteOutgoingRelativeLink rewrites a relative link in the moved file
// from the old path perspective to the new path perspective.
func rewriteOutgoingRelativeLink(rawLink, linkType, from, to string) (string, error) {
	switch linkType {
	case "wikilink":
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")

		var alias, subpath string
		if idx := strings.Index(inner, "|"); idx >= 0 {
			alias = inner[idx:]
			inner = inner[:idx]
		}
		if idx := strings.Index(inner, "#"); idx >= 0 {
			subpath = inner[idx:]
			inner = inner[:idx]
		}

		// Resolve from old location.
		resolvedTarget := normalizePath(filepath.Join(filepath.Dir(from), inner))

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(normalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
			return "", fmt.Errorf("rewritten link would escape vault: %s", rawLink)
		}

		// Add ./ prefix if needed.
		if !strings.HasPrefix(rel, "..") {
			rel = "./" + rel
		}

		// Wikilink: always remove .md.
		rel = strings.TrimSuffix(rel, ".md")

		return "[[" + rel + subpath + alias + "]]", nil

	case "markdown":
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return rawLink, nil
		}
		textPart := rawLink[:start+2]
		urlPart := rawLink[start+2:]
		urlPart = strings.TrimSuffix(urlPart, ")")

		var frag string
		if idx := strings.Index(urlPart, "#"); idx >= 0 {
			frag = urlPart[idx:]
			urlPart = urlPart[:idx]
		}

		hasMdExt := strings.HasSuffix(strings.ToLower(urlPart), ".md")

		// Resolve from old location.
		resolvedTarget := normalizePath(filepath.Join(filepath.Dir(from), urlPart))

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(normalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
			return "", fmt.Errorf("rewritten link would escape vault: %s", rawLink)
		}

		// Add ./ prefix if needed.
		if !strings.HasPrefix(rel, "..") {
			rel = "./" + rel
		}

		// Markdown: preserve .md extension presence.
		if hasMdExt {
			if !strings.HasSuffix(strings.ToLower(rel), ".md") {
				rel += ".md"
			}
		} else {
			rel = strings.TrimSuffix(rel, ".md")
		}

		return textPart + rel + frag + ")", nil
	}
	return rawLink, nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
