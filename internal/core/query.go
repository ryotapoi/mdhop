package core

import "database/sql"

// EntrySpec specifies the entry node for a query.
type EntrySpec struct {
	File    string // vault-relative path
	Tag     string // tag name (# optional)
	Phantom string // phantom name
	Name    string // auto-detect: #tag → tag, otherwise note → phantom
}

const (
	DefaultMaxBacklinks    = 100
	DefaultMaxTwoHop       = 100
	DefaultMaxViaPerTarget = 10
)

// QueryOptions controls which fields to return and their limits.
type QueryOptions struct {
	Fields          []string       // nil/empty = all standard fields
	IncludeHead     int            // 0 = skip
	IncludeSnippet  int            // 0 = skip
	MaxBacklinks    int            // default 100
	MaxTwoHop       int            // default 100
	MaxViaPerTarget int            // default 10
	Exclude         *ExcludeFilter // nil = no exclusion
	Where           *WhereClause   // nil = no filtering
	// Path restricts result nodes (backlinks, outgoing, twohop targets,
	// snippet sources) to paths matching the globs. NULL-path nodes
	// (phantom/tag) are kept, and twohop via nodes are not filtered.
	Path []string
}

// Query field names accepted by QueryOptions.Fields and the query --fields
// CLI flag.
const (
	FieldQueryBacklinks = "backlinks"
	FieldQueryTags      = "tags"
	FieldQueryTwoHop    = "twohop"
	FieldQueryOutgoing  = "outgoing"
	FieldQueryHead      = "head"
	FieldQuerySnippet   = "snippet"
	FieldQueryMeta      = "meta"
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

// TwoHopEntry represents a via node and the targets reachable through it.
type TwoHopEntry struct {
	Via     NodeInfo
	Targets []NodeInfo
}

// SnippetEntry represents lines surrounding a link occurrence in a source file.
type SnippetEntry struct {
	SourcePath string
	LineStart  int
	LineEnd    int
	Lines      []string
}

// QueryResult contains all requested fields for a query.
type QueryResult struct {
	Entry     NodeInfo
	Backlinks []NodeInfo     // nil = not requested
	Outgoing  []NodeInfo     // nil = not requested
	TwoHop    []TwoHopEntry  // nil = not requested
	Tags      []string       // nil = not requested
	Head      []string       // nil = not requested
	Snippets  []SnippetEntry // nil = not requested
	Meta      []MetaRow      // nil = not requested
}

// Query returns related information for the given entry node.
func Query(vaultPath string, entry EntrySpec, opts QueryOptions) (*QueryResult, error) {
	if err := validateGlobPatterns(opts.Path); err != nil {
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

	if opts.MaxBacklinks <= 0 {
		opts.MaxBacklinks = DefaultMaxBacklinks
	}
	if opts.MaxTwoHop <= 0 {
		opts.MaxTwoHop = DefaultMaxTwoHop
	}
	if opts.MaxViaPerTarget <= 0 {
		opts.MaxViaPerTarget = DefaultMaxViaPerTarget
	}

	result := &QueryResult{Entry: info}

	ef := opts.Exclude
	wc := opts.Where

	if isFieldActive(FieldQueryBacklinks, opts.Fields) {
		bl, err := queryBacklinks(db, nodeID, opts.MaxBacklinks, ef, wc, opts.Path)
		if err != nil {
			return nil, err
		}
		result.Backlinks = bl
	}

	if isFieldActive(FieldQueryOutgoing, opts.Fields) {
		if info.Type == NodeTypeNote {
			og, err := queryOutgoing(db, nodeID, ef, wc, opts.Path)
			if err != nil {
				return nil, err
			}
			result.Outgoing = og
		}
	}

	if isFieldActive(FieldQueryTags, opts.Fields) {
		if info.Type == NodeTypeNote {
			tags, err := queryTags(db, nodeID, ef)
			if err != nil {
				return nil, err
			}
			result.Tags = tags
		}
	}

	if isFieldActive(FieldQueryTwoHop, opts.Fields) {
		th, err := queryTwoHop(db, nodeID, info.Type, opts.MaxTwoHop, opts.MaxViaPerTarget, ef, wc, opts.Path)
		if err != nil {
			return nil, err
		}
		result.TwoHop = th
	}

	if isFieldActive(FieldQueryHead, opts.Fields) && opts.IncludeHead > 0 {
		if info.Type == NodeTypeNote && info.Exists {
			head, err := readHead(db, vaultPath, nodeID, opts.IncludeHead)
			if err != nil {
				return nil, err
			}
			result.Head = head
		}
	}

	if isFieldActive(FieldQuerySnippet, opts.Fields) && opts.IncludeSnippet > 0 {
		snippets, err := readSnippets(db, vaultPath, nodeID, opts.IncludeSnippet, ef, opts.Path)
		if err != nil {
			return nil, err
		}
		result.Snippets = snippets
	}

	if isFieldActive(FieldQueryMeta, opts.Fields) && len(opts.Fields) > 0 {
		meta, err := queryMetaByNode(db, nodeID)
		if err != nil {
			return nil, err
		}
		if meta == nil {
			meta = []MetaRow{}
		}
		result.Meta = meta
	}

	return result, nil
}
