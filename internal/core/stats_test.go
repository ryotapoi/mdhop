package core

import (
	"testing"
)

func setupVaultForStats(t *testing.T, name string) string {
	t.Helper()
	vault := copyVaultForQuery(t, name)
	buildForQuery(t, vault)
	return vault
}

func TestStats_Full(t *testing.T) {
	vault := setupVaultForStats(t, "vault_build_full")

	result, err := Stats(vault, StatsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 notes: Index.md, Design.md, sub/Impl.md
	if result.NotesTotal != 3 {
		t.Errorf("notes_total = %d, want 3", result.NotesTotal)
	}
	// All 3 exist on disk
	if result.NotesExists != 3 {
		t.Errorf("notes_exists = %d, want 3", result.NotesExists)
	}
	// 6 tags: #project, #status, #status/active, #overview, #design, #code
	if result.TagsTotal != 6 {
		t.Errorf("tags_total = %d, want 6", result.TagsTotal)
	}
	// 2 phantoms: Missing, NonExistent
	if result.PhantomsTotal != 2 {
		t.Errorf("phantoms_total = %d, want 2", result.PhantomsTotal)
	}
	// 20 edges total (wikilinks + markdown links + tags + frontmatter)
	if result.EdgesTotal != 20 {
		t.Errorf("edges_total = %d, want 20", result.EdgesTotal)
	}
}

func TestStats_Empty(t *testing.T) {
	vault := setupVaultForStats(t, "vault_build_empty")

	result, err := Stats(vault, StatsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NotesTotal != 0 {
		t.Errorf("notes_total = %d, want 0", result.NotesTotal)
	}
	if result.NotesExists != 0 {
		t.Errorf("notes_exists = %d, want 0", result.NotesExists)
	}
	if result.EdgesTotal != 0 {
		t.Errorf("edges_total = %d, want 0", result.EdgesTotal)
	}
	if result.TagsTotal != 0 {
		t.Errorf("tags_total = %d, want 0", result.TagsTotal)
	}
	if result.PhantomsTotal != 0 {
		t.Errorf("phantoms_total = %d, want 0", result.PhantomsTotal)
	}
}

func TestStats_NoDB(t *testing.T) {
	vault := t.TempDir()

	_, err := Stats(vault, StatsOptions{})
	if err == nil {
		t.Fatal("expected error for missing DB")
	}
	if got := err.Error(); got != "index not found: run 'mdhop build' first" {
		t.Errorf("error = %q, want index not found message", got)
	}
}

func TestStats_FieldsFilter(t *testing.T) {
	vault := setupVaultForStats(t, "vault_build_full")

	result, err := Stats(vault, StatsOptions{Fields: []string{"notes_total", "tags_total"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NotesTotal != 3 {
		t.Errorf("notes_total = %d, want 3", result.NotesTotal)
	}
	if result.TagsTotal != 6 {
		t.Errorf("tags_total = %d, want 6", result.TagsTotal)
	}
	// Unrequested fields should remain zero.
	if result.NotesExists != 0 {
		t.Errorf("notes_exists = %d, want 0 (not requested)", result.NotesExists)
	}
	if result.EdgesTotal != 0 {
		t.Errorf("edges_total = %d, want 0 (not requested)", result.EdgesTotal)
	}
	if result.PhantomsTotal != 0 {
		t.Errorf("phantoms_total = %d, want 0 (not requested)", result.PhantomsTotal)
	}
}

func TestStats_UnknownField(t *testing.T) {
	vault := t.TempDir()

	_, err := Stats(vault, StatsOptions{Fields: []string{"invalid"}})
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if got := err.Error(); got != "unknown stats field: invalid" {
		t.Errorf("error = %q, want unknown stats field message", got)
	}
}

func TestStats_Tags(t *testing.T) {
	vault := setupVaultForStats(t, "vault_build_tags")

	result, err := Stats(vault, StatsOptions{Fields: []string{"tags_total"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vault_build_tags has:
	// A.md: frontmatter [fm_tag, nested/deep/tag], inline #simple, #parent/child
	// B.md: inline #simple
	// Expanded tags: #fm_tag, #nested, #nested/deep, #nested/deep/tag, #simple, #parent, #parent/child = 7
	if result.TagsTotal != 7 {
		t.Errorf("tags_total = %d, want 7", result.TagsTotal)
	}
}

func TestStats_Phantoms(t *testing.T) {
	vault := setupVaultForStats(t, "vault_build_phantom")

	result, err := Stats(vault, StatsOptions{Fields: []string{"phantoms_total"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vault_build_phantom: NonExistent and Missing are phantoms.
	if result.PhantomsTotal != 2 {
		t.Errorf("phantoms_total = %d, want 2", result.PhantomsTotal)
	}
}
