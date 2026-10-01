package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
)

func TestWritablePathSymlinkBoundary(t *testing.T) {
	vault := t.TempDir()
	outside := t.TempDir()
	for _, dir := range []string{vault, outside} {
		if err := os.WriteFile(filepath.Join(dir, "Note.md"), []byte("original\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range []struct{ name, target string }{
		{"External.md", filepath.Join(outside, "Note.md")},
		{"external-dir", outside},
		{"Internal.md", filepath.Join(vault, "Note.md")},
		{"internal-dir", vault},
		{norm.NFD.String("Café.md"), filepath.Join(outside, "Note.md")},
	} {
		if err := os.Symlink(link.target, filepath.Join(vault, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	rootLink := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(vault, rootLink); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{vault, rootLink} {
		resolver := newVaultDiskPathResolver(root)
		for _, name := range []string{"External.md", "external-dir/Note.md", "Café.md"} {
			if _, err := resolver.writablePath(name); err == nil {
				t.Fatalf("%s: accepted external path %s", root, name)
			}
		}
		for _, name := range []string{"Note.md", "Internal.md", "internal-dir/Note.md"} {
			if _, err := resolver.writablePath(name); err != nil {
				t.Fatalf("%s: rejected internal path %s: %v", root, name, err)
			}
		}
	}
}

func TestSetSymlinkBoundary(t *testing.T) {
	for _, external := range []bool{true, false} {
		t.Run(map[bool]string{true: "external", false: "internal-root-link"}[external], func(t *testing.T) {
			vault := t.TempDir()
			targetDir := vault
			if external {
				targetDir = t.TempDir()
			}
			target := filepath.Join(targetDir, "Target.md")
			original := "# Original\n"
			if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(vault, "Link.md")); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			beforeDB := mustReadFile(t, dbPath(vault))
			root := vault
			if !external {
				root = filepath.Join(t.TempDir(), "vault")
				if err := os.Symlink(vault, root); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Set(root, SetOptions{File: "Link.md", Key: "status", Value: "done"})
			if external {
				if err == nil {
					t.Fatal("set accepted external symlink")
				}
				if got := readTestFile(t, target); got != original {
					t.Fatalf("external content changed: %q", got)
				}
				if string(mustReadFile(t, dbPath(vault))) != string(beforeDB) {
					t.Fatal("index changed on rejected set")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(readTestFile(t, target), "status: done") {
					t.Fatal("internal target was not updated")
				}
			}
		})
	}
}

func TestRewriteCommandsRejectExternalSymlinkBeforeAnyWrite(t *testing.T) {
	for _, command := range []string{"add", "convert"} {
		t.Run(command, func(t *testing.T) {
			vault := t.TempDir()
			outside := filepath.Join(t.TempDir(), "Outside.md")
			content := "[[B]]\n"
			for _, path := range []string{filepath.Join(vault, "A.md"), outside} {
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(outside, filepath.Join(vault, "Z.md")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(vault, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(vault, "sub/B.md"), []byte("# B\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			beforeDB := mustReadFile(t, dbPath(vault))
			var err error
			if command == "add" {
				if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# Root B\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				_, err = Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true})
			} else {
				_, err = Convert(vault, ConvertOptions{ToFormat: "markdown"})
			}
			if err == nil {
				t.Fatal("accepted external rewrite candidate")
			}
			for _, path := range []string{filepath.Join(vault, "A.md"), outside} {
				if got := readTestFile(t, path); got != content {
					t.Fatalf("%s changed: %q", path, got)
				}
			}
			if string(mustReadFile(t, dbPath(vault))) != string(beforeDB) {
				t.Fatal("index changed on rejected rewrite")
			}
		})
	}
}

func TestRewriteApplyAndRestoreRecheckSymlinkBoundary(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "Note.md")
	outside := filepath.Join(t.TempDir(), "Outside.md")
	for _, p := range []string{path, outside} {
		if err := os.WriteFile(p, []byte("[[Old]]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := prepareFileRewrites(vault, []rewriteEntry{{sourcePath: "Note.md", rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := applyPreparedFileRewrites(prepared); err == nil {
		t.Fatal("apply accepted replaced symlink")
	}
	backup := rewriteBackup{path: "Note.md", content: []byte("backup\n"), perm: 0o644}
	if failures := restoreBackupFiles(vault, []rewriteBackup{backup}); len(failures) != 1 {
		t.Fatalf("restore failures = %v", failures)
	}
	if failures := restoreSetBackup(vault, path, "Note.md", setBackup{content: backup.content, perm: backup.perm, modTime: time.Now()}); len(failures) != 1 {
		t.Fatalf("set restore failures = %v", failures)
	}
	if got := readTestFile(t, outside); got != "[[Old]]\n" {
		t.Fatalf("external content changed: %q", got)
	}
	// Missing ordinary files retain the existing rollback recreation behavior.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if failures := restoreBackupFiles(vault, []rewriteBackup{backup}); len(failures) != 0 {
		t.Fatalf("restore missing file: %v", failures)
	}
	if got := readTestFile(t, path); got != "backup\n" {
		t.Fatalf("restored content = %q", got)
	}
}

func TestRestoreRejectsSymlinkEscapeWithMissingTarget(t *testing.T) {
	for _, ancestor := range []bool{false, true} {
		t.Run(map[bool]string{false: "dangling-leaf", true: "ancestor"}[ancestor], func(t *testing.T) {
			vault := t.TempDir()
			outside := t.TempDir()
			missing := filepath.Join(outside, "Missing.md")
			relative := "Missing.md"
			if ancestor {
				relative = "external/Missing.md"
				if err := os.Symlink(outside, filepath.Join(vault, "external")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(missing, filepath.Join(vault, relative)); err != nil {
				t.Fatal(err)
			}
			failures := restoreBackupFiles(vault, []rewriteBackup{{path: relative, content: []byte("backup\n"), perm: 0o644}})
			if len(failures) != 1 {
				t.Fatalf("restore failures = %v", failures)
			}
			if _, err := os.Stat(missing); !os.IsNotExist(err) {
				t.Fatalf("external target created: %v", err)
			}
		})
	}
}

func TestMovedRewriteRejectsExternalSymlink(t *testing.T) {
	vault := t.TempDir()
	outside := filepath.Join(t.TempDir(), "Outside.md")
	if err := os.WriteFile(outside, []byte("[[Old]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(vault, "Note.md")); err != nil {
		t.Fatal(err)
	}
	moved := []movedFileRewrite{{move: moveInfo{from: "Note.md", to: "New.md"}, original: []byte("[[Old]]\n"), content: []byte("[[Old]]\n"), perm: 0o644,
		outRewrites: []outgoingRewrite{{rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1}},
	}}
	if err := prepareMovedFileRewrites(vault, moved, true); err == nil {
		t.Fatal("prepare accepted external moved note")
	}
	if _, _, err := applyMovedFileRewrites(vault, moved, true); err == nil {
		t.Fatal("apply accepted external moved note")
	}
	if got := readTestFile(t, outside); got != "[[Old]]\n" {
		t.Fatalf("external content changed: %q", got)
	}
}

func TestRewriteRollbackRejectsReplacedSymlink(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "Note.md")
	outside := filepath.Join(t.TempDir(), "Outside.md")
	for _, p := range []string{path, outside} {
		if err := os.WriteFile(p, []byte("[[Old]]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldWrite := rewriteWriteFile
	rewriteWriteFile = func(path string, _ []byte, _ os.FileMode) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := os.Symlink(outside, path); err != nil {
			return err
		}
		return os.ErrPermission
	}
	t.Cleanup(func() { rewriteWriteFile = oldWrite })
	_, _, failures, err := applyFileRewritesWithRollbackFailures(vault, []rewriteEntry{{sourcePath: "Note.md", rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1}})
	if err == nil || len(failures) != 1 {
		t.Fatalf("error = %v, restore failures = %v", err, failures)
	}
	if got := readTestFile(t, outside); got != "[[Old]]\n" {
		t.Fatalf("rollback changed external content: %q", got)
	}
}
