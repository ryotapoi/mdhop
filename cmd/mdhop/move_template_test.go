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

func TestMoveTemplateOutputAfterMetadataUpdate(t *testing.T) {
	for _, directory := range []bool{false, true} {
		name := "single"
		if directory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			vault := t.TempDir()
			if err := os.Mkdir(filepath.Join(vault, "src"), 0o755); err != nil {
				t.Fatal(err)
			}
			files := []string{"src/A.md"}
			if directory {
				files = append(files, "src/B.md")
			}
			for _, file := range files {
				if err := os.WriteFile(filepath.Join(vault, file), []byte("---\nclient: Old\n---\n# Note\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(vault, "Index.md"), []byte("[[src/A.md]]\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := core.Build(vault); err != nil {
				t.Fatal(err)
			}
			from := "src/A.md"
			if directory {
				from = "src/"
			}
			opts := core.MoveTemplateOptions{From: from, Template: "archive/{client}/{basename}", Directory: directory}
			plan, err := core.PlanMoveTemplate(vault, opts)
			if err != nil {
				t.Fatal(err)
			}
			want := make([]core.MovedFile, len(files))
			for i, file := range files {
				if plan.Moved[i].To != "archive/Old/"+filepath.Base(file) {
					t.Fatalf("unexpected plan: %+v", plan)
				}
				if _, err := core.Set(vault, core.SetOptions{File: file, Key: "client", Value: "Actual"}); err != nil {
					t.Fatal(err)
				}
				want[i] = core.MovedFile{From: file, To: "archive/Actual/" + filepath.Base(file)}
			}
			result, err := core.MoveTemplate(vault, opts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Moved, want) {
				t.Fatalf("moved = %+v, want %+v", result.Moved, want)
			}
			if len(result.Rewritten) == 0 {
				t.Fatal("expected Index.md link rewrite")
			}
			for _, moved := range want {
				if _, err := os.Stat(filepath.Join(vault, moved.To)); err != nil {
					t.Fatal(err)
				}
				if _, err := core.Query(vault, core.EntrySpec{File: moved.To}, core.QueryOptions{}); err != nil {
					t.Fatal(err)
				}
				for _, absent := range []string{moved.From, "archive/Old/" + filepath.Base(moved.From)} {
					if _, err := os.Stat(filepath.Join(vault, absent)); !os.IsNotExist(err) {
						t.Fatalf("%s exists, err=%v", absent, err)
					}
					if _, err := core.Query(vault, core.EntrySpec{File: absent}, core.QueryOptions{}); err == nil {
						t.Fatalf("%s still in DB", absent)
					}
				}
			}
			for _, format := range []string{"text", "json"} {
				t.Run(format, func(t *testing.T) {
					out := captureStdout(t, func() error { return printMoveTemplatePlan(format, directory, plan, result) })
					if format == "text" {
						for _, moved := range want {
							if !strings.Contains(out, "from: "+moved.From+"\n") || !strings.Contains(out, "to: "+moved.To+"\n") {
								t.Fatalf("wrong output: %s", out)
							}
						}
						if strings.Contains(out, "archive/Old/") || !strings.Contains(out, "Index.md") {
							t.Fatalf("wrong output: %s", out)
						}
						return
					}
					var got struct {
						From      string           `json:"from"`
						To        string           `json:"to"`
						Moved     []core.MovedFile `json:"moved"`
						Rewritten []rewrittenJSON  `json:"rewritten"`
					}
					if err := json.Unmarshal([]byte(out), &got); err != nil {
						t.Fatal(err)
					}
					if directory {
						if !reflect.DeepEqual(got.Moved, want) {
							t.Fatalf("moved = %+v, want %+v", got.Moved, want)
						}
					} else if got.From != want[0].From || got.To != want[0].To {
						t.Fatalf("wrong output: %s", out)
					}
					if !reflect.DeepEqual(got.Rewritten, toRewrittenJSON(result.Rewritten)) {
						t.Fatalf("rewritten = %+v, want %+v", got.Rewritten, result.Rewritten)
					}
				})
			}
		})
	}
}

func TestMoveTemplateOutputUsesExecutionCount(t *testing.T) {
	plan := &core.MoveTemplatePlanResult{Moved: []core.MovedFile{{From: "A.md", To: "old/A.md"}, {From: "B.md", To: "old/B.md"}}}
	result := &core.MoveDirResult{Moved: []core.MovedFile{{From: "A.md", To: "actual/A.md"}}}
	out := captureStdout(t, func() error { return printMoveTemplatePlan("json", false, plan, result) })
	var got moveJSONOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.To != "actual/A.md" || got.Rewritten == nil || len(got.Rewritten) != 0 {
		t.Fatalf("unexpected output: %s", out)
	}
	plan.Moved = plan.Moved[:1]
	for _, moved := range [][]core.MovedFile{nil, {{From: "A.md"}, {From: "B.md"}}} {
		result.Moved = moved
		if err := printMoveTemplatePlan("json", false, plan, result); err == nil || !strings.Contains(err.Error(), "expected one --to-template move") {
			t.Fatalf("expected execution count error, got %v", err)
		}
	}
}
