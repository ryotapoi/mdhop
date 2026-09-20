package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
}
