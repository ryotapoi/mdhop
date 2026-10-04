package core

// addQueryPreviews runs after filtering, hiding, sorting, and paging.
func addQueryPreviews(db dbExecer, vault string, entryID int64, opts QueryOptions, result *QueryResult) error {
	addHead := func(node *QueryNode) error {
		if opts.IncludeHead == nil || node.Type != NodeTypeNote {
			return nil
		}
		source, err := queryHeadSource(db, node.id)
		if err != nil {
			return err
		}
		node.Head, err = readHead(vault, source, *opts.IncludeHead)
		if err == nil && node.Head == nil {
			node.Head = []string{}
		}
		return err
	}
	addSnippet := func(node *QueryNode, sourceID, targetID int64, key string) error {
		if opts.IncludeSnippet == nil {
			return nil
		}
		sources, err := queryRelationSnippetSources(db, sourceID, targetID, key)
		if err != nil {
			return err
		}
		node.Snippet, err = readSnippets(vault, sources, *opts.IncludeSnippet)
		if err == nil && node.Snippet == nil {
			node.Snippet = []SnippetEntry{}
		}
		return err
	}
	for i := range result.Backlinks {
		node := &result.Backlinks[i]
		if err := addHead(node); err != nil {
			return err
		}
		if err := addSnippet(node, node.id, entryID, opts.LinkKey); err != nil {
			return err
		}
	}
	for i := range result.Outgoing {
		node := &result.Outgoing[i]
		if err := addHead(node); err != nil {
			return err
		}
		if err := addSnippet(node, entryID, node.id, opts.LinkKey); err != nil {
			return err
		}
	}
	for i := range result.TwoHop {
		target := &result.TwoHop[i]
		if err := addHead(&target.QueryNode); err != nil {
			return err
		}
		for j := range target.Relation {
			via := &target.Relation[j]
			if err := addSnippet(via, target.id, via.id, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// Selecting by both indexed endpoints avoids seed multiplication and unrelated edges.
func queryRelationSnippetSources(db dbExecer, sourceID, targetID int64, key string) ([]snippetSource, error) {
	q := `SELECT n.path,n.mtime,e.line_start,e.line_end FROM edges e
 JOIN nodes n ON n.id=e.source_id WHERE e.source_id=? AND e.target_id=?`
	args := []any{sourceID, targetID}
	if key != "" {
		q += " AND e.frontmatter_key=?"
		args = append(args, key)
	}
	q += " ORDER BY n.path,e.line_start,e.id"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := []snippetSource{}
	for rows.Next() {
		var source snippetSource
		if err := rows.Scan(&source.path, &source.mtime, &source.lineStart, &source.lineEnd); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
