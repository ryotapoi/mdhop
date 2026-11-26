package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func locationVault(t *testing.T, files map[string]string) string {
	t.Helper()
	vault := t.TempDir()
	for path, content := range files {
		writeVaultFile(t, vault, path, content)
	}
	buildForQuery(t, vault)
	return vault
}

func loc(startLine, startColumn, endLine, endColumn int) *LinkLocation {
	return &LinkLocation{Start: LinkPosition{startLine, startColumn}, End: LinkPosition{endLine, endColumn}}
}

func assertLocation(t *testing.T, node QueryNode, raw string, want *LinkLocation) {
	t.Helper()
	if node.RawLink != raw {
		t.Fatalf("%s raw=%q, want %q", node.Path, node.RawLink, raw)
	}
	if (node.Location == nil) != (want == nil) || node.Location != nil && *node.Location != *want {
		t.Fatalf("%s location=%+v, want %+v", node.Path, node.Location, want)
	}
}

func TestQueryIncludeLocationBacklinks(t *testing.T) {
	vault := locationVault(t, map[string]string{
		"E.md":         "# E\n",
		"Other.md":     "# Other\n",
		"FM.md":        "---\ntitle: é\n---\nあい [[E]] x [[E]]\n",
		"CRLF.md":      "---\r\na: 1\r\n---\r\nx\r\n[[E]]\r\n",
		"Multi.md":     "[see\nthis](E.md)\n",
		"Code.md":      "`[[E]]` [[E]]\n",
		"Mixed.md":     "[x](E.md) [[E]]\n",
		"FMFirst.md":   "---\nrel: \"[[E]]\"\n---\nbody [[E]]\n",
		"Paragraph.md": "[[Other]]\n\n[[E]]\n",
		"InlineRef.md": "[guide](Other.md) [guide]\n\n[guide]: E.md\n",
		"Escaped.md":   "\\[guide] [guide]\n\n[guide]: E.md\n",
		"Nested.md":    "[[Other|[[E]]]] [[E]]\n",
		"CodeRef.md":   "[see `code`][guide]\n\n[guide]: E.md\n",
	})
	r, err := Query(vault, EntrySpec{File: "E.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, IncludeLocation: true})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		raw  string
		want *LinkLocation
	}{
		{"FM.md", "[[E]]", loc(4, 4, 4, 9)},
		{"CRLF.md", "[[E]]", loc(5, 1, 5, 6)},
		{"Multi.md", "[see\nthis](E.md)", loc(1, 1, 2, 12)},
		{"Code.md", "[[E]]", loc(1, 9, 1, 14)},
		{"Mixed.md", "[x](E.md)", loc(1, 1, 1, 10)},
		{"FMFirst.md", "[[E]]", nil},
		{"Paragraph.md", "[[E]]", loc(3, 1, 3, 6)},
		{"InlineRef.md", "[guide]", loc(1, 19, 1, 26)},
		{"Escaped.md", "[guide]", loc(1, 10, 1, 17)},
		{"Nested.md", "[[E]]", loc(1, 17, 1, 22)},
		{"CodeRef.md", "[see `code`][guide]", loc(1, 1, 1, 20)},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assertLocation(t, previewNode(t, r.Backlinks, tc.path), tc.raw, tc.want)
		})
	}
}

func TestQueryIncludeLocationTagSkipsWikilinkSubpath(t *testing.T) {
	vault := locationVault(t, map[string]string{
		"E.md": "# E\n",
		"T.md": "[[E#tag]] #tags #tag\n",
	})
	r, err := Query(vault, EntrySpec{Tag: "tag"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, IncludeLocation: true})
	if err != nil {
		t.Fatal(err)
	}
	assertLocation(t, previewNode(t, r.Backlinks, "T.md"), "#tag", loc(1, 17, 1, 21))
}

func TestQueryIncludeLocationTwoHopRelation(t *testing.T) {
	vault := locationVault(t, map[string]string{
		"X.md": "[[V]]\n",
		"Y.md": "---\na: 1\n---\nsee [[V|v]]\n",
		"V.md": "# V\n",
	})
	r, err := Query(vault, EntrySpec{File: "X.md"}, QueryOptions{IncludeLocation: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.TwoHop) != 1 || r.TwoHop[0].Path != "Y.md" {
		t.Fatalf("twohop=%+v", r.TwoHop)
	}
	assertLocation(t, r.TwoHop[0].QueryNode, "", nil)
	assertLocation(t, previewNode(t, r.TwoHop[0].Relation, "V.md"), "[[V|v]]", loc(4, 5, 4, 12))
	for _, node := range r.Outgoing {
		assertLocation(t, node, "", nil)
	}
}

func TestQueryIncludeLocationChangedSourcesKeepRawLink(t *testing.T) {
	vault := locationVault(t, map[string]string{
		"E.md":          "# E\n",
		"Deleted.md":    "[[E]]\n",
		"Touched.md":    "[[E]]\n",
		"Kept.md":       "[[E]]\n",
		"Replaced/A.md": "[[E]]\n",
	})
	if err := os.Remove(filepath.Join(vault, "Deleted.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(vault, "Replaced")); err != nil {
		t.Fatal(err)
	}
	writeVaultFile(t, vault, "Replaced", "regular file\n")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(vault, "Touched.md"), later, later); err != nil {
		t.Fatal(err)
	}
	r, err := Query(vault, EntrySpec{File: "E.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, IncludeLocation: true})
	if err != nil {
		t.Fatal(err)
	}
	assertLocation(t, previewNode(t, r.Backlinks, "Deleted.md"), "[[E]]", nil)
	assertLocation(t, previewNode(t, r.Backlinks, "Replaced/A.md"), "[[E]]", nil)
	assertLocation(t, previewNode(t, r.Backlinks, "Touched.md"), "[[E]]", nil)
	assertLocation(t, previewNode(t, r.Backlinks, "Kept.md"), "[[E]]", loc(1, 1, 1, 6))
}

func TestQueryWithoutIncludeLocationDoesNotReadSources(t *testing.T) {
	vault := locationVault(t, map[string]string{
		"E.md": "# E\n",
		"A.md": "[[E]]\n",
	})
	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatal(err)
	}
	r, err := Query(vault, EntrySpec{File: "E.md"}, QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertLocation(t, previewNode(t, r.Backlinks, "A.md"), "", nil)
}
