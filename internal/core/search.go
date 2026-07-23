package core

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// SearchOptions controls search behavior.
type SearchOptions struct {
	Fields      []string       // nil/empty = all (but meta is opt-in)
	Where       *WhereClause   // nil = no meta filtering
	Exclude     *ExcludeFilter // nil = no path exclusion
	Path        []string       // glob patterns for path inclusion filter
	Sort        string         // "key" = asc, "-key" = desc, "" = path order
	Limit       int            // 0 = unlimited
	Offset      int
	Sample      int // 0 = disabled
	Count       bool
	IncludeHead int // 0 = skip
	NoTags      bool
	NoOutgoing  bool
	NoIncoming  bool
}

// SearchResultItem represents a single matched node.
type SearchResultItem struct {
	Node          NodeInfo
	Lines         int       // note line count (whole file, frontmatter included)
	OutgoingCount int       // number of outgoing edges
	IncomingCount int       // number of incoming edges
	Meta          []MetaRow // nil if not requested
	Head          []string  // nil if not requested
}

// SearchResult contains the search results.
type SearchResult struct {
	Items []SearchResultItem
	Total int // total count before limit/offset
}

// Computed search field names. These are derived at query time and are valid
// in both --fields and --sort.
const (
	FieldLines         = "lines"
	FieldOutgoingCount = "outgoing_count"
	FieldIncomingCount = "incoming_count"
)

// wantMetaFields reports whether any meta output was requested via "meta"
// (all keys) or "meta.<key>" (a specific key).
func wantMetaFields(fields []string) bool {
	for _, f := range fields {
		if f == "meta" || strings.HasPrefix(f, "meta.") {
			return true
		}
	}
	return false
}

// computedSortColumn maps a computed sort key to its SQL expression in the main
// search query. Returns ("", false) for non-computed keys (handled as meta).
func computedSortColumn(key string) (string, bool) {
	switch key {
	case FieldLines:
		return "COALESCE(n.lines,0)", true
	case FieldOutgoingCount:
		return "(SELECT COUNT(*) FROM edges e_sort WHERE e_sort.source_id = n.id)", true
	case FieldIncomingCount:
		return "(SELECT COUNT(*) FROM edges e_sort WHERE e_sort.target_id = n.id)", true
	default:
		return "", false
	}
}

// parseSortKey parses a sort string into key and direction.
// "" returns ("", false, nil) meaning no sort.
// "-key" returns ("key", true, nil).
// "key" returns ("key", false, nil).
func parseSortKey(sort string) (string, bool, error) {
	if sort == "" {
		return "", false, nil
	}
	desc := false
	key := sort
	if strings.HasPrefix(sort, "-") {
		desc = true
		key = sort[1:]
	}
	if key == "" {
		return "", false, fmt.Errorf("empty sort key")
	}
	return key, desc, nil
}

// Search returns notes matching the given conditions.
func Search(vaultPath string, opts SearchOptions) (*SearchResult, error) {
	if opts.Limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	if opts.Offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}
	if opts.Sample < 0 {
		return nil, fmt.Errorf("sample must be >= 0")
	}
	if opts.Sample > 0 && (opts.Limit > 0 || opts.Offset > 0) {
		return nil, fmt.Errorf("sample cannot be used with limit or offset")
	}
	if opts.Sample > 0 && opts.Sort != "" {
		return nil, fmt.Errorf("sample cannot be used with sort")
	}
	if opts.Count {
		if opts.Sample > 0 {
			return nil, fmt.Errorf("count cannot be used with sample")
		}
		if len(opts.Fields) > 0 {
			return nil, fmt.Errorf("count cannot be used with fields")
		}
		if opts.IncludeHead > 0 {
			return nil, fmt.Errorf("count cannot be used with include-head")
		}
		if opts.Sort != "" {
			return nil, fmt.Errorf("count cannot be used with sort")
		}
		if opts.Limit > 0 || opts.Offset > 0 {
			return nil, fmt.Errorf("count cannot be used with limit or offset")
		}
	}
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}

	sortKey, sortDesc, err := parseSortKey(opts.Sort)
	if err != nil {
		return nil, err
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Build WHERE clause.
	whereSQL := "WHERE n.type = 'note' AND n.exists_flag = 1"
	var whereArgs []any

	if opts.Exclude != nil {
		pathSQL, pathArgs := opts.Exclude.PathExcludeSQL("n.path")
		whereSQL += pathSQL
		whereArgs = append(whereArgs, pathArgs...)
	}

	inclSQL, inclArgs := pathIncludeSQL("n.path", opts.Path)
	whereSQL += inclSQL
	whereArgs = append(whereArgs, inclArgs...)

	if opts.Where != nil {
		metaSQL, metaArgs := opts.Where.MetaFilterSQL("n.id")
		whereSQL += metaSQL
		whereArgs = append(whereArgs, metaArgs...)
	}
	if opts.NoTags {
		tagTypeSQL, tagTypeArgs := tagLinkTypeSQLIn("e_no_tags.link_type")
		whereSQL += " AND NOT EXISTS (SELECT 1 FROM edges e_no_tags WHERE e_no_tags.source_id = n.id AND " + tagTypeSQL + ")"
		whereArgs = append(whereArgs, tagTypeArgs...)
	}
	if opts.NoOutgoing {
		whereSQL += " AND NOT EXISTS (SELECT 1 FROM edges e_no_outgoing WHERE e_no_outgoing.source_id = n.id)"
	}
	if opts.NoIncoming {
		whereSQL += " AND NOT EXISTS (SELECT 1 FROM edges e_no_incoming WHERE e_no_incoming.target_id = n.id)"
	}

	// Count total.
	countQuery := "SELECT COUNT(*) FROM nodes n " + whereSQL
	var total int
	if err := db.QueryRow(countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count results: %w", err)
	}
	if opts.Count {
		return &SearchResult{Total: total}, nil
	}

	// Build main query. Computed fields (lines, outgoing/incoming edge counts)
	// are always selected: lines comes straight from nodes, and the edge counts
	// are SQL aggregates over edges (the source of truth), so they never go out
	// of sync with the graph.
	const computedSelect = ", COALESCE(n.lines,0)," +
		" (SELECT COUNT(*) FROM edges e_out WHERE e_out.source_id = n.id)," +
		" (SELECT COUNT(*) FROM edges e_in WHERE e_in.target_id = n.id)"

	var joinSQL string
	var joinArgs []any
	var orderSQL string

	if opts.Sample > 0 {
		orderSQL = " ORDER BY n.path ASC"
	} else if sortKey != "" {
		if col, ok := computedSortColumn(sortKey); ok {
			dir := "ASC"
			if sortDesc {
				dir = "DESC"
			}
			orderSQL = fmt.Sprintf(" ORDER BY %s %s, n.path ASC", col, dir)
		} else {
			aggFunc := "MIN"
			if sortDesc {
				aggFunc = "MAX"
			}
			joinSQL = fmt.Sprintf(
				" LEFT JOIN (SELECT node_id, %s(sort_value) AS sort_value FROM meta WHERE key = ? GROUP BY node_id) m_sort ON m_sort.node_id = n.id",
				aggFunc,
			)
			joinArgs = []any{sortKey}

			dir := "ASC"
			if sortDesc {
				dir = "DESC"
			}
			orderSQL = fmt.Sprintf(
				" ORDER BY CASE WHEN m_sort.sort_value IS NULL THEN 1 ELSE 0 END ASC, m_sort.sort_value %s, n.path ASC",
				dir,
			)
		}
	} else {
		orderSQL = " ORDER BY n.path ASC"
	}

	// LIMIT / OFFSET
	var limitSQL string
	var limitArgs []any
	if opts.Limit > 0 {
		limitSQL = " LIMIT ? OFFSET ?"
		limitArgs = []any{opts.Limit, opts.Offset}
	} else if opts.Offset > 0 {
		limitSQL = " LIMIT -1 OFFSET ?"
		limitArgs = []any{opts.Offset}
	}

	mainQuery := "SELECT n.id, n.type, n.name, COALESCE(n.path,''), n.exists_flag" + computedSelect + " FROM nodes n" + joinSQL + " " + whereSQL + orderSQL + limitSQL

	// Combine args: joinArgs + whereArgs + limitArgs
	var mainArgs []any
	mainArgs = append(mainArgs, joinArgs...)
	mainArgs = append(mainArgs, whereArgs...)
	mainArgs = append(mainArgs, limitArgs...)

	rows, err := db.Query(mainQuery, mainArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rowData struct {
		id            int64
		node          NodeInfo
		lines         int
		outgoingCount int
		incomingCount int
	}
	var rowItems []rowData
	for rows.Next() {
		var rd rowData
		var typ NodeType
		var name, path string
		var exists int
		if err := rows.Scan(&rd.id, &typ, &name, &path, &exists, &rd.lines, &rd.outgoingCount, &rd.incomingCount); err != nil {
			return nil, err
		}
		rd.node = NodeInfo{Type: typ, Name: name, Path: path, Exists: exists == 1}
		rowItems = append(rowItems, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if opts.Sample > 0 {
		rand.Shuffle(len(rowItems), func(i, j int) {
			rowItems[i], rowItems[j] = rowItems[j], rowItems[i]
		})
		if opts.Sample < len(rowItems) {
			rowItems = rowItems[:opts.Sample]
		}
	}

	wantMeta := wantMetaFields(opts.Fields)
	wantHead := opts.IncludeHead > 0

	items := make([]SearchResultItem, len(rowItems))
	for i, rd := range rowItems {
		items[i].Node = rd.node
		items[i].Lines = rd.lines
		items[i].OutgoingCount = rd.outgoingCount
		items[i].IncomingCount = rd.incomingCount

		if wantMeta {
			meta, err := queryMetaByNode(db, rd.id)
			if err != nil {
				return nil, err
			}
			if meta == nil {
				meta = []MetaRow{}
			}
			items[i].Meta = meta
		}

		if wantHead && rd.node.Type == NodeTypeNote && rd.node.Exists {
			head, err := readHead(db, vaultPath, rd.id, opts.IncludeHead)
			if err != nil {
				return nil, err
			}
			items[i].Head = head
		}
	}

	return &SearchResult{
		Items: items,
		Total: total,
	}, nil
}
