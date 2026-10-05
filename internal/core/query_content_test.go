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

func TestQueryHeadSourceSelection(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`INSERT INTO nodes (id, node_key, type, name, path, mtime) VALUES (1, 'target', 'note', 'Target', 'Target.md', 100)`); err != nil {
		t.Fatal(err)
	}

	head, err := queryHeadSource(db, 1)
	if err != nil {
		t.Fatalf("query head source: %v", err)
	}
	if head != (contentSource{path: "Target.md", mtime: 100}) {
		t.Errorf("head source = %#v, want Target.md with mtime 100", head)
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

	snippets, err := readSnippets(vault, []snippetSource{{contentSource: source, lineStart: 7, lineEnd: 7}}, 1, make(map[string][]string))
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
	if _, err := readSnippets(vault, []snippetSource{{contentSource: contentSource{path: "Note.md", mtime: source.mtime + 1}, lineStart: 1, lineEnd: 1}}, 0, make(map[string][]string)); !errors.Is(err, ErrSourceStale) {
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
			snippets, err := readSnippets(vault, []snippetSource{source}, tt.contextLines, make(map[string][]string))
			if !errors.Is(err, ErrSourceStale) {
				t.Errorf("error = %v, want ErrSourceStale", err)
			}
			if snippets != nil {
				t.Errorf("snippets = %#v, want nil", snippets)
			}
		})
	}
}

func TestReadSnippetsSharedCacheAndOwnership(t *testing.T) {
	for _, mutation := range []string{"missing", "stale"} {
		t.Run(mutation, func(t *testing.T) {
			vault := t.TempDir()
			path := filepath.Join(vault, "Source.md")
			if err := os.WriteFile(path, []byte("first\n[[Target]]\n"+strings.Repeat("tail\n", 1000)), 0o644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			source := snippetSource{contentSource: contentSource{path: "Source.md", mtime: info.ModTime().Unix()}, lineStart: 2, lineEnd: 2}
			cache := make(map[string][]string)
			first, err := readSnippets(vault, []snippetSource{source, source}, 0, cache)
			if err != nil {
				t.Fatal(err)
			}
			expectedError := ErrFileNotFound
			if mutation == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				expectedError = ErrSourceStale
				if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				later := info.ModTime().Add(10 * time.Second)
				if err := os.Chtimes(path, later, later); err != nil {
					t.Fatal(err)
				}
			}
			// Reusing the cache must return the initial content without accessing disk.
			second, err := readSnippets(vault, []snippetSource{source}, 0, cache)
			if err != nil {
				t.Fatal(err)
			}
			want := SnippetEntry{SourcePath: "Source.md", LineStart: 2, LineEnd: 2, Lines: []string{"[[Target]]"}}
			if !reflect.DeepEqual(first, []SnippetEntry{want, want}) || !reflect.DeepEqual(second, []SnippetEntry{want}) {
				t.Fatalf("first = %#v, second = %#v, want unchanged duplicate occurrences", first, second)
			}
			first[0].Lines[0] = "modified"
			if cache[source.path][1] != "[[Target]]" || first[1].Lines[0] != "[[Target]]" || second[0].Lines[0] != "[[Target]]" {
				t.Fatal("snippet lines alias the cached body or another snippet")
			}
			if _, err := readSnippets(vault, []snippetSource{source}, 0, make(map[string][]string)); !errors.Is(err, expectedError) {
				t.Fatalf("fresh cache error = %v, want %v", err, expectedError)
			}
			// Each occurrence still validates its indexed range on a cache hit.
			for _, bounds := range [][2]int{{1003, 1003}, {2, 1003}} {
				invalid := source
				invalid.lineStart, invalid.lineEnd = bounds[0], bounds[1]
				if _, err := readSnippets(vault, []snippetSource{invalid}, 0, cache); !errors.Is(err, ErrSourceStale) {
					t.Fatalf("cached range %v error = %v, want ErrSourceStale", bounds, err)
				}
			}
		})
	}
}
