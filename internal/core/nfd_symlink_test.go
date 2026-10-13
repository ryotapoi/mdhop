package core

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func nfdSymlinkFixture(t *testing.T, vault string, external bool) string {
	t.Helper()
	target := filepath.Join(vault, "backing")
	if external {
		target = t.TempDir()
	}
	if err := os.MkdirAll(filepath.Join(target, "Re\u0301sume\u0301", "Prive\u0301"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(vault, "Cafe\u0301")); err != nil {
		t.Fatal(err)
	}
	observeMoveNFD(t, vault, "Cafe\u0301")
	observeMoveNFD(t, vault, "Cafe\u0301/Re\u0301sume\u0301")
	return target
}

func TestNFDSymlinkResolver(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			vault := t.TempDir()
			target := nfdSymlinkFixture(t, vault, external)
			actual := filepath.Join(vault, "Cafe\u0301/Re\u0301sume\u0301/Prive\u0301/A\u0301.md")
			if err := os.WriteFile(filepath.Join(target, "Re\u0301sume\u0301/Prive\u0301/A\u0301.md"), []byte("# Content\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			resolver := newVaultDiskPathResolver(vault)
			for _, resolve := range []func(string) (string, error){resolver.existingPath, resolver.movePath} {
				got, err := resolve("Café/Résumé/Privé/Á.md")
				if err != nil || got != actual {
					t.Fatalf("resolved = %q, %v; want %q", got, err, actual)
				}
			}
			got, err := resolver.movePath("Café/Résumé/Privé/new/deep/Á.md")
			want := filepath.Join(vault, "Cafe\u0301/Re\u0301sume\u0301/Prive\u0301/new/deep/Á.md")
			if err != nil || got != want {
				t.Fatalf("missing suffix = %q, %v; want %q", got, err, want)
			}
			terminal := filepath.Join(target, "Re\u0301sume\u0301/Prive\u0301/De\u0301ad.md")
			if err := os.Symlink(filepath.Join(target, "missing"), terminal); err != nil {
				t.Fatal(err)
			}
			got, err = resolver.movePath("Café/Résumé/Privé/Déad.md")
			if err != nil || got != filepath.Join(vault, "Cafe\u0301/Re\u0301sume\u0301/Prive\u0301/De\u0301ad.md") {
				t.Fatalf("terminal = %q, %v", got, err)
			}
			got, err = resolver.existingPath("Café/Résumé/Privé/Déad.md")
			if err != nil || got != filepath.Join(vault, "Cafe\u0301/Re\u0301sume\u0301/Prive\u0301/De\u0301ad.md") {
				t.Fatalf("dangling entry = %q, %v", got, err)
			}
			if _, err := os.Stat(got); !os.IsNotExist(err) {
				t.Fatalf("dangling target Stat = %v", err)
			}
		})
	}
}

func TestNFDSymlinkAdd(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			vault := t.TempDir()
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			target := nfdSymlinkFixture(t, vault, external)
			const rel = "Café/Résumé/Privé/Á.md"
			writeVaultFile(t, target, "Re\u0301sume\u0301/Prive\u0301/A\u0301.md", "# Content\n[[Target]]\n")
			if _, err := Add(vault, AddOptions{Files: []string{rel}}); err != nil {
				t.Fatal(err)
			}
			nodes := queryNodes(t, dbPath(vault), "note")
			if len(nodes) != 1 || nodes[0].path != rel || nodes[0].nodeKey != noteKey(rel) {
				t.Fatalf("nodes = %+v", nodes)
			}
			db, err := openDBChecked(vault)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			id, _, err := findEntryNode(db, EntrySpec{File: rel})
			if err != nil {
				t.Fatal(err)
			}
			source, err := queryHeadSource(db, id)
			if err != nil {
				t.Fatal(err)
			}
			head, err := readHead(newReadVaultDiskPathResolver(vault), source, 2)
			if err != nil || !reflect.DeepEqual(head, []string{"# Content", "[[Target]]"}) {
				t.Fatalf("content = %v, %v", head, err)
			}
		})
	}
}

func TestMoveNFDSymlink(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{"Target.md": "# Target\n"})
			target := nfdSymlinkFixture(t, vault, external)
			writeVaultFile(t, target, "Re\u0301sume\u0301/A\u0301.md", "[target](../../Target.md)\n")
			const from = "Café/Résumé/Á.md"
			const to = "Café/Résumé/Privé/new/Á.md"
			if _, err := Add(vault, AddOptions{Files: []string{from}}); err != nil {
				t.Fatal(err)
			}
			if _, err := Move(vault, MoveOptions{From: from, To: to}); err != nil {
				t.Fatal(err)
			}
			if got := readVaultFile(t, target, "Re\u0301sume\u0301/Prive\u0301/new/Á.md"); got != "[target](../../../../Target.md)\n" {
				t.Fatalf("rewrite = %q", got)
			}
			if _, err := os.Lstat(filepath.Join(target, "Re\u0301sume\u0301/A\u0301.md")); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			assertMoveNFDIndex(t, vault, from, to)
			entries, err := os.ReadDir(filepath.Join(target, "Re\u0301sume\u0301"))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, entry := range entries {
				if NormalizePath(entry.Name()) == "Privé" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("destination parents = %d", count)
			}
		})
	}
}

func TestNFDSymlinkCollision(t *testing.T) {
	for _, component := range []string{"ancestor", "child"} {
		t.Run(component, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{"Source.md": "source\n"})
			target := nfdSymlinkFixture(t, vault, true)
			writeVaultFile(t, target, "Re\u0301sume\u0301/A.md", "keep\n")
			collisionParent, collisionName := vault, "Café"
			if component == "child" {
				collisionParent, collisionName = target, "Résumé"
			}
			if err := os.Mkdir(filepath.Join(collisionParent, collisionName), 0o755); err != nil && !os.IsExist(err) {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(collisionParent)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, entry := range entries {
				t.Logf("collision entry: %q bytes=% x", entry.Name(), []byte(entry.Name()))
				if NormalizePath(entry.Name()) == collisionName {
					count++
				}
			}
			if count != 2 {
				if runtime.GOOS == "linux" {
					t.Fatalf("Linux requires two normalization candidates, got %d", count)
				}
				t.Skipf("filesystem aliases NFC/NFD: candidates=%d", count)
			}
			before := queryNodes(t, dbPath(vault), "note")
			for _, resolve := range []func(string) (string, error){newVaultDiskPathResolver(vault).existingPath, newVaultDiskPathResolver(vault).movePath} {
				if _, err := resolve("Café/Résumé/A.md"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
					t.Fatalf("collision error = %v", err)
				}
			}
			if _, err := Add(vault, AddOptions{Files: []string{"Café/Résumé/A.md"}}); err == nil {
				t.Fatal("Add accepted collision")
			}
			if _, err := Move(vault, MoveOptions{From: "Source.md", To: "Café/Résumé/new/A.md"}); err == nil {
				t.Fatal("Move accepted collision")
			}
			if !reflect.DeepEqual(before, queryNodes(t, dbPath(vault), "note")) {
				t.Fatal("rejection changed DB")
			}
			if got := readVaultFile(t, vault, "Source.md"); got != "source\n" {
				t.Fatal("source changed")
			}
			if got := readVaultFile(t, target, "Re\u0301sume\u0301/A.md"); got != "keep\n" {
				t.Fatal("child changed")
			}
			if _, err := os.Stat(filepath.Join(target, "Re\u0301sume\u0301/new")); !os.IsNotExist(err) {
				t.Fatalf("destination created: %v", err)
			}
		})
	}
}
