package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Require observed distinct entries, rather than assuming the OS's normalization policy.
func requireBuildUnicodeCollision(t *testing.T, parent, normalized string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		t.Logf("disk entry: %+q bytes=% x", entry.Name(), []byte(entry.Name()))
		if NormalizePath(entry.Name()) == normalized {
			count++
		}
	}
	if count != 2 {
		if runtime.GOOS == "linux" {
			t.Fatalf("Linux requires two normalization candidates, got %d", count)
		}
		if count != 1 {
			t.Fatalf("unexpected normalization candidates: %d", count)
		}
		t.Skipf("filesystem aliases NFC/NFD: enumerated %d candidate for %s", count, normalized)
	}
}

func buildUnicodeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[rel] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBuildUnicodeCollision(t *testing.T) {
	// Each publication route is exercised once; both file kinds and ancestor
	// collisions are covered without repeating every combination.
	cases := []struct{ name, mode, first, second, normalized string }{
		{"initial_note", "initial", "Café.md", "Cafe\u0301.md", "Café.md"},
		{"full_asset", "full", "Café.png", "Cafe\u0301.png", "Café.png"},
		{"incremental_ancestor", "incremental", "Café/A.md", "Cafe\u0301/B.md", "Café"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault, index := t.TempDir(), t.TempDir()
			locations := Locations{DBPath: filepath.Join(index, "index.sqlite")}
			buildTestWrite(t, vault, "Keep.md", "keep\n")
			if tc.mode != "initial" {
				if _, err := Build(vault, locations); err != nil {
					t.Fatal(err)
				}
			}
			buildTestWrite(t, vault, tc.first, "first\n")
			buildTestWrite(t, vault, tc.second, "second\n")
			requireBuildUnicodeCollision(t, vault, tc.normalized)
			beforeVault, beforeIndex := buildUnicodeSnapshot(t, vault), buildUnicodeSnapshot(t, index)
			var result *BuildResult
			var err error
			if tc.mode == "full" {
				result, err = BuildFull(vault, locations)
			} else {
				result, err = Build(vault, locations)
			}
			if err == nil || result != nil {
				t.Fatalf("collision build = %v, %v", result, err)
			}
			first, second := strings.Split(tc.first, "/")[0], strings.Split(tc.second, "/")[0]
			for _, want := range []string{"ambiguous normalized disk path: " + filepath.Join(vault, tc.normalized), fmt.Sprintf("%+q", filepath.Join(vault, first)), fmt.Sprintf("%+q", filepath.Join(vault, second))} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("diagnostic %q lacks %q", err, want)
				}
			}
			if !reflect.DeepEqual(beforeVault, buildUnicodeSnapshot(t, vault)) {
				t.Fatal("rejection modified vault inputs")
			}
			if !reflect.DeepEqual(beforeIndex, buildUnicodeSnapshot(t, index)) {
				t.Fatal("rejection published/modified DB or left temporary files")
			}
		})
	}
}

func TestBuildUnicodeExcludedCollision(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("selected_child=%t", selected), func(t *testing.T) {
			vault := t.TempDir()
			buildTestWrite(t, vault, "Keep.md", "keep\n")
			buildTestWrite(t, vault, "Café/A.md", "first\n")
			buildTestWrite(t, vault, "Cafe\u0301/B.md", "second\n")
			requireBuildUnicodeCollision(t, vault, "Café")
			excludes := "Café/**"
			if selected {
				excludes = "Café/B.md"
			}
			buildTestWrite(t, vault, "mdhop.toml", fmt.Sprintf("[build]\nexclude_paths = [%q, 'mdhop.toml']\n", excludes))
			_, err := Build(vault)
			if selected {
				if err == nil || !strings.Contains(err.Error(), "ambiguous normalized disk path") {
					t.Fatalf("selected ambiguous ancestor: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("excluded collision: %v", err)
				}
				notes := queryNodes(t, dbPath(vault), NodeTypeNote)
				if len(notes) != 1 || notes[0].path != "Keep.md" {
					t.Fatalf("notes = %+v", notes)
				}
			}
		})
	}
}
