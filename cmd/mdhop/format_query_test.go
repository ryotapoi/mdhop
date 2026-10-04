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
		Backlinks: []core.NodeInfo{},
		TwoHop: []core.TwoHopEntry{{
			NodeInfo:       core.NodeInfo{Type: core.NodeTypeNote, Name: "B", Path: "B.md", Exists: false},
			Relation:       []core.NodeInfo{{Type: core.NodeTypeTag, Name: "#shared"}},
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
			NodeInfo:       core.NodeInfo{Type: core.NodeTypeNote, Name: "B", Path: "B.md", Exists: true},
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
		Backlinks: []core.NodeInfo{},
		TwoHop: []core.TwoHopEntry{{
			NodeInfo:       core.NodeInfo{Type: core.NodeTypeNote, Path: "d.md"},
			Relation:       []core.NodeInfo{{Type: core.NodeTypeTag, Name: "#x,y"}},
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
