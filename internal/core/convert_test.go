package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

// --- Unit tests ---

func TestConvertMarkdownToWikilink(t *testing.T) {
	tests := []struct {
		name    string
		rawLink string
		want    string
	}{
		{"basic", "[Name](Name.md)", "[[Name]]"},
		{"with subpath", "[Name](Name.md#H)", "[[Name#H]]"},
		{"alias", "[alias](Name.md)", "[[Name|alias]]"},
		{"path", "[Name](path/to/Name.md)", "[[path/to/Name]]"},
		{"path with alias", "[custom](path/to/Name.md)", "[[path/to/Name|custom]]"},
		{"relative", "[Name](./Name.md)", "[[./Name]]"},
		{"asset png", "[img](photo.png)", "[[photo.png|img]]"},
		{"asset with path", "[img](assets/photo.png)", "[[assets/photo.png|img]]"},
		{"self-link", "[#Section](#Section)", "[[#Section]]"},
		{"self-link with alias", "[custom](#Section)", "[[#Section|custom]]"},
		{"URL excluded", "[Google](https://google.com)", "[Google](https://google.com)"},
		{"text matches basename with subpath", "[Name#H](Name.md#H)", "[[Name#H]]"},
		{"subpath alias needed", "[custom](Name.md#H)", "[[Name#H|custom]]"},
		{"asset basename text match", "[photo.png](photo.png)", "[[photo.png]]"},
		{"asset basename in subdir", "[photo.png](assets/photo.png)", "[[assets/photo.png]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertMarkdownToWikilink(tt.rawLink)
			if got != tt.want {
				t.Errorf("convertMarkdownToWikilink(%q) = %q, want %q", tt.rawLink, got, tt.want)
			}
		})
	}
}

func TestConvertWikilinkToMarkdown(t *testing.T) {
	noteNames := map[string]bool{
		"name":    true,
		"deep":    true,
		"note.v1": true,
	}
	isAsset := func(target string) bool {
		return !isNoteTarget(target, noteNames)
	}

	tests := []struct {
		name    string
		rawLink string
		want    string
	}{
		{"basic", "[[Name]]", "[Name](Name.md)"},
		{"with subpath", "[[Name#H]]", "[Name#H](Name.md#H)"},
		{"alias", "[[Name|alias]]", "[alias](Name.md)"},
		{"path", "[[path/to/Name]]", "[Name](path/to/Name.md)"},
		{"relative", "[[./Name]]", "[Name](./Name.md)"},
		{"asset png", "[[photo.png]]", "[photo.png](photo.png)"},
		{"asset with path", "[[assets/photo.png]]", "[photo.png](assets/photo.png)"},
		{"self-link", "[[#Section]]", "[#Section](#Section)"},
		{"self-link with alias", "[[#Section|alias]]", "[alias](#Section)"},
		{"dotted basename", "[[Note.v1]]", "[Note.v1](Note.v1.md)"},
		{"deep with path", "[[sub/Deep]]", "[Deep](sub/Deep.md)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertWikilinkToMarkdown(tt.rawLink, isAsset)
			if got != tt.want {
				t.Errorf("convertWikilinkToMarkdown(%q) = %q, want %q", tt.rawLink, got, tt.want)
			}
		})
	}
}

func TestExtractMarkdownParts(t *testing.T) {
	tests := []struct {
		rawLink  string
		wantText string
		wantURL  string
	}{
		{"[Name](Name.md)", "Name", "Name.md"},
		{"[alias](path/to/Name.md)", "alias", "path/to/Name.md"},
		{"[text](#heading)", "text", "#heading"},
		{"[text](Name.md#H)", "text", "Name.md#H"},
		{"not a link", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.rawLink, func(t *testing.T) {
			gotText, gotURL := extractMarkdownParts(tt.rawLink)
			if gotText != tt.wantText || gotURL != tt.wantURL {
				t.Errorf("extractMarkdownParts(%q) = (%q, %q), want (%q, %q)",
					tt.rawLink, gotText, gotURL, tt.wantText, tt.wantURL)
			}
		})
	}
}

func TestParseMarkdownSelfLinks(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    int
		subpath string
	}{
		{"basic self-link", "[text](#heading)", 1, "#heading"},
		{"no self-link", "[text](Name.md)", 0, ""},
		{"URL", "[Google](https://google.com)", 0, ""},
		{"multiple self-links", "[a](#one) and [b](#two)", 2, ""},
		{"wikilink ignored", "[[#heading]]", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMarkdownSelfLinks(tt.line, 1)
			if len(got) != tt.want {
				t.Errorf("parseMarkdownSelfLinks(%q) returned %d links, want %d", tt.line, len(got), tt.want)
			}
			if tt.want == 1 && len(got) == 1 && got[0].subpath != tt.subpath {
				t.Errorf("subpath = %q, want %q", got[0].subpath, tt.subpath)
			}
		})
	}
}

func TestIsNoteTarget(t *testing.T) {
	noteNames := map[string]bool{
		"note.v1": true,
		"name":    true,
	}

	tests := []struct {
		target string
		want   bool
	}{
		{"Name", true},       // no extension → note
		{"Name.md", true},    // .md → note
		{"photo.png", false}, // .png → asset
		{"Note.v1", true},    // matches noteNameSet
		{"unknown.v2", false},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := isNoteTarget(tt.target, noteNames)
			if got != tt.want {
				t.Errorf("isNoteTarget(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

// --- Integration tests ---

func TestConvertToWikilink(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) == 0 {
		t.Fatal("expected some rewritten links")
	}

	// Check that rewritten links make sense.
	for _, r := range result.Rewritten {
		if !strings.HasPrefix(r.NewLink, "[[") {
			t.Errorf("expected wikilink format, got %q", r.NewLink)
		}
		if !strings.HasPrefix(r.OldLink, "[") {
			t.Errorf("expected markdown link as old, got %q", r.OldLink)
		}
	}

	// Check specific conversion pairs.
	wantPairs := map[string]string{
		"[Target](Target.md)":        "[[Target]]",
		"[Target](Target.md#Heading)": "[[Target#Heading]]",
		"[custom alias](Target.md)":  "[[Target|custom alias]]",
		"[Deep](sub/Deep.md)":        "[[sub/Deep]]",
		"[Sibling](./Sibling.md)":    "[[./Sibling]]",
	}
	for old, want := range wantPairs {
		found := false
		for _, r := range result.Rewritten {
			if r.OldLink == old {
				found = true
				if r.NewLink != want {
					t.Errorf("pair %q: got %q, want %q", old, r.NewLink, want)
				}
			}
		}
		if !found {
			t.Errorf("expected conversion pair for %q not found", old)
		}
	}

	// Verify the file was actually modified.
	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Markdown links should have been converted.
	if strings.Contains(s, "[Target](Target.md)") {
		t.Error("markdown link [Target](Target.md) was not converted")
	}
	// URL should remain.
	if !strings.Contains(s, "[Google](https://google.com)") {
		t.Error("URL link should not be converted")
	}
}

func TestConvertToMarkdown(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "markdown",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) == 0 {
		t.Fatal("expected some rewritten links")
	}

	for _, r := range result.Rewritten {
		if !strings.HasPrefix(r.NewLink, "[") {
			t.Errorf("expected markdown link format, got %q", r.NewLink)
		}
		if !strings.HasPrefix(r.OldLink, "[[") {
			t.Errorf("expected wikilink as old, got %q", r.OldLink)
		}
	}

	// Check specific conversion pairs.
	wantPairs := map[string]string{
		"[[Target]]":               "[Target](Target.md)",
		"[[Target#Heading]]":       "[Target#Heading](Target.md#Heading)",
		"[[Target|custom alias]]":  "[custom alias](Target.md)",
		"[[sub/Deep]]":             "[Deep](sub/Deep.md)",
		"[[./Sibling]]":            "[Sibling](./Sibling.md)",
		"[[photo.png]]":            "[photo.png](photo.png)",
		"[[#Section]]":             "[#Section](#Section)",
	}
	for old, want := range wantPairs {
		found := false
		for _, r := range result.Rewritten {
			if r.OldLink == old {
				found = true
				if r.NewLink != want {
					t.Errorf("pair %q: got %q, want %q", old, r.NewLink, want)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected conversion pair for %q not found", old)
		}
	}

	// Verify the file was actually modified.
	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Wikilinks should have been converted (outside inline code).
	// Check that "- [[Target]]" (the list item) is gone.
	if strings.Contains(s, "- [[Target]]") {
		t.Error("wikilink [[Target]] was not converted")
	}
}

func TestConvertDryRun(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Read original content.
	orig, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) == 0 {
		t.Fatal("expected some rewritten links in dry-run")
	}

	// Verify the file was NOT modified.
	after, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(orig) != string(after) {
		t.Error("dry-run should not modify files")
	}
}

func TestConvertNoMatch(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Target.md has no links to convert.
	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
		Files:    []string{"Target.md"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) != 0 {
		t.Errorf("expected 0 rewritten links, got %d", len(result.Rewritten))
	}
}

func TestConvertMixed(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Convert to wikilink: only markdown links should be converted.
	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if strings.HasPrefix(r.OldLink, "[[") {
			t.Errorf("wikilink %q should not be converted when --to wikilink", r.OldLink)
		}
	}
}

func TestConvertURLExcluded(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if strings.Contains(r.OldLink, "https://") {
			t.Errorf("URL link %q should not be converted", r.OldLink)
		}
	}
}

func TestConvertTagsUntouched(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify tags are preserved.
	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "tags: [test]") {
		t.Error("frontmatter tags should be preserved")
	}

	// No tag should appear in rewritten.
	for _, r := range result.Rewritten {
		if strings.HasPrefix(r.OldLink, "#") {
			t.Errorf("tag %q should not be converted", r.OldLink)
		}
	}
}

func TestConvertCodeFence(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify code fence content is preserved.
	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "[Link](Link.md) should not change") {
		t.Error("code fence content should be preserved")
	}
}

func TestConvertInlineCode(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(tmp, ConvertOptions{
		ToFormat: "markdown",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify inline code content is preserved.
	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "`[[Target]]`") {
		t.Error("inline code content should be preserved")
	}
}

func TestConvertFileScope(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
		Files:    []string{"Note.md"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.File != "Note.md" {
			t.Errorf("expected file scope to Note.md, got %q", r.File)
		}
	}
}

func TestConvertFileScopeExcluded(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
		Files:    []string{"nonexistent.md"},
	})
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "not found or excluded") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConvertFileScopeBuildExcluded(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Create mdhop.yaml that excludes Note.md.
	err := os.WriteFile(filepath.Join(tmp, "mdhop.yaml"), []byte("build:\n  exclude_paths:\n    - Note.md\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Specifying an excluded file should produce an error.
	_, err = Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
		Files:    []string{"Note.md"},
	})
	if err == nil {
		t.Fatal("expected error for build-excluded file")
	}
	if !strings.Contains(err.Error(), "not found or excluded") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConvertAsset(t *testing.T) {
	// Test wikilink → markdown for asset.
	noteNames := map[string]bool{
		"note": true,
	}
	isAsset := func(target string) bool {
		return !isNoteTarget(target, noteNames)
	}

	got := convertWikilinkToMarkdown("[[photo.png]]", isAsset)
	if got != "[photo.png](photo.png)" {
		t.Errorf("asset wikilink → markdown: got %q, want %q", got, "[photo.png](photo.png)")
	}

	// Test markdown → wikilink for asset.
	got2 := convertMarkdownToWikilink("[img](photo.png)")
	if got2 != "[[photo.png|img]]" {
		t.Errorf("asset markdown → wikilink: got %q, want %q", got2, "[[photo.png|img]]")
	}

	// Asset with matching text.
	got3 := convertMarkdownToWikilink("[photo.png](photo.png)")
	if got3 != "[[photo.png]]" {
		t.Errorf("asset markdown → wikilink (text match): got %q, want %q", got3, "[[photo.png]]")
	}
}

func TestConvertSelfLink(t *testing.T) {
	// wikilink → markdown
	noteNames := map[string]bool{}
	isAsset := func(target string) bool {
		return !isNoteTarget(target, noteNames)
	}

	got := convertWikilinkToMarkdown("[[#Section]]", isAsset)
	if got != "[#Section](#Section)" {
		t.Errorf("self-link wikilink → markdown: got %q, want %q", got, "[#Section](#Section)")
	}

	got2 := convertWikilinkToMarkdown("[[#Section|alias]]", isAsset)
	if got2 != "[alias](#Section)" {
		t.Errorf("self-link with alias wikilink → markdown: got %q, want %q", got2, "[alias](#Section)")
	}

	// markdown → wikilink
	got3 := convertMarkdownToWikilink("[#Section](#Section)")
	if got3 != "[[#Section]]" {
		t.Errorf("self-link markdown → wikilink: got %q, want %q", got3, "[[#Section]]")
	}

	got4 := convertMarkdownToWikilink("[custom](#Section)")
	if got4 != "[[#Section|custom]]" {
		t.Errorf("self-link with alias markdown → wikilink: got %q, want %q", got4, "[[#Section|custom]]")
	}
}

func TestConvertBuildExclude(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Create mdhop.yaml that excludes Note.md.
	err := os.WriteFile(filepath.Join(tmp, "mdhop.yaml"), []byte("build:\n  exclude_paths:\n    - Note.md\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.File == "Note.md" {
			t.Error("excluded file Note.md should not be scanned")
		}
	}
}

func TestConvertDottedBasename(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Convert wikilinks to markdown.
	result, err := Convert(tmp, ConvertOptions{
		ToFormat: "markdown",
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find the Note.v1 conversion.
	found := false
	for _, r := range result.Rewritten {
		if r.OldLink == "[[Note.v1]]" {
			found = true
			if r.NewLink != "[Note.v1](Note.v1.md)" {
				t.Errorf("dotted basename: got %q, want %q", r.NewLink, "[Note.v1](Note.v1.md)")
			}
		}
	}
	if !found {
		t.Error("expected [[Note.v1]] to be converted")
	}
}

func TestConvertRelativePath(t *testing.T) {
	// markdown → wikilink preserves relative prefix.
	got := convertMarkdownToWikilink("[Name](./Name.md)")
	if got != "[[./Name]]" {
		t.Errorf("relative markdown → wikilink: got %q, want %q", got, "[[./Name]]")
	}

	// wikilink → markdown preserves relative prefix.
	noteNames := map[string]bool{"name": true}
	isAsset := func(target string) bool {
		return !isNoteTarget(target, noteNames)
	}
	got2 := convertWikilinkToMarkdown("[[./Name]]", isAsset)
	if got2 != "[Name](./Name.md)" {
		t.Errorf("relative wikilink → markdown: got %q, want %q", got2, "[Name](./Name.md)")
	}
}

func TestConvertRoundTrip(t *testing.T) {
	noteNames := map[string]bool{
		"name":    true,
		"note.v1": true,
	}
	isAsset := func(target string) bool {
		return !isNoteTarget(target, noteNames)
	}

	tests := []struct {
		name    string
		mdLink  string
		wikiLink string
	}{
		{"basic", "[Name](Name.md)", "[[Name]]"},
		{"alias", "[custom](Name.md)", "[[Name|custom]]"},
		{"subpath", "[Name#H](Name.md#H)", "[[Name#H]]"},
		{"path", "[Name](path/to/Name.md)", "[[path/to/Name]]"},
		{"self-link", "[#Section](#Section)", "[[#Section]]"},
		{"self-link alias", "[custom](#Section)", "[[#Section|custom]]"},
		{"asset", "[photo.png](photo.png)", "[[photo.png]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" md→wiki→md", func(t *testing.T) {
			wiki := convertMarkdownToWikilink(tt.mdLink)
			if wiki != tt.wikiLink {
				t.Errorf("md→wiki: got %q, want %q", wiki, tt.wikiLink)
			}
			md := convertWikilinkToMarkdown(wiki, isAsset)
			if md != tt.mdLink {
				t.Errorf("wiki→md roundtrip: got %q, want %q", md, tt.mdLink)
			}
		})

		t.Run(tt.name+" wiki→md→wiki", func(t *testing.T) {
			md := convertWikilinkToMarkdown(tt.wikiLink, isAsset)
			if md != tt.mdLink {
				t.Errorf("wiki→md: got %q, want %q", md, tt.mdLink)
			}
			wiki := convertMarkdownToWikilink(md)
			if wiki != tt.wikiLink {
				t.Errorf("md→wiki roundtrip: got %q, want %q", wiki, tt.wikiLink)
			}
		})
	}
}

func TestConvertEmbedPreserved(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

	// Convert wikilink embeds to markdown.
	_, err := Convert(tmp, ConvertOptions{
		ToFormat: "markdown",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "Note.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// ![[photo.png]] should become ![photo.png](photo.png).
	if !strings.Contains(s, "![photo.png](photo.png)") {
		t.Errorf("embed wikilink should become markdown embed, got:\n%s", s)
	}
}
