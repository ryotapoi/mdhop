package core

import (
	"fmt"
	"sort"
	"strings"
)

type queryNode struct {
	id   int64
	info NodeInfo
}

func queryNodeLess(a, b queryNode) bool {
	ap, bp := normalizeTextNFC(a.info.Path), normalizeTextNFC(b.info.Path)
	if (ap != "") != (bp != "") {
		return ap != ""
	}
	if ap != bp {
		return ap < bp
	}
	if ap == "" {
		if a.info.Type != b.info.Type {
			return a.info.Type < b.info.Type
		}
		an, bn := normalizeTextNFC(a.info.Name), normalizeTextNFC(b.info.Name)
		if an != bn {
			return an < bn
		}
	}
	return a.id < b.id
}

func queryTargetSQL(opts QueryOptions) (string, []any) {
	q, args := pathIncludeNullSafeSQL("n.path", opts.Path)
	if opts.Where != nil {
		q += " AND n.type = 'note'"
		where, values := opts.Where.MetaFilterSQL("n.id")
		q += where
		args = append(args, values...)
	}
	return q, args
}

func queryDirect(db dbExecer, entryID int64, relation string, opts QueryOptions) ([]NodeInfo, error) {
	q := `SELECT DISTINCT n.id, n.type, n.name, COALESCE(n.path,''), n.exists_flag FROM edges e JOIN nodes n ON n.id = e.source_id WHERE e.target_id = ? AND n.id != ?`
	if relation == FieldQueryOutgoing {
		q = `SELECT DISTINCT n.id, n.type, n.name, COALESCE(n.path,''), n.exists_flag FROM edges e JOIN nodes n ON n.id = e.target_id WHERE e.source_id = ? AND n.id != ?`
	}
	args := []any{entryID, entryID}
	if opts.LinkKey != "" {
		q += " AND e.frontmatter_key = ?"
		args = append(args, opts.LinkKey)
	}
	condition, values := queryTargetSQL(opts)
	q += condition
	args = append(args, values...)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []queryNode{}
	for rows.Next() {
		id, info, err := scanNodeInfoWithID(rows)
		if err != nil {
			return nil, err
		}
		if !opts.Filter.IsHidden(info) {
			nodes = append(nodes, queryNode{id, info})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(nodes, func(i, j int) bool { return queryNodeLess(nodes[i], nodes[j]) })
	result := make([]NodeInfo, len(nodes))
	for i, n := range nodes {
		result[i] = n.info
	}
	return result, nil
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

func queryTwoHop(db dbExecer, entryID int64, opts QueryOptions) ([]TwoHopEntry, error) {
	q := `SELECT DISTINCT n.id,n.type,n.name,COALESCE(n.path,''),n.exists_flag,
 v.id,v.type,v.name,COALESCE(v.path,''),v.exists_flag
 FROM edges seed JOIN nodes v ON v.id = seed.target_id
 JOIN edges e ON e.target_id = v.id JOIN nodes n ON n.id = e.source_id
 WHERE seed.source_id = ? AND n.id != ?`
	args := []any{entryID, entryID}
	condition, values := queryTargetSQL(opts)
	q += condition
	args = append(args, values...)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type target struct {
		node   queryNode
		via    []queryNode
		hidden bool
	}
	targets := make(map[int64]*target)
	for rows.Next() {
		var n, v queryNode
		var ne, ve int
		if err := rows.Scan(&n.id, &n.info.Type, &n.info.Name, &n.info.Path, &ne, &v.id, &v.info.Type, &v.info.Name, &v.info.Path, &ve); err != nil {
			return nil, err
		}
		n.info.Exists = ne == 1
		v.info.Exists = ve == 1
		if !opts.Filter.AllowsVia(v.info) || opts.Filter.IsHidden(n.info) {
			continue
		}
		t := targets[n.id]
		if t == nil {
			t = &target{node: n}
			targets[n.id] = t
		}
		if opts.Filter.IsHidden(v.info) {
			t.hidden = true
		} else {
			t.via = append(t.via, v)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ordered := make([]*target, 0, len(targets))
	for _, t := range targets {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool { return queryNodeLess(ordered[i].node, ordered[j].node) })
	result := make([]TwoHopEntry, len(ordered))
	for i, t := range ordered {
		sort.Slice(t.via, func(i, j int) bool { return queryNodeLess(t.via[i], t.via[j]) })
		via := make([]NodeInfo, len(t.via))
		for j, v := range t.via {
			via[j] = v.info
		}
		result[i] = TwoHopEntry{NodeInfo: t.node.info, Relation: via, HiddenRelation: t.hidden}
	}
	return result, nil
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
