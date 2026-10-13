package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func assertJSONEqual(t *testing.T, actual []byte, expected string) {
	t.Helper()

	var got, want any
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatalf("unmarshal actual JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("unmarshal expected JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON differs:\ngot:  %s\nwant: %s", actual, expected)
	}
}

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

func TestValidateSearchFields(t *testing.T) {
	valid := [][]string{
		{"meta"},
		{"meta.type"},
		{"meta.type", "meta.status"},
		{"lines"},
		{"outgoing_count", "incoming_count"},
		{"meta", "lines", "meta.type"},
	}
	for _, fields := range valid {
		if err := validateSearchFields(fields); err != nil {
			t.Errorf("validateSearchFields(%v) = %v, want nil", fields, err)
		}
	}
	invalid := [][]string{
		{"bogus"},
		{"meta."}, // empty key after prefix
		{"line"},  // typo
	}
	for _, fields := range invalid {
		if err := validateSearchFields(fields); err == nil {
			t.Errorf("validateSearchFields(%v) = nil, want error", fields)
		}
	}
}

func TestPrintSearchText_ComputedAndMetaKey(t *testing.T) {
	r := &core.SearchResult{
		Total: 1,
		Items: []core.SearchResultItem{
			{
				Node:          core.NodeInfo{Type: core.NodeTypeNote, Name: "A", Path: "A.md", Exists: true},
				Lines:         15,
				OutgoingCount: 4,
				IncomingCount: 2,
				Meta: []core.MetaRow{
					{Key: "type", Value: "note"},
					{Key: "status", Value: "active"},
				},
			},
		},
	}
	var buf bytes.Buffer
	// Request lines, outgoing_count, and only meta.type.
	if err := printSearchText(&buf, r, []string{"lines", "outgoing_count", "meta.type"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := `total: 1
items:
- note: A.md
  lines: 15
  outgoing_count: 4
  meta:
  - type: note
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
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

func TestPrintRewrittenText(t *testing.T) {
	links := []core.RewrittenLink{{File: "B.md", OldLink: "[[A]]", NewLink: "[[sub/A]]"}}

	t.Run("populated", func(t *testing.T) {
		var buf bytes.Buffer
		printRewrittenText(&buf, links)
		want := "rewritten:\n- file: B.md\n  old: \"[[A]]\"\n  new: \"[[sub/A]]\"\n"
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		printRewrittenText(&buf, nil)
		if got := buf.String(); got != "" {
			t.Errorf("got %q, want empty output", got)
		}
	})
}

func TestPrintRewrittenJSON(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printRewrittenJSON(&buf, []core.RewrittenLink{{File: "Note.md", OldLink: "[Target](Target.md)", NewLink: "[[Target]]"}}); err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, buf.Bytes(), `{"rewritten":[{"file":"Note.md","old":"[Target](Target.md)","new":"[[Target]]"}]}`)
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printRewrittenJSON(&buf, nil); err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, buf.Bytes(), `{"rewritten":[]}`)
	})
}

func TestPrintRewriteResultText(t *testing.T) {
	rewritten := []core.RewrittenLink{{File: "A.md", OldLink: "[[old/path/X]]", NewLink: "[[X]]"}}
	skipped := []core.SkippedLink{{File: "A.md", RawLink: "[[old/M]]", Basename: "M", Candidates: []string{"dir1/M.md", "dir2/M.md"}}}

	t.Run("skipped present", func(t *testing.T) {
		var buf bytes.Buffer
		printRewriteResultText(&buf, rewritten, skipped)
		want := `rewritten:
- file: A.md
  old: "[[old/path/X]]"
  new: "[[X]]"
skipped:
- file: A.md
  raw_link: "[[old/M]]"
  basename: M
  candidates:
  - dir1/M.md
  - dir2/M.md
`
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("skipped absent", func(t *testing.T) {
		var buf bytes.Buffer
		printRewriteResultText(&buf, rewritten, nil)
		if got, want := buf.String(), "rewritten:\n- file: A.md\n  old: \"[[old/path/X]]\"\n  new: \"[[X]]\"\n"; got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		printRewriteResultText(&buf, nil, nil)
		if got := buf.String(); got != "" {
			t.Errorf("got %q, want empty output", got)
		}
	})
}

func TestPrintRewriteResultJSON(t *testing.T) {
	rewritten := []core.RewrittenLink{{File: "A.md", OldLink: "[[old/path/X]]", NewLink: "[[X]]"}}
	skipped := []core.SkippedLink{{File: "A.md", RawLink: "[[old/M]]", Basename: "M", Candidates: []string{"dir1/M.md", "dir2/M.md"}}}

	t.Run("populated", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printRewriteResultJSON(&buf, rewritten, skipped); err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, buf.Bytes(), `{"rewritten":[{"file":"A.md","old":"[[old/path/X]]","new":"[[X]]"}],"skipped":[{"file":"A.md","raw_link":"[[old/M]]","basename":"M","candidates":["dir1/M.md","dir2/M.md"]}]}`)
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printRewriteResultJSON(&buf, nil, nil); err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, buf.Bytes(), `{"rewritten":[],"skipped":[]}`)
	})
}

// --- Search text format tests ---

func TestPrintSearchText_Full(t *testing.T) {
	r := &core.SearchResult{
		Total: 2,
		Items: []core.SearchResultItem{
			{
				Node: core.NodeInfo{Type: core.NodeTypeNote, Name: "A", Path: "A.md", Exists: true},
				Meta: []core.MetaRow{
					{Key: "priority", Value: "1"},
					{Key: "status", Value: "active"},
				},
				Head: []string{"# A"},
			},
			{
				Node: core.NodeInfo{Type: core.NodeTypeNote, Name: "B", Path: "sub/B.md", Exists: true},
			},
		},
	}
	var buf bytes.Buffer
	if err := printSearchText(&buf, r, []string{"meta"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	want := `total: 2
items:
- note: A.md
  meta:
  - priority: 1
  - status: active
  head:
  - "# A"
- note: sub/B.md
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Decode independently of the formatter, interpreting xhh as a code point.
func decodeSearchMetaText(t *testing.T, value string) string {
	t.Helper()
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			out.WriteByte(value[i])
			continue
		}
		i++
		if i == len(value) {
			t.Fatal("trailing escape")
		}
		switch value[i] {
		case '\\':
			out.WriteByte('\\')
		case 't':
			out.WriteByte('\t')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 'x':
			if i+2 >= len(value) {
				t.Fatal("short hex escape")
			}
			code, err := strconv.ParseUint(value[i+1:i+3], 16, 8)
			if err != nil {
				t.Fatal(err)
			}
			out.WriteRune(rune(code))
			i += 2
		default:
			t.Fatalf("unknown escape: %q", value[i])
		}
	}
	return out.String()
}

func TestPrintSearchText_MetaEscapes(t *testing.T) {
	var controls strings.Builder
	for r := rune(0); r <= 0x9f; r++ {
		if r < 0x20 || r >= 0x7f {
			controls.WriteRune(r)
		}
	}
	cases := []struct {
		name, original, escaped string
	}{
		{"representative", "日本語 😀 \"quoted\" \x1b[2J\n\t\r\x7f\u0080\u009f \\x1b \\\\ \\n", `日本語 😀 "quoted" \x1b[2J\n\t\r\x7f\x80\x9f \\x1b \\\\ \\n`},
		{"all controls", controls.String(), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := []core.MetaRow{{Key: tc.original, Value: tc.original}, {Key: "status", Value: "active"}}
			r := &core.SearchResult{Total: 1, Items: []core.SearchResultItem{{
				Node: core.NodeInfo{Type: core.NodeTypeNote, Path: "A.md"},
				Meta: append([]core.MetaRow(nil), meta...),
			}}}
			for _, fields := range [][]string{{"meta"}, {"meta." + tc.original}} {
				var buf bytes.Buffer
				if err := printSearchText(&buf, r, fields); err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
				wantLines := 5
				if fields[0] == "meta" {
					wantLines++
					if lines[len(lines)-1] != "  - status: active" {
						t.Fatalf("ordinary metadata changed: %q", buf.String())
					}
				}
				if len(lines) != wantLines {
					t.Fatalf("metadata broke output structure: %q", buf.String())
				}
				payload := strings.TrimPrefix(lines[4], "  - ")
				for _, c := range payload {
					if c < 0x20 || (c >= 0x7f && c <= 0x9f) {
						t.Fatalf("raw control U+%04X in payload %q", c, payload)
					}
				}
				key, value, ok := strings.Cut(payload, ": ")
				if !ok {
					t.Fatalf("missing metadata separator: %q", payload)
				}
				for _, escaped := range []string{key, value} {
					if tc.escaped != "" && escaped != tc.escaped {
						t.Errorf("escaped = %q, want %q", escaped, tc.escaped)
					}
					if decoded := decodeSearchMetaText(t, escaped); decoded != tc.original {
						t.Errorf("decoded = %q, want %q", decoded, tc.original)
					}
				}
			}
			if !reflect.DeepEqual(r.Items[0].Meta, meta) {
				t.Fatal("text formatter mutated input metadata")
			}
			var buf bytes.Buffer
			if err := printSearchJSON(&buf, r, []string{"meta"}); err != nil {
				t.Fatal(err)
			}
			var decoded searchJSONOutput
			if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			want := map[string][]string{tc.original: {tc.original}, "status": {"active"}}
			if !reflect.DeepEqual(decoded.Items[0].Meta, want) {
				t.Fatalf("JSON metadata changed: %q", buf.String())
			}
		})
	}
}

func TestPrintSearchText_Empty(t *testing.T) {
	r := &core.SearchResult{Total: 0}
	var buf bytes.Buffer
	if err := printSearchText(&buf, r, nil); err != nil {
		t.Fatal(err)
	}
	want := "total: 0\n"
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestPrintMetaCheckText(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printMetaCheckText(&buf, &core.MetaCheckResult{}); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != "" {
			t.Errorf("got %q, want empty output", got)
		}
	})

	t.Run("issues", func(t *testing.T) {
		r := &core.MetaCheckResult{Issues: []core.MetaIssue{
			{SourcePath: "Notes/A.md", Key: "related", Value: "Missing.md", Line: 4, Reason: core.ReasonNotFound},
			{SourcePath: "Notes/B.md", Key: "parent", Value: "../outside.md", Line: 7, Reason: core.ReasonVaultEscape},
		}}
		var buf bytes.Buffer
		if err := printMetaCheckText(&buf, r); err != nil {
			t.Fatal(err)
		}
		want := `issues:
- source_path: Notes/A.md
  key: related
  value: Missing.md
  location: Notes/A.md:4
  reason: not_found
- source_path: Notes/B.md
  key: parent
  value: ../outside.md
  location: Notes/B.md:7
  reason: vault_escape
`
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}

func TestPrintMetaCheckJSON(t *testing.T) {
	r := &core.MetaCheckResult{Issues: []core.MetaIssue{
		{SourcePath: "Notes/A.md", Key: "related", Value: "Missing.md", Line: 4, Reason: core.ReasonNotFound},
	}}
	var buf bytes.Buffer
	if err := printMetaCheckJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, buf.Bytes(), `{"issues":[{"source_path":"Notes/A.md","key":"related","value":"Missing.md","line":4,"reason":"not_found"}]}`)
}

func TestPrintMetaValidateText(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		if err := printMetaValidateText(&buf, &core.MetaValidateResult{}); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != "" {
			t.Errorf("got %q, want empty output", got)
		}
	})

	t.Run("violations", func(t *testing.T) {
		r := &core.MetaValidateResult{Violations: []core.MetaViolation{
			{SourcePath: "Notes/A.md", Key: "status", Value: "", Line: 1, Reason: core.ReasonMissing},
			{SourcePath: "Notes/B.md", Key: "due", Value: "tomorrow", Line: 5, Reason: core.ReasonType},
		}}
		var buf bytes.Buffer
		if err := printMetaValidateText(&buf, r); err != nil {
			t.Fatal(err)
		}
		want := `violations:
- source_path: Notes/A.md
  key: status
  value:` + " \n" + `  location: Notes/A.md:1
  reason: missing
- source_path: Notes/B.md
  key: due
  value: tomorrow
  location: Notes/B.md:5
  reason: type
`
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}

func TestPrintMetaValidateJSON(t *testing.T) {
	r := &core.MetaValidateResult{Violations: []core.MetaViolation{
		{SourcePath: "Notes/A.md", Key: "status", Value: "", Line: 1, Reason: core.ReasonMissing},
	}}
	var buf bytes.Buffer
	if err := printMetaValidateJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, buf.Bytes(), `{"violations":[{"source_path":"Notes/A.md","key":"status","value":"","line":1,"reason":"missing"}]}`)
}
