package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMoveDestinationRejectsExternalAncestorBeforeMutation(t *testing.T) {
	for _, mode := range []string{"single", "directory", "disk-only", "template-plan", "template-execute"} {
		t.Run(mode, func(t *testing.T) {
			files := map[string]string{
				"src/A.md":    "---\nbucket: safe\n---\n[[../Target]]\n",
				"Target.md":   "# Target\n",
				"Incoming.md": "[[src/A]]\n",
			}
			if mode != "disk-only" {
				files["src/late/Z.md"] = "---\nbucket: late\n---\n[[../../Target]]\n"
				files["Incoming.md"] += "[[src/late/Z]]\n"
			}
			vault := newMoveVault(t, files)
			outside := t.TempDir()
			if mode == "disk-only" {
				if err := os.MkdirAll(filepath.Join(vault, "src/late"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(vault, "src/late/data.bin"), []byte("unregistered"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(vault, "dest"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(vault, "dest/late")); err != nil {
				t.Fatal(err)
			}
			// Include file bytes, directories, symlinks and the SQLite file so that
			// partial moves, rewrites, index updates and temporary files are visible.
			snapshot := func(root string) map[string]string {
				t.Helper()
				state := make(map[string]string)
				err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(root, path)
					if err != nil {
						return err
					}
					switch {
					case entry.Type()&os.ModeSymlink != 0:
						target, err := os.Readlink(path)
						if err != nil {
							return err
						}
						state[rel] = "symlink:" + target
					case entry.IsDir():
						state[rel] = "directory"
					default:
						content, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						state[rel] = "file:" + string(content)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				return state
			}
			before := snapshot(vault)
			outsideBefore := snapshot(outside)
			var err error
			switch mode {
			case "single":
				_, err = Move(vault, MoveOptions{From: "src/late/Z.md", To: "dest/late/new/deep/Z.md"})
			case "directory", "disk-only":
				_, err = MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dest"})
			case "template-plan":
				_, err = PlanMoveTemplate(vault, MoveTemplateOptions{From: "src", Directory: true, Template: "dest/{bucket}/new/deep/{basename}"})
			case "template-execute":
				_, err = MoveTemplate(vault, MoveTemplateOptions{From: "src", Directory: true, Template: "dest/{bucket}/new/deep/{basename}"})
			}
			if err == nil || !strings.Contains(err.Error(), "resolves outside vault") {
				t.Fatalf("error = %v, want destination boundary rejection", err)
			}
			if !reflect.DeepEqual(snapshot(vault), before) {
				t.Fatal("rejected move changed the vault or index")
			}
			if !reflect.DeepEqual(snapshot(outside), outsideBefore) {
				t.Fatal("rejected move created external files or directories")
			}
		})
	}
}

func TestMoveDestinationAllowsInternalAncestorAndSymlinkRoot(t *testing.T) {
	vault := newMoveVault(t, map[string]string{"A.md": "# A\n"})
	if err := os.Mkdir(filepath.Join(vault, "actual"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(vault, "actual"), filepath.Join(vault, "dest")); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(vault, root); err != nil {
		t.Fatal(err)
	}
	if _, err := Move(root, MoveOptions{From: "A.md", To: "dest/new/deep/A.md"}); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(vault, "actual/new/deep/A.md")); got != "# A\n" {
		t.Fatalf("moved content = %q", got)
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM nodes WHERE path = 'dest/new/deep/A.md'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("destination nodes = %d, want 1", count)
	}
}

func TestWritablePathMissingAncestorBoundary(t *testing.T) {
	vault := t.TempDir()
	if err := os.Symlink(filepath.Join(vault, "missing"), filepath.Join(vault, "dangling")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"dangling", "dangling/new/deep/A.md"} {
		if err := validateVaultWritePath(vault, filepath.Join(vault, path)); err == nil {
			t.Fatalf("accepted dangling symlink: %s", path)
		}
	}
	if err := validateVaultWritePath(vault, filepath.Join(vault, "new/deep/A.md")); err != nil {
		t.Fatalf("rejected missing ordinary ancestors: %v", err)
	}
}
