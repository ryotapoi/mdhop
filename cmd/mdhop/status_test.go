package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestPrintStatusJSONAlwaysIncludesEmptyArrays(t *testing.T) {
	var output bytes.Buffer
	if err := printStatusJSON(&output, &core.StatusResult{}); err != nil {
		t.Fatalf("print json: %v", err)
	}
	var result map[string][]string
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	want := map[string][]string{"untracked": {}, "modified": {}, "deleted": {}}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("json = %#v, want %#v", result, want)
	}
}

func TestPrintStatusText(t *testing.T) {
	var output bytes.Buffer
	err := printStatusText(&output, &core.StatusResult{
		Untracked: []string{"New.md"},
		Deleted:   []string{"Gone.png"},
	})
	if err != nil {
		t.Fatalf("print text: %v", err)
	}
	if got, want := output.String(), "untracked:\n- New.md\ndeleted:\n- Gone.png\n"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestRunStatusInvalidFormat(t *testing.T) {
	err := runStatus([]string{"--format", "yaml"})
	if err == nil || err.Error() != `invalid format: "yaml" (must be json or text)` {
		t.Fatalf("error = %v", err)
	}
}

func TestRunStatusHelp(t *testing.T) {
	if err := runStatus([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v, want flag.ErrHelp", err)
	}
}

func TestRunStatusReturnsOutputError(t *testing.T) {
	vault := t.TempDir()
	if _, err := core.Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "New.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write untracked note: %v", err)
	}
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			err := withClosedStdout(t, func() error {
				return runStatus([]string{"--vault", vault, "--format", format})
			})
			if !errors.Is(err, os.ErrClosed) {
				t.Fatalf("run status error = %v, want closed stdout error", err)
			}
		})
	}
}
