package core

import (
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
