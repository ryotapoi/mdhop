package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMoveTypeChangeRejected(t *testing.T) {
	for _, tc := range []struct{ from, to string }{
		{"Source.MD", "new/Source.txt"},
		{"image.png", "new/image.Md"},
	} {
		for _, alreadyMoved := range []bool{false, true} {
			name := tc.from + " to " + tc.to
			if alreadyMoved {
				name += " already moved"
			}
			t.Run(name, func(t *testing.T) {
				vault := newMoveVault(t, map[string]string{
					"Source.MD": "---\ntitle: Source\ntags: [project]\n---\n[[Target]]\n![[image.png]]\n#active\n",
					"image.png": "asset bytes",
					"Target.md": "# Target\n",
					"User.md":   "[[Source]]\n![[image.png]]\n",
				})
				if alreadyMoved {
					if err := os.MkdirAll(filepath.Join(vault, "new"), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(filepath.Join(vault, tc.from), filepath.Join(vault, tc.to)); err != nil {
						t.Fatal(err)
					}
				}
				before := snapshotAssetMoveVault(t, vault)
				_, err := Move(vault, MoveOptions{From: tc.from, To: tc.to})
				if err == nil || !strings.Contains(err.Error(), "cannot change note/asset type") || !strings.Contains(err.Error(), "rename externally") || !strings.Contains(err.Error(), "mdhop build") {
					t.Fatalf("error = %v, want type rejection and rebuild guidance", err)
				}
				if after := snapshotAssetMoveVault(t, vault); !reflect.DeepEqual(after, before) {
					t.Fatal("vault or index changed after rejected move")
				}
			})
		}
	}
}

func TestMoveTemplateTypeChangeRejectsEntireBatch(t *testing.T) {
	for _, operation := range []string{"plan", "execution"} {
		t.Run(operation, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{
				"src/A.md": "---\ndestination: Good.MD\ntags: [project]\n---\n[[B]]\n",
				"src/B.md": "---\ndestination: Bad.txt\n---\n[[A]]\n",
				"User.md":  "[[A]]\n[[B]]\n",
			})
			before := snapshotAssetMoveVault(t, vault)
			opts := MoveTemplateOptions{From: "src", Template: "new/{destination}", Directory: true}
			var err error
			if operation == "plan" {
				_, err = PlanMoveTemplate(vault, opts)
			} else {
				_, err = MoveTemplate(vault, opts)
			}
			if err == nil || !strings.Contains(err.Error(), "cannot change note/asset type") {
				t.Fatalf("error = %v, want type rejection", err)
			}
			if after := snapshotAssetMoveVault(t, vault); !reflect.DeepEqual(after, before) {
				t.Fatal("vault or index changed after rejected template batch")
			}
		})
	}
}

func TestMoveSameTypePreservedAfterBuild(t *testing.T) {
	for _, tc := range []struct {
		from, to, link string
		wantType       NodeType
	}{
		{"Source.md", "new/Renamed.MD", "[[Source]]", NodeTypeNote},
		{"Source.MD", "new/Renamed.Md", "[[Source]]", NodeTypeNote},
		{"Source.Md", "new/Renamed.md", "[[Source]]", NodeTypeNote},
		{"image.png", "new/picture.bin", "![[image.png]]", NodeTypeAsset},
	} {
		t.Run(tc.from+" to "+tc.to, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{tc.from: "content\n", "User.md": tc.link + "\n"})
			if _, err := Move(vault, MoveOptions{From: tc.from, To: tc.to}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(vault, tc.from)); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			if content, err := os.ReadFile(filepath.Join(vault, tc.to)); err != nil || string(content) != "content\n" {
				t.Fatalf("destination content = %q, error = %v", content, err)
			}
			check := func() {
				t.Helper()
				content, err := os.ReadFile(filepath.Join(vault, "User.md"))
				if err != nil {
					t.Fatal(err)
				}
				links := parseLinks(string(content)).Links
				if len(links) != 1 {
					t.Fatalf("incoming links = %v, want one", links)
				}
				result, err := Resolve(vault, "User.md", links[0].rawLink)
				if err != nil || result.Path != tc.to || result.Type != tc.wantType {
					t.Fatalf("resolution = %+v, error = %v, want %s %s", result, err, tc.wantType, tc.to)
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
