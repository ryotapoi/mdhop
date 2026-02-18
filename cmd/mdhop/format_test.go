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

func TestValidateFields(t *testing.T) {
	cases := []struct {
		name    string
		fields  []string
		valid   map[string]bool
		label   string
		wantErr string // "" means no error
	}{
		{"resolve nil", nil, validResolveFields, "resolve", ""},
		{"resolve valid", []string{"type", "name"}, validResolveFields, "resolve", ""},
		{"resolve invalid", []string{"type", "invalid"}, validResolveFields, "resolve", "unknown resolve field: invalid"},
		{"stats valid", []string{"notes_total"}, validStatsFieldsCLI, "stats", ""},
		{"stats invalid", []string{"bad"}, validStatsFieldsCLI, "stats", "unknown stats field: bad"},
		{"diagnose valid", []string{"phantoms"}, validDiagnoseFieldsCLI, "diagnose", ""},
		{"diagnose invalid", []string{"bad"}, validDiagnoseFieldsCLI, "diagnose", "unknown diagnose field: bad"},
		{"query valid", []string{"backlinks", "tags"}, validQueryFieldsCLI, "query", ""},
		{"query invalid", []string{"bad"}, validQueryFieldsCLI, "query", "unknown query field: bad"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFields(tt.fields, tt.valid, tt.label)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("error = %v, want %q", err, tt.wantErr)
				}
			}
		})
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

// --- Mutation output tests ---

func TestPrintDeleteText(t *testing.T) {
	r := &core.DeleteResult{Deleted: []string{"C.md"}, Phantomed: []string{"B.md"}}
	var buf bytes.Buffer
	printDeleteText(&buf, r)
	got := buf.String()
	if !strings.Contains(got, "deleted:\n- C.md\n") {
		t.Errorf("missing deleted section:\n%s", got)
	}
	if !strings.Contains(got, "phantomed:\n- B.md\n") {
		t.Errorf("missing phantomed section:\n%s", got)
	}
}

func TestPrintDeleteText_Empty(t *testing.T) {
	r := &core.DeleteResult{}
	var buf bytes.Buffer
	printDeleteText(&buf, r)
	if buf.String() != "" {
		t.Errorf("expected empty output for empty result, got:\n%s", buf.String())
	}
}

func TestPrintDeleteJSON(t *testing.T) {
	r := &core.DeleteResult{Phantomed: []string{"B.md"}}
	var buf bytes.Buffer
	if err := printDeleteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	// deleted should be [] not null
	if string(m["deleted"]) != "[]" {
		t.Errorf("deleted = %s, want []", m["deleted"])
	}
	var phantomed []string
	if err := json.Unmarshal(m["phantomed"], &phantomed); err != nil {
		t.Fatal(err)
	}
	if len(phantomed) != 1 || phantomed[0] != "B.md" {
		t.Errorf("phantomed = %v, want [B.md]", phantomed)
	}
}

func TestPrintUpdateText(t *testing.T) {
	r := &core.UpdateResult{Updated: []string{"A.md"}, Deleted: []string{"C.md"}, Phantomed: []string{"B.md"}}
	var buf bytes.Buffer
	printUpdateText(&buf, r)
	got := buf.String()
	if !strings.Contains(got, "updated:\n- A.md\n") {
		t.Errorf("missing updated:\n%s", got)
	}
	if !strings.Contains(got, "deleted:\n- C.md\n") {
		t.Errorf("missing deleted:\n%s", got)
	}
	if !strings.Contains(got, "phantomed:\n- B.md\n") {
		t.Errorf("missing phantomed:\n%s", got)
	}
}

func TestPrintUpdateJSON_EmptySlices(t *testing.T) {
	r := &core.UpdateResult{Updated: []string{"A.md"}}
	var buf bytes.Buffer
	if err := printUpdateJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if string(m["deleted"]) != "[]" {
		t.Errorf("deleted = %s, want []", m["deleted"])
	}
	if string(m["phantomed"]) != "[]" {
		t.Errorf("phantomed = %s, want []", m["phantomed"])
	}
}

func TestPrintAddText(t *testing.T) {
	r := &core.AddResult{
		Added:    []string{"C.md"},
		Promoted: []string{"C.md"},
		Rewritten: []core.RewrittenLink{
			{File: "B.md", OldLink: "[[A]]", NewLink: "[[sub/A]]"},
		},
	}
	var buf bytes.Buffer
	printAddText(&buf, r)
	got := buf.String()
	if !strings.Contains(got, "added:\n- C.md\n") {
		t.Errorf("missing added:\n%s", got)
	}
	if !strings.Contains(got, "promoted:\n- C.md\n") {
		t.Errorf("missing promoted:\n%s", got)
	}
	if !strings.Contains(got, "rewritten:\n- file: B.md\n") {
		t.Errorf("missing rewritten:\n%s", got)
	}
	if !strings.Contains(got, `old: "[[A]]"`) {
		t.Errorf("missing old:\n%s", got)
	}
	if !strings.Contains(got, `new: "[[sub/A]]"`) {
		t.Errorf("missing new:\n%s", got)
	}
}

func TestPrintAddJSON(t *testing.T) {
	r := &core.AddResult{Added: []string{"C.md"}}
	var buf bytes.Buffer
	if err := printAddJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if string(m["promoted"]) != "[]" {
		t.Errorf("promoted = %s, want []", m["promoted"])
	}
	if string(m["rewritten"]) != "[]" {
		t.Errorf("rewritten = %s, want []", m["rewritten"])
	}
}

func TestPrintMoveText(t *testing.T) {
	r := &core.MoveResult{
		Rewritten: []core.RewrittenLink{
			{File: "B.md", OldLink: "[[A]]", NewLink: "[[sub/A]]"},
		},
	}
	var buf bytes.Buffer
	printMoveText(&buf, "A.md", "sub/A.md", r)
	got := buf.String()
	if !strings.Contains(got, "from: A.md\n") {
		t.Errorf("missing from:\n%s", got)
	}
	if !strings.Contains(got, "to: sub/A.md\n") {
		t.Errorf("missing to:\n%s", got)
	}
	if !strings.Contains(got, "rewritten:\n- file: B.md\n") {
		t.Errorf("missing rewritten:\n%s", got)
	}
}

func TestPrintMoveJSON(t *testing.T) {
	r := &core.MoveResult{}
	var buf bytes.Buffer
	if err := printMoveJSON(&buf, "A.md", "sub/A.md", r); err != nil {
		t.Fatal(err)
	}
	var m struct {
		From      string          `json:"from"`
		To        string          `json:"to"`
		Rewritten []rewrittenJSON `json:"rewritten"`
	}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.From != "A.md" {
		t.Errorf("from = %s, want A.md", m.From)
	}
	if m.To != "sub/A.md" {
		t.Errorf("to = %s, want sub/A.md", m.To)
	}
	if m.Rewritten == nil || len(m.Rewritten) != 0 {
		t.Errorf("rewritten = %v, want []", m.Rewritten)
	}
}

func TestPrintDisambiguateText(t *testing.T) {
	r := &core.DisambiguateResult{
		Rewritten: []core.RewrittenLink{
			{File: "B.md", OldLink: "[[A]]", NewLink: "[[sub/A]]"},
		},
	}
	var buf bytes.Buffer
	printDisambiguateText(&buf, r)
	got := buf.String()
	if !strings.Contains(got, "rewritten:\n- file: B.md\n") {
		t.Errorf("missing rewritten:\n%s", got)
	}
}

func TestPrintDisambiguateText_Empty(t *testing.T) {
	r := &core.DisambiguateResult{}
	var buf bytes.Buffer
	printDisambiguateText(&buf, r)
	if buf.String() != "" {
		t.Errorf("expected empty output, got:\n%s", buf.String())
	}
}

func TestPrintDisambiguateJSON(t *testing.T) {
	r := &core.DisambiguateResult{}
	var buf bytes.Buffer
	if err := printDisambiguateJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if string(m["rewritten"]) != "[]" {
		t.Errorf("rewritten = %s, want []", m["rewritten"])
	}
}
