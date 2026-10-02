package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

func TestConvertReportsRollbackFailure(t *testing.T) {
	vault := t.TempDir()
	for _, name := range []string{"One.md", "Two.md"} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte("[A](A.md)\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	oldRewriteWriteFile := rewriteWriteFile
	writeCalls := 0
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 2 {
			return errors.New("primary rewrite blocked")
		}
		return oldRewriteWriteFile(path, data, perm)
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(string, []byte, os.FileMode) error {
		return errors.New("restore blocked")
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"})
	assertRollbackFailureReported(t, err, "primary rewrite blocked", "restore blocked")
}

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
		{"mailto excluded", "[Email](mailto:user@example.com)", "[Email](mailto:user@example.com)"},
		{"FTP excluded", "[FTP](FTP://example.com/file)", "[FTP](FTP://example.com/file)"},
		{"hierarchical URI excluded", "[Repo](git+ssh://example.com/repo)", "[Repo](git+ssh://example.com/repo)"},
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
		return !isNoteTarget(target, "", noteNames, nil)
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
		"note.v1":   true,
		"name":      true,
		"photo.png": true,
	}
	notePaths := map[string]bool{
		"notes/photo.png.md": true,
	}

	tests := []struct {
		target string
		want   bool
	}{
		{"Name", true},       // no extension → note
		{"Name.md", true},    // .md → note
		{"image.png", false}, // .png → asset
		{"Note.v1", true},    // matches noteNameSet
		{"unknown.v2", false},
		{"photo.png", true},            // dotted basename remains a note
		{"assets/photo.png", false},    // explicit asset path ignores other note basenames
		{"notes/photo.png", true},      // vault-relative note path
		{"/notes/PHOTO.PNG", true},     // root-prefixed, case-insensitive note path
		{"./photo.png", true},          // source-relative note path
		{"../assets/photo.png", false}, // source-relative asset path
		{"./notes/photo.png", false},   // relative path is not vault-relative
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := isNoteTarget(tt.target, "notes/Source.md", noteNames, notePaths)
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
		if strings.HasPrefix(r.OldLink, "[[") {
			t.Errorf("existing wikilink %q should not be converted", r.OldLink)
		}
	}

	// Check specific conversion pairs.
	wantPairs := map[string]string{
		"[Target](Target.md)":         "[[Target]]",
		"[Target](Target.md#Heading)": "[[Target#Heading]]",
		"[custom alias](Target.md)":   "[[Target|custom alias]]",
		"[Deep](sub/Deep.md)":         "[[sub/Deep]]",
		"[Sibling](./Sibling.md)":     "[[./Sibling]]",
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

func TestConvertResolvesNFCPathToNFDPath(t *testing.T) {
	tmp := t.TempDir()
	nfdPath := "Cafe\u0301.md"
	if err := os.WriteFile(filepath.Join(tmp, nfdPath), []byte("[Target](Target.md)\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "Target.md"), []byte("# Target\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	result, err := Convert(tmp, ConvertOptions{ToFormat: "wikilink"})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(result.Rewritten) != 1 || result.Rewritten[0].File != "Caf\u00e9.md" {
		t.Fatalf("rewritten = %+v, want NFC source path", result.Rewritten)
	}
	content, err := os.ReadFile(filepath.Join(tmp, nfdPath))
	if err != nil {
		t.Fatalf("read NFD note: %v", err)
	}
	if got := string(content); got != "[[Target]]\n" {
		t.Fatalf("content = %q, want converted wikilink", got)
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
		"[[Target]]":              "[Target](Target.md)",
		"[[Target#Heading]]":      "[Target#Heading](Target.md#Heading)",
		"[[Target|custom alias]]": "[custom alias](Target.md)",
		"[[sub/Deep]]":            "[Deep](sub/Deep.md)",
		"[[./Sibling]]":           "[Sibling](./Sibling.md)",
		"[[photo.png]]":           "[photo.png](photo.png)",
		"[[#Section]]":            "[#Section](#Section)",
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

	notePath := filepath.Join(tmp, "Note.md")
	note, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	note = append(note, []byte("\n~~~~\n[Link](Link.md) [section](#Section) in a tilde fence\n~~~~\n")...)
	if err := os.WriteFile(notePath, note, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = Convert(tmp, ConvertOptions{
		ToFormat: "wikilink",
		DryRun:   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify code fence content is preserved.
	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "[Link](Link.md) should not change") {
		t.Error("code fence content should be preserved")
	}
	if !strings.Contains(string(content), "[Link](Link.md) [section](#Section) in a tilde fence") {
		t.Error("tilde fence content should be preserved")
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

func TestConvertBuildExclude(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_convert", tmp); err != nil {
		t.Fatal(err)
	}

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

func TestConvertExplicitAssetPath(t *testing.T) {
	vault := t.TempDir()
	for path, content := range map[string]string{
		"assets/photo.png":   "asset",
		"notes/photo.png.md": "# Photo\n",
		"notes/Source.md":    "[[assets/photo.png]]\n[[../assets/photo.png|image]]\n[[./photo.png]]\n[[/notes/photo.png]]\n",
	} {
		fullPath := filepath.Join(vault, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Convert(vault, ConvertOptions{ToFormat: "markdown", DryRun: true, Files: []string{"notes/Source.md"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"[[assets/photo.png]]":          "[photo.png](assets/photo.png)",
		"[[../assets/photo.png|image]]": "[image](../assets/photo.png)",
		"[[./photo.png]]":               "[photo.png](./photo.png.md)",
		"[[/notes/photo.png]]":          "[photo.png](/notes/photo.png.md)",
	}
	if len(result.Rewritten) != len(want) {
		t.Fatalf("got %d rewrites, want %d", len(result.Rewritten), len(want))
	}
	for _, rewrite := range result.Rewritten {
		if expected, ok := want[rewrite.OldLink]; !ok || rewrite.NewLink != expected {
			t.Errorf("rewrite %q = %q, want %q", rewrite.OldLink, rewrite.NewLink, expected)
		}
	}
}

func TestConvertEmbedPreserved(t *testing.T) {
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

func TestConvertParenthesesThenBuild(t *testing.T) {
	vault := t.TempDir()
	original := "[[Meeting (weekly)]]\n"
	for name, content := range map[string]string{"Source.md": original, "Meeting (weekly).md": "# Meeting\n"} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	wantRaw := "[Meeting (weekly)](Meeting (weekly).md)"
	content, err := os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != wantRaw+"\n" {
		t.Fatalf("converted content = %q, want %q", content, wantRaw+"\n")
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	edges := queryEdges(t, dbPath(vault), "Source.md")
	if len(edges) != 1 {
		t.Fatalf("edges = %+v, want one existing note edge", edges)
	}
	edge := edges[0]
	if edge.targetKey != "note:path:Meeting (weekly).md" || edge.targetType != NodeTypeNote || edge.linkType != LinkTypeMarkdown || edge.rawLink != wantRaw {
		t.Fatalf("edge = %+v, want Markdown edge to existing Meeting (weekly).md with raw link %q", edge, wantRaw)
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(vault, "Source.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("round-trip content = %q, want %q", content, original)
	}
}
