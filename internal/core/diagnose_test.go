package core

import (
	"os"
	"path/filepath"
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

func TestDiagnose_BrokenAnchors(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_anchor_check")

	// Without "anchors" in fields, broken anchors must not be computed.
	def, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.BrokenAnchors != nil {
		t.Errorf("BrokenAnchors should be nil when not requested, got %v", def.BrokenAnchors)
	}

	// With "anchors", broken heading fragments are reported.
	result, err := Diagnose(vault, DiagnoseOptions{Fields: []string{"anchors"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]string) // fragment -> target path
	for _, a := range result.BrokenAnchors {
		if a.SourcePath != "source.md" {
			t.Errorf("unexpected source path %q", a.SourcePath)
		}
		got[a.Fragment] = a.TargetPath
	}

	// "Missing" (wikilink) and "Nonexistent" (markdown) are broken.
	// "Setup" and "Usage Tips" resolve; "^abc123" (block ref) is skipped.
	if len(result.BrokenAnchors) != 2 {
		t.Fatalf("broken anchors = %d (%+v), want 2", len(result.BrokenAnchors), result.BrokenAnchors)
	}
	if got["Missing"] != "target.md" {
		t.Errorf("expected broken fragment Missing → target.md, got %v", got)
	}
	if got["Nonexistent"] != "target.md" {
		t.Errorf("expected broken fragment Nonexistent → target.md, got %v", got)
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

func TestDiagnose_PathFilter_NoFilterReportsAll(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	result, err := Diagnose(vault, DiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 1 {
		t.Fatalf("basename_conflicts count = %d, want 1", len(result.BasenameConflicts))
	}
	if got := result.BasenameConflicts[0].Paths; len(got) != 2 || got[0] != "Conflict.md" || got[1] != "docs/Conflict.md" {
		t.Errorf("conflict paths = %v, want [Conflict.md docs/Conflict.md]", got)
	}
	if len(result.AssetBasenameConflicts) != 1 {
		t.Fatalf("asset_basename_conflicts count = %d, want 1", len(result.AssetBasenameConflicts))
	}
	if got := result.AssetBasenameConflicts[0].Paths; len(got) != 2 || got[0] != "img/logo.png" || got[1] != "pics/logo.png" {
		t.Errorf("asset conflict paths = %v, want [img/logo.png pics/logo.png]", got)
	}
	if len(result.Phantoms) != 2 || result.Phantoms[0] != "MissingDoc" || result.Phantoms[1] != "MissingOther" {
		t.Errorf("phantoms = %v, want [MissingDoc MissingOther]", result.Phantoms)
	}
}

func TestDiagnose_PathFilter_BasenameLinkSource(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	// docs/a.md links [[Conflict]] (basename) and [[MissingDoc]].
	result, err := Diagnose(vault, DiagnoseOptions{Path: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 1 {
		t.Fatalf("basename_conflicts count = %d, want 1", len(result.BasenameConflicts))
	}
	if got := result.BasenameConflicts[0].Name; !strings.EqualFold(got, "Conflict") {
		t.Errorf("conflict name = %q, want Conflict", got)
	}
	// No basename links to assets from docs/*.
	if len(result.AssetBasenameConflicts) != 0 {
		t.Errorf("asset_basename_conflicts count = %d, want 0", len(result.AssetBasenameConflicts))
	}
	if len(result.Phantoms) != 1 || result.Phantoms[0] != "MissingDoc" {
		t.Errorf("phantoms = %v, want [MissingDoc]", result.Phantoms)
	}
}

func TestDiagnose_PathFilter_PathLinkIsNotConflictRisk(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	// other/b.md links [[docs/Conflict]] (path link, no resolution risk) and [[MissingOther]].
	result, err := Diagnose(vault, DiagnoseOptions{Path: []string{"other/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 0 {
		t.Errorf("basename_conflicts count = %d, want 0", len(result.BasenameConflicts))
	}
	if len(result.Phantoms) != 1 || result.Phantoms[0] != "MissingOther" {
		t.Errorf("phantoms = %v, want [MissingOther]", result.Phantoms)
	}
}

func TestDiagnose_PathFilter_Exclude(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	result, err := Diagnose(vault, DiagnoseOptions{Exclude: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 0 {
		t.Errorf("basename_conflicts count = %d, want 0", len(result.BasenameConflicts))
	}
	if len(result.Phantoms) != 1 || result.Phantoms[0] != "MissingOther" {
		t.Errorf("phantoms = %v, want [MissingOther]", result.Phantoms)
	}
}

func TestDiagnose_PathFilter_IncludeAndExclude(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	result, err := Diagnose(vault, DiagnoseOptions{Path: []string{"*"}, Exclude: []string{"other/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.BasenameConflicts) != 1 {
		t.Errorf("basename_conflicts count = %d, want 1", len(result.BasenameConflicts))
	}
	if len(result.Phantoms) != 1 || result.Phantoms[0] != "MissingDoc" {
		t.Errorf("phantoms = %v, want [MissingDoc]", result.Phantoms)
	}
}

func TestDiagnose_PathFilter_InvalidGlob(t *testing.T) {
	vault := setupVaultForDiagnose(t, "vault_diagnose_path")

	_, err := Diagnose(vault, DiagnoseOptions{Path: []string{"[abc]/*"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported glob pattern") {
		t.Errorf("error = %v, want unsupported glob pattern", err)
	}

	_, err = Diagnose(vault, DiagnoseOptions{Exclude: []string{"[abc]/*"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported glob pattern") {
		t.Errorf("error = %v, want unsupported glob pattern", err)
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

func TestDiagnose_PathFilter_FrontmatterPathBasenameSource(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_diagnose_path")
	// link_keys raw basename value referencing the Conflict group
	// (root-priority resolves it to Conflict.md, so build succeeds).
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("meta:\n  link_keys:\n    - related\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Mkdir(filepath.Join(vault, "topics"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "topics", "ref.md"), []byte("---\nrelated: Conflict\n---\n\n# Ref\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	buildForQuery(t, vault)

	result, err := Diagnose(vault, DiagnoseOptions{Path: []string{"topics/*"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BasenameConflicts) != 1 {
		t.Fatalf("basename_conflicts = %v, want the Conflict group referenced via frontmatter_path", result.BasenameConflicts)
	}
	if got := result.BasenameConflicts[0].Paths; len(got) != 2 || got[0] != "Conflict.md" || got[1] != "docs/Conflict.md" {
		t.Errorf("conflict paths = %v, want [Conflict.md docs/Conflict.md]", got)
	}
}
