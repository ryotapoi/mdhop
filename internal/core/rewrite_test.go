package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreBackupsPreservesPermission(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.md"
	fullPath := filepath.Join(dir, filePath)

	// Create file with 0o600.
	if err := os.WriteFile(fullPath, []byte("modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	backups := []rewriteBackup{
		{path: filePath, content: []byte("original\n"), perm: 0o600},
	}

	restoreBackups(dir, backups)

	// Verify content restored.
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original\n" {
		t.Errorf("content = %q, want %q", string(content), "original\n")
	}

	// Verify permission preserved.
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want %o", perm, 0o600)
	}
}

func TestApplyFileRewritesPreservesPermission(t *testing.T) {
	vault := t.TempDir()

	// Create a file with 0o600 containing a wikilink to replace.
	filePath := "source.md"
	fullPath := filepath.Join(vault, filePath)
	original := []byte("[[OldTarget]]\n")
	if err := os.WriteFile(fullPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	// Ensure permission is exactly 0o600 (not masked by umask).
	if err := os.Chmod(fullPath, 0o600); err != nil {
		t.Fatal(err)
	}

	groups := map[string][]rewriteEntry{
		filePath: {
			{
				edgeID:     1,
				rawLink:    "[[OldTarget]]",
				linkType:   "wikilink",
				lineStart:  1,
				sourcePath: filePath,
				sourceID:   100,
				newRawLink: "[[NewTarget]]",
			},
		},
	}

	_, backups, err := applyFileRewrites(vault, groups)
	if err != nil {
		t.Fatalf("applyFileRewrites: %v", err)
	}

	// Verify file content was rewritten.
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "[[NewTarget]]\n" {
		t.Errorf("content = %q, want %q", string(content), "[[NewTarget]]\n")
	}

	// Verify file permission preserved.
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want %o", perm, 0o600)
	}

	// Verify backup has correct perm field.
	if len(backups) != 1 {
		t.Fatalf("len(backups) = %d, want 1", len(backups))
	}
	if backups[0].perm != 0o600 {
		t.Errorf("backup perm = %o, want %o", backups[0].perm, 0o600)
	}
}
