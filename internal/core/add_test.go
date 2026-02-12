package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddNewFile(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create a new file to add.
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

	// Verify note node exists.
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

	// Verify edges from C.
	edges := queryEdges(t, dbPath(vault), "C.md")
	if len(edges) != 2 {
		t.Fatalf("C.md edges = %d, want 2", len(edges))
	}

	// Check wikilink to A.
	var hasA bool
	for _, e := range edges {
		if e.targetName == "A" && e.linkType == "wikilink" {
			hasA = true
		}
	}
	if !hasA {
		t.Error("expected edge C→A (wikilink)")
	}

	// Check tag edge.
	var hasTag bool
	for _, e := range edges {
		if e.targetName == "#newtag" && e.linkType == "tag" {
			hasTag = true
		}
	}
	if !hasTag {
		t.Error("expected edge C→#newtag (tag)")
	}
}

func TestAddMultipleFiles(t *testing.T) {
	vault := copyVault(t, "vault_add")
	// Build with only A.md and B.md.
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create two new files that reference each other.
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
	if edgesC[0].targetType != "note" || edgesC[0].targetName != "D" {
		t.Errorf("C→D edge: type=%s name=%s, want note/D", edgesC[0].targetType, edgesC[0].targetName)
	}

	edgesD := queryEdges(t, dbPath(vault), "D.md")
	if len(edgesD) != 1 {
		t.Fatalf("D.md edges = %d, want 1", len(edgesD))
	}
	if edgesD[0].targetType != "note" || edgesD[0].targetName != "C" {
		t.Errorf("D→C edge: type=%s name=%s, want note/C", edgesD[0].targetType, edgesD[0].targetName)
	}
}

func TestAddExistingFile(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
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
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Verify phantom "NonExistent" exists before add.
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

	// Get incoming edge count for phantom before add.
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

	// Create the file and add it.
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

	// Phantom should be gone.
	phantomsAfter := queryNodes(t, dbPath(vault), "phantom")
	for _, p := range phantomsAfter {
		if p.name == "NonExistent" {
			t.Error("phantom NonExistent should be gone after promotion")
		}
	}

	// Note should exist.
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

func TestAddAmbiguousLinkInNewFile(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create two files with the same basename in different directories.
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write sub/X.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("# X\n"), 0o644); err != nil {
		t.Fatalf("write X.md: %v", err)
	}

	// Add both, then try to add a file that links to X by basename.
	result, err := Add(vault, AddOptions{Files: []string{"X.md", "sub/X.md"}})
	if err != nil {
		t.Fatalf("add X files: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2 files", result.Added)
	}

	// Now add a file with ambiguous link to X.
	if err := os.WriteFile(filepath.Join(vault, "Linker.md"), []byte("[[X]]\n"), 0o644); err != nil {
		t.Fatalf("write Linker.md: %v", err)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	_, err = Add(vault, AddOptions{Files: []string{"Linker.md"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous link") {
		t.Errorf("expected ambiguous link error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
}

func TestAddCausesExistingAmbiguity(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// A.md contains [[B]] — a basename link to B.md.
	// Adding sub/B.md would make [[B]] ambiguous.
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("# B2\n"), 0o644); err != nil {
		t.Fatalf("write sub/B.md: %v", err)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	_, err := Add(vault, AddOptions{Files: []string{"sub/B.md"}})
	if err == nil || !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("expected existing ambiguity error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
}

func TestAddPartialErrorNoChanges(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create one valid file and one already-registered file.
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
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Create a file with a vault-escaping link.
	if err := os.WriteFile(filepath.Join(vault, "Escape.md"), []byte("[link](../outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"Escape.md"}})
	if err == nil || !strings.Contains(err.Error(), "link escapes vault") {
		t.Errorf("expected vault escape error, got: %v", err)
	}
}

func TestAddAutoDisambiguateNotImplemented(t *testing.T) {
	vault := copyVault(t, "vault_add")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Add(vault, AddOptions{
		Files:            []string{"A.md"},
		AutoDisambiguate: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected not yet implemented error, got: %v", err)
	}
}

func TestAddOrphanCleanup(t *testing.T) {
	vault := copyVault(t, "vault_build_phantom")
	if err := Build(vault); err != nil {
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

	// NonExistent should be promoted.
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

	// "NonExistent" phantom should not exist.
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

	// Create a file with a self-link and build.
	if err := os.WriteFile(filepath.Join(vault, "Note.md"), []byte("# Note\n\n[[#Heading]]\n[self](#other)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := Build(vault); err != nil {
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

func TestAddDuplicateBasenameNewFiles(t *testing.T) {
	// Adding two files with the same basename simultaneously
	// when an existing phantom with basename links exists → error.
	vault := copyVault(t, "vault_build_phantom")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// A.md has [[NonExistent]] — a basename link to phantom "NonExistent".
	// Create two files named NonExistent in different dirs.
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "NonExistent.md"), []byte("# NE1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "NonExistent.md"), []byte("# NE2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	beforeNotes := countNotes(t, dbPath(vault))
	_, err := Add(vault, AddOptions{Files: []string{"NonExistent.md", "sub/NonExistent.md"}})
	if err == nil || !strings.Contains(err.Error(), "adding files would make existing links ambiguous") {
		t.Errorf("expected existing ambiguity error, got: %v", err)
	}

	// DB should be unchanged.
	afterNotes := countNotes(t, dbPath(vault))
	if beforeNotes != afterNotes {
		t.Errorf("notes changed: %d → %d", beforeNotes, afterNotes)
	}
}
