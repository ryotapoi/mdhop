package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}

	from := NormalizePath(opts.From)
	to := NormalizePath(opts.To)

	if from == to {
		return nil, fmt.Errorf("source and destination are the same: %s", from)
	}

	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Check from is registered as a note or asset in DB.
	var nodeID int64
	var dbMtime int64
	var isAsset bool
	if id, ok := rm.pathToID[from]; ok {
		nodeID = id
	} else if id, ok := rm.assetPathToID[from]; ok {
		nodeID = id
		isAsset = true
	} else {
		return nil, fmt.Errorf("%w: %s", ErrFileNotRegistered, from)
	}
	err = db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", nodeID).Scan(&dbMtime)
	if err != nil {
		return nil, err
	}

	// Check to is not already registered in DB (note or asset).
	if isAsset {
		if _, ok := rm.assetPathToID[to]; ok {
			return nil, fmt.Errorf("%w: %s", ErrAlreadyRegistered, to)
		}
	} else {
		if _, ok := rm.pathToID[to]; ok {
			return nil, fmt.Errorf("%w: %s", ErrAlreadyRegistered, to)
		}
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
		return nil, fmt.Errorf("%w: %s", ErrAlreadyExistsOnDisk, to)
	default: // !fromOnDisk && !toOnDisk
		return nil, fmt.Errorf("%w: %s", ErrSourceFileMissing, from)
	}

	// Stale check for the moved file.
	if needDiskMove {
		info, err := os.Stat(filepath.Join(vaultPath, from))
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != dbMtime {
			return nil, fmt.Errorf("%w: %s", ErrSourceStale, from)
		}
	} else {
		// Already moved: check that the file at 'to' has the same mtime as DB recorded for 'from'.
		// os.Rename preserves mtime, so a mismatch means the file was edited after the move.
		info, err := os.Stat(filepath.Join(vaultPath, to))
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != dbMtime {
			return nil, fmt.Errorf("%w: %s", ErrMovedFileStale, to)
		}
	}

	// Phase 1: adjust maps for post-move state.
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
		rm.removeAsset(from)
		rm.assetPathToID[to] = nodeID
		rm.addAsset(to)
		rm.rebuildAssetBasenameToPath()
	} else {
		rm.removeNote(from)
		rm.pathToID[to] = nodeID
		rm.addNote(to)
		rm.rebuildBasenameToPath(nil)
	}

	// Frontmatter_path raw values (incoming, third-party, and the moved
	// file's own) must keep resolving to the same target after the move
	// (they cannot be rewritten).
	if err := validateFrontmatterPathEdges(db, rm, map[string]string{from: to}); err != nil {
		return nil, err
	}

	// Phase 2: incoming link rewrite.
	incomingRows, err := db.Query(fmt.Sprintf(
		`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id
		 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		 WHERE e.target_id = ? AND e.link_type IN (%s)`, pathLinkTypeSQLList), nodeID)
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
			targetType := NodeTypeNote
			collateralName := basename(to)
			if isAsset {
				targetType = NodeTypeAsset
				collateralName = filepath.Base(to)
			}
			singleMovedIDs := map[int64]bool{nodeID: true}
			collateralRewrites, err = queryCollateralRewrites(db, targetType, collateralName, singleMovedIDs)
			if err != nil {
				return nil, err
			}
		}
	}

	// Allocate a new slice to avoid aliasing incomingRewrites's backing array.
	allExternalRewrites := make([]rewriteEntry, 0, len(incomingRewrites)+len(collateralRewrites))
	allExternalRewrites = append(allExternalRewrites, incomingRewrites...)
	allExternalRewrites = append(allExternalRewrites, collateralRewrites...)

	// Phase 3: outgoing link rewrite (only for notes; assets have no outgoing links).
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
		outgoingLinks := parseLinks(string(movedContent)).Links

		for _, link := range outgoingLinks {
			if !isPathLinkType(link.linkType) {
				continue
			}
			// Basename link: check if resolution changes after move.
			if link.isBasename {
				bk := basenameKey(link.target)
				needRewrite := false
				var preMoveTargetPath string

				// Get pre-move target path from DB.
				err := db.QueryRow(fmt.Sprintf(
					`SELECT COALESCE(tn.path, '') FROM edges e
					 JOIN nodes tn ON tn.id = e.target_id
					 WHERE e.source_id = ? AND e.raw_link = ? AND e.link_type IN (%s)
					 LIMIT 1`, pathLinkTypeSQLList), nodeID, link.rawLink).Scan(&preMoveTargetPath)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
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
				newRL, err := rewriteOutgoingRelativeLink(link.rawLink, link.linkType, from, to, nil)
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
	externalMtimes, externalBackups, err := groupAndApplyExternalRewrites(vaultPath, allExternalRewrites)
	if err != nil {
		return nil, err
	}

	// 4.2: apply outgoing relative link rewrites to the moved file.
	var movedFileBackup *rewriteBackup
	if len(outgoingRewrites) > 0 {
		// Save backup of the file at its current disk location.
		movedFileBackup = &rewriteBackup{path: from, content: movedContent, perm: movedPerm}
		if !needDiskMove {
			movedFileBackup.path = to
		}

		movedContent = applyOutgoingRewritesToContent(movedContent, outgoingRewrites)

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
	toKey := noteKey(to)
	if isAsset {
		toKey = assetKey(to)
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
		newLinks := parseLinksWithLinkKeys(string(movedContent), cfg.Meta.LinkKeys).Links
		if err := validateParsedLinks(to, newLinks, rm); err != nil {
			return nil, err
		}
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

	// 5.4: update incoming + collateral edge raw_links and source mtimes.
	externalRewritten, err := updateExternalEdgesAndMtimes(tx, allExternalRewrites, externalMtimes)
	if err != nil {
		return nil, err
	}
	result.Rewritten = append(result.Rewritten, externalRewritten...)

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
	if _, err := promotePhantom(tx, phantomName, nodeID, to, rm); err != nil {
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
