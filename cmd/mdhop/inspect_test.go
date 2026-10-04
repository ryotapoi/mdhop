package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestInspectFormatting(t *testing.T) {
	r := &core.InspectResult{Entry: core.NodeInfo{Type: core.NodeTypeNote, Name: "Plan", Path: "Plan.md", Exists: true}, Tags: []string{}, Meta: map[string][]string{}, Head: []string{}}
	var b bytes.Buffer
	if err := printInspectJSON(&b, r); err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"tags": "[]", "meta": "{}", "head": "[]", "entry": `{"type":"note","name":"Plan","path":"Plan.md","exists":true}`} {
		var compact bytes.Buffer
		if err := json.Compact(&compact, out[key]); err != nil {
			t.Fatal(err)
		}
		if compact.String() != want {
			t.Fatalf("%s: %s", key, out[key])
		}
	}
	b.Reset()
	if err := printInspectText(&b, r); err != nil {
		t.Fatal(err)
	}
	if b.String() != "entry:\n  note \"Plan.md\"\ntags: []\nmeta: {}\nhead: []\n" {
		t.Fatalf("text: %q", b.String())
	}
	r.Tags = []string{"#planning"}
	r.Head = nil
	r.Meta = map[string][]string{"z": {"x\ny", "\"q\""}, "a": {"first"}}
	b.Reset()
	if err := printInspectText(&b, r); err != nil {
		t.Fatal(err)
	}
	if b.String() != "entry:\n  note \"Plan.md\"\ntags:\n  tag \"#planning\"\nmeta:\n  \"a\": \"first\"\n  \"z\": \"x\\ny\"\n  \"z\": \"\\\"q\\\"\"\n" {
		t.Fatalf("quoted text: %q", b.String())
	}
	r.Tags = nil
	b.Reset()
	if err := printInspectJSON(&b, r); err != nil {
		t.Fatal(err)
	}
	out = nil
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["tags"]; ok {
		t.Fatal("unselected tags present")
	}
	if _, ok := out["head"]; ok {
		t.Fatal("unselected head present")
	}
}

func TestRunInspectSelectionAndValidation(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() error {
		return runInspect([]string{"--vault", vault, "--file", "A.md", "--fields", " meta ", "--include-head", "1", "--format", "json"})
	})
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(output)); err != nil {
		t.Fatal(err)
	}
	output = compact.String()
	if strings.Contains(output, `"tags"`) || !strings.Contains(output, `"meta":{}`) || !strings.Contains(output, `"head":["# A"]`) {
		t.Fatalf("selection: %s", output)
	}
	for _, args := range [][]string{{"--fields", ""}, {"--fields", "tags,"}, {"--fields", "head"}, {"--fields", "tags,tags"}, {"--include-head", "0"}, {"--include-head", "-1"}, {"--include-head", "one"}, {"--format", "yaml"}, {"--tag", "x"}, {"extra"}} {
		if err := runInspect(append([]string{"--vault", vault, "--file", "A.md"}, args...)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := runInspect([]string{"--vault", vault}); err == nil {
		t.Fatal("accepted missing file")
	}
}

func TestInspectFormattersReturnWriterError(t *testing.T) {
	for _, print := range []func(io.Writer, *core.InspectResult) error{printInspectJSON, printInspectText} {
		if err := print(&failingWriter{failAt: 1}, &core.InspectResult{}); !errors.Is(err, errOutputWrite) {
			t.Fatalf("writer error: %v", err)
		}
	}
}
