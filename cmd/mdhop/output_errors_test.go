package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

var errOutputWrite = errors.New("output write failed")

type failingWriter struct {
	failAt int
	writes int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errOutputWrite
	}
	return len(p), nil
}

func TestTextFormattersReturnWriterError(t *testing.T) {
	t.Run("shared helper after an earlier write", func(t *testing.T) {
		writer := &failingWriter{failAt: 2}
		err := printAddText(writer, &core.AddResult{
			Added:    []string{"A.md"},
			Promoted: []string{"B.md"},
		})
		if !errors.Is(err, errOutputWrite) {
			t.Fatalf("printAddText error = %v, want %v", err, errOutputWrite)
		}
	})

	t.Run("dot after an earlier write", func(t *testing.T) {
		writer := &failingWriter{failAt: 3}
		err := printGraphDot(writer, &core.GraphResult{
			Nodes: []core.GraphNode{{ID: 1, Type: core.NodeTypeNote, Path: "A.md"}},
			Edges: []core.GraphEdge{{Source: 1, Target: 1}},
		})
		if !errors.Is(err, errOutputWrite) {
			t.Fatalf("printGraphDot error = %v, want %v", err, errOutputWrite)
		}
	})
}

func TestRunSetReturnsTextOutputError(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if _, err := core.Build(vault); err != nil {
		t.Fatalf("build vault: %v", err)
	}

	err := withClosedStdout(t, func() error {
		return runSet([]string{"--vault", vault, "--file", "A.md", "--key", "reviewed", "--value", "done"})
	})
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("runSet error = %v, want closed stdout error", err)
	}
}

func TestRunGraphReturnsDotOutputError(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_graph")
	err := withClosedStdout(t, func() error {
		return runGraph([]string{"--vault", vault, "--format", "dot"})
	})
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("runGraph error = %v, want closed stdout error", err)
	}
}

func withClosedStdout(t *testing.T, run func() error) error {
	t.Helper()
	closedStdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	if err := closedStdout.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = closedStdout
	t.Cleanup(func() { os.Stdout = originalStdout })
	return run()
}
