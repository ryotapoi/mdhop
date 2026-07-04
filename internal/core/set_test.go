package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetUpdatesExistingKey(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: Old\nreviewed: 2026-01-01 # old marker\nstatus: draft\n---\n# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "2026-07-04"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Created {
		t.Fatal("Created = true, want false")
	}

	got := readTestFile(t, filepath.Join(vault, "A.md"))
	want := "---\ntitle: Old\nreviewed: 2026-07-04 # old marker\nstatus: draft\n---\n# A\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant =\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "reviewed", "2026-07-04") {
		t.Fatalf("meta missing reviewed=2026-07-04: %+v", meta)
	}
}

func TestSetAddsMissingKeyBeforeFrontmatterClose(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: Old\n---\n# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}

	got := readTestFile(t, filepath.Join(vault, "A.md"))
	want := "---\ntitle: Old\nreviewed: done\n---\n# A\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant =\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "reviewed", "done") {
		t.Fatalf("meta missing reviewed=done: %+v", meta)
	}
}

// TestSetAddsKeyToEmptyFrontmatter guards a regression where "---\n---\n"
// (frontmatter present but with no keys) unmarshals to a zero-value yaml.Node
// (Kind == 0, not yaml.DocumentNode) rather than an empty mapping, which an
// earlier implementation mistook for "not a mapping" and rejected outright.
func TestSetAddsKeyToEmptyFrontmatter(t *testing.T) {
	vault := t.TempDir()
	content := "---\n---\n# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}

	got := readTestFile(t, filepath.Join(vault, "A.md"))
	want := "---\nreviewed: done\n---\n# A\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant =\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "reviewed", "done") {
		t.Fatalf("meta missing reviewed=done: %+v", meta)
	}
}

func TestSetCreatesFrontmatterWhenMissing(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}

	got := readTestFile(t, filepath.Join(vault, "A.md"))
	want := "---\nreviewed: done\n---\n# A\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant =\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "reviewed", "done") {
		t.Fatalf("meta missing reviewed=done: %+v", meta)
	}
}

func TestSetUnregisteredFileError(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: A\n---\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("---\ntitle: B\n---\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "B.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, ErrFileNotRegistered) {
		t.Fatalf("error = %v, want ErrFileNotRegistered", err)
	}
}

func TestSetMissingDiskFileError(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("---\ntitle: A\n---\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatalf("remove A.md: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("error = %v, want ErrFileNotFound", err)
	}
}

func TestSetStaleFileError(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte("---\ntitle: A\n---\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	future := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes A.md: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, ErrSourceStale) {
		t.Fatalf("error = %v, want ErrSourceStale", err)
	}
}

// TestSetQuotesValuesWithYAMLIndicatorPrefix guards a regression where a
// value starting with a YAML plain-scalar indicator (e.g. "- ", "[", "*")
// was written unquoted. "probe: - leading dash" reparses as a YAML block
// sequence entry, corrupting the frontmatter block for every key that
// follows on subsequent parses (including the next Set/Update/Build call).
func TestSetQuotesValuesWithYAMLIndicatorPrefix(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: Old\n---\n# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	cases := []struct {
		name  string
		value string
	}{
		{"leading dash", "- leading dash"},
		{"leading bracket", "[bracket"},
		{"leading anchor", "*anchor"},
		{"leading ampersand", "&anchor2"},
		{"leading bang", "!tag"},
		{"leading fold", ">fold"},
		{"leading literal", "|literal"},
		{"leading double quote", `"quoted start`},
		{"leading single quote", "'single start"},
		{"colon space inside", "colon: inside"},
		{"trailing colon", "trailing colon:"},
		{"space hash", "has # hash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Set(vault, SetOptions{File: "A.md", Key: "probe", Value: tc.value}); err != nil {
				t.Fatalf("set: %v", err)
			}
			meta := queryMetaForPath(t, dbPath(vault), "A.md")
			if !hasMeta(meta, "probe", tc.value) {
				t.Fatalf("meta missing probe=%q: %+v", tc.value, meta)
			}
			// A corrupted frontmatter block would make this subsequent parse
			// fail (or silently drop keys), which is exactly what the bug did.
			if _, err := Build(vault); err != nil {
				t.Fatalf("build after set(%q): %v (frontmatter likely corrupted)", tc.value, err)
			}
		})
	}
}

func TestSetSequenceValueError(t *testing.T) {
	vault := t.TempDir()
	content := "---\naliases:\n  - one\n  - two\n---\n# A\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", Value: "single"})
	if err == nil || !strings.Contains(err.Error(), "sequence value") {
		t.Fatalf("error = %v, want sequence value", err)
	}
}

func TestSetMultilinePlainScalarValueError(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: This is a long\n  title that wraps\nstatus: draft\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "title", Value: "New"})
	if err == nil || !strings.Contains(err.Error(), "multi-line") {
		t.Fatalf("error = %v, want multi-line", err)
	}
	if got := readTestFile(t, path); got != content {
		t.Fatalf("content changed after failed set:\n%s\nwant:\n%s", got, content)
	}
}

func TestSetLastKeyAllowsTrailingBlankLineBeforeFrontmatterClose(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\nstatus: draft\n\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "status", Value: "new"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	want := "---\ntitle: A\nstatus: new\n\n---\n# A\n"
	if got := readTestFile(t, path); got != want {
		t.Fatalf("content =\n%s\nwant:\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "status", "new") {
		t.Fatalf("meta missing status=new: %+v", meta)
	}
}

func TestSetLastKeyAllowsTrailingCommentBeforeFrontmatterClose(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\nstatus: draft\n# note\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "status", Value: "new"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	want := "---\ntitle: A\nstatus: new\n# note\n---\n# A\n"
	if got := readTestFile(t, path); got != want {
		t.Fatalf("content =\n%s\nwant:\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if !hasMeta(meta, "status", "new") {
		t.Fatalf("meta missing status=new: %+v", meta)
	}
}

func TestSetLastKeyMultilinePlainScalarBeforeTrailingBlankLineError(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\nstatus: long\n  value that wraps\n\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "status", Value: "new"})
	if err == nil || !strings.Contains(err.Error(), "multi-line") {
		t.Fatalf("error = %v, want multi-line", err)
	}
	if got := readTestFile(t, path); got != content {
		t.Fatalf("content changed after failed set:\n%s\nwant:\n%s", got, content)
	}
}

func TestSetLastKeyMultilineQuotedScalarCommentLineError(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\nstatus: \"long\n  # value\"\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "status", Value: "new"})
	if err == nil || !strings.Contains(err.Error(), "multi-line") {
		t.Fatalf("error = %v, want multi-line", err)
	}
	if got := readTestFile(t, path); got != content {
		t.Fatalf("content changed after failed set:\n%s\nwant:\n%s", got, content)
	}
}

func TestSetDuplicateTargetKeyError(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\ntitle: B\nstatus: draft\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "title", Value: "New"})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate", err)
	}
	if got := readTestFile(t, path); got != content {
		t.Fatalf("content changed after failed set:\n%s\nwant:\n%s", got, content)
	}
}

func TestSetAllowsDuplicateNonTargetKey(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: A\nstatus: draft\nstatus: old\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "title", Value: "New"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	want := "---\ntitle: New\nstatus: draft\nstatus: old\n---\n# A\n"
	if got := readTestFile(t, path); got != want {
		t.Fatalf("content =\n%s\nwant:\n%s", got, want)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func hasMeta(rows []MetaRow, key, value string) bool {
	for _, row := range rows {
		if row.Key == key && row.Value == value {
			return true
		}
	}
	return false
}
