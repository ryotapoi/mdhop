package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateUnregisteredFile(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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

func TestUpdateContentChange(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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

func TestUpdateAmbiguousLink(t *testing.T) {
	vault := copyVault(t, "vault_update")
	// Add D.md with same basename concept to create collision.
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "D.md"), []byte("# D sub\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("# D root\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeEdges := countEdges(t, dbPath(vault))

	// Now update A.md to add an ambiguous link to D (which has two candidates).
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n[[D]]\n#tagA\n#shared\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous link error, got: %v", err)
	}

	// DB should be unchanged.
	afterEdges := countEdges(t, dbPath(vault))
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestUpdateOrphanCleanup(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
		if e.targetName == "Missing" && e.targetType == "phantom" {
			hasEdge = true
		}
	}
	if !hasEdge {
		t.Error("edge A→Missing not found")
	}
}

func TestUpdateNewTag(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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

func TestUpdateDeletedFileExistingPhantom(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
		if e.targetName == "B" && e.targetType == "phantom" {
			hasPhantomB = true
		}
	}
	if !hasPhantomB {
		t.Error("A should link to phantom B after simultaneous update+delete")
	}
}

func TestUpdatePartialErrorNoChanges(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
		if e.targetName == "A" && e.targetType == "note" {
			hasNoteA = true
		}
	}
	if !hasNoteA {
		t.Error("B→A should resolve to note A.md after basename collision resolved")
	}
}

func TestUpdateIncomingEdgesPreserved(t *testing.T) {
	vault := copyVault(t, "vault_update")
	if err := Build(vault); err != nil {
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
