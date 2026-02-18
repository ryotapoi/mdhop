package core

import (
	"os"
	"path/filepath"
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
	// B.md and C.md should remain.
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "B")
	expectContains(t, names, "C")
}

func TestQueryBacklinksExcludeMultiplePaths(t *testing.T) {
	vault := setupExcludeVault(t)
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*", "templates/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"backlinks"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bl := range res.Backlinks {
		if bl.Path == "daily/D.md" || bl.Path == "templates/T.md" {
			t.Errorf("%s should be excluded from backlinks", bl.Path)
		}
	}
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
		if th.Via.Type == "tag" && th.Via.Name == "#daily" {
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
		if th.Via.Type == "tag" && th.Via.Name == "#daily" {
			t.Error("#daily should be excluded as via in twohop")
		}
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

func TestQueryExcludePhantomSurvives(t *testing.T) {
	vault := setupExcludeVault(t)
	// A.md has outgoing to Missing (phantom). Path exclusion should not remove phantom.
	ef, _ := NewExcludeFilter(ExcludeConfig{}, []string{"daily/*"}, nil)
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:  []string{"outgoing"},
		Exclude: ef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, og := range res.Outgoing {
		if og.Type == "phantom" && og.Name == "Missing" {
			found = true
		}
	}
	if !found {
		t.Error("phantom Missing should survive path exclusion")
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
	// Write config with exclusion.
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
