package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRewriteFrontmatterPreservesOtherFlowMappingEntries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		yaml    string
		key     string
		value   string
		list    []string
		want    any
		created bool
	}{
		{name: "scalar", yaml: "{status: draft, title: A, extra: {x: 1}} # marker", key: "status", value: "done", want: "done"},
		{name: "scalar to list", yaml: "{status: draft, title: A, extra: {x: 1}} # marker", key: "status", list: []string{"one", "one", ""}, want: []any{"one", "one", ""}},
		{name: "sequence to list", yaml: "{status: [old], title: A, extra: {x: 1}} # marker", key: "status", list: []string{}, want: []any{}},
		{name: "missing scalar", yaml: "{title: A, extra: {x: 1}} # marker", key: "a: b", value: "new, value", want: "new, value", created: true},
		{name: "missing list", yaml: "{title: A, extra: {x: 1}} # marker", key: "status", list: []string{"new"}, want: []any{"new"}, created: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\n" + tc.yaml + "\n---\n# Body\nunchanged\n"
			got, created, err := rewriteFrontmatterValue([]byte(content), tc.key, tc.value, tc.list)
			if err != nil {
				t.Fatal(err)
			}
			if created != tc.created {
				t.Fatalf("created = %v, want %v", created, tc.created)
			}
			lines := strings.Split(string(got), "\n")
			var values map[string]any
			if err := yaml.Unmarshal([]byte(strings.Join(lines[1:frontmatterEnd(lines)], "\n")), &values); err != nil {
				t.Fatalf("invalid rewritten YAML: %v\n%s", err, got)
			}
			want := map[string]any{tc.key: tc.want, "title": "A", "extra": map[string]any{"x": 1}}
			if !reflect.DeepEqual(values, want) {
				t.Fatalf("values = %#v, want %#v", values, want)
			}
			if !strings.Contains(string(got), "# marker") || !strings.HasSuffix(string(got), "---\n# Body\nunchanged\n") {
				t.Fatalf("comment or body changed: %s", got)
			}
		})
	}
}

func TestRewriteFrontmatterQuotesKeysAndPreservesSpacing(t *testing.T) {
	for _, list := range [][]string{nil, {"new"}} {
		for _, key := range []string{"status", "a: b", "日本語\" # key"} {
			t.Run(fmt.Sprintf("key=%s/list=%v", key, list != nil), func(t *testing.T) {
				content := "---\n" + strconv.Quote(key) + ": \"draft\" # marker\n\n# independent comment\ntitle: A\n---\n# Body\n"
				got, created, err := rewriteFrontmatterValue([]byte(content), key, "new", list)
				if err != nil || created {
					t.Fatalf("rewrite = created %v, err %v", created, err)
				}
				var values map[string]any
				lines := strings.Split(string(got), "\n")
				if err := yaml.Unmarshal([]byte(strings.Join(lines[1:frontmatterEnd(lines)], "\n")), &values); err != nil {
					t.Fatalf("invalid rewritten YAML: %v", err)
				}
				var want any = "new"
				if list != nil {
					want = []any{"new"}
				}
				if !reflect.DeepEqual(values, map[string]any{key: want, "title": "A"}) {
					t.Fatalf("values = %#v", values)
				}
				if !strings.Contains(string(got), "# marker") || !strings.HasSuffix(string(got), "\n\n# independent comment\ntitle: A\n---\n# Body\n") {
					t.Fatalf("spacing, comment, other key, or body changed: %s", got)
				}
			})
		}
	}
}

func TestRewriteFrontmatterRejectsActualMultilineScalar(t *testing.T) {
	for _, source := range []string{
		"\"a: b\": long\n  continuation\n\n# comment\ntitle: A",
		"\"a: b\": \"long\n  # continuation\"\ntitle: A",
		"\"a: b\": 'long\n  continuation'\ntitle: A",
		"{\"a: b\": \"long\n  continuation\", title: A}",
		"{\"a: b\": long\n  continuation, title: A}",
	} {
		for _, list := range [][]string{nil, {"new"}} {
			_, _, err := rewriteFrontmatterValue([]byte("---\n"+source+"\n---\n# Body\n"), "a: b", "new", list)
			if err == nil || !strings.Contains(err.Error(), "multi-line") {
				t.Fatalf("source %q, list %v: error = %v, want multi-line", source, list != nil, err)
			}
		}
	}
}

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

func TestSetRestoresContentPermissionAndMtimeWhenUpdateFails(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	original := "---\ntitle: A\n---\n# A\n"
	originalMtime := time.Date(2026, time.January, 2, 3, 4, 5, 123456789, time.UTC)
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if err := os.Chtimes(path, originalMtime, originalMtime); err != nil {
		t.Fatalf("set original mtime: %v", err)
	}
	baseline, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}
	baselineMode := baseline.Mode().Perm()
	baselineMtime := baseline.ModTime()
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	primaryErr := errors.New("update blocked")
	oldSetUpdate := setUpdate
	setUpdate = func(_ string, _ UpdateOptions, _ ...Locations) (*UpdateResult, error) {
		got := readTestFile(t, path)
		if !strings.Contains(got, "reviewed: done") {
			t.Fatalf("Update reached before Set wrote new content:\n%s", got)
		}
		return nil, primaryErr
	}
	t.Cleanup(func() { setUpdate = oldSetUpdate })

	_, err = Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want to retain primary error", err)
	}
	if got := readTestFile(t, path); got != original {
		t.Fatalf("content after rollback = %q, want %q", got, original)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after rollback: %v", err)
	}
	if got := info.Mode().Perm(); got != baselineMode {
		t.Errorf("mode after rollback = %o, want baseline %o", got, baselineMode)
	}
	if got := info.ModTime(); !got.Equal(baselineMtime) {
		t.Errorf("mtime after rollback = %s, want baseline %s", got, baselineMtime)
	}
}

func TestSetReportsRestoreContentOrPermissionFailure(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte("---\ntitle: A\n---\n"), 0o600); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	primaryErr := errors.New("update blocked")
	oldSetUpdate := setUpdate
	setUpdate = func(_ string, _ UpdateOptions, _ ...Locations) (*UpdateResult, error) {
		if got := readTestFile(t, path); !strings.Contains(got, "reviewed: done") {
			t.Fatalf("Update reached before Set wrote new content:\n%s", got)
		}
		return nil, primaryErr
	}
	t.Cleanup(func() { setUpdate = oldSetUpdate })
	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(string, []byte, os.FileMode) error {
		return errors.New("restore content blocked")
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want to retain primary error", err)
	}
	assertRollbackFailureReported(t, err, "update blocked", "restore content blocked")
	if !strings.Contains(err.Error(), "could not restore A.md") {
		t.Fatalf("error missing restore detail:\n%s", err)
	}
}

func TestSetReportsRestoreMtimeFailure(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	original := "---\ntitle: A\n---\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	baseline, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}
	baselineMode := baseline.Mode().Perm()
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	primaryErr := errors.New("update blocked")
	oldSetUpdate := setUpdate
	setUpdate = func(_ string, _ UpdateOptions, _ ...Locations) (*UpdateResult, error) {
		if got := readTestFile(t, path); !strings.Contains(got, "reviewed: done") {
			t.Fatalf("Update reached before Set wrote new content:\n%s", got)
		}
		return nil, primaryErr
	}
	t.Cleanup(func() { setUpdate = oldSetUpdate })
	oldSetChtimes := setChtimes
	setChtimes = func(string, time.Time, time.Time) error {
		return errors.New("restore mtime blocked")
	}
	t.Cleanup(func() { setChtimes = oldSetChtimes })

	_, err = Set(vault, SetOptions{File: "A.md", Key: "reviewed", Value: "done"})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want to retain primary error", err)
	}
	assertRollbackFailureReported(t, err, "update blocked", "restore mtime blocked")
	if !strings.Contains(err.Error(), "could not restore modification time for A.md") {
		t.Fatalf("error missing mtime restore detail:\n%s", err)
	}
	if got := readTestFile(t, path); got != original {
		t.Fatalf("content after failed mtime restore = %q, want %q", got, original)
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat after failed mtime restore: %v", statErr)
	}
	if got := info.Mode().Perm(); got != baselineMode {
		t.Errorf("mode after failed mtime restore = %o, want baseline %o", got, baselineMode)
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

func TestSetListReplacesScalarAndSequencesAndRefreshesIndex(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "scalar",
			content: "---\ntitle: A\naliases: old # retained\nstatus: draft\n---\n# A\n",
			want:    "---\ntitle: A\naliases: # retained\n  - \"one\"\n  - \"\"\n  - \"one\"\nstatus: draft\n---\n# A\n",
		},
		{
			name:    "block sequence",
			content: "---\ntitle: A\naliases: # retained\n  - old\n  - older\nstatus: draft\n---\n# A\n",
			want:    "---\ntitle: A\naliases: # retained\n  - \"one\"\n  - \"\"\n  - \"one\"\nstatus: draft\n---\n# A\n",
		},
		{
			name:    "flow sequence",
			content: "---\ntitle: A\naliases: [old, older] # retained\nstatus: draft\n---\n# A\n",
			want:    "---\ntitle: A\naliases: # retained\n  - \"one\"\n  - \"\"\n  - \"one\"\nstatus: draft\n---\n# A\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := t.TempDir()
			path := filepath.Join(vault, "A.md")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write A.md: %v", err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatalf("build: %v", err)
			}

			result, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", List: []string{"one", "", "one"}})
			if err != nil {
				t.Fatalf("set list: %v", err)
			}
			if result.Created || !slices.Equal(result.List, []string{"one", "", "one"}) {
				t.Fatalf("result = %+v, want existing list result", result)
			}
			if got := readTestFile(t, path); got != tc.want {
				t.Fatalf("content =\n%s\nwant =\n%s", got, tc.want)
			}
			meta := queryMetaForPath(t, dbPath(vault), "A.md")
			if metaCount(meta, "aliases", "one") != 2 {
				t.Fatalf("list metadata = %+v, want duplicate entries", meta)
			}
		})
	}
}

func TestSetListReplacementPreservesFollowingCommentAndFormatting(t *testing.T) {
	content := "---\ntitle: A\naliases:\n  - old\n\n  # independent comment\n\nstatus: draft\n---\nbody\n"
	got, created, err := rewriteFrontmatterValue([]byte(content), "aliases", "", []string{"new"})
	if err != nil {
		t.Fatalf("rewrite frontmatter: %v", err)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	want := "---\ntitle: A\naliases:\n  - \"new\"\n\n  # independent comment\n\nstatus: draft\n---\nbody\n"
	if string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	const preservedSuffix = "\n\n  # independent comment\n\nstatus: draft\n---\nbody\n"
	if !strings.HasSuffix(string(got), preservedSuffix) {
		t.Fatalf("content after old target = %q, want byte-for-byte suffix %q", got, preservedSuffix)
	}
}

func TestSetListReplacesMultilineSequenceItemsAndRefreshesIndex(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	content := "---\ntitle: A\naliases:\n  - |\n    obsolete literal\n    continuation\n  - >\n    obsolete folded\n    continuation\n  - \"obsolete quoted\n    continuation\"\n  - 'obsolete single\n    continuation'\n\n  # independent comment\n\nstatus: draft\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if meta := queryMetaForPath(t, dbPath(vault), "A.md"); !hasMeta(meta, "aliases", "obsolete quoted continuation") {
		t.Fatalf("old list metadata missing before set: %+v", meta)
	}

	if _, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", List: []string{"new"}}); err != nil {
		t.Fatalf("set list: %v", err)
	}
	want := "---\ntitle: A\naliases:\n  - \"new\"\n\n  # independent comment\n\nstatus: draft\n---\nbody\n"
	if got := readTestFile(t, path); got != want {
		t.Fatalf("content =\n%s\nwant =\n%s", got, want)
	}
	meta := queryMetaForPath(t, dbPath(vault), "A.md")
	if hasMeta(meta, "aliases", "obsolete quoted continuation") {
		t.Fatalf("old list metadata remains: %+v", meta)
	}
	if !hasMeta(meta, "aliases", "new") {
		t.Fatalf("new list metadata missing: %+v", meta)
	}
}

func TestSetListReplacesComplexSequenceSyntaxAndRefreshesIndex(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "multiline flow sequence with standalone close bracket",
			content: "---\ntitle: A\naliases: [\n  obsolete,\n  older\n]\n\n# independent comment\n\nstatus: draft\n---\nbody\n",
			want:    "---\ntitle: A\naliases:\n  - \"new\"\n\n# independent comment\n\nstatus: draft\n---\nbody\n",
		},
		{
			name:    "block sequence with plain scalar continuation",
			content: "---\ntitle: A\naliases:\n  - obsolete plain\n    continuation\n  - older plain\n    continuation\n\n# independent comment\n\nstatus: draft\n---\nbody\n",
			want:    "---\ntitle: A\naliases:\n  - \"new\"\n\n# independent comment\n\nstatus: draft\n---\nbody\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := t.TempDir()
			path := filepath.Join(vault, "A.md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write A.md: %v", err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatalf("build: %v", err)
			}

			if _, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", List: []string{"new"}}); err != nil {
				t.Fatalf("set list: %v", err)
			}
			if got := readTestFile(t, path); got != tt.want {
				t.Fatalf("content =\n%s\nwant =\n%s", got, tt.want)
			}
			meta := queryMetaForPath(t, dbPath(vault), "A.md")
			if hasMeta(meta, "aliases", "obsolete") || !hasMeta(meta, "aliases", "new") {
				t.Fatalf("metadata after set = %+v, want only new list value", meta)
			}
		})
	}
}

func TestSetListWritesExplicitYAMLStrings(t *testing.T) {
	values := []string{"2026-01-02", "true", "123", "", "plain"}
	content := "---\ntitle: A\n---\n"
	got, _, err := rewriteFrontmatterValue([]byte(content), "aliases", "", values)
	if err != nil {
		t.Fatalf("rewrite frontmatter: %v", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(got[4:len(got)-4], &doc); err != nil {
		t.Fatalf("parse written YAML: %v", err)
	}
	mapping := doc.Content[0]
	sequence := mapping.Content[3]
	if sequence.Kind != yaml.SequenceNode || len(sequence.Content) != len(values) {
		t.Fatalf("sequence = %#v, want %d items", sequence, len(values))
	}
	for i, item := range sequence.Content {
		if item.Tag != "!!str" || item.Value != values[i] {
			t.Errorf("item %d = tag %q value %q, want !!str %q", i, item.Tag, item.Value, values[i])
		}
	}
}

func TestSetListAddsMissingKeyAndWritesEmptySequence(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	content := "---\ntitle: A\n---\n# A\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", List: []string{}})
	if err != nil {
		t.Fatalf("set list: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}
	if got, want := readTestFile(t, path), "---\ntitle: A\naliases: []\n---\n# A\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestSetListCreatesFrontmatter(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if _, err := Set(vault, SetOptions{File: "A.md", Key: "aliases", List: []string{"one"}}); err != nil {
		t.Fatalf("set list: %v", err)
	}
	if got, want := readTestFile(t, path), "---\naliases:\n  - \"one\"\n---\n# A\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
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

func TestSetListMultilinePlainScalarValueError(t *testing.T) {
	vault := t.TempDir()
	content := "---\ntitle: This is a long\n  title that wraps\nstatus: draft\n---\n# A\n"
	path := filepath.Join(vault, "A.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Set(vault, SetOptions{File: "A.md", Key: "title", List: []string{"New"}})
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

func metaCount(rows []MetaRow, key, value string) int {
	count := 0
	for _, row := range rows {
		if row.Key == key && row.Value == value {
			count++
		}
	}
	return count
}
