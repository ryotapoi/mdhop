package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDiskPathResolverDirectoryChanges(t *testing.T) {
	vault := t.TempDir()
	disk := newReadVaultDiskPathResolver(vault)
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(vault, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	resolve := func(name string) string {
		t.Helper()
		path, err := disk.existingPath(name)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("First.md")
	resolve("First.md")
	write("Second.md")
	resolve("Second.md")
	if err := os.Rename(filepath.Join(vault, "Second.md"), filepath.Join(vault, "Third.md")); err != nil {
		t.Fatal(err)
	}
	resolve("Third.md")
	if _, err := disk.existingPath("Second.md"); !os.IsNotExist(err) {
		t.Fatalf("renamed path: %v", err)
	}
	if err := os.Remove(filepath.Join(vault, "First.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.existingPath("First.md"); !os.IsNotExist(err) {
		t.Fatalf("removed path: %v", err)
	}
	// A later operation observes newly created entries independently.
	write("Later.md")
	if _, err := newReadVaultDiskPathResolver(vault).existingPath("Later.md"); err != nil {
		t.Fatal(err)
	}
}

func TestReadDiskPathResolverSymlinkRetarget(t *testing.T) {
	vault := t.TempDir()
	first, second := t.TempDir(), t.TempDir()
	for _, item := range []struct{ dir, name string }{{first, "First.md"}, {second, "Second.md"}} {
		if err := os.WriteFile(filepath.Join(item.dir, item.name), []byte(item.name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(vault, "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	disk := newReadVaultDiskPathResolver(vault)
	if _, err := disk.existingPath("alias/First.md"); err != nil {
		t.Fatal(err)
	}
	// Match metadata to ensure followed directory identity also participates.
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(second, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.existingPath("alias/Second.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.existingPath("alias/First.md"); !os.IsNotExist(err) {
		t.Fatalf("old target: %v", err)
	}
}

func TestReadDiskPathResolverUnicodeCollision(t *testing.T) {
	vault := t.TempDir()
	actual := filepath.Join(vault, "Re\u0301sume\u0301.md")
	if err := os.WriteFile(actual, []byte("body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := newReadVaultDiskPathResolver(vault)
	if _, err := disk.existingPath("Résumé.md"); err != nil {
		t.Fatal(err)
	}
	exact := filepath.Join(vault, "Résumé.md")
	if err := os.WriteFile(exact, []byte("collision\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Skip("filesystem aliases NFC/NFD names")
	}
	if _, err := disk.existingPath("Résumé.md"); err == nil || !strings.Contains(err.Error(), "ambiguous normalized disk path") {
		t.Fatalf("exact collision: %v", err)
	}
	if err := os.Remove(exact); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.existingPath("Résumé.md"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(actual, exact); err != nil {
		t.Fatal(err)
	}
	if path, err := disk.existingPath("Résumé.md"); err != nil || path != exact {
		t.Fatalf("renamed spelling: %s, %v", path, err)
	}
	if _, err := newReadVaultDiskPathResolver(vault).existingPath("Résumé.md"); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCachedDiskNamesRejectCollision(t *testing.T) {
	vault := t.TempDir()
	actual := filepath.Join(vault, "Re\u0301sume\u0301.md")
	if err := os.WriteFile(actual, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBuild(vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Résumé.md"), []byte("collision\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Skip("filesystem aliases NFC/NFD names")
	}
	if _, err := readBuildNote(prepared.inputs, "Résumé.md"); err == nil || !strings.Contains(err.Error(), "ambiguous normalized disk path") {
		t.Fatalf("cached read: %v", err)
	}
	if _, err := buildPrepared(vault, prepared); err == nil {
		t.Fatal("publication accepted collision")
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed build changed old index")
	}
	assertNoBuildTemps(t, vault)
}

func TestReadHeadSharedResolverFreshness(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "Note.md")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	source := contentSource{path: "Note.md", mtime: info.ModTime().Unix()}
	disk := newReadVaultDiskPathResolver(vault)
	if _, err := readHead(disk, source, 1); err != nil {
		t.Fatal(err)
	}
	writeStaleTestFile(t, path, []byte("changed\n"))
	if _, err := readHead(disk, source, 1); !errors.Is(err, ErrSourceStale) {
		t.Fatalf("stale head: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := readHead(disk, source, 1); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("missing head: %v", err)
	}
}
