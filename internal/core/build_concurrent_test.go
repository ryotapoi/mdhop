package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertNoBuildTemps(t *testing.T, vault string) {
	t.Helper()
	paths, err := filepath.Glob(dbPath(vault) + ".tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("build left temporary resources: %v", paths)
	}
}

// Each caller gets its own snapshot and resolve maps, as in Build.
func prepareBuildSnapshot(t *testing.T, owner string) *preparedBuild {
	t.Helper()
	vault := t.TempDir()
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("%s-%03d.md", owner, i)
		content := fmt.Sprintf("---\nowner: %s\n---\n[[%s-%03d]]\n", owner, owner, (i+1)%100)
		if err := os.WriteFile(filepath.Join(vault, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := prepareBuild(vault)
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func assertBuildSnapshot(t *testing.T, vault string, owners ...string) {
	t.Helper()
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity = %q, error %v", integrity, err)
	}
	var owner string
	if err := db.QueryRow("SELECT value FROM meta LIMIT 1").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	allowed := false
	for _, want := range owners {
		allowed = allowed || owner == want
	}
	if !allowed {
		t.Fatalf("snapshot owner = %q, want one of %v", owner, owners)
	}
	for _, query := range []string{
		"SELECT COUNT(*) FROM nodes WHERE type='note'",
		"SELECT COUNT(*) FROM nodes WHERE type='note' AND path LIKE ? || '-%'",
		"SELECT COUNT(*) FROM edges",
		"SELECT COUNT(*) FROM meta",
		"SELECT COUNT(*) FROM meta WHERE key='owner' AND value=?",
	} {
		var count int
		var args []any
		if strings.Contains(query, "?") {
			args = append(args, owner)
		}
		if err := db.QueryRow(query, args...).Scan(&count); err != nil || count != 100 {
			t.Fatalf("%s = %d, error %v; want 100", query, count, err)
		}
	}
	var dangling int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges e
		LEFT JOIN nodes s ON s.id=e.source_id LEFT JOIN nodes d ON d.id=e.target_id
		WHERE s.id IS NULL OR d.id IS NULL`).Scan(&dangling); err != nil || dangling != 0 {
		t.Fatalf("dangling edges = %d, error %v", dangling, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM meta m LEFT JOIN nodes n ON n.id=m.node_id
		WHERE n.id IS NULL`).Scan(&dangling); err != nil || dangling != 0 {
		t.Fatalf("dangling meta = %d, error %v", dangling, err)
	}
}

func TestBuildConcurrentSnapshots(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(fmt.Sprintf("failing=%v", failing), func(t *testing.T) {
			vault := t.TempDir()
			if _, err := ensureDataDir(vault); err != nil {
				t.Fatal(err)
			}
			// Start from a published index. This also initializes SQLite's process
			// PRNG before concurrent opens (v1.29.0 races on its first PID reset).
			if _, err := buildPrepared(vault, prepareBuildSnapshot(t, "existing")); err != nil {
				t.Fatal(err)
			}
			first := prepareBuildSnapshot(t, "first")
			second := prepareBuildSnapshot(t, "second")
			if failing {
				// Fail in Pass 2, after the private DB and note rows exist.
				second.notes[0].links = []linkOccur{{target: "../outside.md", rawLink: "[[../outside]]", isRelative: true}}
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			for _, prepared := range []*preparedBuild{first, second} {
				go func(p *preparedBuild) {
					<-start
					_, err := buildPrepared(vault, p)
					results <- err
				}(prepared)
			}
			close(start)
			completed := []error{<-results, <-results}
			failures := 0
			for _, err := range completed {
				if err != nil {
					if !failing || !errors.Is(err, ErrLinkEscapesVault) {
						t.Fatalf("unexpected build failure: %v", err)
					}
					failures++
				}
			}
			if failing && failures != 1 {
				t.Fatalf("failures = %d, want 1", failures)
			}
			if failing {
				assertBuildSnapshot(t, vault, "first")
			} else {
				assertBuildSnapshot(t, vault, "first", "second")
			}
			assertNoBuildTemps(t, vault)
		})
	}
}

func TestBuildPreparedFailurePreservesIndexAndForeignTemp(t *testing.T) {
	vault := t.TempDir()
	if _, err := ensureDataDir(vault); err != nil {
		t.Fatal(err)
	}
	if _, err := buildPrepared(vault, prepareBuildSnapshot(t, "existing")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	// Model another build's live resource, not just the old fixed name.
	foreign, err := os.CreateTemp(filepath.Dir(dbPath(vault)), dbFileName+".tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	defer os.Remove(foreign.Name())
	if _, err := foreign.WriteString("other build"); err != nil {
		t.Fatal(err)
	}
	prepared := prepareBuildSnapshot(t, "failed")
	prepared.notes[0].links = []linkOccur{{target: "../outside.md", rawLink: "[[../outside]]", isRelative: true}}
	if _, err := buildPrepared(vault, prepared); !errors.Is(err, ErrLinkEscapesVault) {
		t.Fatalf("build error = %v, want path escape", err)
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("index changed after failed DB construction, error %v", err)
	}
	if got, err := os.ReadFile(foreign.Name()); err != nil || string(got) != "other build" {
		t.Fatalf("foreign temp = %q, error %v", got, err)
	}
	paths, err := filepath.Glob(dbPath(vault) + ".tmp-*")
	if err != nil || len(paths) != 1 || paths[0] != foreign.Name() {
		t.Fatalf("remaining temp paths = %v, error %v; want only foreign temp", paths, err)
	}
	assertBuildSnapshot(t, vault, "existing")
}

func TestBuildPublishFailureCleansPrivateTemp(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(dbPath(vault), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dbPath(vault), "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := buildPrepared(vault, prepareBuildSnapshot(t, "failed"))
	var pathErr *os.LinkError
	if !errors.As(err, &pathErr) || pathErr.Op != "rename" {
		t.Fatalf("build error = %v, want rename failure", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep" {
		t.Fatalf("publish destination changed: %q, error %v", got, err)
	}
	assertNoBuildTemps(t, vault)
}
