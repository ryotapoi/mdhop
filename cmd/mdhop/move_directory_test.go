package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func moveDirectoryVault(t *testing.T) (string, core.Locations) {
	t.Helper()
	vault := t.TempDir()
	locations := core.Locations{DBPath: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := os.Mkdir(filepath.Join(vault, "Cafe\u0301"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{"Cafe\u0301/A.md": "# A\n", "Cafe\u0301/B.md": "# B\n", "Index.md": "[a](Café/A.md)\n[b](Café/B.md)\n"} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Logf("disk entry: name=%q bytes=% x", entry.Name(), []byte(entry.Name()))
	}
	_, err = os.Stat(filepath.Join(vault, "Café"))
	t.Logf("filesystem condition: os=%s NFC-direct-stat=%v", runtime.GOOS, err)
	if runtime.GOOS == "linux" && !os.IsNotExist(err) {
		t.Fatalf("requires distinct NFC/NFD paths: %v", err)
	}
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := core.Build(vault, locations); err != nil {
		t.Fatal(err)
	}
	return vault, locations
}

func TestRunMove_NFDDirectory(t *testing.T) {
	for _, mode := range []string{"to", "template", "dry-run", "slash", "symlink", "already-moved-slash"} {
		t.Run(mode, func(t *testing.T) {
			vault, locations := moveDirectoryVault(t)
			from := "Café"
			if mode == "slash" || mode == "already-moved-slash" {
				from += "/"
			}
			if mode == "symlink" {
				if err := os.Symlink("Cafe\u0301", filepath.Join(vault, "Alias")); err != nil {
					t.Fatal(err)
				}
				// Index notes via the symlink, as build intentionally does not walk it.
				for _, name := range []string{"A.md", "B.md"} {
					if _, err := core.Add(vault, core.AddOptions{Files: []string{"Alias/" + name}}, locations); err != nil {
						t.Fatal(err)
					}
				}
				from = "Alias"
			}
			if mode == "already-moved-slash" {
				if err := os.Rename(filepath.Join(vault, "Cafe\u0301"), filepath.Join(vault, "Archive")); err != nil {
					t.Fatal(err)
				}
			}
			var before map[string]string
			if mode == "dry-run" {
				before = snapshotVaultFiles(t, vault)
			}
			dbBefore, err := os.ReadFile(locations.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"--vault", vault, "--db", locations.DBPath, "--from", from, "--format", "json"}
			if mode == "template" || mode == "dry-run" || mode == "symlink" {
				args = append(args, "--to-template", "Archive/{basename}")
				if mode == "dry-run" {
					args = append(args, "--dry-run")
				}
			} else {
				args = append(args, "--to", "Archive")
			}
			out := captureStdout(t, func() error { return runMove(args) })
			var got struct {
				From  string           `json:"from"`
				Moved []core.MovedFile `json:"moved"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			logicalFrom := strings.TrimSuffix(from, "/")
			want := []core.MovedFile{{From: logicalFrom + "/A.md", To: "Archive/A.md"}, {From: logicalFrom + "/B.md", To: "Archive/B.md"}}
			if got.From != "" || !reflect.DeepEqual(got.Moved, want) {
				t.Fatalf("directory output = %s, want %+v", out, want)
			}
			if mode == "dry-run" {
				dbAfter, err := os.ReadFile(locations.DBPath)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, snapshotVaultFiles(t, vault)) || string(dbBefore) != string(dbAfter) {
					t.Fatal("dry-run changed disk or DB")
				}
				return
			}
			for _, moved := range want {
				if _, err := os.Stat(filepath.Join(vault, moved.To)); err != nil {
					t.Fatal(err)
				}
				if _, err := core.Query(vault, core.EntrySpec{File: moved.To}, core.QueryOptions{}, locations); err != nil {
					t.Fatal(err)
				}
				if _, err := core.Query(vault, core.EntrySpec{File: moved.From}, core.QueryOptions{}, locations); err == nil {
					t.Fatalf("old DB path remains: %s", moved.From)
				}
			}
			if mode != "symlink" {
				body, err := os.ReadFile(filepath.Join(vault, "Index.md"))
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != "[a](/Archive/A.md)\n[b](/Archive/B.md)\n" {
					t.Fatalf("links: %s", body)
				}
			}
		})
	}
}

func TestRunMove_NFDDirectoryCollision(t *testing.T) {
	for _, mode := range []string{"to", "template", "dry-run", "slash"} {
		t.Run(mode, func(t *testing.T) {
			vault, locations := moveDirectoryVault(t)
			err := os.Mkdir(filepath.Join(vault, "Café"), 0o755)
			if err != nil && !os.IsExist(err) {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(vault)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, entry := range entries {
				if core.NormalizePath(entry.Name()) == "Café" {
					count++
					t.Logf("collision entry: %q bytes=% x", entry.Name(), []byte(entry.Name()))
				}
			}
			if count != 2 {
				if runtime.GOOS == "linux" {
					t.Fatalf("requires two distinct entries, got %d", count)
				}
				t.Skipf("filesystem aliases NFC/NFD: normalized entry count=%d", count)
			}
			before := snapshotVaultFiles(t, vault)
			dbBefore, err := os.ReadFile(locations.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			from := "Café"
			if mode == "slash" {
				from += "/"
			}
			args := []string{"--vault", vault, "--db", locations.DBPath, "--from", from, "--format", "json"}
			if mode == "template" || mode == "dry-run" {
				args = append(args, "--to-template", "Archive/{basename}")
				if mode == "dry-run" {
					args = append(args, "--dry-run")
				}
			} else {
				args = append(args, "--to", "Archive")
			}
			out := captureStdout(t, func() error {
				err := runMove(args)
				if err == nil || !strings.Contains(err.Error(), "ambiguous normalized disk path") {
					t.Errorf("collision error = %v", err)
				}
				return nil
			})
			dbAfter, err := os.ReadFile(locations.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			if out != "" || !reflect.DeepEqual(before, snapshotVaultFiles(t, vault)) || string(dbBefore) != string(dbAfter) {
				t.Fatal("collision produced success or changed disk/DB")
			}
		})
	}
}

func TestRunMove_AlreadyMovedFile(t *testing.T) {
	vault, locations := moveDirectoryVault(t)
	if err := os.Rename(filepath.Join(vault, "Cafe\u0301/A.md"), filepath.Join(vault, "Moved.md")); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error {
		return runMove([]string{"--vault", vault, "--db", locations.DBPath, "--from", "Café/A.md", "--to", "Moved.md", "--format", "json"})
	})
	var got struct {
		From, To string
		Moved    []core.MovedFile
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.From != "Café/A.md" || got.To != "Moved.md" || got.Moved != nil {
		t.Fatalf("file output: %s", out)
	}
	if _, err := core.Query(vault, core.EntrySpec{File: "Moved.md"}, core.QueryOptions{}, locations); err != nil {
		t.Fatal(err)
	}
}
