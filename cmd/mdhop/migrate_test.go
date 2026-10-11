package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestMigrateCLI(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--db", "unused"}, {"--config", "unused"}, {"positional"}} {
		if err := runMigrate(append([]string{"--vault", vault}, args...)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("invalid flags created cache files")
	}
	if err := runMigrate([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help: %v", err)
	}
	if err := runMigrate([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Status(vault); err != nil {
		t.Fatal(err)
	}
}
