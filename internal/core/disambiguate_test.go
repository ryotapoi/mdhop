package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDisambiguateBasic(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	// B.md has 5 basename links to A: [[A]], [[A|alias]], [[A#Heading]], [link](A.md), [link2](A.md#frag)
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
		for _, r := range result.Rewritten {
			t.Logf("  %s: %s → %s", r.File, r.OldLink, r.NewLink)
		}
	}

	// Verify disk content of B.md.
	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	got := string(content)

	// B.md is in root, sub/A.md has subdirectory → vault-relative path.
	wantLines := []string{
		"[[sub/A]]",
		"[[sub/A|alias]]",
		"[[sub/A#Heading]]",
		"[link](sub/A.md)",
		"[link2](sub/A.md#frag)",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("B.md should contain %q, got:\n%s", want, got)
		}
	}
}

func TestDisambiguatePathLinkNotRewritten(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Read C.md original content.
	origContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}

	result, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	// C.md has [[sub/A]] which is a path link, not a basename link → not rewritten.
	for _, r := range result.Rewritten {
		if r.File == "C.md" {
			t.Errorf("C.md should not be rewritten, got: %s → %s", r.OldLink, r.NewLink)
		}
	}

	// Verify C.md content unchanged.
	newContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}
	if string(newContent) != string(origContent) {
		t.Errorf("C.md content changed:\n  before: %q\n  after:  %q", origContent, newContent)
	}
}

func TestDisambiguateMultipleCandidatesNoTarget(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	// Overwrite B.md to use path links only (avoid ambiguous link error on build).
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[sub/A]]\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}
	// Create another A.md in root (path link only so build doesn't fail with ambiguity).
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[sub/A]]\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	// Build with path links only → succeeds despite basename collision.
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err == nil {
		t.Fatal("expected error for multiple candidates without --target")
	}
	if !strings.Contains(err.Error(), "--target is required") {
		t.Errorf("error = %q, want containing '--target is required'", err.Error())
	}
}

func TestDisambiguateWithTarget(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Single candidate + explicit --target → success.
	result, err := Disambiguate(vault, DisambiguateOptions{
		Name:   "A",
		Target: "sub/A.md",
	})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
	}
}

func TestDisambiguateTargetNotFound(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{
		Name:   "A",
		Target: "nonexistent/A.md",
	})
	if err == nil {
		t.Fatal("expected error for target not found")
	}
	if !strings.Contains(err.Error(), "target not found") {
		t.Errorf("error = %q, want containing 'target not found'", err.Error())
	}
}

func TestDisambiguateNameNotFound(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for name not found")
	}
	if !strings.Contains(err.Error(), "no note found with basename") {
		t.Errorf("error = %q, want containing 'no note found with basename'", err.Error())
	}
}

func TestDisambiguateFileScope(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate_file_scope")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Disambiguate(vault, DisambiguateOptions{
		Name:  "A",
		Files: []string{"B.md"},
	})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	// Only B.md should be rewritten.
	for _, r := range result.Rewritten {
		if r.File != "B.md" {
			t.Errorf("unexpected rewrite in %s", r.File)
		}
	}
	if len(result.Rewritten) != 1 {
		t.Errorf("Rewritten count = %d, want 1", len(result.Rewritten))
	}

	// D.md should remain unchanged.
	content, err := os.ReadFile(filepath.Join(vault, "D.md"))
	if err != nil {
		t.Fatalf("read D.md: %v", err)
	}
	if !strings.Contains(string(content), "[[A]]") {
		t.Errorf("D.md should still contain [[A]], got: %s", content)
	}
}

func TestDisambiguateFileNotRegistered(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{
		Name:  "A",
		Files: []string{"nonexistent.md"},
	})
	if err == nil {
		t.Fatal("expected error for unregistered file")
	}
	if !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("error = %q, want containing 'file not registered'", err.Error())
	}
}

func TestDisambiguateNoDB(t *testing.T) {
	vault := t.TempDir()

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err == nil {
		t.Fatal("expected error for no DB")
	}
	if !strings.Contains(err.Error(), "index not found") {
		t.Errorf("error = %q, want containing 'index not found'", err.Error())
	}
}

func TestDisambiguateStaleSource(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Modify B.md after build to make it stale.
	bPath := filepath.Join(vault, "B.md")
	time.Sleep(1100 * time.Millisecond) // ensure mtime changes
	if err := os.WriteFile(bPath, []byte("[[A]]\nmodified\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err == nil {
		t.Fatal("expected error for stale source")
	}
	if !strings.Contains(err.Error(), "source file is stale") {
		t.Errorf("error = %q, want containing 'source file is stale'", err.Error())
	}

	// Verify B.md was not changed by disambiguate (still has the modified content).
	content, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "modified") {
		t.Error("B.md should not have been rewritten after stale error")
	}
}

func TestDisambiguateNoRewritesNeeded(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// C is not a basename with links, try disambiguating it.
	// C.md only has path links to sub/A → no basename links for "C" either.
	result, err := Disambiguate(vault, DisambiguateOptions{Name: "C"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}
	if len(result.Rewritten) != 0 {
		t.Errorf("Rewritten count = %d, want 0", len(result.Rewritten))
	}
}

func TestDisambiguateDBUpdated(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	// Verify edge raw_link values are updated in DB.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()

	// Get target node ID for sub/A.md.
	var targetID int64
	err = db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", noteKey("sub/A.md")).Scan(&targetID)
	if err != nil {
		t.Fatalf("query target: %v", err)
	}

	// Get all edges from B.md → sub/A.md.
	var sourceID int64
	err = db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", noteKey("B.md")).Scan(&sourceID)
	if err != nil {
		t.Fatalf("query source: %v", err)
	}

	rows, err := db.Query(
		"SELECT raw_link FROM edges WHERE source_id = ? AND target_id = ? ORDER BY line_start",
		sourceID, targetID)
	if err != nil {
		t.Fatalf("query edges: %v", err)
	}
	defer rows.Close()

	var rawLinks []string
	for rows.Next() {
		var rawLink string
		if err := rows.Scan(&rawLink); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rawLinks = append(rawLinks, rawLink)
	}

	// All should now be path-based links.
	for _, rl := range rawLinks {
		if rl == "[[A]]" || rl == "[[A|alias]]" || rl == "[[A#Heading]]" ||
			rl == "[link](A.md)" || rl == "[link2](A.md#frag)" {
			t.Errorf("edge raw_link still basename: %s", rl)
		}
	}

	// Check B.md's mtime was updated.
	var dbMtime int64
	err = db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", sourceID).Scan(&dbMtime)
	if err != nil {
		t.Fatalf("query mtime: %v", err)
	}
	info, err := os.Stat(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("stat B.md: %v", err)
	}
	if info.ModTime().Unix() != dbMtime {
		t.Errorf("B.md mtime mismatch: disk=%d, db=%d", info.ModTime().Unix(), dbMtime)
	}
}

func TestDisambiguateInlineCodeIgnored(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	// Overwrite B.md to include inline code.
	bPath := filepath.Join(vault, "B.md")
	if err := os.WriteFile(bPath, []byte("[[A]]\n`[[A]]` should not change\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Disambiguate(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	content, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	got := string(content)

	// The inline code [[A]] should remain.
	if !strings.Contains(got, "`[[A]]`") {
		t.Errorf("inline code [[A]] was incorrectly rewritten, got:\n%s", got)
	}
	// The regular link should be rewritten.
	if !strings.Contains(got, "[[sub/A]]") {
		t.Errorf("regular [[A]] was not rewritten, got:\n%s", got)
	}
}

func TestDisambiguateCaseInsensitive(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Use lowercase "a" for --name (should match "A.md").
	result, err := Disambiguate(vault, DisambiguateOptions{Name: "a"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
	}
}

// --- DisambiguateScan tests ---

func TestDisambiguateScanBasic(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	// No build — scan works without DB.
	result, err := DisambiguateScan(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
		for _, r := range result.Rewritten {
			t.Logf("  %s: %s → %s", r.File, r.OldLink, r.NewLink)
		}
	}

	// Verify disk content of B.md.
	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	got := string(content)

	wantLines := []string{
		"[[sub/A]]",
		"[[sub/A|alias]]",
		"[[sub/A#Heading]]",
		"[link](sub/A.md)",
		"[link2](sub/A.md#frag)",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("B.md should contain %q, got:\n%s", want, got)
		}
	}
}

func TestDisambiguateScanMultipleCandidatesNoTarget(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	// Create another A.md in root.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# root A\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}

	_, err := DisambiguateScan(vault, DisambiguateOptions{Name: "A"})
	if err == nil {
		t.Fatal("expected error for multiple candidates without --target")
	}
	if !strings.Contains(err.Error(), "--target is required") {
		t.Errorf("error = %q, want containing '--target is required'", err.Error())
	}
}

func TestDisambiguateScanWithTarget(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	result, err := DisambiguateScan(vault, DisambiguateOptions{
		Name:   "A",
		Target: "sub/A.md",
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
	}
}

func TestDisambiguateScanFileScope(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate_file_scope")

	result, err := DisambiguateScan(vault, DisambiguateOptions{
		Name:  "A",
		Files: []string{"B.md"},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	for _, r := range result.Rewritten {
		if r.File != "B.md" {
			t.Errorf("unexpected rewrite in %s", r.File)
		}
	}
	if len(result.Rewritten) != 1 {
		t.Errorf("Rewritten count = %d, want 1", len(result.Rewritten))
	}

	// D.md should remain unchanged.
	content, err := os.ReadFile(filepath.Join(vault, "D.md"))
	if err != nil {
		t.Fatalf("read D.md: %v", err)
	}
	if !strings.Contains(string(content), "[[A]]") {
		t.Errorf("D.md should still contain [[A]], got: %s", content)
	}
}

func TestDisambiguateScanFileNotFound(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	_, err := DisambiguateScan(vault, DisambiguateOptions{
		Name:  "A",
		Files: []string{"nonexistent.md"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("error = %q, want containing 'file not found'", err.Error())
	}
}

func TestDisambiguateScanInlineCodeIgnored(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	bPath := filepath.Join(vault, "B.md")
	if err := os.WriteFile(bPath, []byte("[[A]]\n`[[A]]` should not change\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := DisambiguateScan(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	content, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	got := string(content)

	if !strings.Contains(got, "`[[A]]`") {
		t.Errorf("inline code [[A]] was incorrectly rewritten, got:\n%s", got)
	}
	if !strings.Contains(got, "[[sub/A]]") {
		t.Errorf("regular [[A]] was not rewritten, got:\n%s", got)
	}
}

func TestDisambiguateScanNoDBRequired(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	// Ensure no .mdhop directory exists.
	mdhopDir := filepath.Join(vault, ".mdhop")
	if _, err := os.Stat(mdhopDir); err == nil {
		t.Fatalf(".mdhop should not exist in fixture, but it does")
	}

	// Should work without DB.
	result, err := DisambiguateScan(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
	}
}

func TestDisambiguateScanCaseInsensitive(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	result, err := DisambiguateScan(vault, DisambiguateOptions{Name: "a"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(result.Rewritten) != 5 {
		t.Errorf("Rewritten count = %d, want 5", len(result.Rewritten))
	}
}

func TestDisambiguateScanPathLinkNotRewritten(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	origContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}

	result, err := DisambiguateScan(vault, DisambiguateOptions{Name: "A"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	for _, r := range result.Rewritten {
		if r.File == "C.md" {
			t.Errorf("C.md should not be rewritten, got: %s → %s", r.OldLink, r.NewLink)
		}
	}

	newContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}
	if string(newContent) != string(origContent) {
		t.Errorf("C.md content changed:\n  before: %q\n  after:  %q", origContent, newContent)
	}
}

func TestDisambiguateScanTargetNotFound(t *testing.T) {
	vault := copyVault(t, "vault_disambiguate")

	_, err := DisambiguateScan(vault, DisambiguateOptions{
		Name:   "A",
		Target: "nonexistent/A.md",
	})
	if err == nil {
		t.Fatal("expected error for target not found")
	}
	if !strings.Contains(err.Error(), "target not found") {
		t.Errorf("error = %q, want containing 'target not found'", err.Error())
	}
}

// openTestDBForDisambiguate is not needed — we reuse openTestDB from build_test.go.

// queryEdgeRawLinks is a helper specific to disambiguate tests.
func queryEdgeRawLinks(t *testing.T, dbp string, sourceFile, targetFile string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+dbp)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT e.raw_link FROM edges e
		 JOIN nodes sn ON sn.id = e.source_id
		 JOIN nodes tn ON tn.id = e.target_id
		 WHERE sn.node_key = ? AND tn.node_key = ?
		 ORDER BY e.line_start`,
		noteKey(sourceFile), noteKey(targetFile))
	if err != nil {
		t.Fatalf("query edges: %v", err)
	}
	defer rows.Close()

	var links []string
	for rows.Next() {
		var rl string
		if err := rows.Scan(&rl); err != nil {
			t.Fatalf("scan: %v", err)
		}
		links = append(links, rl)
	}
	return links
}
