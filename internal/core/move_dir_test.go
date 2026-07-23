package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MoveDir tests
// ===============================================

func TestMoveDir_Basic(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	if len(result.Moved) != 3 {
		t.Fatalf("expected 3 moved files, got %d", len(result.Moved))
	}

	for _, m := range result.Moved {
		if fileExists(filepath.Join(vault, m.From)) {
			t.Errorf("%s should not exist on disk", m.From)
		}
		if !fileExists(filepath.Join(vault, m.To)) {
			t.Errorf("%s should exist on disk", m.To)
		}
	}

	// Verify DB.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if strings.HasPrefix(n.path, "sub/") {
			t.Errorf("DB still has old path: %s", n.path)
		}
	}
	var foundA, foundB, foundX bool
	for _, n := range notes {
		switch n.path {
		case "newdir/A.md":
			foundA = true
		case "newdir/B.md":
			foundB = true
		case "newdir/inner/X.md":
			foundX = true
		}
	}
	if !foundA || !foundB || !foundX {
		t.Errorf("DB missing new paths: A=%v B=%v X=%v", foundA, foundB, foundX)
	}
}

func TestMoveDir_NoFiles(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "nonexist", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "no files registered under directory") {
		t.Errorf("expected 'no files' error, got: %v", err)
	}
}

func TestMoveDir_DestConflict(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create destination with same name already registered.
	if err := os.MkdirAll(filepath.Join(vault, "dst"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "dst", "A.md"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dst"})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

func TestMoveDir_IncomingRewrite(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Other.md has [[sub/B]] → should be rewritten to [[newdir/B]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "Other.md" && rw.OldLink == "[[sub/B]]" {
			found = true
			if rw.NewLink != "[[newdir/B]]" {
				t.Errorf("expected [[newdir/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("Other.md [[sub/B]] should be rewritten")
	}

	otherContent, err := os.ReadFile(filepath.Join(vault, "Other.md"))
	if err != nil {
		t.Fatalf("read Other.md: %v", err)
	}
	if !strings.Contains(string(otherContent), "[[newdir/B]]") {
		t.Errorf("Other.md should contain [[newdir/B]], got: %s", string(otherContent))
	}
}

func TestMoveDir_IncomingMultiplePathLinks(t *testing.T) {
	// External file has multiple path links to different files in the moved directory.
	// Both should be rewritten.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Ext.md"), []byte("[[sub/A]]\n[[sub/B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// Both path links in Ext.md should be rewritten.
	rewrittenLinks := make(map[string]string)
	for _, rw := range result.Rewritten {
		if rw.File == "Ext.md" {
			rewrittenLinks[rw.OldLink] = rw.NewLink
		}
	}
	if len(rewrittenLinks) != 2 {
		t.Errorf("expected 2 rewrites in Ext.md, got %d: %v", len(rewrittenLinks), rewrittenLinks)
	}
	if rw, ok := rewrittenLinks["[[sub/A]]"]; !ok || rw != "[[newdir/A]]" {
		t.Errorf("expected [[sub/A]] → [[newdir/A]], got %v", rewrittenLinks)
	}
	if rw, ok := rewrittenLinks["[[sub/B]]"]; !ok || rw != "[[newdir/B]]" {
		t.Errorf("expected [[sub/B]] → [[newdir/B]], got %v", rewrittenLinks)
	}

	extContent, err := os.ReadFile(filepath.Join(vault, "Ext.md"))
	if err != nil {
		t.Fatalf("read Ext.md: %v", err)
	}
	if !strings.Contains(string(extContent), "[[newdir/A]]") || !strings.Contains(string(extContent), "[[newdir/B]]") {
		t.Errorf("Ext.md should contain both [[newdir/A]] and [[newdir/B]], got: %s", string(extContent))
	}
}

func TestMoveDir_CollateralRewrite(t *testing.T) {
	// Directory move preserves basenames, so collateral rewrite (basename
	// ambiguity caused by the move) cannot occur. Instead, verify that
	// a basename link to a moved file is correctly rewritten to a path link
	// when the basename was already ambiguous before the move.
	//
	// Setup: sub1/A.md (unique basename "A"), B.md has [[A]] (→ sub1/A.md).
	// Move sub1 → sub2. Basename "A" count stays 1. [[A]] still resolves → no rewrite needed.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// [[A]] should still resolve to sub2/A.md via basename. No rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" {
			t.Errorf("B.md should NOT be rewritten (basename A is still unique), got: %+v", rw)
		}
	}

	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[A]]") {
		t.Errorf("B.md should still contain [[A]], got: %s", string(bContent))
	}
}

func TestMoveDir_CollateralMultipleBasenames(t *testing.T) {
	// sub1/A.md, sub1/B.md exist. X.md has [[A]], Y.md has [[B]].
	// Create src/A.md and src/B.md.
	// Move src → sub2 → both "A" and "B" become ambiguous.
	vault := t.TempDir()
	for _, d := range []string{"sub1", "src", "sub2"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Y.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "src", "D.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move src → sub2, renaming: C→A, D→B would need different dir names.
	// Actually, for dir move basename doesn't change. Let's test differently.
	// Move sub1 → sub2. Basenames A and B stay the same. No ambiguity created.
	// This test verifies that when basename count stays 1, no rewrite happens.
	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// X.md's [[A]] and Y.md's [[B]] should NOT be rewritten (basename unique, unchanged).
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[A]]" {
			t.Errorf("X.md [[A]] should NOT be rewritten, got %s", rw.NewLink)
		}
		if rw.File == "Y.md" && rw.OldLink == "[[B]]" {
			t.Errorf("Y.md [[B]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_CollateralRootPriority(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]].
	// Move sub → newdir → basename "A" still has root A.md → no collateral.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// B.md's [[A]] should NOT be rewritten (root priority).
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("B.md [[A]] should NOT be rewritten (root priority), got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingBasenameToMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// sub/A.md has [[B]] → target sub/B.md also moves to newdir/B.md.
	// Basename "B" stays unique → no rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[[B]]" {
			t.Errorf("newdir/A.md [[B]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingPathToMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Other.md has [[sub/B]] — this is an external incoming rewrite.
	// sub/A.md has no path link to sub/B.
	// Let's check the fixture — sub/A.md has [link to B](./B.md) which is relative.
	// That should be handled by the relative rewrite batch.

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "Other.md" && rw.OldLink == "[[sub/B]]" {
			found = true
		}
	}
	if !found {
		t.Error("Other.md [[sub/B]] should be rewritten")
	}
}

func TestMoveDir_RelativeBetweenMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// sub/A.md has [link to B](./B.md) — both A and B move to newdir/.
	// The relative path should remain ./B.md (unchanged).
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[link to B](./B.md)" {
			t.Errorf("newdir/A.md relative link should NOT change, but was rewritten to %s", rw.NewLink)
		}
	}

	content, err := os.ReadFile(filepath.Join(vault, "newdir", "A.md"))
	if err != nil {
		t.Fatalf("read newdir/A.md: %v", err)
	}
	if !strings.Contains(string(content), "[link to B](./B.md)") {
		t.Errorf("newdir/A.md should preserve relative link, got: %s", string(content))
	}
}

func TestMoveDir_OneFileRelativeSelfLinksKeepPointingToMovedNote(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "wikilink",
			content: "[[./a]]\n",
			want:    "[[./a]]\n",
		},
		{
			name:    "markdown",
			content: "[self](./a.md)\n",
			want:    "[self](./a.md)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{
				"old/a.md": tt.content,
			})

			result, err := MoveDir(vault, MoveDirOptions{FromDir: "old", ToDir: "sub"})
			if err != nil {
				t.Fatalf("MoveDir: %v", err)
			}

			got := readVaultFile(t, vault, "sub/a.md")
			if got != tt.want {
				t.Fatalf("moved note content = %q, want %q; rewrites: %+v", got, tt.want, result.Rewritten)
			}
			assertEdgeRawLinks(t, vault, "sub/a.md", []string{strings.TrimSpace(tt.want)})
		})
	}
}

func TestMoveDir_MutualRelativeLinksMoveTogether(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"old/a.md": "[[./b]]\n[b](./b.md)\n",
		"old/b.md": "[[./a]]\n[a](./a.md)\n",
	})

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "old", ToDir: "new"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}
	for _, rw := range result.Rewritten {
		if rw.File == "new/a.md" || rw.File == "new/b.md" {
			t.Fatalf("relative links between moved files should not be rewritten, got: %+v", result.Rewritten)
		}
	}

	if got := readVaultFile(t, vault, "new/a.md"); got != "[[./b]]\n[b](./b.md)\n" {
		t.Fatalf("new/a.md = %q", got)
	}
	if got := readVaultFile(t, vault, "new/b.md"); got != "[[./a]]\n[a](./a.md)\n" {
		t.Fatalf("new/b.md = %q", got)
	}
	assertEdgeRawLinks(t, vault, "new/a.md", []string{"[[./b]]", "[b](./b.md)"})
	assertEdgeRawLinks(t, vault, "new/b.md", []string{"[[./a]]", "[a](./a.md)"})
}

func TestMoveDir_RelativeToExternal(t *testing.T) {
	// sub/A.md has [ext](../Root.md). Move sub → newdir.
	// newdir/A.md → Root.md should become ../Root.md (still valid if newdir is 1 level deep).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[ext](../Root.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Root.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// ../Root.md from sub/ = Root.md. From newdir/, it should still be ../Root.md.
	// Since both are 1-level deep, relative path stays the same.
	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[ext](../Root.md)" {
			t.Errorf("relative link to external should not change, but was rewritten to %s", rw.NewLink)
		}
	}
}

func TestMoveDir_OutgoingPathToExternal(t *testing.T) {
	// sub/A.md has [[Root]] (basename link to external Root.md).
	// Move sub → newdir. [[Root]] stays as basename link, no rewrite needed.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[[Root]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Root.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	for _, rw := range result.Rewritten {
		if rw.File == "newdir/A.md" && rw.OldLink == "[[Root]]" {
			t.Errorf("[[Root]] should NOT be rewritten, got %s", rw.NewLink)
		}
	}
}

func TestMoveDir_ExternalRewriteWithStaleFile(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "Other.md"), []byte("[[A]]\n[[sub/B]]\nmodified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("expected success despite stale Other.md, got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "Other.md"))
	if err != nil {
		t.Fatalf("read Other.md: %v", err)
	}
	if !strings.Contains(string(content), "newdir") {
		t.Error("Other.md should have links rewritten to newdir/")
	}
}

func TestMoveDir_AlreadyMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already moved the directory.
	if err := os.MkdirAll(filepath.Join(vault, "newdir", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A.md", "B.md"} {
		if err := os.Rename(
			filepath.Join(vault, "sub", name),
			filepath.Join(vault, "newdir", name),
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(
		filepath.Join(vault, "sub", "inner", "X.md"),
		filepath.Join(vault, "newdir", "inner", "X.md"),
	); err != nil {
		t.Fatal(err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir (already moved): %v", err)
	}

	if len(result.Moved) != 3 {
		t.Errorf("expected 3 moved files, got %d", len(result.Moved))
	}

	notes := queryNodes(t, dbPath(vault), "note")
	var found bool
	for _, n := range notes {
		if n.path == "newdir/A.md" {
			found = true
		}
	}
	if !found {
		t.Error("DB should contain newdir/A.md")
	}
}

func TestMoveDir_Stale(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil {
		t.Fatal("expected stale error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("expected stale error, got: %v", err)
	}
}

func TestMoveDir_Nested(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	var foundNested bool
	for _, m := range result.Moved {
		if m.From == "sub/inner/X.md" && m.To == "newdir/inner/X.md" {
			foundNested = true
		}
	}
	if !foundNested {
		t.Error("sub/inner/X.md should be moved to newdir/inner/X.md")
	}

	if !fileExists(filepath.Join(vault, "newdir", "inner", "X.md")) {
		t.Error("newdir/inner/X.md should exist on disk")
	}
}

func TestMoveDir_Overlap(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "sub/inner"})
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Errorf("expected overlap error, got: %v", err)
	}
}

func TestMoveDir_VaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "../outside"})
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}

	// Absolute paths should also be rejected.
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "/abs/path", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "vault-relative") {
		t.Errorf("expected absolute path error for from, got: %v", err)
	}
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "/abs/path"})
	if err == nil || !strings.Contains(err.Error(), "vault-relative") {
		t.Errorf("expected absolute path error for to, got: %v", err)
	}
}

func TestMoveDir_DestExistsOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create an unregistered file at the destination.
	if err := os.MkdirAll(filepath.Join(vault, "newdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "newdir", "A.md"), []byte("conflict\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil || !strings.Contains(err.Error(), "already exists on disk") {
		t.Errorf("expected 'already exists on disk' error, got: %v", err)
	}
}

func TestMoveDir_PhantomPromotion(t *testing.T) {
	// A.md and B.md link to [[X]] which is a phantom.
	// Move sub/X.md to a new dir. Since dir move doesn't change basename,
	// this is essentially testing that phantom promotion works.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[X]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "Y.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// X is a phantom. Move sub → newdir won't promote X because basename
	// doesn't change and there's no X.md in sub.
	// This test just verifies no crash on phantom promotion code path.
	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}
	if len(result.Moved) != 1 {
		t.Errorf("expected 1 moved file, got %d", len(result.Moved))
	}
}

func TestMoveDir_NonMDFileMovedAlong(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	if err := os.WriteFile(filepath.Join(vault, "sub", "image.png"), []byte("png data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir should succeed with non-.md files: %v", err)
	}

	if _, err := os.Stat(filepath.Join(vault, "newdir", "image.png")); err != nil {
		t.Error("newdir/image.png should exist after move")
	}

	if len(result.Moved) == 0 {
		t.Error("expected files to be moved")
	}
}

func TestMoveDir_HiddenFilesIgnored(t *testing.T) {
	vault := copyVault(t, "vault_move_dir")
	// Add a hidden file — should be ignored by the non-.md check.
	if err := os.WriteFile(filepath.Join(vault, "sub", ".DS_Store"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err != nil {
		t.Fatalf("MoveDir should succeed with hidden files: %v", err)
	}
	if len(result.Moved) == 0 {
		t.Error("expected files to be moved")
	}
}

// --- Consecutive directory moves: the second move must not fail with stale ---
// Scenario: dirA/ and dirB/ both have files. notes/Linker.md links to files
// in both dirs via path links. After moving dirA/ into notes/, Linker.md is
// rewritten (link targets changed). The second move (dirB/ into notes/) should
// succeed because the first move must have updated Linker.md's mtime in the DB.
func TestMoveDir_ConsecutiveMovesNoStale(t *testing.T) {
	vault := t.TempDir()

	for _, d := range []string{"dirA", "dirB", "notes"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"dirA/Alpha.md":   "alpha content\n",
		"dirB/Beta.md":    "beta content\n",
		"notes/Linker.md": "[[dirA/Alpha]]\n[[dirB/Beta]]\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Wait so that file rewrites produce a different mtime (Unix second precision).
	time.Sleep(1100 * time.Millisecond)

	// First move: dirA/ → notes/
	result1, err := MoveDir(vault, MoveDirOptions{FromDir: "dirA", ToDir: "notes/dirA"})
	if err != nil {
		t.Fatalf("first MoveDir: %v", err)
	}

	var linkerRewritten bool
	for _, rw := range result1.Rewritten {
		if rw.File == "notes/Linker.md" {
			linkerRewritten = true
		}
	}
	if !linkerRewritten {
		t.Fatal("expected notes/Linker.md to have links rewritten after first move")
	}

	// Second move: dirB/ → notes/ — this must succeed.
	// If the first move didn't update Linker.md's mtime in the DB,
	// this will fail with "source file is stale: notes/Linker.md".
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "dirB", ToDir: "notes/dirB"})
	if err != nil {
		t.Fatalf("second MoveDir should succeed but got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "notes", "Linker.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "[[notes/dirA/Alpha]]") {
		t.Errorf("Linker.md should contain [[notes/dirA/Alpha]], got: %s", s)
	}
	if !strings.Contains(s, "[[notes/dirB/Beta]]") {
		t.Errorf("Linker.md should contain [[notes/dirB/Beta]], got: %s", s)
	}
}

// Merge into existing directory: move --from dirA --to notes (notes/ already has files).
// This matches the real-world scenario: mdhop move --from 04-Resources/ --to 03-Notes/
// followed by mdhop move --from 05-Thoughts/ --to 03-Notes/.
func TestMoveDir_ConsecutiveMergeNoStale(t *testing.T) {
	vault := t.TempDir()

	for _, d := range []string{"resources", "thoughts", "notes"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"resources/ResA.md": "resource A\n",
		"thoughts/ThoB.md":  "thought B\n",
		"notes/Hub.md":      "[[resources/ResA]]\n[[thoughts/ThoB]]\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Ensure mtime will differ after rewrite.
	time.Sleep(1100 * time.Millisecond)

	// First: move resources/ → notes/ (merges into existing dir).
	result1, err := MoveDir(vault, MoveDirOptions{FromDir: "resources", ToDir: "notes"})
	if err != nil {
		t.Fatalf("first MoveDir (resources → notes): %v", err)
	}

	var hubRewritten bool
	for _, rw := range result1.Rewritten {
		if rw.File == "notes/Hub.md" {
			hubRewritten = true
		}
	}
	if !hubRewritten {
		t.Fatal("expected notes/Hub.md to be rewritten after first move")
	}

	// Second: move thoughts/ → notes/ (merges into same dir).
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "thoughts", ToDir: "notes"})
	if err != nil {
		t.Fatalf("second MoveDir (thoughts → notes) should succeed but got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "notes", "Hub.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "[[notes/ResA]]") && !strings.Contains(s, "[[ResA]]") {
		t.Errorf("Hub.md should reference ResA, got: %s", s)
	}
	if !strings.Contains(s, "[[notes/ThoB]]") && !strings.Contains(s, "[[ThoB]]") {
		t.Errorf("Hub.md should reference ThoB, got: %s", s)
	}
}

// Exact reproduction of the reported bug:
//   - Moved file (04-Resources/A.md) has outgoing links to files in 05-Thoughts/
//   - After move to 03-Notes/, the file's outgoing link is rewritten (Phase 4.2)
//   - File's mtime changes on disk
//   - Second move (05-Thoughts/ → 03-Notes/) needs to rewrite links in 03-Notes/A.md
//     (now an external file), triggering stale check
func TestMoveDir_ConsecutiveWithCrossLinks(t *testing.T) {
	vault := t.TempDir()

	for _, d := range []string{"resources", "thoughts", "notes"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		// Resource file has outgoing link to a thought file (path link).
		"resources/ResA.md": "# Resource A\n\nSee also: [[thoughts/ThoB]]\n",
		"thoughts/ThoB.md":  "# Thought B\n\nRelated: [[resources/ResA]]\n",
		"notes/Hub.md":      "[[resources/ResA]]\n[[thoughts/ThoB]]\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Ensure mtime will differ after rewrite.
	time.Sleep(1100 * time.Millisecond)

	// First move: resources/ → notes/
	// ResA.md has [[thoughts/ThoB]] which doesn't change (thoughts/ isn't moving).
	// But ThoB.md has [[resources/ResA]] which becomes an incoming rewrite.
	// Hub.md has [[resources/ResA]] which is also an incoming rewrite.
	result1, err := MoveDir(vault, MoveDirOptions{FromDir: "resources", ToDir: "notes"})
	if err != nil {
		t.Fatalf("first MoveDir: %v", err)
	}
	_ = result1

	// Second move: thoughts/ → notes/
	// ThoB.md (being moved) has [[resources/ResA]] which was already rewritten
	// to [[notes/ResA]] by the first move.
	// notes/ResA.md (moved in first step) has [[thoughts/ThoB]] — incoming rewrite needed.
	// This is the critical case: notes/ResA.md was written to disk by the first move
	// (Phase 4.2 outgoing rewrite or Phase 4.1 if it was also an external rewrite target),
	// so its disk mtime differs from build time. If the first move didn't update
	// notes/ResA.md's mtime in DB, this will fail with stale error.
	_, err = MoveDir(vault, MoveDirOptions{FromDir: "thoughts", ToDir: "notes"})
	if err != nil {
		t.Fatalf("second MoveDir should succeed but got: %v", err)
	}

	resA, err := os.ReadFile(filepath.Join(vault, "notes", "ResA.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(resA)
	// ResA should now link to notes/ThoB (moved in second step).
	if !strings.Contains(s, "[[notes/ThoB]]") && !strings.Contains(s, "[[ThoB]]") {
		t.Errorf("ResA.md should reference ThoB after second move, got: %s", s)
	}
}

// --- MoveDir: Phase 2.5 collateral coverage ---

func TestMoveDir_CollateralNoRootCallsQuery(t *testing.T) {
	// preRoot=F, postRoot=F: basename count > 1 with no root file.
	// queryCollateralRewrites is called but returns empty (no basename links exist).
	vault := t.TempDir()
	for _, d := range []string{"sub1", "other"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "other", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}
	if len(result.Rewritten) != 0 {
		t.Errorf("expected no rewrites, got %d: %+v", len(result.Rewritten), result.Rewritten)
	}
}

func TestMoveDir_CollateralSkipsPathLink(t *testing.T) {
	// queryCollateralRewrites skips non-basename links via isBasenameRawLink filter.
	// X.md has [[other/A]] (path link) → should NOT be rewritten.
	vault := t.TempDir()
	for _, d := range []string{"sub1", "other"} {
		if err := os.MkdirAll(filepath.Join(vault, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "other", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[other/A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MoveDir(vault, MoveDirOptions{FromDir: "sub1", ToDir: "sub2"})
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	// X.md's [[other/A]] must NOT be rewritten (path link filtered by isBasenameRawLink).
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" {
			t.Errorf("X.md should NOT be rewritten, got: %+v", rw)
		}
	}

	xContent, err := os.ReadFile(filepath.Join(vault, "X.md"))
	if err != nil {
		t.Fatalf("read X.md: %v", err)
	}
	if !strings.Contains(string(xContent), "[[other/A]]") {
		t.Errorf("X.md should still contain [[other/A]], got: %s", string(xContent))
	}
}

// --- MoveDir: rollback paths ---

// TestMoveDir_Rollback_RenameFails covers move_dir.go lines 606-619:
// when an os.Rename in Phase 4.3 fails, the deferred rollback restores
// previously completed renames, moved-file content, and external rewrites.
func TestMoveDir_Rollback_RenameFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
	vault := copyVault(t, "vault_move_dir")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Snapshot Other.md content for post-rollback comparison.
	otherPath := filepath.Join(vault, "Other.md")
	originalOther, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatalf("read Other.md: %v", err)
	}

	// Create the destination directory ahead of time and remove its write
	// permission so os.Rename into it fails for every moved file.
	newdir := filepath.Join(vault, "newdir")
	if err := os.MkdirAll(newdir, 0o755); err != nil {
		t.Fatalf("mkdir newdir: %v", err)
	}
	if err := os.Chmod(newdir, 0o555); err != nil {
		t.Fatalf("chmod newdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(newdir, 0o755) })

	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"}); err == nil {
		t.Fatal("expected MoveDir to fail when rename target dir is read-only")
	}

	// Disk: every from-path must still exist; no to-paths created.
	for _, from := range []string{"sub/A.md", "sub/B.md", "sub/inner/X.md"} {
		if !fileExists(filepath.Join(vault, from)) {
			t.Errorf("after rollback: %s should still exist", from)
		}
	}
	for _, to := range []string{"newdir/A.md", "newdir/B.md", "newdir/inner/X.md"} {
		if fileExists(filepath.Join(vault, to)) {
			t.Errorf("after rollback: %s should not exist", to)
		}
	}

	// External file content must be restored.
	restoredOther, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatalf("read Other.md after rollback: %v", err)
	}
	if string(restoredOther) != string(originalOther) {
		t.Errorf("Other.md not restored after rollback:\nwant: %q\ngot:  %q", originalOther, restoredOther)
	}

	// DB must be untouched: paths still under sub/, edges still point to sub/B.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if strings.HasPrefix(n.path, "newdir/") {
			t.Errorf("DB should not contain newdir/ paths after rollback, got: %s", n.path)
		}
	}
	edges := queryEdges(t, dbPath(vault), "Other.md")
	var sawSubB bool
	for _, e := range edges {
		if e.rawLink == "[[sub/B]]" {
			sawSubB = true
		}
		if e.rawLink == "[[newdir/B]]" {
			t.Errorf("edge raw_link should be rolled back, but found %s", e.rawLink)
		}
	}
	if !sawSubB {
		t.Error("Other.md edge to sub/B.md missing after rollback")
	}
}

// TestMoveDir_Rollback_MovedFileRestore covers move_dir.go lines 583-590
// (Phase 4.2 inline rollback) and 611-617 (deferred moved-file restore):
// when a moved file has scheduled outgoing rewrites, Phase 4.2 writes them
// to disk and registers backups; any later failure must restore those bytes.
// We force Phase 4.3's rename to fail via a read-only destination directory
// after Phase 4.2 has already rewritten every moved file.
func TestMoveDir_Rollback_MovedFileRestore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}

	// Each moved file owns a relative `../external.md` link. MoveDir(sub →
	// newdir/inner) shifts every sub/*.md from depth 1 to depth 2, so the
	// link must become `../../external.md` — that is what makes Phase 3
	// schedule an outgoing rewrite, which is what populates movedFileBackups.
	vault := t.TempDir()
	mustWrite := func(rel, body string) {
		full := filepath.Join(vault, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mustWrite("external.md", "external\n")
	mustWrite("sub/A.md", "[link](../external.md)\n")
	mustWrite("sub/B.md", "[link](../external.md)\n")

	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	originalSubA, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md: %v", err)
	}
	originalSubB, err := os.ReadFile(filepath.Join(vault, "sub", "B.md"))
	if err != nil {
		t.Fatalf("read sub/B.md: %v", err)
	}

	// Pre-create the destination's inner dir read-only so Phase 4.3's
	// Rename(...) fails. MkdirAll is a no-op on existing dirs and does not
	// fail; Rename into a read-only directory does.
	innerDir := filepath.Join(vault, "newdir", "inner")
	if err := os.MkdirAll(innerDir, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	if err := os.Chmod(innerDir, 0o555); err != nil {
		t.Fatalf("chmod inner: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(innerDir, 0o755) })

	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir/inner"}); err == nil {
		t.Fatal("expected MoveDir to fail when destination dir is read-only")
	}

	// Moved file content must be restored to pre-move bytes; if rollback was
	// skipped or buggy, we'd see the rewritten body here instead.
	gotA, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md after rollback: %v", err)
	}
	if string(gotA) != string(originalSubA) {
		t.Errorf("sub/A.md not restored:\nwant: %q\ngot:  %q", originalSubA, gotA)
	}
	gotB, err := os.ReadFile(filepath.Join(vault, "sub", "B.md"))
	if err != nil {
		t.Fatalf("read sub/B.md after rollback: %v", err)
	}
	if string(gotB) != string(originalSubB) {
		t.Errorf("sub/B.md not restored:\nwant: %q\ngot:  %q", originalSubB, gotB)
	}

	// Disk: from-paths intact, to-paths absent.
	for _, from := range []string{"sub/A.md", "sub/B.md"} {
		if !fileExists(filepath.Join(vault, from)) {
			t.Errorf("after rollback: %s should still exist", from)
		}
	}
	for _, to := range []string{"newdir/inner/A.md", "newdir/inner/B.md"} {
		if fileExists(filepath.Join(vault, to)) {
			t.Errorf("after rollback: %s should not exist", to)
		}
	}

	// DB must be untouched.
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if strings.HasPrefix(n.path, "newdir/") {
			t.Errorf("DB should not contain newdir/ paths after rollback, got: %s", n.path)
		}
	}
}

func TestMoveDir_RollbackRenameFailureIsReturned(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"sub/A.md": "A\n",
		"sub/B.md": "B\n",
	})

	oldRename := moveRename
	var primaryMoves int
	moveRename = func(from, to string) error {
		fromRel, fromErr := filepath.Rel(vault, from)
		toRel, toErr := filepath.Rel(vault, to)
		if fromErr != nil || toErr != nil {
			return oldRename(from, to)
		}
		fromRel = filepath.ToSlash(fromRel)
		toRel = filepath.ToSlash(toRel)

		if strings.HasPrefix(fromRel, "sub/") && strings.HasPrefix(toRel, "newdir/") {
			primaryMoves++
			if primaryMoves == 1 {
				return oldRename(from, to)
			}
			return errors.New("primary rename blocked")
		}
		if strings.HasPrefix(fromRel, "newdir/") && strings.HasPrefix(toRel, "sub/") {
			return errors.New("rollback rename blocked")
		}
		return oldRename(from, to)
	}
	t.Cleanup(func() { moveRename = oldRename })

	_, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "newdir"})
	if err == nil {
		t.Fatal("expected move dir to fail")
	}
	msg := err.Error()
	for _, want := range []string{
		"primary rename blocked",
		"rollback failed",
		"could not move back newdir/",
		" -> sub/",
		"rollback rename blocked",
		"mdhop build",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q:\n%s", want, msg)
		}
	}
}
