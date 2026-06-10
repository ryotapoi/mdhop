package main

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ryotapoi/mdhop/internal/core"
)

type graphJSONNode struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type graphJSONEdge struct {
	Source   int64  `json:"source"`
	Target   int64  `json:"target"`
	LinkType string `json:"link_type"`
}

func printGraphJSON(w io.Writer, r *core.GraphResult) error {
	nodes := make([]graphJSONNode, len(r.Nodes))
	for i, n := range r.Nodes {
		nodes[i] = graphJSONNode{ID: n.ID, Type: string(n.Type), Name: n.Name, Path: n.Path}
	}
	edges := make([]graphJSONEdge, len(r.Edges))
	for i, e := range r.Edges {
		edges[i] = graphJSONEdge{Source: e.Source, Target: e.Target, LinkType: string(e.LinkType)}
	}
	return encodeJSON(w, map[string]any{"nodes": nodes, "edges": edges})
}

func printGraphDot(w io.Writer, r *core.GraphResult) error {
	fmt.Fprintln(w, "digraph mdhop {")
	for _, n := range r.Nodes {
		fmt.Fprintf(w, "  n%d [label=%s];\n", n.ID, dotQuote(n.DotLabel()))
	}
	for _, e := range r.Edges {
		fmt.Fprintf(w, "  n%d -> n%d;\n", e.Source, e.Target)
	}
	fmt.Fprintln(w, "}")
	return nil
}

// dotQuote renders a Graphviz double-quoted string literal. strconv.Quote
// already escapes backslashes and double quotes, which is what dot needs.
func dotQuote(s string) string {
	return strconv.Quote(s)
}
