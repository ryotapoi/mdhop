package core

import (
	"fmt"
	"sort"
)

// GraphOptions controls the subgraph export.
type GraphOptions struct {
	// Path / Exclude filter the node set (existing notes and assets) by
	// path glob. Empty Path means all.
	Path    []string
	Exclude []string
	// IncludePhantoms adds phantom nodes referenced from in-set notes,
	// together with those edges.
	IncludePhantoms bool
}

// GraphNode is one node of the exported subgraph. ID is the export-scoped
// reference key used by GraphEdge; it is not stable across builds.
type GraphNode struct {
	ID   int64
	Type NodeType
	Name string
	Path string // empty for phantoms
}

// GraphEdge is one link occurrence between exported nodes. The same
// source/target pair appears once per occurrence; weighting is up to the
// consumer.
type GraphEdge struct {
	Source   int64
	Target   int64
	LinkType LinkType
}

// GraphResult is the induced subgraph: nodes matching the path filters and
// the edges whose both endpoints are in the node set.
type GraphResult struct {
	Nodes []GraphNode // sorted by type, path, name
	Edges []GraphEdge // sorted by source, target, link type
}

// Graph exports the induced subgraph of existing notes and assets matching
// the path filters. Tag nodes are never exported, so tag edges drop out
// naturally. Interpretation (similarity, clustering, ...) is left to the
// caller by design.
func Graph(vaultPath string, opts GraphOptions) (*GraphResult, error) {
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Node set. type IN ('note','asset') AND exists_flag=1 guarantees a
	// non-NULL path, so the GLOB filters never hit NULL three-valued logic.
	inclSQL, inclArgs := pathIncludeSQL("path", opts.Path)
	ef := &ExcludeFilter{PathGlobs: opts.Exclude}
	exclSQL, exclArgs := ef.PathExcludeSQL("path")
	rows, err := db.Query(
		`SELECT id, type, name, path FROM nodes
		 WHERE type IN ('note','asset') AND exists_flag=1`+inclSQL+exclSQL,
		append(inclArgs, exclArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &GraphResult{}
	inSet := make(map[int64]bool)
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.Type, &n.Name, &n.Path); err != nil {
			return nil, err
		}
		result.Nodes = append(result.Nodes, n)
		inSet[n.ID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Edges from in-set notes; the induced filter (target in set) happens
	// Go-side. Full load + Go-side set check is the same pattern as
	// reachable. With IncludePhantoms, phantom targets are collected too
	// (phantoms have no outgoing edges, so phantom→phantom cannot occur).
	erows, err := db.Query(
		`SELECT e.source_id, e.target_id, e.link_type, tn.type, tn.name, COALESCE(tn.path,'')
		 FROM edges e
		 JOIN nodes tn ON tn.id = e.target_id`)
	if err != nil {
		return nil, err
	}
	defer erows.Close()

	phantoms := make(map[int64]GraphNode)
	for erows.Next() {
		var e GraphEdge
		var tn GraphNode
		if err := erows.Scan(&e.Source, &e.Target, &e.LinkType, &tn.Type, &tn.Name, &tn.Path); err != nil {
			return nil, err
		}
		if !inSet[e.Source] {
			continue
		}
		switch {
		case inSet[e.Target]:
			result.Edges = append(result.Edges, e)
		case opts.IncludePhantoms && tn.Type == NodeTypePhantom:
			tn.ID = e.Target
			phantoms[tn.ID] = tn
			result.Edges = append(result.Edges, e)
		}
	}
	if err := erows.Err(); err != nil {
		return nil, err
	}

	for _, n := range phantoms {
		result.Nodes = append(result.Nodes, n)
	}

	sort.Slice(result.Nodes, func(i, j int) bool {
		a, b := result.Nodes[i], result.Nodes[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Name < b.Name
	})
	sort.Slice(result.Edges, func(i, j int) bool {
		a, b := result.Edges[i], result.Edges[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.LinkType < b.LinkType
	})

	return result, nil
}

// DotLabel returns the Graphviz label for a node: the path for notes and
// assets, "(phantom) <name>" for phantoms (avoids label collisions between
// a phantom and a note sharing the basename).
func (n GraphNode) DotLabel() string {
	if n.Type == NodeTypePhantom {
		return fmt.Sprintf("(phantom) %s", n.Name)
	}
	return n.Path
}
