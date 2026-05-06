package core

// EntrySpec specifies the entry node for a query.
type EntrySpec struct {
	File    string // vault-relative path
	Tag     string // tag name (# optional)
	Phantom string // phantom name
	Name    string // auto-detect: #tag → tag, otherwise note → phantom
}

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
}

// NodeInfo describes a node in the graph.
type NodeInfo struct {
	Type   string // NodeTypeNote ("note"), NodeTypePhantom ("phantom"), NodeTypeTag ("tag"), NodeTypeAsset ("asset")
	Name   string
	Path   string // note/asset only
	Exists bool
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
		opts.MaxBacklinks = 100
	}
	if opts.MaxTwoHop <= 0 {
		opts.MaxTwoHop = 100
	}
	if opts.MaxViaPerTarget <= 0 {
		opts.MaxViaPerTarget = 10
	}

	result := &QueryResult{Entry: info}

	ef := opts.Exclude
	wc := opts.Where

	if isFieldActive("backlinks", opts.Fields) {
		bl, err := queryBacklinks(db, nodeID, opts.MaxBacklinks, ef, wc)
		if err != nil {
			return nil, err
		}
		result.Backlinks = bl
	}

	if isFieldActive("outgoing", opts.Fields) {
		if info.Type == NodeTypeNote {
			og, err := queryOutgoing(db, nodeID, ef, wc)
			if err != nil {
				return nil, err
			}
			result.Outgoing = og
		}
	}

	if isFieldActive("tags", opts.Fields) {
		if info.Type == NodeTypeNote {
			tags, err := queryTags(db, nodeID, ef)
			if err != nil {
				return nil, err
			}
			result.Tags = tags
		}
	}

	if isFieldActive("twohop", opts.Fields) {
		th, err := queryTwoHop(db, nodeID, info.Type, opts.MaxTwoHop, opts.MaxViaPerTarget, ef, wc)
		if err != nil {
			return nil, err
		}
		result.TwoHop = th
	}

	if isFieldActive("head", opts.Fields) && opts.IncludeHead > 0 {
		if info.Type == NodeTypeNote && info.Exists {
			head, err := readHead(db, vaultPath, nodeID, opts.IncludeHead)
			if err != nil {
				return nil, err
			}
			result.Head = head
		}
	}

	if isFieldActive("snippet", opts.Fields) && opts.IncludeSnippet > 0 {
		snippets, err := readSnippets(db, vaultPath, nodeID, opts.IncludeSnippet, ef)
		if err != nil {
			return nil, err
		}
		result.Snippets = snippets
	}

	if isFieldActive("meta", opts.Fields) && len(opts.Fields) > 0 {
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
