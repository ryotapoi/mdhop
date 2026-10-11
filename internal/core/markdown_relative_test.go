package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownSourceRelativeResolution(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{
		"sub/from.md": "[bare](to.md) [dot](./to.md) [nested](path/to.md) [nested dot](./path/to.md) [parent](../to.md) [root](/path/to.md) [missing](missing.md) ![image](pic.png) [reference]\n[reference]: to.md\n[[to]]",
		"sub/to.md":   "", "to.md": "", "other/to.md": "", "sub/path/to.md": "", "path/to.md": "", "missing.md": "", "sub/pic.png": "image", "pic.png": "other image",
	})
	buildVault(t, vault)
	for raw, want := range map[string]string{
		"[bare](to.md)": "sub/to.md", "[dot](./to.md)": "sub/to.md", "[nested](path/to.md)": "sub/path/to.md", "[nested dot](./path/to.md)": "sub/path/to.md", "[parent](../to.md)": "to.md", "[root](/path/to.md)": "path/to.md", "[image](pic.png)": "sub/pic.png", "[reference]": "sub/to.md", "[[to]]": "to.md",
	} {
		got, err := Resolve(vault, "sub/from.md", raw)
		if err != nil || got.Path != want {
			t.Fatalf("%s: %+v %v, want %s", raw, got, err, want)
		}
	}
	got, err := Resolve(vault, "sub/from.md", "[missing](missing.md)")
	if err != nil || got.Type != NodeTypePhantom {
		t.Fatalf("missing relative path: %+v %v", got, err)
	}
}

func TestMarkdownPromotionUsesExactSourcePath(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"sub/from.md": "[target](to.md) [reference]\n[reference]: to.md\n"})
	buildVault(t, vault)
	for _, added := range []string{"other/to.md", "sub/to.md"} {
		writeInterpretationFiles(t, vault, map[string]string{added: ""})
		if _, err := Add(vault, AddOptions{Files: []string{added}, AutoDisambiguate: true}); err != nil {
			t.Fatal(err)
		}
		for _, raw := range []string{"[target](to.md)", "[reference]"} {
			got, err := Resolve(vault, "sub/from.md", raw)
			if err != nil {
				t.Fatal(err)
			}
			if added == "other/to.md" && got.Type != NodeTypePhantom {
				t.Fatalf("wrong promotion: %+v", got)
			}
			if added == "sub/to.md" && got.Path != "sub/to.md" {
				t.Fatalf("exact promotion: %+v", got)
			}
		}
		if _, err := Update(vault, UpdateOptions{Files: []string{"sub/from.md"}}); err != nil {
			t.Fatal(err)
		}
		got, err := Resolve(vault, "sub/from.md", "[target](to.md)")
		if err != nil || added == "other/to.md" && got.Type != NodeTypePhantom || added == "sub/to.md" && got.Path != "sub/to.md" {
			t.Fatalf("update: %+v %v", got, err)
		}
	}
	buildVault(t, vault)
	got, err := Resolve(vault, "sub/from.md", "[target](to.md)")
	if err != nil || got.Path != "sub/to.md" {
		t.Fatalf("full build: %+v %v", got, err)
	}
}

func TestMarkdownMoveAndConversionKeepTargets(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"old/from.md": "[bare](to.md) [missing](later.md) [[unique]]", "old/to.md": "", "else/unique.md": ""})
	buildVault(t, vault)
	if _, err := Move(vault, MoveOptions{From: "old/from.md", To: "new/from.md"}); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"wikilink", "markdown"} {
		if _, err := Convert(vault, ConvertOptions{ToFormat: format}); err != nil {
			t.Fatal(err)
		}
		buildVault(t, vault)
		content, err := os.ReadFile(filepath.Join(vault, "new/from.md"))
		if err != nil {
			t.Fatal(err)
		}
		links := parseLinks(string(content)).Links
		if len(links) != 3 {
			t.Fatalf("links: %s", content)
		}
		for i, want := range []string{"old/to.md", "", "else/unique.md"} {
			got, err := Resolve(vault, "new/from.md", links[i].rawLink)
			if err != nil || want != "" && got.Path != want || want == "" && got.Type != NodeTypePhantom {
				t.Fatalf("%s: %+v %v", links[i].rawLink, got, err)
			}
		}
	}
}

func TestMarkdownRepairAndSimplifyKeepTarget(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"notes/from.md": "[note](broken/to.md) ![image](broken/pic.png) [ambiguous](broken/dup.md)", "notes/to.md": "", "images/pic.png": "image", "one/dup.md": "", "two/dup.md": ""})
	result, err := Repair(vault, RepairOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rewritten) != 2 || len(result.Skipped) != 1 {
		t.Fatalf("repair: %+v", result)
	}
	if _, err := Simplify(vault, SimplifyOptions{}); err != nil {
		t.Fatal(err)
	}
	buildVault(t, vault)
	for raw, want := range map[string]string{"[note](to.md)": "notes/to.md", "[image](../images/pic.png)": "images/pic.png"} {
		got, err := Resolve(vault, "notes/from.md", raw)
		if err != nil || got.Path != want {
			t.Fatalf("%s: %+v %v", raw, got, err)
		}
	}
}

func TestConvertAmbiguousWikilinkUnchanged(t *testing.T) {
	vault := t.TempDir()
	original := "[[duplicate]] [missing](elsewhere.md)"
	writeInterpretationFiles(t, vault, map[string]string{"notes/from.md": original, "one/duplicate.md": "", "two/duplicate.md": ""})
	result, err := Convert(vault, ConvertOptions{ToFormat: "markdown"})
	if err != nil || len(result.Rewritten) != 0 {
		t.Fatalf("conversion: %+v %v", result, err)
	}
	content, err := os.ReadFile(filepath.Join(vault, "notes/from.md"))
	if err != nil || string(content) != original {
		t.Fatalf("ambiguous link changed: %s %v", content, err)
	}
}
