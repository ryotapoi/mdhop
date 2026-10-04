package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

func copyVaultForQuery(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", name)
	dst := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, dst); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	return dst
}

func buildForQuery(t *testing.T, vaultPath string) {
	t.Helper()
	if _, err := Build(vaultPath); err != nil {
		t.Fatalf("build: %v", err)
	}
}

func setupFullVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_build_full")
	buildForQuery(t, vault)
	return vault
}

// --- Entry point tests ---

func TestQueryEntryFile(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Entry.Type, "note")
	}
	if res.Entry.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Entry.Path, "Index.md")
	}
}

func TestQueryEntryTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "overview"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypeTag {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
	if res.Entry.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "#overview")
	}
}

func TestQueryEntryTagWithHash(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "#overview"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypeTag {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
	if res.Entry.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "#overview")
	}
}

func TestQueryEntryPhantom(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypePhantom {
		t.Errorf("type = %q, want %q", res.Entry.Type, "phantom")
	}
	if res.Entry.Name != "Missing" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "Missing")
	}
}

func TestQueryEntryNameNote(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Name: "Design"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Entry.Type, "note")
	}
	if res.Entry.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Entry.Path, "Design.md")
	}
}

func TestQueryEntryNameTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Name: "#overview"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != NodeTypeTag {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
}

func TestQueryEntryMissingErrorsAreEntryNotFound(t *testing.T) {
	vault := setupFullVault(t)
	tests := []struct {
		name    string
		spec    EntrySpec
		context string
	}{
		{name: "tag", spec: EntrySpec{Tag: "missing"}, context: "tag not in index: #missing"},
		{name: "phantom", spec: EntrySpec{Phantom: "UnindexedPhantom"}, context: "phantom not in index: UnindexedPhantom"},
		{name: "name", spec: EntrySpec{Name: "UnindexedName"}, context: "name not found: UnindexedName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Query(vault, tt.spec, QueryOptions{})
			if !errors.Is(err, ErrEntryNotFound) {
				t.Fatalf("error = %v, want ErrEntryNotFound", err)
			}
			if !strings.Contains(err.Error(), tt.context) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.context)
			}
		})
	}
}

func TestQueryEntryNameAmbiguous(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_query_ambiguous_name")
	buildForQuery(t, vault)

	_, err := Query(vault, EntrySpec{Name: "A"}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %q, want containing 'ambiguous'", err.Error())
	}
}

func TestQueryEntryNameRootPriority(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_query_ambiguous_name")
	// Add A.md at root — root-priority resolves [[A]] to root file.
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A at root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildForQuery(t, vault)

	result, err := Query(vault, EntrySpec{Name: "A"}, QueryOptions{})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if result.Entry.Path != "A.md" {
		t.Errorf("Path = %q, want %q", result.Entry.Path, "A.md")
	}
}

func TestQueryErrorMultipleEntry(t *testing.T) {
	vault := setupFullVault(t)
	_, err := Query(vault, EntrySpec{File: "Index.md", Tag: "overview"}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple entry") {
		t.Errorf("error = %q, want containing 'multiple entry'", err.Error())
	}
}

func TestQueryErrorNoEntry(t *testing.T) {
	vault := setupFullVault(t)
	_, err := Query(vault, EntrySpec{}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no entry") {
		t.Errorf("error = %q, want containing 'no entry'", err.Error())
	}
}

func TestQueryErrorFileNotFound(t *testing.T) {
	vault := setupFullVault(t)
	_, err := Query(vault, EntrySpec{File: "X.md"}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrFileNotRegistered) {
		t.Errorf("error = %q, want ErrFileNotRegistered", err.Error())
	}
}

func TestQueryErrorNoDB(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_build_full") // no build
	_, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "index not found") {
		t.Errorf("error = %q, want containing 'index not found'", err.Error())
	}
}

// --- Backlinks tests ---

func TestQueryBacklinks(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Design.md is linked from Index.md and sub/Impl.md.
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "Index")
	expectContains(t, names, "Impl")
	if len(res.Backlinks) != 2 {
		t.Errorf("backlinks count = %d, want 2", len(res.Backlinks))
	}
	for _, bl := range res.Backlinks {
		if bl.Type != NodeTypeNote {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksPhantom(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "Index")
	if len(res.Backlinks) != 1 {
		t.Errorf("backlinks count = %d, want 1", len(res.Backlinks))
	}
	for _, bl := range res.Backlinks {
		if bl.Type != NodeTypeNote {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "overview"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "Design")
	expectContains(t, names, "Index")
	if len(res.Backlinks) != 2 {
		t.Errorf("backlinks count = %d, want 2", len(res.Backlinks))
	}
	for _, bl := range res.Backlinks {
		if bl.Type != NodeTypeNote {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksLimit(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Relations: []string{"backlinks"},
		Limit:     intPtr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Backlinks) != 1 {
		t.Errorf("backlinks count = %d, want 1", len(res.Backlinks))
	}
}

func TestQueryBacklinksDistinct(t *testing.T) {
	vault := setupFullVault(t)
	// sub/Impl.md is linked from Index.md thrice (wikilink + markdown + relative wikilink).
	res, err := Query(vault, EntrySpec{File: "sub/Impl.md"}, QueryOptions{Relations: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, bl := range res.Backlinks {
		if bl.Name == "Index" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Index appears %d times, want 1", count)
	}
}

// --- Outgoing tests ---

func TestQueryOutgoing(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Relations: []string{"outgoing"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Index.md → Design (×3 but distinct), sub/Impl (×3 but distinct), Missing (phantom). Including tags, no self-link.
	names := nodeNames(res.Outgoing)
	expectContains(t, names, "Design")
	expectContains(t, names, "Impl")
	expectContains(t, names, "Missing")
	if len(res.Outgoing) != 7 {
		t.Errorf("outgoing count = %d, want 7, got %v", len(res.Outgoing), names)
	}
	wantTypes := map[string]NodeType{"Design": NodeTypeNote, "Impl": NodeTypeNote, "Missing": NodeTypePhantom}
	for _, o := range res.Outgoing {
		if wantType, ok := wantTypes[o.Name]; ok {
			if o.Type != wantType {
				t.Errorf("outgoing %s: type = %q, want %q", o.Name, o.Type, wantType)
			}
		}
	}
}

func TestQueryOutgoingPhantomEntry(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Relations: []string{"outgoing"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outgoing == nil || len(res.Outgoing) != 0 {
		t.Errorf("outgoing = %v, want selected empty slice for phantom entry", res.Outgoing)
	}
}

// --- Two-hop relation tests ---

func TestQueryTwoHopSharedDestinations(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.TwoHop) == 0 {
		t.Fatal("expected related targets")
	}
	foundDesign := false
	for _, target := range res.TwoHop {
		if target.Path == "Index.md" {
			t.Fatal("entry appeared as its own related target")
		}
		if target.Name == "Design" {
			foundDesign = true
			if len(target.Relation) == 0 {
				t.Fatal("Design has no shared destination")
			}
		}
	}
	if !foundDesign {
		t.Fatalf("Design absent from twohop: %+v", res.TwoHop)
	}
}

func TestQueryTwoHopNonNoteEmpty(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Relations: []string{FieldQueryTwoHop}})
	if err != nil {
		t.Fatal(err)
	}
	if res.TwoHop == nil || len(res.TwoHop) != 0 {
		t.Fatalf("twohop = %+v, want selected empty relation", res.TwoHop)
	}
}

func TestQueryRelationSelectionAndPagination(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}, Limit: intPtr(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outgoing) != 1 || res.Backlinks != nil || res.TwoHop != nil {
		t.Fatalf("selected relations = %+v", res)
	}
	if res.Page.Offset != 0 || res.Page.Limit == nil || *res.Page.Limit != 1 || res.Page.NextOffset == nil || *res.Page.NextOffset != 1 {
		t.Fatalf("page = %+v", res.Page)
	}
	next, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}, Limit: intPtr(1), Offset: res.Page.NextOffset})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Outgoing) != 1 || next.Outgoing[0] == res.Outgoing[0] {
		t.Fatalf("next page = %+v", next)
	}
}

func TestQueryRelationValidation(t *testing.T) {
	vault := setupFullVault(t)
	for _, opts := range []QueryOptions{
		{Relations: []string{}},
		{Relations: []string{"unknown"}},
		{Relations: []string{FieldQueryBacklinks, FieldQueryBacklinks}},
		{Limit: intPtr(1)},
		{Relations: []string{FieldQueryBacklinks, FieldQueryOutgoing}, Limit: intPtr(1)},
		{Relations: []string{FieldQueryBacklinks}, Limit: intPtr(0)},
		{Relations: []string{FieldQueryBacklinks}, Offset: intPtr(-1)},
	} {
		if _, err := Query(vault, EntrySpec{File: "Index.md"}, opts); err == nil {
			t.Errorf("accepted %+v", opts)
		}
	}
}

func TestQueryTagsHelper(t *testing.T) {
	vault := setupFullVault(t)
	db, err := openDBChecked(vault)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _, err := findEntryNode(db, EntrySpec{File: "Index.md"})
	if err != nil {
		t.Fatal(err)
	}
	tags, err := queryTags(db, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#overview", "#project", "#status/active"} {
		expectContains(t, tags, want)
	}
	if len(tags) != 3 {
		t.Fatalf("tags = %v", tags)
	}
}

func TestQueryRelationsBeyondFormerCaps(t *testing.T) {
	vault := t.TempDir()
	var links strings.Builder
	links.WriteString("[[Anchor]]\n")
	for i := range 15 {
		name := fmt.Sprintf("Via%02d", i)
		links.WriteString("[[" + name + "]]\n")
		if err := os.WriteFile(filepath.Join(vault, name+".md"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "Anchor.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Entry.md"), []byte(links.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := range 125 {
		name := fmt.Sprintf("Peer%03d.md", i)
		if err := os.WriteFile(filepath.Join(vault, name), []byte(links.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildForQuery(t, vault)
	backlinks, err := Query(vault, EntrySpec{File: "Anchor.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}})
	if err != nil {
		t.Fatal(err)
	}
	if len(backlinks.Backlinks) != 126 {
		t.Fatalf("backlinks = %d, want 126", len(backlinks.Backlinks))
	}
	full, err := Query(vault, EntrySpec{File: "Entry.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.TwoHop) != 125 || full.Page.Limit != nil || full.Page.NextOffset != nil {
		t.Fatalf("twohop count/page = %d/%+v", len(full.TwoHop), full.Page)
	}
	for i, target := range full.TwoHop {
		want := fmt.Sprintf("Peer%03d.md", i)
		if target.Path != want {
			t.Fatalf("target %d = %s, want %s", i, target.Path, want)
		}
		if len(target.Relation) != 16 {
			t.Fatalf("%s relations = %d, want 16", target.Path, len(target.Relation))
		}
		if target.Relation[0].Path != "Anchor.md" || target.Relation[1].Path != "Via00.md" || target.Relation[15].Path != "Via14.md" {
			t.Fatalf("relation sort = %+v", target.Relation)
		}
	}
	page, err := Query(vault, EntrySpec{File: "Entry.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Limit: intPtr(100)})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.TwoHop) != 100 || page.Page.NextOffset == nil || *page.Page.NextOffset != 100 {
		t.Fatalf("first page = %d/%+v", len(page.TwoHop), page.Page)
	}
	last, err := Query(vault, EntrySpec{File: "Entry.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Limit: intPtr(100), Offset: page.Page.NextOffset})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.TwoHop) != 25 || last.TwoHop[0].Path != "Peer100.md" || last.Page.NextOffset != nil {
		t.Fatalf("last page = %d/%+v", len(last.TwoHop), last.Page)
	}
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Paths: []string{"Peer000.md", "Via00.md"}}, ViaExclude: ExcludeConfig{Paths: []string{"Via01.md"}}})
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := Query(vault, EntrySpec{File: "Entry.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.TwoHop) != 124 || filtered.TwoHop[0].Path != "Peer001.md" {
		t.Fatalf("filtered targets = %d/%+v", len(filtered.TwoHop), filtered.TwoHop[0])
	}
	if len(filtered.TwoHop[0].Relation) != 14 || !filtered.TwoHop[0].HiddenRelation {
		t.Fatalf("filtered relations = %+v", filtered.TwoHop[0])
	}
}

// --- filterLeafTags unit tests ---

func TestFilterLeafTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"single", []string{"#a"}, []string{"#a"}},
		{"ancestor chain", []string{"#a", "#a/b", "#a/b/c"}, []string{"#a/b/c"}},
		{"no ancestors", []string{"#a", "#b", "#c"}, []string{"#a", "#b", "#c"}},
		{"confusing prefix", []string{"#status", "#status2", "#status/active"}, []string{"#status/active", "#status2"}},
		{"hyphen tag", []string{"#a", "#a-1", "#a/b"}, []string{"#a-1", "#a/b"}},
		{"mixed", []string{"#a", "#a/b", "#b", "#b/c"}, []string{"#a/b", "#b/c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterLeafTags(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("filterLeafTags(%v) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("filterLeafTags(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- helpers ---

func nodeNames(nodes []NodeInfo) []string {
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	return names
}

func expectContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, item := range list {
		if item == want {
			return
		}
	}
	t.Errorf("expected %q in %v", want, list)
}
