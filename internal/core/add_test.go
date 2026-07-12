package core

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddReportsRewriteRollbackFailure(t *testing.T) {
	vault := copyVault(t, "vault_add_disambiguate")
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	oldRewriteWriteFile := rewriteWriteFile
	writeCalls := 0
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 2 {
			return errors.New("primary rewrite blocked")
		}
		return oldRewriteWriteFile(path, data, perm)
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(string, []byte, os.FileMode) error {
		return errors.New("restore blocked")
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true})
	assertRollbackFailureReported(t, err, "primary rewrite blocked", "restore blocked")
}

func TestAddReportsRestoreFailureAfterTransactionError(t *testing.T) {
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	primaryErr := errors.New("transaction update blocked")
	oldRewriteWriteFile := rewriteWriteFile
	applySucceeded := false
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		err := oldRewriteWriteFile(path, data, perm)
		if err == nil && filepath.Base(path) == "A.md" && strings.Contains(string(data), "[[sub/B]]") {
			applySucceeded = true
		}
		return err
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	oldRewriteTxExec := rewriteTxExec
	txAttempted := false
	rewriteTxExec = func(dbExecer, string, ...any) (sql.Result, error) {
		txAttempted = true
		return nil, primaryErr
	}
	t.Cleanup(func() { rewriteTxExec = oldRewriteTxExec })

	oldRollbackWriteFile := rollbackWriteFile
	restoreAttempted := false
	rollbackWriteFile = func(string, []byte, os.FileMode) error {
		restoreAttempted = true
		return errors.New("restore blocked")
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, err := Add(vault, AddOptions{Files: []string{"B.md"}, AutoDisambiguate: true})
	if !applySucceeded {
		t.Error("expected file rewrite to succeed before transaction failure")
	}
	if !txAttempted {
		t.Error("expected transaction update after file rewrite")
	}
	if !restoreAttempted {
		t.Error("expected deferred backup restore after transaction failure")
	}
	if !errors.Is(err, primaryErr) {
		t.Errorf("errors.Is(err, primaryErr) = false; err = %v", err)
	}
	assertRollbackFailureReported(t, err, primaryErr.Error(), "restore blocked")
}

func TestAddNewFile(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	newPath := filepath.Join(vault, "C.md")
	if err := os.WriteFile(newPath, []byte("[[A]]\n#newtag\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"C.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Added) != 1 || result.Added[0] != "C.md" {
		t.Errorf("Added = %v, want [C.md]", result.Added)
	}

	notes := queryNodes(t, dbPath(vault), "note")
	var foundC bool
	for _, n := range notes {
		if n.path == "C.md" {
			foundC = true
			if n.existsFlag != 1 {
				t.Errorf("C.md exists_flag = %d, want 1", n.existsFlag)
			}
		}
	}
	if !foundC {
		t.Error("C.md note not found after add")
	}

	edges := queryEdges(t, dbPath(vault), "C.md")
	if len(edges) != 2 {
		t.Fatalf("C.md edges = %d, want 2", len(edges))
	}

	var hasA bool
	for _, e := range edges {
		if e.targetName == "A" && e.linkType == LinkTypeWikilink {
			hasA = true
		}
	}
	if !hasA {
		t.Error("expected edge C→A (wikilink)")
	}

	var hasTag bool
	for _, e := range edges {
		if e.targetName == "#newtag" && e.linkType == LinkTypeTag {
			hasTag = true
		}
	}
	if !hasTag {
		t.Error("expected edge C→#newtag (tag)")
	}
}

func TestAddNormalizesUnicodePathToExistingPhantom(t *testing.T) {
	vault := t.TempDir()
	nfdPath := "Cafe\u0301.md"
	nfcPath := "Caf\u00e9.md"
	if err := os.WriteFile(filepath.Join(vault, "Ref.md"), []byte("[[Caf\u00e9]]\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, nfdPath), []byte("# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{nfdPath}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Added) != 1 || result.Added[0] != nfcPath {
		t.Fatalf("Added = %v, want [%s]", result.Added, nfcPath)
	}
	if len(result.Promoted) != 1 || result.Promoted[0] != nfcPath {
		t.Fatalf("Promoted = %v, want [%s]", result.Promoted, nfcPath)
	}

	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var noteCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='note' AND path = ?", nfcPath).Scan(&noteCount); err != nil {
		t.Fatalf("count NFC note: %v", err)
	}
	if noteCount != 1 {
		t.Fatalf("NFC note count = %d, want 1", noteCount)
	}
	var nfdCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='note' AND path = ?", nfdPath).Scan(&nfdCount); err != nil {
		t.Fatalf("count NFD note: %v", err)
	}
	if nfdCount != 0 {
		t.Fatalf("NFD note count = %d, want 0", nfdCount)
	}
}

func TestAddRejectsUnicodeEquivalentExistingIndexPath(t *testing.T) {
	vault := t.TempDir()
	nfdPath := "Cafe\u0301.md"
	nfcPath := "Caf\u00e9.md"
	if err := os.WriteFile(filepath.Join(vault, nfdPath), []byte("# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		"UPDATE nodes SET node_key = ?, path = ?, name = ? WHERE node_key = ?",
		"note:path:"+nfdPath, nfdPath, "Cafe\u0301", noteKey(nfcPath),
	)
	if err != nil {
		db.Close()
		t.Fatalf("simulate pre-v0.12 NFD row: %v", err)
	}
	db.Close()

	_, err = Add(vault, AddOptions{Files: []string{nfcPath}})
	if err == nil {
		t.Fatal("add succeeded, want already registered error")
	}
	if !strings.Contains(err.Error(), ErrFileAlreadyRegistered.Error()) {
		t.Fatalf("error = %v, want %v", err, ErrFileAlreadyRegistered)
	}
}

func TestAddMultipleFiles(t *testing.T) {
	vault := copyVault(t, "vault_add")
	// Build with only A.md and B.md.
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[D]]\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("[[C]]\n"), 0o644); err != nil {
		t.Fatalf("write D.md: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"C.md", "D.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}

	// Cross-references should resolve to actual notes, not phantoms.
	edgesC := queryEdges(t, dbPath(vault), "C.md")
	if len(edgesC) != 1 {
		t.Fatalf("C.md edges = %d, want 1", len(edgesC))
	}
	if edgesC[0].targetType != NodeTypeNote || edgesC[0].targetName != "D" {
		t.Errorf("C→D edge: type=%s name=%s, want note/D", edgesC[0].targetType, edgesC[0].targetName)
	}

	edgesD := queryEdges(t, dbPath(vault), "D.md")
	if len(edgesD) != 1 {
		t.Fatalf("D.md edges = %d, want 1", len(edgesD))
	}
	if edgesD[0].targetType != NodeTypeNote || edgesD[0].targetName != "C" {
		t.Errorf("D→C edge: type=%s name=%s, want note/C", edgesD[0].targetType, edgesD[0].targetName)
	}
}

func TestAddExistingFile(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Add(vault, AddOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "file already registered") {
		t.Errorf("expected file already registered error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestAddFileNotOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"NotOnDisk.md"}})
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected file not found error, got: %v", err)
	}
}

func TestAddNoDB(t *testing.T) {
	vault := copyVault(t, "vault_add")
	_, err := Add(vault, AddOptions{Files: []string{"A.md"}})
	if err == nil || !strings.Contains(err.Error(), "index not found") {
		t.Errorf("expected index not found error, got: %v", err)
	}
}

func TestAddPhantomPromotion(t *testing.T) {
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	phantomsBefore := queryNodes(t, dbPath(vault), "phantom")
	var hasPhantom bool
	for _, p := range phantomsBefore {
		if p.name == "NonExistent" {
			hasPhantom = true
		}
	}
	if !hasPhantom {
		t.Fatal("phantom NonExistent should exist before add")
	}

	db := openTestDB(t, dbPath(vault))
	var phantomID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE type='phantom' AND name='NonExistent'").Scan(&phantomID); err != nil {
		db.Close()
		t.Fatalf("query phantom: %v", err)
	}
	var incomingBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", phantomID).Scan(&incomingBefore); err != nil {
		db.Close()
		t.Fatalf("count incoming: %v", err)
	}
	db.Close()

	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NonExistent\n\nNow I exist.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"NonExistent.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Promoted) != 1 || result.Promoted[0] != "NonExistent.md" {
		t.Errorf("Promoted = %v, want [NonExistent.md]", result.Promoted)
	}

	phantomsAfter := queryNodes(t, dbPath(vault), "phantom")
	for _, p := range phantomsAfter {
		if p.name == "NonExistent" {
			t.Error("phantom NonExistent should be gone after promotion")
		}
	}

	notes := queryNodes(t, dbPath(vault), "note")
	var foundNote bool
	for _, n := range notes {
		if n.path == "NonExistent.md" {
			foundNote = true
		}
	}
	if !foundNote {
		t.Error("NonExistent.md note should exist after add")
	}

	// Incoming edges should now point to the note.
	db2 := openTestDB(t, dbPath(vault))
	defer db2.Close()
	var noteID int64
	if err := db2.QueryRow("SELECT id FROM nodes WHERE type='note' AND path='NonExistent.md'").Scan(&noteID); err != nil {
		t.Fatalf("query note: %v", err)
	}
	var incomingAfter int
	if err := db2.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ?", noteID).Scan(&incomingAfter); err != nil {
		t.Fatalf("count incoming: %v", err)
	}
	if incomingAfter != incomingBefore {
		t.Errorf("incoming edges: %d → %d (should be preserved)", incomingBefore, incomingAfter)
	}
}

func TestAddAmbiguousLinkInNewFileRootPriority(t *testing.T) {
	// X.md at root + sub/X.md → root priority resolves [[X]] to root.
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write sub/X.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write X.md: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"X.md", "sub/X.md"}})
	if err != nil {
		t.Fatalf("add X files: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}

	// Linker.md with [[X]] — root priority resolves to X.md at root → success.
	if err := os.WriteFile(filepath.Join(vault, "Linker.md"), []byte("[[X]]\n"), 0o644); err != nil {
		t.Fatalf("write Linker.md: %v", err)
	}

	_, err = Add(vault, AddOptions{Files: []string{"Linker.md"}})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
}

func TestAddAmbiguousLinkInNewFileNoRoot(t *testing.T) {
	// sub1/X.md + sub2/X.md (no root) → [[X]] is ambiguous → error.
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write sub1/X.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write sub2/X.md: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"sub1/X.md", "sub2/X.md"}})
	if err != nil {
		t.Fatalf("add X files: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}

	if err := os.WriteFile(filepath.Join(vault, "Linker.md"), []byte("[[X]]\n"), 0o644); err != nil {
		t.Fatalf("write Linker.md: %v", err)
	}

	_, err = Add(vault, AddOptions{Files: []string{"Linker.md"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("expected ambiguous link error, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "(candidates: sub1/X.md, sub2/X.md)") {
		t.Errorf("expected candidates in error, got: %v", err)
	}
}

func TestAddCausesExistingAmbiguityRootPriority(t *testing.T) {
	// B.md is at root. Adding sub/B.md → Pattern A, but old target is root → skip.
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("# B2\n"), 0o644); err != nil {
		t.Fatalf("write sub/B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"sub/B.md"}})
	if err != nil {
		t.Fatalf("expected success (root priority, Pattern A skip), got: %v", err)
	}
}

func TestAddCausesExistingAmbiguityNoRoot(t *testing.T) {
	// sub/B.md is the old unique target. Adding sub2/B.md → Pattern A, old target NOT root → error.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "sub2", "B.md"), []byte("# B2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"sub2/B.md"}})
	if err == nil || !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("expected existing ambiguity error, got: %v", err)
	}
}

func TestAddPartialErrorNoChanges(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("# C\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Add(vault, AddOptions{Files: []string{"C.md", "A.md"}})
	if err == nil || !strings.Contains(err.Error(), "file already registered") {
		t.Errorf("expected file already registered error, got: %v", err)
	}

	afterNotes := countNotes(t, dbPath(vault))
	afterEdges := countEdges(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
	if beforeEdges != afterEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, afterEdges)
	}
}

func TestAddVaultEscape(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "Escape.md"), []byte("[link](../outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"Escape.md"}})
	if err == nil || !strings.Contains(err.Error(), "link escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}
}

func TestAddEscapeVaultNonRelative(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "Escape.md"),
		[]byte("[link](sub/../../outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"Escape.md"}})
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}
}

func TestAddAutoDisambiguateBasic(t *testing.T) {
	// Pattern A: existing unique note (sub/B.md) becomes ambiguous when adding B.md.
	// With AutoDisambiguate enabled, A.md's links should be rewritten to sub/B.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create B.md at root to cause basename collision.
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Added) != 1 || result.Added[0] != "B.md" {
		t.Errorf("Added = %v, want [B.md]", result.Added)
	}

	if len(result.Rewritten) != 5 {
		t.Fatalf("Rewritten = %d, want 5", len(result.Rewritten))
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	// [[B]] → [[sub/B]]
	if lines[0] != "[[sub/B]]" {
		t.Errorf("line 1 = %q, want [[sub/B]]", lines[0])
	}
	// [[B|alias]] → [[sub/B|alias]]
	if lines[1] != "[[sub/B|alias]]" {
		t.Errorf("line 2 = %q, want [[sub/B|alias]]", lines[1])
	}
	// [[B#Heading]] → [[sub/B#Heading]]
	if lines[2] != "[[sub/B#Heading]]" {
		t.Errorf("line 3 = %q, want [[sub/B#Heading]]", lines[2])
	}
	// [link](B.md) → [link](sub/B.md)
	if lines[3] != "[link](sub/B.md)" {
		t.Errorf("line 4 = %q, want [link](sub/B.md)", lines[3])
	}
	// [link2](B.md#frag) → [link2](sub/B.md#frag)
	if lines[4] != "[link2](sub/B.md#frag)" {
		t.Errorf("line 5 = %q, want [link2](sub/B.md#frag)", lines[4])
	}
}

func TestAddAutoDisambiguateRootTarget(t *testing.T) {
	// Old target B.md is at root → Pattern A skip (root priority).
	// No rewrites needed — [[B]] still resolves to root B.md.
	vault := copyVault(t, "vault_add_disambiguate_root")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("# B sub\n"), 0o644); err != nil {
		t.Fatalf("write sub/B.md: %v", err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"sub/B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Added) != 1 {
		t.Errorf("Added = %v, want 1 file", result.Added)
	}

	// No rewrites should occur (root priority, Pattern A skip).
	if len(result.Rewritten) != 0 {
		t.Errorf("Rewritten = %d, want 0 (root priority skip)", len(result.Rewritten))
	}

	// A.md content should be unchanged — [[B]] still valid.
	contentA, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if got := strings.TrimSpace(string(contentA)); got != "[[B]]" {
		t.Errorf("A.md = %q, want [[B]] (unchanged)", got)
	}

	// sub/Source.md content should also be unchanged.
	contentS, err := os.ReadFile(filepath.Join(vault, "sub", "Source.md"))
	if err != nil {
		t.Fatalf("read sub/Source.md: %v", err)
	}
	if got := strings.TrimSpace(string(contentS)); got != "[[B]]" {
		t.Errorf("sub/Source.md = %q, want [[B]] (unchanged)", got)
	}
}

func TestAddAutoDisambiguatePatternBRootPriority(t *testing.T) {
	// Pattern B: phantom + 2 new files. NonExistent.md at root → root priority → success.
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NE1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "NonExistent.md"), []byte("# NE2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"NonExistent.md", "sub/NonExistent.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("expected success (root priority, Pattern B), got: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}
}

func TestAddAutoDisambiguatePatternBNoRoot(t *testing.T) {
	// Pattern B: phantom + 2 new files, both in subdirs (no root) → error.
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "NonExistent.md"), []byte("# NE1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "NonExistent.md"), []byte("# NE2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"sub1/NonExistent.md", "sub2/NonExistent.md"},
		AutoDisambiguate: true,
	})
	if err == nil || !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("expected ambiguity error, got: %v", err)
	}
}

func TestAddAutoDisambiguateNewFileWithRootPriority(t *testing.T) {
	// New file C.md has [[B]]. B.md at root + sub/B.md → root priority → success.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"B.md", "C.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}
}

func TestAddAutoDisambiguateNewFileAmbiguousNoRoot(t *testing.T) {
	// New file has [[B]], sub/B.md exists, add sub2/B.md (no root B) → ambiguous → error.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "B.md"), []byte("# B2\n"), 0o644); err != nil {
		t.Fatalf("write sub2/B.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"sub2/B.md", "C.md"},
		AutoDisambiguate: true,
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("expected ambiguous link error, got: %v", err)
	}
}

func TestAddAutoDisambiguateDBUpdated(t *testing.T) {
	// Verify DB edges have updated raw_link and source mtime is updated.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Check edges from A.md — raw_link should be rewritten.
	edges := queryEdges(t, dbPath(vault), "A.md")
	for _, e := range edges {
		if !isPathLinkType(e.linkType) {
			continue
		}
		if isBasenameRawLink(e.rawLink, e.linkType) {
			t.Errorf("edge raw_link %q is still a basename link after rewrite", e.rawLink)
		}
	}

	// Check that A.md's mtime in DB matches disk.
	info, err := os.Stat(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("stat A.md: %v", err)
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE path = 'A.md'").Scan(&dbMtime); err != nil {
		t.Fatalf("query mtime: %v", err)
	}
	if dbMtime != info.ModTime().Unix() {
		t.Errorf("A.md mtime: DB=%d, disk=%d", dbMtime, info.ModTime().Unix())
	}
}

func TestAddAutoDisambiguateCodeFenceIgnored(t *testing.T) {
	// Links inside code fences are not in the edge table → not rewritten.
	vault := copyVault(t, "vault_add_disambiguate")

	// Overwrite A.md with code fence content.
	aContent := "[[B]]\n```\n[[B]]\n```\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	// Line 1: [[B]] → rewritten to [[sub/B]]
	if lines[0] != "[[sub/B]]" {
		t.Errorf("line 1 = %q, want [[sub/B]]", lines[0])
	}
	// Line 3 (inside code fence): [[B]] → should NOT be rewritten
	if lines[2] != "[[B]]" {
		t.Errorf("line 3 (code fence) = %q, want [[B]]", lines[2])
	}
}

func TestAddAutoDisambiguateInlineCodeIgnored(t *testing.T) {
	// Inline code `[[B]]` should not be rewritten, but [[B]] outside should be.
	vault := copyVault(t, "vault_add_disambiguate")

	aContent := "[[B]] and `[[B]]` here\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	got := strings.TrimSpace(string(content))
	want := "[[sub/B]] and `[[B]]` here"
	if got != want {
		t.Errorf("A.md = %q, want %q", got, want)
	}
}

func TestAddAutoDisambiguateEmbed(t *testing.T) {
	// Embed ![[B]] should be rewritten to ![[sub/B]].
	vault := copyVault(t, "vault_add_disambiguate")

	aContent := "![[B]]\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	got := strings.TrimSpace(string(content))
	want := "![[sub/B]]"
	if got != want {
		t.Errorf("A.md = %q, want %q", got, want)
	}
}

func TestAddAutoDisambiguateStaleMtimeErrors(t *testing.T) {
	// If source file mtime doesn't match DB, error should occur with no changes.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Tamper with A.md's mtime in DB to simulate stale state. We edit the DB
	// directly rather than rewriting the file because os.WriteFile within the
	// same second yields the same Unix mtime, so a file write alone cannot
	// simulate staleness.
	db := openTestDB(t, dbPath(vault))
	if _, err := db.Exec("UPDATE nodes SET mtime = mtime - 100 WHERE path = 'A.md'"); err != nil {
		db.Close()
		t.Fatalf("update mtime: %v", err)
	}
	db.Close()

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err == nil || !strings.Contains(err.Error(), "source file is stale") {
		t.Errorf("expected stale error, got: %v", err)
	}

	// DB should be unchanged (no new notes added).
	afterNotes := countNotes(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}

	// A.md content should not have been rewritten.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if strings.Contains(string(content), "sub/B") {
		t.Error("A.md should not have been rewritten on stale error")
	}
}

func TestAddAutoDisambiguateExtensionPreserved(t *testing.T) {
	// markdown link extension preservation + wikilink .md removal.
	vault := copyVault(t, "vault_add_disambiguate")

	aContent := "[[B.md]]\n[text](B)\n[text2](B.md)\n"
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	// [[B.md]] → [[sub/B]] (wikilink always strips .md)
	if lines[0] != "[[sub/B]]" {
		t.Errorf("line 1 = %q, want [[sub/B]]", lines[0])
	}
	// [text](B) → [text](sub/B) (no extension preserved)
	if lines[1] != "[text](sub/B)" {
		t.Errorf("line 2 = %q, want [text](sub/B)", lines[1])
	}
	// [text2](B.md) → [text2](sub/B.md) (extension preserved)
	if lines[2] != "[text2](sub/B.md)" {
		t.Errorf("line 3 = %q, want [text2](sub/B.md)", lines[2])
	}
}

func TestAddAutoDisambiguateRebuildConsistent(t *testing.T) {
	// After auto-disambiguate, a full rebuild should produce the same DB state.
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	addEdges := countEdges(t, dbPath(vault))
	addNotes := countNotes(t, dbPath(vault))

	if _, err := Build(vault); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// Verify rebuild succeeds and produces same counts.
	rebuildEdges := countEdges(t, dbPath(vault))
	rebuildNotes := countNotes(t, dbPath(vault))

	if addNotes != rebuildNotes {
		t.Errorf("notes: add=%d, rebuild=%d", addNotes, rebuildNotes)
	}
	if addEdges != rebuildEdges {
		t.Errorf("edges: add=%d, rebuild=%d", addEdges, rebuildEdges)
	}

	// Verify the rewritten links in A.md resolve correctly.
	edgesA := queryEdges(t, dbPath(vault), "A.md")
	for _, e := range edgesA {
		if !isPathLinkType(e.linkType) {
			continue
		}
		if isBasenameRawLink(e.rawLink, e.linkType) {
			t.Errorf("after rebuild, edge raw_link %q is still basename", e.rawLink)
		}
	}
}

func TestAddAutoDisambiguateRestoreBackups(t *testing.T) {
	// Verify restoreBackups correctly restores file content.
	dir := t.TempDir()

	original := []byte("original content\n")
	filePath := "test.md"
	if err := os.WriteFile(filepath.Join(dir, filePath), []byte("modified content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	backups := []rewriteBackup{
		{path: filePath, content: original, perm: 0o644},
	}
	if failures := restoreBackupFiles(dir, backups); len(failures) != 0 {
		t.Fatalf("restoreBackupFiles failures: %#v", failures)
	}

	restored, err := os.ReadFile(filepath.Join(dir, filePath))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(restored) != string(original) {
		t.Errorf("restored = %q, want %q", string(restored), string(original))
	}
}

func TestAddOrphanCleanup(t *testing.T) {
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// After build, "Missing" is a phantom (A.md links to [[Missing|alias]]).
	// "NonExistent" is also a phantom (A.md links to [[NonExistent]]).
	// Create NonExistent.md to promote it, and add a file that doesn't link to Missing.
	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NonExistent\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"NonExistent.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if len(result.Promoted) != 1 || result.Promoted[0] != "NonExistent.md" {
		t.Errorf("Promoted = %v, want [NonExistent.md]", result.Promoted)
	}

	// "Missing" phantom should still exist (A.md still links to it).
	phantoms := queryNodes(t, dbPath(vault), "phantom")
	var hasMissing bool
	for _, p := range phantoms {
		if p.name == "Missing" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Error("phantom Missing should still exist (A.md references it)")
	}

	for _, p := range phantoms {
		if p.name == "NonExistent" {
			t.Error("phantom NonExistent should be gone after promotion")
		}
	}
}

func TestAddSelfLinkNotBlockedByAmbiguity(t *testing.T) {
	// A self-link like [[#Heading]] should not be treated as a basename link,
	// so adding a file with the same basename should be allowed.
	vault := copyVault(t, "vault_add")

	if err := os.WriteFile(filepath.Join(vault, "Note.md"), []byte("# Note\n\n[[#Heading]]\n[self](#other)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a same-basename file in a subdirectory — should succeed because
	// the only existing link to "Note" is a self-link, not a basename link.
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "Note.md"), []byte("# Note2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"sub/Note.md"}})
	if err != nil {
		t.Fatalf("add should succeed but got: %v", err)
	}
}

func TestAddDuplicateBasenameNewFilesRootPriority(t *testing.T) {
	// Adding NonExistent.md (root) + sub/NonExistent.md → root priority → success.
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NE1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "NonExistent.md"), []byte("# NE2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"NonExistent.md", "sub/NonExistent.md"}})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}
}

func TestAddDuplicateBasenameNewFilesNoRoot(t *testing.T) {
	// Adding sub1/NonExistent.md + sub2/NonExistent.md (no root) → error.
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "NonExistent.md"), []byte("# NE1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "NonExistent.md"), []byte("# NE2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"sub1/NonExistent.md", "sub2/NonExistent.md"}})
	if err == nil || !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("expected existing ambiguity error, got: %v", err)
	}
}

func TestAddPhantomPromotionRootPriority(t *testing.T) {
	// Phantom [[NonExistent]] exists. Add sub/NonExistent.md and NonExistent.md (root).
	// Root file should be promoted (not the sub one).
	vault := copyVault(t, "vault_build_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Add sub first in list, then root — root should still win for promotion.
	if err := os.WriteFile(filepath.Join(vault, "sub", "NonExistent.md"), []byte("# NE sub\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NE root\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := Add(vault, AddOptions{Files: []string{"sub/NonExistent.md", "NonExistent.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}

	// Phantom should be promoted to root NonExistent.md.
	if len(result.Promoted) != 1 || result.Promoted[0] != "NonExistent.md" {
		t.Errorf("Promoted = %v, want [NonExistent.md] (root priority)", result.Promoted)
	}

	// Incoming edges from A.md should point to root NonExistent.md.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var targetPath string
	err = db.QueryRow(`
		SELECT n.path FROM edges e
		JOIN nodes sn ON sn.id = e.source_id AND sn.path = 'A.md'
		JOIN nodes n ON n.id = e.target_id
		WHERE e.raw_link = '[[NonExistent]]'
	`).Scan(&targetPath)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if targetPath != "NonExistent.md" {
		t.Errorf("[[NonExistent]] target = %q, want NonExistent.md (root)", targetPath)
	}
}

// --- Meta add tests ---

func TestAddMetaInsert(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("---\ntitle: Hello\nauthor: Alice\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"B.md"}}); err != nil {
		t.Fatalf("add: %v", err)
	}

	meta := queryMetaForPath(t, dbPath(vault), "B.md")
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta rows, got %d: %+v", len(meta), meta)
	}
	// ORDER BY key, value → author first, then title.
	if meta[0].Key != "author" || meta[0].Value != "Alice" {
		t.Errorf("meta[0] = %+v, want author=Alice", meta[0])
	}
	if meta[1].Key != "title" || meta[1].Value != "Hello" {
		t.Errorf("meta[1] = %+v, want title=Hello", meta[1])
	}
}

func TestAddMetaNoFrontmatter(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{"B.md"}}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if c := countMeta(t, dbPath(vault)); c != 0 {
		t.Errorf("expected 0 meta rows, got %d", c)
	}
}

func TestAddMetaWarnings(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"),
		[]byte("meta:\n  types:\n    date: date\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("---\ndate: not-a-date\n---\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Add(vault, AddOptions{Files: []string{"B.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestAddInvalidConfig(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	beforeNotes := countNotes(t, dbPath(vault))

	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("meta:\n  types:\n    date: invalid_type\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"B.md"}})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("unexpected error: %v", err)
	}

	afterNotes := countNotes(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
}

func TestAddAutoDisambiguateSubdirTarget(t *testing.T) {
	// Old unique target sub/B.md. Add B.md at root → Pattern A, old target NOT root.
	// Auto-disambiguate rewrites [[B]] → [[sub/B]].
	vault := copyVault(t, "vault_add_disambiguate")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Rewrites should happen — old target is sub/B.md (not root).
	if len(result.Rewritten) != 5 {
		t.Fatalf("Rewritten = %d, want 5", len(result.Rewritten))
	}

	// Verify rewrite content.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	if lines[0] != "[[sub/B]]" {
		t.Errorf("line 1 = %q, want [[sub/B]]", lines[0])
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("rebuild after auto-disambiguate: %v", err)
	}
}

// Add must apply the same vault-escape guard to frontmatter wikilinks as
// build does. Adding a new file whose frontmatter wikilink escapes the vault
// should fail add.
func TestAdd_FrontmatterWikilinkEscapesVault(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	newFile := `---
parent: "[[../escape]]"
---
# C
`
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte(newFile), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"C.md"}})
	if err == nil {
		t.Fatal("expected vault escape error for frontmatter wikilink, got nil")
	}
	if !strings.Contains(err.Error(), "escapes vault") {
		t.Errorf("error = %q, want containing 'escapes vault'", err.Error())
	}
}

// Pattern A with a frontmatter wikilink: sub/B.md is the unique B at build
// time, A.md frontmatter has both quoted and bare basename links to B. Adding
// root B.md must trigger auto-disambiguate and rewrite both occurrences to
// [[sub/B]] while preserving quoted/bare style.
func TestAddAutoDisambiguateFrontmatterWikilink(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	aContent := `---
related: "[[B]]"
parent: [[B]]
---
# A
`
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte(aContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("# B sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create root B.md to trigger Pattern A auto-disambiguate.
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Both bare and quoted [[B]] in A.md frontmatter share the rawLink "[[B]]".
	// applyFileRewrites runs replaceOutsideInlineCode on the line containing
	// each edge — so both lines get rewritten in a single pass.
	var rewriteCount int
	for _, r := range result.Rewritten {
		if r.File == "A.md" && r.OldLink == "[[B]]" {
			rewriteCount++
			if r.NewLink != "[[sub/B]]" {
				t.Errorf("rewrite NewLink = %q, want [[sub/B]]", r.NewLink)
			}
		}
	}
	if rewriteCount != 2 {
		t.Errorf("A.md frontmatter [[B]] rewrite count = %d, want 2 (quoted + bare)", rewriteCount)
		for _, r := range result.Rewritten {
			t.Logf("  %s: %s → %s", r.File, r.OldLink, r.NewLink)
		}
	}

	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, `related: "[[sub/B]]"`) {
		t.Errorf("A.md should contain quoted form, got:\n%s", got)
	}
	if !strings.Contains(got, `parent: [[sub/B]]`) {
		t.Errorf("A.md should contain bare form, got:\n%s", got)
	}

	// DB edges must reflect the rewritten rawLinks (not stale).
	edges := queryEdges(t, dbPath(vault), "A.md")
	var fmEdges int
	for _, e := range edges {
		if e.linkType != LinkTypeFrontmatterWikilink {
			continue
		}
		fmEdges++
		if e.rawLink != "[[sub/B]]" {
			t.Errorf("DB edge raw_link = %q, want [[sub/B]] (stale or unrewritten)", e.rawLink)
		}
	}
	if fmEdges != 2 {
		t.Errorf("frontmatter_wikilink edges in A.md = %d, want 2", fmEdges)
	}
}

// vault_add_disambiguate_frontmatter has both a frontmatter wikilink and a
// body wikilink to basename B. Adding root B.md must trigger auto-disambiguate
// and the post-add DB edges must contain no basename rawLinks for any
// path-resolving link type — including frontmatter_wikilink.
func TestAddAutoDisambiguateFrontmatterDBUpdated(t *testing.T) {
	vault := copyVault(t, "vault_add_disambiguate_frontmatter")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	edges := queryEdges(t, dbPath(vault), "A.md")
	var fmEdges int
	for _, e := range edges {
		if !isPathLinkType(e.linkType) {
			continue
		}
		if isBasenameRawLink(e.rawLink, e.linkType) {
			t.Errorf("edge raw_link %q (type %s) is still a basename link after rewrite", e.rawLink, e.linkType)
		}
		if e.linkType == LinkTypeFrontmatterWikilink {
			fmEdges++
		}
	}
	if fmEdges == 0 {
		t.Error("expected at least one frontmatter_wikilink edge from A.md after rewrite")
	}
}

// Same vault as TestAddAutoDisambiguateFrontmatterDBUpdated, but verifies that
// a full rebuild after auto-disambiguate produces the same DB state and that
// frontmatter_wikilink edges still resolve away from the ambiguous basename.
func TestAddAutoDisambiguateFrontmatterRebuildConsistent(t *testing.T) {
	vault := copyVault(t, "vault_add_disambiguate_frontmatter")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("# B root\n"), 0o644); err != nil {
		t.Fatalf("write B.md: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"B.md"},
		AutoDisambiguate: true,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	addEdges := countEdges(t, dbPath(vault))
	addNotes := countNotes(t, dbPath(vault))

	if _, err := Build(vault); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	rebuildEdges := countEdges(t, dbPath(vault))
	rebuildNotes := countNotes(t, dbPath(vault))

	if addNotes != rebuildNotes {
		t.Errorf("notes: add=%d, rebuild=%d", addNotes, rebuildNotes)
	}
	if addEdges != rebuildEdges {
		t.Errorf("edges: add=%d, rebuild=%d", addEdges, rebuildEdges)
	}

	edgesA := queryEdges(t, dbPath(vault), "A.md")
	var fmEdges int
	for _, e := range edgesA {
		if !isPathLinkType(e.linkType) {
			continue
		}
		if isBasenameRawLink(e.rawLink, e.linkType) {
			t.Errorf("after rebuild, edge raw_link %q (type %s) is still basename", e.rawLink, e.linkType)
		}
		if e.linkType == LinkTypeFrontmatterWikilink {
			fmEdges++
		}
	}
	if fmEdges == 0 {
		t.Error("expected at least one frontmatter_wikilink edge from A.md after rebuild")
	}
}
