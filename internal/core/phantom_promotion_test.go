package core

import (
	"reflect"
	"testing"
)

func TestPhantomPromotionMatchesBuild(t *testing.T) {
	for _, operation := range []string{"add", "move", "move-dir", "move-asset"} {
		t.Run(operation, func(t *testing.T) {
			name, extension := "X", ".md"
			if operation == "move-asset" {
				name, extension = "X.png", ""
			}
			source := "---\nrelated: \"[[missing/" + name + "]]\"\n---\n[[" + name + "#Heading|alias]]\n[[missing/" + name + "]]\n[[other/" + name + "]]\n[right](./" + name + extension + "#heading)\n[wrong](/missing/" + name + extension + ")\n"
			oldName := name
			if operation == "move" {
				oldName = "Y"
			}
			if operation == "move-asset" {
				oldName = "Y.png"
			}
			files := map[string]string{"other/A.md": source}
			if operation != "add" {
				files["old/"+oldName+extension] = "# target\n"
			}
			if operation == "move-dir" {
				delete(files, "other/A.md")
				files["old/A.md"] = source
			}
			vault := newMoveVault(t, files)
			switch operation {
			case "add":
				writeVaultFile(t, vault, "other/"+name+extension, "# target\n")
				result, err := Add(vault, AddOptions{Files: []string{"other/" + name + extension}})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result.Promoted, []string{"other/" + name + extension}) {
					t.Fatalf("Promoted = %v", result.Promoted)
				}
			case "move-dir":
				if _, err := MoveDir(vault, MoveDirOptions{FromDir: "old", ToDir: "other"}); err != nil {
					t.Fatal(err)
				}
			default:
				if _, err := Move(vault, MoveOptions{From: "old/" + oldName + extension, To: "other/" + name + extension}); err != nil {
					t.Fatal(err)
				}
			}
			for _, edge := range queryEdges(t, dbPath(vault), "other/A.md") {
				if edge.rawLink == "[[missing/"+name+"]]" || edge.rawLink == "[wrong](/missing/"+name+extension+")" {
					if edge.targetType != NodeTypePhantom {
						t.Errorf("%s targets %s, want phantom", edge.rawLink, edge.targetKey)
					}
				} else if edge.targetKey != noteKey("other/"+name+extension) && operation != "move-asset" {
					t.Errorf("%s targets %s, want other target", edge.rawLink, edge.targetKey)
				}
			}
			before := indexSnapshot(t, vault)
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			if after := indexSnapshot(t, vault); !reflect.DeepEqual(before, after) {
				t.Fatalf("incremental graph differs from build:\nincremental: %v\nbuild: %v", before, after)
			}
		})
	}
}

func TestAddPhantomPromotionMultiplePaths(t *testing.T) {
	vault := newMoveVault(t, map[string]string{"A.md": "[[X]]\n[[sub/X]]\n[[missing/X]]\n"})
	writeVaultFile(t, vault, "X.md", "# root\n")
	writeVaultFile(t, vault, "sub/X.md", "# sub\n")
	result, err := Add(vault, AddOptions{Files: []string{"sub/X.md", "X.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Promoted) != 2 {
		t.Fatalf("Promoted = %v, want both paths", result.Promoted)
	}
	want := map[string]string{"[[X]]": noteKey("X.md"), "[[sub/X]]": noteKey("sub/X.md"), "[[missing/X]]": phantomKey("X")}
	for _, edge := range queryEdges(t, dbPath(vault), "A.md") {
		if edge.targetKey != want[edge.rawLink] {
			t.Errorf("%s targets %s, want %s", edge.rawLink, edge.targetKey, want[edge.rawLink])
		}
	}
	before := indexSnapshot(t, vault)
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if after := indexSnapshot(t, vault); !reflect.DeepEqual(before, after) {
		t.Fatalf("incremental graph differs from build:\n%v\n%v", before, after)
	}
}

func TestAddPhantomPromotionUnresolvedOnly(t *testing.T) {
	vault := newMoveVault(t, map[string]string{"A.md": "[[missing/X]]\n"})
	writeVaultFile(t, vault, "other/X.md", "# target\n")
	result, err := Add(vault, AddOptions{Files: []string{"other/X.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Promoted) != 0 {
		t.Fatalf("Promoted = %v, want empty", result.Promoted)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	if len(edges) != 1 || edges[0].targetType != NodeTypePhantom {
		t.Fatalf("unresolved link lost phantom: %v", edges)
	}
}
