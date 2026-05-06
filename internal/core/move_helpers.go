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

// queryCollateralRewrites finds basename links to non-moved nodes of the given type
// that need rewriting due to root-priority changes.
func queryCollateralRewrites(db dbExecer, nodeType NodeType, name string, movedNodeIDs map[int64]bool) ([]rewriteEntry, error) {
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

// rewriteOutgoingRelativeLink rewrites a relative link in the moved file
// from the old path perspective to the new path perspective.
// If movedFromTo is non-nil, it also checks whether the target was moved.
func rewriteOutgoingRelativeLink(rawLink, linkType, from, to string, movedFromTo map[string]string) (string, error) {
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

// groupAndApplyExternalRewrites groups rewrites by source path and applies them.
// Returns per-sourceID mtimes and backups for rollback.
func groupAndApplyExternalRewrites(vaultPath string, rewrites []rewriteEntry) (map[int64]int64, []rewriteBackup, error) {
	if len(rewrites) == 0 {
		return nil, nil, nil
	}
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	return applyFileRewrites(vaultPath, groups)
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
// If no phantom exists for the given name, this is a no-op.
func promotePhantom(tx dbExecer, phantomName string, realNodeID int64) error {
	pk := phantomKey(phantomName)
	var phantomID int64
	err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", realNodeID, phantomID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
		return err
	}
	return nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
