package core

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestMovedEdgeTargetsPreserveLookupMeaning(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := initSchema(db); err != nil {
		t.Fatal(err)
	}
	for i, path := range []any{"Source.md", "First.md", "Second.md", nil} {
		if _, err := db.Exec(`INSERT INTO nodes (id,node_key,type,name,path) VALUES (?,?,?,?,?)`, i+1, fmt.Sprint(i), NodeTypeNote, fmt.Sprint(i), path); err != nil {
			t.Fatal(err)
		}
	}
	// Insert the higher target first: the old query's first row need not be
	// insertion order. Include NULL targets and duplicates with the same target.
	fixtures := []struct {
		target int
		raw    string
		line   any
		table  int
		kind   LinkType
	}{
		{3, "[[same]]", 1, 0, LinkTypeWikilink}, {2, "[[same]]", 1, 0, LinkTypeWikilink},
		{3, "[[same]]", 2, 0, LinkTypeWikilink}, {4, "[[same]]", 1, 1, LinkTypeWikilink},
		{2, "[[same]]", 1, 1, LinkTypeWikilink}, {2, "[[same]]", 3, 0, LinkTypeTag},
		{4, "[[missing]]", 4, 0, LinkTypeMarkdown}, {3, "[[other]]", 1, 0, LinkTypeFrontmatterWikilink},
		{2, "[[repeat]]", 5, 0, LinkTypeWikilink}, {2, "[[repeat]]", 5, 0, LinkTypeMarkdown},
		{2, "[[null-line]]", nil, 0, LinkTypeWikilink},
	}
	for _, f := range fixtures {
		if _, err := db.Exec(`INSERT INTO edges(source_id,target_id,raw_link,line_start,in_table,link_type) VALUES(1,?,?,?,?,?)`, f.target, f.raw, f.line, f.table, f.kind); err != nil {
			t.Fatal(err)
		}
	}
	for _, analyzed := range []bool{false, true} {
		t.Run(fmt.Sprintf("analyzed=%t", analyzed), func(t *testing.T) {
			if analyzed {
				if _, err := db.Exec("ANALYZE"); err != nil {
					t.Fatal(err)
				}
			}
			targets, err := loadMovedEdgeTargets(db, 1)
			if err != nil {
				t.Fatal(err)
			}
			cases := []struct {
				key  movedEdgeKey
				want string
			}{
				{movedEdgeKey{"[[same]]", 2, false}, "Second.md"},
				{movedEdgeKey{"[[same]]", 3, false}, ""},
				{movedEdgeKey{"[[missing]]", 4, false}, ""},
				{movedEdgeKey{"[[other]]", 1, false}, "Second.md"},
				{movedEdgeKey{"[[repeat]]", 5, false}, "First.md"},
				{movedEdgeKey{"[[absent]]", 1, false}, ""},
				{movedEdgeKey{"[[null-line]]", 0, false}, ""},
			}
			for _, c := range cases {
				if got := targets[c.key]; got != c.want {
					t.Errorf("%+v = %q, want %q", c.key, got, c.want)
				}
			}
			// Ambiguous keys must keep the actual previous SQL selection, including
			// a phantom chosen ahead of an existing target under some query plans.
			for _, key := range []movedEdgeKey{{"[[same]]", 1, false}, {"[[same]]", 1, true}} {
				want, err := lookupEdgeTargetPath(db, 1, key.rawLink, key.lineStart, key.inTable)
				if err != nil {
					t.Fatal(err)
				}
				if got := targets[key]; got != want {
					t.Errorf("%+v = %q, old lookup = %q", key, got, want)
				}
			}
		})
	}
}
