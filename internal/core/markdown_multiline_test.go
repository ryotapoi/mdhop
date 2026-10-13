package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownMultilineOccurrences(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(newline, func(t *testing.T) {
			inline := "[長い `code [x]` 説明" + newline + "続き](B.md#H)"
			full := "[display" + newline + "continued][long" + newline + "label]"
			collapsed := "[long" + newline + "label][]"
			shortcut := "[long" + newline + "label]"
			content := strings.Join([]string{"[long label]: B.md", inline, full, collapsed, shortcut}, newline)
			links := parseLinks(content).Links
			if len(links) != 4 {
				t.Fatalf("links: %+v", links)
			}
			for i, raw := range []string{inline, full, collapsed, shortcut} {
				lo := links[i]
				start, end := 2, 3
				if i == 1 {
					start, end = 4, 6
				}
				if i == 2 {
					start, end = 7, 8
				}
				if i == 3 {
					start, end = 9, 10
				}
				if lo.rawLink != raw || lo.target != "B" || lo.lineStart != start || lo.lineEnd != end {
					t.Fatalf("link %d: %+v", i, lo)
				}
				if i == 0 && lo.subpath != "#H" {
					t.Fatal(lo)
				}
			}
		})
	}
}

func TestMarkdownRecoveryAndParagraphBoundaries(t *testing.T) {
	for _, prose := range []string{"[ unfinished ", "[ unfinished\n", "[bad](unclosed ", "[bad](unclosed\n"} {
		content := "[id]: B.md\n" + prose + "[inline](B.md) [text][id] [id][] [id]"
		var raws []string
		for _, lo := range parseLinks(content).Links {
			raws = append(raws, lo.rawLink)
		}
		if want := []string{"[inline](B.md)", "[text][id]", "[id][]", "[id]"}; !reflect.DeepEqual(raws, want) {
			t.Fatalf("%q: %v", prose, raws)
		}
	}
	for _, boundary := range []string{"\n\n", "\n```\ncode\n```\n", "\n[id]: B.md\n"} {
		if links := parseLinks("[start" + boundary + "end](B.md)").Links; len(links) != 0 {
			t.Fatalf("boundary %q: %+v", boundary, links)
		}
	}
	for _, content := range []string{"`[not](B.md)`\n~~~\n[not\nlink](B.md)\n~~~", "[text](B.md\n)", "[known]: B.md\n[known\n][missing]", "[id]: B.md\n[outer\n[inner]][id] [es\\caped\nlabel][id]"} {
		if links := parseLinks(content).Links; len(links) != 0 {
			t.Fatalf("unsupported %q: %+v", content, links)
		}
	}
	// Exercise long unmatched openings without a timing-dependent assertion.
	text := strings.Repeat("[", 20000) + " [valid\ncontinued](B.md)"
	if links := parseLinks(text).Links; len(links) != 1 || links[0].rawLink != "[valid\ncontinued](B.md)" {
		t.Fatalf("long unmatched: %+v", links)
	}
	long := "[" + strings.Repeat("long display\n", 2000) + "end](B.md)"
	if links := parseLinks(long).Links; len(links) != 1 || links[0].rawLink != long || links[0].lineEnd != 2001 {
		t.Fatalf("long multiline: %d occurrences", len(links))
	}

}

func TestMultilineMoveResolveAndRebuild(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(newline, func(t *testing.T) {
			vault := t.TempDir()
			raw := "[long `code`" + newline + "[[B]] continued](B.md#H)"
			second := "[other" + newline + "label](B.md)"
			content := raw + " " + second + newline + "`[untouched](B.md)`" + newline
			writeInterpretationFiles(t, vault, map[string]string{"Source.md": content, "B.md": "# H"})
			buildVault(t, vault)
			got, err := ResolveAtLine(vault, "Source.md", raw, 1)
			if err != nil || got.Path != "B.md" || got.Subpath != "#H" {
				t.Fatalf("resolve: %+v %v", got, err)
			}
			if _, err := ResolveAtLine(vault, "Source.md", raw, 2); !errors.Is(err, ErrLinkNotFound) {
				t.Fatal(err)
			}
			if _, err := Move(vault, MoveOptions{From: "B.md", To: "C.md"}); err != nil {
				t.Fatal(err)
			}
			want := "[long `code`" + newline + "[[C]] continued](/C.md#H) [other" + newline + "label](/C.md)" + newline + "`[untouched](B.md)`" + newline
			if actual := readVaultFile(t, vault, "Source.md"); actual != want {
				t.Fatalf("content %q, want %q", actual, want)
			}
			incremental := indexSnapshot(t, vault)
			if _, err := build(vault, true); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(incremental, indexSnapshot(t, vault)) {
				t.Fatal("incremental index differs from full build")
			}
			if err := os.WriteFile(filepath.Join(vault, "Source.md"), []byte("\n"+want), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); err != nil {
				t.Fatal(err)
			}
			updatedRaw := "[long `code`" + newline + "[[C]] continued](/C.md#H)"
			if _, err := ResolveAtLine(vault, "Source.md", updatedRaw, 1); !errors.Is(err, ErrLinkNotFound) {
				t.Fatal(err)
			}
			if got, err := ResolveAtLine(vault, "Source.md", updatedRaw, 2); err != nil || got.Path != "C.md" {
				t.Fatalf("updated resolve %+v %v", got, err)
			}
		})
	}
}

func TestMultilineReferenceMutationGuard(t *testing.T) {
	vault := referenceVault(t, map[string]string{"Source.md": "[long\nlabel][guide]\n[guide]: B.md", "B.md": ""})
	raw := "[long\nlabel][guide]"
	if got, err := ResolveAtLine(vault, "Source.md", raw, 1); err != nil || got.Path != "B.md" {
		t.Fatalf("reference resolve %+v %v", got, err)
	}
	before := indexSnapshot(t, vault)
	content := readVaultFile(t, vault, "Source.md")
	if _, err := Move(vault, MoveOptions{From: "B.md", To: "C.md"}); err == nil {
		t.Fatal("reference move succeeded")
	}
	if content != readVaultFile(t, vault, "Source.md") || !reflect.DeepEqual(before, indexSnapshot(t, vault)) {
		t.Fatal("rejected move changed state")
	}
	if _, err := os.Stat(filepath.Join(vault, "B.md")); err != nil {
		t.Fatal(err)
	}
}

func TestMultilineConvertPreservesUnrepresentableDisplay(t *testing.T) {
	vault := t.TempDir()
	content := "[long `code`\nlabel](B.md) [long\nself](#H) [single `code`](#H)\n"
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": content, "B.md": ""})
	if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
		t.Fatal(err)
	}
	want := "[long `code`\nlabel](B.md) [long\nself](#H) [[#H|single `code`]]\n"
	if got := readVaultFile(t, vault, "Source.md"); got != want {
		t.Fatalf("converted %q", got)
	}
	buildVault(t, vault)
	if got, err := ResolveAtLine(vault, "Source.md", "[long `code`\nlabel](B.md)", 1); err != nil || got.Path != "B.md" {
		t.Fatalf("convert/build %+v %v", got, err)
	}
}

func TestMultilineInterpretationRebuild(t *testing.T) {
	vault := t.TempDir()
	raw := "[long\nlabel](B.md)"
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": raw, "B.md": ""})
	buildVault(t, vault)
	db := openTestDB(t, dbPath(vault))
	if _, err := db.Exec("PRAGMA user_version = 9"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM edges"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{
		func() error { _, e := ResolveAtLine(vault, "Source.md", raw, 1); return e },
		func() error { _, e := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); return e },
		func() error { _, e := Move(vault, MoveOptions{From: "B.md", To: "C.md"}); return e },
	} {
		if err := operation(); err == nil || !strings.Contains(err.Error(), "mdhop build") {
			t.Fatalf("old gate: %v", err)
		}
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("old index mutated")
	}
	buildVault(t, vault)
	if got, err := ResolveAtLine(vault, "Source.md", raw, 1); err != nil || got.Path != "B.md" {
		t.Fatalf("rebuild %+v %v", got, err)
	}
}

func TestMultilineMovePreservesBodyBoundaries(t *testing.T) {
	for _, prefix := range []struct{ name, text string }{
		{"blank", "[unfinished\n\n"},
		{"fence", "[unfinished\n```\n[ignored](B.md)\n```\n"},
		{"definition", "[unfinished\n[unused]: Other.md\n"},
		{"frontmatter", "---\ntitle: '[unfinished'\n---\n"},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(prefix.name+newline, func(t *testing.T) {
				vault := t.TempDir()
				raw := "[long\nlabel](B.md)"
				content := strings.ReplaceAll(prefix.text+raw+"\n]\n", "\n", newline)
				raw = strings.ReplaceAll(raw, "\n", newline)
				writeInterpretationFiles(t, vault, map[string]string{"Source.md": content, "B.md": ""})
				buildVault(t, vault)
				line := 1 + strings.Count(prefix.text, "\n")
				if got, err := ResolveAtLine(vault, "Source.md", raw, line); err != nil || got.Path != "B.md" {
					t.Fatalf("before move: %+v %v", got, err)
				}
				if _, err := Move(vault, MoveOptions{From: "B.md", To: "C.md"}); err != nil {
					t.Fatal(err)
				}
				newRaw := strings.ReplaceAll(raw, "(B.md)", "(/C.md)")
				want := strings.Replace(content, raw, newRaw, 1)
				if got := readVaultFile(t, vault, "Source.md"); got != want {
					t.Fatalf("source %q, want %q", got, want)
				}
				if got, err := ResolveAtLine(vault, "Source.md", newRaw, line); err != nil || got.Path != "C.md" {
					t.Fatalf("after move: %+v %v", got, err)
				}
				incremental := indexSnapshot(t, vault)
				if _, err := build(vault, true); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(incremental, indexSnapshot(t, vault)) {
					t.Fatal("move index differs from full build")
				}
			})
		}
	}
}
