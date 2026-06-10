package core

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

// TestGlobMatchSQLiteEquivalence verifies that the Go globMatch implementation
// agrees with SQLite's GLOB operator for the same inputs. Patterns containing
// '[' are rejected by validateGlobPatterns before reaching either
// implementation, so character classes are out of scope here.
func TestGlobMatchSQLiteEquivalence(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", filepath.Join(t.TempDir(), "glob.sqlite")))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cases := []struct {
		pattern string
		s       string
	}{
		// Literals.
		{"a.md", "a.md"},
		{"a.md", "b.md"},
		{"", ""},
		{"", "a"},
		{"a", ""},
		// Case sensitivity (GLOB is case-sensitive, unlike LIKE).
		{"A.md", "a.md"},
		{"notes/A.md", "notes/A.md"},
		// '*' wildcard ('*' crosses '/' in SQLite GLOB).
		{"*", ""},
		{"*", "a/b.md"},
		{"*.md", "a.md"},
		{"*.md", "a.png"},
		{"*.md", "notes/a.md"},
		{"notes/*", "notes/a.md"},
		{"notes/*", "notes/sub/a.md"},
		{"notes/*", "other/a.md"},
		{"notes/*", "notes/"},
		{"notes/*", "notes"},
		{"a*b*c", "aXbYc"},
		{"a*b*c", "abc"},
		{"a*b*c", "aXbY"},
		{"**", "anything"},
		{"a*", "a"},
		{"*a", "a"},
		{"*a*", "bab"},
		// '?' wildcard (exactly one character).
		{"?.md", "a.md"},
		{"?.md", "ab.md"},
		{"?.md", ".md"},
		{"a?c", "abc"},
		{"a?c", "ac"},
		{"a?c", "a/c"},
		// Mixed wildcards.
		{"notes/*/deep?.md", "notes/x/deep1.md"},
		{"notes/*/deep?.md", "notes/x/y/deep1.md"},
		{"notes/*/deep?.md", "notes/x/deep.md"},
		// Multi-byte characters ('?' matches one character, not one byte).
		{"?.md", "あ.md"},
		{"??.md", "あ.md"},
		{"メモ*", "メモ帳.md"},
		{"メモ?", "メモ帳"},
		// Path-like patterns used in configs.
		{"archive/*", "archive/2024/note.md"},
		{"*/drafts/*", "work/drafts/a.md"},
		{"*/drafts/*", "drafts/a.md"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_vs_%s", tc.pattern, tc.s), func(t *testing.T) {
			goGot := globMatch(tc.pattern, tc.s)

			var sqlGot bool
			if err := db.QueryRow("SELECT ? GLOB ?", tc.s, tc.pattern).Scan(&sqlGot); err != nil {
				t.Fatalf("sqlite glob: %v", err)
			}

			if goGot != sqlGot {
				t.Errorf("globMatch(%q, %q) = %v, SQLite GLOB = %v", tc.pattern, tc.s, goGot, sqlGot)
			}
		})
	}
}
