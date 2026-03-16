package core

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

func TestOpenDBChecked_NoDB(t *testing.T) {
	dir := t.TempDir()
	_, err := openDBChecked(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "index not found") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestOpenDBChecked_OK(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_build_basic", tmp); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}
	if err := Build(tmp); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	db, err := openDBChecked(tmp)
	if err != nil {
		t.Fatalf("openDBChecked failed: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count == 0 {
		t.Fatal("expected nodes in DB, got 0")
	}
}

// newTestDB creates a file-backed SQLite DB with the mdhop schema for unit tests.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbp := filepath.Join(t.TempDir(), "test.db")
	db, err := openDBAt(dbp)
	if err != nil {
		t.Fatalf("openDBAt failed: %v", err)
	}
	if err := initSchema(db); err != nil {
		db.Close()
		t.Fatalf("initSchema failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertNote_ConflictUpdate(t *testing.T) {
	db := newTestDB(t)

	// First insert.
	id1, err := upsertNote(db, "docs/hello.md", "hello", 100)
	if err != nil {
		t.Fatalf("first upsertNote: %v", err)
	}
	if id1 == 0 {
		t.Fatal("first upsertNote returned id 0")
	}

	// Second upsert with updated name and mtime — should return same id.
	id2, err := upsertNote(db, "docs/hello.md", "hello-updated", 200)
	if err != nil {
		t.Fatalf("second upsertNote: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id %d, got %d", id1, id2)
	}

	// Verify fields were updated.
	var typ, name string
	var mtime int64
	row := db.QueryRow("SELECT type, name, mtime FROM nodes WHERE id = ?", id1)
	if err := row.Scan(&typ, &name, &mtime); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if typ != "note" {
		t.Fatalf("expected type 'note', got %q", typ)
	}
	if name != "hello-updated" {
		t.Fatalf("expected name 'hello-updated', got %q", name)
	}
	if mtime != 200 {
		t.Fatalf("expected mtime 200, got %d", mtime)
	}
}

func TestUpsertAsset_ConflictUpdate(t *testing.T) {
	db := newTestDB(t)

	// First insert.
	id1, err := upsertAsset(db, "images/photo.png", "photo.png", 100)
	if err != nil {
		t.Fatalf("first upsertAsset: %v", err)
	}
	if id1 == 0 {
		t.Fatal("first upsertAsset returned id 0")
	}

	// Second upsert with updated name and mtime — should return same id.
	id2, err := upsertAsset(db, "images/photo.png", "photo-updated.png", 300)
	if err != nil {
		t.Fatalf("second upsertAsset: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id %d, got %d", id1, id2)
	}

	// Verify fields were updated.
	var typ, name string
	var mtime int64
	row := db.QueryRow("SELECT type, name, mtime FROM nodes WHERE id = ?", id1)
	if err := row.Scan(&typ, &name, &mtime); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if typ != "asset" {
		t.Fatalf("expected type 'asset', got %q", typ)
	}
	if name != "photo-updated.png" {
		t.Fatalf("expected name 'photo-updated.png', got %q", name)
	}
	if mtime != 300 {
		t.Fatalf("expected mtime 300, got %d", mtime)
	}
}
