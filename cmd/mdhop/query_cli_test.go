package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestRunQuery_RelationAndPageValidation(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"explicit empty", []string{"--relations", ""}},
		{"empty member", []string{"--relations", "backlinks,"}},
		{"duplicate", []string{"--relations", "backlinks,backlinks"}},
		{"unknown", []string{"--relations", "2hoplink"}},
		{"implicit page", []string{"--limit", "1"}},
		{"multiple relation page", []string{"--relations", "backlinks,outgoing", "--offset", "0"}},
		{"zero limit", []string{"--relations", "backlinks", "--limit", "0"}},
		{"noninteger limit", []string{"--relations", "backlinks", "--limit", "one"}},
		{"negative offset", []string{"--relations", "backlinks", "--offset", "-1"}},
		{"noninteger offset", []string{"--relations", "backlinks", "--offset", "one"}},
		{"invalid via", []string{"--via", "other:x"}},
		{"empty via", []string{"--via", "tag:"}},
		{"repeated via", []string{"--via", "note:X.md", "--via", "tag:x"}},
		{"legacy fields", []string{"--fields", "backlinks"}},
		{"legacy max backlinks", []string{"--max-backlinks", "1"}},
		{"legacy max twohop", []string{"--max-twohop", "1"}},
		{"legacy max via", []string{"--max-via-per-target", "1"}},
		{"legacy exclude", []string{"--exclude", "x"}},
		{"legacy exclude tag", []string{"--exclude-tag", "x"}},
		{"legacy no exclude", []string{"--no-exclude"}},
		{"deferred head", []string{"--include-head", "1"}},
		{"deferred snippet", []string{"--include-snippet", "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--vault", vault, "--file", "A.md"}, tc.args...)
			if err := runQuery(args); err == nil {
				t.Fatalf("runQuery(%v) unexpectedly succeeded", tc.args)
			}
		})
	}
}

func TestRunQuery_PageAndHiddenVia(t *testing.T) {
	vault := t.TempDir()
	for path, contents := range map[string]string{
		"A.md": "[[V]]\n",
		"B.md": "[[V]]\n",
		"C.md": "[[V]]\n",
		"V.md": "# Via\n",
	} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatal(err)
	}
	query := func(extra ...string) map[string]json.RawMessage {
		t.Helper()
		args := append([]string{"--vault", vault, "--file", "A.md", "--format", "json"}, extra...)
		output := captureStdout(t, func() error { return runQuery(args) })
		var result map[string]json.RawMessage
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("query JSON: %v: %s", err, output)
		}
		return result
	}

	first := query("--relations", "twohop", "--limit", "1", "--hide-path", "V.md")
	if _, ok := first["backlinks"]; ok {
		t.Fatal("unselected backlinks should be absent")
	}
	var page struct {
		Offset int  `json:"offset"`
		Limit  *int `json:"limit"`
		Next   *int `json:"next_offset"`
	}
	if err := json.Unmarshal(first["page"], &page); err != nil {
		t.Fatal(err)
	}
	if page.Offset != 0 || page.Limit == nil || *page.Limit != 1 || page.Next == nil || *page.Next != 1 {
		t.Fatalf("first page = %+v", page)
	}
	var targets []struct {
		Path     string            `json:"path"`
		Relation []json.RawMessage `json:"relation"`
		Hidden   bool              `json:"hidden_relation"`
	}
	if err := json.Unmarshal(first["2hoplink"], &targets); err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Path != "B.md" || len(targets[0].Relation) != 0 || !targets[0].Hidden {
		t.Fatalf("hidden via result = %+v", targets)
	}

	last := query("--relations", "twohop", "--offset", "1")
	if err := json.Unmarshal(last["page"], &page); err != nil {
		t.Fatal(err)
	}
	if page.Offset != 1 || page.Limit != nil || page.Next != nil {
		t.Fatalf("last page = %+v", page)
	}
	if err := json.Unmarshal(last["2hoplink"], &targets); err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Path != "C.md" || len(targets[0].Relation) != 1 || targets[0].Hidden {
		t.Fatalf("last page targets = %+v", targets)
	}

	past := query("--relations", "twohop", "--offset", "2")
	if string(past["2hoplink"]) != "[]" {
		t.Fatalf("past-end selected relation = %s", past["2hoplink"])
	}
}

func TestRunQuery_ViaMissKeepsDirectRelations(t *testing.T) {
	vault := t.TempDir()
	for path, contents := range map[string]string{"A.md": "[[V]]\n", "B.md": "[[V]]\n", "V.md": "# V\n"} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() error {
		return runQuery([]string{"--vault", vault, "--file", "A.md", "--via", "note:missing.md", "--format", "json"})
	})
	var result struct {
		Outgoing []struct{ Path string } `json:"outgoing"`
		TwoHop   []json.RawMessage       `json:"2hoplink"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Outgoing, []struct{ Path string }{{Path: "V.md"}}) || len(result.TwoHop) != 0 {
		t.Fatalf("via miss result = %s", strings.TrimSpace(output))
	}
}

func TestRunQuery_ConfigControlsStayIndependent(t *testing.T) {
	vault := t.TempDir()
	for path, contents := range map[string]string{
		"A.md":       "[[V]]\n",
		"B.md":       "[[V]]\n",
		"C.md":       "[[V]]\n",
		"V.md":       "# V\n",
		"mdhop.yaml": "query:\n  hide:\n    paths: [B.md]\n  via:\n    exclude:\n      paths: [V.md]\n",
	} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatal(err)
	}
	paths := func(extra ...string) []string {
		t.Helper()
		args := append([]string{"--vault", vault, "--file", "A.md", "--relations", "twohop", "--format", "json"}, extra...)
		output := captureStdout(t, func() error { return runQuery(args) })
		var result struct {
			TwoHop []struct{ Path string } `json:"2hoplink"`
		}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, target := range result.TwoHop {
			got = append(got, target.Path)
		}
		return got
	}
	if got := paths(); len(got) != 0 {
		t.Fatalf("configured via exclusion should remove relation, got %v", got)
	}
	if got := paths("--no-config-via"); !reflect.DeepEqual(got, []string{"C.md"}) {
		t.Fatalf("configured hide should still apply, got %v", got)
	}
	if got := paths("--no-config-via", "--no-config-hide"); !reflect.DeepEqual(got, []string{"B.md", "C.md"}) {
		t.Fatalf("both config purposes disabled, got %v", got)
	}
	if got := paths("--no-config-via", "--no-config-hide", "--exclude-via-path", "V.md"); len(got) != 0 {
		t.Fatalf("CLI via exclusion should still apply, got %v", got)
	}
}
