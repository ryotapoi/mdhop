package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func setupMetaCheckVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_meta_check")
	buildForQuery(t, vault)
	return vault
}

func TestMetaCheck_PathKind(t *testing.T) {
	vault := setupMetaCheckVault(t)

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ./guide.md resolves, https URL is allowed → only ./missing.md is an issue.
	if len(result.Issues) != 1 {
		t.Fatalf("issues = %+v, want 1", result.Issues)
	}
	is := result.Issues[0]
	if is.Value != "./missing.md" || is.Reason != ReasonNotFound {
		t.Errorf("issue = %+v, want ./missing.md not_found", is)
	}
	if is.SourcePath != "docs/index.md" || is.Key != "sources" {
		t.Errorf("issue source/key = %s/%s, want docs/index.md/sources", is.SourcePath, is.Key)
	}
	if is.Line != 4 {
		t.Errorf("issue line = %d, want 4", is.Line)
	}
}

func TestMetaCheck_PersistsValueLinesAcrossUpdate(t *testing.T) {
	vault := t.TempDir()
	notePath := filepath.Join(vault, "Index.md")
	initial := "---\nsources:\n  - Missing.md\n  - Missing.md\n---\n"
	if err := os.WriteFile(notePath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial note: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	assertLines := func(want []int) {
		t.Helper()
		result, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"sources"}, Kind: MetaKindPath})
		if err != nil {
			t.Fatalf("meta-check: %v", err)
		}
		if len(result.Issues) != len(want) {
			t.Fatalf("issues = %+v, want %d", result.Issues, len(want))
		}
		for i, issue := range result.Issues {
			if issue.Value != "Missing.md" || issue.Line != want[i] {
				t.Errorf("issue[%d] = %+v, want Missing.md at line %d", i, issue, want[i])
			}
		}
	}
	assertLines([]int{3, 4})

	updated := "---\nsources:\n  - Missing.md\n\n  - Missing.md\n---\n"
	if err := os.WriteFile(notePath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated note: %v", err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"Index.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	assertLines([]int{3, 5})
}

func TestMetaCheck_BuildReplacesLegacyMetaSchema(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "Index.md"), []byte("---\nsources: Missing.md\n---\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath(vault)), 0o755); err != nil {
		t.Fatalf("create legacy index directory: %v", err)
	}
	legacy, err := openDBAt(dbPath(vault))
	if err != nil {
		t.Fatalf("open legacy DB: %v", err)
	}
	if _, err := legacy.Exec(`CREATE TABLE meta (
		id INTEGER PRIMARY KEY, node_id INTEGER NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL,
		sort_value TEXT, value_type TEXT)`); err != nil {
		legacy.Close()
		t.Fatalf("create legacy meta table: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy DB: %v", err)
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("build replacement index: %v", err)
	}
	result, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"sources"}, Kind: MetaKindPath})
	if err != nil {
		t.Fatalf("meta-check after build: %v", err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Line != 2 {
		t.Fatalf("issues = %+v, want Missing.md at line 2", result.Issues)
	}
}

func TestMetaCheckResolvesNFCValueToNFDPath(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "Cafe\u0301.md"), []byte("# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}
	note := "---\nsources:\n  - Caf\u00e9.md\n---\n"
	if err := os.WriteFile(filepath.Join(vault, "Index.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("issues = %+v, want none", result.Issues)
	}
}

func TestMetaCheckPathKindDirectoryReferences(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "docs", "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	note := "---\nsources:\n  - ./assets/\n  - ./missing-dir/\n  - ../../outside/\n  - docs\n---\n"
	if err := os.WriteFile(filepath.Join(vault, "docs", "Index.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}

	if len(result.Issues) != 3 {
		t.Fatalf("issues = %+v, want 3", result.Issues)
	}
	got := map[string]MetaIssueReason{}
	for _, issue := range result.Issues {
		got[issue.Value] = issue.Reason
	}
	if got["./missing-dir/"] != ReasonNotFound {
		t.Fatalf("missing dir issue = %v, want not_found; issues=%+v", got["./missing-dir/"], result.Issues)
	}
	if got["../../outside/"] != ReasonVaultEscape {
		t.Fatalf("escaping dir issue = %v, want vault_escape; issues=%+v", got["../../outside/"], result.Issues)
	}
	if got["docs"] != ReasonNotFound {
		t.Fatalf("docs without slash issue = %v, want existing non-directory behavior not_found; issues=%+v", got["docs"], result.Issues)
	}
	if _, ok := got["./assets/"]; ok {
		t.Fatalf("existing directory reported as issue: %+v", result.Issues)
	}
}

func TestMetaCheck_WikilinkKind(t *testing.T) {
	vault := setupMetaCheckVault(t)

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"related"},
		Kind: MetaKindWikilink,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// [[guide]] resolves → only [[Nonexistent]] is an issue.
	if len(result.Issues) != 1 {
		t.Fatalf("issues = %+v, want 1", result.Issues)
	}
	if result.Issues[0].Value != "[[Nonexistent]]" || result.Issues[0].Reason != ReasonNotFound {
		t.Errorf("issue = %+v, want [[Nonexistent]] not_found", result.Issues[0])
	}
}

func TestMetaCheck_RequiresKey(t *testing.T) {
	vault := setupMetaCheckVault(t)
	if _, err := MetaCheck(vault, MetaCheckOptions{Kind: MetaKindPath}); err == nil {
		t.Fatal("expected error when no --key given")
	} else if strings.HasPrefix(err.Error(), "meta-check:") {
		t.Errorf("error = %q, must not include command prefix", err.Error())
	}
}

func TestMetaCheck_InvalidKind(t *testing.T) {
	vault := setupMetaCheckVault(t)
	if _, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"sources"}, Kind: "bogus"}); err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestMetaCheck_AutoKind(t *testing.T) {
	vault := t.TempDir()
	dirs := []string{
		"docs/assets",
		"docs/a",
		"docs/b",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for _, p := range []string{"docs/guide.md", "docs/a/Ambig.md", "docs/b/Ambig.md"} {
		if err := os.WriteFile(filepath.Join(vault, p), []byte("# note\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	note := `---
sources:
  - "./guide.md"
  - "[[guide]]"
  - https://example.com/external
  - "[[broken"
  - "[[Missing]]"
  - "./missing-path.md"
  - "../../outside/"
  - Ambig
  - "./missing-dir/"
bare_wikilink:
  - [[BareNote]]
---
`
	if err := os.WriteFile(filepath.Join(vault, "docs/index.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindAuto,
	})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}

	want := map[string]MetaIssueReason{
		"[[broken":          ReasonNotWikilink,
		"[[Missing]]":       ReasonNotFound,
		"./missing-path.md": ReasonNotFound,
		"../../outside/":    ReasonVaultEscape,
		"Ambig":             ReasonAmbiguous,
		"./missing-dir/":    ReasonNotFound,
	}
	if len(result.Issues) != len(want) {
		t.Fatalf("issues = %+v, want %d", result.Issues, len(want))
	}
	got := map[string]MetaIssueReason{}
	for _, issue := range result.Issues {
		if issue.SourcePath != "docs/index.md" || issue.Key != "sources" {
			t.Errorf("unexpected issue source/key: %+v", issue)
		}
		got[issue.Value] = issue.Reason
	}
	for value, reason := range want {
		if got[value] != reason {
			t.Errorf("issue for %q = %q, want %q", value, got[value], reason)
		}
	}

	bareResult, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"bare_wikilink"},
		Kind: MetaKindAuto,
	})
	if err != nil {
		t.Fatalf("meta-check bare_wikilink: %v", err)
	}
	if len(bareResult.Issues) != 0 {
		t.Fatalf("bare wikilink issues = %+v, want none (not indexed in meta table)", bareResult.Issues)
	}
}

func TestMetaCheckExcludedTargetsAreResolveCandidates(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "excluded"), 0o755); err != nil {
		t.Fatalf("mkdir excluded: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "mdhop.toml"), []byte("[build]\nexclude_paths = ['excluded/**']\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	source := `---
path_values:
  - /excluded/Note.md
  - /excluded/image.png
  - /missing.md
wikilink_values:
  - "[[Note]]"
  - "[[image.png]]"
  - "[[Missing]]"
auto_values:
  - /excluded/Note.md
  - "[[image.png]]"
  - /missing.md
---
`
	if err := os.WriteFile(filepath.Join(vault, "Source.md"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	excludedNote := "---\npath_values: /missing-from-excluded.md\n---\n[[Missing]]\n#excluded\n"
	if err := os.WriteFile(filepath.Join(vault, "excluded", "Note.md"), []byte(excludedNote), 0o644); err != nil {
		t.Fatalf("write excluded note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "excluded", "image.png"), []byte("image"), 0o644); err != nil {
		t.Fatalf("write excluded asset: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, node := range queryNodes(t, dbPath(vault), NodeTypeNote) {
		if node.path == "excluded/Note.md" {
			t.Fatal("excluded note was indexed")
		}
	}
	for _, node := range queryNodes(t, dbPath(vault), NodeTypeAsset) {
		if node.path == "excluded/image.png" {
			t.Fatal("excluded asset was indexed")
		}
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var excludedSources int
	if err := db.QueryRow(`SELECT COUNT(*) FROM meta m JOIN nodes n ON n.id = m.node_id WHERE n.path = 'excluded/Note.md'`).Scan(&excludedSources); err != nil {
		t.Fatalf("count excluded meta: %v", err)
	}
	if excludedSources != 0 {
		t.Fatalf("excluded note meta rows = %d, want 0", excludedSources)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	beforeMeta := countMeta(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))
	beforeSource, err := os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil {
		t.Fatalf("read source before meta-check: %v", err)
	}
	beforeExcluded, err := os.ReadFile(filepath.Join(vault, "excluded", "Note.md"))
	if err != nil {
		t.Fatalf("read excluded note before meta-check: %v", err)
	}

	tests := []struct {
		key  string
		kind MetaValueKind
		want map[string]MetaIssueReason
	}{
		{"path_values", MetaKindPath, map[string]MetaIssueReason{"/missing.md": ReasonNotFound}},
		{"wikilink_values", MetaKindWikilink, map[string]MetaIssueReason{"[[Missing]]": ReasonNotFound}},
		{"auto_values", MetaKindAuto, map[string]MetaIssueReason{"/missing.md": ReasonNotFound}},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			result, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{tt.key}, Kind: tt.kind})
			if err != nil {
				t.Fatalf("meta-check: %v", err)
			}
			got := make(map[string]MetaIssueReason, len(result.Issues))
			for _, issue := range result.Issues {
				got[issue.Value] = issue.Reason
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issues = %#v, want %#v", got, tt.want)
			}
		})
	}

	if got := countNotes(t, dbPath(vault)); got != beforeNotes {
		t.Errorf("note count after meta-check = %d, want %d", got, beforeNotes)
	}
	if got := countMeta(t, dbPath(vault)); got != beforeMeta {
		t.Errorf("meta count after meta-check = %d, want %d", got, beforeMeta)
	}
	if got := countEdges(t, dbPath(vault)); got != beforeEdges {
		t.Errorf("edge count after meta-check = %d, want %d", got, beforeEdges)
	}
	if got, err := os.ReadFile(filepath.Join(vault, "Source.md")); err != nil || string(got) != string(beforeSource) {
		t.Errorf("source changed after meta-check: err=%v", err)
	}
	if got, err := os.ReadFile(filepath.Join(vault, "excluded", "Note.md")); err != nil || string(got) != string(beforeExcluded) {
		t.Errorf("excluded note changed after meta-check: err=%v", err)
	}
}

func TestMetaCheckExcludedCandidatesAffectBasenameResolution(t *testing.T) {
	vault := t.TempDir()
	for _, dir := range []string{"excluded/a", "excluded/b"} {
		if err := os.MkdirAll(filepath.Join(vault, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "mdhop.toml"), []byte("[build]\nexclude_paths = ['excluded/**']\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Source.md"), []byte("---\nsources:\n  - Duplicate\n  - RootChoice\n---\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	for _, path := range []string{"excluded/a/Duplicate.md", "excluded/b/Duplicate.md", "RootChoice.md", "excluded/a/RootChoice.md", "excluded/b/RootChoice.md"} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte("# note\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"sources"}, Kind: MetaKindPath})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Value != "Duplicate" || result.Issues[0].Reason != ReasonAmbiguous {
		t.Fatalf("issues = %+v, want Duplicate ambiguous only", result.Issues)
	}
}
