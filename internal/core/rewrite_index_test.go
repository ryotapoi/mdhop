package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// indexSnapshot excludes generated IDs and mtimes so incremental mutations
// can be compared directly with a full build of the same disk contents.
func indexSnapshot(t *testing.T, vault string) []string {
	t.Helper()
	db, err := openDBChecked(vault)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	queries := []string{
		`SELECT n.path,m.key,m.value,m.line,COALESCE(m.sort_value,''),COALESCE(m.value_type,'') FROM meta m JOIN nodes n ON n.id=m.node_id ORDER BY n.path,m.key,m.value,m.line`,
		`SELECT sn.path,tn.node_key,e.raw_link,e.link_type,COALESCE(e.frontmatter_key,''),COALESCE(e.subpath,''),e.line_start,e.line_end FROM edges e JOIN nodes sn ON sn.id=e.source_id JOIN nodes tn ON tn.id=e.target_id ORDER BY sn.path,tn.node_key,e.raw_link,e.line_start`,
		`SELECT node_key FROM nodes ORDER BY node_key`,
	}
	var snapshot []string
	for i, q := range queries {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for j := range vals {
				ptrs[j] = &vals[j]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			snapshot = append(snapshot, fmt.Sprintf("%d:%v", i, vals))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return snapshot
}

func TestRewriteMetadataIndexMatchesBuild(t *testing.T) {
	for _, command := range []string{"add", "move", "move-dir", "disambiguate"} {
		t.Run(command, func(t *testing.T) {
			vault := t.TempDir()
			write := func(path, content string) {
				t.Helper()
				full := filepath.Join(vault, path)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("mdhop.toml", "[build]\nexclude_paths = ['mdhop.toml']\n\n[meta]\n[meta.types]\nrevision = 'number'\n")
			write("sub/B.md", "---\nrelated: \"[[./C]]\"\nrevision: 4\n---\n[[./C]]\n")
			write("A.md", "---\nrelated: \"[[B]]\"\nrevision: 2\n---\n[[B]]\n")
			write("sub/C.md", "---\nrelated: \"[[./B]]\"\nrevision: 3\n---\n[[./B]]\n")
			if command == "move-dir" {
				write("A.md", "---\nrelated: \"[[sub/B]]\"\nrevision: 2\n---\n[[sub/B]]\n")
				write("sub/B.md", "---\nrelated: \"[[sub/C]]\"\nrevision: 4\n---\n[[sub/C]]\n")
			}
			if command == "disambiguate" {
				// Disambiguation must retain real assets even without incoming edges.
				write("unused.png", "unreferenced asset")
				write("A.md", "---\nrelated: \"[[missing/B]]\"\nrevision: 2\n---\n[[missing/B]]\n")
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			switch command {
			case "add":
				write("B.md", "# root B\n")
				if _, err := Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true}); err != nil {
					t.Fatal(err)
				}
			case "move":
				if _, err := Move(vault, MoveOptions{From: "sub/B.md", To: "new/Renamed.md"}); err != nil {
					t.Fatal(err)
				}
			case "move-dir":
				if _, err := MoveDir(vault, MoveDirOptions{FromDir: "sub", ToDir: "new"}); err != nil {
					t.Fatal(err)
				}
			case "disambiguate":
				if _, err := Disambiguate(vault, DisambiguateOptions{Name: "B", Target: "sub/B.md"}); err != nil {
					t.Fatal(err)
				}
			}
			check, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"related"}, Kind: MetaKindWikilink})
			if err != nil {
				t.Fatal(err)
			}
			if len(check.Issues) != 0 {
				t.Fatalf("metadata check after %s: %+v", command, check.Issues)
			}
			content, err := os.ReadFile(filepath.Join(vault, "A.md"))
			if err != nil {
				t.Fatal(err)
			}
			var related string
			for _, entry := range parseLinks(string(content)).Meta {
				if entry.Key == "related" {
					related = entry.Value
				}
			}
			where, err := ParseWhere([]string{"related=" + related}, MetaConfig{})
			if err != nil {
				t.Fatal(err)
			}
			search, err := Search(vault, SearchOptions{Where: where, Fields: []string{"meta"}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range search.Items {
				if item.Node.Path == "A.md" {
					found = true
				}
			}
			if !found {
				t.Fatalf("where search did not find rewritten metadata %q", related)
			}
			incremental := indexSnapshot(t, vault)
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			rebuilt := indexSnapshot(t, vault)
			if !reflect.DeepEqual(incremental, rebuilt) {
				t.Fatalf("incremental mismatch\n%v\n%v", incremental, rebuilt)
			}
		})
	}
}

func TestDisambiguatePhantomRespectsFileScope(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{"sub/B.md": "# B\n", "A.md": "[[missing/B]]\n", "C.md": "[[missing/B]]\n"} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if _, err := Disambiguate(vault, DisambiguateOptions{Name: "B", Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
	before := indexSnapshot(t, vault)
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	after := indexSnapshot(t, vault)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("scoped rewrite differs from build: %v / %v", before, after)
	}
}

func TestRewriteMetadataFailureRollsBack(t *testing.T) {
	vault := copyVault(t, "vault_add_disambiguate_frontmatter")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	before := indexSnapshot(t, vault)
	original, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDBChecked(vault)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER reject_meta BEFORE INSERT ON meta BEGIN SELECT RAISE(FAIL, 'metadata blocked'); END`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# root B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true}); err == nil {
		t.Fatal("expected metadata failure")
	}
	restored, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatal("disk rewrite not rolled back")
	}
	if after := indexSnapshot(t, vault); !reflect.DeepEqual(before, after) {
		t.Fatal("DB mutation not rolled back")
	}
}
