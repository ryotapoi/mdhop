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

func TestRunResolveLine(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	locations := core.Locations{DBPath: filepath.Join(t.TempDir(), "external.sqlite")}
	if _, err := core.Build(vault, locations); err != nil {
		t.Fatal(err)
	}
	base := []string{"--vault", vault, "--db", locations.DBPath, "--from", "A.md", "--link", "[[B]]"}
	for _, line := range []string{"0", "-1", "word", "1.5"} {
		err := runResolve(append(append([]string{}, base...), "--line", line))
		if err == nil || !strings.Contains(err.Error(), "line") {
			t.Fatalf("line %q: %v", line, err)
		}
	}
	for _, format := range []string{"text", "json"} {
		args := append(append([]string{}, base...), "--format", format)
		omitted := captureStdout(t, func() error { return runResolve(args) })
		specified := captureStdout(t, func() error { return runResolve(append(args, "--line", "1")) })
		if specified != omitted {
			t.Fatalf("%s output changed: %s / %s", format, specified, omitted)
		}
		if format == "json" {
			var got map[string]any
			if err := json.Unmarshal([]byte(specified), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"type": "note", "name": "B", "path": "B.md", "exists": true}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSON = %#v", got)
			}
		} else if !strings.Contains(specified, "B.md") {
			t.Fatalf("text = %q", specified)
		}
	}
	output := captureStdout(t, func() error {
		return runResolve(append(base, "--line", "1", "--fields", "type,path", "--format", "json"))
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"type": "note", "path": "B.md"}) {
		t.Fatalf("fields JSON = %#v", got)
	}
}
