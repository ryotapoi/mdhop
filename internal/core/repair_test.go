package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairBasic(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	// X has 1 candidate (existing/X.md) → 3 edges rewritten (A.md×2 + sub/B.md×1).
	// Y has 0 candidates → 1 edge rewritten (A.md).
	// M has 2 candidates → 1 edge skipped (A.md).
	if len(result.Rewritten) != 4 {
		t.Errorf("Rewritten count = %d, want 4", len(result.Rewritten))
		for _, r := range result.Rewritten {
			t.Logf("  %s: %s → %s", r.File, r.OldLink, r.NewLink)
		}
	}
	if len(result.Skipped) != 1 {
		t.Errorf("Skipped count = %d, want 1", len(result.Skipped))
		for _, s := range result.Skipped {
			t.Logf("  %s: %s (basename=%s, candidates=%v)", s.File, s.RawLink, s.Basename, s.Candidates)
		}
	}

	// Verify A.md disk content.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	got := string(content)

	// [[old/path/X]] → [[X]]
	if !strings.Contains(got, "[[X]]") {
		t.Errorf("A.md should contain [[X]], got:\n%s", got)
	}
	// [text](old/path/X.md) → [text](X.md)
	if !strings.Contains(got, "[text](X.md)") {
		t.Errorf("A.md should contain [text](X.md), got:\n%s", got)
	}
	// [[missing/Y]] → [[Y]]
	if !strings.Contains(got, "[[Y]]") {
		t.Errorf("A.md should contain [[Y]], got:\n%s", got)
	}
	// [[old/M]] should NOT be rewritten (multiple candidates).
	if !strings.Contains(got, "[[old/M]]") {
		t.Errorf("A.md should still contain [[old/M]], got:\n%s", got)
	}

	// Verify sub/B.md disk content: subpath preserved.
	contentB, err := os.ReadFile(filepath.Join(vault, "sub/B.md"))
	if err != nil {
		t.Fatalf("read sub/B.md: %v", err)
	}
	gotB := string(contentB)
	// [[broken/path/X#Heading]] → [[X#Heading]]
	if !strings.Contains(gotB, "[[X#Heading]]") {
		t.Errorf("sub/B.md should contain [[X#Heading]], got:\n%s", gotB)
	}
}

func TestRepairDryRun(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	// Read original content.
	origA, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}

	result, err := Repair(vault, RepairOptions{DryRun: true})
	if err != nil {
		t.Fatalf("repair dry-run: %v", err)
	}

	// Output should be the same as non-dry-run.
	if len(result.Rewritten) != 4 {
		t.Errorf("Rewritten count = %d, want 4", len(result.Rewritten))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("Skipped count = %d, want 1", len(result.Skipped))
	}

	// Disk should be unchanged.
	newA, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if string(newA) != string(origA) {
		t.Error("A.md was modified during dry-run")
	}
}

func TestRepairNoBrokenLinks(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(result.Rewritten) != 0 {
		t.Errorf("Rewritten count = %d, want 0", len(result.Rewritten))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped count = %d, want 0", len(result.Skipped))
	}
}

func TestRepairSkippedCandidates(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Fatalf("Skipped count = %d, want 1", len(result.Skipped))
	}

	s := result.Skipped[0]
	if s.File != "A.md" {
		t.Errorf("Skipped file = %q, want A.md", s.File)
	}
	if s.RawLink != "[[old/M]]" {
		t.Errorf("Skipped rawLink = %q, want [[old/M]]", s.RawLink)
	}
	if s.Basename != "M" {
		t.Errorf("Skipped basename = %q, want M", s.Basename)
	}
	if len(s.Candidates) != 2 {
		t.Errorf("Skipped candidates count = %d, want 2", len(s.Candidates))
	}
	// Candidates should be sorted.
	if len(s.Candidates) >= 2 {
		if s.Candidates[0] != "dir1/M.md" || s.Candidates[1] != "dir2/M.md" {
			t.Errorf("Skipped candidates = %v, want [dir1/M.md dir2/M.md]", s.Candidates)
		}
	}
}

func TestRepairBasenameLinksUntouched(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	// Add a file with only basename links to phantoms.
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("[[NoExist]]\n"), 0o644); err != nil {
		t.Fatalf("write D.md: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	// D.md's [[NoExist]] is a basename link → not a repair candidate.
	for _, r := range result.Rewritten {
		if r.File == "D.md" {
			t.Errorf("D.md should not be rewritten, got: %s → %s", r.OldLink, r.NewLink)
		}
	}

	// Verify D.md unchanged.
	content, err := os.ReadFile(filepath.Join(vault, "D.md"))
	if err != nil {
		t.Fatalf("read D.md: %v", err)
	}
	if !strings.Contains(string(content), "[[NoExist]]") {
		t.Errorf("D.md should still contain [[NoExist]], got: %s", content)
	}
}

func TestRepairInlineCodeIgnored(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	// Overwrite A.md to include inline code.
	aPath := filepath.Join(vault, "A.md")
	if err := os.WriteFile(aPath, []byte("[[old/path/X]]\n`[[old/path/X]]` should not change\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}

	_, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	content, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	got := string(content)

	// Inline code should remain.
	if !strings.Contains(got, "`[[old/path/X]]`") {
		t.Errorf("inline code was incorrectly rewritten, got:\n%s", got)
	}
	// Regular link should be rewritten.
	if !strings.Contains(got, "[[X]]") {
		t.Errorf("regular link was not rewritten, got:\n%s", got)
	}
}

func TestRepairExcludedFileUntouched(t *testing.T) {
	vault := copyVault(t, "vault_repair")

	// Create a file that exists on disk but is excluded by build config.
	excludeDir := filepath.Join(vault, "excluded")
	if err := os.MkdirAll(excludeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(excludeDir, "Z.md"), []byte("# Z\n"), 0o644); err != nil {
		t.Fatalf("write Z.md: %v", err)
	}

	// Create a file linking to the excluded path.
	if err := os.WriteFile(filepath.Join(vault, "E.md"), []byte("[[excluded/Z]]\n"), 0o644); err != nil {
		t.Fatalf("write E.md: %v", err)
	}

	// Create mdhop.yaml to exclude the directory.
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("build:\n  exclude_paths:\n    - \"excluded/*\"\n"), 0o644); err != nil {
		t.Fatalf("write mdhop.yaml: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	// [[excluded/Z]] points to a broken path, but excluded/Z.md exists on disk.
	// Repair should NOT rewrite this link.
	for _, r := range result.Rewritten {
		if r.OldLink == "[[excluded/Z]]" {
			t.Errorf("excluded file link should not be rewritten, got: %s → %s", r.OldLink, r.NewLink)
		}
	}

	content, err := os.ReadFile(filepath.Join(vault, "E.md"))
	if err != nil {
		t.Fatalf("read E.md: %v", err)
	}
	if !strings.Contains(string(content), "[[excluded/Z]]") {
		t.Errorf("E.md should still contain [[excluded/Z]], got: %s", content)
	}
}

func TestRepairVaultEscapeRelative(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[../../NoCandidate]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 1 {
		t.Fatalf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	r := result.Rewritten[0]
	if r.OldLink != "[[../../NoCandidate]]" || r.NewLink != "[[NoCandidate]]" {
		t.Errorf("got %s → %s, want [[../../NoCandidate]] → [[NoCandidate]]", r.OldLink, r.NewLink)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(content), "[[NoCandidate]]") {
		t.Errorf("A.md should contain [[NoCandidate]], got: %s", content)
	}
}

func TestRepairVaultEscapePathTraversal(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// sub/../../X = ../X → escapes vault when source is sub/A.md
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[[sub/../../NoCandidate2]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 1 {
		t.Fatalf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	r := result.Rewritten[0]
	if r.NewLink != "[[NoCandidate2]]" {
		t.Errorf("got new = %s, want [[NoCandidate2]]", r.NewLink)
	}
}

func TestRepairVaultEscapeAbsolutePrefix(t *testing.T) {
	vault := t.TempDir()
	// /../../outside.md → pathEscapesVault
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[text](/../../outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 1 {
		t.Fatalf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	r := result.Rewritten[0]
	// normalizeBasename strips .md, filepath.Base("../../outside") = "outside", rewriteRawLink("outside.md") → "outside.md"
	if r.NewLink != "[text](outside.md)" {
		t.Errorf("got new = %s, want [text](outside.md)", r.NewLink)
	}
}

func TestRepairVaultEscapeMultipleCandidates(t *testing.T) {
	vault := t.TempDir()
	// Create 2 candidates for "X" in vault.
	if err := os.MkdirAll(filepath.Join(vault, "d1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "d2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "d1", "X.md"), []byte("# X1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "d2", "X.md"), []byte("# X2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Escape link pointing to X with 2+ candidates → should still be basename-ified (escape resolution priority).
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[../../X]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	// Should be rewritten, NOT skipped (escape resolution takes priority).
	if len(result.Rewritten) != 1 {
		t.Errorf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped count = %d, want 0", len(result.Skipped))
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(content), "[[X]]") {
		t.Errorf("A.md should contain [[X]], got: %s", content)
	}
}

func TestRepairMixed(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "existing"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "existing", "Y.md"), []byte("# Y\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// A.md has broken path link + escape link.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[old/Y]]\n[[../../Escape]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 2 {
		for _, r := range result.Rewritten {
			t.Logf("  %s: %s → %s", r.File, r.OldLink, r.NewLink)
		}
		t.Fatalf("Rewritten count = %d, want 2", len(result.Rewritten))
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(content)
	if !strings.Contains(got, "[[Y]]") {
		t.Errorf("A.md should contain [[Y]], got: %s", got)
	}
	if !strings.Contains(got, "[[Escape]]") {
		t.Errorf("A.md should contain [[Escape]], got: %s", got)
	}
}

func TestRepairNonMdExtension(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[old/path/img.png]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 1 {
		t.Fatalf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	r := result.Rewritten[0]
	if r.NewLink != "[[img.png]]" {
		t.Errorf("got new = %s, want [[img.png]]", r.NewLink)
	}
}

func TestRepairDotInBasename(t *testing.T) {
	vault := t.TempDir()
	// Create a file with dot in basename.
	if err := os.WriteFile(filepath.Join(vault, "Note.v1.md"), []byte("# Note v1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[old/path/Note.v1]]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(result.Rewritten) != 1 {
		t.Fatalf("Rewritten count = %d, want 1", len(result.Rewritten))
	}
	r := result.Rewritten[0]
	// .v1 should NOT be stripped as extension.
	if r.NewLink != "[[Note.v1]]" {
		t.Errorf("got new = %s, want [[Note.v1]]", r.NewLink)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(content), "[[Note.v1]]") {
		t.Errorf("A.md should contain [[Note.v1]], got: %s", content)
	}
}
