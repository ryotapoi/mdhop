package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test 1: from not registered in DB → error ---
func TestMove_NotRegistered(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "Z.md", To: "W.md"})
	if err == nil {
		t.Fatal("expected error for unregistered file")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 2: to already registered in DB → error ---
func TestMove_TargetExists(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "B.md"})
	if err == nil {
		t.Fatal("expected error for already registered destination")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 3: to exists on disk (from also on disk) → error ---
func TestMove_TargetExistsOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "C.md"})
	if err == nil {
		t.Fatal("expected error for destination existing on disk")
	}
	if !strings.Contains(err.Error(), "already exists on disk") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMove_VaultEscape(t *testing.T) {
	t.Run("from escapes vault", func(t *testing.T) {
		vault := newMoveVault(t, map[string]string{
			"a.md": "content\n",
		})

		_, err := Move(vault, MoveOptions{From: "../outside.md", To: "sub/a.md"})
		if err == nil || !strings.Contains(err.Error(), "escapes vault") {
			t.Fatalf("expected source vault escape error, got: %v", err)
		}
	})

	t.Run("to escapes vault", func(t *testing.T) {
		vault := newMoveVault(t, map[string]string{
			"a.md": "content\n",
		})

		_, err := Move(vault, MoveOptions{From: "a.md", To: "../outside.md"})
		if err == nil || !strings.Contains(err.Error(), "escapes vault") {
			t.Fatalf("expected destination vault escape error, got: %v", err)
		}
		if !fileExists(filepath.Join(vault, "a.md")) {
			t.Fatal("source file should remain in vault after rejected move")
		}
		if fileExists(filepath.Join(filepath.Dir(vault), "outside.md")) {
			t.Fatal("move must not create destination outside the vault")
		}
	})

	t.Run("absolute paths are rejected", func(t *testing.T) {
		vault := newMoveVault(t, map[string]string{
			"a.md": "content\n",
		})

		_, err := Move(vault, MoveOptions{From: filepath.Join(vault, "a.md"), To: "sub/a.md"})
		if err == nil || !strings.Contains(err.Error(), "vault-relative") {
			t.Fatalf("expected absolute source path error, got: %v", err)
		}
		_, err = Move(vault, MoveOptions{From: "a.md", To: filepath.Join(vault, "sub", "a.md")})
		if err == nil || !strings.Contains(err.Error(), "vault-relative") {
			t.Fatalf("expected absolute destination path error, got: %v", err)
		}
	})
}

// --- Test 4: move causes ambiguous links (root priority resolves) ---
// --- Test 9: phantom promotion ---
func TestMove_PhantomPromotion(t *testing.T) {
	vault := copyVault(t, "vault_move_phantom")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	dbp := dbPath(vault)

	// Before move: A.md and B.md link to [[X]] which is a phantom.
	phantoms := queryNodes(t, dbp, "phantom")
	var phantomFound bool
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "x" {
			phantomFound = true
		}
	}
	if !phantomFound {
		t.Fatal("expected phantom X before move")
	}

	// Rename A.md to X.md → basename matches phantom "X" → promote.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// After move: phantom X should be gone, replaced by note X.md.
	phantoms = queryNodes(t, dbp, "phantom")
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "x" {
			t.Error("phantom X should be promoted after move")
		}
	}

	notes := queryNodes(t, dbp, "note")
	var noteXFound bool
	for _, n := range notes {
		if n.path == "X.md" {
			noteXFound = true
		}
	}
	if !noteXFound {
		t.Error("note X.md should exist after move")
	}

	edges := queryEdges(t, dbp, "B.md")
	var bToX bool
	for _, e := range edges {
		if e.targetName == "X" && e.targetType == NodeTypeNote {
			bToX = true
		}
	}
	if !bToX {
		t.Error("B.md should link to note X after phantom promotion")
	}
}

// --- Test 10: phantom promotion with orphan cleanup verification ---
func TestMove_PhantomPromotionAndOrphanCleanup(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[Phantom1]]\n[[Phantom2]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	dbp := dbPath(vault)

	phantoms := queryNodes(t, dbp, "phantom")
	if len(phantoms) != 2 {
		t.Fatalf("expected 2 phantoms, got %d", len(phantoms))
	}

	// Move A.md to Phantom1.md → phantom "phantom1" is promoted.
	// Phantom2 is still referenced by moved file's outgoing [[Phantom2]].
	_, err := Move(vault, MoveOptions{From: "A.md", To: "Phantom1.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Phantom1 should be promoted (no longer phantom).
	phantoms = queryNodes(t, dbp, "phantom")
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "phantom1" {
			t.Error("Phantom1 should be promoted, not remain as phantom")
		}
	}
	// Phantom2 should still exist (still referenced).
	var p2Exists bool
	for _, n := range phantoms {
		if strings.ToLower(n.name) == "phantom2" {
			p2Exists = true
		}
	}
	if !p2Exists {
		t.Error("Phantom2 should still exist")
	}
}

// --- Test 11: mkdir auto-creation ---
func TestMove_MkdirAuto(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to deep/nested/A.md — directories should be auto-created.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "deep/nested/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	if !fileExists(filepath.Join(vault, "deep", "nested", "A.md")) {
		t.Error("deep/nested/A.md should exist after move")
	}
}

// --- Test 12: stale from file → error ---
func TestMove_StaleFromError(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	time.Sleep(1100 * time.Millisecond) // ensure mtime changes
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write A.md: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected stale error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 13: external rewrite succeeds even when target file has stale mtime ---
// --- Test 16: both from and to absent on disk ---
func TestMove_BothAbsentOnDisk(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
		t.Fatalf("remove A.md: %v", err)
	}

	// X.md doesn't exist on disk either → default case.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err == nil {
		t.Fatal("expected error when both from and to are absent on disk")
	}
	if !strings.Contains(err.Error(), "source file not found on disk") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClassifyDiskState_OneItemMatchesMoveDiskSwitch(t *testing.T) {
	tests := []struct {
		name         string
		writeFrom    bool
		writeTo      bool
		wantNeedMove bool
		wantErr      error
	}{
		{
			name:         "from exists only",
			writeFrom:    true,
			wantNeedMove: true,
		},
		{
			name:    "to exists only",
			writeTo: true,
		},
		{
			name:      "both exist",
			writeFrom: true,
			writeTo:   true,
			wantErr:   ErrAlreadyExistsOnDisk,
		},
		{
			name:    "neither exists",
			wantErr: ErrSourceFileMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := t.TempDir()
			if tt.writeFrom {
				writeVaultFile(t, vault, "from.md", "from\n")
			}
			if tt.writeTo {
				writeVaultFile(t, vault, "to.md", "to\n")
			}

			gotNeedMove, err := classifyDiskState(vault, []moveInfo{{from: "from.md", to: "to.md"}})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyDiskState: %v", err)
			}
			if gotNeedMove != tt.wantNeedMove {
				t.Fatalf("needDiskMove = %v, want %v", gotNeedMove, tt.wantNeedMove)
			}
		})
	}
}

// --- Test 17: same path → error ---
func TestMove_SamePath(t *testing.T) {
	vault := copyVault(t, "vault_move_error")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "A.md"})
	if err == nil {
		t.Fatal("expected error for same source and destination")
	}
	if !strings.Contains(err.Error(), "source and destination are the same") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 19: already-moved stale file → error ---
func TestMove_AlreadyMovedStale(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already moved and then edited the file.
	if err := os.MkdirAll(filepath.Join(vault, "newsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(vault, "A.md"), filepath.Join(vault, "newsub", "A.md")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "newsub", "A.md"), []byte("modified content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "newsub/A.md"})
	if err == nil {
		t.Fatal("expected stale error for already-moved file")
	}
	if !strings.Contains(err.Error(), "moved file is stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test 20: already moved on disk (from absent, to present) ---
func TestMove_AlreadyMoved(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Simulate user already did: mv A.md sub/A.md
	if err := os.MkdirAll(filepath.Join(vault, "newsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(vault, "A.md"), filepath.Join(vault, "newsub", "A.md")); err != nil {
		t.Fatal(err)
	}

	// Now run move — should detect already-moved state and just update DB + rewrite links.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "newsub/A.md"})
	if err != nil {
		t.Fatalf("move (already moved): %v", err)
	}

	if !fileExists(filepath.Join(vault, "newsub", "A.md")) {
		t.Error("newsub/A.md should exist")
	}
	if fileExists(filepath.Join(vault, "A.md")) {
		t.Error("A.md should not exist")
	}

	notes := queryNodes(t, dbPath(vault), "note")
	var found bool
	for _, n := range notes {
		if n.path == "newsub/A.md" {
			found = true
		}
	}
	if !found {
		t.Error("DB should contain newsub/A.md")
	}

	// C.md had [link to A](./A.md) — path link should be rewritten.
	var cRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "C.md" && rw.OldLink == "[link to A](./A.md)" {
			cRewritten = true
		}
	}
	if !cRewritten {
		t.Error("C.md path link should be rewritten even for already-moved case")
	}
}

// --- Test 21: file permission preserved after move with outgoing rewrite ---
