package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseLinksWithLinkKeysRecordsFrontmatterOrigin(t *testing.T) {
	content := "---\ntags: topic\nquoted: \"[[Quoted]]\"\nrelated: Raw\n---\n[[Body]]\n"
	links := parseLinksWithLinkKeys(content, []string{"related"}).Links

	got := make(map[string]string)
	for _, link := range links {
		got[link.rawLink] = link.frontmatterKey
	}
	want := map[string]string{
		"#topic":     "tags",
		"[[Quoted]]": "quoted",
		"Raw":        "related",
		"[[Body]]":   "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frontmatter origins = %#v, want %#v", got, want)
	}
}

func TestQueryLinkKeyFiltersOnlyDirectLinks(t *testing.T) {
	vault := t.TempDir()
	files := map[string]string{
		"Source.md": "---\ntags: topic\nquoted: \"[[Quoted]]\"\nrelated: Raw\n---\n[[Body]]\n",
		"Quoted.md": "# Quoted\n",
		"Raw.md":    "# Raw\n",
		"Body.md":   "# Body\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(vault, "mdhop.toml"), []byte("[meta]\nlink_keys = ['related']\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	for _, tt := range []struct {
		rawLink string
		key     sql.NullString
	}{
		{rawLink: "[[Body]]", key: sql.NullString{}},
		{rawLink: "[[Quoted]]", key: sql.NullString{String: "quoted", Valid: true}},
		{rawLink: "Raw", key: sql.NullString{String: "related", Valid: true}},
		{rawLink: "#topic", key: sql.NullString{String: "tags", Valid: true}},
	} {
		var got sql.NullString
		err := db.QueryRow(`SELECT frontmatter_key FROM edges WHERE raw_link = ?`, tt.rawLink).Scan(&got)
		if err != nil {
			t.Fatalf("frontmatter_key for %q: %v", tt.rawLink, err)
		}
		if got != tt.key {
			t.Errorf("frontmatter_key for %q = %#v, want %#v", tt.rawLink, got, tt.key)
		}
	}

	for _, tt := range []struct {
		name  string
		entry EntrySpec
		key   string
		field string
		paths []string
	}{
		{name: "outgoing quoted", entry: EntrySpec{File: "Source.md"}, key: "quoted", field: FieldQueryOutgoing, paths: []string{"Quoted.md"}},
		{name: "outgoing raw path", entry: EntrySpec{File: "Source.md"}, key: "related", field: FieldQueryOutgoing, paths: []string{"Raw.md"}},
		{name: "backlinks quoted", entry: EntrySpec{File: "Quoted.md"}, key: "quoted", field: FieldQueryBacklinks, paths: []string{"Source.md"}},
		{name: "backlinks body excluded", entry: EntrySpec{File: "Body.md"}, key: "related", field: FieldQueryBacklinks, paths: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Query(vault, tt.entry, QueryOptions{Relations: []string{tt.field}, LinkKey: tt.key})
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			var infos []QueryNode
			if tt.field == FieldQueryOutgoing {
				infos = result.Outgoing
			} else {
				infos = result.Backlinks
			}
			var got []string
			for _, info := range infos {
				got = append(got, info.Path)
			}
			if !reflect.DeepEqual(got, tt.paths) {
				t.Errorf("paths = %#v, want %#v", got, tt.paths)
			}
		})
	}

	if err := os.WriteFile(filepath.Join(vault, "Raw2.md"), []byte("# Raw2\n"), 0o644); err != nil {
		t.Fatalf("write Raw2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Source.md"), []byte("---\nrelated: Raw2\n---\n[[Body]]\n"), 0o644); err != nil {
		t.Fatalf("rewrite Source: %v", err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); err != nil {
		t.Fatalf("update Source: %v", err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"Raw2.md"}}); err != nil {
		t.Fatalf("add Raw2: %v", err)
	}
	result, err := Query(vault, EntrySpec{File: "Raw2.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, LinkKey: "related"})
	if err != nil {
		t.Fatalf("updated backlinks query: %v", err)
	}
	if len(result.Backlinks) != 1 || result.Backlinks[0].Path != "Source.md" {
		t.Errorf("updated backlinks = %#v, want Source.md only", result.Backlinks)
	}

	if err := os.WriteFile(filepath.Join(vault, "AddedSource.md"), []byte("---\nselected: \"[[Quoted]]\"\n---\n"), 0o644); err != nil {
		t.Fatalf("write AddedSource: %v", err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"AddedSource.md"}}); err != nil {
		t.Fatalf("add source with selected key: %v", err)
	}
	result, err = Query(vault, EntrySpec{File: "Quoted.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, LinkKey: "selected"})
	if err != nil {
		t.Fatalf("added-source backlinks query: %v", err)
	}
	if len(result.Backlinks) != 1 || result.Backlinks[0].Path != "AddedSource.md" {
		t.Errorf("added-source backlinks = %#v, want AddedSource.md only", result.Backlinks)
	}
}

func TestQueryLinkKeyComposesWithDirectQueryBehavior(t *testing.T) {
	vault := t.TempDir()
	files := map[string]string{
		"Target.md":      "# Target\n",
		"Source.md":      "---\nselected: \"[[Target]]\"\nother: \"[[Target]]\"\n---\n[[Target]]\n",
		"A.md":           "---\nother: \"[[Target]]\"\n---\n",
		"C.md":           "---\nselected: \"[[Target]]\"\n---\n",
		"dir/B.md":       "---\nselected: \"[[Target]]\"\n---\n",
		"move/Linker.md": "---\nselected: \"[[Target]]\"\n---\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(vault, path)), 0o755); err != nil {
			t.Fatalf("make parent for %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	paths := func(t *testing.T, entry EntrySpec, opts QueryOptions) []string {
		t.Helper()
		result, err := Query(vault, entry, opts)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		infos := result.Outgoing
		if opts.Relations[0] == FieldQueryBacklinks {
			infos = result.Backlinks
		}
		got := make([]string, 0, len(infos))
		for _, info := range infos {
			got = append(got, info.Path)
		}
		return got
	}

	// "selected" is intentionally absent from meta.link_keys: quoted
	// frontmatter wikilinks retain their own YAML key regardless of config.
	if got := paths(t, EntrySpec{File: "Source.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}, LinkKey: "selected"}); !reflect.DeepEqual(got, []string{"Target.md"}) {
		t.Errorf("selected outgoing = %#v, want one distinct Target.md", got)
	}
	if got := paths(t, EntrySpec{File: "Source.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}}); !reflect.DeepEqual(got, []string{"Target.md"}) {
		t.Errorf("omitted LinkKey outgoing = %#v, want unfiltered Target.md", got)
	}
	if got := paths(t, EntrySpec{File: "Source.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}, LinkKey: "missing"}); len(got) != 0 {
		t.Errorf("unknown LinkKey outgoing = %#v, want empty", got)
	}

	// A.md sorts first but has only the other key. The limit therefore proves
	// the key restriction is applied before ordering and limiting.
	if got := paths(t, EntrySpec{File: "Target.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, LinkKey: "selected", Limit: intPtr(1)}); !reflect.DeepEqual(got, []string{"C.md"}) {
		t.Errorf("limited selected backlinks = %#v, want C.md", got)
	}
	if got := paths(t, EntrySpec{File: "Target.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, LinkKey: "selected", Path: []string{"dir/*"}}); !reflect.DeepEqual(got, []string{"dir/B.md"}) {
		t.Errorf("path-filtered selected backlinks = %#v, want dir/B.md", got)
	}

	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "move", ToDir: "moved"}); err != nil {
		t.Fatalf("move dir: %v", err)
	}
	if got := paths(t, EntrySpec{File: "Target.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, LinkKey: "selected", Path: []string{"moved/*"}}); !reflect.DeepEqual(got, []string{"moved/Linker.md"}) {
		t.Errorf("moved selected backlinks = %#v, want moved/Linker.md", got)
	}
}
