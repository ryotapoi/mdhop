package core

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

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
		if _, err := rewriteTxExec(tx, "UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
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
			if _, err := rewriteTxExec(tx, "UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
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
