package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCacheLocations(t *testing.T) {
	vault := t.TempDir()
	home := t.TempDir()
	cache := t.TempDir()
	t.Setenv("HOME", home)
	for _, tc := range []struct {
		name, xdg, root string
		unset           bool
	}{
		{"absolute", cache, cache, false}, {"unset", "", filepath.Join(home, ".cache"), true},
		{"empty", "", filepath.Join(home, ".cache"), false}, {"relative", "relative-cache", filepath.Join(home, ".cache"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", tc.xdg)
			if tc.unset {
				if err := os.Unsetenv("XDG_CACHE_HOME"); err != nil {
					t.Fatal(err)
				}
			}
			p, err := Paths(vault)
			if err != nil {
				t.Fatal(err)
			}
			rel, err := filepath.Rel(filepath.Join(tc.root, "mdhop", "vaults"), p.DB)
			if err != nil || strings.HasPrefix(rel, "..") {
				t.Fatalf("db=%s rel=%s error=%v", p.DB, rel, err)
			}
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) != 2 || len(parts[0]) != 64 || parts[1] != "index.sqlite" || strings.Trim(parts[0], "0123456789abcdef") != "" {
				t.Fatalf("cache key=%s", rel)
			}
		})
	}
	t.Setenv("XDG_CACHE_HOME", cache)
	if got := dbPath("/"); got != filepath.Join(cache, "mdhop", "vaults", "8a5edab282632443219e051e4ade2d1d5bbc671c781051bf1437897cbdfea0f1", "index.sqlite") {
		t.Fatalf("root hash path=%s", got)
	}

}

func TestCacheVaultIdentityAndExplicitLocations(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	if err := os.Mkdir(vault, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	original := dbPath(vault)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, vault)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{relative, alias} {
		if dbPath(v) != original {
			t.Fatalf("alias %s differs", v)
		}
	}
	if dbPath(vault, Locations{ConfigPath: "different.toml"}) != original {
		t.Fatal("config changed identity")
	}
	if dbPath(t.TempDir()) == original {
		t.Fatal("distinct vault shared cache")
	}
	moved := filepath.Join(root, "moved")
	if err := os.Rename(vault, moved); err != nil {
		t.Fatal(err)
	}
	if dbPath(moved) == original {
		t.Fatal("move retained cache identity")
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	explicit, err := resolveDBPath("missing-vault", Locations{DBPath: "chosen.sqlite"})
	if err != nil || explicit != filepath.Join(cwd, "chosen.sqlite") {
		t.Fatalf("explicit=%s error=%v", explicit, err)
	}
	if _, err := resolveDBPath(moved); err == nil {
		t.Fatal("missing home accepted")
	}
	if _, err := Paths(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing vault accepted")
	}
}

func TestCacheRebuildAndLegacyPreservation(t *testing.T) {
	vault := t.TempDir()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(vault, ".mdhop", "index.sqlite")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}); !errors.Is(err, ErrIndexNotFound) {
		t.Fatalf("legacy used: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# Updated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(cache); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}); !errors.Is(err, ErrIndexNotFound) || !strings.Contains(err.Error(), "mdhop build") {
		t.Fatalf("cache loss=%v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(legacy)
	if err != nil || string(bytes) != "legacy" {
		t.Fatalf("legacy changed: %q %v", bytes, err)
	}
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected vault files: %v", entries)
	}
}
