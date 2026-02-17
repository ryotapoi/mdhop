package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if err := Build(vaultPath); err != nil {
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
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "note" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "note")
	}
	if res.Entry.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Entry.Path, "Index.md")
	}
}

func TestQueryEntryTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "overview"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "tag" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
	if res.Entry.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "#overview")
	}
}

func TestQueryEntryTagWithHash(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "#overview"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "tag" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
	if res.Entry.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "#overview")
	}
}

func TestQueryEntryPhantom(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "phantom" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "phantom")
	}
	if res.Entry.Name != "Missing" {
		t.Errorf("name = %q, want %q", res.Entry.Name, "Missing")
	}
}

func TestQueryEntryNameNote(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Name: "Design"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "note" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "note")
	}
	if res.Entry.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Entry.Path, "Design.md")
	}
}

func TestQueryEntryNameTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Name: "#overview"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Type != "tag" {
		t.Errorf("type = %q, want %q", res.Entry.Type, "tag")
	}
}

func TestQueryEntryNameNotFound(t *testing.T) {
	vault := setupFullVault(t)
	_, err := Query(vault, EntrySpec{Name: "NoSuch"}, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want containing 'not found'", err.Error())
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
	if !strings.Contains(err.Error(), "not in index") {
		t.Errorf("error = %q, want containing 'not in index'", err.Error())
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
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{Fields: []string{"backlinks"}})
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
		if bl.Type != "note" {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksPhantom(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "Index")
	if len(res.Backlinks) != 1 {
		t.Errorf("backlinks count = %d, want 1", len(res.Backlinks))
	}
	for _, bl := range res.Backlinks {
		if bl.Type != "note" {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksTag(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Tag: "overview"}, QueryOptions{Fields: []string{"backlinks"}})
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
		if bl.Type != "note" {
			t.Errorf("backlink %s: type = %q, want %q", bl.Name, bl.Type, "note")
		}
	}
}

func TestQueryBacklinksLimit(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:       []string{"backlinks"},
		MaxBacklinks: 1,
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
	res, err := Query(vault, EntrySpec{File: "sub/Impl.md"}, QueryOptions{Fields: []string{"backlinks"}})
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
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"outgoing"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Index.md → Design (×3 but distinct), sub/Impl (×3 but distinct), Missing (phantom). No tags, no self-link.
	names := nodeNames(res.Outgoing)
	expectContains(t, names, "Design")
	expectContains(t, names, "Impl")
	expectContains(t, names, "Missing")
	if len(res.Outgoing) != 3 {
		t.Errorf("outgoing count = %d, want 3, got %v", len(res.Outgoing), names)
	}
	wantTypes := map[string]string{"Design": "note", "Impl": "note", "Missing": "phantom"}
	for _, o := range res.Outgoing {
		if wantType, ok := wantTypes[o.Name]; ok {
			if o.Type != wantType {
				t.Errorf("outgoing %s: type = %q, want %q", o.Name, o.Type, wantType)
			}
		}
	}
}

func TestQueryOutgoingExcludesTags(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"outgoing"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, o := range res.Outgoing {
		if o.Type == "tag" {
			t.Errorf("outgoing contains tag: %s", o.Name)
		}
	}
}

func TestQueryOutgoingPhantomEntry(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Fields: []string{"outgoing"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outgoing != nil {
		t.Errorf("outgoing = %v, want nil for phantom entry", res.Outgoing)
	}
}

// --- Tags tests ---

func TestQueryTags(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"tags"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Index.md has: #project, #status/active (expanded: #status, #status/active), #overview.
	// Leaf filter: #status excluded because #status/active exists.
	expectContains(t, res.Tags, "#overview")
	expectContains(t, res.Tags, "#project")
	expectContains(t, res.Tags, "#status/active")
	if len(res.Tags) != 3 {
		t.Errorf("tags count = %d, want 3, got %v", len(res.Tags), res.Tags)
	}
}

func TestQueryTagsLeafFilter(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"tags"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, tag := range res.Tags {
		if tag == "#status" {
			t.Error("tags should not contain ancestor #status")
		}
	}
}

func TestQueryTagsNonNote(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Fields: []string{"tags"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Tags != nil {
		t.Errorf("tags = %v, want nil for phantom entry", res.Tags)
	}
}

// --- TwoHop tests ---

func TestQueryTwoHop(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"twohop"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.TwoHop) == 0 {
		t.Fatal("expected twohop entries, got 0")
	}
	// Index→Design, Index→sub/Impl, Index→Missing, Index→Index(self), Index→tags...
	// For outbound seed: via nodes are all targets of Index.
	// e.g., via=Design → targets=[sub/Impl] (Impl also links to Design)
	// via=sub/Impl → targets=[Design] (Design links to sub/Impl)
	viaNames := make(map[string]bool)
	for _, entry := range res.TwoHop {
		viaNames[entry.Via.Name] = true
	}
	if !viaNames["Design"] {
		t.Error("expected Design as a via node")
	}
	if !viaNames["Impl"] {
		t.Error("expected Impl as a via node")
	}
	for _, entry := range res.TwoHop {
		if entry.Via.Type != "note" && entry.Via.Type != "phantom" && entry.Via.Type != "tag" {
			t.Errorf("via %s: unexpected type %q", entry.Via.Name, entry.Via.Type)
		}
		for _, target := range entry.Targets {
			if target.Type != "note" && target.Type != "phantom" && target.Type != "tag" {
				t.Errorf("target %s via %s: unexpected type %q", target.Name, entry.Via.Name, target.Type)
			}
		}
	}
}

func TestQueryTwoHopMaxLimit(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{
		Fields:    []string{"twohop"},
		MaxTwoHop: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.TwoHop) > 1 {
		t.Errorf("twohop count = %d, want <= 1", len(res.TwoHop))
	}
}

func TestQueryTwoHopMaxViaPerTarget(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{
		Fields:          []string{"twohop"},
		MaxViaPerTarget: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, entry := range res.TwoHop {
		if len(entry.Targets) > 1 {
			t.Errorf("via %s has %d targets, want <= 1", entry.Via.Name, len(entry.Targets))
		}
	}
}

func TestQueryTwoHopNoSelf(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"twohop"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, entry := range res.TwoHop {
		for _, target := range entry.Targets {
			if target.Path == "Index.md" {
				t.Errorf("Index.md appears as twohop target via %s", entry.Via.Name)
			}
		}
	}
}

func TestQueryTwoHopTagVia(t *testing.T) {
	vault := setupFullVault(t)
	// Index.md and Design.md both have #overview. So twohop from Index should have
	// via=#overview → targets=[Design] (or vice versa).
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"twohop"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, entry := range res.TwoHop {
		if entry.Via.Type == "tag" && entry.Via.Name == "#overview" {
			found = true
			targetNames := nodeNames(entry.Targets)
			expectContains(t, targetNames, "Design")
		}
	}
	if !found {
		vias := make([]string, len(res.TwoHop))
		for i, e := range res.TwoHop {
			vias[i] = fmt.Sprintf("%s(%s)", e.Via.Name, e.Via.Type)
		}
		t.Errorf("expected #overview as tag-via, got vias: %v", vias)
	}
}

func TestQueryTwoHopPhantom(t *testing.T) {
	vault := setupFullVault(t)
	// Missing is a phantom linked from Index.md.
	// Inbound seed: sources linking to Missing = [Index.md].
	// For each via (Index.md), find other targets of Index.md (excluding Missing).
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{Fields: []string{"twohop"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.TwoHop) == 0 {
		t.Fatal("expected twohop entries for phantom, got 0")
	}
	// Via = Index.md, targets should include Design, sub/Impl, etc. (but not Missing).
	found := false
	for _, entry := range res.TwoHop {
		if entry.Via.Type != "note" && entry.Via.Type != "phantom" && entry.Via.Type != "tag" {
			t.Errorf("via %s: unexpected type %q", entry.Via.Name, entry.Via.Type)
		}
		for _, target := range entry.Targets {
			if target.Type != "note" && target.Type != "phantom" && target.Type != "tag" {
				t.Errorf("target %s via %s: unexpected type %q", target.Name, entry.Via.Name, target.Type)
			}
		}
		if entry.Via.Name == "Index" {
			found = true
			if entry.Via.Type != "note" {
				t.Errorf("via Index: type = %q, want %q", entry.Via.Type, "note")
			}
			for _, target := range entry.Targets {
				if target.Name == "Missing" {
					t.Error("Missing should not appear as a target")
				}
			}
		}
	}
	if !found {
		t.Error("expected Index as a via node for phantom twohop")
	}
}

// --- Head tests ---

func TestQueryHead(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{
		Fields:      []string{"head"},
		IncludeHead: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Index.md has frontmatter (3 lines: ---, tags, ---), then "# Index", "", "Welcome to the vault."
	want := []string{"# Index", "", "Welcome to the vault."}
	if len(res.Head) != len(want) {
		t.Fatalf("head lines = %d, want %d: %v", len(res.Head), len(want), res.Head)
	}
	for i, line := range want {
		if res.Head[i] != line {
			t.Errorf("head[%d] = %q, want %q", i, res.Head[i], line)
		}
	}
}

func TestQueryHeadNoFrontmatter(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:      []string{"head"},
		IncludeHead: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Design.md has no frontmatter: "# Design", ""
	want := []string{"# Design", ""}
	if len(res.Head) != len(want) {
		t.Fatalf("head lines = %d, want %d: %v", len(res.Head), len(want), res.Head)
	}
	for i, line := range want {
		if res.Head[i] != line {
			t.Errorf("head[%d] = %q, want %q", i, res.Head[i], line)
		}
	}
}

func TestQueryHeadStale(t *testing.T) {
	vault := setupFullVault(t)
	// Modify the file after build to make it stale.
	time.Sleep(1100 * time.Millisecond) // ensure mtime changes (1s resolution)
	path := filepath.Join(vault, "Index.md")
	content, _ := os.ReadFile(path)
	os.WriteFile(path, append(content, []byte("\nmodified\n")...), 0o644)

	_, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{
		Fields:      []string{"head"},
		IncludeHead: 3,
	})
	if err == nil {
		t.Fatal("expected stale error, got nil")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("error = %q, want containing 'stale'", err.Error())
	}
}

func TestQueryHeadNotRequested(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{
		Fields:      []string{"head"},
		IncludeHead: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Head != nil {
		t.Errorf("head = %v, want nil when IncludeHead=0", res.Head)
	}
}

// --- Snippet tests ---

func TestQuerySnippet(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Snippets) == 0 {
		t.Fatal("expected snippets, got 0")
	}
	// Design.md is linked from Index.md and sub/Impl.md.
	sourcePaths := make(map[string]bool)
	for _, s := range res.Snippets {
		sourcePaths[s.SourcePath] = true
	}
	if !sourcePaths["Index.md"] {
		t.Error("expected snippet from Index.md")
	}
	if !sourcePaths["sub/Impl.md"] {
		t.Error("expected snippet from sub/Impl.md")
	}
	// Check that lines are populated.
	for _, s := range res.Snippets {
		if len(s.Lines) == 0 {
			t.Errorf("snippet from %s has 0 lines", s.SourcePath)
		}
	}
}

func TestQuerySnippetStale(t *testing.T) {
	vault := setupFullVault(t)
	// Modify a source file after build.
	time.Sleep(1100 * time.Millisecond)
	path := filepath.Join(vault, "Index.md")
	content, _ := os.ReadFile(path)
	os.WriteFile(path, append(content, []byte("\nmodified\n")...), 0o644)

	_, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 1,
	})
	if err == nil {
		t.Fatal("expected stale error, got nil")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("error = %q, want containing 'stale'", err.Error())
	}
}

// --- Fields filter tests ---

func TestQueryFieldsFilter(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Index.md"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Backlinks == nil {
		t.Error("backlinks should be populated")
	}
	if res.Outgoing != nil {
		t.Error("outgoing should be nil when not requested")
	}
	if res.TwoHop != nil {
		t.Error("twohop should be nil when not requested")
	}
	if res.Tags != nil {
		t.Error("tags should be nil when not requested")
	}
}

// --- Snippet context lines tests ---

func TestQuerySnippetContextLines(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Snippets) != 4 {
		t.Fatalf("snippets count = %d, want 4", len(res.Snippets))
	}

	// Helper to find snippets by source path and line start.
	findSnippet := func(sourcePath string, lineStart int) *SnippetEntry {
		for i := range res.Snippets {
			if res.Snippets[i].SourcePath == sourcePath && res.Snippets[i].LineStart == lineStart {
				return &res.Snippets[i]
			}
		}
		return nil
	}

	// Verify line_start == line_end assumption: for single-line links with context=2,
	// a non-boundary snippet should have exactly 2*context+1 = 5 lines.
	// sub/Impl.md L10 (file 12 lines): start=max(0,10-2-1)=7 → LineStart=8, end=min(10+2,12)=12 → LineEnd=12
	s := findSnippet("sub/Impl.md", 8)
	if s == nil {
		t.Fatal("snippet from sub/Impl.md LineStart=8 not found")
	}
	if s.LineEnd != 12 {
		t.Errorf("sub/Impl.md snippet: LineEnd = %d, want 12", s.LineEnd)
	}
	if len(s.Lines) != 5 {
		t.Errorf("sub/Impl.md snippet: lines count = %d, want 5 (confirms line_start==line_end)", len(s.Lines))
	}

	// Index.md L10: start=7 → LineStart=8, end=12 → LineEnd=12
	s = findSnippet("Index.md", 8)
	if s == nil {
		t.Fatal("snippet from Index.md LineStart=8 not found")
	}
	if s.LineEnd != 12 {
		t.Errorf("Index.md L10 snippet: LineEnd = %d, want 12", s.LineEnd)
	}
	if len(s.Lines) != 5 {
		t.Errorf("Index.md L10 snippet: lines count = %d, want 5", len(s.Lines))
	}

	// Index.md L14: start=11 → LineStart=12, end=min(16,16)=16 → LineEnd=16
	s = findSnippet("Index.md", 12)
	if s == nil {
		t.Fatal("snippet from Index.md LineStart=12 not found")
	}
	if s.LineEnd != 16 {
		t.Errorf("Index.md L14 snippet: LineEnd = %d, want 16", s.LineEnd)
	}
	if len(s.Lines) != 5 {
		t.Errorf("Index.md L14 snippet: lines count = %d, want 5", len(s.Lines))
	}

	// Index.md L16: start=13 → LineStart=14, end=min(18,16)=16 → LineEnd=16
	s = findSnippet("Index.md", 14)
	if s == nil {
		t.Fatal("snippet from Index.md LineStart=14 not found")
	}
	if s.LineEnd != 16 {
		t.Errorf("Index.md L16 snippet: LineEnd = %d, want 16", s.LineEnd)
	}
	if len(s.Lines) != 3 {
		t.Errorf("Index.md L16 snippet: lines count = %d, want 3", len(s.Lines))
	}
}

func TestQuerySnippetBoundary(t *testing.T) {
	vault := setupFullVault(t)
	res, err := Query(vault, EntrySpec{File: "Design.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range res.Snippets {
		switch s.SourcePath {
		case "sub/Impl.md":
			// 12-line file, context=20 should clamp to full file.
			if s.LineStart != 1 {
				t.Errorf("sub/Impl.md: LineStart = %d, want 1", s.LineStart)
			}
			if s.LineEnd != 12 {
				t.Errorf("sub/Impl.md: LineEnd = %d, want 12", s.LineEnd)
			}
			if len(s.Lines) != 12 {
				t.Errorf("sub/Impl.md: lines count = %d, want 12", len(s.Lines))
			}
		case "Index.md":
			// 16-line file, context=20 should clamp to full file.
			if s.LineStart != 1 {
				t.Errorf("Index.md (L%d): LineStart = %d, want 1", s.LineStart, s.LineStart)
			}
			if s.LineEnd != 16 {
				t.Errorf("Index.md: LineEnd = %d, want 16", s.LineEnd)
			}
			if len(s.Lines) != 16 {
				t.Errorf("Index.md: lines count = %d, want 16", len(s.Lines))
			}
		}
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
