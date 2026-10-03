package core

import (
	"database/sql"
	"errors"
	"os"
)

// prepareMovedFileRewrites validates every moved-note candidate before disk
// writes begin. The candidate remains attached for both writing and DB reparse.
func prepareMovedFileRewrites(vaultPath string, movedFileRewrites []movedFileRewrite, needDiskMove bool) error {
	diskPaths := newVaultDiskPathResolver(vaultPath)
	for i, mfr := range movedFileRewrites {
		if len(mfr.outRewrites) == 0 {
			continue
		}
		path := mfr.move.to
		if needDiskMove {
			path = mfr.move.from
		}
		if _, err := diskPaths.writablePath(path); err != nil {
			return err
		}
		rewrites := make([]rewriteEntry, 0, len(mfr.outRewrites))
		for _, ow := range mfr.outRewrites {
			rewrites = append(rewrites, rewriteEntry{rawLink: ow.rawLink, newRawLink: ow.newRawLink, linkType: ow.linkType, lineStart: ow.lineStart})
		}
		candidate, err := rewriteContentCandidate(mfr.content, rewrites)
		if err != nil {
			return err
		}
		movedFileRewrites[i].content = candidate
	}
	return nil
}

// applyMovedFileRewrites writes outgoing rewrites to moved files and returns
// backups for later rollback. On write failure, already-written moved files are
// restored best-effort.
func applyMovedFileRewrites(vaultPath string, movedFileRewrites []movedFileRewrite, needDiskMove bool) ([]rewriteBackup, []rollbackFailure, error) {
	diskPaths := newVaultDiskPathResolver(vaultPath)
	fullPaths := make(map[string]string)
	for _, mfr := range movedFileRewrites {
		if len(mfr.outRewrites) == 0 {
			continue
		}
		path := mfr.move.to
		if needDiskMove {
			path = mfr.move.from
		}
		fullPath, err := diskPaths.writablePath(path)
		if err != nil {
			return nil, nil, err
		}
		fullPaths[path] = fullPath
	}
	var backups []rewriteBackup
	for _, mfr := range movedFileRewrites {
		if len(mfr.outRewrites) == 0 {
			continue
		}
		m := mfr.move
		diskPath := m.to
		if needDiskMove {
			diskPath = m.from
		}

		fullPath := fullPaths[diskPath]
		backups = append(backups, rewriteBackup{path: diskPath, content: mfr.original, perm: mfr.perm, mtime: mfr.mtime})
		if err := writeFilePreservePerm(fullPath, mfr.content, mfr.perm); err != nil {
			restoreFailures := restoreBackupFiles(vaultPath, backups)
			return backups, restoreFailures, err
		}
	}
	return backups, nil, nil
}

// updateExternalEdgesAndMtimes updates edge raw_links and source node mtimes
// and reparses metadata for externally rewritten files. Returns rewritten links.
func updateExternalEdgesAndMtimes(tx dbExecer, vaultPath string, metaCfg MetaConfig, rewrites []rewriteEntry, mtimes map[int64]int64) ([]RewrittenLink, error) {
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
		diskPaths := newVaultDiskPathResolver(vaultPath)
		mtimeUpdated := make(map[int64]bool)
		for _, re := range rewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			fullPath, err := diskPaths.existingPath(re.sourcePath)
			if err != nil {
				return nil, err
			}
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			if err := deleteMetaByNode(tx, re.sourceID); err != nil {
				return nil, err
			}
			if _, err := insertMetaEntries(tx, re.sourceID, re.sourcePath, parseLinksWithLinkKeys(string(content), metaCfg.LinkKeys).Meta, metaCfg); err != nil {
				return nil, err
			}
			mt := mtimes[re.sourceID]
			if _, err := rewriteTxExec(tx, "UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// promotePhantom reassigns only edges that resolve to the real node in the
// post-operation maps. Unresolved edges retain the shared phantom. The bool
// reports whether at least one edge was promoted, including partial promotion.
func promotePhantom(tx dbExecer, phantomName string, realNodeID int64, realPath string, rm *resolveMaps) (bool, error) {
	var phantomID int64
	err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey(phantomName)).Scan(&phantomID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	rows, err := tx.Query(`SELECT e.id, e.raw_link, e.link_type, sn.path, COALESCE(e.reference_target,''), e.in_table
		FROM edges e JOIN nodes sn ON sn.id = e.source_id
		WHERE e.target_id = ?`, phantomID)
	if err != nil {
		return false, err
	}
	type phantomEdge struct {
		id              int64
		rawLink         string
		linkType        LinkType
		sourcePath      string
		referenceTarget string
		inTable         bool
	}
	var edges []phantomEdge
	for rows.Next() {
		var e phantomEdge
		if err := rows.Scan(&e.id, &e.rawLink, &e.linkType, &e.sourcePath, &e.referenceTarget, &e.inTable); err != nil {
			rows.Close()
			return false, err
		}
		edges = append(edges, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	promoted := 0
	for _, e := range edges {
		var links []linkOccur
		switch e.linkType {
		case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
			links = parseWikiLinks(e.rawLink, 0, e.inTable)
		case LinkTypeMarkdown:
			links = parseMarkdownLinks(e.rawLink, 0)
		case LinkTypeMarkdownReference:
			if occ, ok := markdownDestinationOccur(e.referenceTarget, e.rawLink, e.linkType, 0); ok {
				links = []linkOccur{occ}
			}
		case LinkTypeFrontmatterPath:
			if occ, ok := frontmatterPathOccur(e.rawLink, 0); ok {
				links = []linkOccur{occ}
			}
		}
		if len(links) != 1 {
			continue
		}
		link := links[0]
		link.linkType = e.linkType
		resolved, _, err := resolveLinkWithBackend(e.sourcePath, link, dryLinkResolver{rm: rm})
		if err != nil || resolved != realPath {
			continue
		}
		if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE id = ?", realNodeID, e.id); err != nil {
			return false, err
		}
		promoted++
	}
	if promoted == len(edges) {
		if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
			return false, err
		}
	}
	return promoted > 0, nil
}
