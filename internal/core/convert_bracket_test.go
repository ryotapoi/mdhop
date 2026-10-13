package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertBracketDisplay(t *testing.T) {
	for _, tt := range []struct{ wiki, markdown string }{
		{`[[Target|a]b]]`, `[a\]b](Target.md)`},
		{`[[Target|a[b]]`, `[a\[b](Target.md)`},
		{`[[Target|a\]b]]`, `[a\]b](Target.md)`},
		{`[[Target|a\\]b]]`, `[a\\\]b](Target.md)`},
		{`[[Target|a\\\]b]]`, `[a\\\]b](Target.md)`},
		{`[[Target|a\]]`, `[a\\](Target.md)`},
		{`[[#H|a]b]]`, `[a\]b](#H)`},
		{`[[A[B]]`, `[A\[B](A[B.md)`},
	} {
		t.Run(tt.wiki, func(t *testing.T) {
			got := convertWikilinkToMarkdown(tt.wiki, nil)
			if got != tt.markdown {
				t.Fatalf("conversion = %q, want %q", got, tt.markdown)
			}
			text, destination := extractMarkdownParts(got)
			if text == "" || destination == "" {
				t.Fatalf("parts lost: %q / %q", text, destination)
			}
			links := parseLinksForConvert(got).Links
			if len(links) != 1 || links[0].rawLink != got {
				t.Fatalf("reparsed links = %+v", links)
			}
			wantTarget := "Target"
			if tt.wiki == `[[#H|a]b]]` {
				wantTarget = ""
			} else if tt.wiki == `[[A[B]]` {
				wantTarget = "A[B"
			}
			if links[0].target != wantTarget {
				t.Fatalf("target = %q, want %q", links[0].target, wantTarget)
			}
		})
	}
}

func TestConvertBracketThenBuild(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[[Target|a]b]]\n![[Target|a[b]]\n| col |\n| --- |\n| [[Target\\|a]b]] |\n[[#H|a]b]]\n", "Target.md": "target"})
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[a\\]b](/Target.md)\n![a\\[b](/Target.md)\n| col |\n| --- |\n| [a\\]b](/Target.md) |\n[a\\]b](#H)\n"
	if string(content) != want {
		t.Fatalf("content = %q, want %q", content, want)
	}
	buildVault(t, vault)
	if got, err := Resolve(vault, "Source.md", `[a\]b](/Target.md)`); err != nil || got.Path != "Target.md" {
		t.Fatalf("resolve = %+v, %v", got, err)
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil || string(content) != want {
		t.Fatalf("reconversion = %q, %v", content, err)
	}
	if _, err := Move(vault, MoveOptions{From: "Target.md", To: "Renamed.md"}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := parseLinks(string(content)).Links; len(got) != 3 || got[0].target != "/Renamed" {
		t.Fatalf("rewritten links = %+v", got)
	}
}

func TestMarkdownBracketBoundaries(t *testing.T) {
	for _, tt := range []struct {
		content string
		want    []string
	}{
		{`[a\]b](Target.md)`, []string{`[a\]b](Target.md)`}},
		{`[a\\]b](Target.md) [later](Later.md)`, []string{`[later](Later.md)`}},
		{`[a\\\]b](Target.md)`, []string{`[a\\\]b](Target.md)`}},
		{`[a\[b](Target.md)`, []string{`[a\[b](Target.md)`}},
		{`[a[b]c](Target.md)`, []string{`[a[b]c](Target.md)`}},
		{"[a `]` b](Target.md)", []string{"[a `]` b](Target.md)"}},
		{"[guide]: Target.md\n[a\\]b][guide] [later](Later.md)", []string{`[later](Later.md)`}},
		{"[guide]: Target.md\n[a\\[b\nend][guide] [later](Later.md)", []string{`[later](Later.md)`}},
		{"[guide]: Target.md\n[a\n\\]b][guide] [later](Later.md)", []string{`[later](Later.md)`}},
	} {
		t.Run(tt.content, func(t *testing.T) {
			got := parseLinks(tt.content).Links
			if len(got) != len(tt.want) {
				t.Fatalf("links = %+v, want %v", got, tt.want)
			}
			for i, raw := range tt.want {
				if got[i].rawLink != raw {
					t.Fatalf("link = %+v, want %q", got[i], raw)
				}
			}
		})
	}
	for _, tt := range []struct {
		content string
		count   int
	}{
		{`\[a\]b](#H)`, 0}, {`\\[a\]b](#H)`, 1},
	} {
		if got := parseMarkdownSelfLinks(tt.content, 1); len(got) != tt.count {
			t.Fatalf("self links %q = %+v", tt.content, got)
		}
	}
}

func TestMarkdownUnmatchedBracketKeepsLaterLink(t *testing.T) {
	for _, content := range []string{
		`[literal\] [later](Target.md)`,
		`[literal\] [a\]b](Target.md)`,
	} {
		got := parseLinks(content).Links
		if len(got) != 1 || got[0].target != "Target" {
			t.Fatalf("links %q = %+v", content, got)
		}
		if spans := bodyLinkSpans(content, got[0].rawLink, LinkTypeMarkdown); len(spans) != 1 {
			t.Fatalf("spans = %+v", spans)
		}
	}
	if got := parseMarkdownSelfLinks(`[literal\] [a\]b](#H)`, 1); len(got) != 1 {
		t.Fatalf("self links = %+v", got)
	}
	// A closed unsupported multiline label must still consume its inner link.
	for _, content := range []string{
		"[literal\\] [later](Target.md)\nend][id]\n[id]: Target.md",
		"[literal\\] [later](Target.md)][id]\n[id]: Target.md",
	} {
		if got := parseLinks(content).Links; len(got) != 0 {
			t.Fatalf("unsupported reference = %+v", got)
		}
	}
}

func TestConvertUnmatchedBracketThenBuild(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[literal\\] [[Target|a]b]]\n[literal\\] [[#H|a]b]]\n", "Target.md": "target"})
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	want := "[literal\\] [a\\]b](/Target.md)\n[literal\\] [a\\]b](#H)\n"
	if got := string(mustReadFile(t, filepath.Join(vault, "Source.md"))); got != want {
		t.Fatalf("content = %q", got)
	}
	buildVault(t, vault)
	if got, err := Resolve(vault, "Source.md", `[a\]b](/Target.md)`); err != nil || got.Path != "Target.md" {
		t.Fatalf("resolve = %+v, %v", got, err)
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadFile(t, filepath.Join(vault, "Source.md"))); got != want {
		t.Fatalf("reconvert = %q", got)
	}
	if _, err := Move(vault, MoveOptions{From: "Target.md", To: "Renamed.md"}); err != nil {
		t.Fatal(err)
	}
	got := string(mustReadFile(t, filepath.Join(vault, "Source.md")))
	if got != "[literal\\] [a\\]b](/Renamed.md)\n[literal\\] [a\\]b](#H)\n" {
		t.Fatalf("rewrite = %q", got)
	}
}
