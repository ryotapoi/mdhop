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

	from := NormalizePath(opts.From)
	to := NormalizePath(opts.To)

	if from == to {
		return nil, fmt.Errorf("source and destination are the same: %s", from)
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Check from is registered as a note or asset in DB.
	var nodeID int64
	var dbMtime int64
	var isAsset bool
	fromKey := noteKey(from)
	err = db.QueryRow("SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'note'", fromKey).Scan(&nodeID, &dbMtime)
	if err == sql.ErrNoRows {
		// Try asset.
		fromKey = assetKey(from)
		err = db.QueryRow("SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'asset'", fromKey).Scan(&nodeID, &dbMtime)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not registered: %s", from)
		}
		if err != nil {
			return nil, err
		}
		isAsset = true
	} else if err != nil {
		return nil, err
	}

	// Check to is not already registered in DB (note or asset).
	var toKey string
	if isAsset {
		toKey = assetKey(to)
	} else {
		toKey = noteKey(to)
	}
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
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Save pre-move pathSet for Phase 2/2.5 root-priority checks.
	var preMovePathSet map[string]string
	if isAsset {
		preMovePathSet = make(map[string]string, len(rm.assetPathSet))
		for k, v := range rm.assetPathSet {
			preMovePathSet[k] = v
		}
	} else {
		preMovePathSet = make(map[string]string, len(rm.pathSet))
		for k, v := range rm.pathSet {
			preMovePathSet[k] = v
		}
	}

	// Remove from and add to in maps.
	if isAsset {
		delete(rm.assetPathToID, from)
		delete(rm.assetPathSet, strings.ToLower(from))
		abk := assetBasenameKey(from)
		rm.assetBasenameCounts[abk]--
		if isRootFile(from) {
			delete(rm.assetRootBasenameToPath, abk)
		}

		rm.assetPathToID[to] = nodeID
		rm.assetPathSet[strings.ToLower(to)] = to
		abkTo := assetBasenameKey(to)
		rm.assetBasenameCounts[abkTo]++
		if isRootFile(to) {
			rm.assetRootBasenameToPath[abkTo] = to
		}

		// Rebuild assetBasenameToPath.
		rm.assetBasenameToPath = make(map[string]string)
		for p := range rm.assetPathToID {
			abk := assetBasenameKey(p)
			if rm.assetBasenameCounts[abk] == 1 {
				rm.assetBasenameToPath[abk] = p
			}
		}
	} else {
		delete(rm.pathToID, from)
		fromLower := strings.ToLower(from)
		delete(rm.pathSet, fromLower)
		fromNoExt := strings.TrimSuffix(from, filepath.Ext(from))
		delete(rm.pathSet, strings.ToLower(fromNoExt))
		rm.basenameCounts[basenameKey(from)]--
		if isRootFile(from) {
			delete(rm.rootBasenameToPath, basenameKey(from))
		}

		rm.pathToID[to] = nodeID
		toLower := strings.ToLower(to)
		rm.pathSet[toLower] = to
		toNoExt := strings.TrimSuffix(to, filepath.Ext(to))
		rm.pathSet[strings.ToLower(toNoExt)] = to
		rm.basenameCounts[basenameKey(to)]++
		if isRootFile(to) {
			rm.rootBasenameToPath[basenameKey(to)] = to
		}

		// Rebuild basenameToPath (count == 1 only).
		rm.basenameToPath = make(map[string]string)
		for p := range rm.pathToID {
			bk := basenameKey(p)
			if rm.basenameCounts[bk] == 1 {
				rm.basenameToPath[bk] = p
			}
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

	// For basename comparison, use the appropriate key function.
	moveBKFrom := basenameKey(from)
	moveBKTo := basenameKey(to)
	if isAsset {
		moveBKFrom = assetBasenameKey(from)
		moveBKTo = assetBasenameKey(to)
	}

	// Select the right counts/pathSet for this node type.
	var moveBasenameCounts map[string]int
	var movePathSet map[string]string
	if isAsset {
		moveBasenameCounts = rm.assetBasenameCounts
		movePathSet = rm.assetPathSet
	} else {
		moveBasenameCounts = rm.basenameCounts
		movePathSet = rm.pathSet
	}

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
			if moveBKFrom != moveBKTo {
				// Basename changed → must rewrite.
				re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, to)
				incomingRewrites = append(incomingRewrites, re)
			} else if moveBasenameCounts[moveBKTo] > 1 {
				// Basename unchanged but ambiguous after move.
				preRoot := hasRootInPathSet(moveBKTo, preMovePathSet)
				postRoot := hasRootInPathSet(moveBKTo, movePathSet)
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

	// Phase 2.5: collateral rewrite for the destination basename.
	var collateralRewrites []rewriteEntry
	if moveBasenameCounts[moveBKTo] > 1 {
		preRoot := hasRootInPathSet(moveBKTo, preMovePathSet)
		postRoot := hasRootInPathSet(moveBKTo, movePathSet)
		if !(preRoot && postRoot) {
			// Query target type matching the moved node.
			targetType := "note"
			collateralName := basename(to)
			if isAsset {
				targetType = "asset"
				collateralName = filepath.Base(to)
			}
			rows, err := db.Query(
				`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id, tn.path, tn.id
				 FROM edges e
				 JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
				 JOIN nodes tn ON tn.id = e.target_id AND tn.type = ? AND tn.exists_flag = 1
				 WHERE tn.name = ? AND e.link_type IN ('wikilink', 'markdown')`,
				targetType, collateralName)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var re rewriteEntry
				var targetPath string
				var targetNodeID int64
				if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID, &targetPath, &targetNodeID); err != nil {
					rows.Close()
					return nil, err
				}
				if re.sourcePath == from {
					continue // handled in outgoing phase
				}
				if !isBasenameRawLink(re.rawLink, re.linkType) {
					continue // path links are safe
				}
				if targetNodeID == nodeID {
					continue // incoming to moved file, handled in Phase 2
				}
				re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, targetPath)
				collateralRewrites = append(collateralRewrites, re)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
	}

	allExternalRewrites := make([]rewriteEntry, 0, len(incomingRewrites)+len(collateralRewrites))
	allExternalRewrites = append(allExternalRewrites, incomingRewrites...)
	allExternalRewrites = append(allExternalRewrites, collateralRewrites...)

	// Phase 3: outgoing link rewrite (only for notes; assets have no outgoing links).
	type outgoingRewrite struct {
		rawLink    string
		newRawLink string
		lineStart  int
	}
	var outgoingRewrites []outgoingRewrite
	var movedContent []byte
	var movedPerm os.FileMode
	var movedFilePath string

	if !isAsset {
		// Read the file content from its current disk location.
		if needDiskMove {
			movedFilePath = filepath.Join(vaultPath, from)
		} else {
			movedFilePath = filepath.Join(vaultPath, to)
		}
		movedInfo, err := os.Stat(movedFilePath)
		if err != nil {
			return nil, err
		}
		movedPerm = movedInfo.Mode().Perm()
		movedContent, err = os.ReadFile(movedFilePath)
		if err != nil {
			return nil, err
		}
		outgoingLinks := parseLinks(string(movedContent))

		for _, link := range outgoingLinks {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			// Basename link: check if resolution changes after move.
			if link.isBasename {
				bk := basenameKey(link.target)
				needRewrite := false
				var preMoveTargetPath string

				// Get pre-move target path from DB.
				err := db.QueryRow(
					`SELECT COALESCE(tn.path, '') FROM edges e
					 JOIN nodes tn ON tn.id = e.target_id
					 WHERE e.source_id = ? AND e.raw_link = ? AND e.link_type IN ('wikilink', 'markdown')
					 LIMIT 1`, nodeID, link.rawLink).Scan(&preMoveTargetPath)
				if err != nil && err != sql.ErrNoRows {
					return nil, err
				}

				if preMoveTargetPath != "" {
					// Determine post-move resolution.
					if p, ok := rm.basenameToPath[bk]; ok {
						if p != preMoveTargetPath {
							needRewrite = true
						}
					} else if p, ok := rm.rootBasenameToPath[bk]; ok {
						if p != preMoveTargetPath {
							needRewrite = true
						}
					} else if rm.basenameCounts[bk] > 1 {
						needRewrite = true
					}
				}

				if needRewrite {
					newRL := rewriteRawLink(link.rawLink, link.linkType, preMoveTargetPath)
					outgoingRewrites = append(outgoingRewrites, outgoingRewrite{
						rawLink:    link.rawLink,
						newRawLink: newRL,
						lineStart:  link.lineStart,
					})
				}
				continue
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
	}

	// Phase 4: disk operations.
	result := &MoveResult{}

	// 4.1: apply incoming + collateral link rewrites to other files.
	var externalBackups []rewriteBackup
	var externalMtimes map[int64]int64
	if len(allExternalRewrites) > 0 {
		groups := make(map[string][]rewriteEntry)
		for _, re := range allExternalRewrites {
			groups[re.sourcePath] = append(groups[re.sourcePath], re)
		}
		var applyErr error
		externalMtimes, externalBackups, applyErr = applyFileRewrites(vaultPath, groups)
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
			restoreBackups(vaultPath, externalBackups)
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
			restoreBackups(vaultPath, externalBackups)
			return nil, err
		}
		if err := os.Rename(filepath.Join(vaultPath, from), toFull); err != nil {
			if movedFileBackup != nil {
				_ = writeFilePreservePerm(filepath.Join(vaultPath, movedFileBackup.path), movedFileBackup.content, movedFileBackup.perm)
			}
			restoreBackups(vaultPath, externalBackups)
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
		restoreBackups(vaultPath, externalBackups)
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
		restoreBackups(vaultPath, externalBackups)
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
			restoreBackups(vaultPath, externalBackups)
		}
	}()

	// 5.1: update node for the moved file.
	var newName string
	if isAsset {
		newName = filepath.Base(to)
	} else {
		newName = basename(to)
	}
	if _, err := tx.Exec(
		"UPDATE nodes SET node_key = ?, name = ?, path = ?, mtime = ? WHERE id = ?",
		toKey, newName, to, toMtime, nodeID); err != nil {
		return nil, err
	}

	if !isAsset {
		// 5.2: delete old outgoing edges.
		if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", nodeID); err != nil {
			return nil, err
		}

		// 5.3: re-parse moved file content and create new edges (using new path).
		newLinks := parseLinks(string(movedContent))
		for _, link := range newLinks {
			targetID, subpath, err := resolveLink(tx, to, link, rm)
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
	}

	// 5.4: update incoming + collateral edge raw_links.
	for _, re := range allExternalRewrites {
		if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
			return nil, err
		}
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	// 5.5: update source file mtimes for all externally rewritten files.
	if externalMtimes != nil {
		mtimeUpdated := make(map[int64]bool)
		for _, re := range allExternalRewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := externalMtimes[re.sourceID]
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
	var phantomName string
	if isAsset {
		phantomName = filepath.Base(to) // asset phantom uses filename with extension
	} else {
		phantomName = basename(to)
	}
	pk := phantomKey(phantomName)
	var phantomID int64
	err = tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
	if err == nil {
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

// queryCollateralRewrites finds basename links to non-moved nodes of the given type
// that need rewriting due to root-priority changes.
func queryCollateralRewrites(db dbExecer, nodeType, name string, movedNodeIDs map[int64]bool) ([]rewriteEntry, error) {
	rows, err := db.Query(
		`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id, tn.path, tn.id
		 FROM edges e
		 JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		 JOIN nodes tn ON tn.id = e.target_id AND tn.type = ? AND tn.exists_flag = 1
		 WHERE tn.name = ? AND e.link_type IN ('wikilink', 'markdown')`,
		nodeType, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []rewriteEntry
	for rows.Next() {
		var re rewriteEntry
		var targetPath string
		var targetNodeID int64
		if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID, &targetPath, &targetNodeID); err != nil {
			return nil, err
		}
		if movedNodeIDs[re.sourceID] {
			continue
		}
		if !isBasenameRawLink(re.rawLink, re.linkType) {
			continue
		}
		if movedNodeIDs[targetNodeID] {
			continue // incoming to moved file, handled in Phase 2
		}
		re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, targetPath)
		result = append(result, re)
	}
	return result, rows.Err()
}

// MoveDirOptions controls the directory move operation.
type MoveDirOptions struct {
	FromDir string // vault-relative directory prefix (e.g., "sub")
	ToDir   string // vault-relative directory prefix (e.g., "newdir")
}

// MoveDirResult reports the outcome of the directory move operation.
type MoveDirResult struct {
	Moved     []MovedFile
	Rewritten []RewrittenLink
}

// MovedFile records a single file move within a directory move.
type MovedFile struct {
	From string
	To   string
}

// MoveDir moves all files under a directory to a new directory prefix,
// updating the index and rewriting links in a single batch.
func MoveDir(vaultPath string, opts MoveDirOptions) (*MoveDirResult, error) {
	// Phase 0: validation.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	fromDir := NormalizePath(opts.FromDir)
	toDir := NormalizePath(opts.ToDir)

	// Absolute path check.
	if filepath.IsAbs(fromDir) {
		return nil, fmt.Errorf("source directory must be vault-relative: %s", fromDir)
	}
	if filepath.IsAbs(toDir) {
		return nil, fmt.Errorf("destination directory must be vault-relative: %s", toDir)
	}

	// Vault escape check.
	if pathEscapesVault(fromDir) {
		return nil, fmt.Errorf("source directory escapes vault: %s", fromDir)
	}
	if pathEscapesVault(toDir) {
		return nil, fmt.Errorf("destination directory escapes vault: %s", toDir)
	}

	if fromDir == toDir {
		return nil, fmt.Errorf("source and destination are the same: %s", fromDir)
	}

	// Overlap check.
	if strings.HasPrefix(toDir+"/", fromDir+"/") || strings.HasPrefix(fromDir+"/", toDir+"/") {
		return nil, fmt.Errorf("source and destination directories overlap")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Get all notes under fromDir.
	fromNotePaths, err := listDirNodesByType(db, fromDir, "note")
	if err != nil {
		return nil, err
	}

	// Get all assets under fromDir.
	fromAssetPaths, err := listDirNodesByType(db, fromDir, "asset")
	if err != nil {
		return nil, err
	}

	if len(fromNotePaths) == 0 && len(fromAssetPaths) == 0 {
		return nil, fmt.Errorf("no files registered under directory: %s", fromDir)
	}

	// Build move list for notes.
	type moveInfo struct {
		from    string
		to      string
		nodeID  int64
		dbMtime int64
		isAsset bool
	}
	moves := make([]moveInfo, 0, len(fromNotePaths)+len(fromAssetPaths))
	for _, from := range fromNotePaths {
		to := toDir + "/" + strings.TrimPrefix(from, fromDir+"/")
		var nodeID, dbMtime int64
		err := db.QueryRow(
			"SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'note'",
			noteKey(from),
		).Scan(&nodeID, &dbMtime)
		if err != nil {
			return nil, err
		}
		moves = append(moves, moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime})
	}

	// Build move list for assets.
	for _, from := range fromAssetPaths {
		to := toDir + "/" + strings.TrimPrefix(from, fromDir+"/")
		var nodeID, dbMtime int64
		err := db.QueryRow(
			"SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'asset'",
			assetKey(from),
		).Scan(&nodeID, &dbMtime)
		if err != nil {
			return nil, err
		}
		moves = append(moves, moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime, isAsset: true})
	}

	// Check destinations not registered.
	for _, m := range moves {
		var toKey string
		if m.isAsset {
			toKey = assetKey(m.to)
		} else {
			toKey = noteKey(m.to)
		}
		var existingID int64
		err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", toKey).Scan(&existingID)
		if err == nil {
			return nil, fmt.Errorf("destination already registered: %s", m.to)
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	// Collect non-registered disk files under fromDir for disk-only move.
	var diskOnlyFiles []struct{ from, to string }
	absDir := filepath.Join(vaultPath, fromDir)
	registeredPaths := make(map[string]bool)
	for _, m := range moves {
		registeredPaths[m.from] = true
	}
	if err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		// Only move non-.md files as disk-only (D5: disk-based move for assets only).
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(vaultPath, path)
		relNorm := NormalizePath(rel)
		if !registeredPaths[relNorm] {
			to := toDir + "/" + strings.TrimPrefix(relNorm, fromDir+"/")
			diskOnlyFiles = append(diskOnlyFiles, struct{ from, to string }{relNorm, to})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Determine disk state.
	var normalMode, alreadyMovedMode bool
	for _, m := range moves {
		fromOnDisk := fileExists(filepath.Join(vaultPath, m.from))
		toOnDisk := fileExists(filepath.Join(vaultPath, m.to))
		switch {
		case fromOnDisk && !toOnDisk:
			normalMode = true
		case !fromOnDisk && toOnDisk:
			alreadyMovedMode = true
		case fromOnDisk && toOnDisk:
			return nil, fmt.Errorf("destination already exists on disk: %s", m.to)
		default:
			return nil, fmt.Errorf("source file not found on disk: %s", m.from)
		}
	}
	if normalMode && alreadyMovedMode {
		return nil, fmt.Errorf("inconsistent disk state for directory move")
	}
	needDiskMove := normalMode

	// Stale check for moved files.
	for _, m := range moves {
		var checkPath string
		if needDiskMove {
			checkPath = filepath.Join(vaultPath, m.from)
		} else {
			checkPath = filepath.Join(vaultPath, m.to)
		}
		info, err := os.Stat(checkPath)
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != m.dbMtime {
			if needDiskMove {
				return nil, fmt.Errorf("source file is stale: %s", m.from)
			}
			return nil, fmt.Errorf("moved file is stale: %s", m.to)
		}
	}

	// Phase 1: build maps and adjust for post-move state.
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	preMovePathSet := make(map[string]string, len(rm.pathSet))
	for k, v := range rm.pathSet {
		preMovePathSet[k] = v
	}
	preMoveAssetPathSet := make(map[string]string, len(rm.assetPathSet))
	for k, v := range rm.assetPathSet {
		preMoveAssetPathSet[k] = v
	}

	// Build movedFromTo and movedNodeIDs.
	movedFromTo := make(map[string]string, len(moves))
	movedNodeIDs := make(map[int64]bool, len(moves))
	for _, m := range moves {
		movedFromTo[m.from] = m.to
		movedNodeIDs[m.nodeID] = true
	}

	// Remove all from paths, add all to paths.
	for _, m := range moves {
		if m.isAsset {
			delete(rm.assetPathToID, m.from)
			delete(rm.assetPathSet, strings.ToLower(m.from))
			abk := assetBasenameKey(m.from)
			rm.assetBasenameCounts[abk]--
			if isRootFile(m.from) {
				delete(rm.assetRootBasenameToPath, abk)
			}
		} else {
			delete(rm.pathToID, m.from)
			fromLower := strings.ToLower(m.from)
			delete(rm.pathSet, fromLower)
			fromNoExt := strings.TrimSuffix(m.from, filepath.Ext(m.from))
			delete(rm.pathSet, strings.ToLower(fromNoExt))
			rm.basenameCounts[basenameKey(m.from)]--
			if isRootFile(m.from) {
				delete(rm.rootBasenameToPath, basenameKey(m.from))
			}
		}
	}
	for _, m := range moves {
		if m.isAsset {
			rm.assetPathToID[m.to] = m.nodeID
			rm.assetPathSet[strings.ToLower(m.to)] = m.to
			abk := assetBasenameKey(m.to)
			rm.assetBasenameCounts[abk]++
			if isRootFile(m.to) {
				rm.assetRootBasenameToPath[abk] = m.to
			}
		} else {
			rm.pathToID[m.to] = m.nodeID
			toLower := strings.ToLower(m.to)
			rm.pathSet[toLower] = m.to
			toNoExt := strings.TrimSuffix(m.to, filepath.Ext(m.to))
			rm.pathSet[strings.ToLower(toNoExt)] = m.to
			rm.basenameCounts[basenameKey(m.to)]++
			if isRootFile(m.to) {
				rm.rootBasenameToPath[basenameKey(m.to)] = m.to
			}
		}
	}

	// Rebuild basenameToPath (count == 1 only).
	rm.basenameToPath = make(map[string]string)
	for p := range rm.pathToID {
		bk := basenameKey(p)
		if rm.basenameCounts[bk] == 1 {
			rm.basenameToPath[bk] = p
		}
	}
	rm.assetBasenameToPath = make(map[string]string)
	for p := range rm.assetPathToID {
		abk := assetBasenameKey(p)
		if rm.assetBasenameCounts[abk] == 1 {
			rm.assetBasenameToPath[abk] = p
		}
	}

	// Phase 2: incoming link rewrite.
	// Batch query: all incoming edges to any moved node, from external sources.
	var incomingRewrites []rewriteEntry
	nodeIDs := make([]int64, 0, len(moves))
	nodeIDToPath := make(map[int64]string, len(moves))
	nodeIDIsAsset := make(map[int64]bool, len(moves))
	for _, m := range moves {
		nodeIDs = append(nodeIDs, m.nodeID)
		nodeIDToPath[m.nodeID] = m.to
		nodeIDIsAsset[m.nodeID] = m.isAsset
	}

	// Process in batches of 500 to stay under SQLite parameter limit.
	const batchSize = 500
	for batchStart := 0; batchStart < len(nodeIDs); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(nodeIDs) {
			batchEnd = len(nodeIDs)
		}
		batch := nodeIDs[batchStart:batchEnd]

		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf(
			`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id, e.target_id
			 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
			 WHERE e.target_id IN (%s) AND e.link_type IN ('wikilink', 'markdown')`,
			strings.Join(placeholders, ","),
		)
		rows, err := db.Query(query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var re rewriteEntry
			var targetID int64
			if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID, &targetID); err != nil {
				rows.Close()
				return nil, err
			}
			// Skip if source is in moved set (handled in Phase 3).
			if movedNodeIDs[re.sourceID] {
				continue
			}
			// Find the target's new path.
			toPath := nodeIDToPath[targetID]
			if toPath == "" {
				continue // should not happen
			}

			if isBasenameRawLink(re.rawLink, re.linkType) {
				// In dir move, basename doesn't change. Check if ambiguous.
				// Use the correct key function/counts/pathSet based on node type.
				var fromBK string
				var counts map[string]int
				var prePS, postPS map[string]string
				if nodeIDIsAsset[targetID] {
					fromBK = assetBasenameKey(toPath)
					counts = rm.assetBasenameCounts
					prePS = preMoveAssetPathSet
					postPS = rm.assetPathSet
				} else {
					fromBK = basenameKey(toPath)
					counts = rm.basenameCounts
					prePS = preMovePathSet
					postPS = rm.pathSet
				}
				if counts[fromBK] > 1 {
					preRoot := hasRootInPathSet(fromBK, prePS)
					postRoot := hasRootInPathSet(fromBK, postPS)
					if !(preRoot && postRoot) {
						re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, toPath)
						incomingRewrites = append(incomingRewrites, re)
					}
				}
				// else: basename unchanged and unique → no rewrite needed.
			} else {
				// Path link → always rewrite.
				re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, toPath)
				incomingRewrites = append(incomingRewrites, re)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Phase 2.5: collateral rewrite for notes and assets.
	var collateralRewrites []rewriteEntry

	// Note collateral: collect affected note basenames.
	affectedNoteBasenames := make(map[string]bool)
	for _, m := range moves {
		if m.isAsset {
			continue
		}
		bk := basenameKey(m.to)
		if rm.basenameCounts[bk] > 1 {
			affectedNoteBasenames[bk] = true
		}
	}
	for bk := range affectedNoteBasenames {
		preRoot := hasRootInPathSet(bk, preMovePathSet)
		postRoot := hasRootInPathSet(bk, rm.pathSet)
		if preRoot && postRoot {
			continue
		}
		var bn string
		for _, m := range moves {
			if !m.isAsset && basenameKey(m.to) == bk {
				bn = basename(m.to)
				break
			}
		}
		crs, err := queryCollateralRewrites(db, "note", bn, movedNodeIDs)
		if err != nil {
			return nil, err
		}
		collateralRewrites = append(collateralRewrites, crs...)
	}

	// Asset collateral: collect affected asset basenames.
	affectedAssetBasenames := make(map[string]bool)
	for _, m := range moves {
		if !m.isAsset {
			continue
		}
		abk := assetBasenameKey(m.to)
		if rm.assetBasenameCounts[abk] > 1 {
			affectedAssetBasenames[abk] = true
		}
	}
	for abk := range affectedAssetBasenames {
		preRoot := hasRootInPathSet(abk, preMoveAssetPathSet)
		postRoot := hasRootInPathSet(abk, rm.assetPathSet)
		if preRoot && postRoot {
			continue
		}
		var bn string
		for _, m := range moves {
			if m.isAsset && assetBasenameKey(m.to) == abk {
				bn = filepath.Base(m.to)
				break
			}
		}
		crs, err := queryCollateralRewrites(db, "asset", bn, movedNodeIDs)
		if err != nil {
			return nil, err
		}
		collateralRewrites = append(collateralRewrites, crs...)
	}

	allExternalRewrites := make([]rewriteEntry, 0, len(incomingRewrites)+len(collateralRewrites))
	allExternalRewrites = append(allExternalRewrites, incomingRewrites...)
	allExternalRewrites = append(allExternalRewrites, collateralRewrites...)

	// Phase 3: outgoing link rewrite.
	type movedFileRewrite struct {
		content    []byte
		perm       os.FileMode
		outRewrites []struct {
			rawLink    string
			newRawLink string
			lineStart  int
		}
	}
	movedFileRewrites := make([]movedFileRewrite, len(moves))
	for i, m := range moves {
		if m.isAsset {
			continue // assets have no outgoing links to rewrite
		}
		var diskPath string
		if needDiskMove {
			diskPath = filepath.Join(vaultPath, m.from)
		} else {
			diskPath = filepath.Join(vaultPath, m.to)
		}
		info, err := os.Stat(diskPath)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(diskPath)
		if err != nil {
			return nil, err
		}
		movedFileRewrites[i] = movedFileRewrite{
			content: content,
			perm:    info.Mode().Perm(),
		}

		links := parseLinks(string(content))
		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}

			if link.isBasename {
				bk := basenameKey(link.target)
				// Get pre-move target path from DB.
				var preMoveTargetPath string
				err := db.QueryRow(
					`SELECT COALESCE(tn.path, '') FROM edges e
					 JOIN nodes tn ON tn.id = e.target_id
					 WHERE e.source_id = ? AND e.raw_link = ? AND e.link_type IN ('wikilink', 'markdown')
					 LIMIT 1`, m.nodeID, link.rawLink).Scan(&preMoveTargetPath)
				if err != nil && err != sql.ErrNoRows {
					return nil, err
				}
				if preMoveTargetPath == "" {
					continue // phantom target, skip
				}

				// Check if target is in moved set → use post-move path.
				postMoveTargetPath := preMoveTargetPath
				if newPath, ok := movedFromTo[preMoveTargetPath]; ok {
					postMoveTargetPath = newPath
				}

				// Determine post-move resolution.
				needRewrite := false
				if p, ok := rm.basenameToPath[bk]; ok {
					if p != postMoveTargetPath {
						needRewrite = true
					}
				} else if p, ok := rm.rootBasenameToPath[bk]; ok {
					if p != postMoveTargetPath {
						needRewrite = true
					}
				} else if rm.basenameCounts[bk] > 1 {
					needRewrite = true
				}

				if needRewrite {
					newRL := rewriteRawLink(link.rawLink, link.linkType, postMoveTargetPath)
					movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, struct {
						rawLink    string
						newRawLink string
						lineStart  int
					}{link.rawLink, newRL, link.lineStart})
				}
				continue
			}

			if link.isRelative {
				newRL, err := rewriteOutgoingRelativeLinkBatch(link.rawLink, link.linkType, m.from, m.to, movedFromTo)
				if err != nil {
					return nil, err
				}
				if newRL != link.rawLink {
					movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, struct {
						rawLink    string
						newRawLink string
						lineStart  int
					}{link.rawLink, newRL, link.lineStart})
				}
				continue
			}

			// Path-specified link (non-relative, non-basename).
			// Get pre-move target from DB.
			var preMoveTargetPath string
			err = db.QueryRow(
				`SELECT COALESCE(tn.path, '') FROM edges e
				 JOIN nodes tn ON tn.id = e.target_id
				 WHERE e.source_id = ? AND e.raw_link = ? AND e.link_type IN ('wikilink', 'markdown')
				 LIMIT 1`, m.nodeID, link.rawLink).Scan(&preMoveTargetPath)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if preMoveTargetPath == "" {
				continue // phantom target, skip
			}
			if newPath, ok := movedFromTo[preMoveTargetPath]; ok {
				newRL := rewriteRawLink(link.rawLink, link.linkType, newPath)
				movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, struct {
					rawLink    string
					newRawLink string
					lineStart  int
				}{link.rawLink, newRL, link.lineStart})
			}
		}
	}

	// Phase 4: disk operations.
	result := &MoveDirResult{}

	// 4.1: apply external rewrites.
	var externalBackups []rewriteBackup
	var externalMtimes map[int64]int64
	if len(allExternalRewrites) > 0 {
		groups := make(map[string][]rewriteEntry)
		for _, re := range allExternalRewrites {
			groups[re.sourcePath] = append(groups[re.sourcePath], re)
		}
		var applyErr error
		externalMtimes, externalBackups, applyErr = applyFileRewrites(vaultPath, groups)
		if applyErr != nil {
			return nil, applyErr
		}
	}

	// 4.2: apply outgoing rewrites to moved files.
	type movedBackup struct {
		restorePath string
		content     []byte
		perm        os.FileMode
	}
	var movedFileBackups []movedBackup
	for i, mfr := range movedFileRewrites {
		if len(mfr.outRewrites) == 0 {
			continue
		}
		m := moves[i]
		var diskPath string
		if needDiskMove {
			diskPath = m.from
		} else {
			diskPath = m.to
		}
		movedFileBackups = append(movedFileBackups, movedBackup{
			restorePath: diskPath,
			content:     mfr.content,
			perm:        mfr.perm,
		})

		lines := strings.Split(string(mfr.content), "\n")
		lineRewrites := make(map[int][]struct {
			rawLink    string
			newRawLink string
			lineStart  int
		})
		for _, ow := range mfr.outRewrites {
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
		newContent := []byte(strings.Join(lines, "\n"))
		movedFileRewrites[i].content = newContent

		fullPath := filepath.Join(vaultPath, diskPath)
		if err := writeFilePreservePerm(fullPath, newContent, mfr.perm); err != nil {
			// Restore previous moved file backups.
			for _, b := range movedFileBackups[:len(movedFileBackups)-1] {
				_ = writeFilePreservePerm(filepath.Join(vaultPath, b.restorePath), b.content, b.perm)
			}
			restoreBackups(vaultPath, externalBackups)
			return nil, err
		}
	}

	// 4.3: disk moves (if needed).
	type completedRename struct {
		from string
		to   string
	}
	var completedRenames []completedRename
	committed := false

	defer func() {
		if committed {
			return
		}
		// Rollback renames.
		for j := len(completedRenames) - 1; j >= 0; j-- {
			cr := completedRenames[j]
			_ = os.Rename(filepath.Join(vaultPath, cr.to), filepath.Join(vaultPath, cr.from))
		}
		// Restore moved file backups.
		for _, b := range movedFileBackups {
			restorePath := b.restorePath
			if needDiskMove {
				// After rollback, files are back at from paths.
				// restorePath is already the from path for normal mode.
			}
			_ = writeFilePreservePerm(filepath.Join(vaultPath, restorePath), b.content, b.perm)
		}
		restoreBackups(vaultPath, externalBackups)
	}()

	if needDiskMove {
		for _, m := range moves {
			toFull := filepath.Join(vaultPath, m.to)
			toFileDir := filepath.Dir(toFull)
			if err := os.MkdirAll(toFileDir, 0o755); err != nil {
				return nil, err
			}
			if err := os.Rename(filepath.Join(vaultPath, m.from), toFull); err != nil {
				return nil, err
			}
			completedRenames = append(completedRenames, completedRename{from: m.from, to: m.to})
		}
		// Move disk-only files (not registered in DB).
		for _, df := range diskOnlyFiles {
			toFull := filepath.Join(vaultPath, df.to)
			toFileDir := filepath.Dir(toFull)
			if err := os.MkdirAll(toFileDir, 0o755); err != nil {
				return nil, err
			}
			if err := os.Rename(filepath.Join(vaultPath, df.from), toFull); err != nil {
				return nil, err
			}
			completedRenames = append(completedRenames, completedRename{from: df.from, to: df.to})
		}
	}

	// Collect mtimes at final locations.
	toMtimes := make(map[int64]int64, len(moves))
	for _, m := range moves {
		info, err := os.Stat(filepath.Join(vaultPath, m.to))
		if err != nil {
			return nil, err
		}
		toMtimes[m.nodeID] = info.ModTime().Unix()
	}

	// Phase 5: DB transaction.
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// 5.1: update nodes for moved files.
	for _, m := range moves {
		var newName, toKeyStr string
		if m.isAsset {
			newName = filepath.Base(m.to)
			toKeyStr = assetKey(m.to)
		} else {
			newName = basename(m.to)
			toKeyStr = noteKey(m.to)
		}
		if _, err := tx.Exec(
			"UPDATE nodes SET node_key = ?, name = ?, path = ?, mtime = ? WHERE id = ?",
			toKeyStr, newName, m.to, toMtimes[m.nodeID], m.nodeID); err != nil {
			return nil, err
		}
	}

	// 5.2: delete old outgoing edges and re-parse (notes only; assets have no outgoing).
	for i, m := range moves {
		if m.isAsset {
			continue
		}
		if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", m.nodeID); err != nil {
			return nil, err
		}
		newLinks := parseLinks(string(movedFileRewrites[i].content))
		for _, link := range newLinks {
			targetID, subpath, err := resolveLink(tx, m.to, link, rm)
			if err != nil {
				return nil, err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, m.nodeID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return nil, err
			}
		}
	}

	// 5.3: update external edge raw_links.
	for _, re := range allExternalRewrites {
		if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
			return nil, err
		}
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	// 5.4: update source file mtimes for externally rewritten files.
	if externalMtimes != nil {
		mtimeUpdated := make(map[int64]bool)
		for _, re := range allExternalRewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := externalMtimes[re.sourceID]
			if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}

	// Add outgoing rewrites to result.
	for i, mfr := range movedFileRewrites {
		for _, ow := range mfr.outRewrites {
			result.Rewritten = append(result.Rewritten, RewrittenLink{
				File:    moves[i].to,
				OldLink: ow.rawLink,
				NewLink: ow.newRawLink,
			})
		}
	}

	// Build moved list.
	for _, m := range moves {
		result.Moved = append(result.Moved, MovedFile{From: m.from, To: m.to})
	}
	for _, df := range diskOnlyFiles {
		result.Moved = append(result.Moved, MovedFile{From: df.from, To: df.to})
	}

	// 5.5: phantom promotion.
	for _, m := range moves {
		var phantomName string
		if m.isAsset {
			phantomName = filepath.Base(m.to)
		} else {
			phantomName = basename(m.to)
		}
		pk := phantomKey(phantomName)
		var phantomID int64
		err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
		if err == nil {
			if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", m.nodeID, phantomID); err != nil {
				return nil, err
			}
			if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
				return nil, err
			}
		} else if err != sql.ErrNoRows {
			return nil, err
		}
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

// rewriteOutgoingRelativeLinkBatch is like rewriteOutgoingRelativeLink
// but accounts for the target file also being moved.
func rewriteOutgoingRelativeLinkBatch(rawLink, linkType, from, to string, movedFromTo map[string]string) (string, error) {
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
		resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), inner))

		// Check if target is also being moved.
		if newTarget, ok := movedFromTo[resolvedTarget]; ok {
			resolvedTarget = newTarget
		} else if newTarget, ok := movedFromTo[resolvedTarget+".md"]; ok {
			resolvedTarget = strings.TrimSuffix(newTarget, ".md")
		}

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
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
		resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), urlPart))

		// Check if target is also being moved.
		if newTarget, ok := movedFromTo[resolvedTarget]; ok {
			resolvedTarget = newTarget
		} else if newTarget, ok := movedFromTo[resolvedTarget+".md"]; ok {
			resolvedTarget = newTarget
		} else {
			// Try with .md stripped for lookup.
			noExt := strings.TrimSuffix(resolvedTarget, ".md")
			if newTarget, ok := movedFromTo[noExt+".md"]; ok {
				resolvedTarget = newTarget
			}
		}

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
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
		resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), inner))

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
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
		resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), urlPart))

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Check vault escape.
		if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
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
