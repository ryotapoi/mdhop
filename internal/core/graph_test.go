package core

import (
	"reflect"
	"testing"
)

// buildGraphVault copies vault_graph into a temp dir and builds it.
func buildGraphVault(t *testing.T) string {
	t.Helper()
	vault := copyVault(t, "vault_graph")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	return vault
}

// graphNodeLabels returns "type:path" (or "type:name" for phantoms) in result
// order, so tests assert content and sort order together.
func graphNodeLabels(r *GraphResult) []string {
	labels := make([]string, len(r.Nodes))
	for i, n := range r.Nodes {
		if n.Type == NodeTypePhantom {
			labels[i] = string(n.Type) + ":" + n.Name
		} else {
			labels[i] = string(n.Type) + ":" + n.Path
		}
	}
	return labels
}

// graphEdgeLabels returns "source -> target (link_type)" in result order,
// using DotLabel so phantom targets are readable.
func graphEdgeLabels(r *GraphResult) []string {
	byID := make(map[int64]GraphNode)
	for _, n := range r.Nodes {
		byID[n.ID] = n
	}
	labels := make([]string, len(r.Edges))
	for i, e := range r.Edges {
		labels[i] = byID[e.Source].DotLabel() + " -> " + byID[e.Target].DotLabel() + " (" + string(e.LinkType) + ")"
	}
	return labels
}

func TestGraphBasic(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	// Assets sort before notes (mdhop.yaml is indexed as an asset like any
	// non-md file); no tag node for #t1, no phantom for Ghost.
	wantNodes := []string{
		"asset:img/pic.png",
		"asset:mdhop.yaml",
		"note:docs/a.md",
		"note:docs/b.md",
		"note:docs/c.md",
		"note:other/x.md",
	}
	if got := graphNodeLabels(res); !reflect.DeepEqual(got, wantNodes) {
		t.Errorf("nodes = %v, want %v", got, wantNodes)
	}
	// Tag edge (a → #t1) and phantom edge (a → Ghost) drop out; the
	// frontmatter_path edge from link_keys is exported like any other.
	wantEdges := []string{
		"docs/a.md -> img/pic.png (markdown)",
		"docs/a.md -> docs/b.md (wikilink)",
		"docs/a.md -> docs/c.md (frontmatter_path)",
		"docs/a.md -> other/x.md (markdown)",
	}
	if got := graphEdgeLabels(res); len(got) != len(wantEdges) {
		t.Fatalf("edges = %v, want %v", got, wantEdges)
	} else {
		gotSet := make(map[string]bool)
		for _, l := range got {
			gotSet[l] = true
		}
		for _, w := range wantEdges {
			if !gotSet[w] {
				t.Errorf("missing edge %q in %v", w, got)
			}
		}
	}
}

func TestGraphEdgeSortDeterministic(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for i := 1; i < len(res.Edges); i++ {
		a, b := res.Edges[i-1], res.Edges[i]
		if a.Source > b.Source ||
			(a.Source == b.Source && a.Target > b.Target) ||
			(a.Source == b.Source && a.Target == b.Target && a.LinkType > b.LinkType) {
			t.Errorf("edges not sorted at %d: %+v before %+v", i, a, b)
		}
	}
}

func TestGraphIncludePhantoms(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{IncludePhantoms: true})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	wantNodes := []string{
		"asset:img/pic.png",
		"asset:mdhop.yaml",
		"note:docs/a.md",
		"note:docs/b.md",
		"note:docs/c.md",
		"note:other/x.md",
		"phantom:Ghost",
	}
	if got := graphNodeLabels(res); !reflect.DeepEqual(got, wantNodes) {
		t.Errorf("nodes = %v, want %v", got, wantNodes)
	}
	found := false
	for _, l := range graphEdgeLabels(res) {
		if l == "docs/a.md -> (phantom) Ghost (wikilink)" {
			found = true
		}
	}
	if !found {
		t.Errorf("edges = %v, want a → Ghost wikilink edge", graphEdgeLabels(res))
	}
}

func TestGraphPathFilterInduced(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{Path: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	wantNodes := []string{"note:docs/a.md", "note:docs/b.md", "note:docs/c.md"}
	if got := graphNodeLabels(res); !reflect.DeepEqual(got, wantNodes) {
		t.Errorf("nodes = %v, want %v", got, wantNodes)
	}
	// Edges to other/x.md and img/pic.png drop because their targets left
	// the set (induced subgraph).
	wantEdges := []string{
		"docs/a.md -> docs/b.md (wikilink)",
		"docs/a.md -> docs/c.md (frontmatter_path)",
	}
	got := graphEdgeLabels(res)
	gotSet := make(map[string]bool)
	for _, l := range got {
		gotSet[l] = true
	}
	if len(got) != len(wantEdges) {
		t.Fatalf("edges = %v, want %v", got, wantEdges)
	}
	for _, w := range wantEdges {
		if !gotSet[w] {
			t.Errorf("missing edge %q in %v", w, got)
		}
	}
}

func TestGraphExclude(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{Path: []string{"docs/*"}, Exclude: []string{"docs/c.md"}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	wantNodes := []string{"note:docs/a.md", "note:docs/b.md"}
	if got := graphNodeLabels(res); !reflect.DeepEqual(got, wantNodes) {
		t.Errorf("nodes = %v, want %v", got, wantNodes)
	}
	for _, l := range graphEdgeLabels(res) {
		if l == "docs/a.md -> docs/c.md (frontmatter_path)" {
			t.Errorf("excluded target must drop its edge: %v", graphEdgeLabels(res))
		}
	}
}

func TestGraphPhantomNotIncludedWithoutFlag(t *testing.T) {
	vault := buildGraphVault(t)
	res, err := Graph(vault, GraphOptions{Path: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for _, n := range res.Nodes {
		if n.Type == NodeTypePhantom {
			t.Errorf("phantom node %q exported without IncludePhantoms", n.Name)
		}
	}
}

func TestGraphInvalidGlob(t *testing.T) {
	vault := buildGraphVault(t)
	if _, err := Graph(vault, GraphOptions{Path: []string{"docs/[ab]*"}}); err == nil {
		t.Errorf("error = nil, want invalid glob error for Path")
	}
	if _, err := Graph(vault, GraphOptions{Exclude: []string{"docs/[ab]*"}}); err == nil {
		t.Errorf("error = nil, want invalid glob error for Exclude")
	}
}

func TestGraphDotLabel(t *testing.T) {
	note := GraphNode{Type: NodeTypeNote, Name: "a", Path: "docs/a.md"}
	if got := note.DotLabel(); got != "docs/a.md" {
		t.Errorf("note DotLabel = %q, want docs/a.md", got)
	}
	ph := GraphNode{Type: NodeTypePhantom, Name: "Ghost"}
	if got := ph.DotLabel(); got != "(phantom) Ghost" {
		t.Errorf("phantom DotLabel = %q, want (phantom) Ghost", got)
	}
}
