package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLinkKeysCreatesFrontmatterPathEdges(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs/index.md")

	type want struct {
		targetName string
		targetType string
		rawLink    string
	}
	wants := []want{
		{"a", "note", "topics/a.md"},         // vault-relative path
		{"b", "note", "./b.md"},              // note-relative path
		{"missing", "phantom", "missing.md"}, // unresolved → phantom
	}

	var pathEdges []edgeRow
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath {
			pathEdges = append(pathEdges, e)
		}
	}
	if len(pathEdges) != len(wants) {
		t.Fatalf("frontmatter_path edges = %d, want %d: %+v", len(pathEdges), len(wants), pathEdges)
	}
	gotKeys := make(map[string]bool)
	for _, e := range pathEdges {
		gotKeys[e.targetName+"|"+string(e.targetType)+"|"+e.rawLink] = true
	}
	for _, w := range wants {
		if !gotKeys[w.targetName+"|"+w.targetType+"|"+w.rawLink] {
			t.Errorf("expected frontmatter_path edge %+v not found in %+v", w, pathEdges)
		}
	}

	// URL value must not produce any edge.
	for _, e := range edges {
		if strings.Contains(e.rawLink, "example.com") {
			t.Errorf("URL value should not become an edge: %+v", e)
		}
	}
	// Wikilink value stays a frontmatter_wikilink edge (no duplicate path edge).
	var wikiCount int
	for _, e := range edges {
		if e.targetName == "Wiki Target" {
			wikiCount++
			if e.linkType != LinkTypeFrontmatterWikilink {
				t.Errorf("Wiki Target edge linkType = %q, want frontmatter_wikilink", e.linkType)
			}
		}
	}
	if wikiCount != 1 {
		t.Errorf("Wiki Target edge count = %d, want 1", wikiCount)
	}
	// Key not listed in link_keys must not produce an edge (only the
	// "related" occurrence links to topics/a.md).
	var aCount int
	for _, e := range pathEdges {
		if e.targetName == "a" {
			aCount++
		}
	}
	if aCount != 1 {
		t.Errorf("edges to topics/a.md = %d, want 1 (key 'other' must be ignored)", aCount)
	}
}

func TestBuildLinkKeysBacklinksReflected(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "topics/a.md"}, QueryOptions{Fields: []string{"backlinks"}})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	names := nodeNames(res.Backlinks)
	expectContains(t, names, "index")
}

func TestBuildLinkKeysUnsetKeepsBehavior(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Remove the config: raw path values must not become edges.
	if err := os.Remove(filepath.Join(vault, "mdhop.yaml")); err != nil {
		t.Fatalf("remove config: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs/index.md")
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath {
			t.Errorf("unexpected frontmatter_path edge without link_keys: %+v", e)
		}
	}
}

func TestBuildLinkKeysAmbiguousBasenameFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Two non-root notes with the same basename make the raw value "dup"
	// ambiguous.
	for _, p := range []string{"x/dup.md", "y/dup.md"} {
		full := filepath.Join(vault, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# dup\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "amb.md"), []byte("---\nrelated: dup.md\n---\n\n# Amb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(vault)
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("error = %v, want ambiguous link", err)
	}
}

func TestBuildLinkKeysVaultEscapeFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if err := os.WriteFile(filepath.Join(vault, "docs", "esc.md"), []byte("---\nrelated: ../../outside.md\n---\n\n# Esc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(vault)
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("error = %v, want escapes vault", err)
	}
}

func TestBuildLinkKeysUpdatePreservesEdges(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Touch the file so update re-parses it.
	full := filepath.Join(vault, "docs", "index.md")
	content, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, append(content, []byte("\nMore text.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"docs/index.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs/index.md")
	var pathEdges int
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath {
			pathEdges++
		}
	}
	if pathEdges != 3 {
		t.Errorf("frontmatter_path edges after update = %d, want 3", pathEdges)
	}
}

func TestBuildLinkKeysMovePreservesEdges(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := Move(vault, MoveOptions{From: "docs/index.md", To: "docs/index2.md"}); err != nil {
		t.Fatalf("move: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs/index2.md")
	var pathEdges int
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath {
			pathEdges++
		}
	}
	if pathEdges != 3 {
		t.Errorf("frontmatter_path edges after move = %d, want 3", pathEdges)
	}
}

func TestAddLinkKeysBasenameConflictFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Raw basename value "a.md" resolves uniquely to topics/a.md.
	if err := os.WriteFile(filepath.Join(vault, "docs", "c.md"), []byte("---\nrelated: a.md\n---\n\n# C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Adding another non-root a.md makes the raw value ambiguous; it cannot
	// be rewritten, so Add must fail.
	full := filepath.Join(vault, "x", "a.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# another a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Add(vault, AddOptions{Files: []string{"x/a.md"}})
	if err == nil || !strings.Contains(err.Error(), "cannot be rewritten") {
		t.Errorf("error = %v, want frontmatter_path guard error", err)
	}
}

func TestAddLinkKeysPhantomBecomesAmbiguousFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// docs/index.md carries `related: missing.md` → phantom "missing".
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Adding two non-root files with that basename makes the phantom raw
	// value ambiguous (Pattern B); it cannot be rewritten, so Add must fail.
	for _, p := range []string{"x/missing.md", "y/missing.md"} {
		full := filepath.Join(vault, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# missing\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := Add(vault, AddOptions{Files: []string{"x/missing.md", "y/missing.md"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("error = %v, want ambiguous link", err)
	}
}

func TestAddLinkKeysNonBasenamePhantomNotPromoted(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// docs/np.md carries a path raw that stays unresolved; it shares phantom
	// "missing" with the basename raw `missing.md` in docs/index.md.
	if err := os.WriteFile(filepath.Join(vault, "docs", "np.md"), []byte("---\nrelated: topics/missing.md\n---\n\n# NP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Adding x/missing.md promotes the basename raw, but the path raw would
	// not resolve there on a full rebuild and must stay a phantom edge.
	full := filepath.Join(vault, "x", "missing.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# missing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Add(vault, AddOptions{Files: []string{"x/missing.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "x/missing.md" {
		t.Errorf("Promoted = %v, want [x/missing.md]", res.Promoted)
	}
	for _, e := range queryEdges(t, dbPath(vault), "docs/np.md") {
		if e.rawLink == "topics/missing.md" && string(e.targetType) != "phantom" {
			t.Errorf("path raw edge promoted to %s, want phantom: %+v", e.targetType, e)
		}
	}
	var basenamePromoted bool
	for _, e := range queryEdges(t, dbPath(vault), "docs/index.md") {
		if e.rawLink == "missing.md" && string(e.targetType) == "note" {
			basenamePromoted = true
		}
	}
	if !basenamePromoted {
		t.Errorf("basename raw edge not promoted to note")
	}
}

func TestAddLinkKeysPartialPromoteKeepsPhantom(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Body wikilink and a path raw share phantom "missing2"; promotion must
	// re-point the wikilink edge while the raw edge keeps the phantom alive.
	if err := os.WriteFile(filepath.Join(vault, "docs", "w.md"), []byte("# W\n\n[[missing2]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "np.md"), []byte("---\nrelated: topics/missing2.md\n---\n\n# NP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	full := filepath.Join(vault, "x", "missing2.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# missing2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"x/missing2.md"}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	var wikiPromoted bool
	for _, e := range queryEdges(t, dbPath(vault), "docs/w.md") {
		if e.targetName == "missing2" && string(e.targetType) == "note" && e.linkType == LinkTypeWikilink {
			wikiPromoted = true
		}
	}
	if !wikiPromoted {
		t.Errorf("wikilink edge not promoted to note")
	}
	var rawOnPhantom bool
	for _, e := range queryEdges(t, dbPath(vault), "docs/np.md") {
		if e.rawLink == "topics/missing2.md" && string(e.targetType) == "phantom" {
			rawOnPhantom = true
		}
	}
	if !rawOnPhantom {
		t.Errorf("path raw edge must stay on the phantom")
	}
}

func TestMoveLinkKeysNonBasenamePhantomNotPromoted(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if err := os.WriteFile(filepath.Join(vault, "docs", "np.md"), []byte("---\nrelated: topics/missing.md\n---\n\n# NP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "free.md"), []byte("# Free\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Moving an unrelated note onto the phantom's basename promotes the
	// basename raw but must leave the path raw on the phantom.
	if _, err := Move(vault, MoveOptions{From: "free.md", To: "x/missing.md"}); err != nil {
		t.Fatalf("move: %v", err)
	}
	for _, e := range queryEdges(t, dbPath(vault), "docs/np.md") {
		if e.rawLink == "topics/missing.md" && string(e.targetType) != "phantom" {
			t.Errorf("path raw edge promoted to %s, want phantom: %+v", e.targetType, e)
		}
	}
	var basenamePromoted bool
	for _, e := range queryEdges(t, dbPath(vault), "docs/index.md") {
		if e.rawLink == "missing.md" && string(e.targetType) == "note" {
			basenamePromoted = true
		}
	}
	if !basenamePromoted {
		t.Errorf("basename raw edge not promoted to note")
	}
}

func TestMoveDirLinkKeysNonBasenamePhantomNotPromoted(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// mv/missing.md resolves the basename raw in docs/index.md at build;
	// only the path raw in docs/np.md creates phantom "missing".
	full := filepath.Join(vault, "mv", "missing.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# missing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "np.md"), []byte("---\nrelated: topics/missing.md\n---\n\n# NP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "mv", ToDir: "x"}); err != nil {
		t.Fatalf("move dir: %v", err)
	}
	for _, e := range queryEdges(t, dbPath(vault), "docs/np.md") {
		if e.rawLink == "topics/missing.md" && string(e.targetType) != "phantom" {
			t.Errorf("path raw edge promoted to %s, want phantom: %+v", e.targetType, e)
		}
	}
}

func TestMoveLinkKeysConfigAddedAfterBuildAmbiguousFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Build without link_keys: raw values produce no frontmatter_path edges.
	if err := os.Remove(filepath.Join(vault, "mdhop.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"x/dup.md", "y/dup.md"} {
		full := filepath.Join(vault, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# dup\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "amb.md"), []byte("---\nrelated: dup.md\n---\n\n# Amb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Configure link_keys after build: the move re-parse now yields an
	// ambiguous frontmatter_path basename with no DB edge to guard, so the
	// re-parse validation must catch it.
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("meta:\n  link_keys:\n    - related\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Move(vault, MoveOptions{From: "docs/amb.md", To: "docs/amb2.md"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("error = %v, want ambiguous link", err)
	}
}

func TestMoveDirLinkKeysConfigAddedAfterBuildAmbiguousFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if err := os.Remove(filepath.Join(vault, "mdhop.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"x/dup.md", "y/dup.md"} {
		full := filepath.Join(vault, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# dup\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "amb.md"), []byte("---\nrelated: dup.md\n---\n\n# Amb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("meta:\n  link_keys:\n    - related\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := MoveDir(vault, MoveDirOptions{FromDir: "docs", ToDir: "docs2"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("error = %v, want ambiguous link", err)
	}
}

func TestMoveLinkKeysIncomingPathRawFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if err := os.WriteFile(filepath.Join(vault, "docs", "c.md"), []byte("---\nrelated: topics/a.md\n---\n\n# C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// The vault-relative raw value in docs/c.md would break; it cannot be
	// rewritten, so Move must fail.
	_, err := Move(vault, MoveOptions{From: "topics/a.md", To: "topics/a2.md"})
	if err == nil || !strings.Contains(err.Error(), "cannot be rewritten") {
		t.Errorf("error = %v, want frontmatter_path guard error", err)
	}
}

func TestMoveLinkKeysBasenameRawSameBasenameOK(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	// Use a note nobody references by raw path, so only the basename raw
	// value in docs/c.md is affected by the move.
	if err := os.WriteFile(filepath.Join(vault, "topics", "solo.md"), []byte("# solo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "c.md"), []byte("---\nrelated: solo.md\n---\n\n# C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Basename raw value keeps resolving to the moved note (same basename,
	// still unique), so the move succeeds without rewriting it.
	if _, err := Move(vault, MoveOptions{From: "topics/solo.md", To: "moved/solo.md"}); err != nil {
		t.Fatalf("move: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs/c.md")
	var found bool
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath && e.targetName == "solo" && e.rawLink == "solo.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("frontmatter_path edge to moved note not preserved: %+v", edges)
	}
}

func TestMoveLinkKeysRelativeRawInMovedFileFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// docs/index.md carries `related: ./b.md`; from other/ it would resolve
	// elsewhere and cannot be rewritten, so Move must fail.
	_, err := Move(vault, MoveOptions{From: "docs/index.md", To: "other/index.md"})
	if err == nil || !strings.Contains(err.Error(), "cannot be rewritten") {
		t.Errorf("error = %v, want frontmatter_path guard error", err)
	}
}

func TestMoveDirLinkKeysPathRawFails(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if err := os.WriteFile(filepath.Join(vault, "c.md"), []byte("---\nrelated: docs/b.md\n---\n\n# C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// The vault-relative raw value in root c.md points into the moved dir;
	// it cannot be rewritten, so MoveDir must fail.
	_, err := MoveDir(vault, MoveDirOptions{FromDir: "docs", ToDir: "docs2"})
	if err == nil || !strings.Contains(err.Error(), "cannot be rewritten") {
		t.Errorf("error = %v, want frontmatter_path guard error", err)
	}
}

func TestMoveDirLinkKeysRelativeRawSurvives(t *testing.T) {
	vault := copyVault(t, "vault_build_link_keys")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// `./b.md` in docs/index.md moves together with its target, and
	// `topics/a.md` points outside the moved dir; both keep resolving.
	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "docs", ToDir: "docs2"}); err != nil {
		t.Fatalf("move dir: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "docs2/index.md")
	var pathEdges int
	for _, e := range edges {
		if e.linkType == LinkTypeFrontmatterPath {
			pathEdges++
		}
	}
	if pathEdges != 3 {
		t.Errorf("frontmatter_path edges after dir move = %d, want 3", pathEdges)
	}
}

func TestLoadConfigLinkKeysTagsRejected(t *testing.T) {
	vault := t.TempDir()
	cfgContent := "meta:\n  link_keys:\n    - tags\n"
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(vault)
	if err == nil || !strings.Contains(err.Error(), "link_keys") {
		t.Errorf("error = %v, want link_keys validation error", err)
	}
}
