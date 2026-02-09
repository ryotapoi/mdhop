package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"type", []string{"type"}},
		{"type,name", []string{"type", "name"}},
		{" type , name , path ", []string{"type", "name", "path"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		got := parseFields(tt.input)
		if tt.want == nil && got != nil {
			t.Errorf("parseFields(%q) = %v, want nil", tt.input, got)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("parseFields(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseFields(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"json", false},
		{"text", false},
		{"yaml", true},
		{"", true},
	}
	for _, tt := range tests {
		err := validateFormat(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestValidateResolveFields(t *testing.T) {
	tests := []struct {
		fields  []string
		wantErr bool
	}{
		{nil, false},
		{[]string{"type", "name"}, false},
		{[]string{"type", "invalid"}, true},
	}
	for _, tt := range tests {
		err := validateResolveFields(tt.fields)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateResolveFields(%v) error = %v, wantErr %v", tt.fields, err, tt.wantErr)
		}
	}
}

func TestPrintResolveText_Note(t *testing.T) {
	r := &core.ResolveResult{
		Type: "note", Name: "Design", Path: "Notes/Design.md",
		Exists: true, Subpath: "#Overview",
	}
	var buf bytes.Buffer
	printResolveText(&buf, r, nil)
	got := buf.String()

	want := "type: note\nname: Design\npath: Notes/Design.md\nexists: true\nsubpath: #Overview\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintResolveText_Phantom(t *testing.T) {
	r := &core.ResolveResult{
		Type: "phantom", Name: "MissingNote", Path: "", Exists: false,
	}
	var buf bytes.Buffer
	printResolveText(&buf, r, nil)
	got := buf.String()

	// path should be omitted for phantom, exists should still show
	if strings.Contains(got, "path:") {
		t.Errorf("phantom should not have path, got:\n%s", got)
	}
	if !strings.Contains(got, "type: phantom") {
		t.Errorf("should contain type: phantom, got:\n%s", got)
	}
}

func TestPrintResolveText_Fields(t *testing.T) {
	r := &core.ResolveResult{
		Type: "note", Name: "A", Path: "A.md", Exists: true, Subpath: "#h",
	}
	var buf bytes.Buffer
	printResolveText(&buf, r, []string{"type", "path"})
	got := buf.String()

	if !strings.Contains(got, "type: note") {
		t.Error("should contain type")
	}
	if !strings.Contains(got, "path: A.md") {
		t.Error("should contain path")
	}
	if strings.Contains(got, "name:") {
		t.Error("should not contain name when not in fields")
	}
}

func TestPrintResolveJSON_Note(t *testing.T) {
	r := &core.ResolveResult{
		Type: "note", Name: "Design", Path: "Notes/Design.md",
		Exists: true, Subpath: "#Overview",
	}
	var buf bytes.Buffer
	printResolveJSON(&buf, r, nil)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "note" {
		t.Errorf("type = %v", m["type"])
	}
	if m["path"] != "Notes/Design.md" {
		t.Errorf("path = %v", m["path"])
	}
	if m["subpath"] != "#Overview" {
		t.Errorf("subpath = %v", m["subpath"])
	}
}

func TestPrintResolveJSON_Fields(t *testing.T) {
	r := &core.ResolveResult{
		Type: "note", Name: "A", Path: "A.md", Exists: true,
	}
	var buf bytes.Buffer
	printResolveJSON(&buf, r, []string{"type", "name"})

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["path"]; ok {
		t.Error("path should be omitted when not in fields")
	}
}

func TestPrintQueryText_Full(t *testing.T) {
	r := &core.QueryResult{
		Entry:     core.NodeInfo{Type: "note", Name: "Index", Path: "Index.md", Exists: true},
		Backlinks: []core.NodeInfo{{Type: "note", Name: "Design", Path: "Notes/Design.md", Exists: true}},
		Outgoing:  []core.NodeInfo{{Type: "note", Name: "B", Path: "Notes/B.md", Exists: true}},
		Tags:      []string{"#project"},
		TwoHop: []core.TwoHopEntry{{
			Via:     core.NodeInfo{Type: "note", Name: "Design", Path: "Notes/Design.md", Exists: true},
			Targets: []core.NodeInfo{{Type: "note", Name: "Spec", Path: "Notes/Spec.md", Exists: true}},
		}},
		Head:     []string{"# Index", "This is the main index."},
		Snippets: []core.SnippetEntry{{SourcePath: "Notes/Design.md", LineStart: 5, LineEnd: 7, Lines: []string{"Before", "See [[Index]]", "After"}}},
	}
	var buf bytes.Buffer
	printQueryText(&buf, r)
	got := buf.String()

	checks := []string{
		"entry:\n  type: note\n  name: Index\n  path: Index.md",
		"backlinks:\n- type: note\n  name: Design",
		"outgoing:\n- type: note",
		"tags:\n- #project",
		"twohop:\n- via: note: Notes/Design.md\n  targets:\n  - note: Notes/Spec.md",
		"head:\n- \"# Index\"",
		"snippet:\n- source: Notes/Design.md\n  lines: 5-7",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("text output missing %q:\n%s", c, got)
		}
	}
}

func TestPrintQueryText_NilSections(t *testing.T) {
	r := &core.QueryResult{
		Entry: core.NodeInfo{Type: "tag", Name: "#project"},
	}
	var buf bytes.Buffer
	printQueryText(&buf, r)
	got := buf.String()

	if strings.Contains(got, "backlinks:") {
		t.Error("nil backlinks should be omitted")
	}
	if strings.Contains(got, "tags:") {
		t.Error("nil tags should be omitted")
	}
}

func TestPrintQueryJSON_Full(t *testing.T) {
	r := &core.QueryResult{
		Entry:     core.NodeInfo{Type: "note", Name: "Index", Path: "Index.md", Exists: true},
		Backlinks: []core.NodeInfo{{Type: "note", Name: "Design", Path: "Notes/Design.md", Exists: true}},
		Tags:      []string{"#project"},
		TwoHop: []core.TwoHopEntry{{
			Via:     core.NodeInfo{Type: "phantom", Name: "MissingConcept"},
			Targets: []core.NodeInfo{{Type: "note", Name: "Spec", Path: "Notes/Spec.md", Exists: true}},
		}},
	}
	var buf bytes.Buffer
	printQueryJSON(&buf, r)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["entry"] == nil {
		t.Error("entry should be present")
	}
	if m["backlinks"] == nil {
		t.Error("backlinks should be present")
	}
	if m["tags"] == nil {
		t.Error("tags should be present")
	}

	// twohop via should be phantom
	twohop := m["twohop"].([]any)
	via := twohop[0].(map[string]any)["via"].(map[string]any)
	if via["type"] != "phantom" {
		t.Errorf("twohop via type = %v", via["type"])
	}
	// phantom should not have path or exists
	if _, ok := via["path"]; ok {
		t.Error("phantom via should not have path")
	}
}

func TestPrintQueryJSON_ExistsFalse(t *testing.T) {
	r := &core.QueryResult{
		Entry:     core.NodeInfo{Type: "note", Name: "A", Path: "A.md", Exists: false},
		Backlinks: []core.NodeInfo{{Type: "note", Name: "B", Path: "B.md", Exists: false}},
	}
	var buf bytes.Buffer
	printQueryJSON(&buf, r)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	entry := m["entry"].(map[string]any)
	if _, ok := entry["exists"]; !ok {
		t.Error("exists should be present for note even when false")
	}
	if entry["exists"] != false {
		t.Errorf("exists = %v, want false", entry["exists"])
	}
	bls := m["backlinks"].([]any)
	bl := bls[0].(map[string]any)
	if _, ok := bl["exists"]; !ok {
		t.Error("backlink exists should be present for note even when false")
	}
}

func TestPrintResolveText_Tag(t *testing.T) {
	r := &core.ResolveResult{Type: "tag", Name: "#project"}
	var buf bytes.Buffer
	printResolveText(&buf, r, nil)
	got := buf.String()

	if !strings.Contains(got, "type: tag") {
		t.Error("should contain type: tag")
	}
	if !strings.Contains(got, "name: #project") {
		t.Error("should contain name: #project")
	}
	if strings.Contains(got, "path:") {
		t.Error("tag should not have path")
	}
	if strings.Contains(got, "subpath:") {
		t.Error("tag should not have subpath")
	}
}

func TestPrintResolveJSON_Phantom(t *testing.T) {
	r := &core.ResolveResult{Type: "phantom", Name: "Missing"}
	var buf bytes.Buffer
	printResolveJSON(&buf, r, nil)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "phantom" {
		t.Errorf("type = %v", m["type"])
	}
	if _, ok := m["path"]; ok {
		t.Error("phantom should not have path")
	}
}

func TestPrintQueryText_PhantomEntry(t *testing.T) {
	r := &core.QueryResult{
		Entry:     core.NodeInfo{Type: "phantom", Name: "Missing"},
		Backlinks: []core.NodeInfo{{Type: "note", Name: "A", Path: "A.md", Exists: true}},
	}
	var buf bytes.Buffer
	printQueryText(&buf, r)
	got := buf.String()

	if !strings.Contains(got, "entry:\n  type: phantom\n  name: Missing\n") {
		t.Errorf("phantom entry format wrong:\n%s", got)
	}
	// phantom entry should not have path or exists lines
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if strings.Contains(line, "type: phantom") {
			// next lines should not be path/exists for phantom
			if i+2 < len(lines) && strings.Contains(lines[i+1], "name: Missing") {
				if i+2 < len(lines) && (strings.HasPrefix(strings.TrimSpace(lines[i+2]), "path:") ||
					strings.HasPrefix(strings.TrimSpace(lines[i+2]), "exists:")) {
					t.Error("phantom should not have path/exists")
				}
			}
		}
	}
}

func TestPrintQueryJSON_NilSections(t *testing.T) {
	r := &core.QueryResult{
		Entry: core.NodeInfo{Type: "note", Name: "A", Path: "A.md", Exists: true},
	}
	var buf bytes.Buffer
	printQueryJSON(&buf, r)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["backlinks"]; ok {
		t.Error("nil backlinks should be omitted from JSON")
	}
	if _, ok := m["tags"]; ok {
		t.Error("nil tags should be omitted from JSON")
	}
}
