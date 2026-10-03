package core

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadHeadOwnershipAndEmptyResults(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		n             int
		want          []string
	}{
		{"small_head", "---\ntitle: test\n---\n\nhead\nbody\n" + strings.Repeat("tail\n", 1000), 2, []string{"head", "body"}},
		{"short_body", "head\n", 3, []string{"head"}},
		{"empty_file", "", 2, nil},
		{"frontmatter_only", "---\ntitle: test\n---\n", 2, []string{}},
		{"blank_only", "\n \n\t\n", 2, []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vault := t.TempDir()
			path := filepath.Join(vault, "Note.md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			head, err := readHead(vault, contentSource{path: "Note.md", mtime: info.ModTime().Unix()}, tt.n)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(head, tt.want) {
				t.Errorf("head = %#v, want %#v", head, tt.want)
			}
			if cap(head) != len(head) {
				t.Errorf("head capacity = %d, want %d", cap(head), len(head))
			}
		})
	}
}

func TestReadHeadScannerFailureAfterHead(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "Note.md")
	if err := os.WriteFile(path, []byte("head\n"+strings.Repeat("x", bufio.MaxScanTokenSize)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	head, err := readHead(vault, contentSource{path: "Note.md", mtime: info.ModTime().Unix()}, 1)
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("error = %v, want Scanner token too long", err)
	}
	if head != nil {
		t.Errorf("head = %#v, want nil on read failure", head)
	}
}

func TestQueryContentSourceSelection(t *testing.T) {
	db := newTestDB(t)

	for _, node := range []struct {
		id, mtime       int64
		key, name, path string
	}{
		{1, 100, "target", "Target", "Target.md"},
		{2, 20, "source-b", "B", "b.md"},
		{3, 10, "source-a", "A", "a.md"},
		{4, 30, "source-excluded", "Excluded", "excluded.md"},
	} {
		if _, err := db.Exec(`INSERT INTO nodes (id, node_key, type, name, path, mtime) VALUES (?, ?, 'note', ?, ?, ?)`, node.id, node.key, node.name, node.path, node.mtime); err != nil {
			t.Fatalf("insert node %q: %v", node.path, err)
		}
	}
	for _, edge := range []struct {
		source, lineStart, lineEnd int64
	}{
		{2, 1, 1},
		{3, 8, 8},
		{3, 2, 2},
		{4, 3, 3},
	} {
		if _, err := db.Exec(`INSERT INTO edges (source_id, target_id, link_type, raw_link, line_start, line_end) VALUES (?, 1, 'wiki', 'Target', ?, ?)`, edge.source, edge.lineStart, edge.lineEnd); err != nil {
			t.Fatalf("insert edge: %v", err)
		}
	}

	head, err := queryHeadSource(db, 1)
	if err != nil {
		t.Fatalf("query head source: %v", err)
	}
	if head != (contentSource{path: "Target.md", mtime: 100}) {
		t.Errorf("head source = %#v, want Target.md with mtime 100", head)
	}

	exclude, err := NewExcludeFilter(ExcludeConfig{}, []string{"excluded.md"}, nil)
	if err != nil {
		t.Fatalf("new exclude filter: %v", err)
	}
	sources, err := querySnippetSources(db, 1, exclude, []string{"a.md", "b.md"})
	if err != nil {
		t.Fatalf("query snippet sources: %v", err)
	}
	want := []snippetSource{
		{contentSource: contentSource{path: "a.md", mtime: 10}, lineStart: 2, lineEnd: 2},
		{contentSource: contentSource{path: "a.md", mtime: 10}, lineStart: 8, lineEnd: 8},
		{contentSource: contentSource{path: "b.md", mtime: 20}, lineStart: 1, lineEnd: 1},
	}
	if !reflect.DeepEqual(sources, want) {
		t.Errorf("snippet sources = %#v, want %#v", sources, want)
	}
}

func TestQueryContentReaders(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "Note.md")
	if err := os.WriteFile(path, []byte("---\ntitle: test\n---\n\n# Head\nbody\ntarget\ntail\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat note: %v", err)
	}
	source := contentSource{path: "Note.md", mtime: info.ModTime().Unix()}

	head, err := readHead(vault, source, 2)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	if want := []string{"# Head", "body"}; !reflect.DeepEqual(head, want) {
		t.Errorf("head = %#v, want %#v", head, want)
	}

	snippets, err := readSnippets(vault, []snippetSource{{contentSource: source, lineStart: 7, lineEnd: 7}}, 1)
	if err != nil {
		t.Fatalf("read snippets: %v", err)
	}
	wantSnippet := []SnippetEntry{{SourcePath: "Note.md", LineStart: 6, LineEnd: 8, Lines: []string{"body", "target", "tail"}}}
	if !reflect.DeepEqual(snippets, wantSnippet) {
		t.Errorf("snippets = %#v, want %#v", snippets, wantSnippet)
	}

	if _, err := readHead(vault, contentSource{path: "missing.md", mtime: source.mtime}, 1); !errors.Is(err, ErrFileNotFound) {
		t.Errorf("missing file error = %v, want ErrFileNotFound", err)
	}
	if _, err := readSnippets(vault, []snippetSource{{contentSource: contentSource{path: "Note.md", mtime: source.mtime + 1}, lineStart: 1, lineEnd: 1}}, 0); !errors.Is(err, ErrSourceStale) {
		t.Errorf("stale source error = %v, want ErrSourceStale", err)
	}

	lockedDir := filepath.Join(vault, "locked")
	if err := os.Mkdir(lockedDir, 0o755); err != nil {
		t.Fatalf("make locked directory: %v", err)
	}
	lockedPath := filepath.Join(lockedDir, "Note.md")
	if err := os.WriteFile(lockedPath, []byte("locked\n"), 0o644); err != nil {
		t.Fatalf("write locked note: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o000); err != nil {
		t.Fatalf("lock directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(lockedDir, 0o755); err != nil {
			t.Errorf("unlock directory: %v", err)
		}
	})
	_, err = readHead(vault, contentSource{path: "locked/Note.md", mtime: source.mtime}, 1)
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("permission error = %v, want permission error", err)
	}
	if errors.Is(err, ErrFileNotFound) {
		t.Errorf("permission error = %v, must not be ErrFileNotFound", err)
	}
}

func TestReadSnippetsSameSecondTruncation(t *testing.T) {
	for _, tt := range []struct {
		name                             string
		content                          string
		lineStart, lineEnd, contextLines int
	}{
		{"start_beyond_shortened_file", "short\n", 8, 8, 1},
		{"end_beyond_shortened_file", "short\n", 1, 8, 1},
		{"large_context", "short\n", 8, 8, 10},
		{"empty_file", "", 8, 8, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vault := t.TempDir()
			path := filepath.Join(vault, "Source.md")
			if err := os.WriteFile(path, []byte("1\n2\n3\n4\n5\n6\n7\n[[Target]]\n"), 0o644); err != nil {
				t.Fatalf("write source: %v", err)
			}
			mtime := time.Unix(1700000000, 123000000)
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatalf("set source time: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat source: %v", err)
			}
			source := snippetSource{contentSource: contentSource{path: "Source.md", mtime: info.ModTime().Unix()}, lineStart: tt.lineStart, lineEnd: tt.lineEnd}
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("truncate source: %v", err)
			}
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatalf("restore source time: %v", err)
			}
			if err := checkStale(path, source.mtime); err != nil {
				t.Fatalf("same-second mtime must pass stale check: %v", err)
			}
			snippets, err := readSnippets(vault, []snippetSource{source}, tt.contextLines)
			if !errors.Is(err, ErrSourceStale) {
				t.Errorf("error = %v, want ErrSourceStale", err)
			}
			if snippets != nil {
				t.Errorf("snippets = %#v, want nil", snippets)
			}
		})
	}
}
