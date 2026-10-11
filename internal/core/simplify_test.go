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

	found := map[string]string{}
	for _, r := range result.Rewritten {
		found[r.File+"\x00"+r.OldLink] = r.NewLink
		if r.OldLink == "#mytag" {
			t.Errorf("tag was reported as a rewritten link: %+v", r)
		}
	}
	for _, tc := range []struct{ name, file, old, want string }{
		{"note wikilink", "A.md", "[[sub/B]]", "[[B]]"},
		{"note Markdown", "A.md", "[text](sub/C.md)", "[text](C.md)"},
		{"asset", "A.md", "[[images/photo.png]]", "[[photo.png]]"},
		{"relative parent", "deep/D.md", "[[../sub/B]]", "[[B]]"},
		{"relative current", "deep/D.md", "[[./E]]", "[[E]]"},
		{"subpath", "A.md", "[[sub/B#Heading]]", "[[B#Heading]]"},
		{"alias", "A.md", "[[sub/B|alias]]", "[[B|alias]]"},
		{"Markdown fragment", "A.md", "[text](sub/B.md#section)", "[text](B.md#section)"},
		{"Markdown without extension", "A.md", "[noext](sub/B)", "[noext](B)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := found[tc.file+"\x00"+tc.old]
			if !ok || got != tc.want {
				t.Errorf("%s: %q → %q (present=%v), want %q", tc.file, tc.old, got, ok, tc.want)
			}
		})
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

func TestSimplifyDryRun(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify", tmp); err != nil {
		t.Fatal(err)
	}

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

	// Exclude deep notes and one of two same-named assets. Simplify should
	// apply the same config to both file collections.
	writeFile(t, tmp, "mdhop.toml", "[build]\nexclude_paths = ['deep/**', 'assets1/**']\n")
	aPath := filepath.Join(tmp, "A.md")
	aContent, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, append(aContent, []byte("\n[[assets2/icon.png]]\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	foundUniqueAsset := false
	for _, r := range result.Rewritten {
		if r.File == "deep/D.md" {
			t.Error("deep/D.md should be excluded by build.exclude_paths")
		}
		if r.File == "A.md" && r.OldLink == "[[assets2/icon.png]]" && r.NewLink == "[[icon.png]]" {
			foundUniqueAsset = true
		}
	}
	if !foundUniqueAsset {
		t.Error("assets1 exclusion should leave assets2/icon.png as a unique asset target")
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

func TestSimplifyFrontmatterWikilink(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify_frontmatter", tmp); err != nil {
		t.Fatal(err)
	}

	result, err := core.Simplify(tmp, core.SimplifyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, r := range result.Rewritten {
		if r.File == "Index.md" {
			found[r.OldLink] = r.NewLink
		}
	}

	// Quoted scalar wikilink rewritten to basename.
	if got, ok := found["[[sub/B]]"]; !ok || got != "[[B]]" {
		t.Errorf("expected [[sub/B]] → [[B]], got %q (ok=%v)", got, ok)
	}
	// Alias preserved.
	if got, ok := found["[[sub/C|alias C]]"]; !ok || got != "[[C|alias C]]" {
		t.Errorf("expected [[sub/C|alias C]] → [[C|alias C]], got %q (ok=%v)", got, ok)
	}
	// Subpath preserved.
	if got, ok := found["[[sub/B#Heading]]"]; !ok || got != "[[B#Heading]]" {
		t.Errorf("expected [[sub/B#Heading]] → [[B#Heading]], got %q (ok=%v)", got, ok)
	}

	// Ambiguous frontmatter wikilink should be skipped, not rewritten.
	if _, ok := found["[[dir1/M]]"]; ok {
		t.Error("[[dir1/M]] in frontmatter should not be simplified (ambiguous)")
	}
	var ambiguousReported bool
	for _, s := range result.Skipped {
		if s.File == "Index.md" && s.RawLink == "[[dir1/M]]" {
			ambiguousReported = true
			if len(s.Candidates) != 2 {
				t.Errorf("expected 2 candidates for [[dir1/M]], got %d: %v", len(s.Candidates), s.Candidates)
			}
		}
	}
	if !ambiguousReported {
		t.Error("expected ambiguous frontmatter [[dir1/M]] in skipped list")
	}
}

func TestSimplifyFrontmatterWikilinkApplied(t *testing.T) {
	tmp := t.TempDir()
	if err := testutil.CopyDir("../../testdata/vault_simplify_frontmatter", tmp); err != nil {
		t.Fatal(err)
	}

	if _, err := core.Simplify(tmp, core.SimplifyOptions{}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "Index.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)

	// Quoted style preserved (still wrapped in "...").
	if !strings.Contains(got, `parent: "[[B]]"`) {
		t.Errorf("expected parent rewritten to quoted [[B]], got:\n%s", got)
	}
	if !strings.Contains(got, `"[[C|alias C]]"`) {
		t.Errorf("expected [[sub/C|alias C]] rewritten with alias preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `"[[B#Heading]]"`) {
		t.Errorf("expected [[sub/B#Heading]] rewritten with subpath preserved, got:\n%s", got)
	}
	// Ambiguous untouched.
	if !strings.Contains(got, `ambiguous: "[[dir1/M]]"`) {
		t.Errorf("expected ambiguous [[dir1/M]] untouched, got:\n%s", got)
	}
	// Body wikilink also rewritten.
	if !strings.Contains(got, "Body link to keep behavior consistent: [[B]]") {
		t.Errorf("expected body [[sub/B]] also rewritten, got:\n%s", got)
	}
}

func TestSimplifyDryRunPreflightsFrontmatterCandidates(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "unsafe encoded candidate",
			content: "---\nref: \"\\u005b\\u005bsub/B\\u005d\\u005d\"\n---\n",
			wantErr: true,
		},
		{
			name:    "safe quoted candidate",
			content: "---\nref: \"[[sub/B]]\"\n---\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vault := t.TempDir()
			writeFile(t, vault, "A.md", tt.content)
			writeFile(t, vault, "sub/B.md", "# B\n")
			original, err := os.ReadFile(filepath.Join(vault, "A.md"))
			if err != nil {
				t.Fatal(err)
			}

			_, dryRunErr := core.Simplify(vault, core.SimplifyOptions{DryRun: true})
			if (dryRunErr != nil) != tt.wantErr {
				t.Fatalf("dry-run error = %v, wantErr %v", dryRunErr, tt.wantErr)
			}
			if dryRunErr != nil && !strings.Contains(dryRunErr.Error(), "correspondence") {
				t.Fatalf("dry-run error = %v, want correspondence rejection", dryRunErr)
			}
			if got, err := os.ReadFile(filepath.Join(vault, "A.md")); err != nil || string(got) != string(original) {
				t.Fatalf("A.md changed during dry-run: got %q, err=%v", got, err)
			}

			_, executionErr := core.Simplify(vault, core.SimplifyOptions{})
			if (executionErr != nil) != tt.wantErr {
				t.Fatalf("execution error = %v, wantErr %v", executionErr, tt.wantErr)
			}
			if tt.wantErr {
				if got, err := os.ReadFile(filepath.Join(vault, "A.md")); err != nil || string(got) != string(original) {
					t.Fatalf("A.md changed during rejected execution: got %q, err=%v", got, err)
				}
			}
		})
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

func TestSimplifyMultipleBacktickSameLink(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub/B.md"), []byte("# B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	content := "[[sub/B]] ``[[sub/B]] [B](sub/B.md) #hidden`` [B](sub/B.md)\n"
	if err := os.WriteFile(filepath.Join(tmp, "A.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Simplify(tmp, core.SimplifyOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[[B]] ``[[sub/B]] [B](sub/B.md) #hidden`` [B](B.md)\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
