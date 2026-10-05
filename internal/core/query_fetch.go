package core

import (
	"container/heap"
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

func queryDirect(db dbExecer, entryID int64, relation string, opts QueryOptions) ([]QueryNode, *int, error) {
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
	nodes, next, err := queryNodePage(db, q, args, opts)
	if err != nil {
		return nil, nil, err
	}
	result := make([]QueryNode, len(nodes))
	for i, n := range nodes {
		result[i] = QueryNode{NodeInfo: n.info, id: n.id}
	}
	return result, next, nil
}

func queryTags(db dbExecer, sourceID int64) ([]string, error) {
	q := `SELECT DISTINCT n.name FROM edges e JOIN nodes n ON n.id = e.target_id
		 WHERE e.source_id = ? AND n.type = 'tag' ORDER BY n.name`

	rows, err := db.Query(q, sourceID)
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

func queryTwoHop(db dbExecer, entryID int64, opts QueryOptions) ([]TwoHopEntry, *int, error) {
	via, err := queryViaNodes(db, entryID, opts.Filter)
	if err != nil {
		return nil, nil, err
	}
	if len(via) == 0 {
		return []TwoHopEntry{}, nil, nil
	}
	// These literals are integer IDs read from the index, avoiding SQLite
	// parameter limits for entries with many shared destinations.
	ids := make([]string, 0, len(via))
	for id := range via {
		ids = append(ids, fmt.Sprint(id))
	}
	from := ` FROM edges e JOIN nodes n ON n.id = e.source_id
 WHERE n.id != ? AND e.target_id IN (` + strings.Join(ids, ",") + `)`
	args := []any{entryID}
	condition, values := queryTargetSQL(opts)
	from += condition
	args = append(args, values...)
	q := `SELECT DISTINCT n.id,n.type,n.name,COALESCE(n.path,''),n.exists_flag,e.target_id` + from
	var next *int
	if opts.Limit != nil || opts.Offset != nil {
		// Acquire distinct targets before expanding relations. The lookahead
		// target and targets outside the page never get relation arrays.
		targetQuery := `SELECT DISTINCT n.id,n.type,n.name,COALESCE(n.path,''),n.exists_flag` + from
		nodes, pageNext, err := queryNodePage(db, targetQuery, args, opts)
		if err != nil {
			return nil, nil, err
		}
		next = pageNext
		if len(nodes) == 0 {
			return []TwoHopEntry{}, next, nil
		}
		targetIDs := make([]string, len(nodes))
		for i, n := range nodes {
			targetIDs[i] = fmt.Sprint(n.id)
		}
		q += ` AND n.id IN (` + strings.Join(targetIDs, ",") + `)`
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	type target struct {
		node   queryNode
		via    []queryNode
		hidden bool
	}
	targets := make(map[int64]*target)
	for rows.Next() {
		var n queryNode
		var viaID int64
		var ne int
		if err := rows.Scan(&n.id, &n.info.Type, &n.info.Name, &n.info.Path, &ne, &viaID); err != nil {
			return nil, nil, err
		}
		n.info.Exists = ne == 1
		v := via[viaID]
		if opts.Filter.IsHidden(n.info) {
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
		return nil, nil, err
	}
	ordered := make([]*target, 0, len(targets))
	for _, t := range targets {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool { return queryNodeLess(ordered[i].node, ordered[j].node) })
	result := make([]TwoHopEntry, len(ordered))
	for i, t := range ordered {
		sort.Slice(t.via, func(i, j int) bool { return queryNodeLess(t.via[i], t.via[j]) })
		via := make([]QueryNode, len(t.via))
		for j, v := range t.via {
			via[j] = QueryNode{NodeInfo: v.info, id: v.id}
		}
		result[i] = TwoHopEntry{QueryNode: QueryNode{NodeInfo: t.node.info, id: t.node.id}, Relation: via, HiddenRelation: t.hidden}
	}
	return result, next, nil
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

// queryViaNodes applies the shared Go predicates before expanding any backlinks.
func queryViaNodes(db dbExecer, entryID int64, filter *QueryFilter) (map[int64]queryNode, error) {
	rows, err := db.Query(`SELECT DISTINCT n.id,n.type,n.name,COALESCE(n.path,''),n.exists_flag FROM edges e JOIN nodes n ON n.id=e.target_id WHERE e.source_id=?`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	via := make(map[int64]queryNode)
	for rows.Next() {
		id, info, err := scanNodeInfoWithID(rows)
		if err != nil {
			return nil, err
		}
		if filter.AllowsVia(info) {
			via[id] = queryNode{id, info}
		}
	}
	return via, rows.Err()
}

func queryOffset(opts QueryOptions) int {
	if opts.Offset != nil {
		return *opts.Offset
	}
	return 0
}

// Zero means unbounded; guard the lookahead sum against integer overflow.
func queryPageCapacity(opts QueryOptions) int {
	if opts.Limit == nil {
		return 0
	}
	offset := queryOffset(opts)
	maxInt := int(^uint(0) >> 1)
	if offset >= maxInt-*opts.Limit {
		return 0
	}
	return offset + *opts.Limit + 1
}

// A max-heap retains only the earliest offset + limit + lookahead nodes.
// Both acquisition and final sorting use the canonical node comparator.
type queryNodeHeap []queryNode

func (h queryNodeHeap) Len() int           { return len(h) }
func (h queryNodeHeap) Less(i, j int) bool { return queryNodeLess(h[j], h[i]) }
func (h queryNodeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *queryNodeHeap) Push(v any)        { *h = append(*h, v.(queryNode)) }
func (h *queryNodeHeap) Pop() any          { last := len(*h) - 1; v := (*h)[last]; *h = (*h)[:last]; return v }
func (h *queryNodeHeap) add(n queryNode, capacity int) {
	if capacity == 0 {
		*h = append(*h, n)
		return
	}
	if len(*h) < capacity {
		heap.Push(h, n)
	} else if queryNodeLess(n, (*h)[0]) {
		(*h)[0] = n
		heap.Fix(h, 0)
	}
}

func queryNodePage(db dbExecer, q string, args []any, opts QueryOptions) ([]queryNode, *int, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	nodes := queryNodeHeap{}
	capacity := queryPageCapacity(opts)
	for rows.Next() {
		id, info, err := scanNodeInfoWithID(rows)
		if err != nil {
			return nil, nil, err
		}
		if !opts.Filter.IsHidden(info) {
			nodes.add(queryNode{id, info}, capacity)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	sort.Slice(nodes, func(i, j int) bool { return queryNodeLess(nodes[i], nodes[j]) })
	start, end, next := queryPageBounds(len(nodes), queryOffset(opts), opts.Limit)
	return nodes[start:end], next, nil
}
