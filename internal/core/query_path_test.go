package core

import (
	"strings"
	"testing"
)

func TestQueryBacklinksIncludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	// A.md is linked from B.md, C.md, daily/D.md, templates/T.md.
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Relations: []string{"backlinks"},
		Path:      []string{"daily/*"},
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
		Relations: []string{"outgoing"},
		Path:      []string{"daily/*"},
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

func TestQueryTwoHopIncludePathFiltersTargetsNotRelations(t *testing.T) {
	vault := setupExcludeVault(t)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Path: []string{"daily/*"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range res.TwoHop {
		if target.Path != "daily/D.md" {
			t.Fatalf("target outside included path: %+v", target)
		}
	}
	if len(res.TwoHop) == 0 {
		t.Fatal("expected daily/D.md through an unfiltered relation")
	}
}

func TestQueryIncludePathWithHide(t *testing.T) {
	vault := setupExcludeVault(t)
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Paths: []string{"templates/*"}}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, Path: []string{"daily/*", "templates/*"}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Backlinks) != 1 || res.Backlinks[0].Path != "daily/D.md" {
		t.Fatalf("backlinks = %+v", res.Backlinks)
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
