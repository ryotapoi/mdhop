package core

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writeStatusFile(t *testing.T, vault, path, content string) {
	t.Helper()
	fullPath := filepath.Join(vault, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("make parent: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestStatus_ClassifiesDifferencesWithoutChangingIndex(t *testing.T) {
	vault := t.TempDir()
	writeStatusFile(t, vault, "Keep.md", "keep\n")
	writeStatusFile(t, vault, "Modified.md", "modified\n")
	writeStatusFile(t, vault, "Gone.png", "gone")
	writeStatusFile(t, vault, "draft/Old.md", "old\n")
	writeStatusFile(t, vault, "Phantom.md", "[[Missing]]\n")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	writeStatusFile(t, vault, "mdhop.toml", "[build]\nexclude_paths = ['draft/*']\n")
	writeStatusFile(t, vault, "New.md", "new\n")
	writeStatusFile(t, vault, "Extra.png", "extra")
	writeStatusFile(t, vault, "draft/New.md", "excluded\n")
	modified := filepath.Join(vault, "Modified.md")
	changedAt := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(modified, changedAt, changedAt); err != nil {
		t.Fatalf("change mtime: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "Gone.png")); err != nil {
		t.Fatalf("remove indexed asset: %v", err)
	}

	beforeDB, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatalf("read database before status: %v", err)
	}
	beforeModified, err := os.ReadFile(modified)
	if err != nil {
		t.Fatalf("read modified file before status: %v", err)
	}

	result, err := Status(vault)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if want := []string{"Extra.png", "New.md", "mdhop.toml"}; !reflect.DeepEqual(result.Untracked, want) {
		t.Errorf("untracked = %#v, want %#v", result.Untracked, want)
	}
	if want := []string{"Modified.md"}; !reflect.DeepEqual(result.Modified, want) {
		t.Errorf("modified = %#v, want %#v", result.Modified, want)
	}
	if want := []string{"Gone.png"}; !reflect.DeepEqual(result.Deleted, want) {
		t.Errorf("deleted = %#v, want %#v", result.Deleted, want)
	}

	afterDB, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatalf("read database after status: %v", err)
	}
	if !bytes.Equal(beforeDB, afterDB) {
		t.Errorf("status changed index database")
	}
	afterModified, err := os.ReadFile(modified)
	if err != nil {
		t.Fatalf("read modified file after status: %v", err)
	}
	if !bytes.Equal(beforeModified, afterModified) {
		t.Errorf("status changed vault file")
	}
}

func TestStatus_HandlesNormalizedPathsAndDirectoryReplacement(t *testing.T) {
	vault := t.TempDir()
	decomposed := "Cafe\u0301.md"
	writeStatusFile(t, vault, decomposed, "cafe\n")
	writeStatusFile(t, vault, "Replaced.md", "file\n")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "Replaced.md")); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(vault, "Replaced.md"), 0o755); err != nil {
		t.Fatalf("replace with directory: %v", err)
	}

	result, err := Status(vault)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if want := []string{"Replaced.md"}; !reflect.DeepEqual(result.Deleted, want) {
		t.Errorf("deleted = %#v, want %#v", result.Deleted, want)
	}
	if len(result.Untracked) != 0 || len(result.Modified) != 0 {
		t.Errorf("unexpected differences: %#v", result)
	}
}

func TestStatus_CaseOnlyRenameIsDeletedAndUntracked(t *testing.T) {
	vault := t.TempDir()
	writeStatusFile(t, vault, "Case.md", "case\n")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.Rename(filepath.Join(vault, "Case.md"), filepath.Join(vault, "case.md")); err != nil {
		t.Fatalf("case-only rename: %v", err)
	}

	result, err := Status(vault)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if want := []string{"case.md"}; !reflect.DeepEqual(result.Untracked, want) {
		t.Errorf("untracked = %#v, want %#v", result.Untracked, want)
	}
	if want := []string{"Case.md"}; !reflect.DeepEqual(result.Deleted, want) {
		t.Errorf("deleted = %#v, want %#v", result.Deleted, want)
	}
	if len(result.Modified) != 0 {
		t.Errorf("modified = %#v, want empty", result.Modified)
	}
}

func TestStatus_NoIndexDoesNotCreateDatabase(t *testing.T) {
	vault := t.TempDir()
	_, err := Status(vault)
	if err == nil || err.Error() != "index not found: run 'mdhop build' first" {
		t.Fatalf("status error = %v", err)
	}
	if _, err := os.Stat(dbPath(vault)); !os.IsNotExist(err) {
		t.Fatalf("status created database, stat error = %v", err)
	}
}

func TestStatus_TracksTerminalSymlinks(t *testing.T) {
	for _, directory := range []bool{true, false} {
		name := "file"
		if directory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			vault := t.TempDir()
			targetVault := t.TempDir()
			target := filepath.Join(targetVault, "target")
			if directory {
				writeStatusFile(t, targetVault, "target/Child.md", "child\n")
			} else {
				writeStatusFile(t, targetVault, "target", "asset")
			}
			linkName := "linked-" + name
			link := filepath.Join(vault, linkName)
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("create symlink: %v", err)
			}
			linkInfo, err := os.Lstat(link)
			if err != nil {
				t.Fatalf("stat symlink: %v", err)
			}
			// Keep the target and link mtimes in different seconds so comparing
			// the link's own mtime against the indexed target mtime would fail.
			indexedAt := time.Unix(linkInfo.ModTime().Unix()-100, 0)
			if err := os.Chtimes(target, indexedAt, indexedAt); err != nil {
				t.Fatalf("set target mtime: %v", err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatalf("build: %v", err)
			}
			db, err := openDBChecked(vault)
			if err != nil {
				t.Fatalf("open index: %v", err)
			}
			defer db.Close()
			var nodeType string
			var mtime int64
			if err := db.QueryRow("SELECT type, mtime FROM nodes WHERE path = ?", linkName).Scan(&nodeType, &mtime); err != nil {
				t.Fatalf("read indexed symlink: %v", err)
			}
			if nodeType != "asset" || mtime != indexedAt.Unix() {
				t.Fatalf("indexed symlink = (%s, %d), want (asset, %d)", nodeType, mtime, indexedAt.Unix())
			}
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type IN ('note', 'asset')").Scan(&count); err != nil {
				t.Fatalf("count indexed inputs: %v", err)
			}
			if count != 1 {
				t.Fatalf("indexed inputs = %d, want only the terminal symlink", count)
			}
			result, err := Status(vault)
			if err != nil {
				t.Fatalf("status after build: %v", err)
			}
			if len(result.Untracked) != 0 || len(result.Modified) != 0 || len(result.Deleted) != 0 {
				t.Fatalf("status after build = %#v, want clean", result)
			}
			changedAt := indexedAt.Add(time.Second)
			if err := os.Chtimes(link, changedAt, changedAt); err != nil {
				t.Fatalf("change target mtime through symlink: %v", err)
			}
			result, err = Status(vault)
			if err != nil {
				t.Fatalf("status after target change: %v", err)
			}
			if !reflect.DeepEqual(result.Modified, []string{linkName}) || len(result.Untracked) != 0 || len(result.Deleted) != 0 {
				t.Errorf("status after target change = %#v, want only symlink modified", result)
			}
			if !directory {
				if err := os.Remove(target); err != nil {
					t.Fatalf("remove file target: %v", err)
				}
				result, err = Status(vault)
				if err != nil {
					t.Fatalf("status after target removal: %v", err)
				}
				if !reflect.DeepEqual(result.Deleted, []string{linkName}) || len(result.Untracked) != 0 || len(result.Modified) != 0 {
					t.Errorf("status after target removal = %#v, want only symlink deleted", result)
				}
				writeStatusFile(t, targetVault, "target", "asset")
			}
			if err := os.Remove(link); err != nil {
				t.Fatalf("remove symlink: %v", err)
			}
			result, err = Status(vault)
			if err != nil {
				t.Fatalf("status after symlink removal: %v", err)
			}
			if !reflect.DeepEqual(result.Deleted, []string{linkName}) || len(result.Untracked) != 0 || len(result.Modified) != 0 {
				t.Errorf("status after symlink removal = %#v, want only symlink deleted", result)
			}
		})
	}
}
