package core

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDeleteNoDB(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	_, err := Delete(vault, DeleteOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "index not found") {
		t.Errorf("expected index not found error, got: %v", err)
	}
}

func TestDeleteUnregisteredFile(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Delete(vault, DeleteOptions{Files: []string{"NotExist.md"}})
	if err == nil || !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("expected file not registered error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestDeleteUnreferencedFile(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove from disk first (delete reflects file removal).
	if err := os.Remove(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("remove C.md: %v", err)
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"C.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}
	if len(result.Phantomed) != 0 {
		t.Errorf("Phantomed = %v, want []", result.Phantomed)
	}

	// C node should not exist.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if n.path == "C.md" {
			t.Error("C.md note should have been deleted")
		}
	}
}

func TestDeleteReferencedFileBecomesPhantom(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove B.md: %v", err)
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v, want []", result.Deleted)
	}

	// B should now be a phantom node.
	phantoms := queryNodes(t, dbPath(vault), "phantom")
	var foundB bool
	for _, p := range phantoms {
		if p.name == "B" {
			foundB = true
			if p.existsFlag != 0 {
				t.Errorf("phantom B should have exists_flag=0")
			}
			if p.path != "" {
				t.Errorf("phantom B should have empty path, got %s", p.path)
			}
		}
	}
	if !foundB {
		t.Error("phantom B not found after delete")
	}

	// B's outgoing edges should be deleted (B→A, B→#shared, B→#only_b).
	// Since B is now phantom, check there are no outgoing edges from B's new node.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var bNodeID int64
	err = db.QueryRow("SELECT id FROM nodes WHERE type='phantom' AND name='B'").Scan(&bNodeID)
	if err != nil {
		t.Fatalf("query phantom B: %v", err)
	}
	var outCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE source_id = ?", bNodeID).Scan(&outCount); err != nil {
		t.Fatalf("count outgoing: %v", err)
	}
	if outCount != 0 {
		t.Errorf("phantom B should have 0 outgoing edges, got %d", outCount)
	}

	// Phantom B must not retain the note's line count.
	var bLines sql.NullInt64
	if err := db.QueryRow("SELECT lines FROM nodes WHERE id = ?", bNodeID).Scan(&bLines); err != nil {
		t.Fatalf("query phantom B lines: %v", err)
	}
	if bLines.Valid {
		t.Errorf("phantom B lines should be NULL, got %d", bLines.Int64)
	}

	// A→B edge should still exist (pointing to phantom B).
	var inCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", bNodeID).Scan(&inCount); err != nil {
		t.Fatalf("count incoming: %v", err)
	}
	if inCount != 1 {
		t.Errorf("phantom B should have 1 incoming edge (from A), got %d", inCount)
	}
}

func TestDeleteOrphanTagCleanup(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Before delete: #only_b should exist.
	tagsBefore := queryNodes(t, dbPath(vault), "tag")
	var hasOnlyB bool
	for _, tag := range tagsBefore {
		if tag.name == "#only_b" {
			hasOnlyB = true
		}
	}
	if !hasOnlyB {
		t.Fatal("#only_b tag should exist before delete")
	}

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove B.md: %v", err)
	}

	if _, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// After delete: #only_b should be cleaned up (orphaned).
	tagsAfter := queryNodes(t, dbPath(vault), "tag")
	for _, tag := range tagsAfter {
		if tag.name == "#only_b" {
			t.Error("#only_b tag should have been cleaned up as orphan")
		}
	}

	// #shared should still exist (A still references it).
	var hasShared bool
	for _, tag := range tagsAfter {
		if tag.name == "#shared" {
			hasShared = true
		}
	}
	if !hasShared {
		t.Error("#shared tag should still exist (A references it)")
	}
}

func TestDeleteMultipleFiles(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove B.md: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("remove C.md: %v", err)
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md", "C.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// B should be phantomed (A references it), C should be deleted.
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}

	// Only A should remain as a note.
	notes := queryNodes(t, dbPath(vault), "note")
	if len(notes) != 1 {
		t.Errorf("expected 1 note remaining, got %d: %+v", len(notes), notes)
	}
	if notes[0].path != "A.md" {
		t.Errorf("remaining note = %s, want A.md", notes[0].path)
	}
}

func TestDeletePartialErrorNoChanges(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Delete(vault, DeleteOptions{Files: []string{"C.md", "NotExist.md"}})
	if err == nil || !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("expected file not registered error, got: %v", err)
	}

	// DB should be unchanged — validation happens before any mutations.
	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestDeleteReferencedFileBecomesNewPhantom(t *testing.T) {
	// Tests the in-place conversion path (no existing phantom with same name).
	vault := copyVault(t, "vault_build_full")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a real Missing.md file, rebuild, then delete it.
	missingPath := filepath.Join(vault, "Missing.md")
	if err := os.WriteFile(missingPath, []byte("# Missing\n\nNow I exist.\n"), 0o644); err != nil {
		t.Fatalf("write Missing.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// Verify no phantom "Missing" exists (build resolved it to note).
	phantomsBefore := queryNodes(t, dbPath(vault), "phantom")
	for _, p := range phantomsBefore {
		if p.name == "Missing" {
			t.Fatal("phantom Missing should not exist after rebuild with real file")
		}
	}

	// Remove from disk, then delete from index.
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove Missing.md: %v", err)
	}

	// Delete Missing.md — incoming references exist, so it becomes a new phantom.
	result, err := Delete(vault, DeleteOptions{Files: []string{"Missing.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "Missing.md" {
		t.Errorf("Phantomed = %v, want [Missing.md]", result.Phantomed)
	}

	// Verify phantom "Missing" exists now.
	phantomsAfter := queryNodes(t, dbPath(vault), "phantom")
	var hasPhantomMissing bool
	for _, p := range phantomsAfter {
		if p.name == "Missing" {
			hasPhantomMissing = true
		}
	}
	if !hasPhantomMissing {
		t.Error("phantom Missing should exist after delete")
	}

	// Verify incoming edges point to the phantom node.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var phantomID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE type='phantom' AND name='Missing'").Scan(&phantomID); err != nil {
		t.Fatalf("query phantom Missing: %v", err)
	}
	var inCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", phantomID).Scan(&inCount); err != nil {
		t.Fatalf("count incoming: %v", err)
	}
	if inCount == 0 {
		t.Error("phantom Missing should have incoming edges")
	}
}

func TestDeleteExistingPhantomEdgeReassignment(t *testing.T) {
	// Tests the edge reassignment path where a phantom with the same name
	// already exists when deleting a note.
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manually insert a phantom "B" into the DB to simulate pre-existing phantom.
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
	var removedNoteID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE type = 'note' AND path = 'B.md'").Scan(&removedNoteID); err != nil {
		db.Close()
		t.Fatalf("query B note id: %v", err)
	}
	db.Close()

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove B.md: %v", err)
	}

	// Delete B.md — A references B, so it should reassign edges to existing phantom.
	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v, want []", result.Deleted)
	}

	// The note node for B should be deleted (not converted).
	db2 := openTestDB(t, dbPath(vault))
	defer db2.Close()
	var noteCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='note' AND name='B'").Scan(&noteCount); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if noteCount != 0 {
		t.Error("note B should have been deleted (edges reassigned to existing phantom)")
	}

	// Incoming edges should point to the pre-existing phantom ID.
	var inCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", existingPhantomID).Scan(&inCount); err != nil {
		t.Fatalf("count incoming: %v", err)
	}
	if inCount != 1 {
		t.Errorf("existing phantom should have 1 incoming edge (from A), got %d", inCount)
	}
	var reassignedEdgeCount int
	if err := db2.QueryRow(`
		SELECT COUNT(*)
		FROM edges e
		JOIN nodes source ON source.id = e.source_id
		WHERE e.target_id = ? AND source.path = 'A.md' AND e.raw_link = '[[B]]' AND e.link_type = 'wikilink'
	`, existingPhantomID).Scan(&reassignedEdgeCount); err != nil {
		t.Fatalf("query reassigned edge: %v", err)
	}
	if reassignedEdgeCount != 1 {
		t.Errorf("expected A.md [[B]] wikilink to be reassigned to existing phantom, got %d", reassignedEdgeCount)
	}

	// The old note ID must no longer be referenced after its incoming edge was moved.
	var oldNodeEdges int
	if err := db2.QueryRow("SELECT COUNT(*) FROM edges WHERE source_id = ? OR target_id = ?", removedNoteID, removedNoteID).Scan(&oldNodeEdges); err != nil {
		t.Fatalf("count edges for removed note: %v", err)
	}
	if oldNodeEdges != 0 {
		t.Errorf("removed note ID %d still has %d edges", removedNoteID, oldNodeEdges)
	}

	// Only one phantom "B" should exist (not two).
	var phantomCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='phantom' AND node_key = ?", phantomKey).Scan(&phantomCount); err != nil {
		t.Fatalf("count phantoms: %v", err)
	}
	if phantomCount != 1 {
		t.Errorf("expected exactly 1 phantom B, got %d", phantomCount)
	}
}

func TestDeleteFileStillExistsOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	// Do NOT remove C.md from disk — delete should fail.
	_, err := Delete(vault, DeleteOptions{Files: []string{"C.md"}})
	if err == nil || !strings.Contains(err.Error(), "file still exists on disk") {
		t.Errorf("expected 'file still exists on disk' error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestDeleteDuplicateFileArgs(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.Remove(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("remove C.md: %v", err)
	}

	// Pass C.md twice — should succeed, processing only once.
	result, err := Delete(vault, DeleteOptions{Files: []string{"C.md", "C.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}
}

func TestDeleteRemoveFiles(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// C.md exists on disk — RemoveFiles should remove it.
	result, err := Delete(vault, DeleteOptions{Files: []string{"C.md"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}

	if _, err := os.Stat(filepath.Join(vault, "C.md")); !os.IsNotExist(err) {
		t.Error("C.md should not exist on disk after RemoveFiles")
	}
}

func TestDeleteRemoveFiles_AlreadyRemoved(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove file first — RemoveFiles should still succeed (idempotent).
	os.Remove(filepath.Join(vault, "C.md"))

	result, err := Delete(vault, DeleteOptions{Files: []string{"C.md"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "C.md" {
		t.Errorf("Deleted = %v, want [C.md]", result.Deleted)
	}
}

func TestDeleteRemoveFiles_Phantomize(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// B.md is referenced by A.md — should become phantom.
	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "B.md" {
		t.Errorf("Phantomed = %v, want [B.md]", result.Phantomed)
	}

	if _, err := os.Stat(filepath.Join(vault, "B.md")); !os.IsNotExist(err) {
		t.Error("B.md should not exist on disk after RemoveFiles")
	}
}

func TestDeleteRemoveFiles_VaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Inject a malicious path directly into the DB.
	db, err := openDBAt(dbPath(vault))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime) VALUES (?, 'note', 'evil', ?, 1, 0)`,
		noteKey("../evil.md"), "../evil.md")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	// Create the file outside vault so os.Remove would succeed without protection.
	evilPath := filepath.Join(vault, "..", "evil.md")
	if err := os.WriteFile(evilPath, []byte("evil"), 0o644); err != nil {
		t.Fatalf("write evil.md: %v", err)
	}
	defer os.Remove(evilPath)

	// RemoveFiles should reject vault-escaping path.
	_, err = Delete(vault, DeleteOptions{Files: []string{"../evil.md"}, RemoveFiles: true})
	if err == nil || !strings.Contains(err.Error(), "path escapes vault") {
		t.Errorf("expected 'path escapes vault' error, got: %v", err)
	}

	if _, err := os.Stat(evilPath); err != nil {
		t.Error("evil.md outside vault should not be deleted")
	}
}

func TestDeleteRemoveFiles_SymlinkAncestorEscapesVault(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	original := filepath.Join(root, "original")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	insideFile := filepath.Join(vault, "sub", "A.md")
	if err := os.WriteFile(insideFile, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.Rename(filepath.Join(vault, "sub"), original); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "A.md")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(vault, "sub")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := Delete(vault, DeleteOptions{Files: []string{"sub/"}, RemoveFiles: true})
	if err == nil || !strings.Contains(err.Error(), "path escapes vault") {
		t.Fatalf("delete error = %v, want path escapes vault", err)
	}
	for path, want := range map[string]string{
		outsideFile:                     "outside\n",
		filepath.Join(original, "A.md"): "original\n",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("file %s = %q, %v; want %q", path, got, err, want)
		}
	}
	var nodeType string
	var existsFlag int
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	if err := db.QueryRow("SELECT type, exists_flag FROM nodes WHERE path = ?", "sub/A.md").Scan(&nodeType, &existsFlag); err != nil {
		t.Fatalf("registered path missing: %v", err)
	}
	if nodeType != "note" || existsFlag != 1 {
		t.Errorf("registered path = (%s, %d), want (note, 1)", nodeType, existsFlag)
	}
}

// --- Directory delete tests ---

func TestDelete_DirExpansion(t *testing.T) {
	// Test that _ and % in directory names are properly escaped for LIKE.
	vault := t.TempDir()
	for dir, name := range map[string]string{"dir_100%": "A.md", "dirX100Y": "B.md"} {
		if err := os.MkdirAll(filepath.Join(vault, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vault, dir, name), []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"dir_100%"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "dir_100%/A.md" {
		t.Errorf("Deleted = %v, want [dir_100%%/A.md]", result.Deleted)
	}
	if _, err := os.Stat(filepath.Join(vault, "dirX100Y", "B.md")); err != nil {
		t.Errorf("similar directory should remain untouched: %v", err)
	}
}

func TestDelete_DirRm_ExpansionAndCleanup(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	lateAsset := filepath.Join(vault, "sub", "late.bin")
	lateMarkdown := filepath.Join(vault, "sub", "late.md")
	hiddenAsset := filepath.Join(vault, "sub", ".hidden", "keep.bin")
	for path, content := range map[string]string{
		lateAsset:    "asset",
		lateMarkdown: "markdown",
		hiddenAsset:  "hidden",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"sub"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// A and B are referenced by Root.md → phantomed.
	// inner/C.md is unreferenced → deleted.
	if len(result.Phantomed) != 2 {
		t.Errorf("Phantomed = %v, want 2 items", result.Phantomed)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "sub/inner/C.md" {
		t.Errorf("Deleted = %v, want [sub/inner/C.md]", result.Deleted)
	}

	for _, path := range []string{"sub/A.md", "sub/B.md", "sub/inner/C.md", "sub/late.bin"} {
		if _, err := os.Stat(filepath.Join(vault, path)); !os.IsNotExist(err) {
			t.Errorf("%s should be gone, got %v", path, err)
		}
	}
	for _, path := range []string{"sub/late.md", "sub/.hidden/keep.bin"} {
		if _, err := os.Stat(filepath.Join(vault, path)); err != nil {
			t.Errorf("%s should remain: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(vault, "sub", "inner")); !os.IsNotExist(err) {
		t.Errorf("empty nested directory should be removed, got %v", err)
	}
}

func TestDelete_DirEmpty(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Delete(vault, DeleteOptions{Files: []string{"nonexist/"}, RemoveFiles: true})
	if err == nil || !strings.Contains(err.Error(), "no files registered under directory: nonexist/") {
		t.Errorf("expected empty directory error, got: %v", err)
	}
}

func TestDelete_DirNoRm(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Remove files from disk first.
	for _, path := range []string{"sub/A.md", "sub/B.md", "sub/inner/C.md"} {
		if err := os.Remove(filepath.Join(vault, path)); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"sub/"}, RemoveFiles: false})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if got := len(result.Phantomed) + len(result.Deleted); got != 3 {
		t.Errorf("processed %d paths, want 3; Phantomed=%v Deleted=%v", got, result.Phantomed, result.Deleted)
	}
}

func TestDelete_DirRm_AssetCleanupErrorAfterDBUpdate(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	asset := filepath.Join(vault, "sub", "late.bin")
	if err := os.WriteFile(asset, []byte("asset"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	cleanupErr := errors.New("asset cleanup denied")
	originalRemove := deleteAssetRemove
	deleteAssetRemove = func(path string) error {
		if path == asset {
			return cleanupErr
		}
		return os.Remove(path)
	}
	t.Cleanup(func() { deleteAssetRemove = originalRemove })

	_, err := Delete(vault, DeleteOptions{Files: []string{"sub/"}, RemoveFiles: true})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("delete error = %v, want asset cleanup error", err)
	}
	for _, want := range []string{"post-delete cleanup failed", "registered files and database updates completed", asset} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("delete error = %q, missing %q", err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(vault, "sub", "A.md")); !os.IsNotExist(err) {
		t.Errorf("registered file remains after cleanup error: %v", err)
	}
	result, err := Stats(vault, StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if result.NotesTotal != 1 {
		t.Errorf("notes_total = %d, want 1 after deleting sub notes", result.NotesTotal)
	}
}

func TestDelete_DirRm_AssetWalkErrorAfterDBUpdate(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	walkPath := filepath.Join(vault, "sub", "late.bin")
	walkErr := errors.New("asset walk denied")
	originalWalk := deleteAssetWalk
	deleteAssetWalk = func(_ string, walkFn filepath.WalkFunc) error {
		return walkFn(walkPath, nil, walkErr)
	}
	t.Cleanup(func() { deleteAssetWalk = originalWalk })

	_, err := Delete(vault, DeleteOptions{Files: []string{"sub/"}, RemoveFiles: true})
	if !errors.Is(err, walkErr) {
		t.Fatalf("delete error = %v, want asset walk error", err)
	}
	for _, want := range []string{"post-delete cleanup failed", "registered files and database updates completed", walkPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("delete error = %q, missing %q", err, want)
		}
	}
	result, statErr := Stats(vault, StatsOptions{Fields: []string{"notes_total"}})
	if statErr != nil {
		t.Fatalf("stats: %v", statErr)
	}
	if result.NotesTotal != 1 {
		t.Errorf("notes_total = %d, want 1 after deleting sub notes", result.NotesTotal)
	}
}

func TestDelete_DirRm_EmptyDirCleanupErrorAfterDBUpdate(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	cleanupErr := errors.New("empty directory cleanup denied")
	originalCleanup := deleteEmptyDirs
	deleteEmptyDirs = func(_ string, _ []string) error { return cleanupErr }
	t.Cleanup(func() { deleteEmptyDirs = originalCleanup })

	_, err := Delete(vault, DeleteOptions{Files: []string{"sub/"}, RemoveFiles: true})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("delete error = %v, want empty directory cleanup error", err)
	}
	for _, want := range []string{"post-delete cleanup failed", "registered files and database updates completed", cleanupErr.Error()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("delete error = %q, missing %q", err, want)
		}
	}
	result, statErr := Stats(vault, StatsOptions{Fields: []string{"notes_total"}})
	if statErr != nil {
		t.Fatalf("stats: %v", statErr)
	}
	if result.NotesTotal != 1 {
		t.Errorf("notes_total = %d, want 1 after deleting sub notes", result.NotesTotal)
	}
}

// --- CleanupEmptyDirs tests ---

func TestCleanupEmptyDirs_Basic(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "a", "b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create and remove a file to leave empty dir.
	f := filepath.Join(dir, "X.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(f)

	if err := CleanupEmptyDirs(vault, []string{"a/b/X.md"}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(vault, "a", "b")); !os.IsNotExist(err) {
		t.Error("a/b should be removed")
	}
	if _, err := os.Stat(filepath.Join(vault, "a")); !os.IsNotExist(err) {
		t.Error("a should be removed")
	}
}

func TestCleanupEmptyDirs_NonEmpty(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Leave a non-.md file in the directory.
	if err := os.WriteFile(filepath.Join(dir, "image.png"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CleanupEmptyDirs(vault, []string{"a/X.md"}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Directory should NOT be removed (has remaining file).
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("a/ should NOT be removed (has image.png)")
	}
}

func TestCleanupEmptyDirs_UnexpectedError(t *testing.T) {
	vault := t.TempDir()
	blocked := filepath.Join(vault, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := CleanupEmptyDirs(vault, []string{"blocked/child/X.md"})
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("CleanupEmptyDirs error = %v, want ENOTDIR", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(vault, "blocked", "child")) {
		t.Errorf("cleanup error should name failed path: %v", err)
	}
}

func TestCleanupEmptyDirs_MissingDirectory(t *testing.T) {
	vault := t.TempDir()
	if err := CleanupEmptyDirs(vault, []string{"gone/X.md"}); err != nil {
		t.Fatalf("CleanupEmptyDirs missing directory: %v", err)
	}
}

func TestCleanupEmptyDirs_VaultRoot(t *testing.T) {
	vault := t.TempDir()
	// Create a file at vault root and remove it.
	f := filepath.Join(vault, "X.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(f)

	// Cleanup should not try to remove vault root.
	if err := CleanupEmptyDirs(vault, []string{"X.md"}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := os.Stat(vault); os.IsNotExist(err) {
		t.Error("vault root should not be removed")
	}
}

// --- Meta delete tests ---

func TestDeleteMetaCleanup(t *testing.T) {
	vault := t.TempDir()
	// A.md has frontmatter, B.md has no references to A.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: Hello\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if len(meta) != 1 {
		t.Fatalf("expected 1 meta row before delete, got %d", len(meta))
	}

	os.Remove(filepath.Join(vault, "A.md"))
	if _, err := Delete(vault, DeleteOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows after delete, got %d", c)
	}
}

func TestDeletePhantomizedMetaCleanup(t *testing.T) {
	vault := t.TempDir()
	// A.md links to B, B has frontmatter.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("---\ntitle: BTitle\npriority: 42\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	meta := queryMetaForPath(t, dbPath(vault), "B.md")
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta rows for B.md, got %d", len(meta))
	}

	os.Remove(filepath.Join(vault, "B.md"))
	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 {
		t.Errorf("expected B.md to be phantomized")
	}

	// All meta for B should be deleted (phantom has no frontmatter).
	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows after phantom conversion, got %d", c)
	}
}

func TestDeletePhantomizedExistingPhantom(t *testing.T) {
	vault := t.TempDir()
	// A.md links to B, B has frontmatter.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("---\ntitle: BTitle\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Manually insert a phantom "B" to test the edge reassignment path.
	db := openTestDB(t, dbPath(vault))
	pk := "phantom:name:b"
	_, err := db.Exec(
		"INSERT INTO nodes (node_key, type, name, path, exists_flag) VALUES (?, 'phantom', 'B', NULL, 0)", pk)
	if err != nil {
		db.Close()
		t.Fatalf("insert phantom: %v", err)
	}
	db.Close()

	os.Remove(filepath.Join(vault, "B.md"))
	result, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 {
		t.Errorf("expected B.md to be phantomized")
	}

	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows after delete with existing phantom, got %d", c)
	}
}

func TestDeleteSameNameAddThenDeleteAgain(t *testing.T) {
	vault := t.TempDir()
	writeVaultFile(t, vault, "A.md", "[[B]]\n")
	writeVaultFile(t, vault, "B.md", "first\n")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove first B.md: %v", err)
	}
	firstDelete, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if len(firstDelete.Phantomed) != 1 {
		t.Fatalf("first delete should phantomize B.md, got: %+v", firstDelete)
	}

	writeVaultFile(t, vault, "B.md", "second\n")
	added, err := Add(vault, AddOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("add second B.md: %v", err)
	}
	if len(added.Promoted) != 1 || added.Promoted[0] != "B.md" {
		t.Fatalf("add should promote B.md phantom, got: %+v", added)
	}

	if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
		t.Fatalf("remove second B.md: %v", err)
	}
	secondDelete, err := Delete(vault, DeleteOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if len(secondDelete.Phantomed) != 1 {
		t.Fatalf("second delete should phantomize B.md again, got: %+v", secondDelete)
	}

	phantoms := queryNodes(t, dbPath(vault), "phantom")
	var bPhantoms int
	for _, n := range phantoms {
		if n.nodeKey == "phantom:name:b" {
			bPhantoms++
		}
	}
	if bPhantoms != 1 {
		t.Fatalf("expected exactly one B phantom after re-delete, got %d: %+v", bPhantoms, phantoms)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	if len(edges) != 1 || edges[0].targetType != NodeTypePhantom || edges[0].targetName != "B" {
		t.Fatalf("A.md should point to the re-created B phantom, got: %+v", edges)
	}
}
