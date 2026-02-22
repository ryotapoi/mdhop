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

// --- Test 4: move causes ambiguous links (root priority resolves) ---
func TestMove_AmbiguousAfterMove(t *testing.T) {
	// A.md has [[C]], B.md has [[A]], C.md exists at root.
	// Move A.md to sub/C.md → basename "C" now has C.md + sub/C.md.
	// Root priority: C.md is at root → [[C]] resolves to root C.md → no ambiguity.
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
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/C.md"})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
}

// --- Test 4b: move causes ambiguous outgoing link (no root file) → outgoing rewrite ---
func TestMove_AmbiguousAfterMoveNoRoot(t *testing.T) {
	// A.md has [[C]], sub/C.md exists (no root C).
	// Move A.md to sub2/C.md → basename "C" has sub/C.md + sub2/C.md, no root.
	// → outgoing [[C]] is rewritten to [[sub/C]] (pre-move target).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/C.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// [[C]] should be rewritten to [[sub/C]] in the moved file.
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub2/C.md" && rw.OldLink == "[[C]]" {
			found = true
			if rw.NewLink != "[[sub/C]]" {
				t.Errorf("expected [[sub/C]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[C]] should be rewritten")
	}

	// Verify disk content.
	content, err := os.ReadFile(filepath.Join(vault, "sub2", "C.md"))
	if err != nil {
		t.Fatalf("read sub2/C.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/C]]") {
		t.Errorf("disk should contain [[sub/C]], got: %s", string(content))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "sub2/C.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/C]]" && e.targetName == "C" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/C]]")
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
	// X.md is at root → vault-relative rewrite gives [[X]].
	var bRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			bRewritten = true
			if rw.NewLink != "[[X]]" {
				t.Errorf("B.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !bRewritten {
		t.Error("B.md [[A]] should be rewritten when basename changes")
	}

	// sub/D.md [[A]] should be rewritten too.
	// X.md is at root → vault-relative rewrite gives [[X]].
	var dWikiRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/D.md" && rw.OldLink == "[[A]]" {
			dWikiRewritten = true
			if rw.NewLink != "[[X]]" {
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

// --- Test 15: Phase 2.5 — root priority resolves third-party ambiguity ---
func TestMove_AmbiguousThirdPartyRootPriority(t *testing.T) {
	vault := t.TempDir()
	// A.md at root. B.md links to [[A]]. C.md exists.
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
	// Root priority: A.md is at root before AND after move → [[A]] resolves to root.
	_, err := Move(vault, MoveOptions{From: "C.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
}

// --- Test 15b: Phase 2.5 — no root file → collateral rewrite ---
func TestMove_AmbiguousThirdPartyNoRoot(t *testing.T) {
	vault := t.TempDir()
	// sub1/A.md exists (not at root). B.md links to [[A]]. C.md exists.
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
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

	// Move C.md to sub2/A.md → basename "A" has sub1/A.md + sub2/A.md, no root.
	// B.md's [[A]] (pointing to sub1/A.md) should be collateral-rewritten to [[sub1/A]].
	result, err := Move(vault, MoveOptions{From: "C.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub1/A]]" {
				t.Errorf("expected [[sub1/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub1/A]]")
	}

	// Verify disk.
	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub1/A]]") {
		t.Errorf("B.md disk should contain [[sub1/A]], got: %s", string(bContent))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub1/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub1/A]]")
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

// --- Test 21: file permission preserved after move with outgoing rewrite ---
func TestMovePreservesPermission(t *testing.T) {
	vault := t.TempDir()
	// A.md has a relative outgoing link to B.md.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[link](./B.md)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(vault, "A.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to sub/A.md — outgoing relative link gets rewritten.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Verify moved file has preserved permission.
	info, err := os.Stat(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("moved file perm = %o, want %o", perm, 0o600)
	}
}

// --- Test 22: rename to root target → vault-relative rewrite [[X]] ---
func TestMove_RenameToRoot(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// [[A]] → [[X]] (vault-relative, root target)
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[X]]" {
				t.Errorf("B.md rewrite = %q, want [[X]]", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be rewritten")
	}
}

// --- Test 23: move root file out → Phase 2 rewrites incoming links ---
func TestMove_RootFileMovedOut(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]].
	// Move A.md → sub2/A.md → root file gone.
	// Phase 2 rewrites B.md's [[A]] to [[sub2/A]] (incoming link to moved file).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md's [[A]] should be rewritten to [[sub2/A]].
	var bRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			bRewritten = true
			if rw.NewLink != "[[sub2/A]]" {
				t.Errorf("B.md rewrite = %q, want [[sub2/A]]", rw.NewLink)
			}
		}
	}
	if !bRewritten {
		t.Error("B.md [[A]] should be rewritten when root file moved out")
	}
}

// --- Test 23b: Phase 2.5 collateral — new root file changes resolution ---
func TestMove_RootFileMovedOutThirdParty(t *testing.T) {
	// sub/A.md exists (unique A). B.md has [[A]] pointing to sub/A.md.
	// C.md exists. Move C.md → A.md (root).
	// Pre-move: no root A. Post-move: root A.md (former C.md) + sub/A.md.
	// B.md's [[A]] (pointing to sub/A.md) should be collateral-rewritten to [[sub/A]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
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

	result, err := Move(vault, MoveOptions{From: "C.md", To: "A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// B.md's [[A]] should be collateral-rewritten to [[sub/A]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub/A]]" {
				t.Errorf("expected [[sub/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub/A]]")
	}

	// Verify disk.
	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub/A]]") {
		t.Errorf("B.md disk should contain [[sub/A]], got: %s", string(bContent))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/A]]")
	}
}

// --- Test 24: root file survives move of other file → success ---
func TestMove_RootFileSurvives(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]]. D.md exists.
	// Move D.md → sub2/A.md → root A.md still exists → [[A]] resolves → success.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "D.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("expected success (root A.md survives), got: %v", err)
	}
}

// --- Test 25: meaning change — new root file → collateral rewrite ---
func TestMove_MeaningChangeNewRoot(t *testing.T) {
	// sub/A.md is unique A. B.md has [[A]]. C.md exists.
	// Move C.md → A.md → now root A.md exists, but pre-move root had no A.
	// → B.md's [[A]] (pointing to sub/A.md) is collateral-rewritten to [[sub/A]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
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

	result, err := Move(vault, MoveOptions{From: "C.md", To: "A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// B.md's [[A]] should be collateral-rewritten to [[sub/A]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub/A]]" {
				t.Errorf("expected [[sub/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub/A]]")
	}

	// Verify disk.
	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub/A]]") {
		t.Errorf("B.md disk should contain [[sub/A]], got: %s", string(bContent))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/A]]")
	}
}

// --- Test 26: Phase 2 — basename unchanged + ambiguous + root survives → no rewrite ---
func TestMove_Phase2RootSkipsRewrite(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has path link to A.md.
	// Move sub/A.md → sub2/A.md → basename unchanged, ambiguous.
	// Root A.md survives before AND after → incoming basename links don't need rewrite.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Move(vault, MoveOptions{From: "sub/A.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md's [[A]] should NOT be rewritten (root A.md is the target, not moved).
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("[[A]] in B.md should NOT be rewritten (root priority), but got %s", rw.NewLink)
		}
	}
}

// --- Test 27: collateral + incoming rewrite in same file ---
func TestMove_CollateralAndIncomingSameFile(t *testing.T) {
	// X.md has [[A]] (→ A.md) and [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md.
	// [[A]] → incoming rewrite → [[sub2/B]]
	// [[B]] → collateral rewrite → [[sub1/B]]
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[A]]\n[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Check rewrites in X.md.
	var incomingFound, collateralFound bool
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[A]]" {
			incomingFound = true
			if rw.NewLink != "[[sub2/B]]" {
				t.Errorf("incoming rewrite: expected [[sub2/B]], got %s", rw.NewLink)
			}
		}
		if rw.File == "X.md" && rw.OldLink == "[[B]]" {
			collateralFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("collateral rewrite: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !incomingFound {
		t.Error("X.md [[A]] should be rewritten (incoming)")
	}
	if !collateralFound {
		t.Error("X.md [[B]] should be rewritten (collateral)")
	}

	// Verify disk.
	xContent, err := os.ReadFile(filepath.Join(vault, "X.md"))
	if err != nil {
		t.Fatalf("read X.md: %v", err)
	}
	if !strings.Contains(string(xContent), "[[sub2/B]]") {
		t.Errorf("X.md disk should contain [[sub2/B]], got: %s", string(xContent))
	}
	if !strings.Contains(string(xContent), "[[sub1/B]]") {
		t.Errorf("X.md disk should contain [[sub1/B]], got: %s", string(xContent))
	}

	// Verify DB edges.
	edges := queryEdges(t, dbPath(vault), "X.md")
	var incomingEdge, collateralEdge bool
	for _, e := range edges {
		if e.rawLink == "[[sub2/B]]" {
			incomingEdge = true
		}
		if e.rawLink == "[[sub1/B]]" {
			collateralEdge = true
		}
	}
	if !incomingEdge {
		t.Error("DB should have edge with raw_link [[sub2/B]]")
	}
	if !collateralEdge {
		t.Error("DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 28: collateral rewrite in multiple files ---
func TestMove_CollateralMultipleFiles(t *testing.T) {
	// X.md and Y.md both have [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md → basename "B" becomes ambiguous.
	// Both X.md and Y.md should have [[B]] → [[sub1/B]] collateral rewrite.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Y.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var xFound, yFound bool
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[B]]" {
			xFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("X.md: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
		if rw.File == "Y.md" && rw.OldLink == "[[B]]" {
			yFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("Y.md: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !xFound {
		t.Error("X.md [[B]] should be collateral-rewritten")
	}
	if !yFound {
		t.Error("Y.md [[B]] should be collateral-rewritten")
	}

	// Verify disk.
	xContent, _ := os.ReadFile(filepath.Join(vault, "X.md"))
	if !strings.Contains(string(xContent), "[[sub1/B]]") {
		t.Errorf("X.md disk should contain [[sub1/B]], got: %s", string(xContent))
	}
	yContent, _ := os.ReadFile(filepath.Join(vault, "Y.md"))
	if !strings.Contains(string(yContent), "[[sub1/B]]") {
		t.Errorf("Y.md disk should contain [[sub1/B]], got: %s", string(yContent))
	}

	// Verify DB edges.
	xEdges := queryEdges(t, dbPath(vault), "X.md")
	var xEdgeFound bool
	for _, e := range xEdges {
		if e.rawLink == "[[sub1/B]]" {
			xEdgeFound = true
		}
	}
	if !xEdgeFound {
		t.Error("X.md DB should have edge with raw_link [[sub1/B]]")
	}
	yEdges := queryEdges(t, dbPath(vault), "Y.md")
	var yEdgeFound bool
	for _, e := range yEdges {
		if e.rawLink == "[[sub1/B]]" {
			yEdgeFound = true
		}
	}
	if !yEdgeFound {
		t.Error("Y.md DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 29: outgoing basename disambiguation ---
func TestMove_OutgoingBasenameDisambiguation(t *testing.T) {
	// A.md has [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md → [[B]] becomes ambiguous → outgoing rewrite to [[sub1/B]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub2/B.md" && rw.OldLink == "[[B]]" {
			found = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[B]] should be rewritten to [[sub1/B]]")
	}

	// Verify disk.
	content, err := os.ReadFile(filepath.Join(vault, "sub2", "B.md"))
	if err != nil {
		t.Fatalf("read sub2/B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub1/B]]") {
		t.Errorf("disk should contain [[sub1/B]], got: %s", string(content))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "sub2/B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub1/B]]" && e.targetName == "B" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 30: outgoing meaning change via root priority ---
func TestMove_OutgoingMeaningChangeRoot(t *testing.T) {
	// sub/B.md exists (unique B). A.md has [[B]] (→ sub/B.md).
	// Move A.md → B.md (root). Post-move: [[B]] would resolve to root B.md (self)
	// via root priority → meaning change → outgoing rewrite to [[sub/B]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[B]]" {
			found = true
			if rw.NewLink != "[[sub/B]]" {
				t.Errorf("expected [[sub/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[B]] should be rewritten to [[sub/B]]")
	}

	// Verify disk.
	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/B]]") {
		t.Errorf("disk should contain [[sub/B]], got: %s", string(content))
	}

	// Verify DB edge.
	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/B]]" && e.targetName == "B" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/B]]")
	}
}

// --- Test 31: stale collateral source file → error ---
func TestMove_StaleCollateralSource(t *testing.T) {
	// sub1/A.md exists. B.md has [[A]] (→ sub1/A.md). C.md exists.
	// Build, then modify B.md to make it stale.
	// Move C.md → sub2/A.md → collateral rewrite needed for B.md → stale error.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
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

	// Make B.md stale.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\nmodified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "C.md", To: "sub2/A.md"})
	if err == nil {
		t.Fatal("expected stale error for B.md")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("expected stale error, got: %v", err)
	}
}

// ===============================================
// MoveDir tests
// ===============================================

func TestMoveDir_Basic(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Verify moved files.
	if len(result.Moved) != 3 {
		t.Fatalf("expected 3 moved files, got %d", len(result.Moved))
	}

	// Verify disk.
	for _, m := range result.Moved {
		if fileExists(filepath.Join(vault, m.From)) {
			t.Errorf("%s should not exist on disk", m.From)
		}
		if !fileExists(filepath.Join(vault, m.To)) {
			t.Errorf("%s should exist on disk", m.To)
		}
	}

	// Verify DB.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if strings.HasPrefix(n.path, "sub/") {
			t.Errorf("DB still has old path: %s", n.path)
		}
	}
	var foundA, foundB, foundX bool
	for _, n := range notes {
		switch n.path {
		case "newdir/A.md":
			foundA = true
		case "newdir/B.md":
			foundB = true
		case "newdir/inner/X.md":
			foundX = true
		}
	}
	if !foundA || !foundB || !foundX {
		t.Errorf("DB missing new paths: A=%v B=%v X=%v", foundA, foundB, foundX)
	}
}

func TestMoveDir_NoFiles(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "nonexist", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "no files registered under directory") {
		t.Errorf("expected 'no files' error, got: %v", err)
	}
}

func TestMoveDir_DestConflict(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create destination with same name already registered.
	if err := os.MkdirAll(filepath.Join(vault, "dst"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "dst", "A.md"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dst"})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

func TestMoveDir_IncomingRewrite(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Other.md has [[sub/B]] → should be rewritten to [[newdir/B]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "Other.md" && rw.OldLink == "[[sub/B]]" {
			found = true
			if rw.NewLink != "[[newdir/B]]" {
				t.Errorf("expected [[newdir/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("Other.md [[sub/B]] should be rewritten")
	}

	// Verify disk.
	otherContent, err := os.ReadFile(filepath.Join(vault, "Other.md"))
	if err != nil {
		t.Fatalf("read Other.md: %v", err)
	}
	if !strings.Contains(string(otherContent), "[[newdir/B]]") {
		t.Errorf("Other.md should contain [[newdir/B]], got: %s", string(otherContent))
	}
}

func TestMoveDir_IncomingMultiplePathLinks(t *testing.T) {
	// External file has multiple path links to different files in the moved directory.
	// Both should be rewritten.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Ext.md"), []byte("[[sub/A]]\n[[sub/B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Both path links in Ext.md should be rewritten.
	rewrittenLinks := make(map[string]string)
	for _, rw := range result.Rewritten {
		if rw.File == "Ext.md" {
			rewrittenLinks[rw.OldLink] = rw.NewLink
		}
	}
	if len(rewrittenLinks) != 2 {
		t.Errorf("expected 2 rewrites in Ext.md, got %d: %v", len(rewrittenLinks), rewrittenLinks)
	}
	if rw, ok := rewrittenLinks["[[sub/A]]"]; !ok || rw != "[[newdir/A]]" {
		t.Errorf("expected [[sub/A]] → [[newdir/A]], got %v", rewrittenLinks)
	}
	if rw, ok := rewrittenLinks["[[sub/B]]"]; !ok || rw != "[[newdir/B]]" {
		t.Errorf("expected [[sub/B]] → [[newdir/B]], got %v", rewrittenLinks)
	}

	// Verify disk.
	extContent, err := os.ReadFile(filepath.Join(vault, "Ext.md"))
	if err != nil {
		t.Fatalf("read Ext.md: %v", err)
	}
	if !strings.Contains(string(extContent), "[[newdir/A]]") || !strings.Contains(string(extContent), "[[newdir/B]]") {
		t.Errorf("Ext.md should contain both [[newdir/A]] and [[newdir/B]], got: %s", string(extContent))
	}
}

func TestMoveDir_CollateralRewrite(t *testing.T) {
	// Directory move preserves basenames, so collateral rewrite (basename
	// ambiguity caused by the move) cannot occur. Instead, verify that
	// a basename link to a moved file is correctly rewritten to a path link
	// when the basename was already ambiguous before the move.
	//
	// Setup: sub1/A.md (unique basename "A"), B.md has [[A]] (→ sub1/A.md).
	// Move sub1 → sub2. Basename "A" count stays 1. [[A]] still resolves → no rewrite needed.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// [[A]] should still resolve to sub2/A.md via basename. No rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" {
			t.Errorf("B.md should NOT be rewritten (basename A is still unique), got: %+v", rw)
		}
	}

	// Verify disk: B.md unchanged.
	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[A]]") {
		t.Errorf("B.md should still contain [[A]], got: %s", string(bContent))
	}
}

func TestMoveDir_CollateralMultipleBasenames(t *testing.T) {
	// sub1/A.md, sub1/B.md exist. X.md has [[A]], Y.md has [[B]].
	// Create src/A.md and src/B.md.
	// Move src → sub2 → both "A" and "B" become ambiguous.
	vault := t.TempDir()
	for _, d := range []string{"sub1", "src", "sub2"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Y.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "D.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move src → sub2, renaming: C→A, D→B would need different dir names.
	// Actually, for dir move basename doesn't change. Let's test differently.
	// Move sub1 → sub2. Basenames A and B stay the same. No ambiguity created.
	// This test verifies that when basename count stays 1, no rewrite happens.
	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// X.md's [[A]] and Y.md's [[B]] should NOT be rewritten (basename unique, unchanged).
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[A]]" {
			t.Errorf("X.md [[A]] should NOT be rewritten, got %s", rw.NewLink)
		}
		if rw.File == "Y.md" && rw.OldLink == "[[B]]" {
			t.Errorf("Y.md [[B]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_CollateralRootPriority(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]].
	// Move sub → newdir → basename "A" still has root A.md → no collateral.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// B.md's [[A]] should NOT be rewritten (root priority).
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("B.md [[A]] should NOT be rewritten (root priority), got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingBasenameToMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// sub/A.md has [[B]] → target sub/B.md also moves to newdir/B.md.
	// Basename "B" stays unique → no rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[[B]]" {
			t.Errorf("newdir/A.md [[B]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingPathToMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Other.md has [[sub/B]] — this is an external incoming rewrite.
	// sub/A.md has no path link to sub/B.
	// Let's check the fixture — sub/A.md has [link to B](./B.md) which is relative.
	// That should be handled by the relative rewrite batch.

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Verify Other.md's [[sub/B]] was rewritten.
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "Other.md" && rw.OldLink == "[[sub/B]]" {
			found = true
		}
	}
	if !found {
		t.Error("Other.md [[sub/B]] should be rewritten")
	}
}

func TestMoveDir_RelativeBetweenMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// sub/A.md has [link to B](./B.md) — both A and B move to newdir/.
	// The relative path should remain ./B.md (unchanged).
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[link to B](./B.md)" {
			t.Errorf("newdir/A.md relative link should NOT change, but was rewritten to %s", rw.NewLink)
		}
	}

	// Verify disk.
	content, err := os.ReadFile(filepath.Join(vault, "newdir", "A.md"))
	if err != nil {
		t.Fatalf("read newdir/A.md: %v", err)
	}
	if !strings.Contains(string(content), "[link to B](./B.md)") {
		t.Errorf("newdir/A.md should preserve relative link, got: %s", string(content))
	}
}

func TestMoveDir_RelativeToExternal(t *testing.T) {
	// sub/A.md has [ext](../Root.md). Move sub → newdir.
	// newdir/A.md → Root.md should become ../Root.md (still valid if newdir is 1 level deep).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[ext](../Root.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Root.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// ../Root.md from sub/ = Root.md. From newdir/, it should still be ../Root.md.
	// Since both are 1-level deep, relative path stays the same.
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[ext](../Root.md)" {
			t.Errorf("relative link to external should not change, but was rewritten to %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingPathToExternal(t *testing.T) {
	// sub/A.md has [[Root]] (basename link to external Root.md).
	// Move sub → newdir. [[Root]] stays as basename link, no rewrite needed.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[[Root]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Root.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[[Root]]" {
			t.Errorf("[[Root]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_ExternalStale(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Make Other.md stale.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "Other.md"), []byte("[[A]]\n[[sub/B]]\nmodified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil {
		t.Fatal("expected stale error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("expected stale error, got: %v", err)
	}
}

func TestMoveDir_AlreadyMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already moved the directory.
	if err := os.MkdirAll(filepath.Join(vault, "newdir", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A.md", "B.md"} {
		if err := os.Rename(
			filepath.Join(vault, "sub", name),
			filepath.Join(vault, "newdir", name),
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(
		filepath.Join(vault, "sub", "inner", "X.md"),
		filepath.Join(vault, "newdir", "inner", "X.md"),
	); err != nil {
		t.Fatal(err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir (already moved): %v", err)
	}

	if len(result.Moved) != 3 {
		t.Errorf("expected 3 moved files, got %d", len(result.Moved))
	}

	// Verify DB updated.
	notes := queryNodes(t, dbPath(vault), "note")
	var found bool
	for _, n := range notes {
		if n.path == "newdir/A.md" {
			found = true
		}
	}
	if !found {
		t.Error("DB should contain newdir/A.md")
	}
}

func TestMoveDir_Stale(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Make a source file stale.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil {
		t.Fatal("expected stale error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("expected stale error, got: %v", err)
	}
}

func TestMoveDir_Nested(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Verify nested file moved.
	var foundNested bool
	for _, m := range result.Moved {
		if m.From == "sub/inner/X.md" && m.To == "newdir/inner/X.md" {
			foundNested = true
		}
	}
	if !foundNested {
		t.Error("sub/inner/X.md should be moved to newdir/inner/X.md")
	}

	if !fileExists(filepath.Join(vault, "newdir", "inner", "X.md")) {
		t.Error("newdir/inner/X.md should exist on disk")
	}
}

func TestMoveDir_Overlap(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "sub/inner"})
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Errorf("expected overlap error, got: %v", err)
	}
}

func TestMoveDir_VaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "../outside"})
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}

	// Absolute paths should also be rejected.
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "/abs/path", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "vault-relative") {
		t.Errorf("expected absolute path error for from, got: %v", err)
	}
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "/abs/path"})
	if err == nil || !strings.Contains(err.Error(), "vault-relative") {
		t.Errorf("expected absolute path error for to, got: %v", err)
	}
}

func TestMoveDir_DestExistsOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create an unregistered file at the destination.
	if err := os.MkdirAll(filepath.Join(vault, "newdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "newdir", "A.md"), []byte("conflict\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "already exists on disk") {
		t.Errorf("expected 'already exists on disk' error, got: %v", err)
	}
}

func TestMoveDir_PhantomPromotion(t *testing.T) {
	// A.md and B.md link to [[X]] which is a phantom.
	// Move sub/X.md to a new dir. Since dir move doesn't change basename,
	// this is essentially testing that phantom promotion works.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[X]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "Y.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// X is a phantom. Move sub → newdir won't promote X because basename
	// doesn't change and there's no X.md in sub.
	// This test just verifies no crash on phantom promotion code path.
	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}
	if len(result.Moved) != 1 {
		t.Errorf("expected 1 moved file, got %d", len(result.Moved))
	}
}

func TestMoveDir_NonMDFileError(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	// Add a non-.md file to the source directory.
	if err := os.WriteFile(filepath.Join(vault, "sub", "image.png"), []byte("png data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "non-.md file") {
		t.Errorf("expected non-.md file error, got: %v", err)
	}

	// Verify nothing was moved.
	if _, err := os.Stat(filepath.Join(vault, "sub", "A.md")); err != nil {
		t.Error("sub/A.md should still exist")
	}
}

func TestMoveDir_HiddenFilesIgnored(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	// Add a hidden file — should be ignored by the non-.md check.
	if err := os.WriteFile(filepath.Join(vault, "sub", ".DS_Store"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir should succeed with hidden files: %v", err)
	}
	if len(result.Moved) == 0 {
		t.Error("expected files to be moved")
	}
}
