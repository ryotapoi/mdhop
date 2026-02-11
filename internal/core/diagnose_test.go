package core

import (
	"strings"
	"testing"
)

func setupVaultForDiagnose(t *testing.T, name string) string {
	t.Helper()
	vault := copyVaultForQuery(t, name)
	buildForQuery(t, vault)
	return vault
}

func TestDiagnose_BasenameConflicts(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_query_ambiguous_name")

	result, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vault_query_ambiguous_name has sub1/A.md and sub2/A.md (same basename)
	if len(result.BasenameConflicts) != 1 {
		t.Fatalf("basename_conflicts count = %d, want 1", len(result.BasenameConflicts))
	}

	conflict := result.BasenameConflicts[0]
	if !strings.EqualFold(conflict.Name, "A") {
		t.Errorf("conflict name = %q, want A (case-insensitive)", conflict.Name)
	}
	if len(conflict.Paths) != 2 {
		t.Fatalf("conflict paths count = %d, want 2", len(conflict.Paths))
	}
	if conflict.Paths[0] != "sub1/A.md" {
		t.Errorf("conflict paths[0] = %q, want sub1/A.md", conflict.Paths[0])
	}
	if conflict.Paths[1] != "sub2/A.md" {
		t.Errorf("conflict paths[1] = %q, want sub2/A.md", conflict.Paths[1])
	}
}

func TestDiagnose_Phantoms(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_build_phantom")

	result, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vault_build_phantom has NonExistent and Missing as phantoms
	if len(result.Phantoms) != 2 {
		t.Fatalf("phantoms count = %d, want 2", len(result.Phantoms))
	}
	if result.Phantoms[0] != "Missing" {
		t.Errorf("phantoms[0] = %q, want Missing", result.Phantoms[0])
	}
	if result.Phantoms[1] != "NonExistent" {
		t.Errorf("phantoms[1] = %q, want NonExistent", result.Phantoms[1])
	}
}

func TestDiagnose_Full(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_build_full")

	result, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vault_build_full has 2 phantoms (Missing, NonExistent) and no basename conflicts
	if len(result.BasenameConflicts) != 0 {
		t.Errorf("basename_conflicts count = %d, want 0", len(result.BasenameConflicts))
	}
	if len(result.Phantoms) != 2 {
		t.Fatalf("phantoms count = %d, want 2", len(result.Phantoms))
	}
	if result.Phantoms[0] != "Missing" {
		t.Errorf("phantoms[0] = %q, want Missing", result.Phantoms[0])
	}
	if result.Phantoms[1] != "NonExistent" {
		t.Errorf("phantoms[1] = %q, want NonExistent", result.Phantoms[1])
	}
}

func TestDiagnose_Empty(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_build_empty")

	result, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 0 {
		t.Errorf("basename_conflicts count = %d, want 0", len(result.BasenameConflicts))
	}
	if len(result.Phantoms) != 0 {
		t.Errorf("phantoms count = %d, want 0", len(result.Phantoms))
	}
}

func TestDiagnose_NoDB(t *testing.T) {
	vault := t.TempDir()

	_, err := Diagnose(vault, DiagnoseOptions{})
	if err == nil {
		t.Fatal("expected error for missing DB")
	}
	if got := err.Error(); got != "index not found: run 'mdhop build' first" {
		t.Errorf("error = %q, want index not found message", got)
	}
}

func TestDiagnose_FieldsFilter(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_build_full")

	// Request only phantoms
	result, err := Diagnose(vault, DiagnoseOptions{Fields: []string{"phantoms"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Phantoms) != 2 {
		t.Errorf("phantoms count = %d, want 2", len(result.Phantoms))
	}
	// basename_conflicts should be nil (not requested)
	if result.BasenameConflicts != nil {
		t.Errorf("basename_conflicts = %v, want nil (not requested)", result.BasenameConflicts)
	}
}

func TestDiagnose_UnknownField(t *testing.T) {
	vault := t.TempDir()

	_, err := Diagnose(vault, DiagnoseOptions{Fields: []string{"invalid"}})
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if got := err.Error(); got != "unknown diagnose field: invalid" {
		t.Errorf("error = %q, want unknown diagnose field message", got)
	}
}
