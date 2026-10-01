package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupExcludeVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_query_exclude")
	buildForQuery(t, vault)
	return vault
}

func TestQueryBacklinksExcludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bl := range res.Backlinks {
		if bl.Path == "daily/D.md" {
			t.Error("daily/D.md should be excluded from backlinks")
		}
	}
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "B")
	expectContains(t, names, "C")
}

func TestQueryOutgoingExcludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	// A.md links to B, C, D, Missing. Exclude daily/* → D should be excluded.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"outgoing"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, og := range res.Outgoing {
		if og.Path == "daily/D.md" {
			t.Error("daily/D.md should be excluded from outgoing")
		}
	}
	names := nodeNames(res.Outgoing)
	expectContains(t, names, "B")
	expectContains(t, names, "C")
	expectContains(t, names, "Missing") // phantom survives
	found := false
	for _, og := range res.Outgoing {
		if og.Type == NodeTypePhantom && og.Name == "Missing" {
			found = true
		}
	}
	if !found {
		t.Error("phantom Missing should survive path exclusion")
	}
}

func TestQueryTagsExcludeTag(t *testing.T) {
	vault := setupExcludeVault(t)
	// B.md has #project, #daily. Exclude #daily → only #project remains.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, nil, []string{"#daily"})
	res, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields:  []string{"tags"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, tag := range res.Tags {
		if tag == "#daily" {
			t.Error("#daily should be excluded from tags")
		}
	}
	expectContains(t, res.Tags, "#project")
}

func TestQueryTwoHopExcludeTagVia(t *testing.T) {
	vault := setupExcludeVault(t)

	// B.md has #daily and #project. Outbound seeds include #daily.
	// D.md also has #daily, so without exclude: via=#daily → targets=[D].
	// First verify #daily appears as via without exclusion.
	res0, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields: []string{"twohop"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundBefore := false
	for _, th := range res0.TwoHop {
		if th.Via.Type == NodeTypeTag && th.Via.Name == "#daily" {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatal("precondition failed: #daily should be a via node without exclusion")
	}

	// With exclude: #daily via should disappear.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, nil, []string{"#daily"})
	res, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, th := range res.TwoHop {
		if th.Via.Type == NodeTypeTag && th.Via.Name == "#daily" {
			t.Error("#daily should be excluded as via in twohop")
		}
	}
}

func setupTwoHopTagTargetsVault(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[Missing]]\n[[B]]\n\n#entry #drop #keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildForQuery(t, vault)

	db, err := openDBAt(dbPath(vault))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer db.Close()

	sourceID, err := getNodeID(db, noteKey("A.md"))
	if err != nil {
		t.Fatalf("find source note: %v", err)
	}
	otherNoteID, err := upsertNote(db, "Other.md", "#drop", 0, 0)
	if err != nil {
		t.Fatalf("insert same-name note: %v", err)
	}
	otherPhantomID, err := upsertPhantom(db, "#drop")
	if err != nil {
		t.Fatalf("insert same-name phantom: %v", err)
	}
	otherAssetID, err := upsertAsset(db, "assets/drop.png", "#drop", 0)
	if err != nil {
		t.Fatalf("insert same-name asset: %v", err)
	}
	for _, targetID := range []int64{otherNoteID, otherPhantomID, otherAssetID} {
		if err := insertEdge(db, sourceID, targetID, LinkTypeWikilink, "[[same-name]]", "", "", 0, 0); err != nil {
			t.Fatalf("insert same-name target edge: %v", err)
		}
	}
	return vault
}

func assertTwoHopTagTargetsExcluded(t *testing.T, res *QueryResult, entryTagVisible bool) {
	t.Helper()
	if len(res.TwoHop) != 1 {
		t.Fatalf("twohop entries = %d, want 1", len(res.TwoHop))
	}
	entry := res.TwoHop[0]
	if entry.Via.Type != NodeTypeNote || entry.Via.Name != "A" {
		t.Fatalf("via = %+v, want note A", entry.Via)
	}
	found := map[NodeType]map[string]bool{}
	for _, target := range entry.Targets {
		if target.Type == NodeTypeTag && strings.EqualFold(target.Name, "#drop") {
			t.Error("excluded #drop tag should not appear as a target")
		}
		if found[target.Type] == nil {
			found[target.Type] = map[string]bool{}
		}
		found[target.Type][target.Name] = true
	}
	for _, target := range []struct {
		typ  NodeType
		name string
	}{
		{NodeTypeTag, "#keep"},
		{NodeTypeNote, "B"},
		{NodeTypeNote, "#drop"},
		{NodeTypePhantom, "#drop"},
		{NodeTypeAsset, "#drop"},
	} {
		if !found[target.typ][target.name] {
			t.Errorf("target (%s, %q) is missing", target.typ, target.name)
		}
	}
	if entryTagVisible && !found[NodeTypeTag]["#entry"] {
		t.Error("#entry tag should remain a target when the entry is a phantom")
	}
}

func TestQueryTwoHopExcludeTagTargetsFromTagEntry(t *testing.T) {
	vault := setupTwoHopTagTargetsVault(t)
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("exclude:\n  tags:\n    - DROP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	ef, err := NewExcludeFilter(cfg.Exclude, nil, nil)
	if err != nil {
		t.Fatalf("create exclude filter: %v", err)
	}
	res, err := Query(vault, EntrySpec{Tag: "entry"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("query tag entry: %v", err)
	}
	assertTwoHopTagTargetsExcluded(t, res, false)
}

func TestQueryTwoHopExcludeTagTargetsFromPhantomEntry(t *testing.T) {
	vault := setupTwoHopTagTargetsVault(t)
	ef, err := NewExcludeFilter(ExcludeConfig{}, nil, []string{"#DrOp"})
	if err != nil {
		t.Fatalf("create exclude filter: %v", err)
	}
	res, err := Query(vault, EntrySpec{Phantom: "Missing"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("query phantom entry: %v", err)
	}
	assertTwoHopTagTargetsExcluded(t, res, true)
}

func TestQueryTwoHopExcludeTagTargetsBeforeLimit(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("#entry #drop #keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildForQuery(t, vault)
	ef, err := NewExcludeFilter(ExcludeConfig{}, nil, []string{"drop"})
	if err != nil {
		t.Fatalf("create exclude filter: %v", err)
	}
	res, err := Query(vault, EntrySpec{Tag: "entry"}, QueryOptions{
		Fields:          []string{"twohop"},
		MaxViaPerTarget: 1,
		Exclude:         ef,
	})
	if err != nil {
		t.Fatalf("query tag entry: %v", err)
	}
	if len(res.TwoHop) != 1 || len(res.TwoHop[0].Targets) != 1 || res.TwoHop[0].Targets[0].Name != "#keep" {
		t.Fatalf("twohop targets = %+v, want only #keep after exclusion and limit", res.TwoHop)
	}
}

func TestQueryTwoHopExcludeTagTargetsDropsEmptyVia(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("#entry #drop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildForQuery(t, vault)
	ef, err := NewExcludeFilter(ExcludeConfig{}, nil, []string{"drop"})
	if err != nil {
		t.Fatalf("create exclude filter: %v", err)
	}
	res, err := Query(vault, EntrySpec{Tag: "entry"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("query tag entry: %v", err)
	}
	if len(res.TwoHop) != 0 {
		t.Fatalf("twohop entries = %+v, want no entries when every target is excluded", res.TwoHop)
	}
}

func TestQueryTwoHopExcludePathVia(t *testing.T) {
	vault := setupExcludeVault(t)
	// Exclude daily/* → D.md as via should not appear.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, th := range res.TwoHop {
		if th.Via.Path == "daily/D.md" {
			t.Error("daily/D.md should be excluded as via in twohop")
		}
	}
}

func TestQueryTwoHopExcludePathTargets(t *testing.T) {
	vault := setupExcludeVault(t)
	// Exclude daily/* → D.md should not appear as a target.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"twohop"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.TwoHop) == 0 {
		t.Fatal("expected twohop entries, got 0")
	}
	for _, th := range res.TwoHop {
		for _, target := range th.Targets {
			if target.Path == "daily/D.md" {
				t.Errorf("daily/D.md should be excluded from twohop targets (via %s)", th.Via.Name)
			}
		}
	}
}

func TestQuerySnippetExcludePath(t *testing.T) {
	vault := setupExcludeVault(t)
	// Snippets for A.md: sources that link to A are B, C, D, T.
	// Exclude daily/* → D.md should not appear as source.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 1,
		Exclude:        ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range res.Snippets {
		if s.SourcePath == "daily/D.md" {
			t.Error("daily/D.md should be excluded from snippets")
		}
	}
}

func TestQueryExcludeNone(t *testing.T) {
	vault := setupExcludeVault(t)
	// nil exclude → all results.
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A.md has backlinks from B, C, D, T.
	if len(res.Backlinks) != 4 {
		names := nodeNames(res.Backlinks)
		t.Errorf("backlinks count = %d, want 4, got %v", len(res.Backlinks), names)
	}
}

func TestQueryEntryNodeNeverExcluded(t *testing.T) {
	vault := setupExcludeVault(t)
	// D.md is in daily/. Using D.md as entry with exclude "daily/*" → entry itself should resolve.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "daily/D.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entry.Path != "daily/D.md" {
		t.Errorf("entry path = %q, want %q", res.Entry.Path, "daily/D.md")
	}
}

func TestQueryNoExcludeIgnoresConfig(t *testing.T) {
	vault := setupExcludeVault(t)
	content := `exclude:
  paths:
    - "daily/*"
`
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Load config and create filter (simulating normal path).
	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatal(err)
	}
	ef, err := NewExcludeFilter(cfg.Exclude, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// With exclude → D.md is excluded.
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, bl := range res.Backlinks {
		if bl.Path == "daily/D.md" {
			t.Error("daily/D.md should be excluded when config is applied")
		}
	}

	// --no-exclude: pass nil (simulating CLI skipping config).
	res2, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, bl := range res2.Backlinks {
		if bl.Path == "daily/D.md" {
			found = true
		}
	}
	if !found {
		t.Error("daily/D.md should be present when --no-exclude")
	}
}

func TestQueryExcludeCLIAddsToConfig(t *testing.T) {
	vault := setupExcludeVault(t)
	// Config excludes daily/*, CLI adds templates/*.
	cfg := ExcludeConfig{Paths: []string{"daily/*"}}
	ef, _ := NewExcludeFilter(cfg, []string{"templates/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bl := range res.Backlinks {
		if bl.Path == "daily/D.md" || bl.Path == "templates/T.md" {
			t.Errorf("%s should be excluded", bl.Path)
		}
	}
	// B.md and C.md should remain.
	if len(res.Backlinks) != 2 {
		names := nodeNames(res.Backlinks)
		t.Errorf("backlinks count = %d, want 2, got %v", len(res.Backlinks), names)
	}
}
