package core

import (
	"testing"
)

func setupSearchVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_query_where")
	buildForQuery(t, vault)
	return vault
}

func searchVaultConfig(t *testing.T, vault string) MetaConfig {
	t.Helper()
	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg.Meta
}

func searchPaths(items []SearchResultItem) []string {
	paths := make([]string, len(items))
	for i, item := range items {
		paths[i] = item.Node.Path
	}
	return paths
}

func TestSearch_AllNotes(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if len(result.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(result.Items))
	}

	// Default order is by path ASC.
	wantPaths := []string{"A.md", "B.md", "C.md", "D.md", "E.md"}
	for i, want := range wantPaths {
		got := result.Items[i].Node.Path
		if got != want {
			t.Errorf("items[%d].path = %q, want %q", i, got, want)
		}
		if result.Items[i].Node.Type != NodeTypeNote {
			t.Errorf("items[%d].type = %q, want %q", i, result.Items[i].Node.Type, "note")
		}
		if !result.Items[i].Node.Exists {
			t.Errorf("items[%d].exists = false, want true", i)
		}
	}

	// Meta should be nil when not requested.
	for i, item := range result.Items {
		if item.Meta != nil {
			t.Errorf("items[%d].meta should be nil when not requested", i)
		}
		if item.Head != nil {
			t.Errorf("items[%d].head should be nil when not requested", i)
		}
	}
}

func TestSearch_WhereEq(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)

	wc, err := ParseWhere([]string{"status=active"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A (active), B (active), E (active). C (done) and D (no frontmatter) excluded.
	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
	paths := searchPaths(result.Items)
	want := []string{"A.md", "B.md", "E.md"}
	for i, w := range want {
		if i >= len(paths) || paths[i] != w {
			t.Errorf("paths = %v, want %v", paths, want)
			break
		}
	}
}

func TestSearch_WhereGt(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)

	wc, err := ParseWhere([]string{"priority>1"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// B (2) and C (3). A (1) excluded by >1. E (abc, value_type=string) excluded by type guard. D has no priority.
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	paths := searchPaths(result.Items)
	want := []string{"B.md", "C.md"}
	for i, w := range want {
		if i >= len(paths) || paths[i] != w {
			t.Errorf("paths = %v, want %v", paths, want)
			break
		}
	}
}

func TestSearch_SortAsc(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Sort: "priority"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sort_value order: A(1...01), B(1...02), C(1...03), E("abc" string fallback), D(null=last)
	// "1..." < "a" in lexicographic order, so: A, B, C, E, D
	wantPaths := []string{"A.md", "B.md", "C.md", "E.md", "D.md"}
	if len(result.Items) != len(wantPaths) {
		t.Fatalf("items = %d, want %d", len(result.Items), len(wantPaths))
	}
	for i, want := range wantPaths {
		got := result.Items[i].Node.Path
		if got != want {
			t.Errorf("items[%d].path = %q, want %q", i, got, want)
		}
	}
}

func TestSearch_SortDesc(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Sort: "-priority"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DESC: E("abc"), C(1...03), B(1...02), A(1...01), D(null=last)
	wantPaths := []string{"E.md", "C.md", "B.md", "A.md", "D.md"}
	if len(result.Items) != len(wantPaths) {
		t.Fatalf("items = %d, want %d", len(result.Items), len(wantPaths))
	}
	for i, want := range wantPaths {
		got := result.Items[i].Node.Path
		if got != want {
			t.Errorf("items[%d].path = %q, want %q", i, got, want)
		}
	}
}

func TestSearch_WhereMulti(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)

	// status=active AND priority>0 (different keys = AND)
	wc, err := ParseWhere([]string{"status=active", "priority>0"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A (active, priority=1) and B (active, priority=2).
	// E has status=active but priority=abc (value_type=string, excluded by >0 type guard).
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	paths := searchPaths(result.Items)
	want := []string{"A.md", "B.md"}
	for i, w := range want {
		if i >= len(paths) || paths[i] != w {
			t.Errorf("paths = %v, want %v", paths, want)
			break
		}
	}
}

func TestSearch_Limit(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5 (total before limit)", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	if result.Items[0].Node.Path != "A.md" {
		t.Errorf("items[0].path = %q, want A.md", result.Items[0].Node.Path)
	}
}

func TestSearch_Offset(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Offset: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	// Offset 3 skips A, B, C → D, E
	if result.Items[0].Node.Path != "D.md" {
		t.Errorf("items[0].path = %q, want D.md", result.Items[0].Node.Path)
	}
}

func TestSearch_LimitOffset(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	// Offset 1, limit 2 → B, C
	wantPaths := []string{"B.md", "C.md"}
	for i, want := range wantPaths {
		if result.Items[i].Node.Path != want {
			t.Errorf("items[%d].path = %q, want %q", i, result.Items[i].Node.Path, want)
		}
	}
}

func setupSearchVaultWithSub(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_search")
	buildForQuery(t, vault)
	return vault
}

func TestSearch_PathInclude(t *testing.T) {
	vault := setupSearchVaultWithSub(t)

	result, err := Search(vault, SearchOptions{Path: []string{"sub/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	wantPaths := []string{"sub/B.md", "sub/C.md"}
	if len(result.Items) != len(wantPaths) {
		t.Fatalf("items = %d, want %d", len(result.Items), len(wantPaths))
	}
	for i, want := range wantPaths {
		if result.Items[i].Node.Path != want {
			t.Errorf("items[%d].path = %q, want %q", i, result.Items[i].Node.Path, want)
		}
	}
}

func TestSearch_FieldsMeta(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{
		Fields: []string{"meta"},
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Node.Path != "A.md" {
		t.Fatalf("path = %q, want A.md", item.Node.Path)
	}
	if item.Meta == nil {
		t.Fatal("meta should not be nil when requested")
	}
	// A.md has priority=1, status=active, aliases=[alpha]
	if len(item.Meta) < 2 {
		t.Errorf("meta count = %d, want >= 2", len(item.Meta))
	}
}

func TestSearch_FieldsDefault(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Items[0].Meta != nil {
		t.Errorf("meta should be nil when not requested")
	}
}

func TestSearch_IncludeHead(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{
		IncludeHead: 2,
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item := result.Items[0]
	if item.Node.Path != "A.md" {
		t.Fatalf("path = %q, want A.md", item.Node.Path)
	}
	if item.Head == nil {
		t.Fatal("head should not be nil when requested")
	}
	if len(item.Head) == 0 {
		t.Fatal("head should not be empty")
	}
	// First non-frontmatter line of A.md is "# A"
	if item.Head[0] != "# A" {
		t.Errorf("head[0] = %q, want %q", item.Head[0], "# A")
	}
}

func TestSearch_IncludeHeadZero(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Items[0].Head != nil {
		t.Errorf("head should be nil when include-head is 0")
	}
}

func TestSearch_ExcludeFilter(t *testing.T) {
	vault := setupSearchVaultWithSub(t)

	ef, err := NewExcludeFilter(ExcludeConfig{}, []string{"sub/*"}, nil)
	if err != nil {
		t.Fatalf("new exclude filter: %v", err)
	}

	result, err := Search(vault, SearchOptions{Exclude: ef})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	wantPaths := []string{"A.md", "D.md"}
	if len(result.Items) != len(wantPaths) {
		t.Fatalf("items = %d, want %d", len(result.Items), len(wantPaths))
	}
	for i, want := range wantPaths {
		if result.Items[i].Node.Path != want {
			t.Errorf("items[%d].path = %q, want %q", i, result.Items[i].Node.Path, want)
		}
	}
}

func TestSearch_NoDB(t *testing.T) {
	vault := t.TempDir()
	_, err := Search(vault, SearchOptions{})
	if err == nil {
		t.Fatal("expected error for missing DB")
	}
}

func TestSearch_InvalidLimit(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Limit: -1})
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestSearch_InvalidOffset(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Offset: -1})
	if err == nil {
		t.Fatal("expected error for negative offset")
	}
}

func TestSearch_EmptyResult(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)

	wc, err := ParseWhere([]string{"status=nonexistent"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if len(result.Items) != 0 {
		t.Errorf("items = %d, want 0", len(result.Items))
	}
}

func TestSearch_InvalidSortKey(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Sort: "-"})
	if err == nil {
		t.Fatal("expected error for empty sort key")
	}
}
