package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

func copyVaultForResolve(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", name)
	dst := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, dst); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	return dst
}

func buildVault(t *testing.T, vaultPath string) {
	t.Helper()
	if err := Build(vaultPath); err != nil {
		t.Fatalf("build: %v", err)
	}
}

func TestResolveWikilinkBasename(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[Design]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
	if !res.Exists {
		t.Errorf("exists = false, want true")
	}
	if res.Subpath != "" {
		t.Errorf("subpath = %q, want empty", res.Subpath)
	}
}

func TestResolveWikilinkVaultRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[sub/Impl]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveWikilinkSubpath(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Design.md", "[[sub/Impl#Details]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
	if res.Subpath != "#Details" {
		t.Errorf("subpath = %q, want %q", res.Subpath, "#Details")
	}
}

func TestResolveWikilinkSelfLink(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[#Index]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Path, "Index.md")
	}
	if res.Subpath != "#Index" {
		t.Errorf("subpath = %q, want %q", res.Subpath, "#Index")
	}
}

func TestResolveMarkdownRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[impl](./sub/Impl.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveMarkdownAbsolute(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "sub/Impl.md", "[index](/Index.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Index" {
		t.Errorf("name = %q, want %q", res.Name, "Index")
	}
	if res.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Path, "Index.md")
	}
}

func TestResolveMarkdownPath(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_edges")
	buildVault(t, vault)

	res, err := Resolve(vault, "A.md", "[C](sub/C.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "C" {
		t.Errorf("name = %q, want %q", res.Name, "C")
	}
	if res.Path != "sub/C.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/C.md")
	}
}

func TestResolveMarkdownRelativeParent(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_relative")
	buildVault(t, vault)

	res, err := Resolve(vault, "dir/Source.md", "[root](../Root.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Root" {
		t.Errorf("name = %q, want %q", res.Name, "Root")
	}
	if res.Path != "Root.md" {
		t.Errorf("path = %q, want %q", res.Path, "Root.md")
	}
}

func TestResolvePhantom(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[Missing]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "phantom" {
		t.Errorf("type = %q, want %q", res.Type, "phantom")
	}
	if res.Name != "Missing" {
		t.Errorf("name = %q, want %q", res.Name, "Missing")
	}
	if res.Exists {
		t.Errorf("exists = true, want false")
	}
}

func TestResolveTag(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "#overview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "tag" {
		t.Errorf("type = %q, want %q", res.Type, "tag")
	}
	if res.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Name, "#overview")
	}
}

func TestResolveFrontmatterTag(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "#project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "tag" {
		t.Errorf("type = %q, want %q", res.Type, "tag")
	}
	if res.Name != "#project" {
		t.Errorf("name = %q, want %q", res.Name, "#project")
	}
}

func TestResolveMarkdownBasename(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[design](Design.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveWikilinkRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[./sub/Impl]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[design]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "note" {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveErrorSourceNotInDB(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	_, err := Resolve(vault, "NonExist.md", "[[Design]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "source not in index") {
		t.Errorf("error = %q, want containing %q", err.Error(), "source not in index")
	}
}

func TestResolveErrorLinkNotInSource(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	// [[NotHere]] is not a link in Index.md
	_, err := Resolve(vault, "Index.md", "[[NotHere]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "link not found") {
		t.Errorf("error = %q, want containing %q", err.Error(), "link not found")
	}
}

func TestResolveBasenameRootPriority(t *testing.T) {
	// Design.md is at root + insert other/Design.md → root priority resolves to root.
	vault := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(filepath.Join("..", "..", "testdata", "vault_build_full"), vault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	buildVault(t, vault)

	// Manually insert a second note with the same basename as "Design".
	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime) VALUES (?, 'note', 'Design', 'other/Design.md', 1, 0)`,
		noteKey("other/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("insert duplicate: %v", err)
	}
	db.Close()

	// Root priority: Design.md is at root → resolves to it.
	res, err := Resolve(vault, "Index.md", "[[Design]]")
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveBasenameAmbiguousNoRoot(t *testing.T) {
	// Two notes in subdirs (no root) → ambiguous.
	vault := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(filepath.Join("..", "..", "testdata", "vault_build_full"), vault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	buildVault(t, vault)

	// Rename Design.md in DB to sub1/Design.md (no root).
	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		`UPDATE nodes SET path = 'sub1/Design.md', node_key = ? WHERE path = 'Design.md'`,
		noteKey("sub1/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("update path: %v", err)
	}
	// Insert second note at sub2/Design.md.
	_, err = db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime) VALUES (?, 'note', 'Design', 'sub2/Design.md', 1, 0)`,
		noteKey("sub2/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("insert duplicate: %v", err)
	}
	db.Close()

	_, err = Resolve(vault, "Index.md", "[[Design]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %q, want containing %q", err.Error(), "ambiguous")
	}
}

func TestResolveErrorDBNotFound(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_empty")
	// Do NOT build — DB should not exist.

	_, err := Resolve(vault, "A.md", "[[X]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "index not found") {
		t.Errorf("error = %q, want containing %q", err.Error(), "index not found")
	}
}
