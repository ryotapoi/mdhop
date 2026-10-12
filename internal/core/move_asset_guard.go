package core

import "fmt"

// validateMoveAssetEdges checks the planned link syntax against the final batch
// maps. Note priority can hide an asset even when neither its path nor basename
// changes, so checking collisions within each namespace is insufficient.
func validateMoveAssetEdges(db dbExecer, dm *dirMoveMaps, external []rewriteEntry, moved []movedFileRewrite) error {
	externalRaw := make(map[int64]string, len(external))
	for _, re := range external {
		externalRaw[re.edgeID] = re.newRawLink
	}
	movedRewrites := make(map[string][]outgoingRewrite, len(moved))
	for _, file := range moved {
		movedRewrites[file.move.from] = file.outRewrites
	}
	linkTypeSQL, args := linkTypeSQLIn("e.link_type", rewriteLinkTypes)
	rows, err := db.Query(fmt.Sprintf(`SELECT e.id, e.raw_link, e.link_type, e.line_start, e.in_table, sn.path, tn.path
		FROM edges e
		JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		JOIN nodes tn ON tn.id = e.target_id AND tn.type = ? AND tn.exists_flag = 1
		WHERE %s ORDER BY e.id`, linkTypeSQL), append([]any{NodeTypeAsset}, args...)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var raw, source, expected string
		var typ LinkType
		var line int
		var inTable bool
		if err := rows.Scan(&id, &raw, &typ, &line, &inTable, &source, &expected); err != nil {
			return err
		}
		if replacement, ok := externalRaw[id]; ok {
			raw = replacement
		}
		for _, re := range movedRewrites[source] {
			if re.rawLink == raw && re.lineStart == line && re.linkType == typ {
				raw = re.newRawLink
				break
			}
		}
		if to, ok := dm.movedFromTo[source]; ok {
			source = to
		}
		if to, ok := dm.movedFromTo[expected]; ok {
			expected = to
		}
		var links []linkOccur
		switch typ {
		case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
			links = parseWikiLinks(raw, line, inTable)
		case LinkTypeMarkdown:
			links = parseMarkdownLinks(raw, line)
		}
		if len(links) != 1 {
			return fmt.Errorf("cannot evaluate asset link %q in %s after move", raw, source)
		}
		link := links[0]
		link.linkType = typ
		if err := validateParsedLink(source, link, dm.rm); err != nil {
			return err
		}
		resolved, _, err := resolveLinkWithBackend(source, link, dryLinkResolver{rm: dm.rm})
		if err != nil {
			return err
		}
		if resolved != expected {
			return fmt.Errorf("asset link %q in %s would no longer resolve to %s after move", raw, source, expected)
		}
	}
	return rows.Err()
}
