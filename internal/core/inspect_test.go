package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestInspectIndexedAttributesAndHead(t *testing.T) {
	vault := t.TempDir()
	for path, body := range map[string]string{
		"A.md":     "---\nstatus: draft\nlabels: [z, a]\nraw: {nested: ignored}\n---\n\n# Plan\n#parent/child [[Ghost]] [[image.png]]\n",
		"Empty.md": "", "image.png": "asset",
		"mdhop.yaml": "meta:\n  types:\n    status: string\n    labels: string\nquery:\n  hide:\n    paths: ['**']\n    tags: ['#parent']\nexclude:\n  tags: ['#parent']\n",
	} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildForQuery(t, vault)
	n := 1
	for _, fields := range [][]string{nil, {"tags"}, {"meta"}} {
		r, err := Inspect(vault, "A.md", InspectOptions{Fields: fields, IncludeHead: &n})
		if err != nil {
			t.Fatal(err)
		}
		if r.Entry.Type != NodeTypeNote || r.Entry.Path != "A.md" || !r.Entry.Exists {
			t.Fatalf("entry: %+v", r.Entry)
		}
		if !reflect.DeepEqual(r.Head, []string{"# Plan"}) {
			t.Fatalf("head: %v", r.Head)
		}
		if fields == nil || fields[0] == "tags" {
			if !reflect.DeepEqual(r.Tags, []string{"#parent/child"}) {
				t.Fatalf("tags: %v", r.Tags)
			}
		} else if r.Tags != nil {
			t.Fatal("unselected tags")
		}
		if fields == nil || fields[0] == "meta" {
			if !reflect.DeepEqual(r.Meta, map[string][]string{"status": {"draft"}, "labels": {"a", "z"}}) {
				t.Fatalf("meta: %v", r.Meta)
			}
		} else if r.Meta != nil {
			t.Fatal("unselected meta")
		}
	}
	maxLines := int(^uint(0) >> 1)
	rAll, err := Inspect(vault, "A.md", InspectOptions{IncludeHead: &maxLines})
	if err != nil || len(rAll.Head) != 2 {
		t.Fatalf("large head: %+v %v", rAll, err)
	}
	q, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{"outgoing"}})
	if err != nil {
		t.Fatal(err)
	}
	parent := false
	for _, node := range q.Outgoing {
		if node.Name == "#parent" {
			parent = true
		}
	}
	if !parent {
		t.Fatal("query should include indexed parent tag")
	}
	r, err := Inspect(vault, "Empty.md", InspectOptions{IncludeHead: &n})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tags == nil || r.Meta == nil || r.Head == nil || len(r.Tags)+len(r.Meta)+len(r.Head) != 0 {
		t.Fatalf("empty: %+v", r)
	}
	for _, path := range []string{"Missing.md", "image.png", "Ghost", "#parent"} {
		if _, err := Inspect(vault, path, InspectOptions{}); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	if err := os.Chtimes(filepath.Join(vault, "A.md"), time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(vault, "A.md", InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(vault, "A.md", InspectOptions{IncludeHead: &n}); !errors.Is(err, ErrSourceStale) {
		t.Fatalf("stale: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(vault, "A.md", InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(vault, "A.md", InspectOptions{IncludeHead: &n}); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestInspectValidation(t *testing.T) {
	for _, fields := range [][]string{{}, {""}, {"tags", ""}, {"tags", "tags"}, {"head"}, {"unknown"}} {
		if _, err := Inspect(t.TempDir(), "A.md", InspectOptions{Fields: fields}); err == nil {
			t.Fatalf("accepted %v", fields)
		}
	}
	for _, n := range []int{0, -1} {
		if _, err := Inspect(t.TempDir(), "A.md", InspectOptions{IncludeHead: &n}); err == nil {
			t.Fatalf("accepted head %d", n)
		}
	}
	if _, err := Inspect(t.TempDir(), "", InspectOptions{}); err == nil {
		t.Fatal("accepted missing file")
	}
	vault := t.TempDir()
	if _, err := Inspect(vault, "A.md", InspectOptions{}); err == nil {
		t.Fatal("accepted no index")
	}
	entries, err := os.ReadDir(vault)
	if err != nil || len(entries) != 0 {
		t.Fatalf("created files: %v %v", entries, err)
	}
}
