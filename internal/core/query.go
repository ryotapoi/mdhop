package core

import (
	"database/sql"
	"fmt"
)

// EntrySpec specifies the entry node for a query.
type EntrySpec struct {
	File    string // vault-relative path
	Tag     string // tag name (# optional)
	Phantom string // phantom name
	Name    string // auto-detect: #tag → tag, otherwise note → phantom
}

// QueryOptions selects relations and filters their targets. Nil Relations selects all.
// Page arguments require an explicitly selected single relation.
type QueryOptions struct {
	Relations      []string
	Filter         *QueryFilter
	Limit          *int
	Offset         *int
	Where          *WhereClause
	LinkKey        string
	Path           []string
	IncludeHead    *int
	IncludeSnippet *int
}

const (
	FieldQueryBacklinks = "backlinks"
	FieldQueryTwoHop    = "twohop"
	FieldQueryOutgoing  = "outgoing"
)

// NodeInfo describes a node in the graph.
type NodeInfo struct {
	Type   NodeType
	Name   string
	Path   string // note/asset only
	Exists bool
}

// scanNodeInfo scans a (type, name, path, exists_flag) row into a NodeInfo.
func scanNodeInfo(rows *sql.Rows) (NodeInfo, error) {
	var typ NodeType
	var name, path string
	var exists int
	if err := rows.Scan(&typ, &name, &path, &exists); err != nil {
		return NodeInfo{}, err
	}
	return NodeInfo{Type: typ, Name: name, Path: path, Exists: exists == 1}, nil
}

// scanNodeInfoWithID scans an (id, type, name, path, exists_flag) row into a
// node id and a NodeInfo.
func scanNodeInfoWithID(rows *sql.Rows) (int64, NodeInfo, error) {
	var id int64
	var typ NodeType
	var name, path string
	var exists int
	if err := rows.Scan(&id, &typ, &name, &path, &exists); err != nil {
		return 0, NodeInfo{}, err
	}
	return id, NodeInfo{Type: typ, Name: name, Path: path, Exists: exists == 1}, nil
}

// QueryNode carries previews belonging to one returned target or via.
type QueryNode struct {
	NodeInfo
	Head    []string
	Snippet []SnippetEntry
	id      int64
}

// TwoHopEntry is a target and all visible shared outgoing destinations.
type TwoHopEntry struct {
	QueryNode
	Relation       []QueryNode
	HiddenRelation bool
}

// QueryPage describes the returned target page without a total count.
type QueryPage struct {
	Offset     int
	Limit      *int
	NextOffset *int
}

// SnippetEntry represents lines surrounding a link occurrence in a source file.
type SnippetEntry struct {
	SourcePath string
	LineStart  int
	LineEnd    int
	Lines      []string
}

// QueryResult keeps unselected relations nil and selected empty relations non-nil.
type QueryResult struct {
	Entry     NodeInfo
	Backlinks []QueryNode
	Outgoing  []QueryNode
	TwoHop    []TwoHopEntry
	Page      QueryPage
}

// Query reads previews only for returned relations when requested.
func Query(vaultPath string, entry EntrySpec, opts QueryOptions) (*QueryResult, error) {
	if err := validateQueryOptions(opts); err != nil {
		return nil, err
	}
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	nodeID, info, err := findEntryNode(db, entry)
	if err != nil {
		return nil, err
	}
	result := &QueryResult{Entry: info, Page: QueryPage{Limit: opts.Limit}}
	if opts.Offset != nil {
		result.Page.Offset = *opts.Offset
	}
	relations := opts.Relations
	if relations == nil {
		relations = []string{FieldQueryBacklinks, FieldQueryOutgoing, FieldQueryTwoHop}
	}
	for _, relation := range relations {
		switch relation {
		case FieldQueryBacklinks, FieldQueryOutgoing:
			nodes := []QueryNode{}
			if relation == FieldQueryBacklinks || info.Type == NodeTypeNote {
				nodes, err = queryDirect(db, nodeID, relation, opts)
				if err != nil {
					return nil, err
				}
			}
			start, end, next := queryPageBounds(len(nodes), result.Page.Offset, opts.Limit)
			nodes = nodes[start:end]
			result.Page.NextOffset = next
			if relation == FieldQueryBacklinks {
				result.Backlinks = nodes
			} else {
				result.Outgoing = nodes
			}
		case FieldQueryTwoHop:
			targets := []TwoHopEntry{}
			if info.Type == NodeTypeNote {
				targets, err = queryTwoHop(db, nodeID, opts)
				if err != nil {
					return nil, err
				}
			}
			start, end, next := queryPageBounds(len(targets), result.Page.Offset, opts.Limit)
			result.TwoHop = targets[start:end]
			result.Page.NextOffset = next
		}
	}
	if err := addQueryPreviews(db, vaultPath, nodeID, opts, result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateQueryOptions(opts QueryOptions) error {
	if opts.IncludeHead != nil && *opts.IncludeHead <= 0 {
		return fmt.Errorf("include-head must be positive")
	}
	if opts.IncludeSnippet != nil && *opts.IncludeSnippet < 0 {
		return fmt.Errorf("include-snippet must be non-negative")
	}
	if err := validateGlobPatterns(opts.Path); err != nil {
		return err
	}
	if opts.Relations != nil {
		if len(opts.Relations) == 0 {
			return fmt.Errorf("relations: select at least one relation")
		}
		seen := make(map[string]bool)
		for _, r := range opts.Relations {
			if r != FieldQueryBacklinks && r != FieldQueryOutgoing && r != FieldQueryTwoHop {
				return fmt.Errorf("relations: unknown relation %q", r)
			}
			if seen[r] {
				return fmt.Errorf("relations: duplicate relation %q", r)
			}
			seen[r] = true
		}
	}
	if opts.Limit != nil && *opts.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	if opts.Offset != nil && *opts.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if (opts.Limit != nil || opts.Offset != nil) && (opts.Relations == nil || len(opts.Relations) != 1) {
		return fmt.Errorf("limit and offset require one explicit relation")
	}
	return nil
}

func queryPageBounds(length, offset int, limit *int) (int, int, *int) {
	start := min(offset, length)
	end := length
	if limit != nil && *limit < length-start {
		end = start + *limit
	}
	if end < length {
		next := end
		return start, end, &next
	}
	return start, end, nil
}
