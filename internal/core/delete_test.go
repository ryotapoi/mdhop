package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a real Missing.md file, rebuild, then delete it.
	missingPath := filepath.Join(vault, "Missing.md")
	if err := os.WriteFile(missingPath, []byte("# Missing\n\nNow I exist.\n"), 0o644); err != nil {
		t.Fatalf("write Missing.md: %v", err)
	}
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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

	// File should be gone from disk.
	if _, err := os.Stat(filepath.Join(vault, "C.md")); !os.IsNotExist(err) {
		t.Error("C.md should not exist on disk after RemoveFiles")
	}
}

func TestDeleteRemoveFiles_AlreadyRemoved(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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

	// File should be gone from disk.
	if _, err := os.Stat(filepath.Join(vault, "B.md")); !os.IsNotExist(err) {
		t.Error("B.md should not exist on disk after RemoveFiles")
	}
}

func TestDeleteRemoveFiles_VaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_delete")
	if err := Build(vault); err != nil {
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

	// File outside vault should still exist.
	if _, err := os.Stat(evilPath); err != nil {
		t.Error("evil.md outside vault should not be deleted")
	}
}

// --- ListDirNotes tests ---

func TestListDirNotes_Basic(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths, err := ListDirNotes(vault, "sub")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}
	sort.Strings(paths)
	want := []string{"sub/A.md", "sub/B.md", "sub/inner/C.md"}
	if len(paths) != len(want) {
		t.Fatalf("got %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %s, want %s", i, paths[i], want[i])
		}
	}
}

func TestListDirNotes_Nested(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths, err := ListDirNotes(vault, "sub/inner")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}
	if len(paths) != 1 || paths[0] != "sub/inner/C.md" {
		t.Errorf("got %v, want [sub/inner/C.md]", paths)
	}
}

func TestListDirNotes_NoMatch(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths, err := ListDirNotes(vault, "nonexist")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("got %v, want empty", paths)
	}
}

func TestListDirNotes_SpecialChars(t *testing.T) {
	// Test that _ and % in directory names are properly escaped for LIKE.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "dir_100%"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "dir_100%", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create a dir that would match without escaping: "dirX100Y" matches "dir_100%"
	if err := os.MkdirAll(filepath.Join(vault, "dirX100Y"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "dirX100Y", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths, err := ListDirNotes(vault, "dir_100%")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}
	if len(paths) != 1 || paths[0] != "dir_100%/A.md" {
		t.Errorf("got %v, want [dir_100%%/A.md]", paths)
	}
}

// --- Directory delete integration tests ---

func TestDelete_DirExpansion(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Get files under sub/ for deletion.
	notes, err := ListDirNotes(vault, "sub")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}

	// Delete with --rm.
	result, err := Delete(vault, DeleteOptions{Files: notes, RemoveFiles: true})
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

	// Files should be gone from disk.
	for _, n := range notes {
		if _, err := os.Stat(filepath.Join(vault, n)); !os.IsNotExist(err) {
			t.Errorf("%s should not exist on disk", n)
		}
	}
}

func TestDelete_DirEmpty(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths, err := ListDirNotes(vault, "nonexist")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty paths for nonexistent directory")
	}
}

func TestDelete_DirNoRm(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	notes, err := ListDirNotes(vault, "sub")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}

	// Remove files from disk first.
	for _, n := range notes {
		if err := os.Remove(filepath.Join(vault, n)); err != nil {
			t.Fatalf("remove %s: %v", n, err)
		}
	}

	// Delete without --rm.
	result, err := Delete(vault, DeleteOptions{Files: notes, RemoveFiles: false})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(result.Phantomed)+len(result.Deleted) != 3 {
		t.Errorf("expected 3 total operations, got Phantomed=%v Deleted=%v", result.Phantomed, result.Deleted)
	}
}

func TestDelete_DirRm_CleanupEmptyDirs(t *testing.T) {
	vault := copyVault(t, "vault_delete_dir")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	notes, err := ListDirNotes(vault, "sub")
	if err != nil {
		t.Fatalf("ListDirNotes: %v", err)
	}

	// Delete with --rm.
	_, err = Delete(vault, DeleteOptions{Files: notes, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Cleanup empty dirs.
	var allPaths []string
	allPaths = append(allPaths, notes...)
	if err := CleanupEmptyDirs(vault, allPaths); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// sub/inner/ should be gone (was emptied).
	if _, err := os.Stat(filepath.Join(vault, "sub", "inner")); !os.IsNotExist(err) {
		t.Error("sub/inner/ should be cleaned up")
	}
	// sub/ should be gone too (all files deleted).
	if _, err := os.Stat(filepath.Join(vault, "sub")); !os.IsNotExist(err) {
		t.Error("sub/ should be cleaned up")
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

	// Vault root should still exist.
	if _, err := os.Stat(vault); os.IsNotExist(err) {
		t.Error("vault root should not be removed")
	}
}

// --- HasNonMDFiles tests ---

func TestHasNonMDFiles_NoNonMD(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := HasNonMDFiles(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if found != "" {
		t.Errorf("expected no non-.md files, got: %s", found)
	}
}

func TestHasNonMDFiles_WithNonMD(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := HasNonMDFiles(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Error("expected non-.md file to be found")
	}
}

func TestHasNonMDFiles_HiddenIgnored(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Hidden directory with non-.md file inside.
	hiddenDir := filepath.Join(dir, ".obsidian")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := HasNonMDFiles(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if found != "" {
		t.Errorf("hidden files should be ignored, got: %s", found)
	}
}

func TestHasNonMDFiles_Nested(t *testing.T) {
	vault := t.TempDir()
	inner := filepath.Join(vault, "sub", "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "doc.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := HasNonMDFiles(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Error("expected nested non-.md file to be found")
	}
}
