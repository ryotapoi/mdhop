package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func countAssets(t *testing.T, dbp string) int {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='asset'").Scan(&count); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	return count
}

func getAssetNode(t *testing.T, dbp, path string) (id int64, name string) {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	key := assetKey(path)
	if err := db.QueryRow("SELECT id, name FROM nodes WHERE node_key = ?", key).Scan(&id, &name); err != nil {
		t.Fatalf("asset node %q not found: %v", path, err)
	}
	return
}

func assetNodeExists(t *testing.T, dbp, path string) bool {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	key := assetKey(path)
	var id int64
	err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key).Scan(&id)
	return err == nil
}

// --- Build tests ---

func TestBuildAssets_CreatesAssetNodes(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	dbp := dbPath(vault)
	got := countAssets(t, dbp)
	// image.png, doc.pdf, sub/photo.jpg, orphan.txt = 4
	if got != 4 {
		t.Fatalf("expected 4 assets, got %d", got)
	}
}

func TestBuildAssets_OrphanAssetRegistered(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if !assetNodeExists(t, dbPath(vault), "orphan.txt") {
		t.Fatal("orphan.txt should be registered as asset node")
	}
}

func TestBuildAssets_AssetNameIsFilename(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, name := getAssetNode(t, dbPath(vault), "image.png")
	if name != "image.png" {
		t.Fatalf("expected name %q, got %q", "image.png", name)
	}

	_, name = getAssetNode(t, dbPath(vault), "sub/photo.jpg")
	if name != "photo.jpg" {
		t.Fatalf("expected name %q, got %q", "photo.jpg", name)
	}
}

func TestBuildAssets_LinksResolveToAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	dbp := dbPath(vault)
	db := openTestDB(t, dbp)
	defer db.Close()

	// A.md links to image.png via ![[image.png]]
	assetID, _ := getAssetNode(t, dbp, "image.png")
	noteKey := noteKey("A.md")
	var noteID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", noteKey).Scan(&noteID); err != nil {
		t.Fatalf("note A.md not found: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE source_id = ? AND target_id = ?", noteID, assetID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("expected edge from A.md to image.png")
	}
}

func TestBuildAssets_AmbiguousAssetLink(t *testing.T) {
	vault := copyVault(t, "vault_asset_ambiguous")
	err := Build(vault)
	if err == nil {
		t.Fatal("expected build error for ambiguous asset link")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got: %v", err)
	}
}

func TestBuildAssets_HiddenFilesExcluded(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	// Create hidden file and .git directory.
	os.WriteFile(filepath.Join(vault, ".hidden"), []byte("hidden"), 0o644)
	os.MkdirAll(filepath.Join(vault, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(vault, ".git", "objects", "abc"), []byte("obj"), 0o644)

	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	dbp := dbPath(vault)
	if assetNodeExists(t, dbp, ".hidden") {
		t.Fatal(".hidden should not be registered")
	}
	if assetNodeExists(t, dbp, ".git/objects/abc") {
		t.Fatal(".git/objects/abc should not be registered")
	}
}

func TestBuildAssets_MdhopDirExcluded(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("first build: %v", err)
	}
	// Rebuild should not register .mdhop/index.sqlite as asset.
	if err := Build(vault); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if assetNodeExists(t, dbPath(vault), ".mdhop/index.sqlite") {
		t.Fatal(".mdhop/index.sqlite should not be registered as asset")
	}
}

// --- Stats test ---

func TestStatsAssetsTotal(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Stats(vault, StatsOptions{Fields: []string{"assets_total"}})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if result.AssetsTotal != 4 {
		t.Fatalf("expected assets_total=4, got %d", result.AssetsTotal)
	}
}

// --- Resolve test ---

func TestResolveAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Note: ! is not part of rawLink in parser. rawLink for ![[image.png]] is [[image.png]].
	result, err := Resolve(vault, "A.md", "[[image.png]]")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Type != "asset" {
		t.Fatalf("expected type=asset, got %q", result.Type)
	}
	if result.Path != "image.png" {
		t.Fatalf("expected path=image.png, got %q", result.Path)
	}
	if !result.Exists {
		t.Fatal("expected exists=true")
	}
}

func TestResolveAssetMarkdownLink(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Resolve(vault, "A.md", "[doc](doc.pdf)")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Type != "asset" {
		t.Fatalf("expected type=asset, got %q", result.Type)
	}
	if result.Path != "doc.pdf" {
		t.Fatalf("expected path=doc.pdf, got %q", result.Path)
	}
}

// --- Query test ---

func TestQueryAssetBacklinks(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Query(vault, EntrySpec{File: "image.png"}, QueryOptions{
		Fields: []string{"backlinks"},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.Entry.Type != "asset" {
		t.Fatalf("expected entry type=asset, got %q", result.Entry.Type)
	}
	if len(result.Backlinks) != 1 {
		t.Fatalf("expected 1 backlink, got %d", len(result.Backlinks))
	}
	if result.Backlinks[0].Path != "A.md" {
		t.Fatalf("expected backlink from A.md, got %q", result.Backlinks[0].Path)
	}
}

func TestQueryNoteOutgoingIncludesAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"outgoing"},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// A.md → image.png, doc.pdf
	foundAsset := false
	for _, n := range result.Outgoing {
		if n.Type == "asset" {
			foundAsset = true
			break
		}
	}
	if !foundAsset {
		t.Fatal("expected outgoing to include an asset node")
	}
}

func TestQueryAssetByName(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Query(vault, EntrySpec{Name: "image.png"}, QueryOptions{
		Fields: []string{"backlinks"},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.Entry.Type != "asset" {
		t.Fatalf("expected entry type=asset, got %q", result.Entry.Type)
	}
}

func TestQueryAssetHeadSkipped(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Query(vault, EntrySpec{File: "image.png"}, QueryOptions{
		Fields:      []string{"head"},
		IncludeHead: 5,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.Head != nil {
		t.Fatal("expected head=nil for asset")
	}
}

// --- Delete test ---

func TestDeleteAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Delete orphan.txt (no incoming edges → completely deleted).
	os.Remove(filepath.Join(vault, "orphan.txt"))
	result, err := Delete(vault, DeleteOptions{Files: []string{"orphan.txt"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "orphan.txt" {
		t.Fatalf("expected deleted=[orphan.txt], got %v", result.Deleted)
	}
	if assetNodeExists(t, dbPath(vault), "orphan.txt") {
		t.Fatal("orphan.txt should be completely deleted from DB")
	}
}

func TestDeleteAssetPhantomized(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Delete image.png (has incoming edge from A.md → phantomized).
	os.Remove(filepath.Join(vault, "image.png"))
	result, err := Delete(vault, DeleteOptions{Files: []string{"image.png"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Phantomed) != 1 || result.Phantomed[0] != "image.png" {
		t.Fatalf("expected phantomed=[image.png], got %v", result.Phantomed)
	}

	// Phantom should exist with name "image.png" (extension preserved).
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	pk := phantomKey("image.png")
	var id int64
	err = db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&id)
	if err != nil {
		t.Fatalf("phantom image.png not found: %v", err)
	}
}

func TestDeleteAssetWithRm(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Delete(vault, DeleteOptions{Files: []string{"orphan.txt"}, RemoveFiles: true})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(result.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.Deleted))
	}
	// File should be removed from disk.
	if _, err := os.Stat(filepath.Join(vault, "orphan.txt")); !os.IsNotExist(err) {
		t.Fatal("orphan.txt should be deleted from disk")
	}
}

// --- Delete directory with assets ---

func TestDeleteDirWithAssets(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Delete sub/ directory which contains B.md and photo.jpg.
	os.Remove(filepath.Join(vault, "sub", "B.md"))
	os.Remove(filepath.Join(vault, "sub", "photo.jpg"))

	// Collect notes and assets under sub/.
	notes, err := ListDirNotes(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	assets, err := ListDirAssets(vault, "sub")
	if err != nil {
		t.Fatal(err)
	}
	allFiles := append(notes, assets...)
	if len(allFiles) == 0 {
		t.Fatal("expected files under sub/")
	}

	result, err := Delete(vault, DeleteOptions{Files: allFiles})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// B.md should be phantomized (incoming link from sub/B.md → photo.jpg,
	// but no incoming to B.md itself from outside, so it should be fully deleted).
	// photo.jpg should be phantomized (incoming from sub/B.md, but B.md is also deleted → orphan cleanup).
	totalProcessed := len(result.Deleted) + len(result.Phantomed)
	if totalProcessed != len(allFiles) {
		t.Fatalf("expected %d processed files, got deleted=%d phantomed=%d", len(allFiles), len(result.Deleted), len(result.Phantomed))
	}
}

// --- Move test ---

func TestMoveAsset_BasenameUnchanged_NoRewrite(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move image.png → images/image.png (same basename, unique → no rewrite needed).
	os.MkdirAll(filepath.Join(vault, "images"), 0o755)
	os.Rename(filepath.Join(vault, "image.png"), filepath.Join(vault, "images", "image.png"))

	result, err := Move(vault, MoveOptions{From: "image.png", To: "images/image.png"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Basename link [[image.png]] still resolves correctly → no rewrite.
	if len(result.Rewritten) != 0 {
		t.Fatalf("expected no rewrites (basename unchanged, unique), got %v", result.Rewritten)
	}

	// Asset node should be updated.
	if !assetNodeExists(t, dbPath(vault), "images/image.png") {
		t.Fatal("asset node should exist at new path")
	}
	if assetNodeExists(t, dbPath(vault), "image.png") {
		t.Fatal("asset node should not exist at old path")
	}
}

func TestMoveAsset_BasenameChanged_Rewrite(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Rename image.png → photo.png (basename changes → must rewrite).
	os.Rename(filepath.Join(vault, "image.png"), filepath.Join(vault, "photo.png"))

	result, err := Move(vault, MoveOptions{From: "image.png", To: "photo.png"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	if len(result.Rewritten) == 0 {
		t.Fatal("expected rewritten links when basename changes")
	}
	foundRewrite := false
	for _, r := range result.Rewritten {
		if r.File == "A.md" && r.NewLink == "[[photo.png]]" {
			foundRewrite = true
		}
	}
	if !foundRewrite {
		t.Fatalf("expected rewrite [[image.png]] → [[photo.png]] in A.md, got %v", result.Rewritten)
	}

	// Read A.md and verify disk content.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "![[photo.png]]") {
		t.Fatalf("expected rewritten wikilink in A.md, got: %s", content)
	}
}

func TestMoveAsset_MarkdownLink_BasenameChanged(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Rename doc.pdf → manual.pdf (basename changes → must rewrite).
	os.Rename(filepath.Join(vault, "doc.pdf"), filepath.Join(vault, "manual.pdf"))

	_, err := Move(vault, MoveOptions{From: "doc.pdf", To: "manual.pdf"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "[doc](manual.pdf)") {
		t.Fatalf("expected rewritten markdown link in A.md, got: %s", content)
	}
}

// --- MoveDir with assets ---

func TestMoveDirWithAssets(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	os.MkdirAll(filepath.Join(vault, "archive"), 0o755)
	os.Rename(filepath.Join(vault, "sub"), filepath.Join(vault, "archive", "sub"))

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "archive/sub"})
	if err != nil {
		t.Fatalf("movedir: %v", err)
	}

	// Both B.md and photo.jpg should be in moved.
	if len(result.Moved) < 2 {
		t.Fatalf("expected at least 2 moved files, got %d", len(result.Moved))
	}
	foundNote := false
	foundAsset := false
	for _, m := range result.Moved {
		if m.From == "sub/B.md" && m.To == "archive/sub/B.md" {
			foundNote = true
		}
		if m.From == "sub/photo.jpg" && m.To == "archive/sub/photo.jpg" {
			foundAsset = true
		}
	}
	if !foundNote {
		t.Fatal("expected sub/B.md in moved files")
	}
	if !foundAsset {
		t.Fatal("expected sub/photo.jpg in moved files")
	}

	// Asset node should be updated.
	if !assetNodeExists(t, dbPath(vault), "archive/sub/photo.jpg") {
		t.Fatal("photo.jpg should be at new path")
	}
	if assetNodeExists(t, dbPath(vault), "sub/photo.jpg") {
		t.Fatal("photo.jpg should not be at old path")
	}
}

// --- Diagnose test ---

func TestDiagnoseAssetBasenameConflicts(t *testing.T) {
	// Build without the ambiguous link to test the diagnose command.
	vault := copyVault(t, "vault_asset_ambiguous")
	// Remove the link so build succeeds.
	os.WriteFile(filepath.Join(vault, "A.md"), []byte("no links\n"), 0o644)
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Diagnose(vault, DiagnoseOptions{Fields: []string{"asset_basename_conflicts"}})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.AssetBasenameConflicts) != 1 {
		t.Fatalf("expected 1 asset basename conflict, got %d", len(result.AssetBasenameConflicts))
	}
	if result.AssetBasenameConflicts[0].Name != "image.png" {
		t.Fatalf("expected conflict name=image.png, got %q", result.AssetBasenameConflicts[0].Name)
	}
}

// --- isAmbiguousBasenameLink tests for asset key space ---

func TestIsAmbiguousBasenameLink_AssetNotAmbiguousInNoteKeySpace(t *testing.T) {
	// "image.png" as target: note basenameCounts["image.png"] = 0, asset assetBasenameCounts["image.png"] = 1 → not ambiguous.
	rm := &resolveMaps{
		basenameCounts:      map[string]int{},
		pathSet:             map[string]string{},
		assetBasenameCounts: map[string]int{"image.png": 1},
		assetPathSet:        map[string]string{},
	}
	if isAmbiguousBasenameLink("image.png", rm) {
		t.Fatal("expected not ambiguous")
	}
}

func TestIsAmbiguousBasenameLink_NoteAndAssetSeparateKeySpaces(t *testing.T) {
	// "Note" as target: note basenameCounts["note"] = 1 → not ambiguous (note found).
	// Even if assetBasenameCounts["note"] = 2, note takes priority.
	rm := &resolveMaps{
		basenameCounts:      map[string]int{"note": 1},
		pathSet:             map[string]string{},
		assetBasenameCounts: map[string]int{"note": 2},
		assetPathSet:        map[string]string{"note": "sub/note"},
	}
	if isAmbiguousBasenameLink("Note", rm) {
		t.Fatal("expected not ambiguous (note key space has unique match)")
	}
}

// --- Phantom extension preservation (D10) ---

func TestPhantomPreservesNonMdExtension(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	// Add a note that links to a non-existent asset.
	os.WriteFile(filepath.Join(vault, "Linker.md"), []byte("![[missing.png]]\n"), 0o644)
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	db := openTestDB(t, dbPath(vault))
	defer db.Close()

	// Phantom should be "missing.png" (extension preserved).
	pk := phantomKey("missing.png")
	var id int64
	err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&id)
	if err == sql.ErrNoRows {
		t.Fatal("phantom missing.png not found")
	}
	if err != nil {
		t.Fatal(err)
	}
}

// --- ExcludeFilter asset support ---

func TestExcludeFilterAssetVia(t *testing.T) {
	ef := &ExcludeFilter{PathGlobs: []string{"images/*"}}
	info := NodeInfo{Type: "asset", Path: "images/photo.png"}
	if !ef.IsViaExcluded(info) {
		t.Fatal("expected asset to be excluded by path glob")
	}
}

// --- Add: asset link resolution (plan test case 12) ---

func TestAddAssetLinkResolvesToAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a new .md file that links to an existing asset.
	os.WriteFile(filepath.Join(vault, "Linker.md"), []byte("![[doc.pdf]]\n"), 0o644)
	result, err := Add(vault, AddOptions{Files: []string{"Linker.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result.Added))
	}

	// Edge should point to the asset node, not a phantom.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()

	assetID, _ := getAssetNode(t, dbPath(vault), "doc.pdf")
	var noteID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", noteKey("Linker.md")).Scan(&noteID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE source_id = ? AND target_id = ?", noteID, assetID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("expected edge from Linker.md to doc.pdf (asset), not phantom")
	}
}

func TestAddNewAssetOnDiskResolvedAsPhantom(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a new asset on disk (not in DB since build has already run).
	os.WriteFile(filepath.Join(vault, "new_asset.png"), []byte("NEW"), 0o644)
	// Add a new .md file that links to the new (unregistered) asset.
	os.WriteFile(filepath.Join(vault, "NewLinker.md"), []byte("![[new_asset.png]]\n"), 0o644)
	result, err := Add(vault, AddOptions{Files: []string{"NewLinker.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result.Added))
	}

	// The link should resolve to a phantom (D4: unregistered assets are phantom until next build).
	db := openTestDB(t, dbPath(vault))
	defer db.Close()

	pk := phantomKey("new_asset.png")
	var phantomID int64
	err = db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&phantomID)
	if err != nil {
		t.Fatalf("expected phantom for new_asset.png, got error: %v", err)
	}
}

// --- Update: asset orphan cleanup (plan test case 13) ---

func TestUpdateRemovesOrphanAsset(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// A.md links to image.png. Remove the link from A.md.
	os.WriteFile(filepath.Join(vault, "A.md"), []byte("no links anymore\n"), 0o644)
	_, err := Update(vault, UpdateOptions{Files: []string{"A.md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	// image.png was linked only from A.md. After removing the link and orphan cleanup,
	// image.png should still exist (orphan cleanup removes assets with no incoming edges).
	// Note: doc.pdf was also linked from A.md via [doc](doc.pdf), which is now removed.
	// Both image.png and doc.pdf should be cleaned up as orphan assets.
	if assetNodeExists(t, dbPath(vault), "image.png") {
		t.Fatal("image.png should be cleaned up as orphan asset")
	}
	if assetNodeExists(t, dbPath(vault), "doc.pdf") {
		t.Fatal("doc.pdf should be cleaned up as orphan asset")
	}
	// photo.jpg is still linked from sub/B.md, so it should remain.
	if !assetNodeExists(t, dbPath(vault), "sub/photo.jpg") {
		t.Fatal("sub/photo.jpg should still exist (linked from sub/B.md)")
	}
}

// --- Phase 2 integration tests ---

func TestQueryAssetTwoHop(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	// Add C.md that also links to image.png, enabling twohop via A.md.
	os.WriteFile(filepath.Join(vault, "C.md"), []byte("![[image.png]]\n"), 0o644)
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Query A.md with twohop. A.md → image.png ← C.md should appear.
	result, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:          []string{"twohop"},
		MaxTwoHop:       10,
		MaxViaPerTarget: 10,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// TwoHopEntry: Via is a single NodeInfo (the intermediate), Targets are the destinations.
	// A.md → image.png ← C.md: Via=image.png, Targets includes C.md.
	found := false
	for _, th := range result.TwoHop {
		if th.Via.Type == "asset" && th.Via.Path == "image.png" {
			for _, tgt := range th.Targets {
				if tgt.Path == "C.md" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("expected twohop to include C.md via image.png")
	}
}

func TestQueryAssetSnippet(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Query image.png with snippet. Snippet should come from A.md (the source note).
	result, err := Query(vault, EntrySpec{File: "image.png"}, QueryOptions{
		Fields:         []string{"snippet"},
		IncludeSnippet: 3,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Snippets) == 0 {
		t.Fatal("expected at least one snippet for asset backlink")
	}
	// Snippet source should be A.md.
	if result.Snippets[0].SourcePath != "A.md" {
		t.Fatalf("expected snippet source=A.md, got %q", result.Snippets[0].SourcePath)
	}
}

func TestQueryAssetExcludeIntegration(t *testing.T) {
	vault := copyVault(t, "vault_build_assets")
	// Add C.md that links to image.png.
	os.WriteFile(filepath.Join(vault, "C.md"), []byte("![[image.png]]\n"), 0o644)
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Query A.md with twohop, excluding image.png via exclude filter.
	ef, err := NewExcludeFilter(ExcludeConfig{}, []string{"image.png"}, nil)
	if err != nil {
		t.Fatalf("NewExcludeFilter: %v", err)
	}
	result, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields:          []string{"twohop"},
		MaxTwoHop:       10,
		MaxViaPerTarget: 10,
		Exclude:         ef,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// image.png via should be excluded, so C.md should not appear via image.png.
	for _, th := range result.TwoHop {
		if th.Via.Type == "asset" && th.Via.Path == "image.png" {
			t.Fatal("image.png via should be excluded by ExcludeFilter")
		}
	}
}
