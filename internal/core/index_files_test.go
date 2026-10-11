package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilterIndexFilesParentAliasesAndFinalEntries(t *testing.T) {
	vault := t.TempDir()
	indexDir := filepath.Join(vault, "indexes")
	if err := os.Mkdir(indexDir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	// The selected final symlink is protected by its entry name; its target
	// remains an ordinary file, even though both refer to the same contents.
	if err := os.WriteFile(filepath.Join(indexDir, "target.sqlite"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.sqlite", filepath.Join(indexDir, "selected.sqlite")); err != nil {
		t.Fatal(err)
	}
	files := []string{"indexes/selected.sqlite", "indexes/selected.sqlite-journal", "indexes/selected.sqlite-wal", "indexes/selected.sqlite-shm", "indexes/selected.sqlite.tmp-build", "indexes/target.sqlite", "indexes/selected.sqlite.tmp", "indexes/other.sqlite"}
	location := []Locations{{DBPath: filepath.Join(alias, "indexes", "selected.sqlite")}}
	for _, root := range []string{vault, alias} {
		got, err := filterIndexFiles(root, files, location)
		if err != nil || !reflect.DeepEqual(got, files[5:]) {
			t.Fatalf("filter(%s) = %v, %v", root, got, err)
		}
	}
}

func TestFilterIndexFilesResolvesParentsAgainForNextOperation(t *testing.T) {
	vault := t.TempDir()
	for _, dir := range []string{"first", "second"} {
		if err := os.Mkdir(filepath.Join(vault, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(vault, "alias")
	if err := os.Symlink("first", alias); err != nil {
		t.Fatal(err)
	}
	files := []string{"first/index.sqlite", "second/index.sqlite"}
	location := []Locations{{DBPath: filepath.Join(alias, "index.sqlite")}}
	got, err := filterIndexFiles(vault, files, location)
	if err != nil || !reflect.DeepEqual(got, files[1:]) {
		t.Fatalf("first filter = %v, %v", got, err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("second", alias); err != nil {
		t.Fatal(err)
	}
	got, err = filterIndexFiles(vault, files, location)
	if err != nil || !reflect.DeepEqual(got, files[:1]) {
		t.Fatalf("next filter = %v, %v", got, err)
	}
}

func BenchmarkFilterIndexFiles(b *testing.B) {
	for _, count := range []int{42, 20000} {
		for _, explicit := range []bool{false, true} {
			b.Run(fmt.Sprintf("files=%d/explicit=%t", count, explicit), func(b *testing.B) {
				vault := b.TempDir()
				b.Setenv("XDG_CACHE_HOME", b.TempDir())
				groupSize := 100
				if count == 42 {
					groupSize = 21
				}
				files := make([]string, count)
				for i := range files {
					dir := fmt.Sprintf("group%03d", i/groupSize)
					if i%groupSize == 0 {
						if err := os.Mkdir(filepath.Join(vault, dir), 0700); err != nil {
							b.Fatal(err)
						}
					}
					files[i] = fmt.Sprintf("%s/Note%05d.md", dir, i)
				}
				var locations []Locations
				if explicit {
					locations = []Locations{{DBPath: filepath.Join(vault, "index.sqlite")}}
				}
				dbp, err := resolveDBPath(vault, locations...)
				if err != nil {
					b.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(dbp), 0700); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					got, err := filterIndexFiles(vault, files, locations)
					if err != nil || len(got) != count {
						b.Fatalf("files = %d, error = %v", len(got), err)
					}
				}
			})
		}
	}
}
