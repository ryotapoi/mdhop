package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateUnregisteredFile(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Update(vault, UpdateOptions{Files: []string{"NotExist.md"}})
	if err == nil || !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("expected file not registered error, got: %v", err)
	}

	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

// Update must apply the same vault-escape guard to frontmatter wikilinks as
// build does (rules/03-data-model.md). Editing A.md to add a frontmatter
// wikilink that escapes the vault should fail update.
func TestUpdate_FrontmatterWikilinkEscapesVault(t *testing.T) {
	vault := t.TempDir()
	aContent := "# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	bad := `---
parent: "[[../escape]]"
---
# A
`
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil {
		t.Fatal("expected vault escape error for frontmatter wikilink, got nil")
	}
	if !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("error = %q, want containing 'escapes vault'", err.Error())
	}
}

func TestUpdateContentChange(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Verify A→B edge exists.
	edges := queryEdges(t, dbPath(vault), "A.md")
	var hasB bool
	for _, e := range edges {
		if e.targetName == "B" && e.linkType == "wikilink" {
			hasB = true
		}
	}
	if !hasB {
		t.Fatal("expected A→B edge before update")
	}

	// Change A.md to link to C instead of B.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "A.md" {
		t.Errorf("Updated = %v, want [A.md]", result.Updated)
	}

	// A→B edge should be gone, A→C should exist.
	edges = queryEdges(t, dbPath(vault), "A.md")
	var hasBAfter, hasCAfter bool
	for _, e := range edges {
		if e.targetName == "B" && e.linkType == "wikilink" {
			hasBAfter = true
		}
		if e.targetName == "C" && e.linkType == "wikilink" {
			hasCAfter = true
		}
	}
	if hasBAfter {
		t.Error("A→B edge should have been removed")
	}
	if !hasCAfter {
		t.Error("A→C edge should have been created")
	}
}

func TestUpdateDeletedFileWithRefs(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove B.md from disk (A references B).
	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}

	// B should now be a phantom.
	phantoms := queryNodes(t, dbPath(vault), "phantom")
	var foundB bool
	for _, p := range phantoms {
		if p.name == "B" {
			foundB = true
			if p.existsFlag != 0 {
				t.Errorf("phantom B should have exists_flag=0")
			}
		}
	}
	if !foundB {
		t.Error("phantom B not found after update")
	}
}

func TestUpdateDeletedFileNoRefs(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove C.md from disk (no references to C).
	if err := os.Remove(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"C.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}

	// C node should not exist at all.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if n.path == "C.md" {
			t.Error("C.md note should have been deleted")
		}
	}
	phantoms := queryNodes(t, dbPath(vault), "phantom")
	for _, p := range phantoms {
		if p.name == "C" {
			t.Error("phantom C should not exist (no references)")
		}
	}
}

func TestUpdateAmbiguousLinkRootPriority(t *testing.T) {
	// D.md at root + sub/D.md → [[D]] resolves to root D.md via root priority → success.
	vault := copyVault(t, "vault_update")
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "D.md"), []byte("# D sub\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("# D root\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Update A.md to link to [[D]] — root priority resolves to D.md at root.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n[[D]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}

	// Verify the edge points to root D.md.
	edges := queryEdges(t, dbPath(vault), "A.md")
	var hasDNote bool
	for _, e := range edges {
		if e.targetName == "D" && e.targetType == NodeTypeNote {
			hasDNote = true
		}
	}
	if !hasDNote {
		t.Error("A→D edge should exist and point to note")
	}
}

func TestUpdateAmbiguousLinkNoRoot(t *testing.T) {
	// sub1/D.md + sub2/D.md (no root) → [[D]] is ambiguous → error.
	vault := copyVault(t, "vault_update")
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "D.md"), []byte("# D1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "D.md"), []byte("# D2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeEdges := countEdges(t, dbPath(vault))

	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n[[D]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous link error, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "(candidates: sub1/D.md, sub2/D.md)") {
		t.Errorf("expected candidates in error, got: %v", err)
	}

	afterEdges := countEdges(t, dbPath(vault))
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestUpdateOrphanCleanup(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// #tagA should exist (only A references it).
	tags := queryNodes(t, dbPath(vault), "tag")
	var hasTagA bool
	for _, tag := range tags {
		if tag.name == "#tagA" {
			hasTagA = true
		}
	}
	if !hasTagA {
		t.Fatal("#tagA should exist before update")
	}

	// Update A.md to remove #tagA.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// #tagA should be cleaned up.
	tags = queryNodes(t, dbPath(vault), "tag")
	for _, tag := range tags {
		if tag.name == "#tagA" {
			t.Error("#tagA should have been cleaned up as orphan")
		}
	}
	// #shared should still exist.
	var hasShared bool
	for _, tag := range tags {
		if tag.name == "#shared" {
			hasShared = true
		}
	}
	if !hasShared {
		t.Error("#shared tag should still exist")
	}
}

func TestUpdateMtime(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Rewrite A.md (changes mtime).
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Verify mtime matches disk.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE path='A.md'").Scan(&dbMtime); err != nil {
		t.Fatalf("query mtime: %v", err)
	}
	info, err := os.Stat(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if dbMtime != info.ModTime().Unix() {
		t.Errorf("mtime = %d, want %d", dbMtime, info.ModTime().Unix())
	}
}

func TestUpdateMultipleFiles(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Rewrite both A and B.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[C]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write B: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"A.md", "B.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Updated) != 2 {
		t.Errorf("Updated = %v, want 2 files", result.Updated)
	}

	// Both should now link to C.
	edgesA := queryEdges(t, dbPath(vault), "A.md")
	edgesB := queryEdges(t, dbPath(vault), "B.md")
	var aToC, bToC bool
	for _, e := range edgesA {
		if e.targetName == "C" && e.linkType == "wikilink" {
			aToC = true
		}
	}
	for _, e := range edgesB {
		if e.targetName == "C" && e.linkType == "wikilink" {
			bToC = true
		}
	}
	if !aToC {
		t.Error("A→C edge not found")
	}
	if !bToC {
		t.Error("B→C edge not found")
	}
}

func TestUpdateNewPhantom(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a link to a non-existent note.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n[[Missing]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	phantoms := queryNodes(t, dbPath(vault), "phantom")
	var hasMissing bool
	for _, p := range phantoms {
		if p.name == "Missing" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Error("phantom Missing should have been created")
	}

	// Edge A→Missing should exist.
	edges := queryEdges(t, dbPath(vault), "A.md")
	var hasEdge bool
	for _, e := range edges {
		if e.targetName == "Missing" && e.targetType == NodeTypePhantom {
			hasEdge = true
		}
	}
	if !hasEdge {
		t.Error("edge A→Missing not found")
	}
}

func TestUpdateNewTag(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a new tag.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n#tagA\n#shared\n#newTag\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	tags := queryNodes(t, dbPath(vault), "tag")
	var hasNewTag bool
	for _, tag := range tags {
		if tag.name == "#newTag" {
			hasNewTag = true
		}
	}
	if !hasNewTag {
		t.Error("tag #newTag should have been created")
	}
}

func TestUpdateNoIndex(t *testing.T) {
	vault := copyVault(t, "vault_update")
	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "index not found") {
		t.Errorf("expected index not found error, got: %v", err)
	}
}

func TestUpdateVaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeEdges := countEdges(t, dbPath(vault))

	// Add a vault-escaping link.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[up](../Outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}

	afterEdges := countEdges(t, dbPath(vault))
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestUpdateEscapeVaultNonRelative(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Write a non-relative path that escapes vault via ".."
	if err := os.WriteFile(filepath.Join(vault, "A.md"),
		[]byte("[link](sub/../../Outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}
}

func TestUpdateDeletedFileExistingPhantom(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manually insert a phantom "B" to simulate pre-existing phantom.
	db := openTestDB(t, dbPath(vault))
	phantomKey := "phantom:name:b"
	_, err := db.Exec(
		"INSERT INTO nodes (node_key, type, name, path, exists_flag) VALUES (?, 'phantom', 'B', NULL, 0)",
		phantomKey,
	)
	if err != nil {
		db.Close()
		t.Fatalf("insert phantom: %v", err)
	}
	var existingPhantomID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&existingPhantomID); err != nil {
		db.Close()
		t.Fatalf("query phantom id: %v", err)
	}
	db.Close()

	// Remove B.md from disk.
	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}

	// Note B should be deleted (edges reassigned to existing phantom).
	db2 := openTestDB(t, dbPath(vault))
	defer db2.Close()
	var noteCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='note' AND name='B'").Scan(&noteCount); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if noteCount != 0 {
		t.Error("note B should have been deleted")
	}

	// Incoming edges should point to the pre-existing phantom.
	var inCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", existingPhantomID).Scan(&inCount); err != nil {
		t.Fatalf("count incoming: %v", err)
	}
	if inCount != 1 {
		t.Errorf("existing phantom should have 1 incoming edge, got %d", inCount)
	}
}

func TestUpdateDeletedWithSimultaneousRef(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove B.md and simultaneously update A.md (A references B).
	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// A still links to B.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"A.md", "B.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "A.md" {
		t.Errorf("Updated = %v, want [A.md]", result.Updated)
	}
	// Phase A creates A→phantom(B) edge via resolveLink (B is removed from maps).
	// Phase B finds note B has 0 incoming edges (the new edge points to phantom B,
	// not note B), so note B is completely deleted rather than phantom-converted.
	if len(result.Deleted) != 1 || result.Deleted[0] != "B.md" {
		t.Errorf("Deleted = %v, want [B.md]", result.Deleted)
	}

	// A→B edge should point to phantom B (created by resolveLink in Phase A).
	edges := queryEdges(t, dbPath(vault), "A.md")
	var hasPhantomB bool
	for _, e := range edges {
		if e.targetName == "B" && e.targetType == NodeTypePhantom {
			hasPhantomB = true
		}
	}
	if !hasPhantomB {
		t.Error("A should link to phantom B after simultaneous update+delete")
	}
}

func TestUpdatePartialErrorNoChanges(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md", "NotExist.md"}})
	if err == nil || !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("expected file not registered error, got: %v", err)
	}

	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestUpdateBasenameCountTransition(t *testing.T) {
	// Two files with same basename → delete one → basename becomes unique.
	vault := copyVault(t, "vault_update")
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create sub/A.md (same basename as A.md).
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("# A sub\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// B.md links to A using path link (not basename → not ambiguous).
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[sub/A]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove sub/A.md from disk.
	if err := os.Remove(filepath.Join(vault, "sub", "A.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Update sub/A.md — after this, basename "a" is unique.
	// B links to sub/A, so sub/A.md has incoming edges → phantom conversion.
	result, err := Update(vault, UpdateOptions{Files: []string{"sub/A.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "sub/A.md" {
		t.Errorf("Phantomed = %v, want [sub/A.md]", result.Phantomed)
	}

	// Now update B.md with [[A]] — should resolve to A.md (now unique basename).
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write B: %v", err)
	}
	result2, err := Update(vault, UpdateOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("update B: %v", err)
	}
	if len(result2.Updated) != 1 {
		t.Errorf("Updated = %v, want [B.md]", result2.Updated)
	}

	// B→A should resolve to note (not phantom).
	edges := queryEdges(t, dbPath(vault), "B.md")
	var hasNoteA bool
	for _, e := range edges {
		if e.targetName == "A" && e.targetType == NodeTypeNote {
			hasNoteA = true
		}
	}
	if !hasNoteA {
		t.Error("B→A should resolve to note A.md after basename collision resolved")
	}
}

func TestUpdateIncomingEdgesPreserved(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// B→A edge should exist.
	edgesB := queryEdges(t, dbPath(vault), "B.md")
	var hasBtoA bool
	for _, e := range edgesB {
		if e.targetName == "A" {
			hasBtoA = true
		}
	}
	if !hasBtoA {
		t.Fatal("B→A edge should exist before update")
	}

	// Update A.md (non-target file B's edges should be preserved).
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// B→A edge should still exist.
	edgesB = queryEdges(t, dbPath(vault), "B.md")
	var hasBtoAAfter bool
	for _, e := range edgesB {
		if e.targetName == "A" {
			hasBtoAAfter = true
		}
	}
	if !hasBtoAAfter {
		t.Error("B→A incoming edge should be preserved after updating A")
	}
}

// --- Meta update tests ---

func TestUpdateMetaReinsert(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: Old\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 1 || meta[0].Value != "Old" {
		t.Fatalf("expected title=Old, got %+v", meta)
	}

	// Change frontmatter.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: New\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	meta = queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 1 || meta[0].Value != "New" {
		t.Errorf("expected title=New after update, got %+v", meta)
	}
}

func TestUpdateMetaRemoveFrontmatter(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: Hello\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if c := countMeta(t, dbPath(vault)); c != 1 {
		t.Fatalf("expected 1 meta row before update, got %d", c)
	}

	// Remove frontmatter.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows after removing frontmatter, got %d", c)
	}
}

func TestUpdateMetaAddFrontmatter(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Fatalf("expected 0 meta rows before update, got %d", c)
	}

	// Add frontmatter.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: Hello\nauthor: Me\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 2 {
		t.Errorf("expected 2 meta rows after adding frontmatter, got %d", len(meta))
	}
}

func TestUpdateDeletedFileMetaCleanup(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: Hi\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if c := countMeta(t, dbPath(vault)); c != 1 {
		t.Fatalf("expected 1 meta row before update, got %d", c)
	}

	os.Remove(filepath.Join(vault, "A.md"))
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows after file deletion via update, got %d", c)
	}
}

func TestUpdateMetaWarnings(t *testing.T) {
	vault := t.TempDir()
	// Create mdhop.yaml with date type for "date" key.
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"),
		[]byte("meta:\n  types:\n    date: date\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ndate: 2024-01-01\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Update with invalid date value.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ndate: not-a-date\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestUpdateMetaRoundTrip(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: V1\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 1 || meta[0].Value != "V1" {
		t.Fatalf("step 1: expected title=V1, got %+v", meta)
	}

	// Update #1: change to V2.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: V2\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	meta = queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 1 || meta[0].Value != "V2" {
		t.Errorf("step 2: expected title=V2, got %+v", meta)
	}

	// Update #2: change to V3 and add a key.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: V3\nauthor: Alice\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("update 2: %v", err)
	}
	meta = queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 2 {
		t.Errorf("step 3: expected 2 meta rows, got %d: %+v", len(meta), meta)
	}
}

func TestUpdateInvalidConfig(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeMeta := countMeta(t, dbPath(vault))

	// Write invalid config.
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("meta:\n  types:\n    date: invalid_type\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("unexpected error: %v", err)
	}

	afterMeta := countMeta(t, dbPath(vault))
	if beforeMeta != afterMeta {
		t.Errorf("meta count changed: %d → %d", beforeMeta, afterMeta)
	}
}

func TestUpdateRootDeletedResolvesToSubdir(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]].
	// Delete A.md from disk, then update A.md + B.md.
	// After update, basename "a" count = 1 (sub/A.md only).
	// B.md's [[A]] resolves to sub/A.md (unique basename) → no ambiguity.
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
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Delete root A.md from disk.
	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Update B.md content to trigger re-parse.
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Update(vault, UpdateOptions{Files: []string{"A.md", "B.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	// A.md should be phantom-converted or deleted.
	if len(result.Phantomed)+len(result.Deleted) == 0 {
		t.Error("A.md should be phantomed or deleted")
	}

	// B.md's [[A]] should now point to sub/A.md (the only remaining A).
	edges := queryEdges(t, dbPath(vault), "B.md")
	var foundA bool
	for _, e := range edges {
		if e.targetName == "A" && e.targetType == NodeTypeNote {
			foundA = true
		}
	}
	if !foundA {
		t.Error("B→A edge should exist pointing to sub/A.md")
	}
}
