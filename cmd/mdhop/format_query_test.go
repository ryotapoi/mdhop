package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestPrintQueryJSON_SelectedEmptyAndTypedRelations(t *testing.T) {
	result := &core.QueryResult{
		Entry:     core.NodeInfo{Type: core.NodeTypeNote, Name: "A", Path: "A.md", Exists: true},
		Backlinks: []core.QueryNode{},
		TwoHop: []core.TwoHopEntry{{
			QueryNode:      core.QueryNode{NodeInfo: core.NodeInfo{Type: core.NodeTypeNote, Name: "B", Path: "B.md", Exists: false}},
			Relation:       []core.QueryNode{{NodeInfo: core.NodeInfo{Type: core.NodeTypeTag, Name: "#shared"}}},
			HiddenRelation: true,
		}},
	}
	var buf bytes.Buffer
	if err := printQueryJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, buf.Bytes(), `{
		"entry":{"type":"note","name":"A","path":"A.md","exists":true},
		"backlinks":[],
		"2hoplink":[{"type":"note","name":"B","path":"B.md","exists":false,
			"relation":[{"type":"tag","name":"#shared"}],"hidden_relation":true}],
		"page":{"offset":0,"limit":null,"next_offset":null}
	}`)
}

func TestPrintQueryJSON_HiddenOnlyAndPage(t *testing.T) {
	limit, next := 1, 2
	result := &core.QueryResult{
		Entry: core.NodeInfo{Type: core.NodeTypeTag, Name: "#entry"},
		TwoHop: []core.TwoHopEntry{{
			QueryNode:      core.QueryNode{NodeInfo: core.NodeInfo{Type: core.NodeTypeNote, Name: "B", Path: "B.md", Exists: true}},
			HiddenRelation: true,
		}},
		Page: core.QueryPage{Offset: 1, Limit: &limit, NextOffset: &next},
	}
	var buf bytes.Buffer
	if err := printQueryJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, buf.Bytes(), `{
		"entry":{"type":"tag","name":"#entry"},
		"2hoplink":[{"type":"note","name":"B","path":"B.md","exists":true,
			"relation":[],"hidden_relation":true}],
		"page":{"offset":1,"limit":1,"next_offset":2}
	}`)
}

func TestPrintQueryText_SelectionAndEscaping(t *testing.T) {
	result := &core.QueryResult{
		Entry:     core.NodeInfo{Type: core.NodeTypeNote, Path: "a,\tb\n\"c\".md"},
		Backlinks: []core.QueryNode{},
		TwoHop: []core.TwoHopEntry{{
			QueryNode:      core.QueryNode{NodeInfo: core.NodeInfo{Type: core.NodeTypeNote, Path: "d.md"}},
			Relation:       []core.QueryNode{{NodeInfo: core.NodeInfo{Type: core.NodeTypeTag, Name: "#x,y"}}},
			HiddenRelation: true,
		}},
	}
	var buf bytes.Buffer
	if err := printQueryText(&buf, result); err != nil {
		t.Fatal(err)
	}
	want := "entry:\n  note \"a,\\tb\\n\\\"c\\\".md\"\n" +
		"backlinks:\n  （該当なし）\n" +
		"2hoplink:\n  note \"d.md\"\n    relation:\n      tag \"#x,y\"\n    hidden_relation: true\n" +
		"page:\n  offset: 0\n  limit: null\n  next_offset: null\n"
	if got := buf.String(); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	if !json.Valid([]byte(strings.TrimSpace(strings.TrimPrefix(strings.Split(want, "\n")[1], "  note ")))) {
		t.Error("entry identifier should follow JSON string quoting")
	}
}

func TestPrintQueryPreviews(t *testing.T) {
	snippet := []core.SnippetEntry{{SourcePath: "s,\t\n\".md", LineStart: 2, LineEnd: 2, Lines: []string{"[[x]]\t\"raw\""}}}
	r := &core.QueryResult{Entry: core.NodeInfo{Type: core.NodeTypeNote, Path: "Entry.md"},
		Backlinks: []core.QueryNode{{NodeInfo: core.NodeInfo{Type: core.NodeTypeNote, Path: "Target.md"}, Head: []string{}, Snippet: snippet}},
		Outgoing:  []core.QueryNode{{NodeInfo: core.NodeInfo{Type: core.NodeTypeTag, Name: "#tag"}, Snippet: []core.SnippetEntry{}}},
		TwoHop:    []core.TwoHopEntry{{QueryNode: core.QueryNode{NodeInfo: core.NodeInfo{Type: core.NodeTypeNote, Path: "Peer.md"}, Head: []string{"# Peer"}}, Relation: []core.QueryNode{{NodeInfo: core.NodeInfo{Type: core.NodeTypeTag, Name: "#via"}, Snippet: snippet}}}}}
	var buf bytes.Buffer
	if err := printQueryJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, buf.Bytes(), `{"entry":{"type":"note","name":"","path":"Entry.md","exists":false},"backlinks":[{"type":"note","name":"","path":"Target.md","exists":false,"head":[],"snippet":[{"source_path":"s,\t\n\".md","start_line":2,"end_line":2,"lines":["[[x]]\t\"raw\""]}]}],"outgoing":[{"type":"tag","name":"#tag","snippet":[]}],"2hoplink":[{"type":"note","name":"","path":"Peer.md","exists":false,"head":["# Peer"],"relation":[{"type":"tag","name":"#via","snippet":[{"source_path":"s,\t\n\".md","start_line":2,"end_line":2,"lines":["[[x]]\t\"raw\""]}]}],"hidden_relation":false}],"page":{"offset":0,"limit":null,"next_offset":null}}`)
	buf.Reset()
	if err := printQueryText(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := "entry:\n  note \"Entry.md\"\nbacklinks:\n  note \"Target.md\"\n    head: []\n    snippet:\n      source_path: \"s,\\t\\n\\\".md\"\n      start_line: 2\n      end_line: 2\n      lines:\n        \"[[x]]\\t\\\"raw\\\"\"\noutgoing:\n  tag \"#tag\"\n    snippet: []\n2hoplink:\n  note \"Peer.md\"\n    head:\n      \"# Peer\"\n    relation:\n      tag \"#via\"\n        snippet:\n          source_path: \"s,\\t\\n\\\".md\"\n          start_line: 2\n          end_line: 2\n          lines:\n            \"[[x]]\\t\\\"raw\\\"\"\npage:\n  offset: 0\n  limit: null\n  next_offset: null\n"
	if buf.String() != want {
		t.Fatalf("text=%q want=%q", buf.String(), want)
	}
	for _, print := range []func(*failingWriter, *core.QueryResult) error{
		func(w *failingWriter, r *core.QueryResult) error { return printQueryText(w, r) },
		func(w *failingWriter, r *core.QueryResult) error { return printQueryJSON(w, r) },
	} {
		if err := print(&failingWriter{failAt: 1}, r); err == nil {
			t.Fatal("missing writer error")
		}
	}
}
