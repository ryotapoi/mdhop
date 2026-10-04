package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func previewVault(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	for path, content := range map[string]string{
		"Plan.md":   "---\ntags: topic/sub\nrelated: \"[[Via]]\"\n---\n\n# Plan\n[[Via]] [[Via]]\n[[Empty]] [[image.png]] [[Ghost]]\n",
		"Guide.md":  "---\ntags: topic/sub\nrelated: \"[[Via]]\"\nstatus: active\n---\n\n# Guide\n[[Plan]] [[Via]] [[Via]] [[image.png]] [[Ghost]]\nlast\n",
		"Other.md":  "[[Plan]]\n[[Via]]\n",
		"Via.md":    "---\nstatus: active\n---\n# Via\n",
		"Empty.md":  "",
		"image.png": "image",
	} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	buildForQuery(t, vault)
	return vault
}
func previewInt(n int) *int { return &n }
func previewNode(t *testing.T, nodes []QueryNode, path string) QueryNode {
	t.Helper()
	for _, node := range nodes {
		if node.Path == path {
			return node
		}
	}
	t.Fatalf("missing %s in %+v", path, nodes)
	return QueryNode{}
}
func TestQueryPreviewsRelationOwnership(t *testing.T) {
	vault := previewVault(t)
	r, err := Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{IncludeHead: previewInt(1), IncludeSnippet: previewInt(0)})
	if err != nil {
		t.Fatal(err)
	}
	guide := previewNode(t, r.Backlinks, "Guide.md")
	if !reflect.DeepEqual(guide.Head, []string{"# Guide"}) || len(guide.Snippet) != 1 || guide.Snippet[0].SourcePath != "Guide.md" || guide.Snippet[0].LineStart != 8 {
		t.Fatalf("backlink=%+v", guide)
	}
	via := previewNode(t, r.Outgoing, "Via.md")
	if len(via.Snippet) != 3 || via.Snippet[0].LineStart != 3 || via.Snippet[1].LineStart != 7 || via.Snippet[2].LineStart != 7 {
		t.Fatalf("outgoing=%+v", via)
	}
	if got := previewNode(t, r.Outgoing, "Empty.md").Head; got == nil || len(got) != 0 {
		t.Fatalf("empty head=%#v", got)
	}
	for _, node := range r.Outgoing {
		if node.Type != NodeTypeNote && (node.Head != nil || len(node.Snippet) == 0 || node.Snippet[0].SourcePath != "Plan.md") {
			t.Fatalf("non-note=%+v", node)
		}
	}
	for _, target := range r.TwoHop {
		if target.Path == "Guide.md" {
			types := map[NodeType]bool{}
			for _, via := range target.Relation {
				types[via.Type] = true
				if via.Head != nil || len(via.Snippet) == 0 || via.Snippet[0].SourcePath != "Guide.md" {
					t.Fatalf("via preview=%+v", via)
				}
			}
			for _, typ := range []NodeType{NodeTypeNote, NodeTypeTag, NodeTypeAsset, NodeTypePhantom} {
				if !types[typ] {
					t.Fatalf("missing via type %s", typ)
				}
			}
			v := previewNode(t, target.Relation, "Via.md")
			if v.Head != nil || len(v.Snippet) != 3 || v.Snippet[0].SourcePath != "Guide.md" {
				t.Fatalf("twohop=%+v", v)
			}
			for _, s := range v.Snippet {
				if s.LineStart != 3 && s.LineStart != 8 {
					t.Fatalf("wrong edge=%+v", s)
				}
			}
		}
	}
	// Direct link-key selection is occurrence based, while twohop remains independent.
	r, err = Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{LinkKey: "related", IncludeSnippet: previewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	v := previewNode(t, r.Outgoing, "Via.md")
	if len(v.Snippet) != 1 || !reflect.DeepEqual(v.Snippet[0].Lines, []string{"tags: topic/sub", "related: \"[[Via]]\"", "---"}) {
		t.Fatalf("raw frontmatter=%+v", v)
	}
	for _, target := range r.TwoHop {
		if target.Path == "Guide.md" {
			v := previewNode(t, target.Relation, "Via.md")
			if len(v.Snippet) != 3 || v.Snippet[1].LineStart != 7 || v.Snippet[1].LineEnd != 9 {
				t.Fatalf("overlapping contexts=%+v", v)
			}
		}
	}
	// Filters select targets, not the outgoing source.
	where, err := ParseWhere([]string{"status=active"}, MetaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = Query(vault, EntrySpec{File: "Via.md"}, QueryOptions{Relations: []string{"backlinks"}, Path: []string{"Guide.md"}, Where: where, IncludeSnippet: previewInt(0)})
	if err != nil || len(r.Backlinks) != 1 || len(r.Backlinks[0].Snippet) != 3 {
		t.Fatalf("filtered backlinks=%+v error=%v", r, err)
	}
	r, err = Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{Relations: []string{"outgoing"}, Path: []string{"Via.md"}, Where: where, IncludeSnippet: previewInt(0)})
	if err != nil || previewNode(t, r.Outgoing, "Via.md").Snippet[0].SourcePath != "Plan.md" {
		t.Fatalf("filtered outgoing=%+v error=%v", r, err)
	}
	// Typed non-note entries retrieve the actual referring source, including parent tags.
	for _, entry := range []EntrySpec{{Tag: "topic"}, {File: "image.png"}, {Phantom: "Ghost"}} {
		r, err := Query(vault, entry, QueryOptions{Relations: []string{"backlinks"}, IncludeSnippet: previewInt(0)})
		if err != nil || len(r.Backlinks) == 0 {
			t.Fatalf("entry=%+v result=%+v err=%v", entry, r, err)
		}
		for _, n := range r.Backlinks {
			if len(n.Snippet) == 0 || n.Snippet[0].SourcePath != n.Path {
				t.Fatalf("typed backlink=%+v", n)
			}
		}
	}
}

func TestQueryPreviewReadBoundaries(t *testing.T) {
	for _, mutation := range []string{"missing", "stale"} {
		t.Run(mutation, func(t *testing.T) {
			vault := previewVault(t)
			mutate := func(path string) {
				t.Helper()
				full := filepath.Join(vault, path)
				if mutation == "missing" {
					if err := os.Remove(full); err != nil {
						t.Fatal(err)
					}
				} else {
					later := time.Now().Add(10 * time.Second)
					if err := os.Chtimes(full, later, later); err != nil {
						t.Fatal(err)
					}
				}
			}
			// The entry, via body, next page, and hidden target are unnecessary for this page.
			mutate("Plan.md")
			mutate("Via.md")
			mutate("Other.md")
			opts := QueryOptions{Relations: []string{"twohop"}, IncludeSnippet: previewInt(0), IncludeHead: previewInt(1), Limit: previewInt(1)}
			r, err := Query(vault, EntrySpec{File: "Plan.md"}, opts)
			if err != nil || len(r.TwoHop) != 1 || r.TwoHop[0].Path != "Guide.md" || r.Page.NextOffset == nil {
				t.Fatalf("page=%+v err=%v", r, err)
			}
			filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Via: []string{"note:Via.md"}, Hide: ExcludeConfig{Paths: []string{"Other.md", "Via.md", "Empty.md", "image.png"}, Tags: []string{"topic", "topic/sub"}}})
			if err != nil {
				t.Fatal(err)
			}
			where, err := ParseWhere([]string{"status=active"}, MetaConfig{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{Relations: []string{"backlinks"}, Path: []string{"Guide.md"}, Where: where, IncludeHead: previewInt(1), IncludeSnippet: previewInt(0)}); err != nil {
				t.Fatalf("filtered target read: %v", err)
			}
			opts.Filter = filter
			opts.Limit = nil
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, opts); err != nil {
				t.Fatalf("hidden: %v", err)
			}
			mutate("Guide.md")
			opts.IncludeHead = nil
			r, err = Query(vault, EntrySpec{File: "Plan.md"}, opts)
			if err != nil || !r.TwoHop[0].HiddenRelation || len(r.TwoHop[0].Relation) != 0 {
				t.Fatalf("hidden-only=%+v err=%v", r, err)
			}
			expected := ErrFileNotFound
			if mutation == "stale" {
				expected = ErrSourceStale
			}
			opts.IncludeHead = previewInt(1)
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, opts); !errors.Is(err, expected) {
				t.Fatalf("required head error=%v", err)
			}
			opts.IncludeHead = nil
			opts.Filter = nil
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, opts); !errors.Is(err, expected) {
				t.Fatalf("required snippet error=%v", err)
			}
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{}); err != nil {
				t.Fatalf("no preview: %v", err)
			}
			// An outgoing non-note still requires the entry source when snippet is requested.
			opts = QueryOptions{Relations: []string{"outgoing"}, Path: []string{"image.png"}, IncludeHead: previewInt(1)}
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, opts); err != nil {
				t.Fatalf("asset head: %v", err)
			}
			opts.IncludeSnippet = previewInt(0)
			if _, err := Query(vault, EntrySpec{File: "Plan.md"}, opts); !errors.Is(err, expected) {
				t.Fatalf("entry source error=%v", err)
			}
		})
	}
}
func TestQueryPreviewValidationAndMaximumContext(t *testing.T) {
	for _, opts := range []QueryOptions{{IncludeHead: previewInt(0)}, {IncludeHead: previewInt(-1)}, {IncludeSnippet: previewInt(-1)}} {
		if _, err := Query(t.TempDir(), EntrySpec{}, opts); err == nil {
			t.Fatalf("accepted %+v", opts)
		}
	}
	vault := previewVault(t)
	r, err := Query(vault, EntrySpec{File: "Plan.md"}, QueryOptions{Relations: []string{"backlinks"}, Path: []string{"Guide.md"}, IncludeSnippet: previewInt(int(^uint(0) >> 1))})
	if err != nil {
		t.Fatal(err)
	}
	s := r.Backlinks[0].Snippet[0]
	if s.LineStart != 1 || s.LineEnd != 9 || len(s.Lines) != 9 {
		t.Fatalf("maximum context=%+v", s)
	}
}
