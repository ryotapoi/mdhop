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
		for _, name := range []string{"External.md", "external-dir/Note.md", "Café.md", "Note.md", "Internal.md", "internal-dir/Note.md"} {
			if _, err := resolver.writablePath(name); err != nil {
				t.Fatalf("%s: rejected filesystem path %s: %v", root, name, err)
			}
		}
		if _, err := resolver.writablePath("../Note.md"); err == nil {
			t.Fatal("accepted logical escape")
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
			root := vault
			if !external {
				root = filepath.Join(t.TempDir(), "vault")
				if err := os.Symlink(vault, root); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Set(root, SetOptions{File: "Link.md", Key: "status", Value: "done"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(readTestFile(t, target), "status: done") {
				t.Fatal("target was not updated")
			}
		})
	}
}

func TestRewriteCommandsAllowExternalSymlink(t *testing.T) {
	for _, command := range []string{"add", "convert", "convert-root-link"} {
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
			var err error
			if command == "add" {
				if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# Root B\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				_, err = Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true})
			} else {
				root := vault
				if command == "convert-root-link" {
					root = filepath.Join(t.TempDir(), "vault")
					if err := os.Symlink(vault, root); err != nil {
						t.Fatal(err)
					}
				}
				_, err = Convert(root, ConvertOptions{ToFormat: "markdown"})
			}
			if err != nil {
				t.Fatal(err)
			}
			want := "[B](/sub/B.md)\n"
			if command == "add" {
				want = "[[sub/B]]\n"
			}
			for _, path := range []string{filepath.Join(vault, "A.md"), outside} {
				if got := readTestFile(t, path); got != want {
					t.Fatalf("%s = %q; want %q", path, got, want)
				}
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
	if _, _, _, err := applyPreparedFileRewrites(prepared); err != nil {
		t.Fatal(err)
	}
	backup := rewriteBackup{path: "Note.md", content: []byte("backup\n"), perm: 0o644, mtime: time.Unix(1234567890, 0)}
	if failures := restoreBackupFiles(vault, []rewriteBackup{backup}); len(failures) != 0 {
		t.Fatalf("restore failures = %v", failures)
	}
	if failures := restoreSetBackup(vault, path, "Note.md", setBackup{content: backup.content, perm: backup.perm, modTime: time.Now()}); len(failures) != 0 {
		t.Fatalf("set restore failures = %v", failures)
	}
	if got := readTestFile(t, outside); got != "backup\n" {
		t.Fatalf("restored external content = %q", got)
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

func TestRestoreMissingTargetFilesystemSemantics(t *testing.T) {
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
			failures := restoreBackupFiles(vault, []rewriteBackup{{path: relative, content: []byte("backup\n"), perm: 0o644, mtime: time.Unix(1234567890, 0)}})
			if ancestor {
				if len(failures) != 0 {
					t.Fatalf("restore failures = %v", failures)
				}
				if got := readTestFile(t, missing); got != "backup\n" {
					t.Fatalf("restored content = %q", got)
				}
			} else {
				if len(failures) != 1 {
					t.Fatalf("dangling leaf failures = %v", failures)
				}
				if _, err := os.Stat(missing); !os.IsNotExist(err) {
					t.Fatalf("dangling target created: %v", err)
				}
			}
		})
	}
}

func TestMovedRewriteAllowsExternalSymlink(t *testing.T) {
	vault := t.TempDir()
	outside := filepath.Join(t.TempDir(), "Outside.md")
	if err := os.WriteFile(outside, []byte("[[Old]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(vault, "Note.md")); err != nil {
		t.Fatal(err)
	}
	moved := []movedFileRewrite{{move: moveInfo{from: "Note.md", to: "New.md", fromDisk: filepath.Join(vault, "Note.md"), toDisk: filepath.Join(vault, "New.md")}, original: []byte("[[Old]]\n"), content: []byte("[[Old]]\n"), perm: 0o644,
		outRewrites: []outgoingRewrite{{rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1}},
	}}
	if err := prepareMovedFileRewrites(vault, moved, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := applyMovedFileRewrites(vault, moved, true); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, outside); got != "[[New]]\n" {
		t.Fatalf("external content changed: %q", got)
	}
}

func TestRewriteRollbackRestoresThroughReplacedSymlink(t *testing.T) {
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
	if err == nil || len(failures) != 0 {
		t.Fatalf("error = %v, restore failures = %v", err, failures)
	}
	if got := readTestFile(t, outside); got != "[[Old]]\n" {
		t.Fatalf("rollback changed external content: %q", got)
	}
}
