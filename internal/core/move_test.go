package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test 1: from not registered in DB → error ---
func TestMove_NotRegistered(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "Z.md", To: "W.md"})
	if err == nil {
		t.Fatal("expected error for unregistered file")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 2: to already registered in DB → error ---
func TestMove_TargetExists(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "B.md"})
	if err == nil {
		t.Fatal("expected error for already registered destination")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 3: to exists on disk (from also on disk) → error ---
func TestMove_TargetExistsOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Create an unregistered file at the destination.
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "C.md"})
	if err == nil {
		t.Fatal("expected error for destination existing on disk")
	}
	if !strings.Contains(err.Error(), "already exists on disk") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 4: move causes ambiguous links → error ---
func TestMove_AmbiguousAfterMove(t *testing.T) {
	// Create a vault where moving a file creates basename ambiguity.
	// A.md (content only), B.md has [[A]], C.md has [[A]].
	// Move A.md to sub/C.md → basename "C" collides with C.md.
	// The outgoing links from moved file may have ambiguous targets.
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Move A.md to sub/C.md → basename "C" now has C.md + sub/C.md.
	// A.md's outgoing [[C]] becomes ambiguous (two files with basename C).
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/C.md"})
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguity error, got: %v", err)
	}
}

// --- Test 5: basename unchanged + unique → links preserved (no rewrite) ---
func TestMove_BasenameUnchanged(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to sub/A.md. Basename stays "A".
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md has [[A]] — basename link, unchanged and unique → no rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("[[A]] in B.md should NOT be rewritten, but was rewritten to %s", rw.NewLink)
		}
	}

	// Verify file moved on disk.
	if fileExists(filepath.Join(vault, "A.md")) {
		t.Error("A.md should not exist on disk after move")
	}
	if !fileExists(filepath.Join(vault, "sub", "A.md")) {
		t.Error("sub/A.md should exist on disk after move")
	}

	// Verify DB updated.
	dbp := dbPath(vault)
	notes := queryNodes(t, dbp, "note")
	var foundNew bool
	for _, n := range notes {
		if n.path == "sub/A.md" {
			foundNew = true
		}
		if n.path == "A.md" {
			t.Error("DB still contains old path A.md")
		}
	}
	if !foundNew {
		t.Error("DB does not contain new path sub/A.md")
	}

	// Verify B.md's edge still targets the moved note via basename.
	edges := queryEdges(t, dbp, "B.md")
	var hasLinkToA bool
	for _, e := range edges {
		if e.targetName == "A" && e.linkType == "wikilink" && e.rawLink == "[[A]]" {
			hasLinkToA = true
		}
	}
	if !hasLinkToA {
		t.Error("B.md should still have [[A]] link")
	}
}

// --- Test 6: path links are always rewritten ---
func TestMove_PathLinkAlwaysRewritten(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// C.md has [link to A](./A.md) — path link.
	// sub/D.md has [path link](../A.md) — path link.
	// Move A.md to sub/A.md.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// C.md's path link should be rewritten.
	var cRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "C.md" && rw.OldLink == "[link to A](./A.md)" {
			cRewritten = true
			// Target is sub/A.md, source is C.md (root). So new link should be sub/A.md.
			if !strings.Contains(rw.NewLink, "sub/A") {
				t.Errorf("C.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !cRewritten {
		t.Error("C.md path link should be rewritten")
	}

	// sub/D.md's path link [path link](../A.md) should be rewritten.
	// Target is sub/A.md, source is sub/D.md. buildRewritePath gives vault-relative
	// for subdirectory targets: "sub/A.md".
	var dRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/D.md" && rw.OldLink == "[path link](../A.md)" {
			dRewritten = true
			if rw.NewLink != "[path link](sub/A.md)" {
				t.Errorf("sub/D.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !dRewritten {
		t.Error("sub/D.md path link should be rewritten")
	}

	// Verify file content was actually rewritten on disk.
	cContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}
	if strings.Contains(string(cContent), "./A.md)") {
		t.Error("C.md should not contain ./A.md anymore")
	}
}

// --- Test 7: basename changes → basename links rewritten ---
func TestMove_BasenameChanged(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// B.md has [[A]], sub/D.md has [[A]].
	// Rename A.md to X.md — basename changes from A to X.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// [[A]] in B.md should be rewritten because basename changed.
	var bRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			bRewritten = true
			if rw.NewLink != "[[./X]]" {
				t.Errorf("B.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !bRewritten {
		t.Error("B.md [[A]] should be rewritten when basename changes")
	}

	// sub/D.md [[A]] should be rewritten too.
	var dWikiRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/D.md" && rw.OldLink == "[[A]]" {
			dWikiRewritten = true
			if rw.NewLink != "[[../X]]" {
				t.Errorf("sub/D.md wikilink rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !dWikiRewritten {
		t.Error("sub/D.md [[A]] should be rewritten when basename changes")
	}

	// Verify DB has new path.
	notes := queryNodes(t, dbPath(vault), "note")
	var foundX bool
	for _, n := range notes {
		if n.path == "X.md" {
			foundX = true
		}
	}
	if !foundX {
		t.Error("DB should contain X.md after move")
	}
}

// --- Test 8: outgoing relative links rewritten ---
func TestMove_OutgoingRelativeRewritten(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// A.md has:
	//   [link to B](./B.md)   — relative link to B.md
	//   [link to C](./C.md)   — relative link to C.md
	// Move A.md to sub/A.md.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Outgoing relative links should be rewritten.
	var bOutRewrite, cOutRewrite bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/A.md" && rw.OldLink == "[link to B](./B.md)" {
			bOutRewrite = true
			if rw.NewLink != "[link to B](../B.md)" {
				t.Errorf("outgoing B link rewrite unexpected: %s", rw.NewLink)
			}
		}
		if rw.File == "sub/A.md" && rw.OldLink == "[link to C](./C.md)" {
			cOutRewrite = true
			if rw.NewLink != "[link to C](../C.md)" {
				t.Errorf("outgoing C link rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !bOutRewrite {
		t.Error("outgoing link to B should be rewritten")
	}
	if !cOutRewrite {
		t.Error("outgoing link to C should be rewritten")
	}

	// Verify the file content on disk.
	content, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md: %v", err)
	}
	if !strings.Contains(string(content), "../B.md") {
		t.Error("sub/A.md should contain ../B.md")
	}
	if !strings.Contains(string(content), "../C.md") {
		t.Error("sub/A.md should contain ../C.md")
	}
}

// --- Test 9: phantom promotion ---
func TestMove_PhantomPromotion(t *testing.T) {
	vault := copyVault(t, "vault_move_phantom")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	dbp := dbPath(vault)

	// Before move: A.md and B.md link to [[X]] which is a phantom.
	phantoms := queryNodes(t, dbp, "phantom")
	var phantomFound bool
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "x" {
			phantomFound = true
		}
	}
	if !phantomFound {
		t.Fatal("expected phantom X before move")
	}

	// Rename A.md to X.md → basename matches phantom "X" → promote.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// After move: phantom X should be gone, replaced by note X.md.
	phantoms = queryNodes(t, dbp, "phantom")
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "x" {
			t.Error("phantom X should be promoted after move")
		}
	}

	// Note X.md should exist.
	notes := queryNodes(t, dbp, "note")
	var noteXFound bool
	for _, n := range notes {
		if n.path == "X.md" {
			noteXFound = true
		}
	}
	if !noteXFound {
		t.Error("note X.md should exist after move")
	}

	// B.md's edge should now point to note X.md.
	edges := queryEdges(t, dbp, "B.md")
	var bToX bool
	for _, e := range edges {
		if e.targetName == "X" && e.targetType == "note" {
			bToX = true
		}
	}
	if !bToX {
		t.Error("B.md should link to note X after phantom promotion")
	}
}

// --- Test 10: phantom promotion with orphan cleanup verification ---
func TestMove_PhantomPromotionAndOrphanCleanup(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[Phantom1]]\n[[Phantom2]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	dbp := dbPath(vault)

	// Both Phantom1 and Phantom2 should exist.
	phantoms := queryNodes(t, dbp, "phantom")
	if len(phantoms) != 2 {
		t.Fatalf("expected 2 phantoms, got %d", len(phantoms))
	}

	// Move A.md to Phantom1.md → phantom "phantom1" is promoted.
	// Phantom2 is still referenced by moved file's outgoing [[Phantom2]].
	_, err := Move(vault, MoveOptions{From: "A.md", To: "Phantom1.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Phantom1 should be promoted (no longer phantom).
	phantoms = queryNodes(t, dbp, "phantom")
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "phantom1" {
			t.Error("Phantom1 should be promoted, not remain as phantom")
		}
	}
	// Phantom2 should still exist (still referenced).
	var p2Exists bool
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "phantom2" {
			p2Exists = true
		}
	}
	if !p2Exists {
		t.Error("Phantom2 should still exist")
	}
}

// --- Test 11: mkdir auto-creation ---
func TestMove_MkdirAuto(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to deep/nested/A.md — directories should be auto-created.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "deep/nested/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	if !fileExists(filepath.Join(vault, "deep", "nested", "A.md")) {
		t.Error("deep/nested/A.md should exist after move")
	}
}

// --- Test 12: stale from file → error ---
func TestMove_StaleFromError(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Modify A.md after build to make it stale.
	time.Sleep(1100 * time.Millisecond) // ensure mtime changes
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected stale error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 13: stale source file for incoming rewrite → error ---
func TestMove_StaleSourceError(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Modify C.md (which has a path link to A.md) after build.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[link to A](./A.md)\n[[B]]\nmodified\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	// Rename A.md to X.md — C.md needs rewriting but is stale.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected stale error for C.md")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 14: self-reference in moved file ---
func TestMove_SelfReference(t *testing.T) {
	vault := t.TempDir()
	// A.md references itself via [[#Heading]].
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[#Heading]]\n## Heading\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move with self-reference: %v", err)
	}

	// Self-reference should be preserved.
	content, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md: %v", err)
	}
	if !strings.Contains(string(content), "[[#Heading]]") {
		t.Error("self-reference [[#Heading]] should be preserved")
	}
}

// --- Test 15: Phase 2.5 — third-party basename links become ambiguous ---
func TestMove_AmbiguousThirdParty(t *testing.T) {
	vault := t.TempDir()
	// A.md is unique with basename "A". B.md links to it via [[A]].
	// C.md exists with no links to A.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move C.md to sub/A.md → basename "A" count becomes 2.
	// B.md's [[A]] (basename link to A.md) is NOT an incoming link to C.md,
	// so it won't be in incomingRewrites. Phase 2.5 should detect it.
	_, err := Move(vault, MoveOptions{From: "C.md", To: "sub/A.md"})
	if err == nil {
		t.Fatal("expected ambiguity error from Phase 2.5")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguity error, got: %v", err)
	}
}

// --- Test 16: both from and to absent on disk ---
func TestMove_BothAbsentOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove A.md from disk after build.
	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatalf("remove A.md: %v", err)
	}

	// X.md doesn't exist on disk either → default case.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected error when both from and to are absent on disk")
	}
	if !strings.Contains(err.Error(), "source file not found on disk") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 17: same path → error ---
func TestMove_SamePath(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "A.md"})
	if err == nil {
		t.Fatal("expected error for same source and destination")
	}
	if !strings.Contains(err.Error(), "source and destination are the same") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 18: multiple incoming rewrites from different files ---
func TestMove_MultipleIncomingRewrites(t *testing.T) {
	vault := t.TempDir()
	// A.md exists. B.md, C.md, D.md all have path links to A.md.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[b link](./A.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[./A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("[d link](./A.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// All three files should have rewritten links with correct new values.
	rewrittenFiles := make(map[string]string)
	for _, rw := range result.Rewritten {
		if rw.File == "sub/A.md" {
			continue // outgoing rewrites
		}
		rewrittenFiles[rw.File] = rw.NewLink
	}

	// Target has subdirectory → vault-relative rewrite (no ./ prefix).
	// B.md: [b link](./A.md) → [b link](sub/A.md)
	if got, ok := rewrittenFiles["B.md"]; !ok {
		t.Error("B.md path link should be rewritten")
	} else if got != "[b link](sub/A.md)" {
		t.Errorf("B.md new link = %q, want %q", got, "[b link](sub/A.md)")
	}
	// C.md: [[./A]] → [[sub/A]]
	if got, ok := rewrittenFiles["C.md"]; !ok {
		t.Error("C.md wikilink path link should be rewritten")
	} else if got != "[[sub/A]]" {
		t.Errorf("C.md new link = %q, want %q", got, "[[sub/A]]")
	}
	// D.md: [d link](./A.md) → [d link](sub/A.md)
	if got, ok := rewrittenFiles["D.md"]; !ok {
		t.Error("D.md path link should be rewritten")
	} else if got != "[d link](sub/A.md)" {
		t.Errorf("D.md new link = %q, want %q", got, "[d link](sub/A.md)")
	}

	// Verify disk content was actually rewritten with correct new links.
	bContent, _ := os.ReadFile(filepath.Join(vault, "B.md"))
	if !strings.Contains(string(bContent), "sub/A.md") {
		t.Errorf("B.md should contain sub/A.md, got: %s", string(bContent))
	}
	cContent, _ := os.ReadFile(filepath.Join(vault, "C.md"))
	if !strings.Contains(string(cContent), "[[sub/A]]") {
		t.Errorf("C.md should contain [[sub/A]], got: %s", string(cContent))
	}
	dContent, _ := os.ReadFile(filepath.Join(vault, "D.md"))
	if !strings.Contains(string(dContent), "sub/A.md") {
		t.Errorf("D.md should contain sub/A.md, got: %s", string(dContent))
	}
}

// --- Test 19: already-moved stale file → error ---
func TestMove_AlreadyMovedStale(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already moved and then edited the file.
	if err := os.MkdirAll(filepath.Join(vault, "newsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(vault, "A.md"), filepath.Join(vault, "newsub", "A.md")); err != nil {
		t.Fatal(err)
	}

	// Modify the file content to change mtime.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "newsub", "A.md"), []byte("modified content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "newsub/A.md"})
	if err == nil {
		t.Fatal("expected stale error for already-moved file")
	}
	if !strings.Contains(err.Error(), "moved file is stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 20: already moved on disk (from absent, to present) ---
func TestMove_AlreadyMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already did: mv A.md sub/A.md
	if err := os.MkdirAll(filepath.Join(vault, "newsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(vault, "A.md"), filepath.Join(vault, "newsub", "A.md")); err != nil {
		t.Fatal(err)
	}

	// Now run move — should detect already-moved state and just update DB + rewrite links.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "newsub/A.md"})
	if err != nil {
		t.Fatalf("move (already moved): %v", err)
	}

	// File should remain at newsub/A.md.
	if !fileExists(filepath.Join(vault, "newsub", "A.md")) {
		t.Error("newsub/A.md should exist")
	}
	if fileExists(filepath.Join(vault, "A.md")) {
		t.Error("A.md should not exist")
	}

	// DB should be updated.
	notes := queryNodes(t, dbPath(vault), "note")
	var found bool
	for _, n := range notes {
		if n.path == "newsub/A.md" {
			found = true
		}
	}
	if !found {
		t.Error("DB should contain newsub/A.md")
	}

	// C.md had [link to A](./A.md) — path link should be rewritten.
	var cRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "C.md" && rw.OldLink == "[link to A](./A.md)" {
			cRewritten = true
		}
	}
	if !cRewritten {
		t.Error("C.md path link should be rewritten even for already-moved case")
	}
}
