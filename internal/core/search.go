package core

import (
	"fmt"
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
	IncludeHead int // 0 = skip
}

// SearchResultItem represents a single matched node.
type SearchResultItem struct {
	Node NodeInfo
	Meta []MetaRow // nil if not requested
	Head []string  // nil if not requested
}

// SearchResult contains the search results.
type SearchResult struct {
	Items []SearchResultItem
	Total int // total count before limit/offset
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
		return "", false, fmt.Errorf("search: empty sort key")
	}
	return key, desc, nil
}

// Search returns notes matching the given conditions.
func Search(vaultPath string, opts SearchOptions) (*SearchResult, error) {
	if opts.Limit < 0 {
		return nil, fmt.Errorf("search: limit must be >= 0")
	}
	if opts.Offset < 0 {
		return nil, fmt.Errorf("search: offset must be >= 0")
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

	// Count total.
	countQuery := "SELECT COUNT(*) FROM nodes n " + whereSQL
	var total int
	if err := db.QueryRow(countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("search count: %w", err)
	}

	// Build main query.
	var joinSQL string
	var joinArgs []any
	var orderSQL string

	if sortKey != "" {
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

	mainQuery := "SELECT n.id, n.type, n.name, COALESCE(n.path,''), n.exists_flag FROM nodes n" + joinSQL + " " + whereSQL + orderSQL + limitSQL

	// Combine args: joinArgs + whereArgs + limitArgs
	var mainArgs []any
	mainArgs = append(mainArgs, joinArgs...)
	mainArgs = append(mainArgs, whereArgs...)
	mainArgs = append(mainArgs, limitArgs...)

	rows, err := db.Query(mainQuery, mainArgs...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	type rowData struct {
		id   int64
		node NodeInfo
	}
	var rowItems []rowData
	for rows.Next() {
		var rd rowData
		var typ, name, path string
		var exists int
		if err := rows.Scan(&rd.id, &typ, &name, &path, &exists); err != nil {
			return nil, err
		}
		rd.node = NodeInfo{Type: typ, Name: name, Path: path, Exists: exists == 1}
		rowItems = append(rowItems, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	wantMeta := isFieldActive("meta", opts.Fields) && len(opts.Fields) > 0
	wantHead := opts.IncludeHead > 0

	items := make([]SearchResultItem, len(rowItems))
	for i, rd := range rowItems {
		items[i].Node = rd.node

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

// pathIncludeSQL generates a SQL fragment for path inclusion filtering.
// Returns ("", nil) for empty patterns.
func pathIncludeSQL(alias string, patterns []string) (string, []any) {
	if len(patterns) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, p := range patterns {
		parts = append(parts, alias+" GLOB ?")
		args = append(args, p)
	}
	return " AND (" + strings.Join(parts, " OR ") + ")", args
}
