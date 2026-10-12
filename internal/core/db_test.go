package core

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestListDirNodesByType_LiteralPrefix(t *testing.T) {
	db := newTestDB(t)
	for _, node := range []struct {
		path   string
		typ    NodeType
		exists int
	}{
		{"docs/a.md", NodeTypeNote, 1},
		{"docs/sub/b.md", NodeTypeNote, 1},
		{"Docs/c.md", NodeTypeNote, 1},
		{"docs-extra/d.md", NodeTypeNote, 1},
		{"other/docs/e.md", NodeTypeNote, 1},
		{"docs", NodeTypeNote, 1},
		{"docs/missing.md", NodeTypeNote, 0},
		{"docs/image.png", NodeTypeAsset, 1},
		{"Docs/other.png", NodeTypeAsset, 1},
		{"docs/missing.png", NodeTypeAsset, 0},
		{"docs/phantom.md", NodeTypePhantom, 1},
		{"資料/é/a.md", NodeTypeNote, 1},
		{"資料/É/b.md", NodeTypeNote, 1},
		{"資料/é-extra/c.md", NodeTypeNote, 1},
		{"100%_done/a.md", NodeTypeNote, 1},
		{"100%_done/sub/b.md", NodeTypeNote, 1},
		{"100xxdone/c.md", NodeTypeNote, 1},
		{"100%xdone/d.md", NodeTypeNote, 1},
		{"100%_done-extra/e.md", NodeTypeNote, 1},
	} {
		if _, err := db.Exec(`INSERT INTO nodes (node_key, type, name, path, exists_flag) VALUES (?, ?, ?, ?, ?)`,
			string(node.typ)+":"+node.path, node.typ, node.path, node.path, node.exists); err != nil {
			t.Fatalf("insert %q: %v", node.path, err)
		}
	}

	for _, tc := range []struct {
		name, prefix string
		typ          NodeType
		want         []string
	}{
		{"lowercase notes", "docs", NodeTypeNote, []string{"docs/a.md", "docs/sub/b.md"}},
		{"uppercase notes", "Docs", NodeTypeNote, []string{"Docs/c.md"}},
		{"assets", "docs", NodeTypeAsset, []string{"docs/image.png"}},
		{"unicode", "資料/é", NodeTypeNote, []string{"資料/é/a.md"}},
		{"wildcard characters", "100%_done", NodeTypeNote, []string{"100%_done/a.md", "100%_done/sub/b.md"}},
		{"unregistered prefix", "DOCS", NodeTypeNote, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := listDirNodesByType(db, tc.prefix, tc.typ)
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("paths = %v, want %v", got, tc.want)
			}
		})
	}
}

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

type lastInsertIDZeroDB struct {
	*sql.DB
}

func (db lastInsertIDZeroDB) Exec(query string, args ...any) (sql.Result, error) {
	res, err := db.DB.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	return lastInsertIDZeroResult{res: res}, nil
}

type lastInsertIDZeroResult struct {
	res sql.Result
}

func (r lastInsertIDZeroResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r lastInsertIDZeroResult) RowsAffected() (int64, error) {
	return r.res.RowsAffected()
}

func TestUpsertNote_ConflictUpdate(t *testing.T) {
	db := newTestDB(t)

	id1, err := upsertNote(db, "docs/hello.md", "hello", 100, 0)
	if err != nil {
		t.Fatalf("first upsertNote: %v", err)
	}
	if id1 == 0 {
		t.Fatal("first upsertNote returned id 0")
	}

	id2, err := upsertNote(db, "docs/hello.md", "hello-updated", 200, 0)
	if err != nil {
		t.Fatalf("second upsertNote: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id %d, got %d", id1, id2)
	}

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

func TestUpsertNode_ConflictUpdateFallsBackWhenLastInsertIDIsZero(t *testing.T) {
	db := newTestDB(t)

	id1, err := upsertNote(db, "docs/hello.md", "hello", 100, 0)
	if err != nil {
		t.Fatalf("first upsertNote: %v", err)
	}

	id2, err := upsertNode(lastInsertIDZeroDB{DB: db}, noteKey("docs/hello.md"), NodeTypeNote, "hello-updated", "docs/hello.md", 200, intPtr(12))
	if err != nil {
		t.Fatalf("upsertNode with LastInsertId fallback: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected fallback to select existing id %d, got %d", id1, id2)
	}

	var name string
	var mtime int64
	var lines int
	if err := db.QueryRow("SELECT name, mtime, lines FROM nodes WHERE id = ?", id1).Scan(&name, &mtime, &lines); err != nil {
		t.Fatalf("scan updated node: %v", err)
	}
	if name != "hello-updated" {
		t.Fatalf("expected updated name, got %q", name)
	}
	if mtime != 200 {
		t.Fatalf("expected updated mtime 200, got %d", mtime)
	}
	if lines != 12 {
		t.Fatalf("expected updated lines 12, got %d", lines)
	}
}

func intPtr(v int) *int {
	return &v
}

func TestUpsertAsset_ConflictUpdate(t *testing.T) {
	db := newTestDB(t)

	id1, err := upsertAsset(db, "images/photo.png", "photo.png", 100)
	if err != nil {
		t.Fatalf("first upsertAsset: %v", err)
	}
	if id1 == 0 {
		t.Fatal("first upsertAsset returned id 0")
	}

	id2, err := upsertAsset(db, "images/photo.png", "photo-updated.png", 300)
	if err != nil {
		t.Fatalf("second upsertAsset: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id %d, got %d", id1, id2)
	}

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

func TestDeleteMetaByNode(t *testing.T) {
	db := newTestDB(t)

	node1, err := upsertNote(db, "a.md", "a", 100, 0)
	if err != nil {
		t.Fatalf("upsertNote 1: %v", err)
	}
	node2, err := upsertNote(db, "b.md", "b", 100, 0)
	if err != nil {
		t.Fatalf("upsertNote 2: %v", err)
	}

	if err := insertMeta(db, node1, "key1", "v1", 2, "v1", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, node1, "key2", "v2", 3, "v2", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, node2, "key1", "v1", 2, "v1", "string"); err != nil {
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

	nodeID, err := upsertNote(db, "docs/hello.md", "hello", 100, 0)
	if err != nil {
		t.Fatalf("upsertNote: %v", err)
	}

	if err := insertMeta(db, nodeID, "date", "2024-01-15", 2, "2024-01-15", "date"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "tags", "cli", 3, "cli", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "tags", "go", 4, "go", "string"); err != nil {
		t.Fatal(err)
	}
	if err := insertMeta(db, nodeID, "weight", "42", 5, "0000000042", "number"); err != nil {
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
	node2, err := upsertNote(db, "empty.md", "empty", 100, 0)
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
		"INSERT INTO meta (node_id, key, value, line, sort_value, value_type) VALUES (?, ?, ?, ?, NULL, NULL)",
		nodeID, "nulltest", "val", 6,
	); err != nil {
		t.Fatal(err)
	}
	rows3, err := queryMetaByNode(db, nodeID)
	if err != nil {
		t.Fatalf("queryMetaByNode after null insert: %v", err)
	}
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
