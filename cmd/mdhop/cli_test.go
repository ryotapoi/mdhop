package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
	"github.com/ryotapoi/mdhop/internal/testutil"
)

func TestRunBuild_InvalidFlag(t *testing.T) {
	err := runBuild([]string{"--invalid"})
	if err == nil {
		t.Error("expected error for invalid flag")
	}
}

func TestRunResolve_MissingFrom(t *testing.T) {
	err := runResolve([]string{"--link", "[[X]]"})
	if err == nil || !strings.Contains(err.Error(), "--from is required") {
		t.Errorf("expected --from required error, got: %v", err)
	}
}

func TestRunResolve_MissingLink(t *testing.T) {
	err := runResolve([]string{"--from", "A.md"})
	if err == nil || !strings.Contains(err.Error(), "--link is required") {
		t.Errorf("expected --link required error, got: %v", err)
	}
}

func TestRunResolve_InvalidFormat(t *testing.T) {
	err := runResolve([]string{"--from", "A.md", "--link", "[[X]]", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunResolve_InvalidField(t *testing.T) {
	err := runResolve([]string{"--from", "A.md", "--link", "[[X]]", "--fields", "type,invalid"})
	if err == nil || !strings.Contains(err.Error(), "unknown resolve field") {
		t.Errorf("expected unknown field error, got: %v", err)
	}
}

func TestRunQuery_InvalidFormat(t *testing.T) {
	err := runQuery([]string{"--file", "A.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

// --- Stats CLI tests ---

func TestRunStats_InvalidFormat(t *testing.T) {
	err := runStats([]string{"--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunStats_InvalidField(t *testing.T) {
	err := runStats([]string{"--fields", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "unknown stats field") {
		t.Errorf("expected unknown stats field error, got: %v", err)
	}
}

func setupVaultForCLI(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", name)
	dst := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, dst); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	if err := core.Build(dst); err != nil {
		t.Fatalf("build: %v", err)
	}
	return dst
}

func TestRunStats_TextOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Stats(vault, core.StatsOptions{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	var buf bytes.Buffer
	if err := printStatsText(&buf, result, nil); err != nil {
		t.Fatalf("printStatsText: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "notes_total: 3") {
		t.Errorf("text output missing notes_total: 3, got:\n%s", out)
	}
	if !strings.Contains(out, "notes_exists: 3") {
		t.Errorf("text output missing notes_exists: 3, got:\n%s", out)
	}
	if !strings.Contains(out, "tags_total: 6") {
		t.Errorf("text output missing tags_total: 6, got:\n%s", out)
	}
	if !strings.Contains(out, "phantoms_total: 2") {
		t.Errorf("text output missing phantoms_total: 2, got:\n%s", out)
	}
	if !strings.Contains(out, "edges_total:") {
		t.Errorf("text output missing edges_total, got:\n%s", out)
	}
}

func TestRunStats_JSONOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Stats(vault, core.StatsOptions{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	var buf bytes.Buffer
	if err := printStatsJSON(&buf, result, nil); err != nil {
		t.Fatalf("printStatsJSON: %v", err)
	}

	var m map[string]int
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if m["notes_total"] != 3 {
		t.Errorf("notes_total = %d, want 3", m["notes_total"])
	}
	if m["notes_exists"] != 3 {
		t.Errorf("notes_exists = %d, want 3", m["notes_exists"])
	}
	if m["tags_total"] != 6 {
		t.Errorf("tags_total = %d, want 6", m["tags_total"])
	}
	if m["phantoms_total"] != 2 {
		t.Errorf("phantoms_total = %d, want 2", m["phantoms_total"])
	}
	if _, ok := m["edges_total"]; !ok {
		t.Error("JSON output missing edges_total field")
	}
}
