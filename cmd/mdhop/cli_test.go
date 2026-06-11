package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if _, err := core.Build(dst); err != nil {
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
	if qr.Entry.Type != core.NodeTypeNote {
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
	if len(result.Phantoms) == 0 {
		if strings.Contains(out, "phantoms:") {
			t.Errorf("expected no phantoms section for empty list, got:\n%s", out)
		}
	} else {
		if !strings.Contains(out, "phantoms:") {
			t.Errorf("text output missing phantoms:, got:\n%s", out)
		}
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

func TestRunQuery_PathFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_exclude")

	out := captureStdout(t, func() error {
		return runQuery([]string{"--vault", vault, "--file", "A.md", "--path", "daily/*", "--fields", "outgoing", "--format", "json"})
	})

	var m struct {
		Outgoing []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"outgoing"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	var names []string
	for _, og := range m.Outgoing {
		names = append(names, og.Name)
		if og.Path == "B.md" || og.Path == "C.md" {
			t.Errorf("outgoing should not contain %s with --path daily/*", og.Path)
		}
	}
	if len(names) != 2 || names[0] != "Missing" || names[1] != "D" {
		// Order: phantom (empty path) sorts first by path.
		t.Errorf("outgoing names = %v, want [Missing D]", names)
	}
}

func TestRunDiagnose_BrokenAnchors(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_anchor_check")

	out := captureStdout(t, func() error {
		return runDiagnose([]string{"--vault", vault, "--fields", "anchors", "--format", "json"})
	})

	var m struct {
		BrokenAnchors []struct {
			SourcePath string `json:"source_path"`
			RawLink    string `json:"raw_link"`
			TargetPath string `json:"target_path"`
			Fragment   string `json:"fragment"`
		} `json:"broken_anchors"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.BrokenAnchors) != 2 {
		t.Fatalf("broken_anchors = %+v, want 2", m.BrokenAnchors)
	}
	frags := map[string]bool{}
	for _, a := range m.BrokenAnchors {
		frags[a.Fragment] = true
	}
	if !frags["Missing"] || !frags["Nonexistent"] {
		t.Errorf("expected Missing and Nonexistent broken, got %v", frags)
	}
}

func TestRunDiagnose_AnchorsOptIn(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_anchor_check")

	// Without --fields anchors, broken_anchors must be absent from JSON.
	out := captureStdout(t, func() error {
		return runDiagnose([]string{"--vault", vault, "--format", "json"})
	})
	if strings.Contains(out, "broken_anchors") {
		t.Errorf("broken_anchors should not appear without --fields anchors: %s", out)
	}
}

func TestRunDiagnose_PathFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_diagnose_path")

	out := captureStdout(t, func() error {
		return runDiagnose([]string{"--vault", vault, "--path", "docs/*", "--format", "json"})
	})

	var m struct {
		BasenameConflicts []struct {
			Name  string   `json:"name"`
			Paths []string `json:"paths"`
		} `json:"basename_conflicts"`
		Phantoms []string `json:"phantoms"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.BasenameConflicts) != 1 || m.BasenameConflicts[0].Name != "Conflict" {
		t.Errorf("basename_conflicts = %+v, want single Conflict group", m.BasenameConflicts)
	}
	if len(m.Phantoms) != 1 || m.Phantoms[0] != "MissingDoc" {
		t.Errorf("phantoms = %v, want [MissingDoc]", m.Phantoms)
	}
}

func TestRunDiagnose_ExcludeFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_diagnose_path")

	out := captureStdout(t, func() error {
		return runDiagnose([]string{"--vault", vault, "--exclude", "docs/*", "--format", "json"})
	})

	var m struct {
		Phantoms []string `json:"phantoms"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.Phantoms) != 1 || m.Phantoms[0] != "MissingOther" {
		t.Errorf("phantoms = %v, want [MissingOther]", m.Phantoms)
	}
}

func TestRunDiagnose_InvalidGlob(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_diagnose_path")

	err := runDiagnose([]string{"--vault", vault, "--path", "[abc]/*"})
	if err == nil || !strings.Contains(err.Error(), "unsupported glob pattern") {
		t.Errorf("expected unsupported glob pattern error, got: %v", err)
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

// --- Version tests ---

func TestVersionOutput(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf)
	out := buf.String()
	if out != "mdhop version dev\n" {
		t.Errorf("version output = %q, want %q", out, "mdhop version dev\n")
	}
}

// --- Add auto-disambiguate default tests ---

func TestRunAdd_AutoDisambiguateDefault(t *testing.T) {
	// Default behavior (no flag) should auto-disambiguate: rewrite links on collision.
	vault := setupVaultForCLI(t, "vault_add_disambiguate")

	// Create B.md at root to cause basename collision with sub/B.md.
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	// Run add without --no-auto-disambiguate (default = auto-disambiguate ON).
	err := runAdd([]string{"--vault", vault, "--file", "B.md"})
	if err != nil {
		t.Fatalf("add should succeed with default auto-disambiguate, got: %v", err)
	}

	// Verify A.md was rewritten (links to sub/B).
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/B]]") {
		t.Errorf("A.md should contain [[sub/B]] after auto-disambiguate, got:\n%s", content)
	}
}

func TestRunAdd_NoAutoDisambiguate(t *testing.T) {
	// --no-auto-disambiguate should cause error on collision.
	vault := setupVaultForCLI(t, "vault_add_disambiguate")

	// Create B.md at root to cause basename collision with sub/B.md.
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	err := runAdd([]string{"--vault", vault, "--file", "B.md", "--no-auto-disambiguate"})
	if err == nil {
		t.Fatal("expected error with --no-auto-disambiguate on collision")
	}
	if !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("unexpected error: %v", err)
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

// --- Directory delete CLI tests ---

func TestRunDelete_DirExpansion(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete_dir")

	err := runDelete([]string{"--vault", vault, "--file", "sub/", "--rm"})
	if err != nil {
		t.Fatalf("delete dir: %v", err)
	}

	// sub/ files should be gone from disk.
	if _, err := os.Stat(filepath.Join(vault, "sub", "A.md")); !os.IsNotExist(err) {
		t.Error("sub/A.md should be gone")
	}
	if _, err := os.Stat(filepath.Join(vault, "sub", "B.md")); !os.IsNotExist(err) {
		t.Error("sub/B.md should be gone")
	}
}

func TestRunDelete_DirEmpty(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete_dir")

	err := runDelete([]string{"--vault", vault, "--file", "nonexist/"})
	if err == nil || !strings.Contains(err.Error(), "no files registered under directory") {
		t.Errorf("expected 'no files registered' error, got: %v", err)
	}
}

// --- Directory move CLI tests ---

func TestRunMove_DirMode(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete_dir")

	err := runMove([]string{"--vault", vault, "--from", "sub/", "--to", "newdir/"})
	if err != nil {
		t.Fatalf("move dir: %v", err)
	}

	// Files should be at new location.
	if _, err := os.Stat(filepath.Join(vault, "newdir", "A.md")); err != nil {
		t.Error("newdir/A.md should exist")
	}
	if _, err := os.Stat(filepath.Join(vault, "newdir", "B.md")); err != nil {
		t.Error("newdir/B.md should exist")
	}
}

func TestRunMove_DirToMdError(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete_dir")

	err := runMove([]string{"--vault", vault, "--from", "sub/", "--to", "newdir.md"})
	if err == nil || !strings.Contains(err.Error(), "looks like a file path") {
		t.Errorf("expected file path error, got: %v", err)
	}
}

func TestRunMove_FileToDirError(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_delete_dir")

	err := runMove([]string{"--vault", vault, "--from", "Root.md", "--to", "newdir/"})
	if err == nil || !strings.Contains(err.Error(), "cannot use directory destination for single file move") {
		t.Errorf("expected directory destination error, got: %v", err)
	}
}

// --- Query --where CLI tests ---

func TestRunQuery_WhereFiltersBacklinks(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")

	// A.md has backlinks from B(active), C(done), D(no meta), E(active).
	// --where status=active should filter to B and E only.
	err := runQuery([]string{
		"--vault", vault, "--file", "A.md",
		"--fields", "backlinks", "--format", "json",
		"--where", "status=active",
	})
	if err != nil {
		t.Fatalf("query with --where: %v", err)
	}
}

func TestRunQuery_WhereInvalidExpr(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")

	err := runQuery([]string{
		"--vault", vault, "--file", "A.md",
		"--where", "=value",
	})
	if err == nil || !strings.Contains(err.Error(), "empty key") {
		t.Errorf("expected empty key error, got: %v", err)
	}
}

func TestRunQuery_WhereWithNoExclude(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")

	// --where + --no-exclude should work (config loaded for meta, exclude disabled)
	err := runQuery([]string{
		"--vault", vault, "--file", "A.md",
		"--fields", "backlinks", "--format", "json",
		"--where", "status=active", "--no-exclude",
	})
	if err != nil {
		t.Fatalf("query with --where --no-exclude: %v", err)
	}
}

func TestRunQuery_WhereAndMetaE2E(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")

	// Capture stdout to verify JSON output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runQuery([]string{
		"--vault", vault, "--file", "A.md",
		"--fields", "backlinks,meta",
		"--format", "json",
		"--where", "status=active",
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("query: %v", err)
	}

	var output bytes.Buffer
	output.ReadFrom(r)

	var m map[string]any
	if err := json.Unmarshal(output.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, output.String())
	}

	// Backlinks should be filtered to status=active notes: B and E
	backlinks, ok := m["backlinks"].([]any)
	if !ok {
		t.Fatalf("expected backlinks array, got: %v", m["backlinks"])
	}
	names := make(map[string]bool)
	for _, bl := range backlinks {
		blMap := bl.(map[string]any)
		names[blMap["name"].(string)] = true
	}
	if !names["B"] {
		t.Error("expected B in filtered backlinks")
	}
	if !names["E"] {
		t.Error("expected E in filtered backlinks")
	}
	if names["C"] {
		t.Error("C (status=done) should be filtered out")
	}
	if names["D"] {
		t.Error("D (no meta) should be filtered out")
	}
	if len(backlinks) != 2 {
		t.Errorf("expected 2 backlinks, got %d: %v", len(backlinks), names)
	}

	// Meta should be present for entry node A
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object, got: %v", m["meta"])
	}
	if _, ok := meta["priority"]; !ok {
		t.Error("expected priority in meta")
	}
	if _, ok := meta["status"]; !ok {
		t.Error("expected status in meta")
	}
}

func TestRunSearch_ComputedFieldsAndMetaKey(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")

	out := captureStdout(t, func() error {
		return runSearch([]string{
			"--vault", vault,
			"--format", "json",
			"--no-exclude",
			"--fields", "lines,outgoing_count,incoming_count,meta.status",
		})
	})

	var m struct {
		Items []struct {
			Path          string              `json:"path"`
			Lines         *int                `json:"lines"`
			OutgoingCount *int                `json:"outgoing_count"`
			IncomingCount *int                `json:"incoming_count"`
			Meta          map[string][]string `json:"meta"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}

	byPath := map[string]struct {
		Path          string              `json:"path"`
		Lines         *int                `json:"lines"`
		OutgoingCount *int                `json:"outgoing_count"`
		IncomingCount *int                `json:"incoming_count"`
		Meta          map[string][]string `json:"meta"`
	}{}
	for _, it := range m.Items {
		byPath[it.Path] = it
	}

	a := byPath["A.md"]
	if a.OutgoingCount == nil || *a.OutgoingCount != 4 {
		t.Errorf("A outgoing_count = %v, want 4", a.OutgoingCount)
	}
	if a.IncomingCount == nil || *a.IncomingCount != 4 {
		t.Errorf("A incoming_count = %v, want 4", a.IncomingCount)
	}
	if a.Lines == nil || *a.Lines <= 0 {
		t.Errorf("A lines = %v, want > 0", a.Lines)
	}
	// Only meta.status was requested, so meta must contain status and not priority.
	if got := a.Meta["status"]; len(got) != 1 || got[0] != "active" {
		t.Errorf("A meta.status = %v, want [active]", got)
	}
	if _, ok := a.Meta["priority"]; ok {
		t.Errorf("meta.priority should not be present when only meta.status requested: %v", a.Meta)
	}

	// D has no frontmatter → no meta map.
	d := byPath["D.md"]
	if len(d.Meta) != 0 {
		t.Errorf("D meta = %v, want empty", d.Meta)
	}
}

func TestRunSearch_UnknownField(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_query_where")
	err := runSearch([]string{"--vault", vault, "--fields", "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown search field") {
		t.Fatalf("expected unknown search field error, got: %v", err)
	}
}

func TestRunSearch_IsolationFlags(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_search_isolation")

	tests := []struct {
		name  string
		flag  string
		paths []string
	}{
		{name: "no tags", flag: "--no-tags", paths: []string{"B.md", "D.md"}},
		{name: "no outgoing", flag: "--no-outgoing", paths: []string{"D.md"}},
		{name: "no incoming", flag: "--no-incoming", paths: []string{"C.md", "D.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := runSearch([]string{
				"--vault", vault,
				"--format", "json",
				tt.flag,
			})

			w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Fatalf("search: %v", err)
			}

			var output bytes.Buffer
			output.ReadFrom(r)

			var m map[string]any
			if err := json.Unmarshal(output.Bytes(), &m); err != nil {
				t.Fatalf("json unmarshal: %v\noutput: %s", err, output.String())
			}
			items, ok := m["items"].([]any)
			if !ok {
				t.Fatalf("expected items array, got: %v", m["items"])
			}
			if len(items) != len(tt.paths) {
				t.Fatalf("items len = %d, want %d; output: %s", len(items), len(tt.paths), output.String())
			}
			for i, item := range items {
				itemMap := item.(map[string]any)
				if got := itemMap["path"].(string); got != tt.paths[i] {
					t.Fatalf("paths mismatch at %d: got %q, want %q; output: %s", i, got, tt.paths[i], output.String())
				}
			}
		})
	}
}

// captureStdout runs fn while capturing everything written to os.Stdout.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := fn()

	w.Close()
	os.Stdout = oldStdout

	var output bytes.Buffer
	output.ReadFrom(r)

	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, output.String())
	}
	return output.String()
}

// --- Search text CLI tests ---

func TestRunSearch_TextOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_search")

	out := captureStdout(t, func() error {
		return runSearch([]string{
			"--vault", vault,
			"--format", "text",
			"--where", "status=active",
			"--fields", "meta",
		})
	})

	if !strings.Contains(out, "total: 2\n") {
		t.Errorf("missing total: 2 in output:\n%s", out)
	}
	if !strings.Contains(out, "- note: A.md\n") {
		t.Errorf("missing A.md item in output:\n%s", out)
	}
	if !strings.Contains(out, "- note: sub/B.md\n") {
		t.Errorf("missing sub/B.md item in output:\n%s", out)
	}
	if !strings.Contains(out, "  - status: active\n") {
		t.Errorf("missing status meta in output:\n%s", out)
	}
	if !strings.Contains(out, "  - priority: 1\n") {
		t.Errorf("missing priority meta in output:\n%s", out)
	}
}

// --- Convert CLI tests ---

func TestRunConvert_MissingTo(t *testing.T) {
	err := runConvert([]string{"--vault", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "--to is required") {
		t.Errorf("expected --to required error, got: %v", err)
	}
}

func TestRunConvert_InvalidFormat(t *testing.T) {
	err := runConvert([]string{"--to", "wikilink", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunConvert_ToWikilinkDryRunJSON(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_convert")

	origNote, err := os.ReadFile(filepath.Join(vault, "Note.md"))
	if err != nil {
		t.Fatalf("read Note.md: %v", err)
	}

	out := captureStdout(t, func() error {
		return runConvert([]string{
			"--vault", vault,
			"--to", "wikilink",
			"--file", "Note.md",
			"--dry-run",
			"--format", "json",
		})
	})

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	rewritten, ok := m["rewritten"].([]any)
	if !ok || len(rewritten) == 0 {
		t.Fatalf("expected non-empty rewritten array, got: %v", m["rewritten"])
	}
	found := false
	for _, rw := range rewritten {
		rwMap := rw.(map[string]any)
		if rwMap["old"] == "[Target](Target.md)" {
			found = true
			if rwMap["file"] != "Note.md" {
				t.Errorf("file = %v, want Note.md", rwMap["file"])
			}
			if rwMap["new"] != "[[Target]]" {
				t.Errorf("new = %v, want [[Target]]", rwMap["new"])
			}
		}
	}
	if !found {
		t.Errorf("missing [Target](Target.md) rewrite in: %s", out)
	}

	// Dry-run must not modify files.
	afterNote, err := os.ReadFile(filepath.Join(vault, "Note.md"))
	if err != nil {
		t.Fatalf("read Note.md after: %v", err)
	}
	if string(origNote) != string(afterNote) {
		t.Error("dry-run modified Note.md")
	}
}

// --- Repair CLI tests ---

func TestRunRepair_InvalidFormat(t *testing.T) {
	err := runRepair([]string{"--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunRepair_DryRunJSON(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_repair")

	out := captureStdout(t, func() error {
		return runRepair([]string{
			"--vault", vault,
			"--dry-run",
			"--format", "json",
		})
	})

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}

	// X has 1 candidate → 3 rewrites, Y has 0 candidates → 1 rewrite (see core TestRepairBasic).
	rewritten, ok := m["rewritten"].([]any)
	if !ok || len(rewritten) != 4 {
		t.Fatalf("rewritten len = %d, want 4; output: %s", len(rewritten), out)
	}
	found := false
	for _, rw := range rewritten {
		rwMap := rw.(map[string]any)
		if rwMap["old"] == "[[old/path/X]]" {
			found = true
			if rwMap["file"] != "A.md" {
				t.Errorf("file = %v, want A.md", rwMap["file"])
			}
			if rwMap["new"] != "[[X]]" {
				t.Errorf("new = %v, want [[X]]", rwMap["new"])
			}
		}
	}
	if !found {
		t.Errorf("missing [[old/path/X]] rewrite in: %s", out)
	}

	// M has 2 candidates → skipped.
	skipped, ok := m["skipped"].([]any)
	if !ok || len(skipped) != 1 {
		t.Fatalf("skipped len = %d, want 1; output: %s", len(skipped), out)
	}
	sk := skipped[0].(map[string]any)
	if sk["file"] != "A.md" || sk["raw_link"] != "[[old/M]]" || sk["basename"] != "M" {
		t.Errorf("skipped[0] = %v", sk)
	}
	candidates, ok := sk["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("candidates = %v, want 2 entries", sk["candidates"])
	}
	if candidates[0] != "dir1/M.md" || candidates[1] != "dir2/M.md" {
		t.Errorf("candidates = %v, want [dir1/M.md dir2/M.md]", candidates)
	}
}

// --- Simplify CLI tests ---

func TestRunSimplify_InvalidFormat(t *testing.T) {
	err := runSimplify([]string{"--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunSimplify_DryRunJSON(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_simplify")

	out := captureStdout(t, func() error {
		return runSimplify([]string{
			"--vault", vault,
			"--file", "A.md",
			"--dry-run",
			"--format", "json",
		})
	})

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}

	rewritten, ok := m["rewritten"].([]any)
	if !ok || len(rewritten) == 0 {
		t.Fatalf("expected non-empty rewritten array, got: %v", m["rewritten"])
	}
	found := false
	for _, rw := range rewritten {
		rwMap := rw.(map[string]any)
		if rwMap["old"] == "[[sub/B]]" {
			found = true
			if rwMap["file"] != "A.md" {
				t.Errorf("file = %v, want A.md", rwMap["file"])
			}
			if rwMap["new"] != "[[B]]" {
				t.Errorf("new = %v, want [[B]]", rwMap["new"])
			}
		}
	}
	if !found {
		t.Errorf("missing [[sub/B]] rewrite in: %s", out)
	}

	// dir1/M is ambiguous (dir1/M.md, dir2/M.md) → skipped.
	skipped, ok := m["skipped"].([]any)
	if !ok || len(skipped) == 0 {
		t.Fatalf("expected non-empty skipped array, got: %v", m["skipped"])
	}
	foundSkip := false
	for _, sk := range skipped {
		skMap := sk.(map[string]any)
		if skMap["raw_link"] == "[[dir1/M]]" {
			foundSkip = true
			if skMap["basename"] != "M" {
				t.Errorf("basename = %v, want M", skMap["basename"])
			}
			candidates, ok := skMap["candidates"].([]any)
			if !ok || len(candidates) != 2 {
				t.Errorf("candidates = %v, want 2 entries", skMap["candidates"])
			}
		}
	}
	if !foundSkip {
		t.Errorf("missing [[dir1/M]] skip in: %s", out)
	}
}

// --- InitMeta CLI tests ---

func TestRunInitMeta_RequiresPresetOrScan(t *testing.T) {
	err := runInitMeta([]string{"--vault", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "--preset or --scan") {
		t.Errorf("expected preset/scan required error, got: %v", err)
	}
}

func TestRunInitMeta_ScanStdout(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_init_meta")

	out := captureStdout(t, func() error {
		return runInitMeta([]string{
			"--vault", vault,
			"--scan",
			"--no-comment",
		})
	})

	if !strings.Contains(out, "meta:") {
		t.Errorf("missing meta: section in output:\n%s", out)
	}
	if !strings.Contains(out, "types:") {
		t.Errorf("missing types: section in output:\n%s", out)
	}
	// priority is a number in all fixture notes → inferred as number.
	if !strings.Contains(out, "priority: number") {
		t.Errorf("missing inferred priority type in output:\n%s", out)
	}

	// Without --write, mdhop.yaml must not be created.
	if _, err := os.Stat(filepath.Join(vault, "mdhop.yaml")); !os.IsNotExist(err) {
		t.Errorf("mdhop.yaml should not be created without --write (stat err: %v)", err)
	}
}

// --- Reachable CLI tests ---

func TestRunReachable_JSONOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_reachable")

	out := captureStdout(t, func() error {
		return runReachable([]string{
			"--vault", vault,
			"--from", "docs/index.md",
			"--path", "docs/*",
			"--format", "json",
		})
	})

	var m struct {
		From        string   `json:"from"`
		Reachable   []string `json:"reachable"`
		Unreachable []string `json:"unreachable"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if m.From != "docs/index.md" {
		t.Errorf("from = %q, want docs/index.md", m.From)
	}
	wantReachable := []string{"docs/fp.md", "docs/fw.md", "docs/index.md", "docs/leaf.md", "docs/md.md", "docs/sub.md"}
	if !reflect.DeepEqual(m.Reachable, wantReachable) {
		t.Errorf("reachable = %v, want %v", m.Reachable, wantReachable)
	}
	if len(m.Unreachable) != 1 || m.Unreachable[0] != "docs/orphan.md" {
		t.Errorf("unreachable = %v, want [docs/orphan.md]", m.Unreachable)
	}
	if strings.Contains(out, "routes") {
		t.Errorf("routes must not appear without --route:\n%s", out)
	}
}

func TestRunReachable_TextRouteOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_reachable")

	out := captureStdout(t, func() error {
		return runReachable([]string{
			"--vault", vault,
			"--from", "docs/index.md",
			"--path", "docs/*",
			"--route",
		})
	})

	if !strings.Contains(out, "unreachable:") || !strings.Contains(out, "- docs/orphan.md") {
		t.Errorf("missing unreachable section:\n%s", out)
	}
	if !strings.Contains(out, "- docs/leaf.md: docs/index.md -> docs/sub.md -> docs/leaf.md") {
		t.Errorf("missing leaf route:\n%s", out)
	}
}

func TestRunReachable_FieldsFilter(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_reachable")

	out := captureStdout(t, func() error {
		return runReachable([]string{
			"--vault", vault,
			"--from", "docs/index.md",
			"--fields", "unreachable",
			"--format", "json",
		})
	})

	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if _, ok := m["reachable"]; ok {
		t.Errorf("reachable must be omitted with --fields unreachable:\n%s", out)
	}
	if _, ok := m["unreachable"]; !ok {
		t.Errorf("unreachable missing:\n%s", out)
	}
	// from is always included regardless of --fields (overview.md).
	if string(m["from"]) != `"docs/index.md"` {
		t.Errorf("from = %s, want \"docs/index.md\" even with --fields:\n%s", m["from"], out)
	}
}

func TestRunReachable_InvalidField(t *testing.T) {
	err := runReachable([]string{"--from", "a.md", "--fields", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error = %v, want invalid field error", err)
	}
}

func TestRunReachable_MissingFrom(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_reachable")
	err := runReachable([]string{"--vault", vault})
	if err == nil || !strings.Contains(err.Error(), "--from") {
		t.Errorf("error = %v, want missing --from error", err)
	}
}

// --- meta-check CLI tests ---

func TestRunMetaCheck_PathKind(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_meta_check")

	out := captureStdout(t, func() error {
		return runMetaCheck([]string{"--vault", vault, "--key", "sources", "--kind", "path", "--format", "json"})
	})

	var m struct {
		Issues []struct {
			SourcePath string `json:"source_path"`
			Key        string `json:"key"`
			Value      string `json:"value"`
			Reason     string `json:"reason"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.Issues) != 1 {
		t.Fatalf("issues = %+v, want 1", m.Issues)
	}
	if m.Issues[0].Value != "./missing.md" || m.Issues[0].Reason != "not_found" {
		t.Errorf("issue = %+v, want ./missing.md not_found", m.Issues[0])
	}
}

func TestRunMetaCheck_InvalidKind(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_meta_check")
	err := runMetaCheck([]string{"--vault", vault, "--key", "sources", "--kind", "bogus"})
	if err == nil || !strings.Contains(err.Error(), "invalid kind") {
		t.Fatalf("expected invalid kind error, got: %v", err)
	}
}

func TestRunMetaValidate_TypeAndEnum(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_meta_validate")

	out := captureStdout(t, func() error {
		return runMetaValidate([]string{"--vault", vault, "--format", "json"})
	})

	type violation struct {
		SourcePath string `json:"source_path"`
		Key        string `json:"key"`
		Value      string `json:"value"`
		Reason     string `json:"reason"`
	}
	var m struct {
		Violations []violation `json:"violations"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.Violations) != 2 {
		t.Fatalf("violations = %+v, want 2", m.Violations)
	}
	byReason := map[string]violation{}
	for _, v := range m.Violations {
		byReason[v.Reason] = v
	}
	tv, ok := byReason["type"]
	if !ok {
		t.Fatalf("no type violation in %+v", m.Violations)
	}
	if tv.SourcePath != "bad_date.md" || tv.Key != "updated" || tv.Value != "someday" {
		t.Errorf("type violation = %+v, want bad_date.md/updated/someday", tv)
	}
	ev, ok := byReason["enum"]
	if !ok {
		t.Fatalf("no enum violation in %+v", m.Violations)
	}
	if ev.SourcePath != "bad_enum.md" || ev.Key != "severity" || ev.Value != "urgent" {
		t.Errorf("enum violation = %+v, want bad_enum.md/severity/urgent", ev)
	}
}

func TestRunMetaValidate_Required(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_meta_validate")

	out := captureStdout(t, func() error {
		return runMetaValidate([]string{"--vault", vault, "--require", "status", "--format", "json"})
	})

	var m struct {
		Violations []struct {
			SourcePath string `json:"source_path"`
			Key        string `json:"key"`
			Value      string `json:"value"`
			Reason     string `json:"reason"`
		} `json:"violations"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	var missing []string
	for _, v := range m.Violations {
		if v.Reason != "missing" {
			continue
		}
		if v.Key != "status" {
			t.Errorf("missing key = %s, want status", v.Key)
		}
		if v.Value != "" {
			t.Errorf("missing value = %q, want empty", v.Value)
		}
		missing = append(missing, v.SourcePath)
	}
	if len(missing) != 1 || missing[0] != "missing.md" {
		t.Errorf("missing = %v, want [missing.md]", missing)
	}
}

func TestRunMetaValidate_NothingToCheck(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_build_basic")
	err := runMetaValidate([]string{"--vault", vault})
	if err == nil || !strings.Contains(err.Error(), "nothing to check") {
		t.Fatalf("expected nothing-to-check error, got: %v", err)
	}
}

// --- Graph CLI tests ---

func TestRunGraph_JSONOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_graph")

	out := captureStdout(t, func() error {
		return runGraph([]string{"--vault", vault, "--path", "docs/*"})
	})

	var m struct {
		Nodes []struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"nodes"`
		Edges []struct {
			Source   int64  `json:"source"`
			Target   int64  `json:"target"`
			LinkType string `json:"link_type"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	if len(m.Nodes) != 3 {
		t.Fatalf("nodes = %v, want 3 docs notes", m.Nodes)
	}
	paths := make(map[int64]string)
	for _, n := range m.Nodes {
		if n.Type != "note" {
			t.Errorf("node type = %q, want note", n.Type)
		}
		paths[n.ID] = n.Path
	}
	gotEdges := make(map[string]bool)
	for _, e := range m.Edges {
		gotEdges[paths[e.Source]+" -> "+paths[e.Target]+" ("+e.LinkType+")"] = true
	}
	for _, w := range []string{
		"docs/a.md -> docs/b.md (wikilink)",
		"docs/a.md -> docs/c.md (frontmatter_path)",
	} {
		if !gotEdges[w] {
			t.Errorf("missing edge %q in %v", w, gotEdges)
		}
	}
	if len(m.Edges) != 2 {
		t.Errorf("edges = %v, want 2 induced edges", m.Edges)
	}
}

func TestRunGraph_JSONPhantomPathEmpty(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_graph")

	out := captureStdout(t, func() error {
		return runGraph([]string{"--vault", vault, "--include-phantoms"})
	})

	var m struct {
		Nodes []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json unmarshal: %v\noutput: %s", err, out)
	}
	found := false
	for _, n := range m.Nodes {
		if n.Type == "phantom" {
			found = true
			if n.Name != "Ghost" {
				t.Errorf("phantom name = %q, want Ghost", n.Name)
			}
			if n.Path != "" {
				t.Errorf("phantom path = %q, want empty string", n.Path)
			}
		}
	}
	if !found {
		t.Errorf("no phantom node in JSON output:\n%s", out)
	}
}

func TestRunGraph_DotOutput(t *testing.T) {
	vault := setupVaultForCLI(t, "vault_graph")

	out := captureStdout(t, func() error {
		return runGraph([]string{"--vault", vault, "--format", "dot", "--include-phantoms"})
	})

	if !strings.HasPrefix(out, "digraph mdhop {") || !strings.HasSuffix(strings.TrimRight(out, "\n"), "}") {
		t.Errorf("not a digraph block:\n%s", out)
	}
	if !strings.Contains(out, `[label="docs/a.md"]`) {
		t.Errorf("missing node label for docs/a.md:\n%s", out)
	}
	if !strings.Contains(out, `[label="(phantom) Ghost"]`) {
		t.Errorf("missing phantom label:\n%s", out)
	}
	if !strings.Contains(out, " -> ") {
		t.Errorf("missing edges:\n%s", out)
	}
}

func TestRunGraph_InvalidFormat(t *testing.T) {
	err := runGraph([]string{"--format", "text"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error = %v, want invalid format error", err)
	}
}
