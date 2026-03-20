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
	if _, err := Build(tmp); err != nil {
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

func TestInsertMeta(t *testing.T) {
	db := newTestDB(t)

	nodeID, err := upsertNote(db, "docs/hello.md", "hello", 100)
	if err != nil {
		t.Fatalf("upsertNote: %v", err)
	}

	// Insert two rows with the same key (list values).
	if err := insertMeta(db, nodeID, "tags", "go", "go", "string"); err != nil {
		t.Fatalf("insertMeta 1: %v", err)
	}
	if err := insertMeta(db, nodeID, "tags", "cli", "cli", "string"); err != nil {
		t.Fatalf("insertMeta 2: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM meta WHERE node_id = ?", nodeID).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 meta rows, got %d", count)
	}
}

func TestDeleteMetaByNode(t *testing.T) {
	db := newTestDB(t)

	node1, err := upsertNote(db, "a.md", "a", 100)
	if err != nil {
		t.Fatalf("upsertNote 1: %v", err)
	}
	node2, err := upsertNote(db, "b.md", "b", 100)
	if err != nil {
		t.Fatalf("upsertNote 2: %v", err)
	}

	if err := insertMeta(db, node1, "key1", "v1", "v1", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, node1, "key2", "v2", "v2", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, node2, "key1", "v1", "v1", "string"); err != nil {
		t.Fatal(err)
	}

	if err := deleteMetaByNode(db, node1); err != nil {
		t.Fatalf("deleteMetaByNode: %v", err)
	}

	var count1, count2 int
	if err := db.QueryRow("SELECT COUNT(*) FROM meta WHERE node_id = ?", node1).Scan(&count1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM meta WHERE node_id = ?", node2).Scan(&count2); err != nil {
		t.Fatal(err)
	}
	if count1 != 0 {
		t.Fatalf("expected 0 meta rows for node1, got %d", count1)
	}
	if count2 != 1 {
		t.Fatalf("expected 1 meta row for node2, got %d", count2)
	}
}

func TestQueryMetaByNode(t *testing.T) {
	db := newTestDB(t)

	nodeID, err := upsertNote(db, "docs/hello.md", "hello", 100)
	if err != nil {
		t.Fatalf("upsertNote: %v", err)
	}

	// Insert various meta rows.
	if err := insertMeta(db, nodeID, "date", "2024-01-15", "2024-01-15", "date"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "tags", "cli", "cli", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "tags", "go", "go", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "weight", "42", "0000000042", "number"); err != nil {
		t.Fatal(err)
	}

	rows, err := queryMetaByNode(db, nodeID)
	if err != nil {
		t.Fatalf("queryMetaByNode: %v", err)
	}

	// Expect ORDER BY key, value: date, tags/cli, tags/go, weight.
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}

	expected := []MetaRow{
		{Key: "date", Value: "2024-01-15", SortValue: "2024-01-15", ValueType: "date"},
		{Key: "tags", Value: "cli", SortValue: "cli", ValueType: "string"},
		{Key: "tags", Value: "go", SortValue: "go", ValueType: "string"},
		{Key: "weight", Value: "42", SortValue: "0000000042", ValueType: "number"},
	}
	for i, exp := range expected {
		got := rows[i]
		if got != exp {
			t.Errorf("row %d: expected %+v, got %+v", i, exp, got)
		}
	}

	// Empty result for node with no meta.
	node2, err := upsertNote(db, "empty.md", "empty", 100)
	if err != nil {
		t.Fatal(err)
	}
	rows2, err := queryMetaByNode(db, node2)
	if err != nil {
		t.Fatalf("queryMetaByNode empty: %v", err)
	}
	if len(rows2) != 0 {
		t.Fatalf("expected 0 rows for empty node, got %d", len(rows2))
	}

	// COALESCE: NULL sort_value and value_type become empty strings.
	if _, err := db.Exec(
		"INSERT INTO meta (node_id, key, value, sort_value, value_type) VALUES (?, ?, ?, NULL, NULL)",
		nodeID, "nulltest", "val",
	); err != nil {
		t.Fatal(err)
	}
	rows3, err := queryMetaByNode(db, nodeID)
	if err != nil {
		t.Fatalf("queryMetaByNode after null insert: %v", err)
	}
	// Find the nulltest row.
	var found bool
	for _, r := range rows3 {
		if r.Key == "nulltest" {
			found = true
			if r.SortValue != "" {
				t.Errorf("expected empty SortValue for NULL, got %q", r.SortValue)
			}
			if r.ValueType != "" {
				t.Errorf("expected empty ValueType for NULL, got %q", r.ValueType)
			}
		}
	}
	if !found {
		t.Error("nulltest row not found in query results")
	}
}
