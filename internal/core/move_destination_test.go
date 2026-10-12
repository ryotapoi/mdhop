package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMoveDestinationAllowsExternalAncestor(t *testing.T) {
	for _, mode := range []string{"single", "directory", "disk-only", "template-plan", "template-execute"} {
		t.Run(mode, func(t *testing.T) {
			files := map[string]string{
				"src/A.md":    "---\nbucket: late\n---\n[Target](../Target.md)\n",
				"Target.md":   "# Target\n",
				"Incoming.md": "[[src/A]]\n",
			}
			vault := newMoveVault(t, files)
			outside := t.TempDir()
			if mode == "disk-only" {
				writeVaultFile(t, vault, "src/late/data.bin", "unregistered")
			}
			if mode == "directory" {
				files["src/late/Z.md"] = "# Z\n"
				writeVaultFile(t, vault, "src/late/Z.md", files["src/late/Z.md"])
				if _, err := Build(vault); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(vault, "dest"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(vault, "dest/late")); err != nil {
				t.Fatal(err)
			}
			dbBefore := readTestFile(t, dbPath(vault))
			var err error
			switch mode {
			case "single":
				_, err = Move(vault, MoveOptions{From: "src/A.md", To: "dest/late/new/deep/A.md"})
			case "directory", "disk-only":
				_, err = MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dest"})
			case "template-plan":
				_, err = PlanMoveTemplate(vault, MoveTemplateOptions{From: "src", Directory: true, Template: "dest/{bucket}/new/deep/{basename}"})
			case "template-execute":
				_, err = MoveTemplate(vault, MoveTemplateOptions{From: "src", Directory: true, Template: "dest/{bucket}/new/deep/{basename}"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "template-plan" {
				for path, want := range files {
					if got := readTestFile(t, filepath.Join(vault, path)); got != want {
						t.Fatalf("plan changed %s", path)
					}
				}
				entries, err := os.ReadDir(outside)
				if err != nil || len(entries) != 0 {
					t.Fatalf("plan changed destination: %v, %v", entries, err)
				}
				if got := readTestFile(t, dbPath(vault)); got != dbBefore {
					t.Fatal("plan changed DB")
				}
				return
			}
			if fileExists(filepath.Join(vault, "src/A.md")) {
				t.Fatal("source remains")
			}
			if mode == "directory" || mode == "disk-only" {
				if got := readTestFile(t, filepath.Join(vault, "dest/A.md")); got != files["src/A.md"] {
					t.Fatalf("moved content = %q", got)
				}
			} else {
				if got := readTestFile(t, filepath.Join(outside, "new/deep/A.md")); !strings.Contains(got, "../../../../Target.md") {
					t.Fatalf("outgoing link = %q", got)
				}
			}
			if mode == "directory" {
				if got := readTestFile(t, filepath.Join(outside, "Z.md")); got != "# Z\n" {
					t.Fatalf("moved content = %q", got)
				}
			}
			if mode == "disk-only" {
				if got := readTestFile(t, filepath.Join(outside, "data.bin")); got != "unregistered" {
					t.Fatalf("disk-only content = %q", got)
				}
			}
		})
	}
}

func TestMoveDirAliasedDestinationCollision(t *testing.T) {
	for _, mode := range []string{"internal", "external", "disk-only", "rollback-failure", "distinct-names"} {
		t.Run(mode, func(t *testing.T) {
			second := "X.md"
			if mode == "distinct-names" || mode == "disk-only" {
				second = "Y.md"
			}
			files := map[string]string{
				"src/one/X.md":      "# One\n[Target](../../Target.md)\n",
				"src/two/" + second: "# Two\n[Target](../../Target.md)\n",
				"Target.md":         "# Target\n",
				"Incoming.md":       "[[src/one/X]]\n[[src/two/" + strings.TrimSuffix(second, ".md") + "]]\n",
			}
			vault := newMoveVault(t, files)
			target := t.TempDir()
			if mode == "internal" {
				target = filepath.Join(vault, "actual")
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "disk-only" {
				// Distinct registered names let the two disk-only files reach rename.
				writeVaultFile(t, vault, "src/one/data.bin", "one")
				writeVaultFile(t, vault, "src/two/data.bin", "two")
				files["src/one/data.bin"] = "one"
				files["src/two/data.bin"] = "two"
			}
			for _, dir := range []string{"one", "two"} {
				if err := os.MkdirAll(filepath.Join(vault, "dest/deep"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(vault, "dest/deep", dir)); err != nil {
					t.Fatal(err)
				}
			}
			nodesBefore := queryNodes(t, dbPath(vault), NodeTypeNote)
			edgesBefore := make(map[string][]edgeRow)
			for path := range files {
				edgesBefore[path] = queryEdges(t, dbPath(vault), path)
			}
			if mode == "rollback-failure" {
				oldRename := moveRename
				moveRename = func(from, to string) error {
					if strings.HasPrefix(from, filepath.Join(vault, "dest/deep")+string(filepath.Separator)) {
						return errors.New("rollback rename blocked")
					}
					return oldRename(from, to)
				}
				t.Cleanup(func() { moveRename = oldRename })
			}
			result, err := MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dest/deep"})
			if mode == "distinct-names" {
				if err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"X.md", "Y.md"} {
					if !fileExists(filepath.Join(target, name)) {
						t.Fatalf("missing %s", name)
					}
				}
				return
			}
			if result != nil || !errors.Is(err, ErrAlreadyExistsOnDisk) {
				t.Fatalf("result = %v, error = %v, want collision", result, err)
			}
			if mode == "rollback-failure" {
				for _, want := range []string{"rollback failed", "could not move back dest/deep/one/X.md -> src/one/X.md", "rollback rename blocked", "mdhop build"} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("error missing %q: %v", want, err)
					}
				}
				return
			}
			for path, want := range files {
				if got := readTestFile(t, filepath.Join(vault, path)); got != want {
					t.Fatalf("%s not restored: %q", path, got)
				}
				if got := queryEdges(t, dbPath(vault), path); !reflect.DeepEqual(got, edgesBefore[path]) {
					t.Fatalf("%s edges changed: %v", path, got)
				}
			}
			if got := queryNodes(t, dbPath(vault), NodeTypeNote); !reflect.DeepEqual(got, nodesBefore) {
				t.Fatalf("nodes changed: %v", got)
			}
			for _, name := range []string{"X.md", "Y.md", "data.bin"} {
				if fileExists(filepath.Join(target, name)) {
					t.Fatalf("destination remains: %s", name)
				}
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
