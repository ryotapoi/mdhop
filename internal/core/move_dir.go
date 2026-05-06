package core

import (
	"os"
	"path/filepath"
)

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
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Phase 0: validation and load.
	fromDir, toDir, err := validateMoveDirOptions(opts)
	if err != nil {
		return nil, err
	}
	moves, err := loadMovesFromDB(db, fromDir, toDir)
	if err != nil {
		return nil, err
	}
	if err := checkDestinationsFree(db, moves); err != nil {
		return nil, err
	}
	diskOnlyFiles, err := collectDiskOnlyFiles(vaultPath, fromDir, toDir, moves)
	if err != nil {
		return nil, err
	}
	needDiskMove, err := classifyDiskState(vaultPath, moves)
	if err != nil {
		return nil, err
	}
	if err := checkMovedFilesNotStale(vaultPath, moves, needDiskMove); err != nil {
		return nil, err
	}

	// Phase 1: build maps and adjust for post-move state.
	dm, err := adjustMapsForDirMove(db, moves)
	if err != nil {
		return nil, err
	}

	// Phase 2 + 2.5: external link rewrites (incoming + collateral).
	incomingRewrites, err := collectIncomingRewritesForDir(db, moves, dm)
	if err != nil {
		return nil, err
	}
	collateralRewrites, err := collectCollateralRewritesForDir(db, moves, dm)
	if err != nil {
		return nil, err
	}
	allExternalRewrites := make([]rewriteEntry, 0, len(incomingRewrites)+len(collateralRewrites))
	allExternalRewrites = append(allExternalRewrites, incomingRewrites...)
	allExternalRewrites = append(allExternalRewrites, collateralRewrites...)

	// Phase 3: outgoing link rewrites for moved notes.
	movedFileRewrites, err := buildMovedFileRewrites(db, vaultPath, moves, dm, needDiskMove)
	if err != nil {
		return nil, err
	}

	// Phase 4: disk operations.
	result := &MoveDirResult{}

	// 4.1: apply external rewrites.
	externalMtimes, externalBackups, err := groupAndApplyExternalRewrites(vaultPath, allExternalRewrites)
	if err != nil {
		return nil, err
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

		newContent := applyOutgoingRewritesToContent(mfr.content, mfr.outRewrites)
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
		// Restore moved file backups. After rename rollback (if any), each backup's
		// restorePath is the original disk location.
		for _, b := range movedFileBackups {
			_ = writeFilePreservePerm(filepath.Join(vaultPath, b.restorePath), b.content, b.perm)
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
		newLinks := parseLinks(string(movedFileRewrites[i].content)).Links
		for _, link := range newLinks {
			targetID, subpath, err := resolveLink(tx, m.to, link, dm.rm)
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

	// 5.3: update external edge raw_links and source mtimes.
	externalRewritten, err := updateExternalEdgesAndMtimes(tx, allExternalRewrites, externalMtimes)
	if err != nil {
		return nil, err
	}
	result.Rewritten = append(result.Rewritten, externalRewritten...)

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
		if err := promotePhantom(tx, phantomName, m.nodeID); err != nil {
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
