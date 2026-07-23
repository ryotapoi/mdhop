package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test 21: file permission preserved after move with outgoing rewrite ---
func TestMovePreservesPermission(t *testing.T) {
	vault := t.TempDir()
	// A.md has a relative outgoing link to B.md.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[link](./B.md)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(vault, "A.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to sub/A.md — outgoing relative link gets rewritten.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	info, err := os.Stat(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("moved file perm = %o, want %o", perm, 0o600)
	}
}

// --- Test 22: rename to root target → vault-relative rewrite [[X]] ---
func TestMove_RollbackRestoreFailureIsReturned(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"A.md":     "content\n",
		"Other.md": "[[A]]\n",
	})

	oldRename := moveRename
	moveRename = func(from, to string) error {
		return errors.New("primary rename blocked")
	}
	t.Cleanup(func() { moveRename = oldRename })

	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, "Other.md") {
			return errors.New("restore blocked")
		}
		return oldRollbackWriteFile(path, data, perm)
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected move to fail")
	}
	msg := err.Error()
	for _, want := range []string{
		"primary rename blocked",
		"rollback failed",
		"could not restore Other.md",
		"restore blocked",
		"mdhop build",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q:\n%s", want, msg)
		}
	}
}

func TestMove_ExternalRewriteRollbackFailureIsReturned(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"A.md":   "content\n",
		"One.md": "[[A]]\n",
		"Two.md": "[[A]]\n",
	})

	oldRewriteWriteFile := rewriteWriteFile
	var writeCalls int
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 2 {
			return errors.New("external rewrite blocked")
		}
		return oldRewriteWriteFile(path, data, perm)
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(path string, data []byte, perm os.FileMode) error {
		return errors.New("restore blocked")
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected move to fail")
	}
	msg := err.Error()
	for _, want := range []string{
		"external rewrite blocked",
		"rollback failed",
		"could not restore",
		"restore blocked",
		"mdhop build",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q:\n%s", want, msg)
		}
	}
}
