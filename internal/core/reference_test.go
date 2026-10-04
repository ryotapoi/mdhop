package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func referenceVault(t *testing.T, files map[string]string) string {
	t.Helper()
	v := t.TempDir()
	for p, c := range files {
		full := filepath.Join(v, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Build(v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReferenceSnapshotRelationsAndUpdate(t *testing.T) {
	v := referenceVault(t, map[string]string{"A.md": "[Read][guide] [guide][] [guide]\n[guide]: B.md#Heading\n[missing]\n[unused]: Never.md", "B.md": "", "C.md": "[also][id]\n[id]: B.md"})
	for _, raw := range []string{"[Read][guide]", "[guide][]", "[guide]"} {
		r, err := Resolve(v, "A.md", raw)
		if err != nil || r.Path != "B.md" || r.Subpath != "#Heading" {
			t.Fatalf("resolve %s: %+v %v", raw, r, err)
		}
	}
	if _, err := Resolve(v, "A.md", "[missing]"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("undefined: %v", err)
	}
	if _, err := Resolve(v, "A.md", "[Other][guide]"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("unindexed: %v", err)
	}
	q, err := Query(v, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing, FieldQueryTwoHop}})
	if err != nil || len(q.Outgoing) != 1 || q.Outgoing[0].Path != "B.md" || len(q.TwoHop) != 1 || q.TwoHop[0].Path != "C.md" {
		t.Fatalf("relations: %+v %v", q, err)
	}
	b, err := Query(v, EntrySpec{File: "B.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}})
	if err != nil || len(b.Backlinks) != 2 {
		t.Fatalf("backlinks: %+v %v", b, err)
	}
	reachable, err := Reachable(v, ReachableOptions{From: "A.md"})
	if err != nil || !reflect.DeepEqual(reachable.Reachable, []string{"A.md", "B.md"}) {
		t.Fatalf("reachable: %+v %v", reachable, err)
	}
	db := openTestDB(t, dbPath(v))
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE link_type=? AND reference_target='B.md#Heading' AND line_start=1", LinkTypeMarkdownReference).Scan(&count); err != nil || count != 3 {
		t.Fatalf("snapshot columns %d: %v", count, err)
	}
	db.Close()
	before, err := os.ReadFile(dbPath(v))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(v, "A.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(v, "A.md", "[guide]"); err != nil {
		t.Fatal(err)
	}
	if _, err := Query(v, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(dbPath(v))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("snapshot reads changed DB")
	}
	if err := os.WriteFile(filepath.Join(v, "A.md"), []byte("[guide]\n[guide]: New.md"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(v, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(v, "A.md", "[guide]")
	if err != nil || r.Type != NodeTypePhantom || r.Name != "New" {
		t.Fatalf("changed definition: %+v %v", r, err)
	}
	writeStaleTestFile(t, filepath.Join(v, "A.md"), []byte("[guide]"))
	if _, err := Update(v, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(v, "A.md", "[guide]"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("removed definition: %v", err)
	}
}

func TestReferencePhantomPromotionAndMoveGuard(t *testing.T) {
	v := referenceVault(t, map[string]string{"A.md": "[guide]\n[guide]: B.md", "C.md": ""})
	r, err := Resolve(v, "A.md", "[guide]")
	if err != nil || r.Name != "B" || r.Type != NodeTypePhantom {
		t.Fatalf("phantom: %+v %v", r, err)
	}
	if err := os.WriteFile(filepath.Join(v, "B.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(v, AddOptions{Files: []string{"B.md"}}); err != nil {
		t.Fatal(err)
	}
	r, err = Resolve(v, "A.md", "[guide]")
	if err != nil || r.Path != "B.md" {
		t.Fatalf("promotion: %+v %v", r, err)
	}
	before, err := os.ReadFile(dbPath(v))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Move(v, MoveOptions{From: "B.md", To: "Renamed.md"}); err == nil {
		t.Fatal("target rename must fail")
	}
	after, err := os.ReadFile(dbPath(v))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed move changed DB")
	}
	if _, err := os.Stat(filepath.Join(v, "B.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v, "Renamed.md")); !os.IsNotExist(err) {
		t.Fatalf("failed move created target: %v", err)
	}
	// A basename destination remains valid when its source moves.
	if _, err := Move(v, MoveOptions{From: "A.md", To: "sub/A.md"}); err != nil {
		t.Fatal(err)
	}
	r, err = Resolve(v, "sub/A.md", "[guide]")
	if err != nil || r.Path != "B.md" {
		t.Fatalf("moved reference: %+v %v", r, err)
	}
}

func TestReferenceMutationMeaningGuard(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		from, to, add string
		wantError     bool
	}{
		{"source relative", map[string]string{"sub/A.md": "[g]\n[g]: ./B.md", "sub/B.md": ""}, "sub/A.md", "other/A.md", "", true},
		{"directory relative preserved", map[string]string{"sub/A.md": "[g]\n[g]: ./B.md", "sub/B.md": ""}, "sub", "other", "", false},
		{"directory vault path changes", map[string]string{"A.md": "[g]\n[g]: sub/B.md", "sub/B.md": ""}, "sub", "other", "", true},
		{"add root changes target", map[string]string{"A.md": "[g]\n[g]: B.md", "sub/B.md": ""}, "", "", "B.md", true},
		{"add ambiguous basename", map[string]string{"A.md": "[g]\n[g]: B.md", "sub/B.md": ""}, "", "", "other/B.md", true},
		{"add unrelated", map[string]string{"A.md": "[g]\n[g]: B.md", "B.md": ""}, "", "", "D.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := referenceVault(t, tt.files)
			if tt.add != "" {
				full := filepath.Join(v, tt.add)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(dbPath(v))
			if err != nil {
				t.Fatal(err)
			}
			if tt.add != "" {
				_, err = Add(v, AddOptions{Files: []string{tt.add}, AutoDisambiguate: true})
			} else if tt.from == "sub" {
				_, err = MoveDir(v, MoveDirOptions{FromDir: tt.from, ToDir: tt.to})
			} else {
				_, err = Move(v, MoveOptions{From: tt.from, To: tt.to})
			}
			if (err != nil) != tt.wantError {
				t.Fatalf("error=%v wantError=%v", err, tt.wantError)
			}
			if tt.wantError {
				after, e := os.ReadFile(dbPath(v))
				if e != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("guard changed DB")
				}
				for p, c := range tt.files {
					contents, e := os.ReadFile(filepath.Join(v, p))
					if e != nil || string(contents) != c {
						t.Fatalf("guard changed %s: %v", p, e)
					}
				}
			} else if tt.from != "" {
				r, e := Resolve(v, "other/A.md", "[g]")
				if e != nil || r.Path != "other/B.md" {
					t.Fatalf("preserved relative reference: %+v %v", r, e)
				}
			}
		})
	}
}

func TestReferenceOldSchemaRebuild(t *testing.T) {
	v := referenceVault(t, map[string]string{"A.md": "[g]\n[g]: B.md", "B.md": ""})
	db := openTestDB(t, dbPath(v))
	if _, err := db.Exec("ALTER TABLE edges DROP COLUMN reference_target"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Resolve(v, "A.md", "[g]"); err == nil {
		t.Fatal("old schema must require build")
	}
	if _, err := Build(v); err != nil {
		t.Fatal(err)
	}
	if r, err := Resolve(v, "A.md", "[g]"); err != nil || r.Path != "B.md" {
		t.Fatalf("rebuilt: %+v %v", r, err)
	}
}

func TestReferenceGraphAndConvertPreservation(t *testing.T) {
	content := "[g] [normal](B.md) [self](#Heading)\n[g]: B.md\n"
	v := referenceVault(t, map[string]string{"A.md": content, "B.md": ""})
	g, err := Graph(v, GraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var types []LinkType
	for _, edge := range g.Edges {
		types = append(types, edge.LinkType)
	}
	if !reflect.DeepEqual(types, []LinkType{LinkTypeMarkdown, LinkTypeMarkdownReference}) {
		t.Fatalf("graph link types: %v", types)
	}
	if _, err := Convert(v, ConvertOptions{ToFormat: "wikilink"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(v, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[g] [[B|normal]] [[#Heading|self]]\n[g]: B.md\n"
	if string(got) != want {
		t.Fatalf("convert changed reference: %s", got)
	}
	if _, err := Build(v); err != nil {
		t.Fatal(err)
	}
	if r, err := Resolve(v, "A.md", "[g]"); err != nil || r.Path != "B.md" {
		t.Fatalf("rebuilt converted reference: %+v %v", r, err)
	}
}

func TestReferenceUnsupportedMultilineDoesNotCreateRelations(t *testing.T) {
	for _, targetExists := range []bool{true, false} {
		name := "existing destination"
		if !targetExists {
			name = "missing destination"
		}
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"A.md": "[text\n[id]\n][id]\n[id]: B.md\n"}
			if targetExists {
				files["B.md"] = ""
			}
			v := referenceVault(t, files)
			q, err := Query(v, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryOutgoing}})
			if err != nil || len(q.Outgoing) != 0 {
				t.Fatalf("unsupported construct produced outgoing: %+v %v", q, err)
			}
			if _, err := Resolve(v, "A.md", "[id]"); !errors.Is(err, ErrLinkNotFound) {
				t.Fatalf("partial shortcut resolve: %v", err)
			}
			db := openTestDB(t, dbPath(v))
			defer db.Close()
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type = ?", NodeTypePhantom).Scan(&count); err != nil || count != 0 {
				t.Fatalf("unsupported construct produced %d phantoms: %v", count, err)
			}
		})
	}
}
