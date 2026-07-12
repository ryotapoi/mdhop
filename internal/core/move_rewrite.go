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

// movedFileRewrite records the original content and outgoing rewrites for one moved note.
type movedFileRewrite struct {
	content     []byte
	perm        os.FileMode
	outRewrites []outgoingRewrite
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
			rm.registerAsset(m.to, m.nodeID)
			rm.addAsset(m.to)
		} else {
			rm.registerNote(m.to, m.nodeID)
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

// relativeLinkParts contains syntax-specific pieces needed by the common
// relative-link rewrite procedure. The target itself is always rewritten by
// the shared path calculation below; only wrappers and extension policy vary by type.
type relativeLinkParts struct {
	prefix       string
	target       string
	suffix       string
	preserveMD   bool
	stripMovedMD bool
}

func parseRelativeLink(rawLink string, linkType LinkType) (relativeLinkParts, bool) {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		inner := strings.TrimSuffix(strings.TrimPrefix(rawLink, "[["), "]]")
		var alias, subpath string
		if idx := strings.Index(inner, "|"); idx >= 0 {
			alias = inner[idx:]
			inner = inner[:idx]
		}
		if idx := strings.Index(inner, "#"); idx >= 0 {
			subpath = inner[idx:]
			inner = inner[:idx]
		}
		return relativeLinkParts{prefix: "[[", target: inner, suffix: subpath + alias + "]]", stripMovedMD: true}, true
	case LinkTypeMarkdown:
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return relativeLinkParts{}, false
		}
		prefix := rawLink[:start+2]
		urlPart := strings.TrimSuffix(rawLink[start+2:], ")")
		var fragment string
		if idx := strings.Index(urlPart, "#"); idx >= 0 {
			fragment = urlPart[idx:]
			urlPart = urlPart[:idx]
		}
		return relativeLinkParts{prefix: prefix, target: urlPart, suffix: fragment + ")", preserveMD: strings.HasSuffix(strings.ToLower(urlPart), ".md")}, true
	default:
		return relativeLinkParts{}, false
	}
}

func resolveMovedRelativeTarget(target string, movedFromTo map[string]string, stripMovedMD bool) string {
	if movedFromTo == nil {
		return target
	}
	keys := []string{target, target + ".md"}
	if !stripMovedMD {
		keys = append(keys, strings.TrimSuffix(target, ".md")+".md")
	}
	for _, key := range keys {
		if newTarget, ok := movedFromTo[key]; ok {
			if stripMovedMD {
				return strings.TrimSuffix(newTarget, ".md")
			}
			return newTarget
		}
	}
	return target
}

// rewriteOutgoingRelativeLink rewrites a relative link in the moved file
// from the old path perspective to the new path perspective.
// If movedFromTo is non-nil, it also checks whether the target was moved.
func rewriteOutgoingRelativeLink(rawLink string, linkType LinkType, from, to string, movedFromTo map[string]string) (string, error) {
	parts, ok := parseRelativeLink(rawLink, linkType)
	if !ok {
		return rawLink, nil
	}
	resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), parts.target))
	resolvedTarget = resolveMovedRelativeTarget(resolvedTarget, movedFromTo, parts.stripMovedMD)

	rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
		return "", fmt.Errorf("rewritten link would escape vault: %s", rawLink)
	}
	if !strings.HasPrefix(rel, "..") {
		rel = "./" + rel
	}
	if parts.preserveMD {
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			rel += ".md"
		}
	} else {
		rel = strings.TrimSuffix(rel, ".md")
	}
	return parts.prefix + rel + parts.suffix, nil
}
