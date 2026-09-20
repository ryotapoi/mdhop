package core

import (
	"fmt"
	"sort"
	"strings"
)

func queryBacklinks(db dbExecer, targetID int64, limit int, ef *ExcludeFilter, wc *WhereClause, include []string) ([]NodeInfo, error) {
	q := `SELECT DISTINCT n.type, n.name, COALESCE(n.path,''), n.exists_flag
		 FROM edges e JOIN nodes n ON n.id = e.source_id
		 WHERE e.target_id = ?`
	args := []any{targetID}

	if ef != nil {
		pathSQL, pathArgs := ef.PathExcludeSQL("n.path")
		q += pathSQL
		args = append(args, pathArgs...)
	}

	inclSQL, inclArgs := pathIncludeNullSafeSQL("n.path", include)
	q += inclSQL
	args = append(args, inclArgs...)

	if wc != nil {
		metaSQL, metaArgs := wc.MetaFilterSQL("n.id")
		q += metaSQL
		args = append(args, metaArgs...)
	}

	q += ` ORDER BY n.path, n.name LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NodeInfo
	for rows.Next() {
		info, err := scanNodeInfo(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, rows.Err()
}

func queryOutgoing(db dbExecer, sourceID int64, ef *ExcludeFilter, wc *WhereClause, include []string) ([]NodeInfo, error) {
	q := `SELECT DISTINCT n.type, n.name, COALESCE(n.path,''), n.exists_flag
		 FROM edges e JOIN nodes n ON n.id = e.target_id
		 WHERE e.source_id = ? AND e.target_id != ? AND n.type IN ('note','phantom','asset')`
	args := []any{sourceID, sourceID}

	if ef != nil {
		pathSQL, pathArgs := ef.PathExcludeSQL("n.path")
		q += pathSQL
		args = append(args, pathArgs...)
	}

	inclSQL, inclArgs := pathIncludeNullSafeSQL("n.path", include)
	q += inclSQL
	args = append(args, inclArgs...)

	if wc != nil {
		metaSQL, metaArgs := wc.MetaFilterSQL("n.id")
		q += metaSQL
		args = append(args, metaArgs...)
	}

	q += ` ORDER BY n.path, n.name`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NodeInfo
	for rows.Next() {
		info, err := scanNodeInfo(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, rows.Err()
}

func queryTags(db dbExecer, sourceID int64, ef *ExcludeFilter) ([]string, error) {
	q := `SELECT DISTINCT n.name FROM edges e JOIN nodes n ON n.id = e.target_id
		 WHERE e.source_id = ? AND n.type = 'tag'`
	args := []any{sourceID}

	if ef != nil {
		tagSQL, tagArgs := ef.TagExcludeSQL("n.name")
		q += tagSQL
		args = append(args, tagArgs...)
	}

	q += ` ORDER BY n.name`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		all = append(all, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return filterLeafTags(all), nil
}

func filterLeafTags(tags []string) []string {
	if len(tags) <= 1 {
		return tags
	}
	sorted := make([]string, len(tags))
	copy(sorted, tags)
	sort.Strings(sorted)
	var leaves []string
	for _, t := range sorted {
		prefix := t + "/"
		idx := sort.SearchStrings(sorted, prefix)
		// If idx < len(sorted) and sorted[idx] starts with prefix, t has a descendant.
		if idx < len(sorted) && strings.HasPrefix(sorted[idx], prefix) {
			continue
		}
		leaves = append(leaves, t)
	}
	return leaves
}

func fetchNodeInfoBatch(db dbExecer, ids []int64) (map[int64]NodeInfo, error) {
	if len(ids) == 0 {
		return map[int64]NodeInfo{}, nil
	}

	result := make(map[int64]NodeInfo, len(ids))
	const chunkSize = 500

	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		placeholders := strings.Repeat("?,", len(chunk))
		placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}

		rows, err := db.Query(
			fmt.Sprintf(`SELECT id, type, name, COALESCE(path,''), exists_flag FROM nodes WHERE id IN (%s)`, placeholders),
			args...,
		)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			id, info, err := scanNodeInfoWithID(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			result[id] = info
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func queryTwoHop(db dbExecer, entryID int64, entryType NodeType, maxTwoHop, maxViaPerTarget int, ef *ExcludeFilter, wc *WhereClause, include []string) ([]TwoHopEntry, error) {
	var seedQuery string
	var seedIsOutbound bool

	switch entryType {
	case NodeTypeNote:
		// Outbound seed: targets of the entry.
		seedQuery = `SELECT DISTINCT target_id FROM edges WHERE source_id = ?`
		seedIsOutbound = true
	default:
		// Inbound seed: sources linking to the entry.
		seedQuery = `SELECT DISTINCT source_id FROM edges WHERE target_id = ?`
		seedIsOutbound = false
	}

	seedRows, err := db.Query(seedQuery, entryID)
	if err != nil {
		return nil, err
	}
	defer seedRows.Close()

	var seedIDs []int64
	for seedRows.Next() {
		var id int64
		if err := seedRows.Scan(&id); err != nil {
			return nil, err
		}
		seedIDs = append(seedIDs, id)
	}
	if err := seedRows.Err(); err != nil {
		return nil, err
	}

	viaInfoMap, err := fetchNodeInfoBatch(db, seedIDs)
	if err != nil {
		return nil, err
	}

	var wcSQL string
	var wcArgs []any
	if wc != nil {
		wcSQL, wcArgs = wc.MetaFilterSQL("n.id")
	}

	var entries []TwoHopEntry
	for _, viaID := range seedIDs {
		if len(entries) >= maxTwoHop {
			break
		}

		viaInfo, ok := viaInfoMap[viaID]
		if !ok {
			return nil, fmt.Errorf("node not found in batch: id=%d", viaID)
		}

		if ef != nil && ef.IsViaExcluded(viaInfo) {
			continue
		}

		var targetQuery string
		var targetArgs []any
		if seedIsOutbound {
			targetQuery = `SELECT DISTINCT n.type, n.name, COALESCE(n.path,''), n.exists_flag
				 FROM edges e JOIN nodes n ON n.id = e.source_id
				 WHERE e.target_id = ? AND e.source_id != ?`
			targetArgs = []any{viaID, entryID}
		} else {
			targetQuery = `SELECT DISTINCT n.type, n.name, COALESCE(n.path,''), n.exists_flag
				 FROM edges e JOIN nodes n ON n.id = e.target_id
				 WHERE e.source_id = ? AND e.target_id != ?`
			targetArgs = []any{viaID, entryID}
		}

		if ef != nil {
			pathSQL, pathArgs := ef.PathExcludeSQL("n.path")
			targetQuery += pathSQL
			targetArgs = append(targetArgs, pathArgs...)
		}

		// Include filter applies to targets only; via nodes are kept as
		// connectors even when outside the included paths.
		inclSQL, inclArgs := pathIncludeNullSafeSQL("n.path", include)
		targetQuery += inclSQL
		targetArgs = append(targetArgs, inclArgs...)

		if wcSQL != "" {
			targetQuery += wcSQL
			targetArgs = append(targetArgs, wcArgs...)
		}

		targetQuery += ` ORDER BY n.path, n.name LIMIT ?`
		targetArgs = append(targetArgs, maxViaPerTarget)

		targetRows, err := db.Query(targetQuery, targetArgs...)
		if err != nil {
			return nil, err
		}

		var targets []NodeInfo
		for targetRows.Next() {
			info, err := scanNodeInfo(targetRows)
			if err != nil {
				targetRows.Close()
				return nil, err
			}
			targets = append(targets, info)
		}
		targetRows.Close()
		if err := targetRows.Err(); err != nil {
			return nil, err
		}

		if len(targets) > 0 {
			entries = append(entries, TwoHopEntry{Via: viaInfo, Targets: targets})
		}
	}

	return entries, nil
}

type contentSource struct {
	path  string
	mtime int64
}

type snippetSource struct {
	contentSource
	lineStart int
	lineEnd   int
}

func queryHeadSource(db dbExecer, nodeID int64) (contentSource, error) {
	var source contentSource
	err := db.QueryRow(
		`SELECT path, mtime FROM nodes WHERE id = ?`,
		nodeID,
	).Scan(&source.path, &source.mtime)
	if err != nil {
		return contentSource{}, err
	}
	return source, nil
}

func querySnippetSources(db dbExecer, targetID int64, ef *ExcludeFilter, include []string) ([]snippetSource, error) {
	q := `SELECT n.path, n.mtime, e.line_start, e.line_end
		 FROM edges e JOIN nodes n ON n.id = e.source_id
		 WHERE e.target_id = ?`
	args := []any{targetID}

	if ef != nil {
		pathSQL, pathArgs := ef.PathExcludeSQL("n.path")
		q += pathSQL
		args = append(args, pathArgs...)
	}

	inclSQL, inclArgs := pathIncludeNullSafeSQL("n.path", include)
	q += inclSQL
	args = append(args, inclArgs...)

	q += ` ORDER BY n.path, e.line_start`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []snippetSource
	for rows.Next() {
		var source snippetSource
		if err := rows.Scan(&source.path, &source.mtime, &source.lineStart, &source.lineEnd); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sources, nil
}
