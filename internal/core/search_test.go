package core

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSearch_ComputedFields(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byPath := make(map[string]SearchResultItem)
	for _, item := range result.Items {
		byPath[item.Node.Path] = item
	}

	// A links to B, C, D, E → outgoing 4. B/C/D/E each link back to A.
	a := byPath["A.md"]
	if a.OutgoingCount != 4 {
		t.Errorf("A outgoing_count = %d, want 4", a.OutgoingCount)
	}
	// A is the target of B, C, D, E → incoming 4.
	if a.IncomingCount != 4 {
		t.Errorf("A incoming_count = %d, want 4", a.IncomingCount)
	}
	// Every note has a positive line count; D is the smallest (no frontmatter).
	for _, p := range []string{"A.md", "B.md", "C.md", "D.md", "E.md"} {
		if byPath[p].Lines <= 0 {
			t.Errorf("%s lines = %d, want > 0", p, byPath[p].Lines)
		}
	}
	if byPath["D.md"].Lines >= byPath["A.md"].Lines {
		t.Errorf("D lines (%d) should be smaller than A lines (%d)", byPath["D.md"].Lines, byPath["A.md"].Lines)
	}
}

func TestSearch_SortByComputedField(t *testing.T) {
	vault := setupSearchVault(t)

	// Sort by incoming_count descending: A (4 incoming) must come first.
	result, err := Search(vault, SearchOptions{Sort: "-incoming_count"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("no items")
	}
	if result.Items[0].Node.Path != "A.md" {
		t.Errorf("first by -incoming_count = %q, want A.md", result.Items[0].Node.Path)
	}

	// Sort by lines ascending: D (smallest) must come first.
	resultLines, err := Search(vault, SearchOptions{Sort: "lines"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resultLines.Items[0].Node.Path != "D.md" {
		t.Errorf("first by lines = %q, want D.md", resultLines.Items[0].Node.Path)
	}
}

func TestSearch_WhereRelativeDate(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)

	// All created dates in the fixture are in 2025, well before today, so
	// created<today matches every note that has a created date (A, B, C, E).
	// D has no frontmatter. The result is stable regardless of the run date.
	wc, err := ParseWhere([]string{"created<today"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := searchPaths(result.Items)
	want := []string{"A.md", "B.md", "C.md", "E.md"}
	if len(got) != len(want) {
		t.Fatalf("created<today paths = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("created<today paths = %v, want %v", got, want)
		}
	}

	// created>today matches nothing (no future-dated notes).
	wcFuture, err := ParseWhere([]string{"created>today"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	resultFuture, err := Search(vault, SearchOptions{Where: wcFuture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resultFuture.Total != 0 {
		t.Errorf("created>today total = %d, want 0", resultFuture.Total)
	}
}

// TestSearch_WhereRelativeDate_UndeclaredKey documents that relative date
// comparison only works on keys declared `date` in meta.types. An undeclared
// key is stored with value_type="string", so the value_type='date' guard in the
// comparison SQL skips it — even though its raw value looks like a date.
func TestSearch_WhereRelativeDate_UndeclaredKey(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_query_where")
	// checked holds a date-looking value but is NOT declared in meta.types.
	note := "---\nchecked: 2025-01-01\n---\n\n# Checked\n"
	if err := os.WriteFile(filepath.Join(vault, "reviewed.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	buildForQuery(t, vault)
	meta := searchVaultConfig(t, vault)

	// Sanity check: the key is genuinely undeclared.
	if _, ok := meta.Types["checked"]; ok {
		t.Fatal("checked must be undeclared for this test")
	}

	wc, err := ParseWhere([]string{"checked<today"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	result, err := Search(vault, SearchOptions{Where: wc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The value_type='date' guard skips the string-stored value → no match.
	if result.Total != 0 {
		t.Errorf("checked<today on undeclared key total = %d, want 0 (date guard skips string value_type)", result.Total)
	}

	// Contrast: declaring checked as `date` (and rebuilding) makes the same
	// query match, confirming the guard — not the syntax — is what gates it.
	yaml := "meta:\n  types:\n    priority: number\n    status: string\n    created: date\n    checked: date\n"
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write mdhop.yaml: %v", err)
	}
	buildForQuery(t, vault)
	metaDate := searchVaultConfig(t, vault)
	if metaDate.Types["checked"].Name != MetaTypeDate {
		t.Fatal("checked must be declared date after rewrite")
	}
	wc2, err := ParseWhere([]string{"checked<today"}, metaDate)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	result2, err := Search(vault, SearchOptions{Where: wc2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.Total != 1 {
		t.Errorf("checked<today after declaring date total = %d, want 1", result2.Total)
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

func TestSearch_Sample(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc, Sample: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}

	candidates := map[string]bool{"A.md": true, "B.md": true, "E.md": true}
	seen := map[string]bool{}
	for _, item := range result.Items {
		if !candidates[item.Node.Path] {
			t.Fatalf("sample item %q is outside candidates", item.Node.Path)
		}
		if seen[item.Node.Path] {
			t.Fatalf("duplicate sample item %q", item.Node.Path)
		}
		seen[item.Node.Path] = true
	}
}

func TestSearch_SampleGreaterThanTotal(t *testing.T) {
	vault := setupSearchVault(t)

	result, err := Search(vault, SearchOptions{Sample: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if len(result.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(result.Items))
	}
	seen := map[string]bool{}
	for _, item := range result.Items {
		if seen[item.Node.Path] {
			t.Fatalf("duplicate sample item %q", item.Node.Path)
		}
		seen[item.Node.Path] = true
	}
}

func TestSearch_SampleInvalid(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Sample: -1})
	if err == nil || !strings.Contains(err.Error(), "sample must be >= 0") {
		t.Fatalf("expected sample validation error, got: %v", err)
	}
}

func TestSearch_SampleWithLimitError(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Sample: 2, Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "sample cannot be used with limit or offset") {
		t.Fatalf("expected sample/limit conflict error, got: %v", err)
	}

	_, err = Search(vault, SearchOptions{Sample: 2, Offset: 1})
	if err == nil || !strings.Contains(err.Error(), "sample cannot be used with limit or offset") {
		t.Fatalf("expected sample/offset conflict error, got: %v", err)
	}
}

func TestSearch_SampleWithSortError(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Sample: 2, Sort: "priority"})
	if err == nil || !strings.Contains(err.Error(), "sample cannot be used with sort") {
		t.Fatalf("expected sample/sort conflict error, got: %v", err)
	}
}

func setupSearchVaultWithSub(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_search")
	buildForQuery(t, vault)
	return vault
}

func setupSearchIsolationVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_search_isolation")
	buildForQuery(t, vault)
	return vault
}

func assertSearchPaths(t *testing.T, items []SearchResultItem, want []string) {
	t.Helper()
	paths := searchPaths(items)
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	}
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

func TestSearch_NoTags(t *testing.T) {
	vault := setupSearchIsolationVault(t)

	result, err := Search(vault, SearchOptions{NoTags: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	assertSearchPaths(t, result.Items, []string{"B.md", "D.md"})
}

func TestSearch_NoOutgoing(t *testing.T) {
	vault := setupSearchIsolationVault(t)

	result, err := Search(vault, SearchOptions{NoOutgoing: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	assertSearchPaths(t, result.Items, []string{"D.md"})
}

func TestSearch_NoIncoming(t *testing.T) {
	vault := setupSearchIsolationVault(t)

	result, err := Search(vault, SearchOptions{NoIncoming: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	assertSearchPaths(t, result.Items, []string{"C.md", "D.md"})
}

func TestSearch_NoTagsAndNoIncoming(t *testing.T) {
	vault := setupSearchIsolationVault(t)

	result, err := Search(vault, SearchOptions{NoTags: true, NoIncoming: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	assertSearchPaths(t, result.Items, []string{"D.md"})
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

func TestSearch_Count(t *testing.T) {
	vault := setupSearchVault(t)
	meta := searchVaultConfig(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, meta)
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}

	result, err := Search(vault, SearchOptions{Where: wc, Count: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
	if len(result.Items) != 0 {
		t.Errorf("items = %d, want 0", len(result.Items))
	}
}

func TestSearch_CountWithOutputOptionsError(t *testing.T) {
	vault := setupSearchVault(t)

	tests := []struct {
		name string
		opts SearchOptions
		want string
	}{
		{name: "fields", opts: SearchOptions{Count: true, Fields: []string{"meta"}}, want: "count cannot be used with fields"},
		{name: "include head", opts: SearchOptions{Count: true, IncludeHead: 1}, want: "count cannot be used with include-head"},
		{name: "sample", opts: SearchOptions{Count: true, Sample: 1}, want: "count cannot be used with sample"},
		{name: "sort", opts: SearchOptions{Count: true, Sort: "priority"}, want: "count cannot be used with sort"},
		{name: "limit", opts: SearchOptions{Count: true, Limit: 1}, want: "count cannot be used with limit or offset"},
		{name: "offset", opts: SearchOptions{Count: true, Offset: 1}, want: "count cannot be used with limit or offset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Search(vault, tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got: %v", tt.want, err)
			}
		})
	}
}

func TestSearch_InvalidSortKey(t *testing.T) {
	vault := setupSearchVault(t)
	_, err := Search(vault, SearchOptions{Sort: "-"})
	if err == nil {
		t.Fatal("expected error for empty sort key")
	}
}
