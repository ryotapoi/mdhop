package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
	"github.com/ryotapoi/mdhop/internal/testutil"
)

func TestSimplifyBasic(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	// Check that unique path links are rewritten.
	found := map[string]string{} // old → new
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	// Wikilink: [[sub/B]] → [[B]]
	if got, ok := found["[[sub/B]]"]; !ok || got != "[[B]]" {
		t.Errorf("expected [[sub/B]] → [[B]], got %q (ok=%v)", got, ok)
	}
	// Markdown: [text](sub/C.md) → [text](C.md)
	if got, ok := found["[text](sub/C.md)"]; !ok || got != "[text](C.md)" {
		t.Errorf("expected [text](sub/C.md) → [text](C.md), got %q (ok=%v)", got, ok)
	}
}

func TestSimplifyAsset(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	if got, ok := found["[[images/photo.png]]"]; !ok || got != "[[photo.png]]" {
		t.Errorf("expected [[images/photo.png]] → [[photo.png]], got %q (ok=%v)", got, ok)
	}
}

func TestSimplifySkippedAmbiguous(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	// [[dir1/M]] should be skipped because M exists in dir1 and dir2.
	var skippedRawLink string
	for _, s := range result.Skipped {
		if s.RawLink == "[[dir1/M]]" {
			skippedRawLink = s.RawLink
			if len(s.Candidates) != 2 {
				t.Errorf("expected 2 candidates, got %d: %v", len(s.Candidates), s.Candidates)
			}
		}
	}
	if skippedRawLink == "" {
		t.Error("expected [[dir1/M]] in skipped list")
	}

	// Ensure it's not in rewritten.
	for _, r := range result.Rewritten {
		if r.OldLink == "[[dir1/M]]" {
			t.Error("[[dir1/M]] should not be rewritten")
		}
	}
}

func TestSimplifySkippedAmbiguousAsset(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, s := range result.Skipped {
		if s.RawLink == "[[assets1/icon.png]]" {
			found = true
			if len(s.Candidates) != 2 {
				t.Errorf("expected 2 candidates, got %d: %v", len(s.Candidates), s.Candidates)
			}
		}
	}
	if !found {
		t.Error("expected [[assets1/icon.png]] in skipped list")
	}
}

func TestSimplifyRelativePath(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		if r.File == "deep/D.md" {
			found[r.OldLink] = r.NewLink
		}
	}

	if got, ok := found["[[../sub/B]]"]; !ok || got != "[[B]]" {
		t.Errorf("expected [[../sub/B]] → [[B]], got %q (ok=%v)", got, ok)
	}
	if got, ok := found["[[./E]]"]; !ok || got != "[[E]]" {
		t.Errorf("expected [[./E]] → [[E]], got %q (ok=%v)", got, ok)
	}
}

func TestSimplifyDryRun(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	// Read original content.
	origA, _ := os.ReadFile(filepath.Join(tmp, "A.md"))
	origD, _ := os.ReadFile(filepath.Join(tmp, "deep/D.md"))

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) == 0 {
		t.Fatal("expected some rewritten links")
	}

	// Files should be unchanged.
	afterA, _ := os.ReadFile(filepath.Join(tmp, "A.md"))
	afterD, _ := os.ReadFile(filepath.Join(tmp, "deep/D.md"))
	if string(afterA) != string(origA) {
		t.Error("A.md was modified during dry-run")
	}
	if string(afterD) != string(origD) {
		t.Error("deep/D.md was modified during dry-run")
	}
}

func TestSimplifyBasenameUntouched(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.OldLink == "[[B]]" {
			t.Error("basename link [[B]] should not be rewritten")
		}
	}
}

func TestSimplifyInlineCodeIgnored(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	// Run actual (non-dry-run) simplify.
	_, err := core.Simplify(tmp, core.SimplifyOptions{})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(tmp, "A.md"))
	if got := string(content); !strings.Contains(got, "`[[sub/B]]`") {
		t.Error("inline code [[sub/B]] should be preserved")
	}
}

func TestSimplifySubpathPreserved(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	if got, ok := found["[[sub/B#Heading]]"]; !ok || got != "[[B#Heading]]" {
		t.Errorf("expected [[sub/B#Heading]] → [[B#Heading]], got %q (ok=%v)", got, ok)
	}
}

func TestSimplifyAliasPreserved(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	if got, ok := found["[[sub/B|alias]]"]; !ok || got != "[[B|alias]]" {
		t.Errorf("expected [[sub/B|alias]] → [[B|alias]], got %q (ok=%v)", got, ok)
	}
}

func TestSimplifyMarkdownFragment(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	if got, ok := found["[text](sub/B.md#section)"]; !ok || got != "[text](B.md#section)" {
		t.Errorf("expected [text](sub/B.md#section) → [text](B.md#section), got %q (ok=%v)", got, ok)
	}
}

func TestSimplifySelfLinkSkipped(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.OldLink == "[[#Heading]]" {
			t.Error("self-link [[#Heading]] should not be rewritten")
		}
	}
}

func TestSimplifyRootPriority(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify_root", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	// [[sub/B]] points to sub/B.md. root B.md exists → basename [[B]] would resolve to root.
	// Since sub/B != root B, this should NOT be simplified. Skip silently.
	if _, ok := found["[[sub/B]]"]; ok {
		t.Error("[[sub/B]] should not be simplified (points to non-root, but basename resolves to root)")
	}

	// [[sub2/B]] should also NOT be simplified for the same reason.
	if _, ok := found["[[sub2/B]]"]; ok {
		t.Error("[[sub2/B]] should not be simplified")
	}

	// Neither should appear in Skipped (intentional path links).
	for _, s := range result.Skipped {
		if s.RawLink == "[[sub/B]]" || s.RawLink == "[[sub2/B]]" {
			t.Errorf("root-priority non-root link should not be in skipped: %s", s.RawLink)
		}
	}
}

func TestSimplifyRootPriorityAsset(t *testing.T) {
	tmp := t.TempDir()
	// Create a minimal vault with root-priority asset scenario.
	writeFile(t, tmp, "linker.md", "[[sub/icon.png]]\n[[icon.png]]\n")
	writeFile(t, tmp, "icon.png", "root icon")
	os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)
	writeFile(t, tmp, "sub/icon.png", "sub icon")

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	// [[sub/icon.png]] points to sub/icon.png, but basename [[icon.png]] resolves to root.
	// Should NOT be simplified.
	for _, r := range result.Rewritten {
		if r.OldLink == "[[sub/icon.png]]" {
			t.Error("[[sub/icon.png]] should not be simplified (non-root, root-priority)")
		}
	}
}

func TestSimplifyBrokenPathSkipped(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "A.md", "[[sub/NonExistent]]\n")
	os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) != 0 {
		t.Errorf("expected no rewrites for broken links, got %d", len(result.Rewritten))
	}
}

func TestSimplifyVaultEscapeSkipped(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "A.md", "[[../../outside]]\n")

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Rewritten) != 0 {
		t.Errorf("expected no rewrites for vault-escape links, got %d", len(result.Rewritten))
	}
}

func TestSimplifyFileScope(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{
		DryRun: true,
		Files:  []string{"deep/D.md"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Only deep/D.md should be scanned.
	for _, r := range result.Rewritten {
		if r.File != "deep/D.md" {
			t.Errorf("unexpected rewrite in file %s (expected only deep/D.md)", r.File)
		}
	}
	if len(result.Rewritten) == 0 {
		t.Error("expected rewrites for deep/D.md")
	}
}

func TestSimplifyBuildExclude(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	// Create config that excludes deep/.
	writeFile(t, tmp, "mdhop.yaml", "build:\n  exclude_paths:\n    - \"deep/**\"\n")

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.File == "deep/D.md" {
			t.Error("deep/D.md should be excluded by build.exclude_paths")
		}
	}
}

func TestSimplifyMarkdownNoMdExt(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.OldLink] = r.NewLink
	}

	// [noext](sub/B) → [noext](B) (no .md extension preserved)
	if got, ok := found["[noext](sub/B)"]; !ok || got != "[noext](B)" {
		t.Errorf("expected [noext](sub/B) → [noext](B), got %q (ok=%v)", got, ok)
	}
}

func TestSimplifyTagsUntouched(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range result.Rewritten {
		if r.OldLink == "#mytag" {
			t.Error("tag should not be rewritten")
		}
	}
}

func TestSimplifyAssetNoteNamespaceConflict(t *testing.T) {
	tmp := t.TempDir()
	// Create a vault where asset photo.png and note photo.png.md coexist.
	os.MkdirAll(filepath.Join(tmp, "images"), 0o755)
	writeFile(t, tmp, "images/photo.png", "fake png")
	writeFile(t, tmp, "photo.png.md", "# Photo\n")
	writeFile(t, tmp, "A.md", "[[images/photo.png]]\n")

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	// [[images/photo.png]] should NOT be simplified because note photo.png.md
	// has basename key "photo.png" which matches the asset basename.
	for _, r := range result.Rewritten {
		if r.OldLink == "[[images/photo.png]]" {
			t.Error("[[images/photo.png]] should not be simplified (namespace conflict with note)")
		}
	}
}

func TestSimplifyFileScopeNotFound(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

	_, err := core.Simplify(tmp, core.SimplifyOptions{
		Files: []string{"nonexistent.md"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// writeFile is a test helper that writes content to a file relative to dir.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}


