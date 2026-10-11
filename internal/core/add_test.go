package core

import (
	"bytes"
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

	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))

	_, err := Add(vault, AddOptions{Files: []string{"sub2/B.md"}})
	if err == nil {
		t.Fatal("expected existing ambiguity error, got nil")
	}
	if !errors.Is(err, ErrAddingMakesAmbiguous) {
		t.Errorf("errors.Is(err, ErrAddingMakesAmbiguous) = false, err: %v", err)
	}
	if !strings.Contains(err.Error(), "B") {
		t.Errorf("expected conflicting basename B in error, got: %v", err)
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

func TestAddRejectsOutsideFile(t *testing.T) {
	for _, input := range []string{"../Outside.md", "sub/../../Outside.md", ".."} {
		t.Run(input, func(t *testing.T) {
			vault := copyVault(t, "vault_add_disambiguate")
			outside := filepath.Join(filepath.Dir(vault), "Outside.md")
			outsideContent := []byte("---\ntitle: External marker\n---\n[[ExternalTarget]]\n")
			if err := os.WriteFile(outside, outsideContent, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatalf("build: %v", err)
			}
			// This valid input would rewrite existing basename links if applied.
			if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(vault, "sub2/B.md"), []byte("---\ntitle: Inside marker\n---\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			paths := []string{dbPath(vault), outside, filepath.Join(vault, "A.md"), filepath.Join(vault, "sub2/B.md")}
			before := make([][]byte, len(paths))
			for i, path := range paths {
				var err error
				before[i], err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, files := range [][]string{{input}, {"sub2/B.md", input}} {
				result, err := Add(vault, AddOptions{Files: files, AutoDisambiguate: true})
				if err == nil || !strings.Contains(err.Error(), "path escapes vault: "+input) {
					t.Fatalf("Add(%v) = %v, want path escape error", files, err)
				}
				if result != nil {
					t.Fatalf("Add(%v) returned result on rejection: %+v", files, result)
				}
				// Byte equality of the DB covers nodes, meta, and edges together.
				for i, path := range paths {
					after, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(before[i], after) {
						t.Errorf("Add(%v) changed %s", files, path)
					}
				}
			}
		})
	}
}

func TestAddAcceptsParentReferenceWithinVault(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Inside.md"), []byte("---\ntitle: Inside\n---\n[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Add(vault, AddOptions{Files: []string{"sub/../Inside.md", "Inside.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(result.Added) != 1 || result.Added[0] != "Inside.md" {
		t.Fatalf("added = %v, want [Inside.md]", result.Added)
	}
	meta := queryMetaForPath(t, dbPath(vault), "Inside.md")
	if len(meta) != 1 || meta[0].Value != "Inside" {
		t.Fatalf("meta = %+v, want title=Inside", meta)
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
	beforeNotes := countNotes(t, dbPath(vault))
	beforeEdges := countEdges(t, dbPath(vault))
	beforeA, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var phantomID int64
	if err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ? AND type = 'phantom'", phantomKey("nonexistent")).Scan(&phantomID); err != nil {
		t.Fatalf("query phantom: %v", err)
	}
	var targetBefore int64
	if err := db.QueryRow(`SELECT e.target_id FROM edges e JOIN nodes n ON n.id = e.source_id
		WHERE n.path = 'A.md' AND e.raw_link = '[[NonExistent]]'`).Scan(&targetBefore); err != nil {
		t.Fatalf("query link target: %v", err)
	}
	if targetBefore != phantomID {
		t.Fatalf("link target = %d, want phantom %d", targetBefore, phantomID)
	}

	_, err = Add(vault, AddOptions{
		Files:            []string{"sub1/NonExistent.md", "sub2/NonExistent.md"},
		AutoDisambiguate: true,
	})
	if !errors.Is(err, ErrAddingMakesAmbiguous) {
		t.Errorf("expected ErrAddingMakesAmbiguous, got: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected conflicting basename in error, got: %v", err)
	}
	if got := countNotes(t, dbPath(vault)); got != beforeNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, got)
	}
	if got := countEdges(t, dbPath(vault)); got != beforeEdges {
		t.Errorf("edges changed: %d → %d", beforeEdges, got)
	}
	var phantomType, phantomName string
	if err := db.QueryRow("SELECT type, name FROM nodes WHERE id = ?", phantomID).Scan(&phantomType, &phantomName); err != nil {
		t.Fatalf("query phantom after add: %v", err)
	}
	if phantomType != "phantom" || phantomName != "NonExistent" {
		t.Errorf("phantom changed: type=%q name=%q", phantomType, phantomName)
	}
	var targetAfter int64
	if err := db.QueryRow(`SELECT e.target_id FROM edges e JOIN nodes n ON n.id = e.source_id
		WHERE n.path = 'A.md' AND e.raw_link = '[[NonExistent]]'`).Scan(&targetAfter); err != nil {
		t.Fatalf("query link target after add: %v", err)
	}
	if targetAfter != targetBefore {
		t.Errorf("link target changed: %d → %d", targetBefore, targetAfter)
	}
	for path, want := range map[string]string{
		"A.md":                string(beforeA),
		"sub1/NonExistent.md": "# NE1\n",
		"sub2/NonExistent.md": "# NE2\n",
	} {
		got, err := os.ReadFile(filepath.Join(vault, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s changed: %q", path, got)
		}
	}
	for _, dir := range []string{"sub1", "sub2"} {
		entries, err := os.ReadDir(filepath.Join(vault, dir))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "NonExistent.md" {
			t.Errorf("unexpected files in %s: %v", dir, entries)
		}
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
	aContent := "[[B]]\n```\n[[B]]\n```\n~~~\n[[B]]\n~~~\n"
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
	// Line 6 (inside tilde fence): [[B]] → should NOT be rewritten.
	if lines[5] != "[[B]]" {
		t.Errorf("line 6 (tilde fence) = %q, want [[B]]", lines[5])
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

	if err := os.MkdirAll(filepath.Join(vault, "existing"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "existing", "Note.md"), []byte("# Note\n\n[[#Heading]]\n[self](./Note.md#other)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Add a same-basename file without a root note to exercise ambiguity checks.
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

	nodes := queryNodes(t, dbPath(vault), NodeTypeNote)
	for _, path := range []string{"existing/Note.md", "sub/Note.md"} {
		var found bool
		for _, node := range nodes {
			if node.nodeKey == noteKey(path) {
				found = true
				if node.path != path || node.existsFlag != 1 {
					t.Errorf("node %s = %+v, want path %s and exists_flag=1", path, node, path)
				}
			}
		}
		if !found {
			t.Errorf("node %s not found", path)
		}
	}

	edges := queryEdges(t, dbPath(vault), "existing/Note.md")
	if len(edges) != 2 {
		t.Fatalf("existing/Note.md edges = %+v, want 2 self-links", edges)
	}
	for i, want := range []struct {
		linkType LinkType
		rawLink  string
		subpath  string
	}{
		{LinkTypeWikilink, "[[#Heading]]", "#Heading"},
		{LinkTypeMarkdown, "[self](./Note.md#other)", "#other"},
	} {
		edge := edges[i]
		if edge.targetKey != noteKey("existing/Note.md") || edge.targetType != NodeTypeNote ||
			edge.linkType != want.linkType || edge.rawLink != want.rawLink || edge.subpath != want.subpath {
			t.Errorf("edge %d = %+v, want target %s, type note, link type %s, raw link %q, subpath %q",
				i, edge, noteKey("existing/Note.md"), want.linkType, want.rawLink, want.subpath)
		}
	}
	if edges := queryEdges(t, dbPath(vault), "sub/Note.md"); len(edges) != 0 {
		t.Errorf("sub/Note.md edges = %+v, want none", edges)
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
	if err := os.WriteFile(filepath.Join(vault, "mdhop.toml"),
		[]byte("[meta]\n[meta.types]\ndate = 'date'\n"), 0o644); err != nil {
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

	if err := os.WriteFile(filepath.Join(vault, "mdhop.toml"), []byte("[meta]\n[meta.types]\ndate = 'invalid_type'\n"), 0o644); err != nil {
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
// time, A.md frontmatter has a quoted basename link to B and a bare parent
// line that must stay untouched. Adding root B.md must trigger
// auto-disambiguate and rewrite only the quoted occurrence to [[sub/B]].
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

	// Only the quoted [[B]] is a frontmatter_wikilink edge.
	var rewriteCount int
	for _, r := range result.Rewritten {
		if r.File == "A.md" && r.OldLink == "[[B]]" {
			rewriteCount++
			if r.NewLink != "[[sub/B]]" {
				t.Errorf("rewrite NewLink = %q, want [[sub/B]]", r.NewLink)
			}
		}
	}
	if rewriteCount != 1 {
		t.Errorf("A.md frontmatter [[B]] rewrite count = %d, want 1 (quoted only)", rewriteCount)
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
	if !strings.Contains(got, `parent: [[B]]`) {
		t.Errorf("bare parent line must remain untouched, got:\n%s", got)
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
	if fmEdges != 1 {
		t.Errorf("frontmatter_wikilink edges in A.md = %d, want 1", fmEdges)
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
