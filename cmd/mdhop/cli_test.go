package main

import (
	"bytes"
	"encoding/json"
	"os"
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

func TestRunQuery_InvalidField(t *testing.T) {
	// Use an empty temp dir (no index) to verify validation happens before DB open.
	vault := t.TempDir()
	err := runQuery([]string{"--vault", vault, "--file", "A.md", "--fields", "bad"})
	if err == nil || !strings.Contains(err.Error(), "unknown query field") {
		t.Errorf("expected unknown query field error, got: %v", err)
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

func TestPrintStatsText_FieldsFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Stats(vault, core.StatsOptions{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	var buf bytes.Buffer
	if err := printStatsText(&buf, result, []string{"notes_total", "tags_total"}); err != nil {
		t.Fatalf("printStatsText: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "notes_total:") {
		t.Errorf("expected notes_total in output, got:\n%s", out)
	}
	if !strings.Contains(out, "tags_total:") {
		t.Errorf("expected tags_total in output, got:\n%s", out)
	}
	if strings.Contains(out, "notes_exists:") {
		t.Errorf("unexpected notes_exists in output, got:\n%s", out)
	}
	if strings.Contains(out, "edges_total:") {
		t.Errorf("unexpected edges_total in output, got:\n%s", out)
	}
	if strings.Contains(out, "phantoms_total:") {
		t.Errorf("unexpected phantoms_total in output, got:\n%s", out)
	}
}

func TestPrintStatsJSON_FieldsFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Stats(vault, core.StatsOptions{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	var buf bytes.Buffer
	if err := printStatsJSON(&buf, result, []string{"notes_total"}); err != nil {
		t.Fatalf("printStatsJSON: %v", err)
	}

	var m map[string]int
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if _, ok := m["notes_total"]; !ok {
		t.Error("expected notes_total in JSON output")
	}
	if _, ok := m["notes_exists"]; ok {
		t.Error("unexpected notes_exists in JSON output")
	}
	if _, ok := m["edges_total"]; ok {
		t.Error("unexpected edges_total in JSON output")
	}
	if _, ok := m["tags_total"]; ok {
		t.Error("unexpected tags_total in JSON output")
	}
	if _, ok := m["phantoms_total"]; ok {
		t.Error("unexpected phantoms_total in JSON output")
	}
}

// --- Delete CLI tests ---

func TestRunDelete_InvalidFormat(t *testing.T) {
	err := runDelete([]string{"--file", "A.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunDelete_MissingFile(t *testing.T) {
	err := runDelete([]string{})
	if err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Errorf("expected --file required error, got: %v", err)
	}
}

func TestRunDelete_Integration(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete")

	// Remove file from disk first (delete reflects file removal).
	if err := os.Remove(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("remove C.md: %v", err)
	}

	// Delete C.md (unreferenced)
	err := runDelete([]string{"--vault", vault, "--file", "C.md"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify C.md is gone from the index.
	result, err := core.Stats(vault, core.StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if result.NotesTotal != 2 {
		t.Errorf("notes_total = %d, want 2 after deleting C.md", result.NotesTotal)
	}
}

func TestRunDelete_Rm_FileExists(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete")

	// C.md still exists on disk; --rm should remove it and update index.
	if _, err := os.Stat(filepath.Join(vault, "C.md")); err != nil {
		t.Fatalf("C.md should exist before --rm: %v", err)
	}

	err := runDelete([]string{"--vault", vault, "--file", "C.md", "--rm"})
	if err != nil {
		t.Fatalf("delete --rm: %v", err)
	}

	// Verify file is gone from disk.
	if _, err := os.Stat(filepath.Join(vault, "C.md")); !os.IsNotExist(err) {
		t.Error("C.md should not exist on disk after --rm")
	}

	// Verify gone from index.
	result, err := core.Stats(vault, core.StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if result.NotesTotal != 2 {
		t.Errorf("notes_total = %d, want 2", result.NotesTotal)
	}
}

func TestRunDelete_Rm_FileAlreadyGone(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete")

	// Remove file first, then --rm should still succeed (idempotent).
	os.Remove(filepath.Join(vault, "C.md"))

	err := runDelete([]string{"--vault", vault, "--file", "C.md", "--rm"})
	if err != nil {
		t.Fatalf("delete --rm with already-removed file: %v", err)
	}

	result, err := core.Stats(vault, core.StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if result.NotesTotal != 2 {
		t.Errorf("notes_total = %d, want 2", result.NotesTotal)
	}
}

func TestRunDelete_Rm_UnregisteredFile(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete")

	// Create an unregistered file.
	unregistered := filepath.Join(vault, "unregistered.md")
	if err := os.WriteFile(unregistered, []byte("test"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// --rm should fail for unregistered file and not delete it.
	err := runDelete([]string{"--vault", vault, "--file", "unregistered.md", "--rm"})
	if err == nil || !strings.Contains(err.Error(), "file not registered") {
		t.Errorf("expected 'file not registered' error, got: %v", err)
	}

	// File should still exist on disk.
	if _, err := os.Stat(unregistered); err != nil {
		t.Error("unregistered file should still exist after failed --rm")
	}
}

// --- Update CLI tests ---

func TestRunUpdate_InvalidFormat(t *testing.T) {
	err := runUpdate([]string{"--file", "A.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunUpdate_MissingFile(t *testing.T) {
	err := runUpdate([]string{})
	if err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Errorf("expected --file required error, got: %v", err)
	}
}

func TestRunUpdate_Integration(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_update")

	// Get baseline edge count.
	before, err := core.Stats(vault, core.StatsOptions{Fields: []string{"edges_total"}})
	if err != nil {
		t.Fatalf("stats before: %v", err)
	}

	// Modify A.md: add a link to C.
	aPath := filepath.Join(vault, "A.md")
	content, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if err := os.WriteFile(aPath, append(content, []byte("\n[[C]]\n")...), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}

	// Run update.
	if err := runUpdate([]string{"--vault", vault, "--file", "A.md"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Verify edges increased.
	after, err := core.Stats(vault, core.StatsOptions{Fields: []string{"edges_total"}})
	if err != nil {
		t.Fatalf("stats after: %v", err)
	}
	if after.EdgesTotal <= before.EdgesTotal {
		t.Errorf("edges_total did not increase: before=%d, after=%d", before.EdgesTotal, after.EdgesTotal)
	}
}

// --- Add CLI tests ---

func TestRunAdd_InvalidFormat(t *testing.T) {
	err := runAdd([]string{"--file", "A.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunAdd_MissingFile(t *testing.T) {
	err := runAdd([]string{})
	if err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Errorf("expected --file required error, got: %v", err)
	}
}

func TestRunAdd_Integration(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_add")

	// Get baseline notes count.
	before, err := core.Stats(vault, core.StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats before: %v", err)
	}

	// Create a new file on disk.
	newFile := filepath.Join(vault, "C.md")
	if err := os.WriteFile(newFile, []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	// Run add.
	if err := runAdd([]string{"--vault", vault, "--file", "C.md"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Verify notes_total increased.
	after, err := core.Stats(vault, core.StatsOptions{Fields: []string{"notes_total"}})
	if err != nil {
		t.Fatalf("stats after: %v", err)
	}
	if after.NotesTotal != before.NotesTotal+1 {
		t.Errorf("notes_total = %d, want %d", after.NotesTotal, before.NotesTotal+1)
	}
}

// --- Move CLI tests ---

func TestRunMove_InvalidFormat(t *testing.T) {
	err := runMove([]string{"--from", "A.md", "--to", "B.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunMove_MissingFrom(t *testing.T) {
	err := runMove([]string{"--to", "X.md"})
	if err == nil || !strings.Contains(err.Error(), "--from is required") {
		t.Errorf("expected --from required error, got: %v", err)
	}
}

func TestRunMove_MissingTo(t *testing.T) {
	err := runMove([]string{"--from", "A.md"})
	if err == nil || !strings.Contains(err.Error(), "--to is required") {
		t.Errorf("expected --to required error, got: %v", err)
	}
}

func TestRunMove_Integration(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_move_basic")

	err := runMove([]string{"--vault", vault, "--from", "A.md", "--to", "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Verify A.md moved on disk.
	if _, err := os.Stat(filepath.Join(vault, "A.md")); err == nil {
		t.Error("A.md should not exist on disk after move")
	}
	if _, err := os.Stat(filepath.Join(vault, "sub", "A.md")); err != nil {
		t.Error("sub/A.md should exist on disk after move")
	}

	// Verify DB updated: old path should be gone, new path should exist.
	_, err = core.Query(vault, core.EntrySpec{File: "A.md"}, core.QueryOptions{})
	if err == nil {
		t.Error("querying A.md should fail after move")
	}
	qr, err := core.Query(vault, core.EntrySpec{File: "sub/A.md"}, core.QueryOptions{})
	if err != nil {
		t.Fatalf("querying sub/A.md: %v", err)
	}
	if qr.Entry.Type != "note" {
		t.Errorf("sub/A.md type = %q, want note", qr.Entry.Type)
	}
}

// --- Diagnose CLI tests ---

func TestRunDiagnose_InvalidFormat(t *testing.T) {
	err := runDiagnose([]string{"--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunDiagnose_InvalidField(t *testing.T) {
	err := runDiagnose([]string{"--fields", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "unknown diagnose field") {
		t.Errorf("expected unknown diagnose field error, got: %v", err)
	}
}

func TestRunDiagnose_TextOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_ambiguous_name")

	result, err := core.Diagnose(vault, core.DiagnoseOptions{})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	var buf bytes.Buffer
	if err := printDiagnoseText(&buf, result, nil); err != nil {
		t.Fatalf("printDiagnoseText: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "basename_conflicts:") {
		t.Errorf("text output missing basename_conflicts:, got:\n%s", out)
	}
	if !strings.Contains(out, "- name: A") {
		t.Errorf("text output missing conflict name A, got:\n%s", out)
	}
	if !strings.Contains(out, "sub1/A.md") {
		t.Errorf("text output missing sub1/A.md, got:\n%s", out)
	}
	if !strings.Contains(out, "sub2/A.md") {
		t.Errorf("text output missing sub2/A.md, got:\n%s", out)
	}
	if !strings.Contains(out, "phantoms:") {
		t.Errorf("text output missing phantoms:, got:\n%s", out)
	}
}

func TestRunDiagnose_JSONOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_ambiguous_name")

	result, err := core.Diagnose(vault, core.DiagnoseOptions{})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	var buf bytes.Buffer
	if err := printDiagnoseJSON(&buf, result, nil); err != nil {
		t.Fatalf("printDiagnoseJSON: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	// Check basename_conflicts exists
	if _, ok := m["basename_conflicts"]; !ok {
		t.Fatal("JSON output missing basename_conflicts field")
	}

	var conflicts []struct {
		Name  string   `json:"name"`
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal(m["basename_conflicts"], &conflicts); err != nil {
		t.Fatalf("unmarshal basename_conflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("basename_conflicts count = %d, want 1", len(conflicts))
	}
	if conflicts[0].Name != "A" {
		t.Errorf("conflict name = %q, want A", conflicts[0].Name)
	}
	if len(conflicts[0].Paths) != 2 {
		t.Fatalf("conflict paths count = %d, want 2", len(conflicts[0].Paths))
	}
}

func TestPrintDiagnoseText_FieldsFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Diagnose(vault, core.DiagnoseOptions{})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	var buf bytes.Buffer
	if err := printDiagnoseText(&buf, result, []string{"phantoms"}); err != nil {
		t.Fatalf("printDiagnoseText: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "phantoms:") {
		t.Errorf("expected phantoms in output, got:\n%s", out)
	}
	if strings.Contains(out, "basename_conflicts:") {
		t.Errorf("unexpected basename_conflicts in output, got:\n%s", out)
	}
}

func TestPrintDiagnoseJSON_FieldsFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_full")

	result, err := core.Diagnose(vault, core.DiagnoseOptions{})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	var buf bytes.Buffer
	if err := printDiagnoseJSON(&buf, result, []string{"basename_conflicts"}); err != nil {
		t.Fatalf("printDiagnoseJSON: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if _, ok := m["basename_conflicts"]; !ok {
		t.Error("expected basename_conflicts in JSON output")
	}
	if _, ok := m["phantoms"]; ok {
		t.Error("unexpected phantoms in JSON output")
	}
}

// --- Disambiguate CLI tests ---

func TestRunDisambiguate_InvalidFormat(t *testing.T) {
	err := runDisambiguate([]string{"--name", "A", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunDisambiguate_MissingName(t *testing.T) {
	err := runDisambiguate([]string{})
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Errorf("expected --name required error, got: %v", err)
	}
}

func TestRunDisambiguate_Integration(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_disambiguate")

	err := runDisambiguate([]string{"--vault", vault, "--name", "A"})
	if err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/A]]") {
		t.Errorf("B.md should contain [[sub/A]], got:\n%s", content)
	}
}

func TestRunDisambiguate_ScanIntegration(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "vault_disambiguate")
	vault := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, vault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}

	err := runDisambiguate([]string{"--vault", vault, "--name", "A", "--scan"})
	if err != nil {
		t.Fatalf("disambiguate scan: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/A]]") {
		t.Errorf("B.md should contain [[sub/A]], got:\n%s", content)
	}
}
