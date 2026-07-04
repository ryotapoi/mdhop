package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// outgoingRewrite records a single outgoing link rewrite in the moved file.
type outgoingRewrite struct {
	rawLink    string
	newRawLink string
	lineStart  int
}

// moveInfo records one file move within a directory move.
type moveInfo struct {
	from    string
	to      string
	nodeID  int64
	dbMtime int64
	isAsset bool
}

// movedFileRewrite records the original content and outgoing rewrites for one moved note.
type movedFileRewrite struct {
	content     []byte
	perm        os.FileMode
	outRewrites []outgoingRewrite
}

// diskOnlyMove records a non-registered file (not in DB) that must be moved alongside
// the registered files in a directory move.
type diskOnlyMove struct {
	from string
	to   string
}

// dirMoveMaps captures the resolveMaps state before and after applying directory move
// path adjustments, used for root-priority decisions during link rewrite.
type dirMoveMaps struct {
	rm                  *resolveMaps
	preMovePathSet      map[string]string
	preMoveAssetPathSet map[string]string
	movedFromTo         map[string]string
	movedNodeIDs        map[int64]bool
}

// validateMoveDirOptions checks that source and destination directories are well-formed
// and disjoint, returning the normalized fromDir/toDir or an error.
func validateMoveDirOptions(opts MoveDirOptions) (fromDir, toDir string, err error) {
	fromDir = NormalizePath(opts.FromDir)
	toDir = NormalizePath(opts.ToDir)

	if filepath.IsAbs(fromDir) {
		return "", "", fmt.Errorf("source directory must be vault-relative: %s", fromDir)
	}
	if filepath.IsAbs(toDir) {
		return "", "", fmt.Errorf("destination directory must be vault-relative: %s", toDir)
	}
	if pathEscapesVault(fromDir) {
		return "", "", fmt.Errorf("source directory escapes vault: %s", fromDir)
	}
	if pathEscapesVault(toDir) {
		return "", "", fmt.Errorf("destination directory escapes vault: %s", toDir)
	}
	if fromDir == toDir {
		return "", "", fmt.Errorf("source and destination are the same: %s", fromDir)
	}
	if strings.HasPrefix(toDir+"/", fromDir+"/") || strings.HasPrefix(fromDir+"/", toDir+"/") {
		return "", "", fmt.Errorf("source and destination directories overlap")
	}
	return fromDir, toDir, nil
}

// loadSingleMoveFromDB resolves one registered note or asset for single-file Move.
func loadSingleMoveFromDB(db dbExecer, from, to string) (moveInfo, error) {
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return moveInfo{}, err
	}

	var nodeID int64
	var isAsset bool
	if id, ok := rm.pathToID[from]; ok {
		nodeID = id
	} else if id, ok := rm.assetPathToID[from]; ok {
		nodeID = id
		isAsset = true
	} else {
		return moveInfo{}, fmt.Errorf("%w: %s", ErrFileNotRegistered, from)
	}

	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", nodeID).Scan(&dbMtime); err != nil {
		return moveInfo{}, err
	}
	return moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime, isAsset: isAsset}, nil
}

// loadMovesFromDB resolves all registered notes and assets under fromDir into moveInfo
// records carrying their computed to-paths under toDir.
func loadMovesFromDB(db dbExecer, fromDir, toDir string) ([]moveInfo, error) {
	fromNotePaths, err := listDirNodesByType(db, fromDir, NodeTypeNote)
	if err != nil {
		return nil, err
	}
	fromAssetPaths, err := listDirNodesByType(db, fromDir, NodeTypeAsset)
	if err != nil {
		return nil, err
	}
	if len(fromNotePaths) == 0 && len(fromAssetPaths) == 0 {
		return nil, fmt.Errorf("no files registered under directory: %s", fromDir)
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
	return moves, nil
}

// checkDestinationsFree returns ErrAlreadyRegistered if any destination key already exists.
func checkDestinationsFree(db dbExecer, moves []moveInfo) error {
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
			return fmt.Errorf("%w: %s", ErrAlreadyRegistered, m.to)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// collectDiskOnlyFiles walks fromDir on disk and returns non-.md files that are not
// registered in the DB. They will be moved verbatim alongside the registered files.
// Returns an empty slice when fromDir does not exist on disk (already-moved mode).
func collectDiskOnlyFiles(vaultPath, fromDir, toDir string, moves []moveInfo) ([]diskOnlyMove, error) {
	var diskOnlyFiles []diskOnlyMove
	absDir := filepath.Join(vaultPath, fromDir)
	registeredPaths := make(map[string]bool, len(moves))
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
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(vaultPath, path)
		relNorm := NormalizePath(rel)
		if !registeredPaths[relNorm] {
			to := toDir + "/" + strings.TrimPrefix(relNorm, fromDir+"/")
			diskOnlyFiles = append(diskOnlyFiles, diskOnlyMove{from: relNorm, to: to})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return diskOnlyFiles, nil
}

// classifyDiskState determines whether the registered files still need to be moved on disk.
// Returns needDiskMove=true when all files are at their from paths, false when they are
// already at the to paths. Mixed or missing states return an error.
//
// Callers must pass a non-empty moves slice; an empty slice returns (false, nil) which
// is meaningless. MoveDir guards this via loadMovesFromDB which errors on no matches.
func classifyDiskState(vaultPath string, moves []moveInfo) (needDiskMove bool, err error) {
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
			return false, fmt.Errorf("%w: %s", ErrAlreadyExistsOnDisk, m.to)
		default:
			return false, fmt.Errorf("%w: %s", ErrSourceFileMissing, m.from)
		}
	}
	if normalMode && alreadyMovedMode {
		return false, fmt.Errorf("inconsistent disk state for directory move")
	}
	return normalMode, nil
}

// checkMovedFilesNotStale verifies that on-disk mtimes match the DB-recorded mtimes
// at the appropriate location for each move.
func checkMovedFilesNotStale(vaultPath string, moves []moveInfo, needDiskMove bool) error {
	for _, m := range moves {
		var checkPath string
		if needDiskMove {
			checkPath = filepath.Join(vaultPath, m.from)
		} else {
			checkPath = filepath.Join(vaultPath, m.to)
		}
		info, err := os.Stat(checkPath)
		if err != nil {
			return err
		}
		if info.ModTime().Unix() != m.dbMtime {
			if needDiskMove {
				return fmt.Errorf("%w: %s", ErrSourceStale, m.from)
			}
			return fmt.Errorf("%w: %s", ErrMovedFileStale, m.to)
		}
	}
	return nil
}

// adjustMapsForDirMove builds resolveMaps then snapshots the pre-move pathSets and
// applies the move (remove from-paths, add to-paths, rebuild basename maps).
func adjustMapsForDirMove(db dbExecer, moves []moveInfo) (*dirMoveMaps, error) {
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

	movedFromTo := make(map[string]string, len(moves))
	movedNodeIDs := make(map[int64]bool, len(moves))
	for _, m := range moves {
		movedFromTo[m.from] = m.to
		movedNodeIDs[m.nodeID] = true
	}

	for _, m := range moves {
		if m.isAsset {
			rm.removeAsset(m.from)
		} else {
			rm.removeNote(m.from)
		}
	}
	for _, m := range moves {
		if m.isAsset {
			rm.assetPathToID[m.to] = m.nodeID
			rm.addAsset(m.to)
		} else {
			rm.pathToID[m.to] = m.nodeID
			rm.addNote(m.to)
		}
	}
	rm.rebuildBasenameToPath(nil)
	rm.rebuildAssetBasenameToPath()

	return &dirMoveMaps{
		rm:                  rm,
		preMovePathSet:      preMovePathSet,
		preMoveAssetPathSet: preMoveAssetPathSet,
		movedFromTo:         movedFromTo,
		movedNodeIDs:        movedNodeIDs,
	}, nil
}

// collectIncomingRewritesForDir scans edges that target any moved node from external
// sources and decides which need rewriting based on basename/path resolution.
func collectIncomingRewritesForDir(db dbExecer, moves []moveInfo, dm *dirMoveMaps) ([]rewriteEntry, error) {
	nodeIDs := make([]int64, 0, len(moves))
	nodeIDFromPath := make(map[int64]string, len(moves))
	nodeIDToPath := make(map[int64]string, len(moves))
	nodeIDIsAsset := make(map[int64]bool, len(moves))
	for _, m := range moves {
		nodeIDs = append(nodeIDs, m.nodeID)
		nodeIDFromPath[m.nodeID] = m.from
		nodeIDToPath[m.nodeID] = m.to
		nodeIDIsAsset[m.nodeID] = m.isAsset
	}

	rm := dm.rm
	var incomingRewrites []rewriteEntry
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
			 WHERE e.target_id IN (%s) AND e.link_type IN (%s)`,
			strings.Join(placeholders, ","),
			pathLinkTypeSQLList,
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
			if dm.movedNodeIDs[re.sourceID] {
				continue
			}
			toPath := nodeIDToPath[targetID]
			if toPath == "" {
				continue
			}

			if isBasenameRawLink(re.rawLink, re.linkType) {
				fromPath := nodeIDFromPath[targetID]
				var fromBK, toBK string
				var counts map[string]int
				var prePS, postPS map[string]string
				if nodeIDIsAsset[targetID] {
					fromBK = assetBasenameKey(fromPath)
					toBK = assetBasenameKey(toPath)
					counts = rm.assetBasenameCounts
					prePS = dm.preMoveAssetPathSet
					postPS = rm.assetPathSet
				} else {
					fromBK = basenameKey(fromPath)
					toBK = basenameKey(toPath)
					counts = rm.basenameCounts
					prePS = dm.preMovePathSet
					postPS = rm.pathSet
				}
				if fromBK != toBK {
					re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, toPath)
					incomingRewrites = append(incomingRewrites, re)
				} else if counts[toBK] > 1 {
					preRoot := hasRootInPathSet(toBK, prePS)
					postRoot := hasRootInPathSet(toBK, postPS)
					if !(preRoot && postRoot) {
						re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, toPath)
						incomingRewrites = append(incomingRewrites, re)
					}
				}
			} else {
				re.newRawLink = rewriteRawLink(re.rawLink, re.linkType, toPath)
				incomingRewrites = append(incomingRewrites, re)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return incomingRewrites, nil
}

// collectCollateralRewritesForDir finds basename links to non-moved nodes whose
// resolution flips because of root-priority changes after the directory move.
// In a pure directory rename, file basenames are unchanged so basenameCounts stay
// constant; collateral rewrites still fire when a file moves between a
// subdirectory and the vault root (changing hasRootInPathSet).
func collectCollateralRewritesForDir(db dbExecer, moves []moveInfo, dm *dirMoveMaps) ([]rewriteEntry, error) {
	rm := dm.rm
	var collateralRewrites []rewriteEntry

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
		preRoot := hasRootInPathSet(bk, dm.preMovePathSet)
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
		crs, err := queryCollateralRewrites(db, NodeTypeNote, bn, dm.movedNodeIDs)
		if err != nil {
			return nil, err
		}
		collateralRewrites = append(collateralRewrites, crs...)
	}

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
		preRoot := hasRootInPathSet(abk, dm.preMoveAssetPathSet)
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
		crs, err := queryCollateralRewrites(db, NodeTypeAsset, bn, dm.movedNodeIDs)
		if err != nil {
			return nil, err
		}
		collateralRewrites = append(collateralRewrites, crs...)
	}
	return collateralRewrites, nil
}

// buildMovedFileRewrites reads each moved note from disk and computes its outgoing
// link rewrites. Assets have no outgoing links and yield empty entries.
func buildMovedFileRewrites(db dbExecer, vaultPath string, moves []moveInfo, dm *dirMoveMaps, needDiskMove bool) ([]movedFileRewrite, error) {
	rm := dm.rm
	movedFileRewrites := make([]movedFileRewrite, len(moves))
	for i, m := range moves {
		if m.isAsset {
			continue
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
		movedFileRewrites[i] = movedFileRewrite{content: content, perm: info.Mode().Perm()}

		links := parseLinks(string(content)).Links
		for _, link := range links {
			if !isPathLinkType(link.linkType) {
				continue
			}

			if link.isBasename {
				bk := basenameKey(link.target)
				preMoveTargetPath, err := lookupEdgeTargetPath(db, m.nodeID, link.rawLink)
				if err != nil {
					return nil, err
				}
				if preMoveTargetPath == "" {
					continue
				}

				postMoveTargetPath := preMoveTargetPath
				if newPath, ok := dm.movedFromTo[preMoveTargetPath]; ok {
					postMoveTargetPath = newPath
				}

				needRewrite := false
				if basenameKey(postMoveTargetPath) != bk {
					needRewrite = true
				} else if p, ok := rm.basenameToPath[bk]; ok {
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
					movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, outgoingRewrite{
						rawLink:    link.rawLink,
						newRawLink: newRL,
						lineStart:  link.lineStart,
					})
				}
				continue
			}

			if link.isRelative {
				newRL, err := rewriteOutgoingRelativeLink(link.rawLink, link.linkType, m.from, m.to, dm.movedFromTo)
				if err != nil {
					return nil, err
				}
				if newRL != link.rawLink {
					movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, outgoingRewrite{
						rawLink:    link.rawLink,
						newRawLink: newRL,
						lineStart:  link.lineStart,
					})
				}
				continue
			}

			if link.target == "" {
				continue
			}
			preMoveTargetPath, err := lookupEdgeTargetPath(db, m.nodeID, link.rawLink)
			if err != nil {
				return nil, err
			}
			if preMoveTargetPath == "" {
				continue
			}
			if newPath, ok := dm.movedFromTo[preMoveTargetPath]; ok {
				newRL := rewriteRawLink(link.rawLink, link.linkType, newPath)
				movedFileRewrites[i].outRewrites = append(movedFileRewrites[i].outRewrites, outgoingRewrite{
					rawLink:    link.rawLink,
					newRawLink: newRL,
					lineStart:  link.lineStart,
				})
			}
		}
	}
	return movedFileRewrites, nil
}

// lookupEdgeTargetPath returns the target node path for the edge identified by
// (sourceID, rawLink) among path-resolving link types (see pathLinkTypeSQLList).
// Returns ("", nil) when no matching edge exists or when the target is a
// phantom node (whose path is stored as NULL); callers treat both cases as
// "skip".
func lookupEdgeTargetPath(db dbExecer, sourceID int64, rawLink string) (string, error) {
	var path string
	err := db.QueryRow(fmt.Sprintf(
		`SELECT COALESCE(tn.path, '') FROM edges e
		 JOIN nodes tn ON tn.id = e.target_id
		 WHERE e.source_id = ? AND e.raw_link = ? AND e.link_type IN (%s)
		 LIMIT 1`, pathLinkTypeSQLList), sourceID, rawLink).Scan(&path)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return path, nil
}

// queryCollateralRewrites finds basename links to non-moved nodes of the given type
// that need rewriting due to root-priority changes.
// The JOIN condition tn.exists_flag = 1 excludes phantom nodes (path=NULL), which
// have no disk content to rewrite.
func queryCollateralRewrites(db dbExecer, nodeType NodeType, name string, movedNodeIDs map[int64]bool) ([]rewriteEntry, error) {
	rows, err := db.Query(fmt.Sprintf(
		`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id, tn.path, tn.id
		 FROM edges e
		 JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		 JOIN nodes tn ON tn.id = e.target_id AND tn.type = ? AND tn.exists_flag = 1
		 WHERE tn.name = ? AND e.link_type IN (%s)`, pathLinkTypeSQLList),
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

// rewriteOutgoingRelativeLink rewrites a relative link in the moved file
// from the old path perspective to the new path perspective.
// If movedFromTo is non-nil, it also checks whether the target was moved.
func rewriteOutgoingRelativeLink(rawLink string, linkType LinkType, from, to string, movedFromTo map[string]string) (string, error) {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
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

		// Check if target is also being moved. movedFromTo keys are vault-relative
		// paths with .md extension (as stored in DB), but wikilink targets resolve
		// without .md, so check both forms and strip .md when matching the bare form.
		if movedFromTo != nil {
			if newTarget, ok := movedFromTo[resolvedTarget]; ok {
				resolvedTarget = newTarget
			} else if newTarget, ok := movedFromTo[resolvedTarget+".md"]; ok {
				resolvedTarget = strings.TrimSuffix(newTarget, ".md")
			}
		}

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))

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

	case LinkTypeMarkdown:
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
		if movedFromTo != nil {
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
		}

		// Compute relative from new location.
		rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))

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

// groupAndApplyExternalRewrites groups rewrites by source path and applies them.
// Returns per-sourceID mtimes and backups for rollback.
func groupAndApplyExternalRewrites(vaultPath string, rewrites []rewriteEntry) (map[int64]int64, []rewriteBackup, []rollbackFailure, error) {
	if len(rewrites) == 0 {
		return nil, nil, nil, nil
	}
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	return applyFileRewritesWithRollbackFailures(vaultPath, groups)
}

// applyOutgoingRewritesToContent applies outgoing rewrites to file content,
// returning new content. The original content is not modified.
func applyOutgoingRewritesToContent(content []byte, rewrites []outgoingRewrite) []byte {
	lines := strings.Split(string(content), "\n")
	lineRewrites := make(map[int][]outgoingRewrite)
	for _, ow := range rewrites {
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
	return []byte(strings.Join(lines, "\n"))
}

// applyMovedFileRewrites writes outgoing rewrites to moved files and returns
// backups for later rollback. On write failure, already-written moved files are
// restored best-effort.
func applyMovedFileRewrites(vaultPath string, moves []moveInfo, movedFileRewrites []movedFileRewrite, needDiskMove bool) ([]rewriteBackup, []rollbackFailure, error) {
	var backups []rewriteBackup
	for i, mfr := range movedFileRewrites {
		if len(mfr.outRewrites) == 0 {
			continue
		}
		m := moves[i]
		diskPath := m.to
		if needDiskMove {
			diskPath = m.from
		}

		newContent := applyOutgoingRewritesToContent(mfr.content, mfr.outRewrites)
		movedFileRewrites[i].content = newContent

		fullPath := filepath.Join(vaultPath, diskPath)
		if err := writeFilePreservePerm(fullPath, newContent, mfr.perm); err != nil {
			restoreFailures := restoreBackupFiles(vaultPath, backups)
			return backups, restoreFailures, err
		}
		backups = append(backups, rewriteBackup{path: diskPath, content: mfr.content, perm: mfr.perm})
	}
	return backups, nil, nil
}

// updateExternalEdgesAndMtimes updates edge raw_links and source node mtimes
// for externally rewritten files. Returns the list of rewritten links.
func updateExternalEdgesAndMtimes(tx dbExecer, rewrites []rewriteEntry, mtimes map[int64]int64) ([]RewrittenLink, error) {
	var result []RewrittenLink
	for _, re := range rewrites {
		if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
			return nil, err
		}
		result = append(result, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}
	if mtimes != nil {
		mtimeUpdated := make(map[int64]bool)
		for _, re := range rewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := mtimes[re.sourceID]
			if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// promotePhantom replaces a phantom node with a real node by reassigning edges.
// If no phantom exists for the given name, this is a no-op (returns false).
//
// Frontmatter_path raw values re-resolve by path, not basename, on a full
// build (ADR 0014): edges whose raw value does not resolve to realPath stay
// on the phantom, which is then kept alive instead of deleted. The returned
// bool reports whether the real node took over at least one edge — a partial
// promotion (phantom surviving for unresolvable raws) still counts.
func promotePhantom(tx dbExecer, phantomName string, realNodeID int64, realPath string, rm *resolveMaps) (bool, error) {
	pk := phantomKey(phantomName)
	var phantomID int64
	err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	rows, err := tx.Query(`SELECT e.id, e.raw_link, sn.path
		FROM edges e JOIN nodes sn ON sn.id = e.source_id
		WHERE e.target_id = ? AND e.link_type = ?`, phantomID, LinkTypeFrontmatterPath)
	if err != nil {
		return false, err
	}
	type fmEdge struct {
		id         int64
		rawLink    string
		sourcePath string
	}
	var fmEdges []fmEdge
	for rows.Next() {
		var e fmEdge
		if err := rows.Scan(&e.id, &e.rawLink, &e.sourcePath); err != nil {
			rows.Close()
			return false, err
		}
		fmEdges = append(fmEdges, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	var keep []int64
	for _, e := range fmEdges {
		occ, ok := frontmatterPathOccur(e.rawLink, 0)
		if !ok {
			continue
		}
		resolved, err := resolveFrontmatterPathDry(e.sourcePath, occ, rm)
		if err != nil || resolved != realPath {
			keep = append(keep, e.id)
		}
	}

	if len(keep) == 0 {
		if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", realNodeID, phantomID); err != nil {
			return false, err
		}
		if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
			return false, err
		}
		return true, nil
	}

	placeholders := make([]string, len(keep))
	args := []any{realNodeID, phantomID}
	for i, id := range keep {
		placeholders[i] = "?"
		args = append(args, id)
	}
	res, err := tx.Exec(fmt.Sprintf(
		"UPDATE edges SET target_id = ? WHERE target_id = ? AND id NOT IN (%s)",
		strings.Join(placeholders, ",")), args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
