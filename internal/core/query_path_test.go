package core

import (
	"strings"
	"testing"
)

func TestQueryBacklinksIncludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	// A.md is linked from B.md, C.md, daily/D.md, templates/T.md.
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Path:   []string{"daily/*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Backlinks) != 1 {
		t.Fatalf("backlinks count = %d, want 1: %+v", len(res.Backlinks), res.Backlinks)
	}
	if res.Backlinks[0].Path != "daily/D.md" {
		t.Errorf("backlinks[0].Path = %q, want daily/D.md", res.Backlinks[0].Path)
	}
}

func TestQueryOutgoingIncludePathKeepsPhantom(t *testing.T) {
	vault := setupExcludeVault(t)
	// A.md links to B, C, D, Missing (phantom).
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"outgoing"},
		Path:   []string{"daily/*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := nodeNames(res.Outgoing)
	expectContains(t, names, "D")
	expectContains(t, names, "Missing") // NULL-path phantom is kept
	for _, og := range res.Outgoing {
		if og.Path == "B.md" || og.Path == "C.md" {
			t.Errorf("outgoing should not contain %s with --path daily/*", og.Path)
		}
	}
}

func TestQueryTwoHopIncludePathFiltersTargetsNotVia(t *testing.T) {
	vault := setupExcludeVault(t)
	// A → B (via), B is linked from C.md and daily/D.md (targets).
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"twohop"},
		Path:   []string{"daily/*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.TwoHop) != 1 {
		t.Fatalf("twohop count = %d, want 1: %+v", len(res.TwoHop), res.TwoHop)
	}
	entry := res.TwoHop[0]
	// Via B.md is outside daily/* but kept as connector.
	if entry.Via.Name != "B" {
		t.Errorf("via name = %q, want B", entry.Via.Name)
	}
	if len(entry.Targets) != 1 || entry.Targets[0].Path != "daily/D.md" {
		t.Errorf("targets = %+v, want only daily/D.md", entry.Targets)
	}
}

func TestQuerySnippetsIncludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	// B.md is linked from A.md, C.md, daily/D.md.
	res, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 1,
		Path:           []string{"daily/*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Snippets) != 1 {
		t.Fatalf("snippets count = %d, want 1: %+v", len(res.Snippets), res.Snippets)
	}
	if res.Snippets[0].SourcePath != "daily/D.md" {
		t.Errorf("snippet source = %q, want daily/D.md", res.Snippets[0].SourcePath)
	}
}

func TestQueryIncludePathWithExclude(t *testing.T) {
	vault := setupExcludeVault(t)
	// Include daily/* and templates/*, then exclude templates/*.
	ef, err := NewExcludeFilter(ExcludeConfig{}, []string{"templates/*"}, nil)
	if err != nil {
		t.Fatalf("NewExcludeFilter: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Path:    []string{"daily/*", "templates/*"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Backlinks) != 1 || res.Backlinks[0].Path != "daily/D.md" {
		t.Errorf("backlinks = %+v, want only daily/D.md", res.Backlinks)
	}
}

func TestQueryIncludePathInvalidGlob(t *testing.T) {
	vault := setupExcludeVault(t)
	_, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Path: []string{"[abc]/*"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported glob pattern") {
		t.Errorf("error = %v, want unsupported glob pattern", err)
	}
}
