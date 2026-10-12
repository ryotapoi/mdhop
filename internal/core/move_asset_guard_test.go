package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Snapshot all entries, including index bytes, to detect partial rewrites,
// build-state changes, mkdirs, renames, or temporary files on rejection.
func snapshotAssetMoveVault(t *testing.T, vault string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	err := filepath.WalkDir(vault, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(vault, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			entries[rel] = "directory"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	entries["@index"] = string(index)
	return entries
}

func TestMoveAssetReferenceShadowRejected(t *testing.T) {
	cases := []struct{ name, user, other string }{
		{"basename embed", "![[image.png]]\n[[Other]]\n", "# Other\n"},
		{"vault path", "![[/image.png]]\n[[Other]]\n", "# Other\n"},
		{"relative markdown", "![image](image.png)\n[[Other]]\n", "# Other\n"},
		{"moved source", "[[Other]]\n", "![[image.png]]\n"},
		{"table context", "| image |\n| --- |\n| ![[image.png\\|preview]] |\n[[Other]]\n", "# Other\n"},
		{"frontmatter wikilink", "---\nimage: '[[image.png]]'\n---\n[[Other]]\n", "# Other\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{"image.png": "asset", "Other.md": tc.other, "User.md": tc.user})
			before := snapshotAssetMoveVault(t, vault)
			_, err := Move(vault, MoveOptions{From: "Other.md", To: "image.png.md"})
			if err == nil || !strings.Contains(err.Error(), "asset link") {
				t.Fatalf("error = %v, want asset protection rejection", err)
			}
			if after := snapshotAssetMoveVault(t, vault); !reflect.DeepEqual(after, before) {
				t.Fatal("vault or index changed after rejected move")
			}
		})
	}
}

func TestMoveAssetReferenceBatchShadowRejected(t *testing.T) {
	for _, operation := range []string{"directory", "template plan", "template execution", "ambiguous notes"} {
		t.Run(operation, func(t *testing.T) {
			files := map[string]string{"src/image.png.md": "# Other\n", "src/Good.md": "# Good\n", "dest/image.png": "asset", "User.md": "![image](dest/image.png)\n[[Good]]\n"}
			if operation == "ambiguous notes" {
				files = map[string]string{"src/A.md": "---\nfolder: one\n---\n", "src/B.md": "---\nfolder: two\n---\n", "image.png": "asset", "User.md": "![[image.png]]\n"}
			}
			vault := newMoveVault(t, files)
			before := snapshotAssetMoveVault(t, vault)
			var err error
			switch operation {
			case "directory":
				_, err = MoveDir(vault, MoveDirOptions{FromDir: "src", ToDir: "dest"})
			case "template plan":
				_, err = PlanMoveTemplate(vault, MoveTemplateOptions{From: "src", Template: "dest/{basename}", Directory: true})
			case "template execution":
				_, err = MoveTemplate(vault, MoveTemplateOptions{From: "src", Template: "dest/{basename}", Directory: true})
			case "ambiguous notes":
				_, err = MoveTemplate(vault, MoveTemplateOptions{From: "src", Template: "{folder}/image.png.md", Directory: true})
			}
			if err == nil || !(strings.Contains(err.Error(), "asset link") || strings.Contains(err.Error(), "ambiguous")) {
				t.Fatalf("move should reject changed asset resolution: %v", err)
			}
			if after := snapshotAssetMoveVault(t, vault); !reflect.DeepEqual(after, before) {
				t.Fatal("vault or index changed after rejected batch")
			}
		})
	}
}

func TestMoveAssetReferencesPreservedAfterFullBuild(t *testing.T) {
	for _, operation := range []string{"source", "asset", "batch"} {
		t.Run(operation, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{"old/User.md": "![image](./image.png)\n![[image.png]]\n", "old/image.png": "asset", "Outside.md": "![[image.png]]\n"})
			source, asset := "old/User.md", "old/image.png"
			var err error
			switch operation {
			case "source":
				_, err = Move(vault, MoveOptions{From: source, To: "new/User.md"})
				source = "new/User.md"
			case "asset":
				_, err = Move(vault, MoveOptions{From: asset, To: "new/picture.png"})
				asset = "new/picture.png"
			case "batch":
				_, err = MoveDir(vault, MoveDirOptions{FromDir: "old", ToDir: "new"})
				source = "new/User.md"
				asset = "new/image.png"
			}
			if err != nil {
				t.Fatalf("ordinary move failed: %v", err)
			}
			check := func() {
				t.Helper()
				for _, from := range []string{source, "Outside.md"} {
					content, err := os.ReadFile(filepath.Join(vault, from))
					if err != nil {
						t.Fatal(err)
					}
					for _, link := range parseLinks(string(content)).Links {
						result, err := Resolve(vault, from, link.rawLink)
						if err != nil || result.Type != NodeTypeAsset || result.Path != asset {
							t.Fatalf("%s %s resolved to %+v, err=%v, want asset %s", from, link.rawLink, result, err, asset)
						}
					}
				}
			}
			check()
			if _, err := BuildFull(vault); err != nil {
				t.Fatal(err)
			}
			check()
		})
	}
}
